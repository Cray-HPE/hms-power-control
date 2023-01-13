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

package api

import (
	"errors"
	base "github.com/Cray-HPE/hms-base"
	"github.com/Cray-HPE/hms-power-control/internal/domain"
	"github.com/Cray-HPE/hms-power-control/internal/model"
	"github.com/sirupsen/logrus"
	"net/http"
)

// The API layer is responsible for Json Unmarshaling and Marshaling,
// creating the correct parameter types, validating the parameters by schema
// and calling the domain layer.   Validation in the API layer does not include 'domain level validation'.
// e.g. Check to see if an PowerStatus filter (like xname) is valid type, not check if this xname is available in the system.
// That is the responsibility of the domain layer.

// GetPowerStatus - Returns the power status of the hardware
func GetPowerStatus(w http.ResponseWriter, req *http.Request) {
	var pb model.Passback

	/////////
	// RETRIEVE PARAMS
	/////////

	queryParams := req.URL.Query()

	//xnames really is an array
	xnamesReq := queryParams["xname"]

	//The specification only allows 1 instance of this to be passed; the .Get returns only a single instance
	powerStateFilterReq := queryParams.Get("powerStateFilter")

	//The specification only allows 1 instance of this to be passed; the .Get returns only a single instance
	managementStateFilterReq := queryParams.Get("managementStateFilter")

	///////////
	// Validate Params & Cast to Types
	///////////

	psf, err := model.ToPowerStateFilter(powerStateFilterReq)
	if err != nil {
		pb = model.BuildErrorPassback(http.StatusBadRequest, err)
		logrus.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Invalid PowerStateFilter")
		WriteHeaders(w, pb)
		return
	}

	msf, err := model.ToManagementStateFilter(managementStateFilterReq)
	if err != nil {
		pb = model.BuildErrorPassback(http.StatusBadRequest, err)
		logrus.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode}).Error("Invalid ManagementStateFilter")
		WriteHeaders(w, pb)
		return
	}
	//validates the schema of the xname, not that the xname actually exists; that requires a HSM call.
	xnames, badXnames := base.ValidateCompIDs(xnamesReq, true)
	if len(badXnames) > 0 {

		errormsg := "invalid xnames detected:"
		for _, badxname := range badXnames {
			errormsg += " " + badxname
		}
		err := errors.New(errormsg)
		pb = model.BuildErrorPassback(http.StatusBadRequest, err)
		logrus.WithFields(logrus.Fields{"ERROR": err, "HttpStatusCode": pb.StatusCode, "xnames": badXnames}).Error("Invalid xnames detected")
		WriteHeaders(w, pb)
		return
	}

	pb = domain.GetPowerStatus(xnames, psf, msf)

	WriteHeaders(w, pb)
	return
}
