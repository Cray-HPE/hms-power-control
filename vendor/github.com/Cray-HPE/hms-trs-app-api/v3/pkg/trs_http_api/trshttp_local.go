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
	"runtime"
	"sync"
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
	//wrapperLogger = tloc.Logger	// only uncomment if debugging wrapper issues

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

// leveledLogrus implements the LeveledLogger interface in retryablehttp so
// we can control its log levels.  We match TRS's log level as this is what
// TRS's caller wants to see.  The code for this comes from the community as
// a recommended work around for the following issue:
//
//      https://github.com/hashicorp/go-retryablehttp/issues/93
//
// In the final version of the commit that added this capability, the
// decision was made NOT to pass our leveledLogrus down to retryablehttp.
// With the prior implementation, TRS was passing down a non-leveled
// logger set at Error level.  With the leveled logger set at Error level
// it is actually much more chatty and logs every standard http request
// that fails for whatever reason.  While this is helpful, and useful,
// it produces much more log data than previously.  To prevent any
// topential log volume issues, the prior mechanism will be kept in place.
// The code needed for leveled logging will remain in place though in the
// event a new version of retryablehttp becomes less chatty at the Error
// level.

type leveledLogrus struct {
	*logrus.Logger
}

func (l *leveledLogrus) fields(keysAndValues ...interface{}) map[string]interface{} {
	fields := make(map[string]interface{})

	for i := 0; i < len(keysAndValues) - 1; i += 2 {
		fields[keysAndValues[i].(string)] = keysAndValues[i+1]
	}

	return fields
}

func (l *leveledLogrus) Error(msg string, keysAndValues ...interface{}) {
	l.WithFields(l.fields(keysAndValues...)).Error(msg)
}
func (l *leveledLogrus) Warn(msg string, keysAndValues ...interface{}) {
	l.WithFields(l.fields(keysAndValues...)).Warn(msg)
}
func (l *leveledLogrus) Info(msg string, keysAndValues ...interface{}) {
	l.WithFields(l.fields(keysAndValues...)).Info(msg)
}
func (l *leveledLogrus) Debug(msg string, keysAndValues ...interface{}) {
	l.WithFields(l.fields(keysAndValues...)).Debug(msg)
}

// The retryablehttp module closes idle connections in an overly aggressive
// manner.  If a single request experiences a timeout, all idle connections
// are closed.  If a single requests exceeds all retries, all idle
// connections are closed.  The following RoundTrip wrapper helps us
// wrap various retryablehttp and http interfaces to prevent this.

type trsRoundTripper struct {
	transport                             *http.Transport
	closeIdleConnectionsFn                func()
	skipCloseCount                        uint64
	skipCloseMutex                        sync.Mutex
	timeLastClosedOrReachedZeroCloseCount time.Time
}

// Our RoundTripper(). Just call RoundTrip interface at next level down.
func (c *trsRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return c.transport.RoundTrip(req)
}

// Our wrapper around the standard http.Client's CloseIdleConnections()
// We only call the interface at the next level down if skipCloseCount
// counter is zero. This counter is decremented by our CheckRetry
// wrapper (further below) if it detects a http timeout, context
// timeout, or retry limit exceeded for a request.
//
// There may be a hole in the logic whereby a request sets the skip
// counter but is then killed for whatever reason before it can call into
// this wrapper. This would leave the counter at non-zero forever.  It
// would be very unlikely to happen, but logically possible. We guard
// against this by resetting the counter to zero if its been 2 hours since:
//
//	* The last call to c.CloseIdleConnectionsFn()
//	* The last time the c.skipCloseCount reached zero
//
// Prior to this change, the TRS module would ALWAYS closing all idle
// connections after every single http timeout, context timeout, or retry
// count exceeded condition.  So, if we close out all the connections
// occasionally after a two hour period, not a big deal.
//
// WARNING!  The Go runtime behavior surrounding connections has changed in
//           more recent versions of Go.  Prior to version 1.23, if any
//           request experiences a timeout, the Go runtime closes ALL idle
//           connections.  There is nothing we can do about this in TRS,
//           other than use a newer version of Go that doesn't exhibit this
//           behavior.  Issue is documented here:
//
//           * https://github.com/golang/go/issues/59017
//           * https://github.com/golang/go/commit/334ce510046ad30b1be466634cf313aad3040892
//
//           Will attempt to jump to Go 1.23 with this commit.

//var wrapperLogger *logrus.Logger	// only uncomment if debugging wrapper issues

func (c *trsRoundTripper) CloseIdleConnections() {
	// Skip closing idle connections if counter > 0

	c.skipCloseMutex.Lock()
	if c.skipCloseCount > 0 {
		c.skipCloseCount--

		//wrapperLogger.Errorf("CloseIdleConnections: skipCloseCount now %v", c.skipCloseCount)

		if c.skipCloseCount == 0 {
			// Mark the time the counter last reached zero
			c.timeLastClosedOrReachedZeroCloseCount = time.Now()
		}

		if time.Since(c.timeLastClosedOrReachedZeroCloseCount) > (2 * time.Hour) {
			// If its been two hours since we last closed idle connections
			// or since the counter last reached zero, reset the counter to
			// zero and proceed to close idle connections

			c.skipCloseCount = 0

			//wrapperLogger.Errorf("CloseIdleConnections: skipCloseCount reset due to expired time")

			// Time will be marked below when we close idle connections
		} else {
			c.skipCloseMutex.Unlock()

			//wrapperLogger.Errorf("CloseIdleConnections: returning without closing idle connections")

			return
		}
	}

	// Continue holding mutex until skipCloseCountResetTime is updated

	if c.closeIdleConnectionsFn == nil {
		// Nothing to do so release mutex
		c.skipCloseMutex.Unlock()

		//wrapperLogger.Errorf("CloseIdleConnections: no lower closeIdleConnectionsFn() to call")
	} else {
		// Mark the time of this call to close connections
		c.timeLastClosedOrReachedZeroCloseCount = time.Now()

		c.skipCloseMutex.Unlock()

		//wrapperLogger.Errorf("CloseIdleConnections: calling lower closeIdleConnectionsFn()")

		// Call next level down
		c.closeIdleConnectionsFn()
	}
}

// Custom request wrapper that includes a retry counter that we'll use to
// determine whether or not to close idle connections
type trsWrappedReq struct {
	orig        *http.Request // request we're wrapping
	retryMax    int           // retry max from request
	retryCount  int           // retry count for this request
}

// drainAndCloseBody is a simple helper function reused in many places

func drainAndCloseBody(resp *http.Response) {
    if resp != nil && resp.Body != nil {
        _, _ = io.Copy(io.Discard, resp.Body) // Drain the body
        resp.Body.Close()                     // Close the body
        resp.Body = nil                       // Prevent double-handling
    }
}

// Our wrapper around retryablehttp's CheckRetry().  if we detect an http
// timeout, context timeout, or a retry limit exceeded for a request, then
// we decrement the skipCloseCount counter so that the next time our
// CloseIdleConnections() wrapper is called, it skips calling the lower
// level system version that actually closes idle connections.

type retryKey string	// avoids compiler warning
var trsRetryCountKey retryKey = "trsRetryCount"

func (c *trsRoundTripper) trsCheckRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	//wrapperLogger.Errorf("trsCheckRetry: err=%v errType=%T", err, err)

	// Skip a retry for this request if it hit one of these specific timeouts

	if err != nil {
		c.skipCloseMutex.Lock()

		// Skip retries for HTTPClient.Timeout
		if err.Error() == "net/http: request canceled" {
			c.skipCloseCount++

			c.skipCloseMutex.Unlock()

			//wrapperLogger.Errorf("trsCheckRetry: skipCloseCount now %v (http timeout)", c.skipCloseCount)

			// retryablehttp will close any response bodies on retry skip + error

			return false, err	// skip it
		}

		// Context timeout set by TRS.  No request should retry.
		if errors.Is(err, context.DeadlineExceeded) {
			c.skipCloseCount++

			c.skipCloseMutex.Unlock()

			//wrapperLogger.Errorf("trsCheckRetry: skipCloseCount now %v (ctx timeout)", c.skipCloseCount)

			// retryablehttp will close any response bodies on retry skip + error

			return false, err	// skip it
		}

		// Lower level HTTPClient.Timeout triggered timeouts
		//
		// Unsure if this is wise so I left it commented out.  If these
		// happen they don't happen very much so closing all idle
		// connections when they do happen is not a big deal.
		//
		// if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		//     c.skipCloseCount++
		//
		//     c.skipCloseMutex.Unlock()
		//
		//     wrapperLogger.Errorf("trsCheckRetry: skipCloseCount now %v (ctx timeout)", c.skipCloseCount)
		//
		//     // retryablehttp will close any response bodies on retry skip + error
		//
		//     return false, err  // skip it
		// }

		c.skipCloseMutex.Unlock()

		//wrapperLogger.Errorf("trsCheckRetry: deferring to DefaultRetryPolicy() for this error")
	}

	// If none of the above, delegate retry check to retryablehttp
	shouldRetry, defaultRPErr := retryablehttp.DefaultRetryPolicy(ctx, resp, err)

	//wrapperLogger.Errorf("trsCheckRetry: DefaultRetryPolicy: shouldRetry=%v", shouldRetry)

	// Determine if we should override DefaultRetryPolicy()'s opinion
	if shouldRetry {
		// This is our own personal copy of the retry counter for this
		// request. Let's increment it then compare to the retry limit

		trsWR := ctx.Value(trsRetryCountKey).(*trsWrappedReq)
		trsWR.retryCount++

		//wrapperLogger.Errorf("trsCheckRetry: retryCount now %v", trsWR.retryCount)

		// If the retry limit was reached we do not want to close all idle
		// connections unnecessarily so imcrement skipCloseCount counter so
		// that our CloseIdleConnections() wrapper skips the next one

		if trsWR.retryCount > trsWR.retryMax {
			// If no error present, let's give the caller the underlying
			// reason why retries were exhausted, if we can determine it
			if err == nil {
				if resp != nil {
					err = fmt.Errorf("retries exhausted: last attempt received status %d (%s)",
									 resp.StatusCode, http.StatusText(resp.StatusCode))
				} else {
					err = fmt.Errorf("retries exhausted")
				}
			}

			// Skip an idle connection close
			c.skipCloseMutex.Lock()
			c.skipCloseCount++
			c.skipCloseMutex.Unlock()

			//wrapperLogger.Errorf("trsCheckRetry: skipCloseCount now %v and err is %v", c.skipCloseCount, err)

			// retryablehttp will close any response bodies on retry skip + error

			return false, err
		}
	}

	return shouldRetry, defaultRPErr
}

// Create and configure a new client transport for use with HTTP clients.
func createClient(task *HttpTask, tloc *TRSHTTPLocal, clientType string) (client *retryablehttp.Client) {
	// Configure the base transport

	tr := &http.Transport{}

	if clientType == "insecure" {
		tr.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	} else {     // secure
		tr.TLSClientConfig = &tls.Config{
			RootCAs: tloc.CACertPool,
		}
		tr.TLSClientConfig.BuildNameToCertificate()
	}

	// Configure base transport policies if requested
	httpTxPolicy := task.CPolicy.Tx
	if httpTxPolicy.Enabled {
		tr.MaxIdleConns          = httpTxPolicy.MaxIdleConns          // if 0 defaults to 2
		tr.MaxIdleConnsPerHost   = httpTxPolicy.MaxIdleConnsPerHost   // if 0 defaults to 100
		tr.IdleConnTimeout       = httpTxPolicy.IdleConnTimeout       // if 0 defaults to no timeout
		tr.ResponseHeaderTimeout = httpTxPolicy.ResponseHeaderTimeout // if 0 defaults to no timeout
		tr.TLSHandshakeTimeout   = httpTxPolicy.TLSHandshakeTimeout   // if 0 defaults to 10s
		tr.DisableKeepAlives	 = httpTxPolicy.DisableKeepAlives     // if 0 defaults to false
	}

	// Wrap base transport with retryablehttp
	retryabletr := &trsRoundTripper{
		transport:                             tr,
		closeIdleConnectionsFn:                tr.CloseIdleConnections,
		timeLastClosedOrReachedZeroCloseCount: time.Now(),
	}

	// Create the httpretryable client and start configuring it
	client = retryablehttp.NewClient()

	client.HTTPClient.Transport = retryabletr

	// We could set a global http timeout for all users of the client but
	// that's a bit inflexible.  Let's keep it at the default (unlimited)
	// and use the user provided context timeout to limit the request.
	// The context timeout they provide could be different from caller to
	// caller as well.
	//
	//client.HTTPClient.Timeout   = task.Timeout * 9 / 10

	// Wrap httpretryable's DefaultRetryPolicy() so we can override
	client.CheckRetry = retryabletr.trsCheckRetry

	// Configure the httpretryable client retry count
	if (task.CPolicy.Retry.Retries >= 0) {
		client.RetryMax = task.CPolicy.Retry.Retries
	} else {
		client.RetryMax = DFLT_RETRY_MAX
	}

	// Configure the httpretryable client backoff timeout
	if (task.CPolicy.Retry.BackoffTimeout > 0) {
		client.RetryWaitMax = task.CPolicy.Retry.BackoffTimeout
	} else {
		client.RetryWaitMax = DFLT_BACKOFF_MAX * time.Second
	}

	// Log this client's requested configuration and actual configuration

	tloc.Logger.Errorf("Created %s TRS client", clientType)
	tloc.Logger.Errorf("    CPolicy request:           %v", task.CPolicy)
	tloc.Logger.Errorf("    RetryMax:                  %v", client.RetryMax)
	tloc.Logger.Errorf("    RetryWaitMax:              %v", client.RetryWaitMax)
	tloc.Logger.Errorf("    task.Timeout:              %v", task.Timeout)
	tloc.Logger.Errorf("    HTTPClient.Timeout:        %v", client.HTTPClient.Timeout)

	if (httpTxPolicy.Enabled) {
		tloc.Logger.Errorf("    tx.MaxIdleConns:           %v", tr.MaxIdleConns)
		tloc.Logger.Errorf("    tx.MaxIdleConnsPerHost:    %v", tr.MaxIdleConnsPerHost)
		tloc.Logger.Errorf("    tx.IdleConnTimeout:        %v", tr.IdleConnTimeout)
		tloc.Logger.Errorf("    tx.ResponseHeaderTimeout:  %v", tr.ResponseHeaderTimeout)
		tloc.Logger.Errorf("    tx.TLSHandshakeTimeout:    %v", tr.TLSHandshakeTimeout)
		tloc.Logger.Errorf("    tx.DisableKeepAlives:      %v", tr.DisableKeepAlives)
	}

	tloc.Logger.Errorf("    Client pair count:         %v", len(tloc.clientMap) + 1)
	//tloc.Logger.Errorf("    Log level:                 %v", tloc.Logger.GetLevel())
	tloc.Logger.Errorf("    Log level:                 %v", logrus.ErrorLevel)
	tloc.Logger.Errorf("    Go version:                %v", runtime.Version())

	return client
}

//	Reference:  https://pkg.go.dev/github.com/hashicorp/go-retryablehttp

func ExecuteTask(tloc *TRSHTTPLocal, tct taskChannelTuple) {

	// Find a client to use, or make a new one!

	var cpack *clientPack
	tloc.clientMutex.Lock()
	if _, ok := tloc.clientMap[tct.task.CPolicy]; !ok {
		cpack = new(clientPack)

		cpack.insecure = createClient(tct.task, tloc, "insecure")

		// Do not use leveled logging for now.  See explanation further
		// up in the source code.  Instead, use standard logger set at
		// error level to avoid excessive logging
		//
		//httpLogger := logrus.New()
		//httpLogger.SetLevel(tloc.Logger.GetLevel())
		//retryablehttpLogger := retryablehttp.LeveledLogger(&leveledLogrus{httpLogger})
		//cpack.insecure.Logger = retryablehttpLogger

		httpLogger := logrus.New()
		httpLogger.SetLevel(logrus.ErrorLevel)
		cpack.insecure.Logger = httpLogger

		if (tloc.CACertPool != nil) {
			cpack.secure = createClient(tct.task, tloc, "secure")

			//cpack.secure.Logger = retryablehttpLogger
			cpack.secure.Logger = httpLogger
		}

		tloc.clientMap[tct.task.CPolicy] = cpack
	} else {
		cpack = tloc.clientMap[tct.task.CPolicy]
	}
	tloc.clientMutex.Unlock()

	// Found a client to use (or created a new one), now set up a request

	// First validate our task
	if ok, err := tct.task.Validate(); !ok {
		tloc.Logger.Errorf("Failed validation of request: %+v, err: %s", tct.task, err)
		tct.task.Err = &err
		tct.taskListChannel <- tct.task
		return
	}

	// Add user agent header to the request
	base.SetHTTPUserAgent(tct.task.Request,tloc.svcName)

	// Wrap the request so trsCheckRetry() can keep its own retry count
	// We'll attach this to the context further below so that it has access
	// to it
	trsWR := &trsWrappedReq{
		orig:       tct.task.Request,        // Core request
		retryCount: 0,                       // Counter for CLIC()
		retryMax:   cpack.insecure.RetryMax, // CLIC() will need access to this
		                                     // same for both secure & insecure
	}

	// Create child context with timeout and our own retry counter

	baseCtx, cancel := context.WithTimeout(tloc.ctx, tct.task.Timeout)
	ctxWithValue := context.WithValue(baseCtx, trsRetryCountKey, trsWR)

	tct.task.context = ctxWithValue
	tct.task.contextCancel = cancel

	// Create a retryablehttp request using the caller's request
	req, err := retryablehttp.FromRequest(tct.task.Request)
	if err != nil {
		tloc.Logger.Errorf("Failed wrapping request with retryablehttp: %v", err)
		tct.task.Err = &err
		tct.taskListChannel <- tct.task
		return
	}

	// Link retryablehttp's request context to the task's context
	req.Request = req.Request.WithContext(tct.task.context)

	// Execute the request
	var tmpError error
	if (tct.task.forceInsecure || tloc.CACertPool == nil || cpack.secure == nil) {
		tloc.Logger.Tracef("Using INSECURE client to send request")
		tct.task.Request.Response, tmpError = cpack.insecure.Do(req)
	} else {
		tloc.Logger.Tracef("Using secure client to send request")
		tct.task.Request.Response, tmpError = cpack.secure.Do(req)

		// Fall back to insecure only if the enclosing context was not
		// cancelled or timed out.
		if tmpError != nil && tct.task.context.Err() == nil {
			// But first make sure we drain/close the body of the failed
			// response, if there was one
			drainAndCloseBody(tct.task.Request.Response)

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
	} else {
		tloc.Logger.Tracef("No response received")
	}

	// TODO: Consider cancelling the context for this task here instead of
	// leaving it up to the caller - Would have to read and close response
	// here as well

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
	if taskList != nil {
		for _, v := range *taskList {
			if (v.Ignore == false) {
				if v.Request.Response == nil && v.Err == nil {
					return true, nil
				}
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
	if taskList != nil {
		for _, v := range *taskList {
			if (v.Ignore == false) {
				v.contextCancel()
			}
		}
	}
	tloc.Logger.Tracef("Cancel() completed")
}

// Close out a task list transaction.  The frees up a small amount of resources
// so it should not be skipped.
//
// taskList:  Ptr to a recently launched task list.

func (tloc *TRSHTTPLocal) Close(taskList *[]HttpTask) {
	if taskList != nil {
		for _, v := range *taskList {
			if (v.Ignore == false) {
				// The caller should have closed the response body, but we'll also
				// do it here to prevent resource leaks.  Note that if that was
				// the case, that connection was closed by the above cancel.

				drainAndCloseBody(v.Request.Response)

				// All tasks must be cancelled to prevent resource leaks.  The
				// caller may have called Cancel() to prematurely cancel the
				// operation, but that's probably not a common thing so we will
				// do it here.  There is no harm in cancelling twice.  We must
				// do this before closing the response body.
				//
				// TODO: We may also want to consider having each go routine
				// call cancel above in ExecuteTask() for themselves.  Doing it
				// like that might be less error prone.

				v.contextCancel()
			}
			tloc.taskMutex.Lock()
			delete(tloc.taskMap, v.id)
			tloc.taskMutex.Unlock()

		}
	}

	tloc.Logger.Tracef("Close() completed")
}

// Clean up a local HTTP task system.

func (tloc *TRSHTTPLocal) Cleanup() {
	//Just call the cancel func.
	tloc.ctxCancelFunc()

	//clean up client map?
	if tloc.clientMap != nil {
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
	}

	//clean up task map
	if tloc.taskMap != nil {
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
	}
	tloc.Logger.Tracef("Cleanup() completed")
	// this really just a big red button to STOP ALL? b/c im not clearing any memory
}
