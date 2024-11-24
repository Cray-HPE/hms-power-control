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
	"errors"
	"fmt"
	"io"
	"net/http"
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

type TransitionComponent struct {
	PState        *model.PowerStatusComponent
	HSMData       *hsm.HsmData
	PowerSupplies []PowerSupply
	Task          *model.TransitionTask
	Actions       map[string]string
	ActionCount   int // Number of actions until task competion
}

type PowerSeqElem struct {
	Action    string
	CompTypes []base.HMSType
	Wait      int
}

type PowerSupply struct {
	ID    string
	State model.PowerStateFilter
}

var PowerSequenceFull = []PowerSeqElem{
	{
		Action:    "gracefulshutdown",
		CompTypes: []base.HMSType{base.Node},
	}, {
		Action:    "forceoff",
		CompTypes: []base.HMSType{base.Node},
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
		// since power isn't being dropped doing them all (except BMCs) at the
		// same time should be fine.
		CompTypes: []base.HMSType{base.Node, base.RouterModule, base.ComputeModule, base.Chassis, base.CabinetPDUPowerConnector, base.MgmtSwitch, base.MgmtHLSwitch, base.CDUMgmtSwitch},
	}, {
		Action:    "gracefulrestart",
		// Restart BMCs after everything else because restarting the BMC will cause
		// redfish to temporarily become unresponsive.
		CompTypes: []base.HMSType{base.ChassisBMC, base.NodeBMC, base.RouterBMC},
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
		CompTypes: []base.HMSType{base.Node},
	},
}

func GetTransition(transitionID uuid.UUID) (pb model.Passback) {
	var tasks []model.TransitionTask
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
		return
	}
	// Compressed transitions don't have tasks anymore. No need to look them up.
	if !transition.IsCompressed {
		// Get the tasks for the transition
		tasks, err = (*GLOB.DSP).GetAllTasksForTransition(transitionID)
		if err != nil {
			pb = model.BuildErrorPassback(http.StatusInternalServerError, err)
			logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Error retrieving transition tasks")
			return
		}
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
		var tasks []model.TransitionTask
		// Compressed transitions don't have tasks anymore. No need to look them up.
		if !transition.IsCompressed {
			tasks, err = (*GLOB.DSP).GetAllTasksForTransition(transition.TransitionID)
			if err != nil {
				pb = model.BuildErrorPassback(http.StatusInternalServerError, err)
				logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Error retrieving transition tasks")
				return
			}
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
		if transition.Status == model.TransitionStatusCompleted {
			err := errors.New("Transition is already finished and cannot be aborted.")
			pb = model.BuildErrorPassback(http.StatusBadRequest, err)
			return
		}
		if transition.Status == model.TransitionStatusAborted {
			abortResp := model.TransitionAbortResp{AbortStatus: "Accepted - abort initiated"}
			pb = model.BuildSuccessPassback(http.StatusAccepted, abortResp)
			return
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
			abortResp := model.TransitionAbortResp{AbortStatus: "Accepted - abort initiated"}
			pb = model.BuildSuccessPassback(http.StatusAccepted, abortResp)
			return
		}
	}

	err := errors.New("Failed to signal abort")
	pb = model.BuildErrorPassback(http.StatusInternalServerError, err)
	logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Error storing abort-signaled status")
	return
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
		xnameHierarchy  []string
		isSoft          bool
		noWait          bool
		waitForever     bool
	)

	fname := "doTransition"

	tr, err := (*GLOB.DSP).GetTransition(transitionID)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Cannot retrieve transition, cannot generate tasks")
		return
	}

	defer logger.Log.Infof("Transition %s Completed", tr.TransitionID.String())

	// Restarting a transition
	if tr.Status != model.TransitionStatusNew {
		logger.Log.Infof("Restarting Transition %s", tr.TransitionID.String())
		if tr.Status == model.TransitionStatusCompleted ||
		   tr.Status == model.TransitionStatusAborted {
			// Shouldn't pick up completed Transitions anyway
			return
		}
	} else {
		logger.Log.Infof("Starting Transition %s", tr.TransitionID.String())
	}

	// Start the Keep Alive thread
	cancelChan := make(chan bool)
	go transitionKeepAlive(tr.TransitionID, cancelChan)
	// TODO: Where to close channel?

	if tr.Operation == model.Operation_SoftOff {
		isSoft = true
	}

	if tr.TaskDeadline == 0 {
		noWait = true
	} else if tr.TaskDeadline < 0 {
		waitForever = true
	}

	// Vet and turn the list of requested xnames into a map. This also
	// checks for previously created tasks for restarted transitions.
	xnameMap, xnames := setupTransitionTasks(&tr)

	if len(xnames) == 0 {
		// All xnames were invalid
		err = errors.New("No components to operate on")
		logrus.WithFields(logrus.Fields{"ERROR": err}).Error("No components to operate on")
		compressAndCompleteTransition(tr, model.TransitionStatusCompleted)
		return
	}

	// Store the transition with its initial set of tasks. May have more added later.
	tr.Status = model.TransitionStatusInProgress
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
		// TODO: Maybe retry indefinitely? For now exit. Another instance
		//       (possibly us) will restart this transition later.
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Cannot retrieve Power States, cannot continue")
		cancelChan <- true
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
			   compType != base.CabinetPDUPowerConnector &&
			   compType != base.ChassisBMC &&
			   compType != base.NodeBMC &&
			   compType != base.RouterBMC &&
			   compType != base.MgmtSwitch &&
			   compType != base.MgmtHLSwitch &&
			   compType != base.CDUMgmtSwitch {
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
		compressAndCompleteTransition(tr, model.TransitionStatusCompleted)
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
	
	///////////////////////////////////////////////////////////////////////////
	// o Get the power maps data from HSM.
	///////////////////////////////////////////////////////////////////////////
	err = (*GLOB.HSM).FillPowerMapData(hsmData)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error retrieving HSM power maps data")
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
		comp.PowerSupplies = getPowerSupplies(hData)

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
					PowerSupplies: getPowerSupplies(switchHData),
				}
			}
		}
	}

	abortSignaled, err = storeTransition(tr)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition")
	}
	if abortSignaled {
		doAbort(tr, xnameMap)
	}

	// Sort components into groups so they can follow a proper power sequence
	seqMap, reservationData := sequenceComponents(tr.Operation, xnameMap)

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
	resData, err := (*GLOB.HSM).ReserveComponents(reservationData)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error acquiring reservations")
		// An error occurred while reserving components. This does not include partial failure. Fail everything.
		for _, comp := range xnameMap {
			if comp.Task.Status != model.TransitionTaskStatusNew &&
			   comp.Task.Status != model.TransitionTaskStatusInProgress {
				continue
			}
			comp.Task.Status = model.TransitionTaskStatusFailed
			comp.Task.Error = err.Error()
			comp.Task.StatusDesc = "Error acquiring reservations"
			err = (*GLOB.DSP).StoreTransitionTask(*comp.Task)
			if err != nil {
				logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
			}
		}
		compressAndCompleteTransition(tr, model.TransitionStatusCompleted)
		return
	} else {
		// Check to see if we got everything back. ReserveComponents returns
		// everything that either was successfully reserved or had a valid
		// deputy key.
		resMap := make(map[string]*hsm.ReservationData)
		for _, res := range resData {
			resMap[res.XName] = res
		}
		for _, res := range reservationData {
			reservation, ok := resMap[res.XName]
			if !ok {
				var depErrMsg string
				var powerAction string
				comp := xnameMap[res.XName]
				if comp.Task.Status != model.TransitionTaskStatusNew &&
				   comp.Task.Status != model.TransitionTaskStatusInProgress {
					// Don't change status on already completed tasks.
					continue
				}
				comp.Task.Status = model.TransitionTaskStatusFailed
				if res.DeputyKey == "" {
					// TODO: Check ExpirationTime and wait to try again?
					//       Could be that we restarted and just need to
					//       wait for the old locks to fall off.
					comp.Task.Error = "Unable to reserve component"
					depErrMsg = fmt.Sprintf("Unable to reserve dependent component, %s.", comp.Task.Xname)
				} else {
					// We were given a deputy key that was invalid so we tried
					// reserving the component but we failed to reserve it.
					comp.Task.Error = "Invalid deputy key and unable to reserve component"
					depErrMsg = fmt.Sprintf("Invalid deputy key and unable to reserve dependent component, %s.", comp.Task.Xname)
				}
				comp.Task.StatusDesc = "Failed to achieve transition"
				switch(comp.Task.Operation) {
				case model.Operation_SoftRestart: fallthrough
				case model.Operation_HardRestart: fallthrough
				case model.Operation_SoftOff: fallthrough
				case model.Operation_Off:
					powerAction = "gracefulshutdown"
				case model.Operation_ForceOff:
					powerAction = "forceoff"
				case model.Operation_On:
					powerAction = "on"
				case model.Operation_Init:
					if strings.ToLower(comp.PState.PowerState) == "on" {
						powerAction = "gracefulshutdown"
					} else {
						powerAction = "on"
					}
				}
				failDependentComps(xnameMap, powerAction, comp.Task.Xname, depErrMsg)
				err = (*GLOB.DSP).StoreTransitionTask(*comp.Task)
				if err != nil {
					logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
				}
			} else {
				// Store the keys in the task
				comp := xnameMap[res.XName]
				comp.Task.ReservationKey = reservation.ReservationKey
				comp.Task.DeputyKey = reservation.DeputyKey
				err = (*GLOB.DSP).StoreTransitionTask(*comp.Task)
				if err != nil {
					logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
				}
			}
		}
		defer (*GLOB.HSM).ReleaseComponents(reservationData)
	}

	///////////////////////////////////////////////////////////////////////////
	// o seqMap[Action][Comptype] has by now been sorted to include what action(s)
	//   each component needs to have applied.
	//
	// o Power sequencing is controlled by the PowerSequenceFull array that defines
	//   an order of Actions to perform on a set of component types. The order is:
	//   1) GracefulShutdown/Off on Nodes
	//   2) ForceOff on Nodes
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
	//   13) On Nodes
	//
	// o TODO: Verify if GracefulRestart/ForceRestart happened?
	///////////////////////////////////////////////////////////////////////////

	waitForBMCPower := false
	for _, elm := range PowerSequenceFull {
		var compList []*TransitionComponent
		powerAction := elm.Action
		powerActionOp := getOpForPowerAction(powerAction)
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

		// The previous power operation resulted in supplied power
		// to child BMCs. Give the BMCs time to power on.
		if waitForBMCPower {
			waitForBMC(compList)
			waitForBMCPower = false
		}

		abort, _ := checkAbort(tr)
		if abort {
			doAbort(tr, xnameMap)
			return
		}

		// Check reservations are good
		err := (*GLOB.HSM).CheckDeputyKeys(reservationData)
		if err != nil {
			// TODO: Couldn't reach HSM. Retry?
			logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Couldn't check reservations")
		}
		for _, res := range reservationData {
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

		// Repeated and frequent power transitions to the same BMCs is not
		// common so we use the default TRS configuration provided by the
		// default BaseTRSTask task prototype.  It may be beneficial to
		// consider sharing the PCS TRS client in the future as requesting
		// power state transitions generally shares the same set of BMC targets
		// we want to talk to

		// Create TRS task list
		trsTaskMap := make(map[uuid.UUID]*TransitionComponent)
		trsTaskList := (*GLOB.RFTloc).CreateTaskList(GLOB.BaseTRSTask, len(compList))
		trsTaskIdx := 0
		for _, comp := range compList {
			if comp.Task.Status == model.TransitionTaskStatusFailed {
				continue
			}
			if comp.Task.State == model.TaskState_Waiting &&
			   comp.Task.Operation == powerActionOp {
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
				depErrMsg := fmt.Sprintf("Failed to apply transition, %s, to dependency, %s.", powerAction, comp.Task.Xname)
				failDependentComps(xnameMap, powerAction, comp.Task.Xname, depErrMsg)
				err = (*GLOB.DSP).StoreTransitionTask(*comp.Task)
				if err != nil {
					logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
				}
				continue
			}

			comp.Task.StatusDesc = "Applying transition, " + powerAction
			comp.Task.State = model.TaskState_Sending
			comp.Task.Operation = powerActionOp
			trsTaskMap[trsTaskList[trsTaskIdx].GetID()] = comp
			trsTaskList[trsTaskIdx].CPolicy.Retry.Retries = 3
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
			logger.Log.Infof("%s: Initiating %d/%d transition requests to BMCs",
						     fname, trsTaskIdx, len(compList))

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
					_, err := io.ReadAll(tdone.Request.Response.Body)

					// Must always close response bodies
					tdone.Request.Response.Body.Close()

					if err != nil {
						taskErr = err
						break
					}
				}
				if taskErr != nil {
					comp.Task.Status = model.TransitionTaskStatusFailed
					comp.Task.Error = taskErr.Error()
					comp.Task.StatusDesc = "Failed to apply transition, " + powerAction
					logger.Log.WithFields(logrus.Fields{"ERROR": taskErr, "URI": tdone.Request.URL.String()}).Error("Redfish request failed")
					delete(trsTaskMap, tdone.GetID())
					depErrMsg := fmt.Sprintf("Failed to apply transition, %s, to dependency, %s.", powerAction, comp.Task.Xname)
					failDependentComps(xnameMap, powerAction, comp.Task.Xname, depErrMsg)
				} else if noWait {
					comp.ActionCount--
					if comp.ActionCount == 0 {
						comp.Task.Status = model.TransitionTaskStatusSucceeded
						comp.Task.StatusDesc = fmt.Sprintf("Transition applied, %s. Not confirming.", powerAction)
						comp.Task.State = model.TaskState_Confirmed
					} else {
						comp.Task.Status = model.TransitionTaskStatusInProgress
						comp.Task.StatusDesc = fmt.Sprintf("Transition applied, %s. Waiting for next transition.", powerAction)
						comp.Task.State = model.TaskState_Confirmed
					}
				} else {
					comp.Task.Status = model.TransitionTaskStatusInProgress
					comp.Task.StatusDesc = "Confirming successful transition, " + powerAction
					comp.Task.State = model.TaskState_Waiting
				}
				err = (*GLOB.DSP).StoreTransitionTask(*comp.Task)
				if err != nil {
					logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
				}

				// Cancel task contexts after they're no longer needed
				if tdone.ContextCancel != nil {
					tdone.ContextCancel()
				}
			}
			(*GLOB.RFTloc).Close(&trsTaskList)
			close(rchan)
			logger.Log.Infof("%s: Done processing BMC responses", fname)
		} else {
			// Free up this memory
			(*GLOB.RFTloc).Close(&trsTaskList)
		}

		// TRS section for getting power state for confirmation.
		if len(trsTaskMap) > 0 || !noWait {
			var waitExpireTime time.Time
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

				// The update interval for power status in ETCD is 30 seconds but we could get an update sooner.
				time.Sleep(15 * time.Second)
				for trsTaskID, comp := range trsTaskMap {
					// Get the state from ETCD
					pState, err := (*GLOB.DSP).GetPowerStatus(comp.Task.Xname)
					if err != nil {
						comp.Task.Status = model.TransitionTaskStatusFailed
						comp.Task.Error = err.Error()
						comp.Task.StatusDesc = "Failed to confirm transition"
						if !strings.Contains(err.Error(), "does not exist") {
							// Database error
							logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error getting power status from database")
						}
						delete(trsTaskMap, trsTaskID)
						depErrMsg := fmt.Sprintf("Failed to confirm transition, %s, to dependency, %s.", powerAction, comp.Task.Xname)
						failDependentComps(xnameMap, powerAction, comp.Task.Xname, depErrMsg)
						err = (*GLOB.DSP).StoreTransitionTask(*comp.Task)
						if err != nil {
							logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
						}
					} else if strings.ToLower(pState.PowerState) == endState {
						comp.ActionCount--
						comp.Task.State = model.TaskState_Confirmed
						if comp.ActionCount == 0 {
							comp.Task.Status = model.TransitionTaskStatusSucceeded
							comp.Task.StatusDesc = "Transition confirmed, " + powerAction
						} else {
							comp.Task.StatusDesc = "Transition confirmed, " + powerAction + ". Waiting for next transition"
						}
						delete(trsTaskMap, trsTaskID)
						err = (*GLOB.DSP).StoreTransitionTask(*comp.Task)
						if err != nil {
							logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
						}
					}
				}
				// The map is either empty because everything in this tier has been confirmed or has failed.
				if len(trsTaskMap) == 0 {
					break
				}
				// Check to see if the time has expired.
				if !waitForever && time.Now().After(waitExpireTime) {
					for _, comp := range trsTaskMap {
						_, hasForceOff := comp.Actions["forceoff"]
						if powerAction == "gracefulshutdown" && !isSoft && hasForceOff {
							// Add components that timed out to the ForceOff list (if we're doing ForceOff)
							compType := base.GetHMSType(comp.Task.Xname)
							seqMap["forceoff"][compType] = append(seqMap["forceoff"][compType], comp)
						} else {
							// We have timed out and we have either tried ForceOff or are not doing a ForceOff.
							// Fail the leftover components.
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
					}
					break
				}
			}
			// If we might be powering on child components, we'll
			// want to give their BMCs some time to become ready.
			if powerAction == "on" {
				for _, compType := range compTypes {
					if compType == base.RouterModule ||
					   compType == base.ComputeModule {
						waitForBMCPower = true
					}
				}
			} else if powerAction == "gracefulrestart" {
				for _, compType := range compTypes {
					if compType == base.ChassisBMC || 
					   compType == base.NodeBMC || 
					   compType == base.RouterBMC {
						waitForBMCPower = true
					}
				}
			}
		}
	}

	///////////////////////////////////////////////////////////////////////////
	// o When Launch() completes, release any reservations PCS obtained for targets.
	///////////////////////////////////////////////////////////////////////////

	// (*GLOB.HSM).ReleaseComponents(reservationData) <- defered above

	///////////////////////////////////////////////////////////////////////////
	// o Once the service inst is done executing its task, "close out" the ETCD task
	//   record.  The reaper takes care of the rest.
	///////////////////////////////////////////////////////////////////////////

	// Task Complete
	compressAndCompleteTransition(tr, model.TransitionStatusCompleted)
	return
}

// Checks for AbortSignaled before storing the transition using test-and-set to
// avoid overwriting statuses set by other instances. Retries the TAS operation
// a few times upon failure so it may eventually happen.
//
// Returns true if an abort was signaled.
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
	for _, comp := range xnameMap {
		if comp.Task.Status == model.TransitionTaskStatusNew ||
		   comp.Task.Status == model.TransitionTaskStatusInProgress {
			comp.Task.Status = model.TransitionTaskStatusFailed
			comp.Task.Error = "Transition aborted"
			comp.Task.StatusDesc = "Aborted. Last status - " + comp.Task.StatusDesc
			err := (*GLOB.DSP).StoreTransitionTask(*comp.Task)
			if err != nil {
				logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
			}
		}
	}
	compressAndCompleteTransition(tr, model.TransitionStatusAborted)
}

// Periodically updates the LastActiveTime field of the given transition. Will kill itself
// if the transition moves to the Completed or Aborted state or gets deleted.
func transitionKeepAlive(transitionID uuid.UUID, cancelChan chan bool) {
	logger.Log.Infof("Starting keep alive for Transition, %s.", transitionID.String())
	keepAlive := time.NewTicker(time.Duration(model.TransitionKeepAliveInterval) * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case <-cancelChan:
			logger.Log.Infof("Keep alive for Transition, %s, has been cancelled.", transitionID.String())
			return
		case <-keepAlive.C:
			for {
				// Get the transition
				transition, err := (*GLOB.DSP).GetTransition(transitionID)
				if err != nil {
					if strings.Contains(err.Error(), "does not exist") {
						logger.Log.WithFields(logrus.Fields{"ERROR": err}).Errorf("Transition, %s, does not exist, stopping keep alive thread", transitionID.String())
						return
					} else {
						logger.Log.WithFields(logrus.Fields{"ERROR": err}).Errorf("Error retreiving Transition, %s, retrying...", transitionID.String())
						continue
					}
				}
				if transition.TransitionID.String() != transitionID.String() {
					err := errors.New("TransitionID does not exist")
					logger.Log.WithFields(logrus.Fields{"ERROR": err}).Errorf("Transition, %s, does not exist, stopping keep alive thread", transitionID.String())
					return
				}

				// End states
				if transition.Status == model.TransitionStatusAborted ||
				   transition.Status == model.TransitionStatusCompleted {
					logger.Log.Infof("Transition %s is finished. Stopping keep alive thread", transitionID.String())
					return
				}
				transitionOld := transition
				// Only change the LastActiveTime
				transition.LastActiveTime = time.Now()
				// Use test and set to prevent overwriting another thread's store operation.
				ok, err := (*GLOB.DSP).TASTransition(transition, transitionOld)
				if err != nil {
					logger.Log.WithFields(logrus.Fields{"ERROR": err}).Errorf("Error storing Transition, %s, retrying...", transitionID.String())
					continue
				}
				if ok {
					break
				}
			}
		}
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
			case base.ChassisBMC:    fallthrough
			case base.NodeBMC:       fallthrough
			case base.RouterBMC:     fallthrough
			case base.Node:          fallthrough
			case base.Chassis:       fallthrough
			case base.ComputeModule: fallthrough
			case base.MgmtSwitch:    fallthrough
			case base.MgmtHLSwitch:  fallthrough
			case base.CDUMgmtSwitch: fallthrough
			case base.CabinetPDUPowerConnector:
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
			case base.RouterModule:
				found := false
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
					case base.RouterModule:
						xnameMap[ps.XName] = ps
						// Make sure our original xname is part
						// of the list of components found.
						if ps.XName == xname {
							found = true
						}
					}
				}
				if !found {
					badList = append(badList, xname)
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
	for i, task := range tasks {
		xnameMap[task.Xname] = &TransitionComponent{
			Task:      &tasks[i],
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
		if _, ok := xnameMap[loc.Xname]; ok {
			continue
		}

		// Create tasks for everything requested so we can
		// communicate reasons for failures.
		task := model.NewTransitionTask(tr.TransitionID, tr.Operation)
		task.Xname = loc.Xname
		task.DeputyKey = loc.DeputyKey

		// Weed out invalid xnames and components we can't power control here.
		compType := base.GetHMSType(loc.Xname)
		switch(compType) {
		case base.ChassisBMC:    fallthrough
		case base.NodeBMC:       fallthrough
		case base.RouterBMC:     fallthrough
		case base.Node:          fallthrough
		case base.Chassis:       fallthrough
		case base.ComputeModule: fallthrough
		case base.RouterModule:  fallthrough
		case base.MgmtSwitch:    fallthrough
		case base.MgmtHLSwitch:  fallthrough
		case base.CDUMgmtSwitch: fallthrough
		case base.CabinetPDUPowerConnector:
			task.StatusDesc = "Gathering data"
		case base.HMSTypeInvalid:
			task.Status = model.TransitionTaskStatusFailed
			task.Error = "Invalid xname"
			task.StatusDesc = "Failed to achieve transition"
		default:
			task.Status = model.TransitionTaskStatusUnsupported
			task.Error = "No power control for component type " + compType.String()
			task.StatusDesc = "Failed to achieve transition"
		}
		tr.TaskIDs = append(tr.TaskIDs, task.TaskID)
		err = (*GLOB.DSP).StoreTransitionTask(task)
		if err != nil {
			logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
		}
		xnameMap[loc.Xname] = &TransitionComponent{Task: &task}
		if task.Status == model.TransitionTaskStatusNew {
			xnames = append(xnames, loc.Xname)
		}
	}
	return xnameMap, xnames
}

// Sorts components into groups by power action then comptype so they can follow a proper power sequence.
func sequenceComponents(operation model.Operation, xnameMap map[string]*TransitionComponent) (map[string]map[base.HMSType][]*TransitionComponent, []hsm.ReservationData) {
	var resData []hsm.ReservationData
	seqMap := map[string]map[base.HMSType][]*TransitionComponent{
		"on":               make(map[base.HMSType][]*TransitionComponent),
		"gracefulshutdown": make(map[base.HMSType][]*TransitionComponent),
		"forceoff":         make(map[base.HMSType][]*TransitionComponent),
		"gracefulrestart":  make(map[base.HMSType][]*TransitionComponent),
	}

	for xname, comp := range xnameMap {
		if comp.Task.Status != model.TransitionTaskStatusNew &&
		   comp.Task.Status != model.TransitionTaskStatusInProgress {
			// Add completed tasks that might already have a valid reservation
			// in HSM to our reservations list so they'll get properly released.
			if comp.Task.ReservationKey != "" {
				res := hsm.ReservationData{
					XName: xname,
					ReservationKey: comp.Task.ReservationKey,
					DeputyKey: comp.Task.DeputyKey,
				}
				resData = append(resData, res)
			}
			
			continue
		}

		compType := base.GetHMSType(xname)

		isBMC := base.IsHMSTypeController(compType)
		supportsOp := false
		for _, op := range comp.PState.SupportedPowerTransitions {
			if op == operation.String() {
				supportsOp = true
			}
		}
		if !supportsOp {
			comp.Task.Status = model.TransitionTaskStatusUnsupported
			comp.Task.StatusDesc = fmt.Sprintf("Component does not support the specified transition operation, %s", operation.String())
			comp.Task.Error = "Unsupported for transition operation"
			err := (*GLOB.DSP).StoreTransitionTask(*comp.Task)
			if err != nil {
				logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
			}
			continue
		}

		psf, _ := model.ToPowerStateFilter(comp.PState.PowerState)
		switch(operation) {
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
				if isBMC && !hasAction {
					// Controllers only support ForceRestart redfish resetTypes
					_, hasAction = comp.Actions["forcerestart"]
				}
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
							_, hasGracefulRestart := parent.Actions["gracefulrestart"]
							_, hasForceRestart := parent.Actions["forcerestart"]
							if !hasGracefulRestart && !hasForceRestart {
								parentSupportsRestart = false
							}
						} else {
							id = base.GetHMSCompParent(id)
						}
					}
				}
				if hasAction && parentSupportsRestart {
					if comp.Task.State == model.TaskState_Waiting {
						comp.Task.Status = model.TransitionTaskStatusSucceeded
						comp.Task.StatusDesc = "Transition confirmed, gracefulrestart"
					} else {
						comp.ActionCount++
						seqMap["gracefulrestart"][compType] = append(seqMap["gracefulrestart"][compType], comp)
					}
				} else if isBMC {
						// There are parent components in our request that will
						// be powered off->on. BMCs don't support on/off operations
						// and will get reset anyway as a result of the parent's
						// power off->on.
						comp.Task.Status = model.TransitionTaskStatusSucceeded
						comp.Task.StatusDesc = fmt.Sprintf("Component will be reset as a result of its parent component getting powered off->on.")
				} else {
					if comp.Task.Operation == model.Operation_On {
						comp.Task.Status = model.TransitionTaskStatusSucceeded
						comp.Task.StatusDesc = "Transition confirmed, On"
					} else {
						comp.ActionCount += 2
						seqMap["gracefulshutdown"][compType] = append(seqMap["gracefulshutdown"][compType], comp)
						seqMap["on"][compType] = append(seqMap["on"][compType], comp)
					}
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
				if comp.Task.Operation == model.Operation_On {
					comp.Task.Status = model.TransitionTaskStatusSucceeded
					comp.Task.StatusDesc = "Transition confirmed, On"
				} else {
					comp.ActionCount += 2
					seqMap["gracefulshutdown"][compType] = append(seqMap["gracefulshutdown"][compType], comp)
					seqMap["on"][compType] = append(seqMap["on"][compType], comp)
				}
			}
		case model.Operation_Init:
			if psf == model.PowerStateFilter_On {
				if comp.Task.Operation == model.Operation_On ||
				   ((comp.Task.Operation == model.Operation_Off ||
				     comp.Task.Operation == model.Operation_ForceOff) &&
				    comp.Task.State == model.TaskState_Confirmed) {
					// We previously confirmed an off command but was restarted before we could send the On command.
					// However, upon restarting, the component is On. Success?
					comp.Task.Status = model.TransitionTaskStatusSucceeded
					comp.Task.StatusDesc = "Transition confirmed, on"
				} else if comp.Task.Operation == model.Operation_ForceOff {
					// Restarted ForceOff stage either Sending or waiting.
					comp.ActionCount += 2
					seqMap["forceoff"][compType] = append(seqMap["forceoff"][compType], comp)
					seqMap["on"][compType] = append(seqMap["on"][compType], comp)
				} else {
					comp.ActionCount += 2
					seqMap["gracefulshutdown"][compType] = append(seqMap["gracefulshutdown"][compType], comp)
					seqMap["on"][compType] = append(seqMap["on"][compType], comp)
				}
			} else {
				if comp.Task.Operation == model.Operation_Off ||
				   comp.Task.Operation == model.Operation_ForceOff {
					comp.Task.StatusDesc = "Transition confirmed, off. Waiting for next transition"
					comp.Task.State = model.TaskState_Confirmed
				}
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
		   comp.Task.Status == model.TransitionTaskStatusInProgress ||
		   comp.Task.ReservationKey != "" {
			res := hsm.ReservationData{
				XName: xname,
				ReservationKey: comp.Task.ReservationKey, // Only present if the transition was restarted
				DeputyKey: comp.Task.DeputyKey,
			}
			resData = append(resData, res)
		}
		err := (*GLOB.DSP).StoreTransitionTask(*comp.Task)
		if err != nil {
			logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
		}
	}
	return seqMap, resData
}

// Builds a json payload for the redfish command to apply the given power action.
// The power action comes from the sequence array and gets translated into a redfish
// value the hardware supports.
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
		} else if action == "gracefulrestart" {
			resetType, ok = comp.Actions["forcerestart"]
			if !ok {
				return "", errors.New("Power action not supported for " + comp.PState.XName)
			}
		} else {
			return "", errors.New("Power action not supported for " + comp.PState.XName)
		}
	}
	if compType == base.CabinetPDUPowerConnector {
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

// Translate sequence power actions into TaskOperations for database
// retention if the transition gets restarted.
func getOpForPowerAction(powerAction string) model.Operation {
	op := model.Operation_Nil
	switch(powerAction) {
	case "gracefulshutdown":
		op = model.Operation_Off
	case "gracefulrestart":
		op = model.Operation_SoftRestart
	case "forceoff":
		op = model.Operation_ForceOff
	case "forcerestart":
		op = model.Operation_HardRestart
	case "on":
		op = model.Operation_On
	}
	return op
}

// Checks if the failed component has components that depend on it having
// successfully transitioned. Check for:
//
// - If the failed component was a node and the power action was soft-off (no ForceOff).
//   Find and fail parent ComputeModule.
//
// - Check to see if any components supplying power are
//   in the list. If so and it would result in the component becoming unpowered, fail the
//   operation for one of the components that supply power.
//
//
// Other components will fail organically. Chassis won't power off if slots are
// still on and no child component will power on if the parent failed to power on.
// Setting tasks in xnameMap to failed affects the instances in the sequence map.
// The newly failed parent components will get skipped when it is their turn.
func failDependentComps(xnameMap map[string]*TransitionComponent, powerAction string, xname string, errMsg string) {
	var parents []string
	if powerAction == "gracefulshutdown" && base.GetHMSType(xname) == base.Node {
		parent := base.GetHMSCompParent(xname) // NodeBMC
		parent = base.GetHMSCompParent(parent) // ComputeModule
		parents = append(parents, parent)
	}
	if len(parents) > 0 {
		comp, ok := xnameMap[xname]
		// If we have powerMap data, look at the list of components supplying power
		// to the one we're working on. If there is at least one power supply in the
		// 'on' state that is not in our operation list, there is no need to fail any
		// in our operation list.
		if ok && len(comp.PowerSupplies) > 0 {
			willHavePower := false
			failSupplyXname := ""
			for _, supply := range comp.PowerSupplies {
				if _, found := xnameMap[supply.ID]; !found && supply.State == model.PowerStateFilter_On {
					willHavePower = true
					break
				} else if supply.State == model.PowerStateFilter_On {
					// It doesn't matter if there is more than one in our list. We only
					// need to fail one of them. We'll take the last one.
					failSupplyXname = supply.ID
				}
			}
			if !willHavePower && failSupplyXname != "" {
				parents = append(parents, failSupplyXname)
			}
		}
		for _, parent := range parents {
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
}

// Checks all of the transition records in etcd and does the following:
//
// - Deletes completed (completed/aborted) records that have expired (AutomaticExpirationTime).
//
// - Restarts incomplete transitions (new/in-progress) that have been abandoned (LastActiveTime > 3*TransitionKeepAliveInterval).
//
// - Aborts incomplete transitions (new/in-progress) that have expired (AutomaticExpirationTime).
func transitionsReaper() {
	// Get all transitions
	transitions, err := (*GLOB.DSP).GetAllTransitions()
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error retreiving transitions")
		return
	}
	if len(transitions) == 0 {
		// No transitions to act upon
		return
	}

	numComplete := 0
	for _, transition := range transitions {
		expired := transition.AutomaticExpirationTime.Before(time.Now())
		abandoned := transition.LastActiveTime.Before(time.Now().Add(time.Duration(model.TransitionKeepAliveInterval) * -3 * time.Second))
		if expired {
			if transition.Status == model.TransitionStatusAborted ||
			   transition.Status == model.TransitionStatusCompleted {
				deleteTransition(transition.TransitionID)
			} else {
				transitionOld := transition
				transition.Status = model.TransitionStatusAbortSignaled
				if abandoned {
					transition.LastActiveTime = time.Now()
				}
				// No need to check if the TAS succeeded because, if it didn't,
				// it means someone else took care of it.
				ok, err := (*GLOB.DSP).TASTransition(transition, transitionOld)
				if err != nil {
					logger.Log.WithFields(logrus.Fields{"ERROR": err}).Errorf("Error aborting transition, %s.", transition.TransitionID.String())
					continue
				}
				// Pick it up to process the abort if it has been abandoned.
				if ok && abandoned {
					go doTransition(transition.TransitionID)
				}
			}
		} else if abandoned &&
		          transition.Status != model.TransitionStatusAborted && 
		          transition.Status != model.TransitionStatusCompleted {
			// Assume the transition has been abandoned if it has been 3 times
			// the keep alive interval since it was last active.
			// Pick up an abandoned transition by first refreshing its LastActiveTime
			// so other instances know we got it first.
			transitionOld := transition
			transition.LastActiveTime = time.Now()
			ok, err := (*GLOB.DSP).TASTransition(transition, transitionOld)
			if err != nil {
				logger.Log.WithFields(logrus.Fields{"ERROR": err}).Errorf("Error storing transition, %s.", transition.TransitionID.String())
				continue
			}
			// ok == true mean we got it first. Start the processing thread.
			if ok {
				go doTransition(transition.TransitionID)
			}
		} else if transition.Status == model.TransitionStatusAborted ||
		          transition.Status == model.TransitionStatusCompleted {
			numComplete++
			// Compress the completed transition if the it was not previuosly compressed
			// upon completion (probably because it was stored by an older version
			// of PCS).
			if !transition.IsCompressed {
				compressAndCompleteTransition(transition, transition.Status)
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
		// Find the oldest 'numDelete' records and delete them.
		tToDelete := make([]*model.Transition, numDelete)
		for t, transition := range transitions {
			if transition.Status != model.TransitionStatusAborted &&
			   transition.Status != model.TransitionStatusCompleted {
				continue
			}
			for i := 0; i < numDelete; i++ {
				if tToDelete[i] == nil {
					tToDelete[i] = &transitions[t]
					break
				} else if tToDelete[i].CreateTime.After(transition.CreateTime) {
					// Found an older record. Shift the array elements.
					currTransition := &transitions[t]
					for j := i; j < numDelete; j++ {
						if tToDelete[j] == nil {
							tToDelete[j] = currTransition
							break
						}
						tmpTransition := tToDelete[j]
						tToDelete[j] = currTransition
						currTransition = tmpTransition
					}
					break
				}
			}
		}
		for _, transition := range tToDelete {
			deleteTransition(transition.TransitionID)
		}
	}
}

// Deletes the transition and any associated tasks. 
func deleteTransition(transitionID uuid.UUID) error {
	// Get the tasks for the transition
	tasks, err := (*GLOB.DSP).GetAllTasksForTransition(transitionID)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Errorf("Error retreiving tasks for transition, %s.", transitionID.String())
		return err
	}
	for _, task := range tasks {
		err = (*GLOB.DSP).DeleteTransitionTask(transitionID, task.TaskID)
		if err != nil {
			logger.Log.WithFields(logrus.Fields{"ERROR": err}).Errorf("Error deleting transition task, %s.", task.TaskID.String())
			return err
		}
	}
	err = (*GLOB.DSP).DeleteTransition(transitionID)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Errorf("Error deleting transition, %s.", transitionID.String())
		return err
	}
	return nil
}

// Wait for BMCs to become responsive. This waits for the component's
// ManagementState to become available.
func waitForBMC(compList []*TransitionComponent) {
	// Wait a max of ~5mins. As of 01/12/2023 the wait time is ~3mins.
	for retry := 0; retry < 20; retry++ {
		isWaiting := false
		for _, comp := range compList {
			if retry == 0 {
				comp.Task.StatusDesc = "Waiting for controller to be ready"
				err := (*GLOB.DSP).StoreTransitionTask(*comp.Task)
				if err != nil {
					logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition task")
				}
			}
			// Get the state from ETCD
			pState, err := (*GLOB.DSP).GetPowerStatus(comp.Task.Xname)
			if err != nil {
				// If everything ends up being an error. We'll just stop waiting.
				logger.Log.WithFields(logrus.Fields{"ERROR": err}).Errorf("Error getting power status from database for %s", comp.Task.Xname)
			} else if strings.ToLower(pState.ManagementState) != model.ManagementStateFilter_available.String() {
				isWaiting = true
			}
		}
		if !isWaiting {
			break
		}
		time.Sleep(15 * time.Second)
	}
}

func getPowerSupplies(hData *hsm.HsmData) (powerSupplies []PowerSupply) {
	for _, pConnector := range hData.PoweredBy {
		powerState := model.PowerStateFilter_Undefined
		pState, err := (*GLOB.DSP).GetPowerStatus(pConnector)
		if err == nil {
			powerState, _ = model.ToPowerStateFilter(pState.PowerState)
		}
		powerSupply := PowerSupply{
			ID:    pConnector,
			State: powerState,
		}
		powerSupplies = append(powerSupplies, powerSupply)
	}
	return
}

func compressAndCompleteTransition(transition model.Transition, status string) {
	// Get the tasks for the transition
	tasks, err := (*GLOB.DSP).GetAllTasksForTransition(transition.TransitionID)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Errorf("Error retreiving tasks for transition, %s.", transition.TransitionID.String())
		return
	}
	// Build the response struct
	rsp := model.ToTransitionResp(transition, tasks, true)
	transition.TaskCounts = rsp.TaskCounts
	transition.Tasks = rsp.Tasks
	transition.Status = status
	transition.IsCompressed = true
	err = (*GLOB.DSP).StoreTransition(transition)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Error storing transition")
		// Don't delete the tasks if we were unsuccessful storing the transition.
		return
	}
	for _, task := range tasks {
		err = (*GLOB.DSP).DeleteTransitionTask(transition.TransitionID, task.TaskID)
		if err != nil {
			logger.Log.WithFields(logrus.Fields{"ERROR": err}).Errorf("Error deleting transition task, %s.", task.TaskID.String())
		}
	}
	return
}
