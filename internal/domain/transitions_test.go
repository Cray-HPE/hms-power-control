/*
 * MIT License
 *
 * (C) Copyright [2022-2024] Hewlett Packard Enterprise Development LP
 *
 * Permission is hereby granted, free of charge, to any person obtaining a
 * copy of this software and associated documentation files (the "Software"),
 * to deal in the Software without restriction, including without limitation
 * the rights to use, copy, modify, merge, publish, distribute, sublicense,
 * and/or sell copies of the Software, and to permit persons to whom the
 * Software is furnished to do so, subject to the following conditions:
 *
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
 */

package domain

import (
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Cray-HPE/hms-certs/pkg/hms_certs"
	"github.com/Cray-HPE/hms-power-control/internal/credstore"
	"github.com/Cray-HPE/hms-power-control/internal/hsm"
	"github.com/Cray-HPE/hms-power-control/internal/logger"
	"github.com/Cray-HPE/hms-power-control/internal/model"
	"github.com/Cray-HPE/hms-power-control/internal/storage"
	base "github.com/Cray-HPE/hms-base"
	trsapi "github.com/Cray-HPE/hms-trs-app-api/v3/pkg/trs_http_api"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

type Transitions_TS struct {
    suite.Suite
    hsmURL string
}

// Sets up everything needed for running power capping domain functions such as
// httptest servers, HSM/storage/trs packages, etc.
func (ts *Transitions_TS) SetupSuite() {
	var (
		Running bool = true
		svcClient *hms_certs.HTTPClientPair
		TLOC_rf, TLOC_svc trsapi.TrsAPI
		rfClientLock *sync.RWMutex = &sync.RWMutex{}
		serviceName string = "PCS-domain-transitions-test"
		DSP storage.StorageProvider
		DLOCK storage.DistributedLockProvider
		HSM hsm.HSMProvider
		// StateManagerServer string
		credStoreGlob credstore.CREDSTORE_GLOBALS
		CS credstore.CredStoreProvider
		BaseTRSTask trsapi.HttpTask
		domainGlobals DOMAIN_GLOBALS
	)

	logger.Init()
	logger.Log.Error()

	enableVault := false
	if os.Getenv("VAULT_ENABLED") != "" {
		enableVault = true
	}

	if enableVault {
		tcs := &credstore.VAULTv0{}
		CS = tcs
		vkp := os.Getenv("VAULT_KEYPATH")
		if vkp == "" {
			vkp = "hms-creds"
		}
		credStoreGlob.NewGlobals(logger.Log, &Running, 3600, vkp)
		CS.Init(&credStoreGlob)
	}

	svcClient, _ = hms_certs.CreateRetryableHTTPClientPair("", 10, 10, 4)

	BaseTRSTask.ServiceName = serviceName
	BaseTRSTask.Timeout = 40 * time.Second
	BaseTRSTask.Request, _ = http.NewRequest("GET", "", nil)
	BaseTRSTask.Request.Header.Set("Content-Type", "application/json")
	BaseTRSTask.Request.Header.Add("HMS-Service", BaseTRSTask.ServiceName)

	workerSec := &trsapi.TRSHTTPLocal{}
	workerSec.Logger = logger.Log
	workerInsec := &trsapi.TRSHTTPLocal{}
	workerInsec.Logger = logger.Log
	TLOC_rf = workerSec
	TLOC_svc = workerInsec
	TLOC_rf.Init(serviceName, logger.Log)
	TLOC_svc.Init(serviceName, logger.Log)

	tmpStorageImplementation := &storage.ETCDStorage{
		Logger: logger.Log,
	}
	DSP = tmpStorageImplementation
	DSP.Init(logger.Log)

	tmpStorageLockImplementation := &storage.ETCDLockProvider{
		Logger: logger.Log,
	}
	DLOCK = tmpStorageLockImplementation
	DLOCK.Init(logger.Log)

	HSM = &hsm.HSMv2{}

	hsmGlob := hsm.HSM_GLOBALS{
		SvcName: serviceName,
		Logger: logger.Log,
		Running: &Running,
		LockEnabled: true,
		SMUrl: os.Getenv("SMS_SERVER"),
		SVCHttpClient: svcClient,
	}
	HSM.Init(&hsmGlob)
	ts.hsmURL = hsmGlob.SMUrl

	domainGlobals.NewGlobals(&BaseTRSTask, &TLOC_rf, &TLOC_svc, nil, svcClient,
	                         rfClientLock, &Running, &DSP, &HSM, enableVault, &CS,
	                         &DLOCK, 20000, 1440, "transitions_test-pod")
	Init(&domainGlobals)

	// Calling PowerStatusMonitorInit() is required to initialize the
	// globals it uses but we don't want the hardware monitor to run
	// so we'll just immediatly stop it.
	glogger = logger.Log
	hsmHandle = domainGlobals.HSM
	kvStore = domainGlobals.DSP
	ccStore = domainGlobals.CS
	distLocker = domainGlobals.DistLock
	tloc = domainGlobals.RFTloc
	distLockMaxTime = 600 * time.Second
	pmSampleInterval = 5 * time.Second
	serviceRunning = domainGlobals.Running
	vaultEnabled = domainGlobals.VaultEnabled

	// Clear power-status internal state
	hwStateMap = make(map[string]*componentPowerInfo)
}

func TestTransitionsTestSuite(t *testing.T) {
    suite.Run(t, new(Transitions_TS))
}

//////////
// Tests
//////////

func (ts *Transitions_TS) TestDoTransition() {
	var (
		t              *testing.T
		testParams     model.TransitionParameter
		testTransition model.Transition
		resultsPb      model.Passback
		results        model.TransitionResp
	)
	t = ts.T()
	/////////
	// Test 1 - doTransition() all xnames invalid.
	/////////
	t.Logf("Test 1 - doTransition() all xnames invalid.")
	testParams = model.TransitionParameter{
		Operation: "On",
		Location: []model.LocationParameter{
			model.LocationParameter{Xname: "foo"},
			model.LocationParameter{Xname: "bar"},
		},
	}
	testTransition, _ = model.ToTransition(testParams, GLOB.ExpireTimeMins)
	(*GLOB.DSP).StoreTransition(testTransition)
	doTransition(testTransition.TransitionID)
	resultsPb = GetTransition(testTransition.TransitionID)
	results = resultsPb.Obj.(model.TransitionResp)
	ts.Assert().Equal(model.TransitionStatusCompleted, results.TransitionStatus,
	                  "Test 1 failed with transition status, %s. Expected %s",
	                  results.TransitionStatus, model.TransitionStatusCompleted)
	ts.Assert().Equal(2, results.TaskCounts.Failed,
	                  "Test 1 failed with unexpected task failure count, %d. Expected %d",
	                  results.TaskCounts.Failed, 2)

	/////////
	// Test 2 - doTransition() No power state data.
	/////////
	t.Logf("Test 2 - doTransition() No power state data.")
	testParams = model.TransitionParameter{
		Operation: "On",
		Location: []model.LocationParameter{
			model.LocationParameter{Xname: "x0c0s1b0n0"},
			model.LocationParameter{Xname: "x0c0s1"},
		},
	}
	testTransition, _ = model.ToTransition(testParams, GLOB.ExpireTimeMins)
	(*GLOB.DSP).StoreTransition(testTransition)
	doTransition(testTransition.TransitionID)
	resultsPb = GetTransition(testTransition.TransitionID)
	results = resultsPb.Obj.(model.TransitionResp)
	ts.Assert().Equal(model.TransitionStatusCompleted, results.TransitionStatus,
	                  "Test 2 failed with transition status, %s. Expected %s",
	                  results.TransitionStatus, model.TransitionStatusCompleted)
	ts.Assert().Equal(2, results.TaskCounts.Failed,
	                  "Test 2 failed with unexpected task failure count, %d. Expected %d",
	                  results.TaskCounts.Failed, 2)

	/////////
	// Gather power state data for the next tests
	/////////
	t.Logf("Gather power state data for the next tests")
	updateComponentMap()
	getHWStatesFromHW()

	/////////
	// Test 3 - doTransition() Already correct state.
	/////////
	t.Logf("Test 3 - doTransition() Already correct state.")
	testParams = model.TransitionParameter{
		Operation: "On",
		Location: []model.LocationParameter{
			model.LocationParameter{Xname: "x0c0s1b0n0"},
			model.LocationParameter{Xname: "x0c0s1"},
		},
	}
	testTransition, _ = model.ToTransition(testParams, GLOB.ExpireTimeMins)
	(*GLOB.DSP).StoreTransition(testTransition)
	doTransition(testTransition.TransitionID)
	resultsPb = GetTransition(testTransition.TransitionID)
	results = resultsPb.Obj.(model.TransitionResp)
	ts.Assert().Equal(model.TransitionStatusCompleted, results.TransitionStatus,
	                  "Test 3 failed with transition status, %s. Expected %s",
	                  results.TransitionStatus, model.TransitionStatusCompleted)
	ts.Assert().Equal(2, results.TaskCounts.Succeeded,
	                  "Test 3 failed with unexpected task succeeded count, %d. Expected %d",
	                  results.TaskCounts.Succeeded, 2)

	/////////
	// Test 4 - doTransition() Off
	/////////
	t.Logf("Test 4 - doTransition() Off")
	testParams = model.TransitionParameter{
		Operation: "Off",
		Location: []model.LocationParameter{
			model.LocationParameter{Xname: "x0c0s1b0n0"},
			model.LocationParameter{Xname: "x0c0s1"},
		},
	}
	testTransition, _ = model.ToTransition(testParams, GLOB.ExpireTimeMins)
	(*GLOB.DSP).StoreTransition(testTransition)
	go doTransition(testTransition.TransitionID)
	// Wait for completion
	for i := 0; i < 60; i++ {
		time.Sleep(5 * time.Second)
		getHWStatesFromHW()
		resultsPb = GetTransition(testTransition.TransitionID)
		results = resultsPb.Obj.(model.TransitionResp)
		if results.TransitionStatus == model.TransitionStatusCompleted {
			break
		}
	}
	ts.Assert().Equal(model.TransitionStatusCompleted, results.TransitionStatus,
	                  "Test 4 failed with transition status, %s. Expected %s",
	                  results.TransitionStatus, model.TransitionStatusCompleted)
	ts.Assert().Equal(2, results.TaskCounts.Succeeded,
	                  "Test 4 failed with unexpected task succeeded count, %d. Expected %d",
	                  results.TaskCounts.Succeeded, 2)

	/////////
	// Update power state data for the next tests
	/////////
	t.Logf("Update power state data for the next tests")
	getHWStatesFromHW()

	/////////
	// Test 5 - doTransition() Restart Init task w/ 4 previously created incomplete tasks.
	// Transition state: 
	// - x0c0s1b0n0(off) off command received waiting to confirm
	// - x0c0s2b0n0(on) off command may have been sent.
	// - x0c0s1(off) Nothing done yet
	// - x0c0s2(on) Nothing done yet
	/////////
	t.Logf("Test 5 - doTransition() Restart Init task w/ 4 previously created incomplete tasks.")
	testParams = model.TransitionParameter{
		Operation: "Init",
		Location: []model.LocationParameter{
			model.LocationParameter{Xname: "x0c0s1b0n0"},
			model.LocationParameter{Xname: "x0c0s2b0n0"},
			model.LocationParameter{Xname: "x0c0s1"},
			model.LocationParameter{Xname: "x0c0s2"},
		},
	}
	testTransition, _ = model.ToTransition(testParams, GLOB.ExpireTimeMins)
	testTransition.Status = model.TransitionStatusInProgress
	task := model.NewTransitionTask(testTransition.TransitionID, testTransition.Operation)
	task.Xname = "x0c0s1b0n0"
	task.Operation = model.Operation_Off
	task.State = model.TaskState_Waiting
	testTransition.TaskIDs = append(testTransition.TaskIDs, task.TaskID)
	(*GLOB.DSP).StoreTransitionTask(task)
	task = model.NewTransitionTask(testTransition.TransitionID, testTransition.Operation)
	task.Xname = "x0c0s2b0n0"
	task.Operation = model.Operation_Off
	task.State = model.TaskState_Sending
	testTransition.TaskIDs = append(testTransition.TaskIDs, task.TaskID)
	(*GLOB.DSP).StoreTransitionTask(task)
	task = model.NewTransitionTask(testTransition.TransitionID, testTransition.Operation)
	task.Xname = "x0c0s1"
	task.Operation = model.Operation_Init
	task.State = model.TaskState_GatherData
	testTransition.TaskIDs = append(testTransition.TaskIDs, task.TaskID)
	(*GLOB.DSP).StoreTransitionTask(task)
	task = model.NewTransitionTask(testTransition.TransitionID, testTransition.Operation)
	task.Xname = "x0c0s2"
	task.Operation = model.Operation_Init
	task.State = model.TaskState_GatherData
	testTransition.TaskIDs = append(testTransition.TaskIDs, task.TaskID)
	(*GLOB.DSP).StoreTransitionTask(task)
	(*GLOB.DSP).StoreTransition(testTransition)
	go doTransition(testTransition.TransitionID)
	// Wait for completion
	for i := 0; i < 60; i++ {
		time.Sleep(5 * time.Second)
		getHWStatesFromHW()
		resultsPb = GetTransition(testTransition.TransitionID)
		results = resultsPb.Obj.(model.TransitionResp)
		if results.TransitionStatus == model.TransitionStatusCompleted {
			break
		}
	}
	ts.Assert().Equal(model.TransitionStatusCompleted, results.TransitionStatus,
	                  "Test 5 failed with transition status, %s. Expected %s",
	                  results.TransitionStatus, model.TransitionStatusCompleted)
	ts.Assert().Equal(4, results.TaskCounts.Succeeded,
	                  "Test 5 failed with unexpected task succeeded count, %d. Expected %d",
	                  results.TaskCounts.Succeeded, 4)
}

func (ts *Transitions_TS) TestAbortTransitionID() {
	var (
		t              *testing.T
		testParams     model.TransitionParameter
		testTransition model.Transition
		resultsPb      model.Passback
		results        model.TransitionResp
	)
	t = ts.T()

	/////////
	// Test 1 - AbortTransitionID() Does not exist.
	/////////
	t.Logf("Test 1 - AbortTransitionID() Does not exist.")
	id := uuid.New()
	resultsPb = AbortTransitionID(id)
	ts.Assert().Equal(http.StatusNotFound, resultsPb.StatusCode,
	                  "Test 1 failed with status code, %d. Expected %d",
	                  resultsPb.StatusCode, http.StatusNotFound)

	/////////
	// Test 2 - AbortTransitionID() Already complete.
	/////////
	t.Logf("Test 2 - AbortTransitionID() Already complete.")
	testParams = model.TransitionParameter{
		Operation: "Off",
		Location: []model.LocationParameter{
			model.LocationParameter{Xname: "x0c0s1b0n0"},
		},
	}
	testTransition, _ = model.ToTransition(testParams, GLOB.ExpireTimeMins)
	testTransition.Status = model.TransitionStatusCompleted
	(*GLOB.DSP).StoreTransition(testTransition)
	resultsPb = AbortTransitionID(testTransition.TransitionID)
	ts.Assert().Equal(http.StatusBadRequest, resultsPb.StatusCode,
	                  "Test 2 failed with status code, %d. Expected %d",
	                  resultsPb.StatusCode, http.StatusBadRequest)

	/////////
	// Test 3 - AbortTransitionID() Success.
	/////////
	t.Logf("Test 3 - AbortTransitionID() Success.")
	testParams = model.TransitionParameter{
		Operation: "Off",
		Location: []model.LocationParameter{
			model.LocationParameter{Xname: "x0c0s1b0n0"},
		},
	}
	testTransition, _ = model.ToTransition(testParams, GLOB.ExpireTimeMins)
	(*GLOB.DSP).StoreTransition(testTransition)
	resultsPb = AbortTransitionID(testTransition.TransitionID)
	ts.Assert().Equal(http.StatusAccepted, resultsPb.StatusCode,
	                  "Test 3 failed with status code, %d. Expected %d",
	                  resultsPb.StatusCode, http.StatusAccepted)
	resultsPb = GetTransition(testTransition.TransitionID)
	results = resultsPb.Obj.(model.TransitionResp)
	ts.Assert().Equal(model.TransitionStatusAbortSignaled, results.TransitionStatus,
	                  "Test 3 failed with transition status, %s. Expected %s",
	                  results.TransitionStatus, model.TransitionStatusAbortSignaled)
}

func (ts *Transitions_TS) TestCheckAbort() {
	var (
		t              *testing.T
		testParams     model.TransitionParameter
		testTransition model.Transition
		resultsAbort   bool
		err            error
	)
	t = ts.T()

	/////////
	// Test 1 - checkAbort() Does not exist.
	/////////
	t.Logf("Test 1 - checkAbort() Does not exist.")
	testParams = model.TransitionParameter{
		Operation: "Off",
		Location: []model.LocationParameter{
			model.LocationParameter{Xname: "x0c0s1b0n0"},
		},
	}
	testTransition, _ = model.ToTransition(testParams, GLOB.ExpireTimeMins)
	resultsAbort, err = checkAbort(testTransition)
	ts.Assert().Empty(err,
	                  "Test 1 failed with error, %v. Expected %s",
	                  err, "nil")
	ts.Assert().Equal(false, resultsAbort,
	                  "Test 1 failed with abort bool, %v. Expected %v",
	                  resultsAbort, false)

	/////////
	// Test 2 - checkAbort() Exists, no abort.
	/////////
	t.Logf("Test 2 - checkAbort() Exists, no abort.")
	testParams = model.TransitionParameter{
		Operation: "Off",
		Location: []model.LocationParameter{
			model.LocationParameter{Xname: "x0c0s1b0n0"},
		},
	}
	testTransition, _ = model.ToTransition(testParams, GLOB.ExpireTimeMins)
	(*GLOB.DSP).StoreTransition(testTransition)
	resultsAbort, err = checkAbort(testTransition)
	ts.Assert().Empty(err,
	                  "Test 2 failed with error, %v. Expected %s",
	                  err, "nil")
	ts.Assert().Equal(false, resultsAbort,
	                  "Test 2 failed with abort bool, %v. Expected %v",
	                  resultsAbort, false)

	/////////
	// Test 3 - checkAbort() Exists, w\ abort.
	/////////
	t.Logf("Test 3 - checkAbort() Exists, w\\ abort.")
	testTransition.Status = model.TransitionStatusAbortSignaled
	(*GLOB.DSP).StoreTransition(testTransition)
	resultsAbort, err = checkAbort(testTransition)
	ts.Assert().Empty(err,
	                  "Test 3 failed with error, %v. Expected %s",
	                  err, "nil")
	ts.Assert().Equal(true, resultsAbort,
	                  "Test 3 failed with abort bool, %v. Expected %v",
	                  resultsAbort, true)
}

func (ts *Transitions_TS) TestDoAbort() {
	var (
		t              *testing.T
		testParams     model.TransitionParameter
		testTransition model.Transition
		resultsPb      model.Passback
		results        model.TransitionResp
	)
	t = ts.T()

	/////////
	// Test 1 - doAbort()
	/////////
	t.Logf("Test 1 - doAbort()")
	testParams = model.TransitionParameter{
		Operation: "Off",
		Location: []model.LocationParameter{
			model.LocationParameter{Xname: "x0c0s1b0n0"},
			model.LocationParameter{Xname: "x0c0s2b0n0"},
			model.LocationParameter{Xname: "x0c0s1"},
			model.LocationParameter{Xname: "x0c0s2"},
		},
	}
	testTransition, _ = model.ToTransition(testParams, GLOB.ExpireTimeMins)
	task1 := model.NewTransitionTask(testTransition.TransitionID, testTransition.Operation)
	task1.Xname = "x0c0s1b0n0"
	task1.Status = model.TransitionTaskStatusNew
	(*GLOB.DSP).StoreTransitionTask(task1)
	task2 := model.NewTransitionTask(testTransition.TransitionID, testTransition.Operation)
	task2.Xname = "x0c0s2b0n0"
	task2.Status = model.TransitionTaskStatusInProgress
	(*GLOB.DSP).StoreTransitionTask(task2)
	task3 := model.NewTransitionTask(testTransition.TransitionID, testTransition.Operation)
	task3.Xname = "x0c0s1"
	task3.Status = model.TransitionTaskStatusFailed
	task3.Error = "My Error"
	task3.StatusDesc = "My description"
	(*GLOB.DSP).StoreTransitionTask(task3)
	task4 := model.NewTransitionTask(testTransition.TransitionID, testTransition.Operation)
	task4.Xname = "x0c0s2"
	task4.Status = model.TransitionTaskStatusSucceeded
	(*GLOB.DSP).StoreTransitionTask(task4)
	testXnameMap := map[string]*TransitionComponent{
		"x0c0s1b0n0": &TransitionComponent{Task: &task1},
		"x0c0s2b0n0": &TransitionComponent{Task: &task2},
		"x0c0s1": &TransitionComponent{Task: &task3},
		"x0c0s2": &TransitionComponent{Task: &task4},
	}
	doAbort(testTransition, testXnameMap)

	resultsPb = GetTransition(testTransition.TransitionID)
	results = resultsPb.Obj.(model.TransitionResp)
	ts.Assert().Equal(model.TransitionStatusAborted, results.TransitionStatus,
	                  "Test 1 failed with transition status, %s. Expected %s",
	                  results.TransitionStatus, model.TransitionStatusAborted)
	ts.Assert().Equal(4, results.TaskCounts.Total,
	                  "Test 1 failed with unexpected task total count, %d. Expected %d",
	                  results.TaskCounts.Total, 4)
	ts.Assert().Equal(3, results.TaskCounts.Failed,
	                  "Test 1 failed with unexpected task failed count, %d. Expected %d",
	                  results.TaskCounts.Failed, 3)
	ts.Assert().Equal(1, results.TaskCounts.Succeeded,
	                  "Test 1 failed with unexpected task succeeded count, %d. Expected %d",
	                  results.TaskCounts.Succeeded, 1)
}

func (ts *Transitions_TS) TestSequenceComponents() {
	var (
		t              *testing.T
		testParams     model.TransitionParameter
		testTransition model.Transition
		testXnameMap   map[string]*TransitionComponent
		resultsSeq     map[string]map[base.HMSType][]*TransitionComponent
	)
	t = ts.T()

	/////////
	// Test 1 - sequenceComponents() - 4 components Off
	/////////
	t.Logf("Test 1 - sequenceComponents() - 4 components Off")
	testParams = model.TransitionParameter{
		Operation: "Off",
		Location: []model.LocationParameter{
			model.LocationParameter{Xname: "x0c0s1b0n0"},
			model.LocationParameter{Xname: "x0c0s2b0n0"},
			model.LocationParameter{Xname: "x0c0s1"},
			model.LocationParameter{Xname: "x0c0s2"},
		},
	}
	testTransition, _ = model.ToTransition(testParams, GLOB.ExpireTimeMins)
	(*GLOB.DSP).StoreTransition(testTransition)
	xnames := []string{"x0c0s1b0n0","x0c0s2b0n0","x0c0s1","x0c0s2"}
	pStates, _, _ := getPowerStateHierarchy(xnames)
	hsmData, _ := (*GLOB.HSM).FillHSMData(xnames)
	task1 := model.NewTransitionTask(testTransition.TransitionID, testTransition.Operation)
	task1.Xname = "x0c0s1b0n0"
	pState1 := pStates["x0c0s1b0n0"]
	pState1.PowerState = "on"
	actions1 := make(map[string]string)
	for _, action := range hsmData["x0c0s1b0n0"].AllowableActions {
		actions1[strings.ToLower(action)] = action
	}
	(*GLOB.DSP).StoreTransitionTask(task1)
	task2 := model.NewTransitionTask(testTransition.TransitionID, testTransition.Operation)
	task2.Xname = "x0c0s2b0n0"
	pState2 := pStates["x0c0s2b0n0"]
	pState2.PowerState = "on"
	actions2 := make(map[string]string)
	for _, action := range hsmData["x0c0s2b0n0"].AllowableActions {
		actions2[strings.ToLower(action)] = action
	}
	(*GLOB.DSP).StoreTransitionTask(task2)
	task3 := model.NewTransitionTask(testTransition.TransitionID, testTransition.Operation)
	task3.Xname = "x0c0s1"
	pState3 := pStates["x0c0s1"]
	pState3.PowerState = "on"
	actions3 := make(map[string]string)
	for _, action := range hsmData["x0c0s1"].AllowableActions {
		actions3[strings.ToLower(action)] = action
	}
	(*GLOB.DSP).StoreTransitionTask(task3)
	task4 := model.NewTransitionTask(testTransition.TransitionID, testTransition.Operation)
	task4.Xname = "x0c0s2"
	pState4 := pStates["x0c0s2"]
	pState4.PowerState = "on"
	actions4 := make(map[string]string)
	for _, action := range hsmData["x0c0s2"].AllowableActions {
		actions4[strings.ToLower(action)] = action
	}
	(*GLOB.DSP).StoreTransitionTask(task4)

	testXnameMap = map[string]*TransitionComponent{
		"x0c0s1b0n0": &TransitionComponent{
			PState: &pState1,
			HSMData: hsmData["x0c0s1b0n0"],
			Task: &task1,
			Actions: actions1,
		},
		"x0c0s2b0n0": &TransitionComponent{
			PState: &pState2,
			HSMData: hsmData["x0c0s2b0n0"],
			Task: &task2,
			Actions: actions2,
		},
		"x0c0s1": &TransitionComponent{
			PState: &pState3,
			HSMData: hsmData["x0c0s1"],
			Task: &task3,
			Actions: actions3,
		},
		"x0c0s2": &TransitionComponent{
			PState: &pState4,
			HSMData: hsmData["x0c0s2"],
			Task: &task4,
			Actions: actions4,
		},
	}

	resultsSeq, _ = sequenceComponents(testTransition.Operation, testXnameMap)
	ts.Assert().Equal(0, len(resultsSeq["on"]),
	                  "Test 1 failed with sequence map 'on' len, %d. Expected %d",
	                  len(resultsSeq["on"]), 0)
	ts.Assert().Equal(2, len(resultsSeq["gracefulshutdown"]),
	                  "Test 1 failed with sequence map 'gracefulshutdown' len, %d. Expected %d",
	                  len(resultsSeq["gracefulshutdown"]), 2)
	ts.Assert().Equal(2, len(resultsSeq["gracefulshutdown"]["Node"]),
	                  "Test 1 failed with sequence map 'gracefulshutdown'.'Node' len, %d. Expected %d",
	                  len(resultsSeq["gracefulshutdown"]["Node"]), 2)
	ts.Assert().Equal(2, len(resultsSeq["gracefulshutdown"]["ComputeModule"]),
	                  "Test 1 failed with sequence map 'gracefulshutdown'.'ComputeModule' len, %d. Expected %d",
	                  len(resultsSeq["gracefulshutdown"]["ComputeModule"]), 2)
	ts.Assert().Equal(0, len(resultsSeq["gracefulrestart"]),
	                  "Test 1 failed with sequence map 'gracefulrestart' len, %d. Expected %d",
	                  len(resultsSeq["gracefulrestart"]), 0)
	ts.Assert().Equal(0, len(resultsSeq["forceoff"]),
	                  "Test 1 failed with sequence map 'forceoff' len, %d. Expected %d",
	                  len(resultsSeq["forceoff"]), 0)

	/////////
	// Test 2 - sequenceComponents() - 4 components in-progress init
	// Transition state: 
	// - x0c0s1b0n0(off) off command received waiting to confirm
	// - x0c0s2b0n0(on) off command may have been sent.
	// - x0c0s1(off) Nothing done yet
	// - x0c0s2(on) Nothing done yet
	/////////
	t.Logf("Test 2 - sequenceComponents() - 4 components in-progress init")
	testParams = model.TransitionParameter{
		Operation: "Init",
		Location: []model.LocationParameter{
			model.LocationParameter{Xname: "x0c0s1b0n0"},
			model.LocationParameter{Xname: "x0c0s2b0n0"},
			model.LocationParameter{Xname: "x0c0s1"},
			model.LocationParameter{Xname: "x0c0s2"},
		},
	}
	testTransition, _ = model.ToTransition(testParams, GLOB.ExpireTimeMins)
	testTransition.Status = model.TransitionStatusInProgress
	(*GLOB.DSP).StoreTransition(testTransition)
	xnames = []string{"x0c0s1b0n0","x0c0s2b0n0","x0c0s1","x0c0s2"}
	pStates, _, _ = getPowerStateHierarchy(xnames)
	hsmData, _ = (*GLOB.HSM).FillHSMData(xnames)
	task1 = model.NewTransitionTask(testTransition.TransitionID, testTransition.Operation)
	task1.Xname = "x0c0s1b0n0"
	task1.Status = model.TransitionTaskStatusInProgress
	task1.Operation = model.Operation_Off
	task1.State = model.TaskState_Waiting
	pState1 = pStates["x0c0s1b0n0"]
	pState1.PowerState = "off"
	actions1 = make(map[string]string)
	for _, action := range hsmData["x0c0s1b0n0"].AllowableActions {
		actions1[strings.ToLower(action)] = action
	}
	(*GLOB.DSP).StoreTransitionTask(task1)
	task2 = model.NewTransitionTask(testTransition.TransitionID, testTransition.Operation)
	task2.Xname = "x0c0s2b0n0"
	task2.Operation = model.Operation_Off
	task2.State = model.TaskState_Sending
	pState2 = pStates["x0c0s2b0n0"]
	pState2.PowerState = "on"
	actions2 = make(map[string]string)
	for _, action := range hsmData["x0c0s2b0n0"].AllowableActions {
		actions2[strings.ToLower(action)] = action
	}
	(*GLOB.DSP).StoreTransitionTask(task2)
	task3 = model.NewTransitionTask(testTransition.TransitionID, testTransition.Operation)
	task3.Xname = "x0c0s1"
	task3.Operation = model.Operation_Init
	task3.State = model.TaskState_GatherData
	pState3 = pStates["x0c0s1"]
	pState3.PowerState = "off"
	actions3 = make(map[string]string)
	for _, action := range hsmData["x0c0s1"].AllowableActions {
		actions3[strings.ToLower(action)] = action
	}
	(*GLOB.DSP).StoreTransitionTask(task3)
	task4 = model.NewTransitionTask(testTransition.TransitionID, testTransition.Operation)
	task4.Xname = "x0c0s2"
	task4.Operation = model.Operation_Init
	task4.State = model.TaskState_GatherData
	pState4 = pStates["x0c0s2"]
	pState4.PowerState = "on"
	actions4 = make(map[string]string)
	for _, action := range hsmData["x0c0s2"].AllowableActions {
		actions4[strings.ToLower(action)] = action
	}
	(*GLOB.DSP).StoreTransitionTask(task4)

	testXnameMap = map[string]*TransitionComponent{
		"x0c0s1b0n0": &TransitionComponent{
			PState: &pState1,
			HSMData: hsmData["x0c0s1b0n0"],
			Task: &task1,
			Actions: actions1,
		},
		"x0c0s2b0n0": &TransitionComponent{
			PState: &pState2,
			HSMData: hsmData["x0c0s2b0n0"],
			Task: &task2,
			Actions: actions2,
		},
		"x0c0s1": &TransitionComponent{
			PState: &pState3,
			HSMData: hsmData["x0c0s1"],
			Task: &task3,
			Actions: actions3,
		},
		"x0c0s2": &TransitionComponent{
			PState: &pState4,
			HSMData: hsmData["x0c0s2"],
			Task: &task4,
			Actions: actions4,
		},
	}

	resultsSeq, _ = sequenceComponents(testTransition.Operation, testXnameMap)
	ts.Assert().Equal(2, len(resultsSeq["on"]),
	                  "Test 2 failed with sequence map 'on' len, %d. Expected %d",
	                  len(resultsSeq["on"]), 2)
	ts.Assert().Equal(2, len(resultsSeq["on"]["Node"]),
	                  "Test 2 failed with sequence map 'on'.'Node' len, %d. Expected %d",
	                  len(resultsSeq["on"]["Node"]), 2)
	ts.Assert().Equal(2, len(resultsSeq["on"]["ComputeModule"]),
	                  "Test 2 failed with sequence map 'on'.'ComputeModule' len, %d. Expected %d",
	                  len(resultsSeq["on"]["ComputeModule"]), 2)
	ts.Assert().Equal(2, len(resultsSeq["gracefulshutdown"]),
	                  "Test 2 failed with sequence map 'gracefulshutdown' len, %d. Expected %d",
	                  len(resultsSeq["gracefulshutdown"]), 2)
	ts.Assert().Equal(1, len(resultsSeq["gracefulshutdown"]["Node"]),
	                  "Test 2 failed with sequence map 'gracefulshutdown'.'Node' len, %d. Expected %d",
	                  len(resultsSeq["gracefulshutdown"]["Node"]), 1)
	ts.Assert().Equal(1, len(resultsSeq["gracefulshutdown"]["ComputeModule"]),
	                  "Test 2 failed with sequence map 'gracefulshutdown'.'ComputeModule' len, %d. Expected %d",
	                  len(resultsSeq["gracefulshutdown"]["ComputeModule"]), 1)
	ts.Assert().Equal(0, len(resultsSeq["gracefulrestart"]),
	                  "Test 2 failed with sequence map 'gracefulrestart' len, %d. Expected %d",
	                  len(resultsSeq["gracefulrestart"]), 0)
	ts.Assert().Equal(0, len(resultsSeq["forceoff"]),
	                  "Test 2 failed with sequence map 'forceoff' len, %d. Expected %d",
	                  len(resultsSeq["forceoff"]), 0)
}

func (ts *Transitions_TS) TestGetPowerSupplies() {
	var (
		t          *testing.T
		testParams hsm.HsmData
		results    []PowerSupply
	)
	t = ts.T()

	/////////
	// Test 1 - getPowerSupplies() - 2 power supplies, 1 missing
	/////////
	t.Logf("Test 1 - getPowerSupplies() - 2 power supplies, 1 missing")
	connectorXname1 := "x0m0p0v10"
	connectorXname2 := "x0m0p1v10"
	testParams = hsm.HsmData{
		PoweredBy: []string{connectorXname1, connectorXname2},
	}
	connectorPowerStatus := model.PowerStatusComponent{
		XName:           connectorXname1,
		PowerState:      model.PowerStateFilter_On.String(),
		ManagementState: model.ManagementStateFilter_available.String(),
	}
	(*GLOB.DSP).StorePowerStatus(connectorPowerStatus)

	results = getPowerSupplies(&testParams)

	ts.Assert().Equal(2, len(results),
	                  "Test 1 failed with powerSupply array len, %d. Expected %d",
	                  len(results), 2)
	ts.Assert().Equal(connectorXname1, results[0].ID,
	                  "Test 1 failed with powerSupply[0] ID, %s. Expected %s",
	                  results[0].ID, connectorXname1)
	ts.Assert().Equal(model.PowerStateFilter_On, results[0].State,
	                  "Test 1 failed with powerSupply[0] State, %s. Expected %s",
	                  results[0].State.String(), model.PowerStateFilter_On.String())
	ts.Assert().Equal(connectorXname2, results[1].ID,
	                  "Test 1 failed with powerSupply[1] ID, %s. Expected %s",
	                  results[1].ID, connectorXname2)
	ts.Assert().Equal(model.PowerStateFilter_Undefined, results[1].State,
	                  "Test 1 failed with powerSupply[1] State, %s. Expected %s",
	                  results[1].State.String(), model.PowerStateFilter_Undefined.String())

	/////////
	// Test 2 - getPowerSupplies() - 2 power supplies
	/////////
	t.Logf("Test 2 - getPowerSupplies() - 2 power supplies")
	connectorXname1 = "x0m0p0v10"
	connectorXname2 = "x0m0p1v10"
	testParams = hsm.HsmData{
		PoweredBy: []string{connectorXname1, connectorXname2},
	}
	connectorPowerStatus = model.PowerStatusComponent{
		XName:           connectorXname2,
		PowerState:      model.PowerStateFilter_Off.String(),
		ManagementState: model.ManagementStateFilter_available.String(),
	}
	(*GLOB.DSP).StorePowerStatus(connectorPowerStatus)

	results = getPowerSupplies(&testParams)

	ts.Assert().Equal(2, len(results),
	                  "Test 1 failed with powerSupply array len, %d. Expected %d",
	                  len(results), 2)
	ts.Assert().Equal(connectorXname1, results[0].ID,
	                  "Test 1 failed with powerSupply[0] ID, %s. Expected %s",
	                  results[0].ID, connectorXname1)
	ts.Assert().Equal(model.PowerStateFilter_On, results[0].State,
	                  "Test 1 failed with powerSupply[0] State, %s. Expected %s",
	                  results[0].State.String(), model.PowerStateFilter_On.String())
	ts.Assert().Equal(connectorXname2, results[1].ID,
	                  "Test 1 failed with powerSupply[1] ID, %s. Expected %s",
	                  results[1].ID, connectorXname2)
	ts.Assert().Equal(model.PowerStateFilter_Off, results[1].State,
	                  "Test 1 failed with powerSupply[1] State, %s. Expected %s",
	                  results[1].State.String(), model.PowerStateFilter_Off.String())

	/////////
	// Test 3 - getPowerSupplies() - No power supplies
	/////////
	t.Logf("Test 3 - getPowerSupplies() - No power supplies")
	testParams = hsm.HsmData{}

	results = getPowerSupplies(&testParams)

	ts.Assert().Equal(0, len(results),
	                  "Test 1 failed with powerSupply array len, %d. Expected %d",
	                  len(results), 0)
}

func (ts *Transitions_TS) TestFailDependentComps() {
	var (
		t            *testing.T
		testXnameMap map[string]*TransitionComponent
	)
	t = ts.T()

	/////////
	// Test 1 - failDependentComps() - Node 2 power supplies, 1 in our list, will lose power
	/////////
	t.Logf("Test 1 - failDependentComps() - Node 2 power supplies, 1 in our list, will lose power")
	nodeXname := "x0c0s0b0n0"
	moduleXname := "x0c0s0"
	connectorXname1 := "x0m0p0v10"
	connectorXname2 := "x0m0p1v10"
	transactionID := uuid.New()

	task1 := model.NewTransitionTask(transactionID, model.Operation_SoftOff)
	task1.Xname = nodeXname

	task2 := model.NewTransitionTask(transactionID, model.Operation_SoftOff)
	task2.Xname = moduleXname

	task3 := model.NewTransitionTask(transactionID, model.Operation_SoftOff)
	task3.Xname = connectorXname1

	powerSupplies := []PowerSupply{
		PowerSupply{
			ID:    connectorXname1,
			State: model.PowerStateFilter_On,
		},
		PowerSupply{
			ID:    connectorXname2,
			State: model.PowerStateFilter_Off,
		},
	}

	testXnameMap = map[string]*TransitionComponent{
		"x0c0s0b0n0": &TransitionComponent{
			Task:          &task1,
			PowerSupplies: powerSupplies,
		},
		"x0c0s0": &TransitionComponent{
			Task: &task2,
		},
		"x0m0p0v10": &TransitionComponent{
			Task: &task3,
		},
	}

	failDependentComps(testXnameMap, "gracefulshutdown", nodeXname, "Test Error")

	ts.Assert().Equal(model.TransitionTaskStatusFailed, task2.Status,
	                  "Test 1 failed with computeModule Task.Status, %s. Expected %s",
	                  task2.Status, model.TransitionTaskStatusFailed)
	ts.Assert().Equal(model.TransitionTaskStatusFailed, task3.Status,
	                  "Test 1 failed with powerConnector Task.Status, %s. Expected %s",
	                  task3.Status, model.TransitionTaskStatusFailed)

	/////////
	// Test 2 - failDependentComps() - Node 2 power supplies, 1 in our list, will not lose power
	/////////
	t.Logf("Test 2 - failDependentComps() - Node 2 power supplies, 1 in our list, will not lose power")
	nodeXname = "x0c0s0b0n0"
	moduleXname = "x0c0s0"
	connectorXname1 = "x0m0p0v10"
	connectorXname2 = "x0m0p1v10"
	transactionID = uuid.New()

	task1 = model.NewTransitionTask(transactionID, model.Operation_SoftOff)
	task1.Xname = nodeXname

	task2 = model.NewTransitionTask(transactionID, model.Operation_SoftOff)
	task2.Xname = moduleXname

	task3 = model.NewTransitionTask(transactionID, model.Operation_SoftOff)
	task3.Xname = connectorXname1

	powerSupplies = []PowerSupply{
		PowerSupply{
			ID:    connectorXname1,
			State: model.PowerStateFilter_On,
		},
		PowerSupply{
			ID:    connectorXname2,
			State: model.PowerStateFilter_On,
		},
	}

	testXnameMap = map[string]*TransitionComponent{
		"x0c0s0b0n0": &TransitionComponent{
			Task:          &task1,
			PowerSupplies: powerSupplies,
		},
		"x0c0s0": &TransitionComponent{
			Task: &task2,
		},
		"x0m0p0v10": &TransitionComponent{
			Task: &task3,
		},
	}

	failDependentComps(testXnameMap, "gracefulshutdown", nodeXname, "Test Error")

	ts.Assert().Equal(model.TransitionTaskStatusFailed, task2.Status,
	                  "Test 1 failed with computeModule Task.Status, %s. Expected %s",
	                  task2.Status, model.TransitionTaskStatusFailed)
	ts.Assert().Equal(model.TransitionTaskStatusNew, task3.Status,
	                  "Test 1 failed with powerConnector Task.Status, %s. Expected %s",
	                  task3.Status, model.TransitionTaskStatusNew)

	/////////
	// Test 3 - failDependentComps() - Node missing power supplies, 1 in our list
	/////////
	t.Logf("Test 3 - failDependentComps() - Node missing power supplies, 1 in our list")
	nodeXname = "x0c0s0b0n0"
	moduleXname = "x0c0s0"
	connectorXname1 = "x0m0p0v10"
	transactionID = uuid.New()

	task1 = model.NewTransitionTask(transactionID, model.Operation_SoftOff)
	task1.Xname = nodeXname

	task2 = model.NewTransitionTask(transactionID, model.Operation_SoftOff)
	task2.Xname = moduleXname

	task3 = model.NewTransitionTask(transactionID, model.Operation_SoftOff)
	task3.Xname = connectorXname1

	testXnameMap = map[string]*TransitionComponent{
		"x0c0s0b0n0": &TransitionComponent{
			Task: &task1,
		},
		"x0c0s0": &TransitionComponent{
			Task: &task2,
		},
		"x0m0p0v10": &TransitionComponent{
			Task: &task3,
		},
	}

	failDependentComps(testXnameMap, "gracefulshutdown", nodeXname, "Test Error")

	ts.Assert().Equal(model.TransitionTaskStatusFailed, task2.Status,
	                  "Test 1 failed with computeModule Task.Status, %s. Expected %s",
	                  task2.Status, model.TransitionTaskStatusFailed)
	ts.Assert().Equal(model.TransitionTaskStatusNew, task3.Status,
	                  "Test 1 failed with powerConnector Task.Status, %s. Expected %s",
	                  task3.Status, model.TransitionTaskStatusNew)

	/////////
	// Test 4 - failDependentComps() - Node 2 power supplies, both in our list
	/////////
	t.Logf("Test 4 - failDependentComps() - Node 2 power supplies, both in our list")
	nodeXname = "x0c0s0b0n0"
	moduleXname = "x0c0s0"
	connectorXname1 = "x0m0p0v10"
	connectorXname2 = "x0m0p1v10"
	transactionID = uuid.New()

	task1 = model.NewTransitionTask(transactionID, model.Operation_SoftOff)
	task1.Xname = nodeXname

	task2 = model.NewTransitionTask(transactionID, model.Operation_SoftOff)
	task2.Xname = moduleXname

	task3 = model.NewTransitionTask(transactionID, model.Operation_SoftOff)
	task3.Xname = connectorXname1

	task4 := model.NewTransitionTask(transactionID, model.Operation_SoftOff)
	task4.Xname = connectorXname2

	powerSupplies = []PowerSupply{
		PowerSupply{
			ID:    connectorXname1,
			State: model.PowerStateFilter_On,
		},
		PowerSupply{
			ID:    connectorXname2,
			State: model.PowerStateFilter_On,
		},
	}

	testXnameMap = map[string]*TransitionComponent{
		"x0c0s0b0n0": &TransitionComponent{
			Task:          &task1,
			PowerSupplies: powerSupplies,
		},
		"x0c0s0": &TransitionComponent{
			Task: &task2,
		},
		"x0m0p0v10": &TransitionComponent{
			Task: &task3,
		},
		"x0m0p1v10": &TransitionComponent{
			Task: &task4,
		},
	}

	failDependentComps(testXnameMap, "gracefulshutdown", nodeXname, "Test Error")

	ts.Assert().Equal(model.TransitionTaskStatusFailed, task2.Status,
	                  "Test 1 failed with computeModule Task.Status, %s. Expected %s",
	                  task2.Status, model.TransitionTaskStatusFailed)
	ts.Assert().Equal(model.TransitionTaskStatusNew, task3.Status,
	                  "Test 1 failed with powerConnector Task.Status, %s. Expected %s",
	                  task3.Status, model.TransitionTaskStatusNew)
	ts.Assert().Equal(model.TransitionTaskStatusFailed, task4.Status,
	                  "Test 1 failed with powerConnector Task.Status, %s. Expected %s",
	                  task4.Status, model.TransitionTaskStatusFailed)
}
