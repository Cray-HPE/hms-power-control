/*
 * (C) Copyright [2021-2025] Hewlett Packard Enterprise Development LP
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
	"io"
	"net/http"

	base "github.com/Cray-HPE/hms-base/v2"
	"github.com/Cray-HPE/hms-power-control/internal/domain"
	"github.com/Cray-HPE/hms-power-control/internal/logger"
	"github.com/Cray-HPE/hms-power-control/internal/model"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// The API layer is responsible for Json Unmarshaling and Marshaling,
// creating the correct parameter types, validating the parameters by schema
// and calling the domain layer.   Validation in the API layer does not include 'domain level validation'.
// e.g. Check to see if an Operation is valid type, not check if this transition can be completed.
// That is the responsibility of the domain layer.

// CreateTransition - creates a transition and will trigger a 'transition' flow
func CreateTransition(w http.ResponseWriter, req *http.Request) {
	var pb model.Passback
	var parameters model.TransitionParameter
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)

		base.DrainAndCloseRequestBody(req)

		logger.Log.WithFields(logrus.Fields{"body": string(body)}).Trace("Printing request body")

		if err != nil {
			pb := model.BuildErrorPassback(http.StatusInternalServerError, err)
			logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Error detected retrieving body")
			WriteHeaders(w, pb)
			return
		}

		//This will validate that the uuid is valid
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

	//Validate the transition (specifically the Operation type)
	transition, err := model.ToTransition(parameters, domain.GLOB.ExpireTimeMins)
	if err != nil {
		pb = model.BuildErrorPassback(http.StatusBadRequest, err)
		logrus.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Invalid operation")
		WriteHeaders(w, pb)
		return
	}

	//Call the domain logic to do something!
	pb = domain.TriggerTransition(transition)

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

	defer base.DrainAndCloseRequestBody(req)

	//If actionID is not in the params, then do ALL
	if _, ok := params["transitionID"]; ok {
		//parse uuid and if its good then call GetTransition
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

	base.DrainAndCloseRequestBody(req)

	if pb.IsError {
		WriteHeaders(w, pb)
		return
	}
	transitionID := pb.Obj.(uuid.UUID)
	pb = domain.AbortTransitionID(transitionID)
	WriteHeaders(w, pb)
	return
}
