// MIT License
//
// (C) Copyright [2020-2021] Hewlett Packard Enterprise Development LP
//
// Permission is hereby granted, free of charge, to any person obtaining a
// copy of this software and associated documentation files (the "Software"),
// to deal in the Software without restriction, including without limitation
// the rights to use, copy, modify, merge, publish, distribute, sublicense,
// and/or sell copies of the Software, and to permit persons to whom the
// Software is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included
// in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
// THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR
// OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
// ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
// OTHER DEALINGS IN THE SOFTWARE.

package trs_kafka

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/Shopify/sarama"
	"github.com/sirupsen/logrus"
)

type TRSKafkaConsumer struct {
	Errors    chan error
	Responses chan *sarama.ConsumerMessage
	Logger    *logrus.Logger
}

type TRSKafkaClient struct {
	Config        *sarama.Config
	Producer      *sarama.AsyncProducer
	ConsumerGroup *sarama.ConsumerGroup
	Consumer      *TRSKafkaConsumer
	Logger        *logrus.Logger
}

type TRSKafka struct {
	Client           *TRSKafkaClient
	ConsumerShutdown chan int
	RcvTopicNames    []string
	Mux              sync.Mutex
	Logger           *logrus.Logger
}

/////////////////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////////////////
// Support for log-like logging with a more generic interface
/////////////////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////////////////

/////////////////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////////////////
// Functions to support the sarama kafka ConsumerGroup interface
/////////////////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////////////////

func (consumer *TRSKafkaConsumer) Setup(sarama.ConsumerGroupSession) error {
	// Push nil into the errors channel to signify that we've started up and
	// are ready to rock.

	consumer.Logger.Tracef("Kafka Setup() entered.\n")
	consumer.Errors <- nil
	consumer.Logger.Tracef("Kafka Setup() complete.\n")
	return nil
}

func (consumer *TRSKafkaConsumer) Cleanup(sarama.ConsumerGroupSession) error {
	consumer.Logger.Tracef("Kafka Cleanup() complete.\n")
	return nil
}

func (consumer *TRSKafkaConsumer) ConsumeClaim(session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		// If the worker library wants responses, send it now.
		if consumer.Responses != nil {
			consumer.Responses <- message
		}
		consumer.Logger.Tracef("Message claimed: key = '%s', timestamp = %v, topic = %s",
			string(message.Key), message.Timestamp, message.Topic)
		session.MarkMessage(message, "")
	}

	return nil
}

func handleConsumerErrors(trsKafka *TRSKafka) {
	for kafkaError := range (*trsKafka.Client.ConsumerGroup).Errors() {
		trsKafka.Logger.Errorf("Failed to consume message, error: '%v'\n",
			kafkaError)
	}
}

func handleProducerErrors(trsKafka *TRSKafka) {
	for kafkaError := range (*trsKafka.Client.Producer).Errors() {
		trsKafka.Logger.Errorf("Failed to produce message;  Msg.Topic: %s, Msg.Err: %v\n",
			kafkaError.Msg.Topic, kafkaError.Err)
	}
}

func handleProducerSuccesses(trsKafka *TRSKafka) {
	for kafkaSuccess := range (*trsKafka.Client.Producer).Successes() {
		bytes, _ := kafkaSuccess.Value.Encode()
		trsKafka.Logger.Tracef("Produced message;  Msg.Topic: %s\t Msg.Value: %s\n",
			kafkaSuccess.Topic, string(bytes))
	}
}

/////////////////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////////////////
// TRS Kafka API Functions
/////////////////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////////////////

// This function sets up a kafka interface capable of publishing tasks to
// a topic and consuming data from a TRS worker.
//
// ctx:           Cancel-able global context that everything runs under.
// appName:       Application name, used for constructing topics.
// workerClass:   TRS worker class.  Currently REST.
// consumerGroup: Kafka response consumer group.  nil == random.
//                Random will cause all instances of your app to receive
//                all responses from TRS workers.
// brokerSpec:    Kafka broker to use (e.g. 10.2.3.4:9092)
// tasksChan:     Channel through which TRS responses are consumed. "" == N/A.

func (trsKafka *TRSKafka) Init(ctx context.Context,
	initialReceiveTopics []string,
	consumerGroup string,
	brokerSpec string,
	tasksChan chan *sarama.ConsumerMessage, logger *logrus.Logger) (err error) {

	if logger != nil {
		trsKafka.Logger = logger
	} else {
		trsKafka.Logger = logrus.New()
	}

	var consumerGroupName string

	if len(initialReceiveTopics) == 0 {
		err = fmt.Errorf("Invalid empty initial receiver topic.")
		return
	}

	if ctx == nil {
		err = fmt.Errorf("Non-nil context required.")
		return
	}

	if brokerSpec == "" {
		err = fmt.Errorf("Non-nil broker specification is required.")
		return
	}
	trsKafka.Mux.Lock()
	trsKafka.RcvTopicNames = initialReceiveTopics
	trsKafka.Mux.Unlock()

	//Consumer group is only for responses.  nil == use a random one.
	if consumerGroup == "" {
		s1 := rand.NewSource(time.Now().UnixNano())
		r1 := rand.New(s1)
		consumerGroupName = fmt.Sprintf("%d", r1.Int31())
	} else {
		consumerGroupName = consumerGroup
	}

	// Setup the Kafka configuration.
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Version = sarama.V2_3_0_0

	// Producer
	kafkaConfig.Producer.RequiredAcks = sarama.WaitForAll
	kafkaConfig.Producer.Compression = sarama.CompressionNone
	kafkaConfig.Producer.Retry.Max = 5
	kafkaConfig.Producer.Return.Successes = true
	kafkaConfig.Producer.Return.Errors = true

	// Consumer
	kafkaConfig.Consumer.Return.Errors = true
	brokers := []string{brokerSpec}
	connected := false

	// Spin up Kafka Producer
	var kafkaProducer sarama.AsyncProducer
	var kafkaConnectErr error
	for !connected {
		kafkaProducer, kafkaConnectErr = sarama.NewAsyncProducer(brokers, kafkaConfig)
		if kafkaConnectErr != nil {
			trsKafka.Logger.Errorf("BrokerAddress: %s -- Unable to connect to Kafka! Trying again in 1 second... err: %s",
				brokerSpec, kafkaConnectErr)
			time.Sleep(1 * time.Second)
		} else {
			connected = true
		}
	}
	trsKafka.Logger.Infof("Broker: %s -- Connected to Kafka server.\n",
		brokerSpec)

	// Now, setup a consumer on the response channel.  The client might
	// not care about responses, but we're going to listen for them anyway.

	kafkaConsumer, kafkaConsumerErr := sarama.NewConsumerGroup(brokers, consumerGroupName, kafkaConfig)
	if kafkaConsumerErr != nil {
		err = fmt.Errorf("unable to setup consumer group: %s", kafkaConsumerErr)
		return
	}

	trsKafka.Logger.Tracef("NewConsumerGroup() succeeded.\n")

	consumer := &TRSKafkaConsumer{
		Errors:    make(chan error),
		Responses: tasksChan,
	}

	consumer.Logger = trsKafka.Logger

	go func() {
		for {
			trsKafka.Mux.Lock()
			tmpTopics := trsKafka.RcvTopicNames
			trsKafka.Mux.Unlock()
			consumeErr := kafkaConsumer.Consume(ctx, tmpTopics, consumer)
			if consumeErr != nil {
				consumer.Errors <- consumeErr
				err = consumeErr
				return
			}

			// Check if context was cancelled, signaling that the consumer
			// should stop.

			if ctx.Err() != nil {
				consumer.Errors <- ctx.Err()
				err = ctx.Err()
				return
			}
		}
	}()

	// Wait for the consumer to start up.
	consumerErr := <-consumer.Errors
	if consumerErr != nil {
		err = fmt.Errorf("error from consumer: %s", consumerErr)
		return
	}

	trsKafka.Logger.Tracef("consumerErr received, Kafka channel is running.\n")

	trsKafka.Client = &TRSKafkaClient{Config: kafkaConfig,
		Producer:      &kafkaProducer,
		ConsumerGroup: &kafkaConsumer,
		Consumer:      consumer,
	}
	trsKafka.Client.Logger = trsKafka.Logger
	trsKafka.ConsumerShutdown = make(chan int)

	// Now that we know the client is up and running, spin up a goroutine
	// to monitor the context and when it cancels cleanup the client.
	go func() {
		select {
		case <-trsKafka.ConsumerShutdown:
		case <-ctx.Done():
			// Best attempt, no point in complaining, we're ending anyway.
			_ = kafkaConsumer.Close()
		}
	}()

	// MUST listen for errors otherwise deadlock will happen and this thing
	// grinds to a halt.
	go handleProducerErrors(trsKafka)
	go handleProducerSuccesses(trsKafka)
	go handleConsumerErrors(trsKafka)

	return
}

// Convenience function to create topic names for a service.  Given the
// app name, worker class, and consumer group names, returns the "cooked"
// send, receive topic and the consumer group.
//
// appName:       Name of this application.
// workerClass:   Application class (e.g. "REST")
// consumerGroup: If empty, a random consumer group will be generated.
//                Otherwise a predictable consumer group name is generated.
// Return:        Formatted send and receive topics, and consumer group name.

func GenerateSendReceiveConsumerGroupName(appName, workerClass, consumerGroup string) (sendTopic string, rcvTopic string, cgroupName string) {
	cgroupName = ""
	if consumerGroup == "" {
		s1 := rand.NewSource(time.Now().UnixNano())
		r1 := rand.New(s1)
		cgroupName = fmt.Sprintf("%s-%s-%d", appName, workerClass, r1.Int31())
	} else {
		cgroupName = fmt.Sprintf("%s-%s-%s", appName, workerClass, consumerGroup)
	}
	sendTopic = fmt.Sprintf("trs-%s-%s-send", appName, workerClass)
	rcvTopic = fmt.Sprintf("trs-%s-%s-rcv", appName, workerClass)
	return
}

// This function will cause the given payload to be written using the
// topic specified when opening the connection.

func (trsKafka *TRSKafka) Write(topic string, payload []byte) {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		//Key:   sarama.StringEncoder(key.String()),
		Value: sarama.ByteEncoder(payload),
	}

	(*trsKafka.Client.Producer).Input() <- msg

	trsKafka.Logger.Tracef("Sent message; topic: %s, msg.Value: %s\n", topic, string(payload))
}

func (trsKafka *TRSKafka) SetTopics(topics []string) (err error) {
	if len(topics) == 0 {
		err = fmt.Errorf("Invalid receiver topics")
		return
	}
	trsKafka.Logger.Tracef("Current topics: %s\n", trsKafka.RcvTopicNames)
	trsKafka.Mux.Lock()
	trsKafka.RcvTopicNames = topics
	trsKafka.Mux.Unlock()
	trsKafka.Logger.Tracef("New topics: %s\n", trsKafka.RcvTopicNames)
	return
}

// Shut down a Kafka client connection

func (trsKafka *TRSKafka) Shutdown() {
	if trsKafka.Client != nil {
		trsKafka.ConsumerShutdown <- 1
		if trsKafka.Client.Producer != nil {
			(*trsKafka.Client.Producer).Close()
		}
	}
}
