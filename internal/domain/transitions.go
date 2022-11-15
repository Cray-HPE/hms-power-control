/*
 * (C) Copyright [2021-2022] Hewlett Packard Enterprise Development LP
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
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	base "github.com/Cray-HPE/hms-base"
	"github.com/Cray-HPE/hms-power-control/internal/hsm"
	"github.com/Cray-HPE/hms-power-control/internal/logger"
	"github.com/Cray-HPE/hms-power-control/internal/model"
	// "github.com/Cray-HPE/hms-power-control/internal/storage"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type TransitionComponent struct {
	PState      *model.PowerStatusComponent
	HSMData     *hsm.HsmData
	Task        *model.TransitionTask
	DeputyKey   string
	Actions     map[string]string
	ActionCount int // Number of actions until task competion
}

type PowerSeqElem struct {
	Action    string
	CompTypes []base.HMSType
	Wait      int
}

var PowerSequenceFull = []PowerSeqElem{
	{
		Action:    "gracefulshutdown",
		CompTypes: []base.HMSType{base.Node, base.HSNBoard},
	}, {
		Action:    "forceoff",
		CompTypes: []base.HMSType{base.Node, base.HSNBoard},
	}, {
		Action:    "gracefulshutdown",
		CompTypes: []base.HMSType{base.RouterModule, base.ComputeModule},
	}, {
		Action:    "forceoff",
		CompTypes: []base.HMSType{base.RouterModule, base.ComputeModule},
	}, {
		Action:    "gracefulshutdown",
		CompTypes: []base.HMSType{base.Chassis},
	}, {
		Action:    "forceoff",
		CompTypes: []base.HMSType{base.Chassis},
	}, {
		Action:    "gracefulshutdown",
		CompTypes: []base.HMSType{base.CabinetPDUPowerConnector},
	}, {
		Action:    "forceoff",
		CompTypes: []base.HMSType{base.CabinetPDUPowerConnector},
	}, {
		Action:    "gracefulrestart",
		// Not all of these components support GracefulRestart but, if they did,
		// since power isn't being dropped doing them all at the same time should
		// be fine.
		CompTypes: []base.HMSType{base.Node, base.HSNBoard, base.RouterModule, base.ComputeModule, base.Chassis, base.CabinetPDUPowerConnector},
	}, {
		Action:    "on",
		CompTypes: []base.HMSType{base.CabinetPDUPowerConnector},
	}, {
		Action:    "on",
		CompTypes: []base.HMSType{base.Chassis},
	}, {
		Action:    "on",
		CompTypes: []base.HMSType{base.RouterModule, base.ComputeModule},
	}, {
		Action:    "on",
		CompTypes: []base.HMSType{base.Node, base.HSNBoard},
	},
}

func GetTransition(transitionID uuid.UUID) (pb model.Passback) {
	// Get the transition
	transition, err := (*GLOB.DSP).GetTransition(transitionID)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			pb = model.BuildErrorPassback(http.StatusNotFound, err)
		} else {
			pb = model.BuildErrorPassback(http.StatusInternalServerError, err)
		}
		logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Error retrieving transition")
		return
	}
	if transition.TransitionID.String() != transitionID.String() {
		err := errors.New("TransitionID does not exist")
		pb = model.BuildErrorPassback(http.StatusNotFound, err)
		logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Error retrieving transition")
	}
	// Get the operations for the task
	tasks, err := (*GLOB.DSP).GetAllTasksForTransition(transitionID)
	if err != nil {
		pb = model.BuildErrorPassback(http.StatusInternalServerError, err)
		logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Error retrieving transition tasks")
		return
	}

	// Build the response struct
	rsp := model.ToTransitionResp(transition, tasks, true)

	pb = model.BuildSuccessPassback(http.StatusOK, rsp)
	return
}

func GetTransitionStatuses() (pb model.Passback) {

	// Get all transitions
	transitions, err := (*GLOB.DSP).GetAllTransitions()
	if err != nil {
		pb = model.BuildErrorPassback(http.StatusInternalServerError, err)
		logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Error retrieving transitions")
		return
	}

	rsp := model.TransitionRespArray{
		Transitions: []model.TransitionResp{},
	}
	// Get the tasks for each transition
	for _, transition := range transitions {
		tasks, err := (*GLOB.DSP).GetAllTasksForTransition(transition.TransitionID)
		if err != nil {
			pb = model.BuildErrorPassback(http.StatusInternalServerError, err)
			logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Error retrieving transition tasks")
			return
		}
		// Build the response struct
		transitionRsp := model.ToTransitionResp(transition, tasks, false)
		rsp.Transitions = append(rsp.Transitions, transitionRsp)
	}

	pb = model.BuildSuccessPassback(http.StatusOK, rsp)
	return
}

// This uses Test-And-Set operations to signal an abort to prevent overwriting
// another instance's store operation. Try a couple times before giving up.
func AbortTransitionID(transitionID uuid.UUID) (pb model.Passback) {
	for retry := 0; retry < 3; retry++ {
		// Get the transition
		transition, err := (*GLOB.DSP).GetTransition(transitionID)
		if err != nil {
			if strings.Contains(err.Error(), "does not exist") {
				pb = model.BuildErrorPassback(http.StatusNotFound, err)
			} else {
				pb = model.BuildErrorPassback(http.StatusInternalServerError, err)
			}
			logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Error retrieving transition")
			return
		}
		if transition.TransitionID.String() != transitionID.String() {
			err := errors.New("TransitionID does not exist")
			pb = model.BuildErrorPassback(http.StatusNotFound, err)
			logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Error retrieving transition")
		}
		transitionOld := transition
		transition.Status = model.TransitionStatusAbortSignaled
		// Use test and set to prevent overwriting another thread's store operation.
		ok, err := (*GLOB.DSP).TASTransition(transition, transitionOld)
		if err != nil {
			pb = model.BuildErrorPassback(http.StatusInternalServerError, err)
			logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Error storing new transition")
			return
		}
		if ok {
			pb = model.BuildSuccessPassback(202, "Accepted - abort initiated")
			return pb
		}
	}

	err := errors.New("Failed to signal abort")
	pb = model.BuildErrorPassback(http.StatusInternalServerError, err)
	logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Error storing abort-signaled status")
	return pb
}

func TriggerTransition(transition model.Transition) (pb model.Passback) {

	// Store transition
	err := (*GLOB.DSP).StoreTransition(transition)
	if err != nil {
		pb = model.BuildErrorPassback(http.StatusInternalServerError, err)
		logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Error storing new transition")
		return
	}

	// Start transition
	go doTransition(transition.TransitionID)

	rsp := model.TransitionCreation{
		TransitionID: transition.TransitionID,
		Operation: transition.Operation.String(),
	}
	pb = model.BuildSuccessPassback(http.StatusOK, rsp)
	return
}

///////////////////////////
// Non-exported functions (helpers, utils, etc)
///////////////////////////

// Main worker for executing transitions
func doTransition(transitionID uuid.UUID) {
	var (
		//xnames          []string
		resData         []hsm.ReservationData
		reservationData []*hsm.ReservationData
		xnameHierarchy  []string
		isSoft          bool
		noWait          bool
		waitForever     bool
	)
	//xnameMap := make(map[string]*TransitionComponent)
	seqMap := map[string]map[base.HMSType][]*TransitionComponent{
		"on":               make(map[base.HMSType][]*TransitionComponent),
		"gracefulshutdown": make(map[base.HMSType][]*TransitionComponent),
		"forceoff":         make(map[base.HMSType][]*TransitionComponent),
		"gracefulrestart":  make(map[base.HMSType][]*TransitionComponent),
	}

	tr, err := (*GLOB.DSP).GetTransition(transitionID)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Cannot retrieve transition, cannot generate tasks")
		return
	}
	// // Get any tasks that may have previously been created for our operation.
	// tasks, err = (*GLOB.DSP).GetAllTasksForTransition(tr.TransitionID)
	// if err != nil {
		// logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error retrieving tasks for transition, " + tr.TransitionID.String())
		// return
	// }
	// // Rebuild our xnameMap based on the previous tasks
	// for _, task := range tasks {
		// xnameMap[loc.Xname] = &TransitionComponent{
			// Task:      &task,
		// }
	// }
	// // Make sure all the previously created tasks are in the transition's task array.
	// if len(xnameMap) > 0 {
		// taskMap := make(map[uuid.UUID]bool)
		// for _, taskID := range tr.TaskIDs {
			// taskMap[taskID] = true
		// }
		// for _, comp := range xnameMap {
			// comp.Task.TaskID
			// if _, ok := taskMap[comp.Task.TaskID]; !ok {
				// tr.TaskIDs = append(tr.TaskIDs, comp.Task.TaskID)
			// }
			// if comp.Task.Status == TransitionTaskStatusNew ||
			   // comp.Task.Status == TransitionTaskStatusInProgress {
				// xnames = append(xnames, comp.Task.Xname)
			// }
		// }
	// }

	defer logger.Log.Debugf("Transition %s Completed", tr.TransitionID.String())

	// Restarting a transition
	if tr.Status != model.TransitionStatusNew {
		logger.Log.Debugf("Restarting Transition %s", tr.TransitionID.String())
		if tr.Status == model.TransitionStatusCompleted ||
		   tr.Status == model.TransitionStatusAborted {
			// Shouldn't pick up completed Transitions anyway
			return
		}
	} else {
		logger.Log.Debugf("Starting Transition %s", tr.TransitionID.String())
	}

	// // Start abort watcher
	// abortChan := make(chan bool)
	// cbHandle, err := (*GLOB.DSP).WatchTransitionCB(tr.TransitionID, storage.TransitionWatchCBFunc(transitionAbortWatchCB), abortChan)
	// if err != nil {
		// logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error starting abort watcher")
	// }
	// defer (*GLOB.DSP).WatchTransitionCBCancel(cbHandle)

	if tr.Operation == model.Operation_SoftOff {
		isSoft = true
	}

	if tr.TaskDeadline == 0 {
		noWait = true
	} else if tr.TaskDeadline < 0 {
		waitForever = true
	}

	// // Vet and turn the list of requested xnames into a map
	// for _, loc := range tr.Location {
		// if comp, ok := xnameMap[loc.Xname]; ok {
			// // Is a duplicate or from a restart.
			// // Restarted tasks will need just the deputy key readded.
			// if comp.DeputyKey == "" {
				// comp.DeputyKey = loc.DeputyKey
			// }
			// continue
		// }

		// // Create tasks for everything requested so we can
		// // communicate reasons for failures.
		// task := model.NewTransitionTask(tr.TransitionID, tr.Operation)
		// task.Xname = loc.Xname

		// // Weed out invalid xnames and components we can't power control here.
		// compType := base.GetHMSType(loc.Xname)
		// switch(compType) {
		// case base.Node:                     fallthrough
		// case base.CabinetPDUOutlet:         fallthrough
		// case base.CabinetPDUPowerConnector: fallthrough
		// case base.HSNBoard:                 fallthrough
		// case base.Chassis:                  fallthrough
		// case base.ComputeModule:            fallthrough
		// case base.RouterModule:
			// task.StatusDesc = "Gathering data"
		// case base.HMSTypeInvalid:
			// task.Status = model.TransitionTaskStatusFailed
			// task.Error = "Invalid xname"
			// task.StatusDesc = "Failed to achieve transition"
		// default:
			// task.Status = model.TransitionTaskStatusFailed
			// task.Error = "No power control for component type " + compType.String()
			// task.StatusDesc = "Failed to achieve transition"
		// }
		// tr.TaskIDs = append(tr.TaskIDs, task.TaskID)
		// err = (*GLOB.DSP).StoreTransitionTask(task)
		// if err != nil {
			// logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
		// }
		// xnameMap[loc.Xname] = &TransitionComponent{
			// Task:      &task,
			// DeputyKey: loc.DeputyKey,
		// }
		// if task.Status != model.TransitionTaskStatusFailed {
			// xnames = append(xnames, loc.Xname)
		// }
	// }

	xnameMap, xnames := setupTransitionTasks(&tr)

	if len(xnames) == 0 {
		// All xnames were invalid
		err = errors.New("No components to operate on")
		logrus.WithFields(logrus.Fields{"ERROR": err}).Error("No components to operate on")
		tr.Status = model.TransitionStatusCompleted
		err = (*GLOB.DSP).StoreTransition(tr)
		if err != nil {
			logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition")
		}
		return
	}

	// Finish up abort-signaled transitions. Shouldn't be picking
	// up Aborted transitions but handle them just in case they're
	// incompletely aborted.
	// if tr.Status == model.TransitionStatusAbortSignaled ||
	   // checkAbort(abortChan) {
		// doAbort(tr, xnameMap)
		// return
	// }

	// Store the transition with its initial set of tasks. May have more added later.
	tr.Status = model.TransitionStatusInProgress
	// err = (*GLOB.DSP).StoreTransition(tr)
	// if err != nil {
		// logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition")
	// }
	abortSignaled, err := storeTransition(tr)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition")
	}
	if abortSignaled {
		doAbort(tr, xnameMap)
	}

	///////////////////////////////////////////////////////////////////////////
	// o Vet XNames with our internal stored Power Status. Should have everything
	//   HSM has plus a recently captured power state from hardware.
	///////////////////////////////////////////////////////////////////////////

	// Expand the list of xnames to include power controlled subcomponents. This way we
	// already have the information for additional components we might need to add.
	pStates, missingXnames, err := getPowerStateHierarchy(xnames)
	if err != nil {
		// This failed to an ETCD error. Likely because we couldn't reach it
		// which means we really don't have a way to inform anyone about this
		// failed job because we'd need to do a STORE to mark the job as failed.
		// TODO: Maybe retry indefinitely? For now exit.
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Cannot retrieve Power States, cannot continue")
		return
	}
	// Finish out tasks for components that were not found or we cannot power control. 
	if len(missingXnames) > 0 {
		logrus.WithFields(logrus.Fields{"ERROR": err, "xnames": missingXnames}).Error("Missing xnames detected")
		for _, xname := range missingXnames {
			comp, ok := xnameMap[xname]
			if !ok {
				// We don't care about xnames not in our list
				continue
			}
			// Set failures for each listed xname
			comp.Task.Status = model.TransitionTaskStatusFailed
			compType := base.GetHMSType(xname)
			if compType != base.Chassis &&
			   compType != base.ComputeModule &&
			   compType != base.Node &&
			   compType != base.RouterModule &&
			   compType != base.HSNBoard &&
			   compType != base.CabinetPDUOutlet &&
			   compType != base.CabinetPDUPowerConnector {
				comp.Task.Error = "No power control for component type " + compType.String()
			} else {
				comp.Task.Error = "Missing xname"
			}
			comp.Task.StatusDesc = "Failed to achieve transition"
			err = (*GLOB.DSP).StoreTransitionTask(*comp.Task)
			if err != nil {
				logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
			}
		}
	}
	if len(pStates) == 0 {
		// No xnames found
		err = errors.New("No xnames to operate on")
		logrus.WithFields(logrus.Fields{"ERROR": err}).Error("No xnames to operate on")
		tr.Status = model.TransitionStatusCompleted
		err = (*GLOB.DSP).StoreTransition(tr)
		if err != nil {
			logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition")
		}
		return
	}

	for xname, _ := range pStates {
		xnameHierarchy = append(xnameHierarchy, xname)
	}

	///////////////////////////////////////////////////////////////////////////
	// o Get the component state and ComponentEndpoint data from HSM.
	///////////////////////////////////////////////////////////////////////////

	hsmData, err := (*GLOB.HSM).FillHSMData(xnameHierarchy)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error retrieving HSM data")
		// Failed to get data from HSM. Fail everything.
		for _, xname := range xnames {
			comp, ok := xnameMap[xname]
			if !ok {
				// We don't care about xnames not in our list
				continue
			}
			if comp.Task.Status != model.TransitionTaskStatusNew &&
			   comp.Task.Status != model.TransitionTaskStatusInProgress {
				// Skip it if it is already complete
				continue
			}
			comp.Task.Status = model.TransitionTaskStatusFailed
			comp.Task.Error = "Error retrieving HSM data"
			comp.Task.StatusDesc = "Failed to achieve transition"
			err = (*GLOB.DSP).StoreTransitionTask(*comp.Task)
			if err != nil {
				logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
			}
		}
	} else {
		// Check to see if we got everything back.
		if len(hsmData) != len(xnameHierarchy) {
			for _, xname := range xnames {
				// This xname was not found in the response from HSM.
				// Set a failed "Not found" task for it.
				if _, ok := hsmData[xname]; !ok {
					comp, ok := xnameMap[xname]
					if !ok {
						// We don't care about xnames not in our list
						continue
					}
					if comp.Task.Status != model.TransitionTaskStatusNew &&
					   comp.Task.Status != model.TransitionTaskStatusInProgress {
						// Skip it if it is already complete
						continue
					}

					comp.Task.Status = model.TransitionTaskStatusFailed
					comp.Task.Error = "Xname not found in HSM"
					comp.Task.StatusDesc = "Failed to achieve transition"
					err = (*GLOB.DSP).StoreTransitionTask(*comp.Task)
					if err != nil {
						logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
					}
				}
			}
		}
	}

	// Attach collected data and add any dependent components (i.e. Rosettas).
	for _, xname := range xnames {
		comp, ok := xnameMap[xname]
		if !ok {
			// We don't care about xnames not in our list
			continue
		}
		if comp.Task.Status != model.TransitionTaskStatusNew &&
		   comp.Task.Status != model.TransitionTaskStatusInProgress {
			continue
		}
		ps, ok := pStates[xname]
		if !ok {
			continue
		}
		hData, ok := hsmData[xname]
		if !ok {
			continue
		}

		actions := make(map[string]string)
		for _, action := range hData.AllowableActions {
			actions[strings.ToLower(action)] = action
		}
		comp.PState = &ps
		comp.HSMData = hData
		comp.Actions = actions

		// Add any Rosettas if we're powering off RouterModules
		if (base.GetHMSType(xname) == base.RouterModule) &&
		   ((hData.BaseData.Class == base.ClassHill.String()) || (hData.BaseData.Class == base.ClassMountain.String())) &&
		   (tr.Operation != model.Operation_On) {
			switchXname := xname + "e0"
			_, compOk := xnameMap[switchXname]
			switchPs, psOk := pStates[switchXname]
			switchHData, hsmOk := hsmData[switchXname]
			// Skip if the rosetta is already in our list. The below
			// will be or has been already done for that component.
			if psOk && hsmOk && !compOk {
				task := model.NewTransitionTask(tr.TransitionID, tr.Operation)
				task.Xname = switchXname
				task.StatusDesc = "Gathering data"
				tr.TaskIDs = append(tr.TaskIDs, task.TaskID)
				err = (*GLOB.DSP).StoreTransitionTask(task)
				if err != nil {
					logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
				}
				switchActions := make(map[string]string)
				for _, action := range switchHData.AllowableActions {
					actions[strings.ToLower(action)] = action
				}
				xnameMap[switchXname] = &TransitionComponent{
					Task: &task,
					PState: &switchPs,
					HSMData: switchHData,
					Actions: switchActions,
				}
			}
		}
	}

	// if checkAbort(abortChan) {
		// doAbort(tr, xnameMap)
		// return
	// }

	// err = (*GLOB.DSP).StoreTransition(tr)
	// if err != nil {
		// logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition")
	// }
	abortSignaled, err = storeTransition(tr)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition")
	}
	if abortSignaled {
		doAbort(tr, xnameMap)
	}

	// Sort components into groups so they can follow a proper power sequence
	for xname, comp := range xnameMap {
		if comp.Task.Status != model.TransitionTaskStatusNew &&
		   comp.Task.Status != model.TransitionTaskStatusInProgress {
			continue
		}

		compType := base.GetHMSType(xname)
		psf, _ := model.ToPowerStateFilter(comp.PState.PowerState)
		switch(tr.Operation) {
		case model.Operation_On:
			if psf == model.PowerStateFilter_On {
				// Already complete
				comp.Task.Status = model.TransitionTaskStatusSucceeded
				if comp.Task.State == model.TaskState_GatherData {
					comp.Task.StatusDesc = "Component already in desired state"
				} else {
					comp.Task.StatusDesc = "Transition confirmed, On"
				}
			} else {
				comp.ActionCount++
				// Attempt to power on even if the last collected powerstate is "undefined".
				// It may become available after powering on a parent component.
				seqMap["on"][compType] = append(seqMap["on"][compType], comp)
			}
		case model.Operation_SoftOff: fallthrough
		case model.Operation_Off:
			if psf == model.PowerStateFilter_Off {
				// Already complete
				comp.Task.Status = model.TransitionTaskStatusSucceeded
				if comp.Task.State == model.TaskState_GatherData {
					comp.Task.StatusDesc = "Component already in desired state"
				} else {
					comp.Task.StatusDesc = "Transition confirmed, Off"
				}
			} else {
				comp.ActionCount++
				seqMap["gracefulshutdown"][compType] = append(seqMap["gracefulshutdown"][compType], comp)
			}
		case model.Operation_SoftRestart:
			if psf != model.PowerStateFilter_On {
				if comp.Task.State == model.TaskState_GatherData {
					// Not a restarted task
					comp.Task.Status = model.TransitionTaskStatusFailed
					comp.Task.StatusDesc = "Component must be in the On state for Soft-Restart"
					comp.Task.Error = "Component is in the wrong power state"
				} else if comp.Task.Operation == model.Operation_On {
					// Task restarted after we powered off the component but before we confirmed
					// it powered back on. Let the logic below decide whether we will resend the
					// command or just wait.
					comp.ActionCount++
					seqMap["on"][compType] = append(seqMap["on"][compType], comp)
				} else if (comp.Task.Operation == model.Operation_Off ||
				           comp.Task.Operation == model.Operation_ForceOff) {
					// Restarted after the component was powered off but may still need to be powered on.
					comp.ActionCount++
					seqMap["on"][compType] = append(seqMap["on"][compType], comp)
					comp.Task.State = model.TaskState_Confirmed
				} else {
					comp.Task.Status = model.TransitionTaskStatusFailed
					if comp.Task.Operation == model.Operation_SoftRestart {
						comp.Task.StatusDesc = "Failed to apply transition, GracefulRestart"
					} else {
						comp.Task.StatusDesc = "Failed to apply transition, ForceRestart"
					}
					comp.Task.Error = "Unknown error"
				}
			} else {
				parentSupportsRestart := true
				_, hasAction := comp.Actions["gracefulrestart"]
				if hasAction {
					// If a parent component has also been requested and it doesn't
					// support gracefulrestart, power will be dropped. Do an off->on
					// if power will be dropped.
					id := base.GetHMSCompParent(xname)
					for {
						parentType := base.GetHMSType(id)
						if parentType == base.HMSTypeInvalid {
							break
						}
						if parent, ok := xnameMap[id]; ok {
							if _, ok := parent.Actions["gracefulrestart"]; !ok {
								parentSupportsRestart = false
							}
						} else {
							id = base.GetHMSCompParent(id)
						}
					}
				}
				if hasAction && parentSupportsRestart {
					comp.ActionCount++
					seqMap["gracefulrestart"][compType] = append(seqMap["gracefulrestart"][compType], comp)
				} else {
					comp.ActionCount += 2
					seqMap["gracefulshutdown"][compType] = append(seqMap["gracefulshutdown"][compType], comp)
					seqMap["on"][compType] = append(seqMap["on"][compType], comp)
				}
			}
		case model.Operation_HardRestart:
			if psf != model.PowerStateFilter_On {
				if comp.Task.State == model.TaskState_GatherData {
					comp.Task.Status = model.TransitionTaskStatusFailed
					comp.Task.StatusDesc = "Component must be in the On state for Hard-Restart"
					comp.Task.Error = "Component is in the wrong power state"
				} else if comp.Task.Operation == model.Operation_On {
					// Task restarted after we powered off the component but before we confirmed
					// it powered back on. Let the logic below decide whether we will resend the
					// command or just wait.
					comp.ActionCount++
					seqMap["on"][compType] = append(seqMap["on"][compType], comp)
				} else if (comp.Task.Operation == model.Operation_Off ||
				           comp.Task.Operation == model.Operation_ForceOff) {
					// Restarted after the component was powered off but may still need to be powered on.
					comp.ActionCount++
					seqMap["on"][compType] = append(seqMap["on"][compType], comp)
					comp.Task.State = model.TaskState_Confirmed
				}
			} else {
				comp.ActionCount += 2
				seqMap["gracefulshutdown"][compType] = append(seqMap["gracefulshutdown"][compType], comp)
				seqMap["on"][compType] = append(seqMap["on"][compType], comp)
			}
		case model.Operation_Init:
			if psf == model.PowerStateFilter_On {
				comp.ActionCount += 2
				seqMap["gracefulshutdown"][compType] = append(seqMap["gracefulshutdown"][compType], comp)
				seqMap["on"][compType] = append(seqMap["on"][compType], comp)
			} else {
				comp.ActionCount++
				seqMap["on"][compType] = append(seqMap["on"][compType], comp)
			}
		case model.Operation_ForceOff:
			if psf == model.PowerStateFilter_Off {
				// Already complete
				comp.Task.Status = model.TransitionTaskStatusSucceeded
				if comp.Task.State == model.TaskState_GatherData {
					comp.Task.StatusDesc = "Component already in desired state"
				} else {
					comp.Task.StatusDesc = "Transition confirmed, ForceOff"
				}
			} else {
				comp.ActionCount++
				seqMap["forceoff"][compType] = append(seqMap["forceoff"][compType], comp)
			}
		}
		// Form the ReservationData array for use with the HSM API for acquiring component reservations.
		if comp.Task.Status == model.TransitionTaskStatusNew ||
		   comp.Task.Status == model.TransitionTaskStatusInProgress {
			res := hsm.ReservationData{
				XName: xname,
				DeputyKey: comp.DeputyKey,
			}
			resData = append(resData, res)
		}
		err = (*GLOB.DSP).StoreTransitionTask(*comp.Task)
		if err != nil {
			logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
		}
	}

	///////////////////////////////////////////////////////////////////////////
	// o Reserve components. This will make sure we aren't already operating on
	//   any of the targets. The Go lib keeps these alive.
	//
	// o Validate deputy keys and try to reserve components with invalid deputy
	//   keys before giving up on them.
	//
	// o Set errors on anything we couldn't reserve
	///////////////////////////////////////////////////////////////////////////

	// ReserveComponents() will validate any deputy keys and reserve any
	// components with an invalid deputy key or without a deputy key.
	reservationData, err = (*GLOB.HSM).ReserveComponents(resData)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error acquiring reservations")
		// An error occurred while reserving components. This does not include partial failure. Fail everything.
		for _, comp := range xnameMap {
			if comp.Task.Status != model.TransitionTaskStatusNew ||
			   comp.Task.Status != model.TransitionTaskStatusInProgress {
				continue
			}
			comp.Task.Status = model.TransitionTaskStatusFailed
			comp.Task.Error = "Error acquiring reservations"
			comp.Task.StatusDesc = "Failed to achieve transition"
			err = (*GLOB.DSP).StoreTransitionTask(*comp.Task)
			if err != nil {
				logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
			}
		}
		tr.Status = model.TransitionStatusCompleted
		err = (*GLOB.DSP).StoreTransition(tr)
		if err != nil {
			logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition")
		}
		return
	} else {
		// Check to see if we got everything back. ReserveComponents returns
		// everything that either was successfully reserved or had a valid
		// deputy key.
		if len(resData) != len(reservationData) {
			resMap := make(map[string]*hsm.ReservationData)
			for _, res := range reservationData {
				resMap[res.XName] = res
			}
			for _, res := range resData {
				if _, ok := resMap[res.XName]; !ok {
					comp := xnameMap[res.XName]
					comp.Task.Status = model.TransitionTaskStatusFailed
					if res.DeputyKey == "" {
						// TODO: Check ExpirationTime and wait to try again?
						//       Could be that we restarted and just need to
						//       wait for the old locks to fall off.
						comp.Task.Error = "Unable to reserve component"
					} else {
						// We were given a deputy key that was invalid so we tried
						// reserving the component but we failed to reserve it.
						comp.Task.Error = "Invalid deputy key and unable to reserve component"
					}
					comp.Task.StatusDesc = "Failed to achieve transition"
					err = (*GLOB.DSP).StoreTransitionTask(*comp.Task)
					if err != nil {
						logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
					}
				}
			}
		}
		defer (*GLOB.HSM).ReleaseComponents(resData)
	}

	///////////////////////////////////////////////////////////////////////////
	// o seqMap[Action][Comptype] has by now been sorted to include what action(s)
	//   each component needs to have applied.
	//
	// o Power sequencing is controlled by the PowerSequenceFull array that defines
	//   an order of Actions to perform on a set of component types. The order is:
	//   1) GracefulShutdown/Off on Nodes+HSNBoards
	//   2) ForceOff on Nodes+HSNBoards
	//   3) GracefulShutdown/Off on Router+Compute Modules
	//   4) ForceOff on Router+Compute Modules
	//   5) GracefulShutdown/Off on Chassis
	//   6) ForceOff on Chassis
	//   7) GracefulShutdown/Off on CabinetPDUPowerConnector
	//   8) ForceOff on CabinetPDUPowerConnector
	//   9) Any GracefulRestarts
	//   10) On CabinetPDUPowerConnector
	//   11) On Chassis
	//   12) On Router+Compute Modules
	//   13) On Nodes+HSNBoards
	//
	// o TODO: Verify if GracefulRestart/ForceRestart happened?
	///////////////////////////////////////////////////////////////////////////

	for _, elm := range PowerSequenceFull {
		var compList []*TransitionComponent
		powerAction := elm.Action
		compTypes := elm.CompTypes
		// Get the list of components we'll be acting on
		for _, compType := range compTypes {
			list, ok := seqMap[powerAction][compType]
			if !ok || len(list) == 0 {
				continue
			}
			compList = append(compList, list...)
		}
		if len(compList) == 0 {
			continue
		}

		abort, _ := checkAbort(tr)
		if abort {
			doAbort(tr, xnameMap)
			return
		}

		// Check reservations are good
		err := (*GLOB.HSM).CheckDeputyKeys(resData)
		if err != nil {
			// TODO: Couldn't reach HSM. Retry?
			logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Couldn't check reservations")
		}
		for _, res := range resData {
			// Check res.Error for errors and fail components that don't have valid reservations.
			if res.Error != nil {
				comp, ok := xnameMap[res.XName]
				if !ok { continue }
				if comp.Task.Status == model.TransitionTaskStatusNew || 
				   comp.Task.Status == model.TransitionTaskStatusInProgress {
					comp.Task.Status = model.TransitionTaskStatusFailed
					comp.Task.Error = "Reservation expired"
					comp.Task.StatusDesc = "Failed to achieve transition"
					depErrMsg := fmt.Sprintf("Reservation expired for dependency, %s.", comp.Task.Xname)
					failDependentComps(xnameMap, powerAction, comp.Task.Xname, depErrMsg)
					err = (*GLOB.DSP).StoreTransitionTask(*comp.Task)
					if err != nil {
						logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
					}
				}
			}
		}

		// Create TRS task list
		trsTaskMap := make(map[uuid.UUID]*TransitionComponent)
		trsTaskList := (*GLOB.RFTloc).CreateTaskList(GLOB.BaseTRSTask, len(compList))
		trsTaskIdx := 0
		for _, comp := range compList {
			if comp.Task.Status == model.TransitionTaskStatusFailed {
				continue
			}
			if comp.Task.State != model.TaskState_Waiting {
				// Restarted task that we just need to wait to confirm transition.
				// Add it to the trsTaskMap but don't add it to the trsTaskList to
				// avoid resending the command.
				trsTaskMap[uuid.New()] = comp
				continue
			}
			payload, err := generateTransitionPayload(comp, powerAction)
			if err != nil {
				comp.Task.Status = model.TransitionTaskStatusFailed
				comp.Task.StatusDesc = "Failed to construct payload"
				comp.Task.Error = err.Error()
				err = (*GLOB.DSP).StoreTransitionTask(*comp.Task)
				if err != nil {
					logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
				}
				continue
			}

			comp.Task.StatusDesc = "Applying transition, " + powerAction
			comp.Task.State = model.TaskState_Sending
			setTaskOperation(comp.Task, powerAction)
			trsTaskMap[trsTaskList[trsTaskIdx].GetID()] = comp
			trsTaskList[trsTaskIdx].RetryPolicy.Retries = 3
			trsTaskList[trsTaskIdx].Request, _ = http.NewRequest("POST", "https://" + comp.HSMData.RfFQDN + comp.HSMData.PowerActionURI, bytes.NewBuffer([]byte(payload)))
			trsTaskList[trsTaskIdx].Request.Header.Set("Content-Type", "application/json")
			trsTaskList[trsTaskIdx].Request.Header.Add("HMS-Service", GLOB.BaseTRSTask.ServiceName)
			// Vault enabled?
			if GLOB.VaultEnabled {
				user, pw, err := (*GLOB.CS).GetControllerCredentials(comp.PState.XName)
				if err != nil {
					logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Unable to get credentials for " + comp.PState.XName)
				} // Retry credentials? Fail operation here? For now, just let it fail with empty credentials
				if !(user == "" && pw == "") {
					trsTaskList[trsTaskIdx].Request.SetBasicAuth(user, pw)
				}
			}
			trsTaskIdx++
			err = (*GLOB.DSP).StoreTransitionTask(*comp.Task)
			if err != nil {
				logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
			}
		}
		// Shrink the taskList to size incase we were left with empty ones
		trsTaskList = trsTaskList[:trsTaskIdx]

		// Launch the TRS tasks and wait to hear back
		if len(trsTaskList) > 0 {
			rchan, err := (*GLOB.RFTloc).Launch(&trsTaskList)
			if err != nil {
				logrus.Error(err)
			}
			for _, _ = range trsTaskList {
				var taskErr error
				tdone := <-rchan
				comp := trsTaskMap[tdone.GetID()]
				for i := 0; i < 1; i++ {

					if *tdone.Err != nil {
						taskErr = *tdone.Err
						break
					}
					if tdone.Request.Response.StatusCode < 200 && tdone.Request.Response.StatusCode >= 300 {
						taskErr = errors.New("bad status code: " + strconv.Itoa(tdone.Request.Response.StatusCode))
						break
					}
					if tdone.Request.Response.Body == nil {
						taskErr = errors.New("empty body")
						break
					}
					_, err := ioutil.ReadAll(tdone.Request.Response.Body)
					if err != nil {
						taskErr = err
						break
					}

				}
				comp.ActionCount--
				if taskErr != nil {
					comp.Task.Status = model.TransitionTaskStatusFailed
					comp.Task.Error = taskErr.Error()
					comp.Task.StatusDesc = "Failed to apply transition, " + powerAction
					logger.Log.WithFields(logrus.Fields{"ERROR": taskErr, "URI": tdone.Request.URL.String()}).Error("Redfish request failed")
					delete(trsTaskMap, tdone.GetID())
					depErrMsg := fmt.Sprintf("Failed to apply transition, %s, to dependency, %s.", powerAction, comp.Task.Xname)
					failDependentComps(xnameMap, powerAction, comp.Task.Xname, depErrMsg)
				} else {
					comp.Task.Status = model.TransitionTaskStatusInProgress
					comp.Task.StatusDesc = "Confirming successful transition, " + powerAction
					comp.Task.State = model.TaskState_Waiting
				}
				err = (*GLOB.DSP).StoreTransitionTask(*comp.Task)
				if err != nil {
					logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
				}
			}
			if (len(trsTaskMap) > 0) || !noWait {
				var waitExpireTime time.Time
				trsWaitTaskMap := trsTaskMap
				if !waitForever {
					waitExpireTime = time.Now().Add(time.Duration(tr.TaskDeadline) * time.Minute)
				}
				endState := ""
				switch(powerAction) {
				case "gracefulshutdown": fallthrough
				case "forceoff":
					endState = "off"
				case "gracefulrestart": fallthrough
				case "on":
					endState = "on"
				}
				for {
					abort, _ := checkAbort(tr)
					if abort {
						doAbort(tr, xnameMap)
						return
					}

					time.Sleep(15 * time.Second)
					// Create TRS task list
					trsTempTaskMap := make(map[uuid.UUID]*TransitionComponent)
					trsWaitTaskList := (*GLOB.RFTloc).CreateTaskList(GLOB.BaseTRSTask, len(trsWaitTaskMap))
					trsWaitTaskIdx := 0
					for _, comp := range trsWaitTaskMap {
						trsTempTaskMap[trsWaitTaskList[trsWaitTaskIdx].GetID()] = comp
						trsWaitTaskList[trsWaitTaskIdx].RetryPolicy.Retries = 3
						trsWaitTaskList[trsWaitTaskIdx].Request.URL, _ = url.Parse("https://" + comp.HSMData.RfFQDN + comp.HSMData.PowerStatusURI)
						// Vault enabled?
						if GLOB.VaultEnabled {
							user, pw, err := (*GLOB.CS).GetControllerCredentials(comp.PState.XName)
							if err != nil {
								logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Unable to get credentials for " + comp.PState.XName)
							} // Retry credentials? Fail operation here? For now, just let it fail with empty credentials
							if !(user == "" && pw == "") {
								trsWaitTaskList[trsWaitTaskIdx].Request.SetBasicAuth(user, pw)
							}
						}
						trsWaitTaskIdx++
					}
					trsWaitTaskMap = trsTempTaskMap
					// Launch the TRS tasks and wait to hear back
					rchan, err := (*GLOB.RFTloc).Launch(&trsWaitTaskList)
					if err != nil {
						logrus.Error(err)
					}
					for _, _ = range trsWaitTaskList {
						var taskErr error
						var hwStatus struct {
							PowerState string `json:"PowerState"`
						}
						tdone := <-rchan
						comp := trsWaitTaskMap[tdone.GetID()]
						for i := 0; i < 1; i++ {

							if *tdone.Err != nil {
								taskErr = *tdone.Err
								break
							}
							if tdone.Request.Response.StatusCode < 200 && tdone.Request.Response.StatusCode >= 300 {
								taskErr = errors.New("bad status code: " + strconv.Itoa(tdone.Request.Response.StatusCode))
								break
							}
							if tdone.Request.Response.Body == nil {
								taskErr = errors.New("empty body")
								break
							}
							body, err := ioutil.ReadAll(tdone.Request.Response.Body)
							if err != nil {
								taskErr = err
								break
							}
							err = json.Unmarshal(body, &hwStatus)
							if err != nil {
								taskErr = err
								break
							}

						}
						comp.ActionCount--
						if taskErr != nil {
							comp.Task.Status = model.TransitionTaskStatusFailed
							comp.Task.Error = taskErr.Error()
							comp.Task.StatusDesc = "Failed to confirm transition"
							logger.Log.WithFields(logrus.Fields{"ERROR": taskErr, "URI": tdone.Request.URL.String()}).Error("Redfish request failed")
							delete(trsWaitTaskMap, tdone.GetID())
							depErrMsg := fmt.Sprintf("Failed to confirm transition, %s, to dependency, %s.", powerAction, comp.Task.Xname)
							failDependentComps(xnameMap, powerAction, comp.Task.Xname, depErrMsg)
							err = (*GLOB.DSP).StoreTransitionTask(*comp.Task)
							if err != nil {
								logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
							}
						}
						if strings.ToLower(hwStatus.PowerState) == endState {
							if comp.ActionCount == 0 {
								comp.Task.Status = model.TransitionTaskStatusSucceeded
								comp.Task.StatusDesc = "Transition confirmed, " + powerAction
							} else {
								comp.Task.StatusDesc = "Transition confirmed, " + powerAction + ". Waiting for next transition"
							}
							comp.Task.State = model.TaskState_Confirmed
							delete(trsWaitTaskMap, tdone.GetID())
							err = (*GLOB.DSP).StoreTransitionTask(*comp.Task)
							if err != nil {
								logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
							}
						}
					}
					// The map is either empty because everything in this tier has been confirmed or has failed.
					if len(trsWaitTaskMap) > 0 {
						break
					}
					// Check to see if the time has expired.
					if !waitForever && time.Now().After(waitExpireTime) {
						// Add components that timed out to the ForceOff list (if we're doing ForceOff)
						if powerAction == "gracefulshutdown" && !isSoft {
							for _, comp := range trsWaitTaskMap {
								comp.ActionCount++
								compType := base.GetHMSType(comp.Task.Xname)
								seqMap["forceoff"][compType] = append(seqMap["forceoff"][compType], comp)
								comp.Task.State = model.TaskState_Confirmed
							}
							continue
						}
						// We have timed out and we have either tried ForceOff or are not doing a ForceOff.
						// Fail the leftover components.
						for _, comp := range trsWaitTaskMap {
							comp.Task.Status = model.TransitionTaskStatusFailed
							comp.Task.Error = fmt.Sprintf("Timeout waiting for transition, %s.", powerAction)
							comp.Task.StatusDesc = "Failed to achieve transition"
							depErrMsg := fmt.Sprintf("Timeout waiting for transition, %s, on dependency, %s.", powerAction, comp.Task.Xname)
							failDependentComps(xnameMap, powerAction, comp.Task.Xname, depErrMsg)
							err = (*GLOB.DSP).StoreTransitionTask(*comp.Task)
							if err != nil {
								logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
							}
						}
						break
					}
				}
			}
		}
	}

	///////////////////////////////////////////////////////////////////////////
	// o When Launch() completes, release any reservations PCS obtained for targets.
	///////////////////////////////////////////////////////////////////////////

	// (*GLOB.HSM).ReleaseComponents(resData) <- defered above

	///////////////////////////////////////////////////////////////////////////
	// o Once the service inst is done executing its task, "close out" the ETCD task
	//   record.  The reaper takes care of the rest.
	///////////////////////////////////////////////////////////////////////////

	// Task Complete
	tr.Status = model.TransitionStatusCompleted
	err = (*GLOB.DSP).StoreTransition(tr)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition")
	}
	return
}

func storeTransition(tr model.Transition) (bool, error) {
	abort := false
	for retry := 0; retry < 3; retry++ {
		// Get the transition
		trOld, err := (*GLOB.DSP).GetTransition(tr.TransitionID)
		if err != nil {
			if !strings.Contains(err.Error(), "does not exist") {
				logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error getting transition")
				return abort, err
			}
		}
		tr.LastActiveTime = time.Now()
		if trOld.TransitionID.String() != tr.TransitionID.String() {
			//Blank struct, do a normal store.
			err = (*GLOB.DSP).StoreTransition(tr)
			if err != nil {
				logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition")
			}
			return abort, err
		}
		if trOld.Status == model.TransitionStatusAbortSignaled {
			tr.Status = model.TransitionStatusAbortSignaled
			abort = true
		}
		// Use test and set to prevent overwriting another thread's store operation.
		ok, err := (*GLOB.DSP).TASTransition(tr, trOld)
		if err != nil {
			logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition")
		}
		if ok {
			return abort, nil
		}
	}
	err := errors.New("Retries expired storing transition")
	logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition")
	return abort, err
}

// Callback for watching for "Abort-signaled" on transitions to inform the main worker thread if it needs to abort.
// Done this way so even if the "Abort-signaled" status gets overwritten, the desire to abort is still captured.
func transitionAbortWatchCB(transition model.Transition, wasDeleted bool, err error, abortChan interface{}) bool {
	if err == nil && !wasDeleted {
		if transition.Status == model.TransitionStatusAbortSignaled {
			// Tell the main thread to abort
			abortChan.(chan bool) <- true
		}
		if transition.Status == model.TransitionStatusNew ||
		   transition.Status == model.TransitionStatusInProgress {
			// Continue watching
			return true
		}
	}
	// Stop watching
	return false
}

// Checks the channel for abort signals and returns true if atleast 1 was found.
// Empties the channel if more than one was found.
func checkAbort(tr model.Transition) (bool, error) {
	transition, err := (*GLOB.DSP).GetTransition(tr.TransitionID)
	if err != nil {
		if !strings.Contains(err.Error(), "does not exist") {
			logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error getting transition")
			return false, err
		} else {
			// No abort to check.
			return false, nil
		}
	}
	if transition.TransitionID.String() != tr.TransitionID.String() {
		// No abort to check.
		return false, nil
	}
	if transition.Status == model.TransitionStatusAbortSignaled {
		return true, nil
	}
	return false, nil
}

// Fail any tasks that have not finished and mark the transition as "Aborted".
func doAbort(tr model.Transition, xnameMap map[string]*TransitionComponent) {
	tr.Status = model.TransitionStatusAborted
	for _, comp := range xnameMap {
		if comp.Task.Status == model.TransitionTaskStatusNew ||
		   comp.Task.Status == model.TransitionTaskStatusInProgress {
			comp.Task.Status = model.TransitionTaskStatusFailed
			comp.Task.Error = "Transition aborted"
			comp.Task.StatusDesc = "Failed to achieve transition"
			err := (*GLOB.DSP).StoreTransitionTask(*comp.Task)
			if err != nil {
				logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
			}
		}
	}
	err := (*GLOB.DSP).StoreTransition(tr)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition")
	}
}

// Go to our internal power status mapping to retrieve a component hierarchy and power states.
// This will be faster than going to HSM and will include a recently collected set of hardware power states.
func getPowerStateHierarchy(xnames []string) (map[string]model.PowerStatusComponent, []string, error) {
	var badList []string
	xnameMap := make(map[string]model.PowerStatusComponent)
	for _, xname := range xnames {
		if _, ok := xnameMap[xname]; !ok {
			switch(base.GetHMSType(xname)) {
			case base.Node:                     fallthrough
			case base.CabinetPDUOutlet:         fallthrough
			case base.CabinetPDUPowerConnector: fallthrough
			case base.HSNBoard:
				pState, err := (*GLOB.DSP).GetPowerStatus(xname)
				if err != nil {
					if strings.Contains(err.Error(), "does not exist") {
						badList = append(badList, xname)
						continue
					} else {
						// Database error. Bail
						return nil, nil, err
					}
				} else {
					xnameMap[xname] = pState
				}
			case base.Chassis:                  fallthrough
			case base.ComputeModule:            fallthrough
			case base.RouterModule:
				pStates, err := (*GLOB.DSP).GetPowerStatusHierarchy(xname)
				if err != nil {
					// Database error. Bail
					return nil, nil, err
				}
				if len(pStates.Status) == 0 {
					badList = append(badList, xname)
					continue
				}
				for _, ps := range pStates.Status {
					switch(base.GetHMSType(ps.XName)) {
					case base.Chassis:                  fallthrough
					case base.ComputeModule:            fallthrough
					case base.Node:                     fallthrough
					case base.RouterModule:             fallthrough
					case base.HSNBoard:                 fallthrough
					case base.CabinetPDUPowerConnector:
						xnameMap[ps.XName] = ps
					}
				}
			default:
				badList = append(badList, xname)
			}
		}
	}
	return xnameMap, badList, nil
}

// Create an initial set of transition tasks from the transition parameters.
// This checks for previously existing tasks for the transition and adds them
// too.
func setupTransitionTasks(tr *model.Transition) (map[string]*TransitionComponent, []string) {
	var xnames []string
	xnameMap := make(map[string]*TransitionComponent)

	// Get any tasks that may have previously been created for our operation.
	tasks, err := (*GLOB.DSP).GetAllTasksForTransition(tr.TransitionID)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error retrieving tasks for transition, " + tr.TransitionID.String())
	}
	// Rebuild our xnameMap based on the previous tasks
	for _, task := range tasks {
		xnameMap[task.Xname] = &TransitionComponent{
			Task:      &task,
		}
	}
	// Make sure all the previously created tasks are in the transition's task array.
	if len(xnameMap) > 0 {
		taskMap := make(map[uuid.UUID]bool)
		for _, taskID := range tr.TaskIDs {
			taskMap[taskID] = true
		}
		for _, comp := range xnameMap {
			if _, ok := taskMap[comp.Task.TaskID]; !ok {
				tr.TaskIDs = append(tr.TaskIDs, comp.Task.TaskID)
			}
			if comp.Task.Status == model.TransitionTaskStatusNew ||
			   comp.Task.Status == model.TransitionTaskStatusInProgress {
				xnames = append(xnames, comp.Task.Xname)
			}
		}
	}

	// Vet and turn the list of requested xnames into a map
	for _, loc := range tr.Location {
		if comp, ok := xnameMap[loc.Xname]; ok {
			// Is a duplicate or from a restart.
			// Restarted tasks will need just the deputy key readded.
			if comp.DeputyKey == "" {
				comp.DeputyKey = loc.DeputyKey
			}
			continue
		}

		// Create tasks for everything requested so we can
		// communicate reasons for failures.
		task := model.NewTransitionTask(tr.TransitionID, tr.Operation)
		task.Xname = loc.Xname

		// Weed out invalid xnames and components we can't power control here.
		compType := base.GetHMSType(loc.Xname)
		switch(compType) {
		case base.Node:                     fallthrough
		case base.CabinetPDUOutlet:         fallthrough
		case base.CabinetPDUPowerConnector: fallthrough
		case base.HSNBoard:                 fallthrough
		case base.Chassis:                  fallthrough
		case base.ComputeModule:            fallthrough
		case base.RouterModule:
			task.StatusDesc = "Gathering data"
		case base.HMSTypeInvalid:
			task.Status = model.TransitionTaskStatusFailed
			task.Error = "Invalid xname"
			task.StatusDesc = "Failed to achieve transition"
		default:
			task.Status = model.TransitionTaskStatusFailed
			task.Error = "No power control for component type " + compType.String()
			task.StatusDesc = "Failed to achieve transition"
		}
		tr.TaskIDs = append(tr.TaskIDs, task.TaskID)
		err = (*GLOB.DSP).StoreTransitionTask(task)
		if err != nil {
			logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
		}
		xnameMap[loc.Xname] = &TransitionComponent{
			Task:      &task,
			DeputyKey: loc.DeputyKey,
		}
		if task.Status != model.TransitionTaskStatusFailed {
			xnames = append(xnames, loc.Xname)
		}
	}
	return xnameMap, xnames
}

func generateTransitionPayload(comp *TransitionComponent, action string) (string, error) {
	var body string
	compType := base.GetHMSType(comp.PState.XName)
	resetType, ok := comp.Actions[action]
	if !ok {
		if action == "gracefulshutdown" {
			resetType, ok = comp.Actions["off"]
			if !ok {
				return "", errors.New("Power action not supported for " + comp.PState.XName)
			}
		} else {
			return "", errors.New("Power action not supported for " + comp.PState.XName)
		}
	}
	if compType == base.CabinetPDUOutlet || compType == base.CabinetPDUPowerConnector {
		if !strings.Contains(comp.HSMData.RfFQDN, "rts") {
			outlet := strings.Split(comp.PState.XName, "v")
			if len(outlet) < 2 {
				return "", errors.New("Could not get outlet number from " + comp.PState.XName)
			}
			body = fmt.Sprintf(`{"OutletNumber":%s,"StartupState":"on","Outletname":"OUTLET%s","OnDelay":0,"OffDelay":0,"RebootDelay":5,"OutletStatus":"%s"}`, outlet[1], outlet[1], strings.ToLower(resetType))
		} else {
			body = fmt.Sprintf(`{"PowerState": "%s"}`, resetType)
		}
	} else {
		body = fmt.Sprintf(`{"ResetType": "%s"}`, resetType)
	}
	return body, nil
}

func setTaskOperation(task *model.TransitionTask, powerAction string) {
	switch(powerAction) {
	case "gracefulshutdown":
		task.Operation = model.Operation_Off
	case "gracefulrestart":
		task.Operation = model.Operation_SoftRestart
	case "forceoff":
		task.Operation = model.Operation_ForceOff
	case "forcerestart":
		task.Operation = model.Operation_HardRestart
	case "on":
		task.Operation = model.Operation_On
	}
}

// Checks if the failed component has components that depend on it having
// successfully transitioned. Check for:
//
// - If the failed component was an HSNBoard trying to be powered off because
//   that can cause damage. Find and fail parent RouterModule.
//
// - If the failed component was a node and the power action was soft-off (no ForceOff).
//   Find and fail parent ComputeModule.
//
// - If the failed component
//
//
// Other components will fail organically. Chassis won't power off if slots are
// still on and no child component will power on if the parent failed to power on.
// Setting tasks in xnameMap to failed affects the instances in the sequence map.
// The newly failed parent components will get skipped when it is their turn.
func failDependentComps(xnameMap map[string]*TransitionComponent, powerAction string, xname string, errMsg string) {
	parent := ""
	if (powerAction == "gracefulshutdown" || powerAction == "forceoff") && base.GetHMSType(xname) == base.HSNBoard {
		parent = base.GetHMSCompParent(xname)
	} else if powerAction == "gracefulshutdown" && base.GetHMSType(xname) == base.Node {
		parent = base.GetHMSCompParent(xname) // NodeBMC
		parent = base.GetHMSCompParent(parent) // ComputeModule
	}
	if parent != "" {
		pComp, ok := xnameMap[parent]
		if ok {
			// If we have the parent, set it to failed.
			pComp.Task.Status = model.TransitionTaskStatusFailed
			pComp.Task.Error = errMsg
			pComp.Task.StatusDesc = errMsg
			err := (*GLOB.DSP).StoreTransitionTask(*pComp.Task)
			if err != nil {
				logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
			}
		}
	}
}


