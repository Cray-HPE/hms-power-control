/*
 * MIT License
 *
 * (C) Copyright [2021-2024] Hewlett Packard Enterprise Development LP
 *
 * Permission is hereby granted, free of charge, to any person obtaining a
 * copy of this software and associated documentation files (the "Software"),
 * to deal in the Software without restriction, including without limitation
 * the rights to use, copy, modify, merge, publish, distribute, sublicense,
 * and/or sell copies of the Software, and to permit persons to whom the
 * Software is furnished to do so, subject to the following conditions:
 * The above copyright notice and this permission notice shall be included
 * in all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
 * THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR
 * OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
 * ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
 * OTHER DEALINGS IN THE SOFTWARE.
 *
 */

package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/Cray-HPE/hms-base"
	"github.com/Cray-HPE/hms-certs/pkg/hms_certs"
	"github.com/Cray-HPE/hms-power-control/internal/api"
	"github.com/Cray-HPE/hms-power-control/internal/credstore"
	"github.com/Cray-HPE/hms-power-control/internal/domain"
	"github.com/Cray-HPE/hms-power-control/internal/hsm"
	"github.com/Cray-HPE/hms-power-control/internal/logger"
	"github.com/Cray-HPE/hms-power-control/internal/storage"
	trsapi "github.com/Cray-HPE/hms-trs-app-api/v2/pkg/trs_http_api"
	"github.com/namsral/flag"
	"github.com/sirupsen/logrus"
)

// Default Port to use
const defaultPORT = "28007"

const defaultSMSServer = "https://api-gw-service-nmn/apis/smd"

// The ETCD database volume usage can grow significantly if services are making
// many transition/power-cap requests in succession. These values can be used
// to mitigate the usage. PCS will keep completed transactions/power-cap
// operations until there are MAX_NUM_COMPLETED or they expire after
// EXPIRE_TIME_MINS at which point PCS will start deleting the oldest entries.
// NOTE: Transactions and power-cap operations are counted separately for
//       MAX_NUM_COMPLETED.
const (
	defaultMaxNumCompleted = 20000 // Maximum number of completed records to keep (default 20k).
	defaultExpireTimeMins = 1440   // Time, in mins, to keep completed records (default 24 hours).
)

const (
	dfltMaxHTTPRetries = 5
	dfltMaxHTTPTimeout = 40
	dfltMaxHTTPBackoff = 8
)

var (
	Running                          = true
	restSrv             *http.Server = nil
	waitGroup           sync.WaitGroup
	rfClient, svcClient *hms_certs.HTTPClientPair
	TLOC_rf, TLOC_svc   trsapi.TrsAPI
	caURI               string
	rfClientLock        *sync.RWMutex = &sync.RWMutex{}
	serviceName         string
	DSP                 storage.StorageProvider
	HSM                 hsm.HSMProvider
	CS                  credstore.CredStoreProvider
	DLOCK               storage.DistributedLockProvider
)

func main() {

	var err error
	logger.Init()
	logger.Log.Error()

	serviceName, err = base.GetServiceInstanceName()
	if err != nil {
		serviceName = "PCS"
		logger.Log.Errorf("Can't get service instance name, using %s", serviceName)
	}

	logger.Log.Info("Service/Instance name: " + serviceName)

	var VaultEnabled bool
	var VaultKeypath string
	var StateManagerServer string
	var hsmlockEnabled bool = true
	var runControl bool = false //noting to run yet!
	var credCacheDuration int = 600 //In seconds. 10 mins?
	var maxNumCompleted int
	var expireTimeMins int

	srv := &http.Server{Addr: defaultPORT}

	///////////////////////////////
	//ENVIRONMENT PARSING
	//////////////////////////////

	flag.StringVar(&StateManagerServer, "sms_server", defaultSMSServer, "SMS Server")

	flag.BoolVar(&runControl, "run_control", runControl, "run control loop; false runs API only") //this was a flag useful for dev work
	flag.BoolVar(&hsmlockEnabled, "hsmlock_enabled", true, "Use HSM Locking")                     // This was a flag useful for dev work
	flag.BoolVar(&VaultEnabled, "vault_enabled", true, "Should vault be used for credentials?")
	flag.StringVar(&VaultKeypath, "vault_keypath", "secret/hms-creds",
		"Keypath for Vault credentials.")
	flag.IntVar(&credCacheDuration, "cred_cache_duration", 600,
		"Duration in seconds to cache vault credentials.")

	flag.IntVar(&maxNumCompleted, "max_num_completed", defaultMaxNumCompleted, "Maximum number of completed records to keep.")
	flag.IntVar(&expireTimeMins, "expire_time_mins", defaultExpireTimeMins, "The time, in mins, to keep completed records.")

	flag.Parse()

	logger.Log.Info("SMS Server: " + StateManagerServer)
	logger.Log.Info("HSM Lock Enabled: ", hsmlockEnabled)
	logger.Log.Info("Vault Enabled: ", VaultEnabled)
	logger.Log.Info("Max Completed Records: ", maxNumCompleted)
	logger.Log.Info("Completed Record Expire Time: ", expireTimeMins)
	logger.Log.SetReportCaller(true)

	///////////////////////////////
	//CONFIGURATION
	//////////////////////////////

	var BaseTRSTask trsapi.HttpTask
	BaseTRSTask.ServiceName = serviceName
	BaseTRSTask.Timeout = 40 * time.Second
	BaseTRSTask.Request, _ = http.NewRequest("GET", "", nil)
	BaseTRSTask.Request.Header.Set("Content-Type", "application/json")
	BaseTRSTask.Request.Header.Add("HMS-Service", BaseTRSTask.ServiceName)

	//INITIALIZE TRS

	trsLogger := logrus.New()
	trsLogger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	trsLogger.SetLevel(logrus.ErrorLevel) //It could be set to logger.Log.GetLevel()
	trsLogger.SetReportCaller(true)

	var envstr string
	envstr = os.Getenv("TRS_IMPLEMENTATION")

	if envstr == "REMOTE" {
		workerSec := &trsapi.TRSHTTPRemote{}
		workerSec.Logger = trsLogger
		workerInsec := &trsapi.TRSHTTPRemote{}
		workerInsec.Logger = trsLogger
		TLOC_rf = workerSec
		TLOC_svc = workerInsec
	} else {
		workerSec := &trsapi.TRSHTTPLocal{}
		workerSec.Logger = trsLogger
		workerInsec := &trsapi.TRSHTTPLocal{}
		workerInsec.Logger = trsLogger
		TLOC_rf = workerSec
		TLOC_svc = workerInsec
	}

	//Set up TRS TLOCs and HTTP clients, all insecure to start with

	envstr = os.Getenv("PCS_CA_URI")
	if envstr != "" {
		caURI = envstr
	}
	//These are for debugging/testing
	envstr = os.Getenv("PCS_CA_PKI_URL")
	if envstr != "" {
		logger.Log.Infof("Using CA PKI URL: '%s'", envstr)
		hms_certs.ConfigParams.VaultCAUrl = envstr
	}
	envstr = os.Getenv("PCS_VAULT_PKI_URL")
	if envstr != "" {
		logger.Log.Infof("Using VAULT PKI URL: '%s'", envstr)
		hms_certs.ConfigParams.VaultPKIUrl = envstr
	}
	envstr = os.Getenv("PCS_VAULT_JWT_FILE")
	if envstr != "" {
		logger.Log.Infof("Using Vault JWT file: '%s'", envstr)
		hms_certs.ConfigParams.VaultJWTFile = envstr
	}
	envstr = os.Getenv("PCS_LOG_INSECURE_FAILOVER")
	if envstr != "" {
		yn, _ := strconv.ParseBool(envstr)
		if yn == false {
			logger.Log.Infof("Not logging Redfish insecure failovers.")
			hms_certs.ConfigParams.LogInsecureFailover = false
		}
	}

	TLOC_rf.Init(serviceName, trsLogger)
	TLOC_svc.Init(serviceName, trsLogger)
	rfClient, _ = hms_certs.CreateRetryableHTTPClientPair("", dfltMaxHTTPTimeout, dfltMaxHTTPRetries, dfltMaxHTTPBackoff)
	svcClient, _ = hms_certs.CreateRetryableHTTPClientPair("", dfltMaxHTTPTimeout, dfltMaxHTTPRetries, dfltMaxHTTPBackoff)

	//STORAGE/DISTLOCK CONFIGURATION
	envstr = os.Getenv("STORAGE")
	if envstr == "" || envstr == "MEMORY" {
		tmpStorageImplementation := &storage.MEMStorage{
			Logger: logger.Log,
		}
		DSP = tmpStorageImplementation
		logger.Log.Info("Storage Provider: In Memory")
		tmpDistLockImplementation := &storage.MEMLockProvider{}
		DLOCK = tmpDistLockImplementation
		logger.Log.Info("Distributed Lock Provider: In Memory")
	} else if envstr == "ETCD" {
		tmpStorageImplementation := &storage.ETCDStorage{
			Logger: logger.Log,
		}
		DSP = tmpStorageImplementation
		logger.Log.Info("Storage Provider: ETCD")
		tmpDistLockImplementation := &storage.ETCDLockProvider{}
		DLOCK = tmpDistLockImplementation
		logger.Log.Info("Distributed Lock Provider: ETCD")
	}
	DSP.Init(logger.Log)
	DLOCK.Init(logger.Log)
	//TODO: there should be a Ping() to insure dist lock mechanism is alive

	//Hardware State Manager CONFIGURATION
	HSM = &hsm.HSMv2{}
	hsmGlob := hsm.HSM_GLOBALS{
		SvcName: serviceName,
		Logger: logger.Log,
		Running: &Running,
		LockEnabled: hsmlockEnabled,
		SMUrl: StateManagerServer,
		SVCHttpClient: svcClient,
	}
	HSM.Init(&hsmGlob)
	//TODO: there should be a Ping() to insure HSM is alive

	//Vault CONFIGURATION
	tmpCS := &credstore.VAULTv0{}

	CS = tmpCS
	if VaultEnabled {
		var credStoreGlob credstore.CREDSTORE_GLOBALS
		credStoreGlob.NewGlobals(logger.Log, &Running, credCacheDuration, VaultKeypath)
		CS.Init(&credStoreGlob)
	}

	//DOMAIN CONFIGURATION
	var domainGlobals domain.DOMAIN_GLOBALS
	domainGlobals.NewGlobals(&BaseTRSTask, &TLOC_rf, &TLOC_svc, rfClient, svcClient,
	                         rfClientLock, &Running, &DSP, &HSM, VaultEnabled,
	                         &CS, &DLOCK, maxNumCompleted, expireTimeMins)

	//Wait for vault PKI to respond for CA bundle.  Once this happens, re-do
	//the globals.  This goroutine will run forever checking if the CA trust
	//bundle has changed -- if it has, it will reload it and re-do the globals.

	//Set a flag "CA not ready" that the /liveness and /readiness APIs will
	//use to signify that PCS is not ready based on the transport readiness.

	go func() {
		if caURI != "" {
			var err error
			var caChain string
			var prevCaChain string
			RFTransportReady := false

			tdelay := time.Duration(0)
			for {
				time.Sleep(tdelay)
				tdelay = 3 * time.Second

				caChain, err = hms_certs.FetchCAChain(caURI)
				if err != nil {
					logger.Log.Errorf("Error fetching CA chain from Vault PKI: %v, retrying...",
						err)
					continue
				} else {
					logger.Log.Infof("CA trust chain loaded.")
				}

				//If chain hasn't changed, do nothing, expand retry time.

				if caChain == prevCaChain {
					tdelay = 10 * time.Second
					continue
				}

				//CA chain accessible.  Re-do the verified transports

				logger.Log.Infof("CA trust chain has changed, re-doing Redfish HTTP transports.")
				rfClient, err = hms_certs.CreateRetryableHTTPClientPair(caURI, dfltMaxHTTPTimeout, dfltMaxHTTPRetries, dfltMaxHTTPBackoff)
				if err != nil {
					logger.Log.Errorf("Error creating TLS-verified transport: %v, retrying...",
						err)
					continue
				}
				logger.Log.Infof("Locking RF operations...")
				rfClientLock.Lock() //waits for all RW locks to release
				tchain := hms_certs.NewlineToTuple(caChain)
				secInfo := trsapi.TRSHTTPLocalSecurity{CACertBundleData: tchain}
				err = TLOC_rf.SetSecurity(secInfo)
				if err != nil {
					logger.Log.Errorf("Error setting TLOC security info: %v, retrying...",
						err)
					rfClientLock.Unlock()
					continue
				} else {
					logger.Log.Info("TRS CA security updated.")
				}
				prevCaChain = caChain

				//update RF tloc and rfclient to the global areas! //TODO im not sure what part of this code is still needed; im guessing part of it at least!
				domainGlobals.RFTloc = &TLOC_rf
				domainGlobals.RFHttpClient = rfClient
				//hsmGlob.RFTloc = &TLOC_rf
				//hsmGlob.RFHttpClient = rfClient
				//HSM.Init(&hsmGlob)
				rfClientLock.Unlock()
				RFTransportReady = true
				domainGlobals.RFTransportReady = &RFTransportReady
			}
		}
	}()

	///////////////////////////////
	//INITIALIZATION
	//////////////////////////////
	domain.Init(&domainGlobals)

	dlockTimeout := 60
	pwrSampleInterval := 30
	statusHttpTimeout := 30
	statusHttpRetries := 3
	envstr = os.Getenv("PCS_POWER_SAMPLE_INTERVAL")
	if (envstr != "") {
		tps,err := strconv.Atoi(envstr)
		if (err != nil) {
			logger.Log.Errorf("Invalid value of PCS_POWER_SAMPLE_INTERVAL, defaulting to %d",
				pwrSampleInterval)
		} else {
			pwrSampleInterval = tps
		}
	}
	envstr = os.Getenv("PCS_DISTLOCK_TIMEOUT")
	if (envstr != "") {
		tps,err := strconv.Atoi(envstr)
		if (err != nil) {
			logger.Log.Errorf("Invalid value of PCS_DISTLOCK_TIMEOUT, defaulting to %d",
				dlockTimeout)
		} else {
			dlockTimeout = tps
		}
	}
	envstr = os.Getenv("PCS_STATUS_HTTP_TIMEOUT")
	if (envstr != "") {
		tps,err := strconv.Atoi(envstr)
		if (err != nil) {
			logger.Log.Errorf("Invalid value of PCS_STATUS_HTTP_TIMEOUT, defaulting to %d",
				statusHttpTimeout)
		} else {
			statusHttpTimeout = tps
		}
	}
	envstr = os.Getenv("PCS_STATUS_HTTP_RETRIES")
	if (envstr != "") {
		tps,err := strconv.Atoi(envstr)
		if (err != nil) {
			logger.Log.Errorf("Invalid value of PCS_STATUS_HTTP_RETRIES, defaulting to %d",
				statusHttpRetries)
		} else {
			statusHttpRetries = tps
		}
	}

	domain.PowerStatusMonitorInit(&domainGlobals,
		(time.Duration(dlockTimeout)*time.Second),
		logger.Log,(time.Duration(pwrSampleInterval)*time.Second), statusHttpTimeout, statusHttpRetries)

	domain.StartRecordsReaper()

	///////////////////////////////
	//SIGNAL HANDLING -- //TODO does this need to move up ^ so it happens sooner?
	//////////////////////////////

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	idleConnsClosed := make(chan struct{})
	go func() {
		<-c
		Running = false

		//TODO; cannot Cancel the context on retryablehttp; because I havent set them up!
		//cancel()

		// Gracefully shutdown the HTTP server.
		if err := srv.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			logger.Log.Infof("HTTP server Shutdown: %v", err)
		}

		ctx := context.Background()
		if restSrv != nil {
			if err := restSrv.Shutdown(ctx); err != nil {
				logger.Log.Panic("ERROR: Unable to stop REST collection server!")
			}
		}

		close(idleConnsClosed)
	}()


	///////////////////////
	// START
	///////////////////////

	//Master Control

	if runControl {
		logger.Log.Info("Starting control loop")
		//Go start control loop!
	} else {
		logger.Log.Info("NOT starting control loop")
	}
	//Rest Server
	waitGroup.Add(1)
	doRest(defaultPORT)

	//////////////////////
	// WAIT FOR GOD
	/////////////////////

	waitGroup.Wait()
	logger.Log.Info("HTTP server shutdown, waiting for idle connection to close...")
	<-idleConnsClosed
	logger.Log.Info("Done. Exiting.")
}

func doRest(serverPort string) {

	logger.Log.Info("**RUNNING -- Listening on " + defaultPORT)

	srv := &http.Server{Addr: ":" + serverPort}
	router := api.NewRouter()

	http.Handle("/", router)

	go func() {
		defer waitGroup.Done()
		if err := srv.ListenAndServe(); err != nil {
			// Cannot panic because this is probably just a graceful shutdown.
			logger.Log.Error(err)
			logger.Log.Info("REST collection server shutdown.")
		}
	}()

	logger.Log.Info("REST collection server started on port " + serverPort)
	restSrv = srv
}
