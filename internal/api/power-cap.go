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
	"github.com/sirupsen/logrus"
)

// The API layer is responsible for Json Unmarshaling and Marshaling,
// creating the correct parameter types, validating the parameters by schema
// and calling the domain layer.   Validation in the API layer does not include 'domain level validation'.
// e.g. Check to see if the Snapshot request xnames array is valid type, not check if this xnames are in the system.
// That is the responsibility of the domain layer.

// SnapshotPowerCap - creates a power cap snapshot for an array of xnames
func SnapshotPowerCap(w http.ResponseWriter, req *http.Request) {
	var pb model.Passback
	var parameters model.PowerCapSnapshotParameter
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

		err = json.Unmarshal(body, &parameters)
		if err != nil {
			pb = model.BuildErrorPassback(http.StatusBadRequest, err)
			logger.Log.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Unparseable json")
			WriteHeaders(w, pb)
			return
		}

		if len(parameters.Xnames) == 0 {
			err := errors.New("Xname list is empty. A list of xnames is required")
			pb = model.BuildErrorPassback(http.StatusBadRequest, err)
			logrus.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Empty xname list")
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

	//Call the domain logic to do something!
	pb = domain.SnapshotPowerCap(parameters)

	if pb.IsError == false {
		location := "../power-cap/" + (pb.Obj.(model.PowerCapTaskCreation).TaskID.String())

		WriteHeadersWithLocation(w, pb, location)
	} else {
		WriteHeaders(w, pb)
	}
	return
}

// PatchPowerCap - Patch power-cap values
func PatchPowerCap(w http.ResponseWriter, req *http.Request) {
	var pb model.Passback
	var parameters model.PowerCapPatchParameter
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

	var badCtls []string
	for _, comp := range parameters.Components {
		isBad := false
		for _, ctl := range comp.Controls {
			if ctl.Name == "" || ctl.Value < 0 {
				isBad = true
			}
		}
		if isBad {
			badCtls = append(badCtls, comp.Xname)
		}
	}
	if len(badCtls) > 0 {
		errormsg := "Invalid control parameters for xnames detected "
		for _, xname := range badCtls {
			errormsg += xname + " "
		}
		err := errors.New(errormsg)
		pb = model.BuildErrorPassback(http.StatusBadRequest, err)
		logrus.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode, "xnames": badCtls}).Error("Invalid control parameters for xnames detected")
		WriteHeaders(w, pb)
		return
	}

	//Call the domain logic to do something!
	pb = domain.PatchPowerCap(parameters)

	if pb.IsError == false {
		location := "../power-cap/" + (pb.Obj.(model.PowerCapTaskCreation).TaskID.String())

		WriteHeadersWithLocation(w, pb, location)
	} else {
		WriteHeaders(w, pb)
	}
	return
}

// GetPowerCap - Get PowerCap tasks array
func GetPowerCap(w http.ResponseWriter, req *http.Request) {
	base.DrainAndCloseRequestBody(req)

	var pb model.Passback
	pb = domain.GetPowerCap()
	WriteHeaders(w, pb)
	return
}

// GetPowerCapQuery - Get PowerCap information by ID
func GetPowerCapQuery(w http.ResponseWriter, req *http.Request) {
	pb := GetUUIDFromVars("taskID", req)

	base.DrainAndCloseRequestBody(req)
	// Drain and close request body to ensure connection reuse

	if pb.IsError {
		WriteHeaders(w, pb)
		return
	}
	taskID := pb.Obj.(uuid.UUID)
	pb = domain.GetPowerCapQuery(taskID)
	WriteHeaders(w, pb)
	return
}
