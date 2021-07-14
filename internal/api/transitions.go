/*
 * MIT License
 *
 * (C) Copyright [2021] Hewlett Packard Enterprise Development LP
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

package api

import (
	"encoding/json"
	"errors"
	"github.com/Cray-HPE/hms-power-control/internal/logger"
	"io/ioutil"
	"net/http"

	"github.com/Cray-HPE/hms-power-control/internal/domain"
	"github.com/Cray-HPE/hms-power-control/internal/model"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// CreateTransition - creates a transition and will trigger a 'transition' flow
func CreateTransition(w http.ResponseWriter, req *http.Request) {
	var pb model.Passback
	var parameters model.TransitionParameter
	if req.Body != nil {
		body, err := ioutil.ReadAll(req.Body)
		logger.Log.WithFields(logrus.Fields{"body": string(body)}).Trace("Printing request body")

		if err != nil {
			pb := model.BuildErrorPassback(http.StatusInternalServerError, err)
			logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Error detected retrieving body")
			WriteHeaders(w, pb)
			return
		}

		err = json.Unmarshal(body, &parameters)
		if err != nil {
			pb = model.BuildErrorPassback(http.StatusBadRequest, err)
			logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Unparseable json")
			WriteHeaders(w, pb)
			return
		}
	} else {
		err := errors.New("empty body not allowed")
		pb = model.BuildErrorPassback(http.StatusBadRequest, err)
		logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("empty body")
		WriteHeaders(w, pb)
		return
	}

	pb = domain.TriggerTransition(parameters)

	if pb.IsError == false {
		location := "../transitions/" + (pb.Obj.(model.TransitionCreation).TransitionID.String())

		WriteHeadersWithLocation(w, pb, location)
	} else {
		WriteHeaders(w, pb)
	}
	return
}

// GetTransitions - returns all transitions
func GetTransitions(w http.ResponseWriter, req *http.Request) {
	var pb model.Passback
	params := mux.Vars(req)

	//If actionID is not in the params, then do ALL
	if _, ok := params["transitionID"]; ok {
		//parse uuid and if its good then call getUpdates
		pb = GetUUIDFromVars("transitionID", req)
		if pb.IsError {
			WriteHeaders(w, pb)
			return
		}
		transitionID := pb.Obj.(uuid.UUID)
		pb = domain.GetTransition(transitionID)

	} else {
		pb = domain.GetTransitionStatuses()
	}
	WriteHeaders(w, pb)
	return
}


// AbortTransitionID - abort transition by transitionID
func AbortTransitionID(w http.ResponseWriter, req *http.Request) {
	pb := GetUUIDFromVars("transitionID", req)
	if pb.IsError {
		WriteHeaders(w, pb)
		return
	}
	transitionID := pb.Obj.(uuid.UUID)
	pb = domain.AbortTransitionID(transitionID)
	WriteHeaders(w, pb)
	return
}


