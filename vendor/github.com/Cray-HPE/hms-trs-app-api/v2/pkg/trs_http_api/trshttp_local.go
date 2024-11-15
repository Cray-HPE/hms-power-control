// MIT License
//
// (C) Copyright [2020-2021,2024] Hewlett Packard Enterprise Development LP
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
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	base "github.com/Cray-HPE/hms-base/v2"
	"github.com/google/uuid"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/sirupsen/logrus"
)

const (
	DFLT_RETRY_MAX   = 3	//default max # of retries on failure
	DFLT_BACKOFF_MAX = 5	//default max seconds per retry
)

/////////////////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////////////////
//                   L O C A L  I N T E R F A C E
/////////////////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////////////////

// Initialize a local HTTP task system.
//
// ServiceName: Name of running service/application.
// Return:      Error string if something went wrong.

func (tloc *TRSHTTPLocal) Init(serviceName string, logger *logrus.Logger) error {
	if logger != nil {
		tloc.Logger = logger
	} else {
		tloc.Logger = logrus.New()
	}

	tloc.ctx, tloc.ctxCancelFunc = context.WithCancel(context.Background())

	if tloc.taskMap == nil {
		tloc.taskMutex.Lock()
		tloc.taskMap = make(map[uuid.UUID]*taskChannelTuple)
		tloc.taskMutex.Unlock()
	}
	if tloc.clientMap == nil {
		tloc.clientMutex.Lock()
		tloc.clientMap = make(map[ClientPolicy]*clientPack)
		tloc.clientMutex.Unlock()
	}
	tloc.svcName = serviceName

	tloc.Logger.Tracef("Init() successful")

	return nil
}

// Set up security parameters.  For HTTP-local operations, this is ingesting
// the CA root bundle at the very least, and optionally the client-side
// TLS leaf cert and TLS key.

func (tloc *TRSHTTPLocal) SetSecurity(inParams interface{}) error {
	params := inParams.(TRSHTTPLocalSecurity)

	if (params.CACertBundleData == "") {
		err := fmt.Errorf("CA cert bundle required.")
		tloc.Logger.Errorf("SetSecurity(): %v",err)
		return err
	}

	tloc.CACertPool,_ = x509.SystemCertPool()
	if (tloc.CACertPool == nil) {
		tloc.CACertPool = x509.NewCertPool()
	}
	tloc.CACertPool.AppendCertsFromPEM([]byte(params.CACertBundleData))

	if ((params.ClientCertData != "") && (params.ClientKeyData != "")) {
		var err error
		tloc.ClientCert,err = tls.X509KeyPair([]byte(params.ClientCertData),[]byte(params.ClientKeyData))
		if (err != nil) {
			tloc.Logger.Errorf("SetSecurity(): Error generating client cert: %v",
				err)
			return err
		}
	}

	tloc.Logger.Tracef("SetSecurity() successful")

	return nil
}

// Create an array of task descriptors.  Copy data from the source task
// into each element of the returned array.  Per-task data has to be
// populated separately by the caller.
//
// The message id in each task is populated regardless of the value in
// the source.   It is generated using a pseudo-random value in the upper
// 32 bits, which is the message group id, followed by a monotonically
// increasing value in the lower 32 bits, starting with 0, which functions
// as the message id.
//
// source:   Ptr to a task descriptor populated with relevant data.
// numTasks: Number of elements in the returned array.
// Return:   Array of populated task descriptors.

func (tloc *TRSHTTPLocal) CreateTaskList(source *HttpTask, numTasks int) []HttpTask {
	return createHTTPTaskArray(source, numTasks)
}

// LeveledLogrus implements the LeveledLogger interface in retryablehttp so
// we can control its log levels.  We match TRS's log level as this is what
// TRS's caller wants to see.  Without doing this, retryablehttp spams the
// logs with debug messages.  The code for this comes from the community as
// a recommended workaround for working around the following issue:
// https://github.com/hashicorp/go-retryablehttp/issues/93

type LeveledLogrus struct {
	*logrus.Logger
}

func (l *LeveledLogrus) fields(keysAndValues ...interface{}) map[string]interface{} {
	fields := make(map[string]interface{})

	for i := 0; i < len(keysAndValues) - 1; i += 2 {
		fields[keysAndValues[i].(string)] = keysAndValues[i+1]
	}

	return fields
}

func (l *LeveledLogrus) Error(msg string, keysAndValues ...interface{}) {
	l.WithFields(l.fields(keysAndValues...)).Error(msg)
}
func (l *LeveledLogrus) Info(msg string, keysAndValues ...interface{}) {
	l.WithFields(l.fields(keysAndValues...)).Info(msg)
}
func (l *LeveledLogrus) Debug(msg string, keysAndValues ...interface{}) {
	l.WithFields(l.fields(keysAndValues...)).Debug(msg)
}
func (l *LeveledLogrus) Warn(msg string, keysAndValues ...interface{}) {
	l.WithFields(l.fields(keysAndValues...)).Warn(msg)
}

// Create and configure a new client transport for use with HTTP clients.

func configureClient(client *retryablehttp.Client, task *HttpTask, tloc *TRSHTTPLocal, clientType string) {
	retryPolicy := task.CPolicy.Retry
	httpTxPolicy := task.CPolicy.Tx

	// Configure the httpretryable client retry count
	if (retryPolicy.Retries > 0) {
		client.RetryMax = retryPolicy.Retries
	} else {
		client.RetryMax = DFLT_RETRY_MAX
	}

	// Configure the httpretryable client backoff timeout
	if (retryPolicy.BackoffTimeout > 0) {
		client.RetryWaitMax = retryPolicy.BackoffTimeout
	} else {
		client.RetryWaitMax = DFLT_BACKOFF_MAX * time.Second
	}

	// HTTPClient timeout should be 90% of the task's context timeout
	client.HTTPClient.Timeout = task.Timeout * 9 / 10

	// Configure TLS for the client transport
	var tr *http.Transport
	if clientType == "insecure" {
		tr = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true,},}
	} else {
		tlsConfig := &tls.Config{RootCAs: tloc.CACertPool,}
		tlsConfig.BuildNameToCertificate()
		tr = &http.Transport{TLSClientConfig: tlsConfig,}
	}

	// Configure the client't http transport policy
	if httpTxPolicy.Enabled {
		tr.MaxIdleConns          = httpTxPolicy.MaxIdleConns          // if 0 defaults to 2
		tr.MaxIdleConnsPerHost   = httpTxPolicy.MaxIdleConnsPerHost   // if 0 defaults to 100
		tr.IdleConnTimeout       = httpTxPolicy.IdleConnTimeout       // if 0 defaults to no timeout
		tr.ResponseHeaderTimeout = httpTxPolicy.ResponseHeaderTimeout // if 0 defaults to no timeout
		tr.TLSHandshakeTimeout   = httpTxPolicy.TLSHandshakeTimeout   // if 0 defaults to 10s
		tr.DisableKeepAlives	 = httpTxPolicy.DisableKeepAlives     // if 0 defaults to false
	}

	// Log the configuration we're going to use. Clients are generally long
	// lived so this shouldn't be too spammy. Knowing this information can
	// be pretty critical when debugging issues on site
	tloc.Logger.Errorf("Created %s client with incoming policy %v (to's %s and %s) (ll %v)",
					   clientType, task.CPolicy, task.Timeout, client.HTTPClient.Timeout, tloc.Logger.GetLevel())

	// Write through to the client
	client.HTTPClient.Transport = tr
}

//	Reference:  https://pkg.go.dev/github.com/hashicorp/go-retryablehttp

func ExecuteTask(tloc *TRSHTTPLocal, tct taskChannelTuple) {
	//Find a client or make one!
	var cpack *clientPack
	tloc.clientMutex.Lock()
	if _, ok := tloc.clientMap[tct.task.CPolicy]; !ok {
		log := logrus.New()
		log.SetLevel(tloc.Logger.GetLevel())
		httpLogger := retryablehttp.LeveledLogger(&LeveledLogrus{log})

		cpack = new(clientPack)

		cpack.insecure = retryablehttp.NewClient()
		cpack.insecure.Logger = httpLogger

		configureClient(cpack.insecure, tct.task, tloc, "insecure")

		if (tloc.CACertPool != nil) {
			cpack.secure = retryablehttp.NewClient()
			cpack.secure.Logger = httpLogger

			configureClient(cpack.secure, tct.task, tloc, "secure")

			tloc.Logger.Tracef("Created secure client with policy %v", tct.task.CPolicy)
		}
		tloc.clientMap[tct.task.CPolicy] = cpack
	} else {
		cpack = tloc.clientMap[tct.task.CPolicy]
	}
	tloc.clientMutex.Unlock()

	if ok, err := tct.task.Validate(); !ok {
		tloc.Logger.Errorf("Failed validation of request: %+v, err: %s", tct.task, err)
		tct.task.Err = &err
		tct.taskListChannel <- tct.task
		return
	}

	//setup timeouts and context for request
	tct.task.context, tct.task.contextCancel = context.WithTimeout(tloc.ctx, tct.task.Timeout)

	base.SetHTTPUserAgent(tct.task.Request,tloc.svcName)
	req, err := retryablehttp.FromRequest(tct.task.Request)
	if err != nil {
		tloc.Logger.Errorf("Failed wrapping request with retryablehttp: %v", err)
		tct.task.Err = &err
		tct.taskListChannel <- tct.task
		return
	}

	req = req.WithContext(tct.task.context)

	// Execute the request
	var tmpError error
	if (tct.task.forceInsecure || tloc.CACertPool == nil || cpack.secure == nil) {
		tloc.Logger.Tracef("Using INSECURE client to send request")
		tct.task.Request.Response, tmpError = cpack.insecure.Do(req)
	} else {
		tloc.Logger.Tracef("Using secure client to send request")
		tct.task.Request.Response, tmpError = cpack.secure.Do(req)

		//If the error is a TLS error, fall back to insecure and log it.
		if (tmpError != nil) {
			tloc.Logger.Warnf("TLS request failed, retrying using INSECURE client (TLS failure: '%v')", tmpError)
			tct.task.Request.Response, tmpError = cpack.insecure.Do(req)
		}
	}

	tct.task.Err = &tmpError
	if (*tct.task.Err) != nil {
		tloc.Logger.Tracef("Request failed: %s", (*tct.task.Err).Error())
	}
	if tct.task.Request.Response != nil {
		tloc.Logger.Tracef("Response: %d", tct.task.Request.Response.StatusCode)
	}

	tct.taskListChannel <- tct.task
}

// Launch an array of tasks.  This is non-blocking.  Use Check() to get
// current status of the task launch.
//
// taskList:  Ptr to a list of HTTP tasks to launch.
// Return:    Chan of *HttpTxTask, sized by task list, which caller can
//            use to get notified of each task completion, or safely
//            ignore.  CALLER MUST CLOSE.
//            Error message if something went wrong with the launch.

func (tloc *TRSHTTPLocal) Launch(taskList *[]HttpTask) (chan *HttpTask, error) {
	if len(*taskList) == 0 {
		rchan := make(chan *HttpTask, 1)
		err := fmt.Errorf("Empty task list, nothing to do.")
		return rchan, err
	}

	//Set all time stamps
	taskListChannel := make(chan *HttpTask, len(*taskList))

	for ii := 0; ii < len(*taskList); ii++ {
		if (*taskList)[ii].Ignore == true {
			continue
		}

		//Always set the response to nil; make sure its clean.
		//Add user-agent header.
		if (*taskList)[ii].Request != nil {
			(*taskList)[ii].Request.Response = nil
			base.SetHTTPUserAgent((*taskList)[ii].Request,tloc.svcName)
		}
		//make sure the id is set
		(*taskList)[ii].SetIDIfNotPopulated()

		//make sure the service name is set
		if (*taskList)[ii].ServiceName == "" {
			(*taskList)[ii].ServiceName = tloc.svcName
		}

		//make sure the timestamp is set
		if (*taskList)[ii].TimeStamp == "" {
			(*taskList)[ii].TimeStamp = time.Now().Format(time.RFC3339Nano)
		}

		//Setup the channel stuff
		tct := taskChannelTuple{
			taskListChannel: taskListChannel,
			task:            &(*taskList)[ii],
		}
		tloc.taskMutex.Lock()
		tloc.taskMap[(*taskList)[ii].id ] = &tct
		tloc.taskMutex.Unlock()

		// pass the Tloc (so it can find a client) + the task channel tuple (so it knows what to do);
		// let it execute!
		go ExecuteTask(tloc, tct)
	}

	tloc.Logger.Tracef("Launch() completed")

	return taskListChannel, nil
}

// Check on the status of the most recently launched task list.  This is
// an alternative to waiting on the task-complete chan returned by Launch().
//
// taskList:  Ptr to a recently launched task list.
// Return:    Task list still running: true/false
//            Error message, if any, associated with the task run.

func (tloc *TRSHTTPLocal) Check(taskList *[]HttpTask) (bool, error) {
	for _, v := range *taskList {
		if (v.Ignore == false) {
			if v.Request.Response == nil && v.Err == nil {
				return true, nil
			}
		}
	}
	return false, nil
}

// Check the health of the local HTTP task launch system.
//
// Return: Alive and operational -- true/false
//         Error message associated with non-alive/functional state

func (tloc *TRSHTTPLocal) Alive() (bool, error) {
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
// taskList:  Ptr to a recently launched task list.

func (tloc *TRSHTTPLocal) Cancel(taskList *[]HttpTask) {
	for _, v := range *taskList {
		if (v.Ignore == false) {
			v.contextCancel()
		}
	}
	tloc.Logger.Tracef("Cancel() completed")
}

// Close out a task list transaction.  The frees up a small amount of resources
// so it should not be skipped.
//
// taskList:  Ptr to a recently launched task list.

func (tloc *TRSHTTPLocal) Close(taskList *[]HttpTask) {
	for _, v := range *taskList {
		if (v.Ignore == false) {
			// All tasks must be cancelled to prevent resource leaks.  The
			// caller may have called Cancel() to prematurely cancel the
			// operation, but that's probably not a common thing so we will
			// do it here.  There is no harm in cancelling twice.  We must
			// do this before closing the response body.

			v.contextCancel()

			// The caller should have closed the response body, but we'll also
			// do it here to both prevent resource leaks.  Note that if that
			// was the case, that connection was closed by the above cancel.

			if v.Request.Response != nil && v.Request.Response.Body != nil {
				_, _ = io.Copy(io.Discard, v.Request.Response.Body)
				v.Request.Response.Body.Close()
				v.Request.Response.Body = nil
				tloc.Logger.Tracef("Response body for task %s closed", v.id)
			}
		}
		tloc.taskMutex.Lock()
		delete(tloc.taskMap, v.id)
		tloc.taskMutex.Unlock()

	}
	tloc.Logger.Tracef("Close() completed")
}

// Clean up a local HTTP task system.

func (tloc *TRSHTTPLocal) Cleanup() {
	//Just call the cancel func.
	tloc.ctxCancelFunc()
	//clean up client map?
	for k := range tloc.clientMap {
		//cancel it first
		if (tloc.clientMap[k].insecure != nil) {
			tloc.clientMap[k].insecure.HTTPClient.CloseIdleConnections()
		}
		if (tloc.clientMap[k].secure != nil) {
			tloc.clientMap[k].secure.HTTPClient.CloseIdleConnections()
		}
		//delete it out of the map
		tloc.clientMutex.Lock()
		delete(tloc.clientMap, k)
		tloc.clientMutex.Unlock()
	}

	//clean up task map
	for k := range tloc.taskMap {
		//cancel it first
		tloc.taskMap[k].task.contextCancel()
		//close the channel
		close(tloc.taskMap[k].taskListChannel)
		//delete it out of the map
		tloc.taskMutex.Lock()
		delete(tloc.taskMap, k)
		tloc.taskMutex.Unlock()

	}
	tloc.Logger.Tracef("Cleanup() completed")
	// this really just a big red button to STOP ALL? b/c im not clearing any memory
	// TEST
}
