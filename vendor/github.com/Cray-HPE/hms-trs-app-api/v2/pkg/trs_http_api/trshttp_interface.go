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

package trs_http_api

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/google/uuid"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/sirupsen/logrus"
	tkafka "github.com/Cray-HPE/hms-trs-kafkalib/v2/pkg/trs-kafkalib"
	"sync"
	"time"
)

// Interface for TRS operations.  There will be 2 sets of interface methods --
// one for local operations (no kafka or TRS workers) and one for worker
// mode (uses Kafka/TRS workers).
//
// Note the lack of Kafka topic specifications in any of the interface funcs.
// This is intentional -- using the local or remote variants of the interface
// shouldn't make the code any different at all.   If using the remote/worker
// variant, the Kafka topics are calculated based on service name, sender id,
// and send/receive.   This is all done under the covers, but will be 
// predictable enough for debugging.
//
// If for whatever reason an application wants a different return topic, it
// can specify it in the source descriptor in CreateTaskArray.  This is
// hacky, but using a different return topic is somewhat bypassing the
// intended use model of this API anyway.

//var TrsAPIContext TrsAPI

type TrsAPI interface {
	Init(serviceName string, logger *logrus.Logger) error
	SetSecurity(params interface{}) error
	CreateTaskList(source *HttpTask, numTasks int) []HttpTask
	Launch(taskList *[]HttpTask) (chan *HttpTask, error)
	Check(taskList *[]HttpTask) (running bool, err error)
	Cancel(taskList *[]HttpTask)
	Close(taskList *[]HttpTask)
	Alive() (ok bool, err error)
	Cleanup()
}

// Local operations

type TRSHTTPLocalSecurity struct {
	CACertBundleData string
	ClientCertData string
	ClientKeyData string
}

type clientPack struct {
	secure *retryablehttp.Client
	insecure *retryablehttp.Client
}

type TRSHTTPLocal struct {
	Logger        *logrus.Logger
	svcName       string
	ctx           context.Context
	ctxCancelFunc context.CancelFunc
	CACertPool    *x509.CertPool
	ClientCert    tls.Certificate
	clientMap     map[RetryPolicy]*clientPack
	clientMutex   sync.Mutex
	taskMap       map[uuid.UUID]*taskChannelTuple
	taskMutex     sync.Mutex
}

// Remote/worker operations

type TRSHTTPRemote struct {
	Logger        *logrus.Logger
	svcName       string
	brokerSpec    string
	ctx           context.Context
	ctxCancelFunc context.CancelFunc
	taskMap       map[uuid.UUID]*taskChannelTuple
	kafkaRspChan  chan *kafka.Message
	KafkaInstance *tkafka.TRSKafka
	sendTopic     string
	receiveTopics []string
	consumerGroup string
	taskMutex     sync.Mutex
}

type taskChannelTuple struct {
	taskListChannel chan *HttpTask
	task            *HttpTask
}

/////////////////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////////////////

// Generic func to create a task array.  Called by the interface methods
// with similar names.
//
// source:  Ptr to source HTTP task descriptor.
// svcName: Service or application name.
// sender:  Sender/k8s UUID name. Can be "".
// nitems:  Number of elements to create in resulting task array.
// Return:  Array of HTTP task descriptors.

func createHTTPTaskArray(source *HttpTask,
	nitems int) []HttpTask {
	sarr := make([]HttpTask, nitems)

	for ii := 0; ii < nitems; ii++ {
		sarr[ii] = *source
		if sarr[ii].Request != nil {
			sarr[ii].Request = source.Request.Clone(context.Background())
		}
		sarr[ii].TimeStamp = time.Now().String()
		sarr[ii].id = uuid.New()
	}

	return sarr
}
