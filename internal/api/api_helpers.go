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
	"encoding/json"
	"github.com/Cray-HPE/hms-power-control/internal/logger"
	"github.com/Cray-HPE/hms-power-control/internal/model"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	logrus "github.com/sirupsen/logrus"
	"net/http"
)

// WriteJSON - writes JSON to the open http connection
func WriteJSON(w http.ResponseWriter, i interface{}) {
	obj, err := json.Marshal(i)
	if err != nil {
		logger.Log.Error(err)
	}
	_, err = w.Write(obj)
	if err != nil {
		logger.Log.Error(err)
	}
}

// WriteHeaders - writes JSON to the open http connection along with headers
func WriteHeaders(w http.ResponseWriter, pb model.Passback) {
	if pb.IsError {
		w.Header().Add("Content-Type", "application/problem+json")
		w.WriteHeader(pb.StatusCode)
		WriteJSON(w, pb.Error)
	} else if pb.Obj != nil {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(pb.StatusCode)
		switch val := pb.Obj.(type) {
		case []uuid.UUID:
			WriteJSON(w, model.IDList{val})
		case uuid.UUID:
			WriteJSON(w, model.IDResp{val})
		default:
			WriteJSON(w, pb.Obj)
		}
	} else {
		w.WriteHeader(pb.StatusCode)
	}
}

func WriteHeadersWithLocation(w http.ResponseWriter, pb model.Passback, location string) {
	w.Header().Add("Location", location)
	WriteHeaders(w, pb)
}

// GetUUIDFromVars - attempts to retrieve a UUID from an http.Request URL
// returns a passback
func GetUUIDFromVars(key string, r *http.Request) (passback model.Passback) {
	vars := mux.Vars(r)
	value := vars[key]
	logger.Log.WithFields(logrus.Fields{"key": value}).Debug("Attempting to parse UUID")

	UUID, err := uuid.Parse(value)

	if err != nil {
		passback = model.BuildErrorPassback(http.StatusBadRequest, err)
		logger.Log.WithFields(logrus.Fields{"ERROR": err}).Error("Could not parse UUID: " + key)
		return passback
	}
	passback = model.BuildSuccessPassback(http.StatusOK, UUID)
	return passback
}
