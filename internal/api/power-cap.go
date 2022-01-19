package api

import (
	"encoding/json"
	"errors"
	base "github.com/Cray-HPE/hms-base"
	"github.com/Cray-HPE/hms-power-control/internal/domain"
	"github.com/Cray-HPE/hms-power-control/internal/logger"
	"github.com/Cray-HPE/hms-power-control/internal/model"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
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

	//validates the schema of the xname, not that the xname actually exists; that requires a HSM call.
	_, badXnames := base.ValidateCompIDs(parameters.Xnames, true)
	if len(badXnames) > 0 {

		errormsg := "invalid xnames detected "
		for _, badxname := range badXnames {
			errormsg += badxname + " "
		}
		err := errors.New(errormsg)
		pb = model.BuildErrorPassback(http.StatusBadRequest, err)
		logrus.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode, "xnames": badXnames}).Error("Invalid xnames detected")
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

	//TODO something needs to validate the ranges of values (can they be negative? doubles? is there a max?).
	// Its key/value, so it might be quite permissive across our many hw types
	
	//validates the schema of the xname, not that the xname actually exists; that requires a HSM call.
	var xnamesReq []string
	for _, component := range parameters.Components {
		xnamesReq = append(xnamesReq, component.Xname)
	}
	_, badXnames := base.ValidateCompIDs(xnamesReq, true)
	if len(badXnames) > 0 {

		errormsg := "invalid xnames detected "
		for _, badxname := range badXnames {
			errormsg += badxname + " "
		}
		err := errors.New(errormsg)
		pb = model.BuildErrorPassback(http.StatusBadRequest, err)
		logrus.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode, "xnames": badXnames}).Error("Invalid xnames detected")
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
	var pb model.Passback
	pb = domain.GetPowerCap()
	WriteHeaders(w, pb)
	return
}

// GetPowerCapQuery - Get PowerCap information by ID
func GetPowerCapQuery(w http.ResponseWriter, req *http.Request) {
	pb := GetUUIDFromVars("taskID", req)
	if pb.IsError {
		WriteHeaders(w, pb)
		return
	}
	taskID := pb.Obj.(uuid.UUID)
	pb = domain.GetPowerCapQuery(taskID)
	WriteHeaders(w, pb)
	return
}
