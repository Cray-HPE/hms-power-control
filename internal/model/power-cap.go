/*
 * (C) Copyright [2021-2023] Hewlett Packard Enterprise Development LP
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

package model

import (
	"time"

	"github.com/Cray-HPE/hms-power-control/internal/hsm"
	"github.com/google/uuid"
)

///////////////////////////
// Power Capping Definitions
///////////////////////////

const (
	PowerCapTaskTypeSnapshot = "snapshot"
	PowerCapTaskTypePatch    = "patch"
)

const (
	PowerCapTaskStatusNew        = "new"
	PowerCapTaskStatusInProgress = "in-progress"
	PowerCapTaskStatusCompleted  = "completed"
)

const (
	PowerCapOpStatusNew         = "new"
	PowerCapOpStatusInProgress  = "in-progress"
	PowerCapOpStatusFailed      = "failed"
	PowerCapOpStatusSucceeded   = "Succeeded"
	PowerCapOpStatusUnsupported = "Unsupported"
)

///////////////////////////
//INPUT - Generally from the API layer
///////////////////////////

type PowerCapSnapshotParameter struct {
	Xnames []string `json:"xnames"`
}

type PowerCapPatchParameter struct {
	Components []PowerCapComponentParameter `json:"components"`
}

type PowerCapComponentParameter struct {
	Xname    string                     `json:"xname"`
	Controls []PowerCapControlParameter `json:"controls"`
}

type PowerCapControlParameter struct {
	Name  string `json:"name"`
	Value int    `json:"value"` //TODO is this the right data type? can it be double?
}

//////////////
// INTERNAL - Generally passed around /internal/* packages
//////////////

type PowerCapTask struct {
	TaskID                  uuid.UUID                  `json:"taskID"`
	Type                    string                     `json:"type"`
	SnapshotParameters      *PowerCapSnapshotParameter `json:"snapshotParameters,omitempty"`
	PatchParameters         *PowerCapPatchParameter    `json:"patchParameters,omitempty"`
	TaskCreateTime          time.Time                  `json:"taskCreateTime"`
	AutomaticExpirationTime time.Time                  `json:"automaticExpirationTime"`
	TaskStatus              string                     `json:"taskStatus"`
	OperationIDs            []uuid.UUID

	// Only populated when the task is completed
	IsCompressed bool                `json:"isCompressed"`
	TaskCounts   PowerCapTaskCounts  `json:"taskCounts"`
	Components   []PowerCapComponent `json:"components,omitempty"`
}

type PowerCapOperation struct {
	OperationID uuid.UUID         `json:"operationID"`
	TaskID      uuid.UUID         `json:"taskID"`
	Type        string            `json:"type"`
	Status      string            `json:"status"`
	Component   PowerCapComponent `json:"Component"`

	// From HSM /Inventory/ComponentEndpoints
	RfFQDN                string                  `json:"RfFQDN"`
	PowerCapURI           string                  `json:"powerCapURI"`
	PowerCapTargetURI     string                  `json:"powerCapTargetURI"`
	PowerCapControlsCount int                     `json:"powerCapControlsCount"`
	PowerCapCtlInfoCount  int                     `json:"powerCapCtlInfoCount"`
	PowerCaps             map[string]hsm.PowerCap `json:"powerCap"`
}

//////////////
// OUTPUT - Generally passed back to the API layer.
//////////////

type PowerCapTaskCreation struct {
	TaskID uuid.UUID `json:"taskID"`
}

type PowerCapTaskRespArray struct {
	Tasks []PowerCapTaskResp `json:"tasks"`
}

type PowerCapTaskResp struct {
	TaskID                  uuid.UUID           `json:"taskID"`
	Type                    string              `json:"type"`
	TaskCreateTime          time.Time           `json:"taskCreateTime"`
	AutomaticExpirationTime time.Time           `json:"automaticExpirationTime"`
	TaskStatus              string              `json:"taskStatus"`
	TaskCounts              PowerCapTaskCounts  `json:"taskCounts"`
	Components              []PowerCapComponent `json:"components,omitempty"`
}

type PowerCapTaskCounts struct {
	Total       int `json:"total"`
	New         int `json:"new"`
	InProgress  int `json:"in-progress"`
	Failed      int `json:"failed"`
	Succeeded   int `json:"succeeded"`
	Unsupported int `json:"un-supported"`
}

type PowerCapComponent struct {
	Xname          string             `json:"xname"`
	Error          string             `json:"error,omitempty"`
	Limits         *PowerCapabilities `json:"limits,omitempty"`
	PowerCapLimits []PowerCapControls `json:"powerCapLimits,omitempty"`
}

type PowerCapabilities struct {
	HostLimitMax *int `json:"hostLimitMax,omitempty"`
	HostLimitMin *int `json:"hostLimitMin,omitempty"`
	PowerupPower *int `json:"powerupPower,omitempty"`
}

type PowerCapControls struct {
	Name         string `json:"name"`
	CurrentValue *int   `json:"currentValue,omitempty"`
	MaximumValue *int   `json:"maximumValue,omitempty"`
	MinimumValue *int   `json:"minimumValue,omitempty"`
}

//////////////
// FUNCTIONS
//////////////

func NewPowerCapSnapshotTask(parameters PowerCapSnapshotParameter, expirationTimeMins int) PowerCapTask {
	task := newPowerCapTask(expirationTimeMins)
	task.Type = PowerCapTaskTypeSnapshot
	task.SnapshotParameters = &parameters
	return task
}

func NewPowerCapPatchTask(parameters PowerCapPatchParameter, expirationTimeMins int) PowerCapTask {
	task := newPowerCapTask(expirationTimeMins)
	task.Type = PowerCapTaskTypePatch
	task.PatchParameters = &parameters
	return task
}

func newPowerCapTask(expirationTimeMins int) PowerCapTask {
	return PowerCapTask{
		TaskID:                  uuid.New(),
		TaskCreateTime:          time.Now(),
		AutomaticExpirationTime: time.Now().Add(time.Minute * time.Duration(expirationTimeMins)),
		TaskStatus:              PowerCapTaskStatusNew,
		OperationIDs:            []uuid.UUID{},
	}
}

func NewPowerCapOperation(taskID uuid.UUID, operationType string) PowerCapOperation {
	return PowerCapOperation{
		OperationID: uuid.New(),
		TaskID:      taskID,
		Type:        operationType,
		Status:      PowerCapOpStatusNew,
	}
}

func (a *PowerCapTaskResp) Equals(b PowerCapTaskResp) bool {
	//Not comparing TaskID or any of the timestamp fields because we don't care.
	if a.Type != b.Type ||
		a.TaskStatus != b.TaskStatus ||
		a.TaskCounts != b.TaskCounts ||
		len(a.Components) != len(b.Components) {
		return false
	}
	for _, compA := range a.Components {
		found := false
		for _, compB := range b.Components {
			if compA.Xname != compB.Xname ||
				compA.Error != compB.Error ||
				(compA.Limits == nil) != (compB.Limits == nil) ||
				len(compA.PowerCapLimits) != len(compB.PowerCapLimits) {
				continue
			}
			if compA.Limits != nil {
				if (compA.Limits.HostLimitMax == nil) != (compB.Limits.HostLimitMax == nil) ||
					(compA.Limits.HostLimitMin == nil) != (compB.Limits.HostLimitMin == nil) ||
					(compA.Limits.PowerupPower == nil) != (compB.Limits.PowerupPower == nil) {
					continue
				}
				if ((compA.Limits.HostLimitMax != nil) && (*compA.Limits.HostLimitMax != *compB.Limits.HostLimitMax)) ||
					((compA.Limits.HostLimitMin != nil) && (*compA.Limits.HostLimitMin != *compB.Limits.HostLimitMin)) ||
					((compA.Limits.PowerupPower != nil) && (*compA.Limits.PowerupPower != *compB.Limits.PowerupPower)) {
					continue
				}
			}
			for _, ctlA := range compA.PowerCapLimits {
				ctlFound := false
				for _, ctlB := range compB.PowerCapLimits {
					if ctlA.Name != ctlB.Name ||
						(ctlA.CurrentValue == nil) != (ctlB.CurrentValue == nil) ||
						(ctlA.MaximumValue == nil) != (ctlB.MaximumValue == nil) ||
						(ctlA.MinimumValue == nil) != (ctlB.MinimumValue == nil) {
						continue
					}
					if ((ctlA.CurrentValue != nil) && (*ctlA.CurrentValue != *ctlB.CurrentValue)) ||
						((ctlA.MaximumValue != nil) && (*ctlA.MaximumValue != *ctlB.MaximumValue)) ||
						((ctlA.MinimumValue != nil) && (*ctlA.MinimumValue != *ctlB.MinimumValue)) {
						continue
					}
					ctlFound = true
					break
				}
				if !ctlFound {
					return false
				}
			}
			found = true
			break
		}
		if !found {
			return false
		}
	}
	return true
}
