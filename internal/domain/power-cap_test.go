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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Cray-HPE/hms-certs/pkg/hms_certs"
	"github.com/Cray-HPE/hms-power-control/internal/hsm"
	"github.com/Cray-HPE/hms-power-control/internal/logger"
	"github.com/Cray-HPE/hms-power-control/internal/model"
	"github.com/Cray-HPE/hms-power-control/internal/storage"
	trsapi "github.com/Cray-HPE/hms-trs-app-api/v2/pkg/trs_http_api"
)

var RFServerUrl string
var RFServer *httptest.Server
var HSMServer *httptest.Server

func TestDoPowerCapTask(t *testing.T) {
	var err error

	err = doSetup()
	if err != nil {
		t.Errorf("ERROR %v", err)
	}
	defer doShutdown()

	/////////
	// Test 1 - SnapShot Success
	// Tests:
	//   - doPowerCapTask(Snapshot Task Path)
	//   - GetPowerCapQuery()
	/////////
	params := model.PowerCapSnapshotParameter{
		Xnames: []string{"x0c0s0b0n0", "x0c0s1b0n0"},
	}

	task := model.NewPowerCapSnapshotTask(params, GLOB.ExpireTimeMins)

	err = (*GLOB.DSP).StorePowerCapTask(task)
	if err != nil {
		t.Errorf("ERROR doPowerCapTask(1) failed - %s", err.Error())
		return
	}

	doPowerCapTask(task.TaskID)

	pb := GetPowerCapQuery(task.TaskID)
	if pb.IsError {
		t.Errorf("ERROR doPowerCapTask(1) failed - %s", pb.Error.Detail)
	}

	pctc, ok := pb.Obj.(model.PowerCapTaskResp)
	if !ok {
		t.Errorf("ERROR doPowerCapTask(1) failed - Unexpected passback %v", pb)
	}

	if !pctc.Equals(expectedPowerCapTaskResp1) {
		pctcJSON, _ := json.MarshalIndent(pctc, "", "   ")
		expectedJSON, _ := json.MarshalIndent(expectedPowerCapTaskResp1, "", "   ")
		t.Errorf("ERROR doPowerCapTask(1) failed - Unexpected PowerCapTaskResp %s; Expected %s", pctcJSON, expectedJSON)
	}

	/////////
	// Test 2 - Patch Success
	// Tests:
	//   - doPowerCapTask(Patch Task Path)
	//   - GetPowerCapQuery()
	/////////
	params2 := model.PowerCapPatchParameter{Components: []model.PowerCapComponentParameter{{
		Xname: "x0c0s0b0n0",
		Controls: []model.PowerCapControlParameter{{
			Name:  "Node Power Control",
			Value: 450,
		}, {
			Name:  "Accelerator0 Power Control",
			Value: 300,
		}},
	}, {
		Xname: "x0c0s1b0n0",
		Controls: []model.PowerCapControlParameter{{
			Name:  "Node Power Control",
			Value: 500,
		}},
	}}}

	task2 := model.NewPowerCapPatchTask(params2, GLOB.ExpireTimeMins)

	err = (*GLOB.DSP).StorePowerCapTask(task2)
	if err != nil {
		t.Errorf("ERROR doPowerCapTask(2) failed - %s", err.Error())
		return
	}

	doPowerCapTask(task2.TaskID)

	pb2 := GetPowerCapQuery(task2.TaskID)
	if pb2.IsError {
		t.Errorf("ERROR doPowerCapTask(2) failed - %s", pb2.Error.Detail)
	}

	pctc2, ok := pb2.Obj.(model.PowerCapTaskResp)
	if !ok {
		t.Errorf("ERROR doPowerCapTask(2) failed - Unexpected passback %v", pb2)
	}

	if !pctc2.Equals(expectedPowerCapTaskResp2) {
		pctc2JSON, _ := json.MarshalIndent(pctc2, "", "   ")
		expected2JSON, _ := json.MarshalIndent(expectedPowerCapTaskResp2, "", "   ")
		t.Errorf("ERROR doPowerCapTask(2) failed - Unexpected PowerCapTaskResp %s; Expected %s", pctc2JSON, expected2JSON)
	}

	/////////
	// Test 3 - Get All Tasks.
	// Tests:
	//   - GetPowerCap()
	// NOTE: Doing this here in TestDoPowerCapTask()
	//       for the convenience of having 2 tasks
	//       already in storage.
	/////////
	// Fill our expected value with the results from the previous tests.
	expectedPowerCapTaskResp3Map := make(map[string]model.PowerCapTaskResp)
	expectedPowerCapTaskResp3Map[pctc.TaskID.String()] = expectedPowerCapTaskResp1
	expectedPowerCapTaskResp3Map[pctc2.TaskID.String()] = expectedPowerCapTaskResp2

	pb3 := GetPowerCap()
	if pb3.IsError {
		t.Errorf("ERROR doPowerCapTask(3) failed - %s", pb3.Error.Detail)
	}

	pctc3Array, ok := pb3.Obj.(model.PowerCapTaskRespArray)
	if !ok {
		t.Errorf("ERROR doPowerCapTask(3) failed - Unexpected passback %v", pb3)
	}

	for _, pctc3 := range pctc3Array.Tasks {
		expected, _ := expectedPowerCapTaskResp3Map[pctc3.TaskID.String()]
		// Minus the Components[] because they won't have that info.
		expected.Components = nil
		if !pctc3.Equals(expected) {
			pctc3JSON, _ := json.MarshalIndent(pctc3, "", "   ")
			expected3JSON, _ := json.MarshalIndent(expected, "", "   ")
			t.Errorf("ERROR doPowerCapTask(3) failed - Unexpected PowerCapTaskResp %s; Expected %s", pctc3JSON, expected3JSON)
		}
	}

	/////////
	// Test 4 - Test reaper function.
	// Tests:
	//   - powerCapReaper()
	// NOTE: Doing this here in TestDoPowerCapTask()
	//       for the convenience of having 2 tasks
	//       already in storage.
	/////////
	expiredTask, err := (*GLOB.DSP).GetPowerCapTask(task2.TaskID)
	if err != nil {
		t.Errorf("ERROR powerCapReaper() failed - %s", err.Error())
		return
	}
	expiredTask.AutomaticExpirationTime = time.Now().Add(-3 * time.Second)
	err = (*GLOB.DSP).StorePowerCapTask(expiredTask)
	if err != nil {
		t.Errorf("ERROR powerCapReaper() failed - %s", err.Error())
		return
	}
	powerCapReaper()
	pb4 := GetPowerCapQuery(expiredTask.TaskID)
	if !pb4.IsError {
		t.Errorf("ERROR powerCapReaper() failed - Expected an error.")
		return
	}
	if pb4.StatusCode != 404 {
		t.Errorf("ERROR powerCapReaper() failed - Expected a 404 error, received %s.", pb4.Error.Detail)
		return
	}
}

/////////////////////////
// Non-UnitTest functions (Helpers, servers, etc)
/////////////////////////

// Sets up everything needed for running power capping domain functions such as
// httptest servers, HSM/storage/trs packages, etc.
func doSetup() error {
	var (
		err                error
		Running            bool = true
		svcClient          *hms_certs.HTTPClientPair
		TLOC_rf, TLOC_svc  trsapi.TrsAPI
		rfClientLock       *sync.RWMutex = &sync.RWMutex{}
		serviceName        string        = "PCS-domain-power-cap-test"
		DSP                storage.StorageProvider
		HSM                hsm.HSMProvider
		VaultEnabled       bool = false
		StateManagerServer string
		BaseTRSTask        trsapi.HttpTask
		domainGlobals      DOMAIN_GLOBALS
	)

	logger.Init()
	logger.Log.Error()

	RFServer = httptest.NewTLSServer(http.HandlerFunc(rfHandler))
	rfUrl, _ := url.Parse(RFServer.URL)
	RFServerUrl = rfUrl.Host

	HSMServer = httptest.NewServer(http.HandlerFunc(hsmHandler))
	StateManagerServer = HSMServer.URL

	svcClient, err = hms_certs.CreateRetryableHTTPClientPair("", 10, 10, 4)
	if err != nil {
		return err
	}

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

	tmpStorageImplementation := &storage.MEMStorage{
		Logger: logger.Log,
	}
	DSP = tmpStorageImplementation
	DSP.Init(logger.Log)

	HSM = &hsm.HSMv2{}

	hsmGlob := hsm.HSM_GLOBALS{
		SvcName:       serviceName,
		Logger:        logger.Log,
		Running:       &Running,
		SMUrl:         StateManagerServer,
		SVCHttpClient: svcClient,
	}
	HSM.Init(&hsmGlob)

	domainGlobals.NewGlobals(&BaseTRSTask, &TLOC_rf, &TLOC_svc, nil, nil,
		rfClientLock, &Running, &DSP, &HSM, VaultEnabled, nil, nil, 20000, 1440)
	Init(&domainGlobals)
	return nil
}

// Cleans up things from doSetup() that don't get deleted/overwritten by doSetup() (i.e. httptest servers)
func doShutdown() {
	RFServer.Close()
	HSMServer.Close()
}

// Compare function for PowerCapTaskResp structs. Doesn't compare TaskID or any
// of the timestamp fields because they will inevitably not match our test data.
// TODO make this an equality function, put it in the models

// Fake HSM http server
func hsmHandler(w http.ResponseWriter, req *http.Request) {
	switch req.URL.RequestURI() {
	case testPathStateComps_PowerCap_1:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testPayloadStateComps_PowerCap_1))
	case testPathCompEPs_PowerCap_1:
		fallthrough
	case testPathCompEPs_PowerCap_1_var:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(genPayload(testPayloadCompEPs_PowerCap_1)))
	default:
		msg := "{\"Message\":\"Bad URL " + req.URL.RequestURI() + "\"}"
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(msg))
	}
}

// Used for modifying fake HSM data to aim power capping operations at our fake redfish server
func genPayload(payload string) string {
	pnew := strings.Replace(payload, "%FQDN%", RFServerUrl, -1)
	return pnew
}

// Fake redfish http server
func rfHandler(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		switch req.URL.RequestURI() {
		case testPathRF_x0c0s0b0_Power:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(testPayloadRF_x0c0s0b0_Power))
		case testPathRF_x0c0s1b0_Power:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(testPayloadRF_x0c0s1b0_Power))
		default:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"Message":"You suck at GETs"}`))
		}
	case http.MethodPatch:
		fallthrough
	case http.MethodPost:
		switch req.URL.RequestURI() {
		case testPathRF_x0c0s0b0_PatchPower_Success:
			fallthrough
		case testPathRF_x0c0s1b0_PatchPower_Success:
			fallthrough
		case testPathRF_PatchPower_Success:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"Message":"Success"}`))
		default:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"Message":"You suck at POST/PATCH"}`))
		}
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"Message":"You just plain suck"}`))
	}
}

/////////////////////////
// Test Data for TestDoPowerCapTask()
/////////////////////////

var expectedHostLimitMaxComp1Resp1 int = 850
var expectedHostLimitMinComp1Resp1 int = 350
var expectedPowerupPowerComp1Resp1 int = 250

var expectedCurrentValue1Comp1Resp1 int = 500
var expectedMaximumValue1Comp1Resp1 int = 850
var expectedMinimumValue1Comp1Resp1 int = 350
var expectedCurrentValue2Comp1Resp1 int = 300
var expectedMaximumValue2Comp1Resp1 int = 350
var expectedMinimumValue2Comp1Resp1 int = 200

var expectedPowerCapTaskResp1 = model.PowerCapTaskResp{
	Type:       model.PowerCapTaskTypeSnapshot,
	TaskStatus: model.PowerCapTaskStatusCompleted,
	TaskCounts: model.PowerCapTaskCounts{
		Total:       2,
		New:         0,
		InProgress:  0,
		Failed:      0,
		Succeeded:   2,
		Unsupported: 0,
	},
	Components: []model.PowerCapComponent{{
		Xname: "x0c0s0b0n0",
		Error: "",
		Limits: &model.PowerCapabilities{
			HostLimitMax: &expectedHostLimitMaxComp1Resp1,
			HostLimitMin: &expectedHostLimitMinComp1Resp1,
			PowerupPower: &expectedPowerupPowerComp1Resp1,
		},
		PowerCapLimits: []model.PowerCapControls{{
			Name:         "Node Power Control",
			CurrentValue: &expectedCurrentValue1Comp1Resp1,
			MaximumValue: &expectedMaximumValue1Comp1Resp1,
			MinimumValue: &expectedMinimumValue1Comp1Resp1,
		}, {
			Name:         "Accelerator0 Power Control",
			CurrentValue: &expectedCurrentValue2Comp1Resp1,
			MaximumValue: &expectedMaximumValue2Comp1Resp1,
			MinimumValue: &expectedMinimumValue2Comp1Resp1,
		}},
	}, {
		Xname: "x0c0s1b0n0",
		Error: "",
		Limits: &model.PowerCapabilities{
			HostLimitMax: &expectedHostLimitMaxComp1Resp1,
			HostLimitMin: &expectedHostLimitMinComp1Resp1,
			PowerupPower: &expectedPowerupPowerComp1Resp1,
		},
		PowerCapLimits: []model.PowerCapControls{{
			Name:         "Node Power Control",
			CurrentValue: &expectedCurrentValue1Comp1Resp1,
			MaximumValue: &expectedMaximumValue1Comp1Resp1,
			MinimumValue: &expectedMinimumValue1Comp1Resp1,
		}, {
			Name:         "Accelerator0 Power Control",
			CurrentValue: &expectedCurrentValue2Comp1Resp1,
			MaximumValue: &expectedMaximumValue2Comp1Resp1,
			MinimumValue: &expectedMinimumValue2Comp1Resp1,
		}},
	}},
}

var expectedCurrentValue1Comp1Resp2 int = 450
var expectedMaximumValue1Comp1Resp2 int = 850
var expectedMinimumValue1Comp1Resp2 int = 350
var expectedCurrentValue2Comp1Resp2 int = 300
var expectedMaximumValue2Comp1Resp2 int = 350
var expectedMinimumValue2Comp1Resp2 int = 200
var expectedCurrentValue1Comp2Resp2 int = 500

var expectedPowerCapTaskResp2 = model.PowerCapTaskResp{
	Type:       model.PowerCapTaskTypePatch,
	TaskStatus: model.PowerCapTaskStatusCompleted,
	TaskCounts: model.PowerCapTaskCounts{
		Total:       2,
		New:         0,
		InProgress:  0,
		Failed:      0,
		Succeeded:   2,
		Unsupported: 0,
	},
	Components: []model.PowerCapComponent{{
		Xname: "x0c0s0b0n0",
		Error: "",
		PowerCapLimits: []model.PowerCapControls{{
			Name:         "Node Power Control",
			CurrentValue: &expectedCurrentValue1Comp1Resp2,
			MaximumValue: &expectedMaximumValue1Comp1Resp2,
			MinimumValue: &expectedMinimumValue1Comp1Resp2,
		}, {
			Name:         "Accelerator0 Power Control",
			CurrentValue: &expectedCurrentValue2Comp1Resp2,
			MaximumValue: &expectedMaximumValue2Comp1Resp2,
			MinimumValue: &expectedMinimumValue2Comp1Resp2,
		}},
	}, {
		Xname: "x0c0s1b0n0",
		Error: "",
		PowerCapLimits: []model.PowerCapControls{{
			Name:         "Node Power Control",
			CurrentValue: &expectedCurrentValue1Comp2Resp2,
			MaximumValue: &expectedMaximumValue1Comp1Resp2,
			MinimumValue: &expectedMinimumValue1Comp1Resp2,
		}},
	}},
}

/////////////////////////
// Fake HSM Test Vars
// NOTE: Add '%FQDN%' in place of any FQDN in the fake data so it will get
//       replaced with the URL of our fake redfish server.
/////////////////////////

var testPathStateComps_PowerCap_1 = "/hsm/v2/State/Components/Query"

var testPayloadStateComps_PowerCap_1 = `{
    "Components": [
        {
            "ID": "x0c0s0b0n0",
            "Type": "Node",
            "State": "Ready",
            "Flag": "OK",
            "Enabled": true,
            "Role": "Compute",
            "NID": 100006,
            "NetType": "Sling",
            "Arch": "X86",
            "Class": "River"
        },
        {
            "ID": "x0c0s1b0n0",
            "Type": "Node",
            "State": "Ready",
            "Flag": "OK",
            "Enabled": true,
            "Role": "Compute",
            "NID": 100007,
            "NetType": "Sling",
            "Arch": "X86",
            "Class": "River"
        }
    ]
}
`

var testPathCompEPs_PowerCap_1 = "/hsm/v2/Inventory/ComponentEndpoints?id=x0c0s0b0n0&id=x0c0s1b0n0"
var testPathCompEPs_PowerCap_1_var = "/hsm/v2/Inventory/ComponentEndpoints?id=x0c0s1b0n0&id=x0c0s0b0n0"

var testPayloadCompEPs_PowerCap_1 = `{
    "ComponentEndpoints": [
        {
            "ID": "x0c0s0b0n0",
            "Type": "Node",
            "RedfishType": "ComputerSystem",
            "RedfishSubtype": "Physical",
            "OdataID": "/x0c0s0b0/redfish/v1/Systems/Node0",
            "RedfishEndpointID": "x0c0s0b0",
            "Enabled": true,
            "RedfishEndpointFQDN": "%FQDN%",
            "RedfishURL": "%FQDN%/x0c0s0b0/redfish/v1/Systems/Node0",
            "ComponentEndpointType": "ComponentEndpointComputerSystem",
            "RedfishSystemInfo": {
                "Name": "Node0",
                "Actions": {
                    "#ComputerSystem.Reset": {
                        "ResetType@Redfish.AllowableValues": [
                            "Off",
                            "ForceOff",
                            "On"
                        ],
                        "@Redfish.ActionInfo": "/x0c0s0b0/redfish/v1/Systems/Node0/ResetActionInfo",
                        "target": "/x0c0s0b0/redfish/v1/Systems/Node0/Actions/ComputerSystem.Reset"
                    }
                },
                "PowerURL": "/x0c0s0b0/redfish/v1/Chassis/Node0/Power",
                "PowerControl": [
                    {
                        "@odata.id": "/x0c0s0b0/redfish/v1/Chassis/Node0/Power#/PowerControl/Node",
                        "MemberId": "0",
                        "Name": "Node Power Control",
                        "PowerCapacityWatts": 900,
                        "OEM": {
                            "Cray": {
                                "PowerIdleWatts": 250,
                                "PowerLimit": {
                                    "Min": 350,
                                    "Max": 850
                                },
                                "PowerResetWatts": 250
                            }
                        }
                    },
                    {
                        "@odata.id": "/x0c0s0b0/redfish/v1/Chassis/Node0/Power#/PowerControl/Accelerator0",
                        "Name": "Accelerator0 Power Control",
                        "Oem": {
                            "Cray": {
                                "PowerIdleWatts": 100,
                                "PowerLimit": {
                                    "Min": 200,
                                    "Max": 350
                                }
                            }
                        }
                    }
                ]
            }
        },
        {
            "ID": "x0c0s1b0n0",
            "Type": "Node",
            "RedfishType": "ComputerSystem",
            "RedfishSubtype": "Physical",
            "OdataID": "/x0c0s1b0/redfish/v1/Systems/Node0",
            "RedfishEndpointID": "x0c0s1b0",
            "Enabled": true,
            "RedfishEndpointFQDN": "%FQDN%",
            "RedfishURL": "%FQDN%/x0c0s1b0/redfish/v1/Systems/Node0",
            "ComponentEndpointType": "ComponentEndpointComputerSystem",
            "RedfishSystemInfo": {
                "Name": "Node0",
                "Actions": {
                    "#ComputerSystem.Reset": {
                        "ResetType@Redfish.AllowableValues": [
                            "Off",
                            "ForceOff",
                            "On"
                        ],
                        "@Redfish.ActionInfo": "/x0c0s1b0/redfish/v1/Systems/Node0/ResetActionInfo",
                        "target": "/x0c0s1b0/redfish/v1/Systems/Node0/Actions/ComputerSystem.Reset"
                    }
                },
                "PowerURL": "/x0c0s1b0/redfish/v1/Chassis/Node0/Power",
                "PowerControl": [
                    {
                        "@odata.id": "/x0c0s1b0/redfish/v1/Chassis/Node0/Power#/PowerControl/Node",
                        "MemberId": "0",
                        "Name": "Node Power Control",
                        "PowerCapacityWatts": 900,
                        "OEM": {
                            "Cray": {
                                "PowerIdleWatts": 250,
                                "PowerLimit": {
                                    "Min": 350,
                                    "Max": 850
                                },
                                "PowerResetWatts": 250
                            }
                        }
                    },
                    {
                        "@odata.id": "/x0c0s1b0/redfish/v1/Chassis/Node0/Power#/PowerControl/Accelerator0",
                        "Name": "Accelerator0 Power Control",
                        "Oem": {
                            "Cray": {
                                "PowerIdleWatts": 100,
                                "PowerLimit": {
                                    "Min": 200,
                                    "Max": 350
                                }
                            }
                        }
                    }
                ]
            }
        }
    ]
}
`

/////////////////////////
// Fake Redfish Test Vars
/////////////////////////

var testPathRF_x0c0s0b0_Power = "/x0c0s0b0/redfish/v1/Chassis/Node0/Power"

var testPayloadRF_x0c0s0b0_Power = `
{
    "@odata.context": "/x0c0s0b0/redfish/v1/$metadata#Power.Power(Voltages,Id,Voltages@odata.count,Name,Description)",
    "@odata.etag": "W/\"1569785935\"",
    "@odata.id": "/x0c0s0b0/redfish/v1/Chassis/Node0/Power",
    "@odata.type": "#Power.v1_4_0.Power",
    "Description": "Power sensor readings",
    "Id": "Power",
    "Name": "Power",
    "PowerControl": [
        {
            "RelatedItem@odata.count": 1,
            "PowerCapacityWatts": 900,
            "Name": "Node Power Control",
            "Oem": {
                "Cray": {
                    "PowerAllocatedWatts": 900,
                    "PowerIdleWatts": 250,
                    "PowerLimit": {
                        "Min": 350,
                        "Max": 850,
                        "Factor": 1.02
                    },
                    "PowerFloorTargetWatts": 0,
                    "PowerResetWatts": 250
                }
            },
            "@odata.id": "/x0c0s0b0/redfish/v1/Chassis/Node0/Power#/PowerControl/Node",
            "PowerLimit": {
                "LimitException": "LogEventOnly",
                "CorrectionInMs": 6000,
                "LimitInWatts": 500
            },
            "RelatedItem": [
                {
                    "@odata.id": "/x0c0s0b0/redfish/v1/Chassis/Node0/Power#/PowerControl/Accelerator0"
                }
            ]
        },
        {
            "RelatedItem@odata.count": 0,
            "Name": "Accelerator0 Power Control",
            "Oem": {
                "Cray": {
                    "PowerIdleWatts": 100,
                    "PowerLimit": {
                        "Min": 200,
                        "Max": 350,
                        "Factor": 1.0
                    },
                    "PowerFloorTargetWatts": 0
                }
            },
            "@odata.id": "/x0c0s0b0/redfish/v1/Chassis/Node0/Power#/PowerControl/Accelerator0",
            "PowerLimit": {
                "LimitException": "LogEventOnly",
                "CorrectionInMs": 6000,
                "LimitInWatts": 300
            }
        }
    ]
}
`

var testPathRF_x0c0s1b0_Power = "/x0c0s1b0/redfish/v1/Chassis/Node0/Power"

var testPayloadRF_x0c0s1b0_Power = `
{
    "@odata.context": "/x0c0s1b0/redfish/v1/$metadata#Power.Power(Voltages,Id,Voltages@odata.count,Name,Description)",
    "@odata.etag": "W/\"1569785935\"",
    "@odata.id": "/x0c0s1b0/redfish/v1/Chassis/Node0/Power",
    "@odata.type": "#Power.v1_4_0.Power",
    "Description": "Power sensor readings",
    "Id": "Power",
    "Name": "Power",
    "PowerControl": [
        {
            "RelatedItem@odata.count": 1,
            "PowerCapacityWatts": 900,
            "Name": "Node Power Control",
            "Oem": {
                "Cray": {
                    "PowerAllocatedWatts": 900,
                    "PowerIdleWatts": 250,
                    "PowerLimit": {
                        "Min": 350,
                        "Max": 850,
                        "Factor": 1.02
                    },
                    "PowerFloorTargetWatts": 0,
                    "PowerResetWatts": 250
                }
            },
            "@odata.id": "/x0c0s1b0/redfish/v1/Chassis/Node0/Power#/PowerControl/Node",
            "PowerLimit": {
                "LimitException": "LogEventOnly",
                "CorrectionInMs": 6000,
                "LimitInWatts": 500
            },
            "RelatedItem": [
                {
                    "@odata.id": "/x0c0s1b0/redfish/v1/Chassis/Node0/Power#/PowerControl/Accelerator0"
                }
            ]
        },
        {
            "RelatedItem@odata.count": 0,
            "Name": "Accelerator0 Power Control",
            "Oem": {
                "Cray": {
                    "PowerIdleWatts": 100,
                    "PowerLimit": {
                        "Min": 200,
                        "Max": 350,
                        "Factor": 1.0
                    },
                    "PowerFloorTargetWatts": 0
                }
            },
            "@odata.id": "/x0c0s1b0/redfish/v1/Chassis/Node0/Power#/PowerControl/Accelerator0",
            "PowerLimit": {
                "LimitException": "LogEventOnly",
                "CorrectionInMs": 6000,
                "LimitInWatts": 300
            }
        }
    ]
}
`

var testPathRF_x0c0s0b0_PatchPower_Success = "/x0c0s0b0/redfish/v1/Chassis/Node0/Power"
var testPathRF_x0c0s1b0_PatchPower_Success = "/x0c0s0b0/redfish/v1/Chassis/Node0/Power"
var testPathRF_PatchPower_Success = "/redfish/v1/Chassis/1/Power/AccPowerService/PowerLimit/Actions/HpeServerAccPowerLimit.ConfigurePowerLimit"
