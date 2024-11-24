/*
 * (C) Copyright [2021-2024] Hewlett Packard Enterprise Development LP
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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"

	base "github.com/Cray-HPE/hms-base"
	"github.com/Cray-HPE/hms-power-control/internal/hsm"
	"github.com/Cray-HPE/hms-power-control/internal/logger"
	"github.com/Cray-HPE/hms-power-control/internal/model"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

///////////////////////////
// Redfish Structs
///////////////////////////

type Power struct {
	OContext string `json:"@odata.context,omitempty"`
	Oetag    string `json:"@odata.etag,omitempty"`
	Oid      string `json:"@odata.id,omitempty"`
	Otype    string `json:"@odata.type,omitempty"`
	Id       string `json:"Id,omitempty"`
	Name     string `json:"Name,omitempty"`

	// Redfish Power.v1_5_4.json
	Description string         `json:"Description,omitempty"`
	PowerCtl    []PowerControl `json:"PowerControl,omitempty"`
	PowerCtlCnt int            `json:"PowerControl@odata.count,omitempty"`

	// HpeServerAccPowerLimit.v1_0_0.HpeServerAccPowerLimit
	Error             *HpeError              `json:"error,omitempty"`
	ActualPowerLimits []HpeActualPowerLimits `json:"ActualPowerLimits,omitempty"`
	PowerLimitRanges  []HpePowerLimitRanges  `json:"PowerLimitRanges,omitempty"`
	PowerLimits       []HpePowerLimits       `json:"PowerLimits,omitempty"`

	// Redfish Control.v1_0_0.Control
	RFControl
}

type RFControlsDeep struct {
	Members []RFControl `json:"Members"`
}

// Structs used to [un]marshal HPE Redfish
// HpeServerAccPowerLimit.v1_0_0.HpeServerAccPowerLimit data
type HpeConfigurePowerLimit struct {
	PowerLimits []HpePowerLimits `json:"PowerLimits"`
}

type HpeActualPowerLimits struct {
	PowerLimitInWatts *int `json:"PowerLimitInWatts"`
	ZoneNumber        *int `json:"ZoneNumber"`
}

type HpePowerLimitRanges struct {
	MaximumPowerLimit *int `json:"MaximumPowerLimit"`
	MinimumPowerLimit *int `json:"MinimumPowerLimit"`
	ZoneNumber        *int `json:"ZoneNumber"`
}

type HpePowerLimits struct {
	PowerLimitInWatts *int `json:"PowerLimitInWatts"`
	ZoneNumber        *int `json:"ZoneNumber"`
}

type HpeError struct {
	Code         string                   `json:"code"`
	Message      string                   `json:"message"`
	ExtendedInfo []HpeMessageExtendedInfo `json:"@Message.ExtendedInfo"`
}

type HpeMessageExtendedInfo struct {
	MessageId string `json:"MessageId"`
}

// PowerControl struct used to unmarshal the Redfish Power.v1_5_4 data
type PowerControl struct {
	Oid                 string           `json:"@odata.id,omitempty"`
	Name                string           `json:"Name,omitempty"`
	OEM                 *PowerControlOEM `json:"Oem,omitempty"`
	PowerAllocatedWatts *int             `json:"PowerAllocatedWatts,omitempty"`
	PowerAvailableWatts *int             `json:"PowerAvailableWatts,omitempty"`
	PowerCapacityWatts  *int             `json:"PowerCapacityWatts,omitempty"`
	PowerConsumedWatts  *interface{}     `json:"PowerConsumedWatts,omitempty"`
	PowerLimit          *PowerLimit      `json:"PowerLimit,omitempty"`
	PowerMetrics        *PowerMetric     `json:"PowerMetrics,omitempty"`
	PowerRequestedWatts *int             `json:"PowerRequestedWatts,omitempty"`
}

// StatusRF struct used to unmarshal health info from Redfish Control.v1_0_0
type StatusRF struct {
	Health       string `json:"Health,omitempty"`
	HealthRollUp string `json:"HealthRollUp,omitempty"`
	State        string `json:"State,omitempty"`
}

// RFControl struct used to unmarshal the Redfish Control.v1_0_0.Control data
type RFControl struct {
	Oid                 string    `json:"@odata.id,omitempty"`
	ControlDelaySeconds *int      `json:"ControlDelaySeconds,omitempty"`
	ControlMode         string    `json:"ControlMode,omitempty"`
	ControlType         string    `json:"ControlType,omitempty"`
	Id                  string    `json:"Id,omitempty"`
	Name                string    `json:"Name,omitempty"`
	PhysicalContext     string    `json:"PhysicalContext,omitempty"`
	SetPoint            *int      `json:"SetPoint,omitempty"`
	SetPointUnits       string    `json:"SetPointUnits,omitempty"`
	SettingRangeMax     *int      `json:"SettingRangeMax,omitempty"`
	SettingRangeMin     *int      `json:"SettingRangeMin,omitempty"`
	Status              *StatusRF `json:"Status,omitempty"`
}

// PowerControlOEM contains a pointer to the OEM specific information
type PowerControlOEM struct {
	Cray *PowerControlOEMCray `json:"Cray,omitempty"`
}

// PowerControlOEMCray describes the Mountain specific power information
type PowerControlOEMCray struct {
	PowerAllocatedWatts   *int               `json:"PowerAllocatedWatts,omitempty"`
	PowerIdleWatts        *int               `json:"PowerIdleWatts,omitempty"`
	PowerLimit            *PowerLimitOEMCray `json:"PowerLimit,omitempty"`
	PowerFloorTargetWatts *int               `json:"PowerFloorTargetWatts,omitempty"`
	PowerResetWatts       *int               `json:"PowerResetWatts,omitempty"`
}

// PowerLimitOEMCray describes the power limit status and configuration for
// Mountain nodes
type PowerLimitOEMCray struct {
	Min    *int     `json:"Min,omitempty"`
	Max    *int     `json:"Max,omitempty"`
	Factor *float32 `json:"Factor,omitempty"`
}

// PowerLimit describes the power limit status and configuration for a
// compute module
type PowerLimit struct {
	CorrectionInMs *int   `json:"CorrectionInMs,omitempty"`
	LimitException string `json:"LimitException,omitempty"`
	LimitInWatts   *int   `json:"LimitInWatts"`
}

// PowerMetric describes the power readings for the compute module
type PowerMetric struct {
	AverageConsumedWatts *int `json:"AverageConsumedWatts,omitempty"`
	IntervalInMin        *int `json:"IntervalInMin,omitempty"`
	MaxConsumedWatts     *int `json:"MaxConsumedWatts,omitempty"`
	MinConsumedWatts     *int `json:"MinConsumedWatts,omitempty"`
}

///////////////////////////
// Power Capping Domain Functions
///////////////////////////

// Start a power cap snapshot task
func SnapshotPowerCap(parameters model.PowerCapSnapshotParameter) (pb model.Passback) {

	// Create task
	task := model.NewPowerCapSnapshotTask(parameters, GLOB.ExpireTimeMins)

	// Store task
	err := (*GLOB.DSP).StorePowerCapTask(task)
	if err != nil {
		pb = model.BuildErrorPassback(http.StatusInternalServerError, err)
		logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Error storing new power capping task")
		return
	}

	// Start task
	go doPowerCapTask(task.TaskID)

	rsp := model.PowerCapTaskCreation{
		TaskID: task.TaskID,
	}
	pb = model.BuildSuccessPassback(http.StatusOK, rsp)
	return
}

// Start a power cap patch task for setting power limits for nodes.
func PatchPowerCap(parameters model.PowerCapPatchParameter) (pb model.Passback) {
	// Create task
	task := model.NewPowerCapPatchTask(parameters, GLOB.ExpireTimeMins)

	// Store task
	err := (*GLOB.DSP).StorePowerCapTask(task)
	if err != nil {
		pb = model.BuildErrorPassback(http.StatusInternalServerError, err)
		logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Error storing new power capping task")
		return
	}

	// Start task
	go doPowerCapTask(task.TaskID)

	rsp := model.PowerCapTaskCreation{
		TaskID: task.TaskID,
	}
	pb = model.BuildSuccessPassback(http.StatusOK, rsp)
	return
}

// Get all of the existing power capping tasks
func GetPowerCap() (pb model.Passback) {
	// Get all tasks
	tasks, err := (*GLOB.DSP).GetAllPowerCapTasks()
	if err != nil {
		pb = model.BuildErrorPassback(http.StatusInternalServerError, err)
		logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Error retrieving all power cap tasks")
		return
	}

	rsp := model.PowerCapTaskRespArray{
		Tasks: []model.PowerCapTaskResp{},
	}
	// Get the operations for each task
	for _, task := range tasks {
		var ops []model.PowerCapOperation
		// Compressed tasks don't have operations anymore. No need to look them up.
		if !task.IsCompressed {
			ops, err = (*GLOB.DSP).GetAllPowerCapOperationsForTask(task.TaskID)
			if err != nil {
				pb = model.BuildErrorPassback(http.StatusInternalServerError, err)
				logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Error retrieving power cap operations")
				return
			}
		}
		// Build the response struct
		taskRsp := buildPowerCapResponse(task, ops, false)
		rsp.Tasks = append(rsp.Tasks, taskRsp)
	}

	pb = model.BuildSuccessPassback(http.StatusOK, rsp)
	return
}

// Get a specific power capping task
func GetPowerCapQuery(taskID uuid.UUID) (pb model.Passback) {
	var ops []model.PowerCapOperation
	// Get the task
	task, err := (*GLOB.DSP).GetPowerCapTask(taskID)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			pb = model.BuildErrorPassback(http.StatusNotFound, err)
		} else {
			pb = model.BuildErrorPassback(http.StatusInternalServerError, err)
		}
		logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Error retrieving power cap task")
		return
	}
	if task.TaskID.String() != taskID.String() {
		err := errors.New("TaskID does not exist")
		pb = model.BuildErrorPassback(http.StatusNotFound, err)
		logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Error retrieving power cap task")
	}
	// Compressed tasks don't have operations anymore. No need to look them up.
	if !task.IsCompressed {
		// Get the operations for the task
		ops, err = (*GLOB.DSP).GetAllPowerCapOperationsForTask(taskID)
		if err != nil {
			pb = model.BuildErrorPassback(http.StatusInternalServerError, err)
			logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Error retrieving power cap operations")
			return
		}
	}

	// Build the response struct
	rsp := buildPowerCapResponse(task, ops, true)

	pb = model.BuildSuccessPassback(http.StatusOK, rsp)
	return
}

///////////////////////////
// Non-exported functions (helpers, utils, etc)
///////////////////////////

// Assembles a PowerCapTaskResp struct from a task and an array of its operations.
// If 'full' == true, full operation information is included (power capping limits,
// capabilities, errors, etc).
func buildPowerCapResponse(task model.PowerCapTask, ops []model.PowerCapOperation, full bool) model.PowerCapTaskResp {
	// Build the response struct
	rsp := model.PowerCapTaskResp{
		TaskID:                  task.TaskID,
		Type:                    task.Type,
		TaskCreateTime:          task.TaskCreateTime,
		AutomaticExpirationTime: task.AutomaticExpirationTime,
		TaskStatus:              task.TaskStatus,
	}

	// Is a compressed record
	if task.IsCompressed {
		rsp.TaskCounts = task.TaskCounts
		if full {
			rsp.Components = task.Components
		}
		return rsp
	}

	counts := model.PowerCapTaskCounts{}
	componentMap := make(map[string]model.PowerCapComponent)
	for _, op := range ops {
		// Get the count of operations with each status type.
		switch op.Status {
		case model.PowerCapOpStatusNew:
			counts.New++
		case model.PowerCapOpStatusInProgress:
			counts.InProgress++
		case model.PowerCapOpStatusFailed:
			counts.Failed++
		case model.PowerCapOpStatusSucceeded:
			counts.Succeeded++
		case model.PowerCapOpStatusUnsupported:
			counts.Unsupported++
		}
		counts.Total++
		// Include information about individual operations if full == true
		if full {
			if comp, ok := componentMap[op.Component.Xname]; !ok {
				componentMap[op.Component.Xname] = op.Component
			} else {
				if op.Component.Error != "" {
					if comp.Error != "" {
						comp.Error += "; "
					}
					comp.Error += op.Component.Error
				} else {
					if op.Component.Limits != nil && comp.Limits != nil {
						comp.Limits = op.Component.Limits
					}
					if len(op.Component.PowerCapLimits) > 0 {
						comp.PowerCapLimits = append(comp.PowerCapLimits, op.Component.PowerCapLimits...)
					}
				}
				componentMap[op.Component.Xname] = comp
			}
		}
	}
	rsp.TaskCounts = counts
	// Include information about individual operations if full == true
	if full {
		for _, comp := range componentMap {
			rsp.Components = append(rsp.Components, comp)
		}
	}
	return rsp
}

// Main worker function for executing power capping tasks.
func doPowerCapTask(taskID uuid.UUID) {
	var err error
	var taskIsPatch bool
	var xnames []string
	goodOps := make([]model.PowerCapOperation, 0)
	patchParametersMap := make(map[string]map[string]int)
	fname := "doPowerCapTask"

	task, err := (*GLOB.DSP).GetPowerCapTask(taskID)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Cannot retrieve power capping task, cannot generate operations")
		return
	}

	defer logger.Log.Infof("Power Capping Task %s Completed", task.TaskID.String())

	if task.Type == model.PowerCapTaskTypePatch {
		logger.Log.Infof("Starting Power Capping Patch Task %s - %v", task.TaskID.String(), *task.PatchParameters)
		taskIsPatch = true
		for _, param := range task.PatchParameters.Components {
			xnames = append(xnames, param.Xname)
			ctlMap, ok := patchParametersMap[param.Xname]
			if !ok {
				ctlMap = make(map[string]int)
			}
			for _, control := range param.Controls {
				ctlMap[control.Name] = control.Value
			}
			patchParametersMap[param.Xname] = ctlMap
		}
	} else {
		logger.Log.Infof("Starting Power Capping Snapshot Task %s - %v", task.TaskID.String(), *task.SnapshotParameters)
		xnames = task.SnapshotParameters.Xnames
	}

	// Clean the xname list of invalid xnames
	goodXnames, badXnames := base.ValidateCompIDs(xnames, true)
	if len(badXnames) > 0 {
		errormsg := "Invalid xnames detected "
		for _, badxname := range badXnames {
			errormsg += badxname + " "
		}
		err := errors.New(errormsg)
		logrus.WithFields(logrus.Fields{"ERROR": err, "xnames": badXnames}).Error("Invalid xnames detected")
		for _, id := range badXnames {
			// Create failures for each invalid xname
			op := model.NewPowerCapOperation(task.TaskID, task.Type)
			op.Status = model.PowerCapOpStatusFailed
			op.Component.Xname = id
			op.Component.Error = "Invalid xname"
			task.OperationIDs = append(task.OperationIDs, op.OperationID)
			err = (*GLOB.DSP).StorePowerCapOperation(op)
			if err != nil {
				logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing power capping operation")
			}
		}
	}
	if len(goodXnames) == 0 {
		// All xnames were invalid
		err = errors.New("No xnames to operate on")
		logrus.WithFields(logrus.Fields{"ERROR": err}).Error("No xnames to operate on")
		compressAndCompleteTask(task)
		return
	}
	// Call again to remove any duplicates
	betterXnames, _ := base.ValidateCompIDs(goodXnames, false)

	// Verify xnames exist in HSM
	hsmData, err := (*GLOB.HSM).FillHSMData(betterXnames)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error retrieving HSM data")
		// Failed to get data from HSM. Fail everything.
		for _, id := range betterXnames {
			op := model.NewPowerCapOperation(task.TaskID, task.Type)
			op.Status = model.PowerCapOpStatusFailed
			op.Component.Xname = id
			op.Component.Error = "Error retrieving HSM data"
			task.OperationIDs = append(task.OperationIDs, op.OperationID)
			err = (*GLOB.DSP).StorePowerCapOperation(op)
			if err != nil {
				logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing power capping operation")
			}
		}
	} else {
		// Check to see if we got everything back.
		if len(hsmData) != len(betterXnames) {
			for _, id := range betterXnames {
				if _, ok := hsmData[id]; !ok {
					// This xname was not found in the response from HSM. Add a failed "Not found" operation for it.
					op := model.NewPowerCapOperation(task.TaskID, task.Type)
					op.Status = model.PowerCapOpStatusFailed
					op.Component.Xname = id
					op.Component.Error = "Xname not found in HSM"
					task.OperationIDs = append(task.OperationIDs, op.OperationID)
					err = (*GLOB.DSP).StorePowerCapOperation(op)
					if err != nil {
						logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing power capping operation")
					}
				}
			}
		}
	}

	// Generate operations
	for id, comp := range hsmData {
		if taskIsPatch {
			if ctlMap, ok := patchParametersMap[id]; !ok {
				// Shouldn't happen because all xnames from HSM SHOULD be in the patchParametersMap
				continue
			} else {
				// Check that the power control that we want to affect exists in HSM.
				// We won't operate on it if it doesn't.
				found := false
				for name, _ := range ctlMap {
					if _, ok := comp.PowerCaps[name]; !ok {
						op := model.NewPowerCapOperation(task.TaskID, task.Type)
						op.Component.Xname = id
						op.Status = model.PowerCapOpStatusFailed
						op.Component.Error = "Skipping undefined control '" + name + "'"
						task.OperationIDs = append(task.OperationIDs, op.OperationID)
						err = (*GLOB.DSP).StorePowerCapOperation(op)
						if err != nil {
							logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing power capping operation")
						}
					} else {
						found = true
					}
				}
				if !found {
					// None of the requested power controls for this component are defined in HSM.
					// Move on to the next component.
					continue
				}
			}
		}
		tempOps := []model.PowerCapOperation{}
		if comp.PowerCapControlsCount > 0 {
			// When a component is using the Controls schema, it is because each available power control
			// is located at a different URL. Make an operation for each URL.
			for name, pwrCtl := range comp.PowerCaps {
				if taskIsPatch {
					if _, ok := patchParametersMap[id][name]; !ok {
						// This power control isn't in our request. Skip it.
						continue
					}
				}
				op := model.NewPowerCapOperation(task.TaskID, task.Type)
				op.PowerCapURI = pwrCtl.Path

				if taskIsPatch {
					// Use Controls.Deep URL for patching Cray EX hardware.  To get there, need to
					// go up two levels in the path from ../Controls/NodePowerLimit in order to
					// replace Controls with Controls.Deep
					url := path.Dir(path.Dir(op.PowerCapURI))
					op.PowerCapURI = url + "/Controls.Deep"

					// For a patch we only care about Controls.Deep so only need one op.  We came into this
					// loop only to pick up the first pwrCtl.Path to form the /Controls.Deep URL
					op.PowerCaps = comp.PowerCaps
					tempOps = append(tempOps, op)
					break
				}

				op.PowerCaps = make(map[string]hsm.PowerCap)
				op.PowerCaps[name] = pwrCtl
				tempOps = append(tempOps, op)
			}
		} else {
			op := model.NewPowerCapOperation(task.TaskID, task.Type)
			op.PowerCapURI = comp.PowerCapURI
			op.PowerCaps = comp.PowerCaps
			tempOps = append(tempOps, op)
		}

		// Validate that we have the required HSM data for each operation.
		for _, op := range tempOps {
			op.Component.Xname = id
			isError := true
			// Validate the HSM data
			if comp.RfFQDN == "" {
				op.Status = model.PowerCapOpStatusFailed
				op.Component.Error = "Missing RfFQDN"
			} else if comp.BaseData.Type != base.Node.String() {
				// We only support node power capping
				op.Status = model.PowerCapOpStatusUnsupported
				op.Component.Error = "Type, " + comp.BaseData.Type + " unsupported for power capping"
			} else if op.PowerCapURI == "" {
				op.Status = model.PowerCapOpStatusFailed
				op.Component.Error = "Missing Power Cap URI"
			} else if comp.BaseData.Role == base.RoleManagement.String() {
				// Power capping Management nodes is dangerous to the system. Lets not.
				op.Status = model.PowerCapOpStatusUnsupported
				op.Component.Error = "Power capping Management nodes is not supported"
			} else if taskIsPatch && isHpeApollo6500(op.PowerCapURI) && comp.PowerCapTargetURI == "" {
				// Apollo 6500's use a separate URL target for setting
				// power limits. We need PowerCapTargetURI from HSM.
				op.Status = model.PowerCapOpStatusFailed
				op.Component.Error = "Missing Power Cap Target URI"
			} else {
				if taskIsPatch {
					// Validate that the desired power limit is within
					// range based on the information we got from HSM.
					for name, pwrCtl := range op.PowerCaps {
						val, ok := patchParametersMap[id][name]
						if !ok {
							// This power control isn't in our request. Skip it.
							continue
						}
						// The value of Zero is used by most vendors as a method of turning off
						// power capping so it is a valid option.
						if pwrCtl.Min != -1 && val != 0 && val < pwrCtl.Min {
							op.Status = model.PowerCapOpStatusFailed
							if op.Component.Error != "" {
								op.Component.Error += "; "
							}
							op.Component.Error += fmt.Sprintf("Control (%s) value (%d) is less than minimum (%d)",
								name, val, pwrCtl.Min)
						} else if pwrCtl.Max != -1 && val > pwrCtl.Max {
							op.Status = model.PowerCapOpStatusFailed
							if op.Component.Error != "" {
								op.Component.Error += "; "
							}
							op.Component.Error += fmt.Sprintf("Control (%s) value (%d) is more than maximum (%d)",
								name, val, pwrCtl.Max)
						} else {
							if _, ok := patchParametersMap[id][name]; !ok {
								// This power control isn't in our request. Skip it.
								continue
							}
							currentVal := val
							min := pwrCtl.Min
							max := pwrCtl.Max
							ctl := model.PowerCapControls{
								Name:         name,
								CurrentValue: &currentVal,
							}
							// -1 means limits were not available to HSM via redfish.
							if min != -1 {
								ctl.MinimumValue = &min
							}
							if max != -1 {
								ctl.MaximumValue = &max
							}
							op.Component.PowerCapLimits = append(op.Component.PowerCapLimits, ctl)
							isError = false
						}
					}
				} else {
					isError = false
				}
			}
			if !isError {
				// Found an operation with all the valid data.
				op.Status = model.PowerCapOpStatusInProgress
				op.RfFQDN = comp.RfFQDN
				op.PowerCapControlsCount = comp.PowerCapControlsCount
				op.PowerCapCtlInfoCount = comp.PowerCapCtlInfoCount
				op.PowerCapTargetURI = comp.PowerCapTargetURI
				goodOps = append(goodOps, op)
			}
			task.OperationIDs = append(task.OperationIDs, op.OperationID)
			err = (*GLOB.DSP).StorePowerCapOperation(op)
			if err != nil {
				logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing power capping operation")
			}
		}
	}

	// Everything failed. We're done here.
	if len(goodOps) == 0 {
		compressAndCompleteTask(task)
		return
	}
	task.TaskStatus = model.PowerCapTaskStatusInProgress
	err = (*GLOB.DSP).StorePowerCapTask(task)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing power capping task")
	}

	// Repeated and frequent power caps to the same BMCs is not
	// common so we use the default TRS configuration provided by the
	// default BaseTRSTask task prototype.  It may be beneficial to
	// consider sharing the PCS TRS client in the future as power capping
	// generally shares the same set of BMC targets we want to talk to.
	//
	// SPC does initiatefrequent repeated http requests to BMCs so when
	// it is added to CSM for formal release, we should consider adding
	// a new client or sharing (and enlarging) the PCS TRS client.

	// Create TRS task list
	trsTaskMap := make(map[uuid.UUID]model.PowerCapOperation)
	trsTaskList := (*GLOB.RFTloc).CreateTaskList(GLOB.BaseTRSTask, len(goodOps))
	trsTaskIdx := 0
	for _, op := range goodOps {
		trsTaskMap[trsTaskList[trsTaskIdx].GetID()] = op
		trsTaskList[trsTaskIdx].CPolicy.Retry.Retries = 3
		if taskIsPatch {
			var method string
			var path string
			if isHpeApollo6500(op.PowerCapURI) {
				// Apollo 6500's use a POST operation to set power limits
				method = "POST"
				path = op.RfFQDN + op.PowerCapTargetURI
			} else {
				method = "PATCH"
				path = op.RfFQDN + op.PowerCapURI
			}
			payload, _ := generatePowerCapPayload(op)
			trsTaskList[trsTaskIdx].Request, _ = http.NewRequest(method, "https://"+path, bytes.NewBuffer(payload))
			trsTaskList[trsTaskIdx].Request.Header.Set("Content-Type", "application/json")
			trsTaskList[trsTaskIdx].Request.Header.Add("HMS-Service", GLOB.BaseTRSTask.ServiceName)
		} else {
			trsTaskList[trsTaskIdx].Request.URL, _ = url.Parse("https://" + op.RfFQDN + op.PowerCapURI)
		}
		// Vault enabled?
		if GLOB.VaultEnabled {
			user, pw, err := (*GLOB.CS).GetControllerCredentials(op.Component.Xname)
			if err != nil {
				logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Unable to get credentials for " + op.Component.Xname)
			} // Retry credentials? Fail operation here? For now, just let it fail with empty credentials
			if !(user == "" && pw == "") {
				trsTaskList[trsTaskIdx].Request.SetBasicAuth(user, pw)
			}
		}
		trsTaskIdx++
	}

	if len(trsTaskList) > 0 {
		logger.Log.Infof("%s: Initiating %d/%d power cap requests to BMCs",
					     fname, trsTaskIdx, len(goodOps))

		rchan, err := (*GLOB.RFTloc).Launch(&trsTaskList)
		if err != nil {
			logrus.Error(err)
		}
		for _, _ = range trsTaskList {
			var taskErr error
			var rfPower Power
			tdone := <-rchan
			op := trsTaskMap[tdone.GetID()]
			for i := 0; i < 1; i++ {

				if *tdone.Err != nil {
					taskErr = *tdone.Err

					// Must always drain and close response bodies even if we don't use them
					if tdone.Request.Response != nil && tdone.Request.Response.Body != nil {
						_, _ = io.Copy(io.Discard, tdone.Request.Response.Body)
						tdone.Request.Response.Body.Close()
					}

					break
				}
				if tdone.Request.Response.StatusCode < 200 && tdone.Request.Response.StatusCode >= 300 {
					taskErr = errors.New("bad status code: " + strconv.Itoa(tdone.Request.Response.StatusCode))

					// Must always drain and close response bodies even if we don't use them
					if tdone.Request.Response != nil && tdone.Request.Response.Body != nil {
						_, _ = io.Copy(io.Discard, tdone.Request.Response.Body)
						tdone.Request.Response.Body.Close()
					}

					break
				}
				if tdone.Request.Response.Body == nil {
					taskErr = errors.New("empty body")
					break
				}
				body, err := io.ReadAll(tdone.Request.Response.Body)

				// Must always close response bodies
				tdone.Request.Response.Body.Close()

				if err != nil {
					taskErr = err
					break
				}

				if !taskIsPatch {
					err = json.Unmarshal(body, &rfPower)
					if err != nil {
						taskErr = err
						break
					}
				}

				// Convert PowerConsumedWatts to an int if not already (it's an interface{}
				// type that can support ints and floats) - Needed for Foxconn Paradise,
				// perhaps others in the future
				for _, pwrCtl := range rfPower.PowerCtl {
					if pwrCtl.PowerConsumedWatts != nil {
						switch v := (*pwrCtl.PowerConsumedWatts).(type) {
						case float64:	// Convert to int
							*pwrCtl.PowerConsumedWatts = int(math.Round(v))
						case int:		// noop - no conversion needed
						default:		// unexpected type, set to zero
							*pwrCtl.PowerConsumedWatts = int(0)
							logger.Log.WithFields(logrus.Fields{"type": reflect.TypeOf(*pwrCtl.PowerConsumedWatts), "value": *pwrCtl.PowerConsumedWatts}).Errorf("Unexpected type/value detected for PowerConsumedWatts, setting to 0\n")
						}
					}
				}
			}
			if taskErr != nil {
				op.Status = model.PowerCapOpStatusFailed
				op.Component.Error = taskErr.Error()
				logger.Log.WithFields(logrus.Fields{"ERROR": taskErr, "URI": tdone.Request.URL.String()}).Error("Redfish request failed")
			} else if taskIsPatch {
				op.Status = model.PowerCapOpStatusSucceeded
			} else {
				op.Component.PowerCapLimits, op.Component.Limits, err = parsePowerCapRFData(op, rfPower)
				if err != nil {
					op.Status = model.PowerCapOpStatusFailed
					op.Component.Error = err.Error()
					logger.Log.WithFields(logrus.Fields{"ERROR": err, "URI": tdone.Request.URL.String()}).Error("Redfish request failed")
				} else {
					op.Status = model.PowerCapOpStatusSucceeded
				}
			}
			err = (*GLOB.DSP).StorePowerCapOperation(op)
			if err != nil {
				logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing power capping operation")
			}

			// Cancel task contexts after they're no longer needed
			if tdone.ContextCancel != nil {
				tdone.ContextCancel()
			}
		}
		(*GLOB.RFTloc).Close(&trsTaskList)
		close(rchan)
		logger.Log.Infof("%s: Done processing BMC responses", fname)
	}

	// Task Complete
	compressAndCompleteTask(task)
	return
}

// Generates a POST/PATCH payload for the specified operation
func generatePowerCapPayload(op model.PowerCapOperation) ([]byte, error) {
	if op.PowerCapControlsCount > 0 {
		// Newer bard peak power capping schema deep patch
		p := RFControlsDeep{
			Members: make([]RFControl, 0, 1),
		}
		for _, limit := range op.Component.PowerCapLimits {
			var ctl RFControl
			if *limit.CurrentValue > 0 {
				ctl = RFControl{
					Oid:         op.PowerCaps[limit.Name].Path,
					SetPoint:    limit.CurrentValue,
					ControlMode: "Automatic",
				}
			} else {
				ctl = RFControl{
					Oid:         op.PowerCaps[limit.Name].Path,
					ControlMode: "Disabled",
				}
			}
			p.Members = append(p.Members, ctl)
		}
		return json.Marshal(p)
	} else if isHpeApollo6500(op.PowerCapURI) {
		// Apollo 6500 redfish POST power capping schema
		zero := 0
		p := HpeConfigurePowerLimit{
			PowerLimits: []HpePowerLimits{
				{
					PowerLimitInWatts: op.Component.PowerCapLimits[0].CurrentValue,
					ZoneNumber:        &zero,
				},
			},
		}
		return json.Marshal(p)
	} else {
		// Generic redfish power capping schema
		p := Power{
			PowerCtl: make([]PowerControl, op.PowerCapCtlInfoCount),
		}
		for _, limit := range op.Component.PowerCapLimits {
			idx := op.PowerCaps[limit.Name].PwrCtlIndex
			pCtl := PowerControl{
				PowerLimit: &PowerLimit{
					LimitInWatts: limit.CurrentValue,
				},
			}
			p.PowerCtl[idx] = pCtl
		}
		return json.Marshal(p)
	}
}

// Parsing function to populate PowerCapLimits and Limits structs for an
// operation based off of the data we got back from redfish.
func parsePowerCapRFData(op model.PowerCapOperation, rfPower Power) ([]model.PowerCapControls, *model.PowerCapabilities, error) {
	var err error
	var controls []model.PowerCapControls
	var limits *model.PowerCapabilities
	if rfPower.Error != nil {
		err = errors.New("Invalid license for power capping")
		return nil, nil, err
	}

	if op.PowerCapControlsCount > 0 {
		var val int
		// Newer Cray-HPE Controls Schema
		if rfPower.SetPoint != nil {
			val = *rfPower.SetPoint
		}
		control := model.PowerCapControls{
			Name:         rfPower.Name,
			CurrentValue: &val,
			MaximumValue: rfPower.SettingRangeMax,
			MinimumValue: rfPower.SettingRangeMin,
		}
		controls = append(controls, control)
		// Use the capabilities from the node's control struct.
		if strings.ToLower(rfPower.PhysicalContext) == "chassis" {
			limits = &model.PowerCapabilities{
				HostLimitMax: rfPower.SettingRangeMax,
				HostLimitMin: rfPower.SettingRangeMin,
			}
		}
	} else if len(rfPower.ActualPowerLimits) > 0 ||
		len(rfPower.PowerLimitRanges) > 0 ||
		len(rfPower.PowerLimits) > 0 {
		// Apollo 6500 AccPowerService
		for i, pl := range rfPower.PowerLimits {
			var min *int
			var max *int
			var val int
			name := "Node Power Limit"

			if pl.PowerLimitInWatts != nil {
				val = *pl.PowerLimitInWatts
			}
			if len(rfPower.PowerLimitRanges) > i {
				max = rfPower.PowerLimitRanges[i].MaximumPowerLimit
				min = rfPower.PowerLimitRanges[i].MinimumPowerLimit
			}
			control := model.PowerCapControls{
				Name:         name,
				CurrentValue: &val,
				MaximumValue: max,
				MinimumValue: min,
			}
			controls = append(controls, control)
			if i == 0 {
				limits = &model.PowerCapabilities{
					HostLimitMax: max,
					HostLimitMin: min,
				}
			}
		}
	} else {
		// Standard Redfish PowerControl
		for i, pc := range rfPower.PowerCtl {
			var name string
			var val int
			if isHpeServer(op.PowerCapURI) {
				// HPE Server
				name = "Node Power Limit"
			} else {
				name = pc.Name
			}
			control := model.PowerCapControls{
				Name: name,
			}

			if pc.PowerLimit != nil && pc.PowerLimit.LimitInWatts != nil {
				val = *pc.PowerLimit.LimitInWatts
			}
			control.CurrentValue = &val
			if pc.OEM != nil && pc.OEM.Cray != nil && pc.OEM.Cray.PowerLimit != nil {
				control.MaximumValue = pc.OEM.Cray.PowerLimit.Max
				control.MinimumValue = pc.OEM.Cray.PowerLimit.Min
			} else {
				control.MaximumValue = pc.PowerCapacityWatts
			}
			controls = append(controls, control)
			if i == 0 {
				var hostLimitMax *int
				var hostLimitMin *int
				var powerupPower *int

				if pc.OEM != nil && pc.OEM.Cray != nil {
					powerupPower = pc.OEM.Cray.PowerResetWatts
					if pc.OEM.Cray.PowerLimit != nil {
						hostLimitMax = pc.OEM.Cray.PowerLimit.Max
						if pc.OEM.Cray.PowerLimit.Min != nil {
							hostLimitMin = pc.OEM.Cray.PowerLimit.Min
						}
					}
				} else {
					hostLimitMax = pc.PowerCapacityWatts
				}
				limits = &model.PowerCapabilities{
					HostLimitMax: hostLimitMax,
					HostLimitMin: hostLimitMin,
					PowerupPower: powerupPower,
				}
			}
		}
	}
	return controls, limits, nil
}

// Determines if the device is an HPE BMC server using the powerURL
func isHpeServer(powerURL string) bool {
	if strings.Contains(powerURL, "Chassis/1/Power") {
		return true
	}

	return false
}

// Determines if the device is an Apollo 6500 using the powerURL
func isHpeApollo6500(powerURL string) bool {
	if strings.Contains(powerURL, "AccPowerService/PowerLimit") {
		return true
	}

	return false
}

// Checks all of the power cap records in etcd and does the following deletes
// records that have expired.
func powerCapReaper() {
	// Get all power cap tasks
	tasks, err := (*GLOB.DSP).GetAllPowerCapTasks()
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error retreiving power cap tasks")
		return
	}
	if len(tasks) == 0 {
		// No power cap tasks to act upon
		return
	}

	numComplete := 0
	for _, task := range tasks {
		expired := task.AutomaticExpirationTime.Before(time.Now())
		if expired {
			// Get the operations for the power cap task
			ops, err := (*GLOB.DSP).GetAllPowerCapOperationsForTask(task.TaskID)
			if err != nil {
				logger.Log.WithFields(logrus.Fields{"ERROR": err}).Errorf("Error retreiving operations for power cap task, %s.", task.TaskID.String())
				continue
			}
			for _, op := range ops {
				err = (*GLOB.DSP).DeletePowerCapOperation(task.TaskID, op.OperationID)
				if err != nil {
					logger.Log.WithFields(logrus.Fields{"ERROR": err}).Errorf("Error deleting operation, %s, for power cap task, %s.", op.OperationID.String(), task.TaskID.String())
				}
			}
			err = (*GLOB.DSP).DeletePowerCapTask(task.TaskID)
			if err != nil {
				logger.Log.WithFields(logrus.Fields{"ERROR": err}).Errorf("Error deleting power cap task, %s.", task.TaskID.String())
			}
		} else if task.TaskStatus == model.PowerCapTaskStatusCompleted {
			numComplete++
			// Compress the completed task if the it was not previuosly compressed
			// upon completion (probably because it was stored by an older version
			// of PCS).
			if !task.IsCompressed {
				compressAndCompleteTask(task)
			}
		}
	}

	if GLOB.MaxNumCompleted <= 0 {
		// No limit
		return
	}

	// Additionally, delete records if we've exceeded our maximum.
	numDelete := numComplete - GLOB.MaxNumCompleted
	if numDelete > 0 {
		logger.Log.Infof("Deleting %d overflow record(s)", numDelete)
		// Find the oldest 'numDelete' records and delete them.
		tasksToDelete := make([]*model.PowerCapTask, numDelete)
		for t, task := range tasks {
			if task.TaskStatus != model.PowerCapTaskStatusCompleted { continue }
			for i := 0; i < numDelete; i++ {
				if tasksToDelete[i] == nil {
					tasksToDelete[i] = &tasks[t]
					break
				} else if tasksToDelete[i].TaskCreateTime.After(task.TaskCreateTime) {
					// Found an older record. Shift the array elements.
					currTask := &tasks[t]
					for j := i; j < numDelete; j++ {
						if tasksToDelete[j] == nil {
							tasksToDelete[j] = currTask
							break
						}
						tmpTask := tasksToDelete[j]
						tasksToDelete[j] = currTask
						currTask = tmpTask
					}
					break
				}
			}
		}
		for _, task := range tasksToDelete {
			// Get the operations for the power cap task
			ops, err := (*GLOB.DSP).GetAllPowerCapOperationsForTask(task.TaskID)
			if err != nil {
				logger.Log.WithFields(logrus.Fields{"ERROR": err}).Errorf("Error retreiving operations for power cap task, %s.", task.TaskID.String())
				continue
			}
			for _, op := range ops {
				err = (*GLOB.DSP).DeletePowerCapOperation(task.TaskID, op.OperationID)
				if err != nil {
					logger.Log.WithFields(logrus.Fields{"ERROR": err}).Errorf("Error deleting operation, %s, for power cap task, %s.", op.OperationID.String(), task.TaskID.String())
				}
			}
			err = (*GLOB.DSP).DeletePowerCapTask(task.TaskID)
			if err != nil {
				logger.Log.WithFields(logrus.Fields{"ERROR": err}).Errorf("Error deleting power cap task, %s.", task.TaskID.String())
			}
		}
	}
}

// Compress the completed Power Cap task by adding all of the relevant
// operation data to the task for storage and deleting the operations.
// Keep only what is needed.
func compressAndCompleteTask(task model.PowerCapTask) {
	// Get the operations for the power cap task
	ops, err := (*GLOB.DSP).GetAllPowerCapOperationsForTask(task.TaskID)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Errorf("Error retreiving operations for power cap task, %s.", task.TaskID.String())
		return
	}
	// Build the response struct
	rsp := buildPowerCapResponse(task, ops, true)
	task.TaskCounts = rsp.TaskCounts
	task.Components = rsp.Components
	task.TaskStatus = model.PowerCapTaskStatusCompleted
	task.IsCompressed = true
	err = (*GLOB.DSP).StorePowerCapTask(task)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing power capping task")
		// Don't delete the operations if we were unsuccessful storing the task.
		return
	}
	for _, op := range ops {
		err = (*GLOB.DSP).DeletePowerCapOperation(task.TaskID, op.OperationID)
		if err != nil {
			logger.Log.WithFields(logrus.Fields{"ERROR": err}).Errorf("Error deleting operation, %s, for power cap task, %s.", op.OperationID.String(), task.TaskID.String())
		}
	}
	return
}
