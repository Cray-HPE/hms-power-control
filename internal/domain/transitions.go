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
	"errors"
	"net/http"
	"strings"

	"github.com/Cray-HPE/hms-power-control/internal/logger"
	"github.com/Cray-HPE/hms-power-control/internal/model"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

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

func AbortTransitionID(transitionID uuid.UUID) (pb model.Passback) {
	//TODO stuff here!
	pb = model.BuildSuccessPassback(501, "AbortTransitionID")
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

	// // Start transition
	// go doTransitionTask(transition.TransitionID)

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


