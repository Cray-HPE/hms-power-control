// MIT License
//
// (C) Copyright [2020-2022] Hewlett Packard Enterprise Development LP
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
	"sort"
	"sync"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/sirupsen/logrus"
)

type TRSKafkaConsumer struct {
	Errors    chan error
	Responses chan *kafka.Message
	Logger    *logrus.Logger
}

type TRSKafkaClient struct {
	Producer      *kafka.Producer
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
	tasksChan chan *kafka.Message, logger *logrus.Logger) (err error) {

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

	connected := false

	// Spin up Kafka Producer
	var kafkaProducer *kafka.Producer
	var kafkaConnectErr error
	for !connected {
		kafkaProducer, kafkaConnectErr = kafka.NewProducer(&kafka.ConfigMap{
								"bootstrap.servers": brokerSpec})

		if kafkaConnectErr != nil {
			trsKafka.Logger.Errorf("BrokerAddress: %s -- Unable to connect to Kafka! Trying again in 1 second... err: %s",
				brokerSpec, kafkaConnectErr)
			time.Sleep(1 * time.Second)
		} else {
			connected = true
		}
	}
	trsKafka.Logger.Infof("Broker: %s -- Connected to Kafka server.",
		brokerSpec)

	// Now, setup a consumer on the response channel.  The client might
	// not care about responses, but we're going to listen for them anyway.

	kafkaConsumer, kafkaConsumerErr := kafka.NewConsumer(&kafka.ConfigMap{
								"bootstrap.servers": brokerSpec,
								"broker.address.family": "v4",
								"group.id": consumerGroupName,
								"session.timeout.ms": 10000,
								"auto.offset.reset": "latest"})
	if kafkaConsumerErr != nil {
		err = fmt.Errorf("unable to setup consumer: %s", kafkaConsumerErr)
		return
	}

	trsKafka.Logger.Tracef("NewConsumer() succeeded.")

	consumer := &TRSKafkaConsumer{
		Errors:    make(chan error),
		Responses: tasksChan,
	}

	consumer.Logger = trsKafka.Logger

	//Subscribe to initial topics.

	subErr := kafkaConsumer.SubscribeTopics(trsKafka.RcvTopicNames,nil)
	if (subErr != nil) {
		err = fmt.Errorf("Error subscribing to topics: '%v': %v",
				trsKafka.RcvTopicNames,subErr)
		return
	}

	//Consumer thread, reads messages and handles error messages.

	go func() {
		topix := trsKafka.RcvTopicNames
		for {
			ev := kafkaConsumer.Poll(500)	//Block no more than .5 sec
			switch e := ev.(type) {
				case *kafka.Message:
					consumer.Responses <-e

				case *kafka.Error:
					err = fmt.Errorf(e.Error())
					consumer.Errors <-err
					//TODO: really kill the loop on consume error?
					return

				default:
					trsKafka.Logger.Tracef("Kafka poll() info: %v",e)
			}

			// Check if context was cancelled, signaling that the consumer
			// should stop.

			if ctx.Err() != nil {
				consumer.Errors <- ctx.Err()
				err = ctx.Err()
				return
			}

			//Check for topic changes.  If there are changes, apply them.

			if (newTopics(topix,trsKafka.RcvTopicNames)) {
				topix = trsKafka.RcvTopicNames
				subErr := kafkaConsumer.SubscribeTopics(trsKafka.RcvTopicNames,nil)
				if (subErr != nil) {
					trsKafka.Logger.Errorf("Changing consumer topics to '%v' failed...")
				}
			}
		}
	}()

	//Producer thread.  Drains error messages only.
	go func() {
		evChan := kafkaProducer.Events()
		for {
			e,isOpen := <-evChan
			if (!isOpen) {
				trsKafka.Logger.Tracef("Closing kafka producer thread, producer is gone.")
				return
			}

			switch ev := e.(type) {
				case *kafka.Message:
					m := ev
					if m.TopicPartition.Error != nil {
						logger.Errorf("Kafka message delivery failed: key: '%s', topic: '%s', partition: %d, error: %v",
							string(m.Key),
							*m.TopicPartition.Topic,
							m.TopicPartition.Partition,
							m.TopicPartition.Error)
					} else {
						logger.Tracef("Kafka delivered message to topic %s [%d] at offset %v",
							*m.TopicPartition.Topic, m.TopicPartition.Partition,
							m.TopicPartition.Offset)
					}

				default:
					logger.Tracef("Kafka producer: Ignored event: %s", ev)
				}
		}
	}()

	trsKafka.Client = &TRSKafkaClient{
		Producer:      kafkaProducer,
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
	msg := &kafka.Message{TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny}, Value: payload}

	(*trsKafka.Client.Producer).ProduceChannel() <- msg

	trsKafka.Logger.Tracef("Sent message; topic: %s, msg.Value: %s",
		topic, string(payload))
}

func (trsKafka *TRSKafka) SetTopics(topics []string) (err error) {
	if len(topics) == 0 {
		err = fmt.Errorf("Invalid receiver topics")
		return
	}
	trsKafka.Logger.Tracef("Current topics: %s", trsKafka.RcvTopicNames)
	trsKafka.Mux.Lock()
	trsKafka.RcvTopicNames = topics
	trsKafka.Mux.Unlock()
	trsKafka.Logger.Tracef("New topics: %s", trsKafka.RcvTopicNames)
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

// Convenience function, checks topic list to see if it has changed.

func newTopics(currTopics []string, newTopics []string) bool {
	isNew := false
	if (len(currTopics) != len(newTopics)) {
		isNew = true
	} else {
		//Pain, should sort to be sure someone didn't just re-do
		//the same topics but in a different order.
		tSorted := currTopics
		sort.Strings(tSorted)
		rtnSorted := newTopics
		sort.Strings(rtnSorted)
		for ix := 0; ix < len(tSorted); ix ++ {
			if (tSorted[ix] != rtnSorted[ix]) {
				isNew = true
				break
			}
		}
	}

	return isNew
}

