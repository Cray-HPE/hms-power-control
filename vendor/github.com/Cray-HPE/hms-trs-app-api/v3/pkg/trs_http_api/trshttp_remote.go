// MIT License
// 
// (C) Copyright [2020-2022,2024] Hewlett Packard Enterprise Development LP
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

package trs_http_api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"os"
	"github.com/Cray-HPE/hms-base/v2"
	tkafka "github.com/Cray-HPE/hms-trs-kafkalib/v2/pkg/trs-kafkalib"
	"strings"
	"time"
)

/////////////////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////////////////
//                 R E M O T E   I N T E R F A C E
/////////////////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////////////////

// Initialize a remote HTTP task system.  NOTE: if multiple instances of
// an HTTP interface are to be used at the same time, then one of ServiceName,
// or sender must be unique to each instance.
//
// ServiceName: Name of running service/application.
// logger:    logger instance the library should use.
// Return:      Error string if something went wrong.

func (tloc *TRSHTTPRemote) Init(serviceName string, logger *logrus.Logger) error {
	if logger != nil {
		tloc.Logger = logger
	} else {
		tloc.Logger = logrus.New()
	}

	tloc.svcName = strings.ToLower(serviceName)

	tloc.brokerSpec = "kafka:9092"
	envstr := os.Getenv("BROKER_SPEC")
	if envstr != "" {
		tloc.brokerSpec = envstr
	} else {
		tloc.Logger.Warningf("env var: BROKER_SPEC is not set; falling back to: %s", tloc.brokerSpec)
	}

	tloc.Logger.Infof("BROKER_SPEC: %s", tloc.brokerSpec)

	tloc.ctx, tloc.ctxCancelFunc = context.WithCancel(context.Background())

	if tloc.taskMap == nil {
		tloc.taskMutex.Lock()
		tloc.taskMap = make(map[uuid.UUID]*taskChannelTuple)
		tloc.taskMutex.Unlock()
	}

	tloc.kafkaRspChan = make(chan *kafka.Message)
	tloc.KafkaInstance = &tkafka.TRSKafka{}

	//tkafka.DebugLevel(5)
	//This makes the send topic / return topic global for the instance of the TRSHTTPRemote obj.
	var tmpRecTop string
	tloc.sendTopic, tmpRecTop, tloc.consumerGroup = tkafka.GenerateSendReceiveConsumerGroupName(tloc.svcName, "http-v1", "")

	tloc.receiveTopics = append(tloc.receiveTopics, tmpRecTop)

	tloc.Logger.Tracef("SEND: %s, RECEIVE: %s, ConsumerGroup: %s", tloc.sendTopic, tloc.receiveTopics, tloc.consumerGroup)
	kafkaLogger := logrus.New()
	err := tloc.KafkaInstance.Init(tloc.ctx, tloc.receiveTopics, tloc.consumerGroup, tloc.brokerSpec, tloc.kafkaRspChan, kafkaLogger)
	if err != nil {
		tloc.Logger.Error(err)
	}

	go func() {
		var tData HttpKafkaRx
		for {
			select {
			//This was a MAJOR PAIN!!! : https://stackoverflow.com/questions/3398490/checking-if-a-channel-has-a-ready-to-read-value-using-go
			case raw, ok := <-tloc.kafkaRspChan:
				if ok {
					tloc.Logger.Debugf("RECEIVED: '%s'\n", string(raw.Value))
					//unmarshal, then look up task tuple, put into it's chan
					err := json.Unmarshal(raw.Value, &tData)
					if err != nil {
						tloc.Logger.Printf("ERROR unmarshaling received data: %v\n", err)
						continue
					}
					taskMapEntry, ok := tloc.taskMap[tData.ID]
					if !ok {
						tloc.Logger.Printf("ERROR, can't find task map entry for msgID %d\n",
							tData.ID)
						break
					}
					resp := tData.Response.ToHttpResponse()
					tloc.taskMutex.Lock()
					tloc.taskMap[tData.ID].task.Request.Response = &resp
					tloc.taskMap[tData.ID].task.Err = tData.Err
					tloc.taskMutex.Unlock()

					taskMapEntry.taskListChannel <- taskMapEntry.task

				} else {
					tloc.Logger.Info("Kafka Response Channel closed! Exiting Go Routine")
					return
				}
			}
		}
	}()

	return err
}

// Set up security paramters.  This is a place holder for now.

func (tloc *TRSHTTPRemote) SetSecurity(inParams interface{}) error {
	return nil
}

// Create an array of task descriptors.  Copy data from the source task
// into each element of the returned array.  Per-task data has to be
// populated separately by the caller.
//
// The message ID in each task is populated regardless of the value in
// the source.   It is generated using a pseudo-random value in the upper
// 32 bits, which is the message group ID, followed by a monotonically
// increasing value in the lower 32 bits, starting with 0, which functions
// as the message ID.
//
// source:   Ptr to a task descriptor populated with relevant data.
// numTasks: Number of elements in the returned array.
// Return:   Array of populated task descriptors.

func (tloc *TRSHTTPRemote) CreateTaskList(source *HttpTask, numTasks int) []HttpTask {
	return createHTTPTaskArray(source, numTasks)
}

// Launch an array of tasks.  This is non-blocking.  Use Check() to get
// current status of the task launch.
//
// taskList:  Ptr to a list of HTTP tasks to launch.
// Return:    Chan of *HttpTxTask, sized by task list, which caller can
//            use to get notified of each task completion, or safely
//            ignore.  CALLER MUST CLOSE.
//            Error message if something went wrong with the launch.

func (tloc *TRSHTTPRemote) Launch(taskList *[]HttpTask) (chan *HttpTask, error) {
	var retErr error
	stockError := errors.New("error encountered while processing, check tasks for error ")

	if len(*taskList) == 0 {
		emptyChannel := make(chan *HttpTask, 1)
		err := fmt.Errorf("empty task list, nothing to do")
		return emptyChannel, err
	}

	taskListChannel := make(chan *HttpTask, len(*taskList))

	for ii := 0; ii < len(*taskList); ii++ {
		if (*taskList)[ii].Ignore == true {
			continue
		}

		//Always set the response to nil; make sure its clean, add
		//user-agent header
		if (*taskList)[ii].Request != nil {
			base.SetHTTPUserAgent((*taskList)[ii].Request,tloc.svcName)
			(*taskList)[ii].Request.Response = nil
		}
		//make sure the id is set
		if (*taskList)[ii].id == uuid.Nil {
			(*taskList)[ii].id = uuid.New()
		}

		//make sure the service name is set
		if (*taskList)[ii].ServiceName == "" {
			(*taskList)[ii].ServiceName = tloc.svcName
		}

		//make sure the timestamp is set
		if (*taskList)[ii].TimeStamp == "" {
			(*taskList)[ii].TimeStamp = time.Now().Format(time.RFC3339Nano)
		}
		tct := taskChannelTuple{
			taskListChannel: taskListChannel,
			task:            &(*taskList)[ii],
		}
		tloc.taskMutex.Lock()
		tloc.taskMap[(*taskList)[ii].id ] = &tct
		tloc.taskMutex.Unlock()

		kdata := (*taskList)[ii].ToHttpKafkaTx()
		jdata, jerr := json.Marshal(kdata)
		if jerr != nil {
			retErr = stockError
			tct.task.Err = &jerr
		}
		if valid, err := tct.task.Validate(); !valid {
			retErr = stockError
			tct.task.Err = &err
		}

		if tct.task.Err != nil {
			go SendDelayedError(tct, tloc.Logger)
		} else {
			tloc.KafkaInstance.Write(tloc.sendTopic, jdata)
		}

	}
	return taskListChannel, retErr
}

// Check on the status of the most recently launched task list.
//
// taskList:  Ptr to a recently launched task list.
// Return:    Task list still running: true/false
//            Error message, if any, associated with the task run.

func (tloc *TRSHTTPRemote) Check(taskList *[]HttpTask) (bool, error) {
	for _, v := range *taskList {
		if v.Ignore == false {
			if v.Request.Response == nil && v.Err == nil {
				return true, nil
			}
		}
	}
	return false, nil
}

// Check the health of the remote HTTP task launch system.
//
// Return: Alive and operational -- true/false
//         Error message associated with non-alive/functional state

func (tloc *TRSHTTPRemote) Alive() (bool, error) {
	if tloc.KafkaInstance.Client == nil {
		return false, errors.New("Kafka client is nil")
	}
	if tloc.taskMap == nil {
		return false, errors.New("taskMap is nil")
	}

	return true, nil
}

// Cancel a currently-running task set.  Note that this won't (yet) kill
// the individual in-flight tasks, but just kills the overall operation.
// Thus, for tasks with no time-out which are hung, it could result in 
// a resource leak.   But this can be used to at least wrestle control
// over a task set.
//
// taskList:  Ptr to a list of HTTP tasks to launch.

func (tloc *TRSHTTPRemote) Cancel(taskList *[]HttpTask) {
	//TODO Mk3 -> send a messaage via Kafka to CANCEL a running task
}

// Close out a task list transaction.  The frees up a small amount of resources
// so it should not be skipped.
//
// taskList:  Ptr to a recently launched task list.

func (tloc *TRSHTTPRemote) Close(taskList *[]HttpTask) {
	for _, v := range *taskList {
		tloc.taskMutex.Lock()
		delete(tloc.taskMap, v.id)
		tloc.taskMutex.Unlock()
	}
}

// Clean up a Remote HTTP task system.
func (tloc *TRSHTTPRemote) Cleanup() {
	//statement order is important! NO CHANGE
	tloc.KafkaInstance.Shutdown()
	tloc.ctxCancelFunc()
	close(tloc.kafkaRspChan)
	//statement order is important! NO CHANGE
}
