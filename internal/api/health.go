/*
 * (C) Copyright [2022-2023] Hewlett Packard Enterprise Development LP
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
	"reflect"
	"strings"

	"github.com/Cray-HPE/hms-power-control/internal/domain"
	"github.com/Cray-HPE/hms-power-control/internal/model"
	"github.com/Cray-HPE/hms-power-control/internal/logger"
	"net/http"
)


type healthRsp struct {
	KvStore        string `json:"KvStore"`
	DistLocking    string `json:"DistLocking"`
	StateManager   string `json:"StateManager"`
	Vault          string `json:"Vault"`
	TaskRunner     string `json:"TaskRunner"`
}


// The API layer is responsible for Json Unmarshaling and Marshaling,
// creating the correct parameter types, validating the parameters by schema
// and calling the domain layer.   


// Returns the microservice liveness indicator.  Any response means we're live.
func GetLiveness(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// Readiness - Returns the microservice readiness indicator
func GetReadiness(w http.ResponseWriter, req *http.Request) {
	var err error

	fname := "GetReadiness"
	glb   := *domain.GLOB
	ready := true

	//Check KVStore and dist locks

	err = (*glb.DSP).Ping()
	if (err != nil) {
		logger.Log.Errorf("%s: Ping() failed to storage provider.",fname)
		ready = false
	}

	err = (*glb.DistLock).Ping()
	if (err != nil) {
		logger.Log.Errorf("%s: Ping() failed to dist lock provider.",fname)
		ready = false
	}

	//Check HSM

	err = (*glb.HSM).Ping()
	if (err != nil) {
		logger.Log.Errorf("%s: Ping() failed to HSM.",fname)
		ready = false
	}

	//Check Vault

	if (glb.VaultEnabled) {
		if (!(*glb.CS).IsReady()) {
			logger.Log.Errorf("%s: Cred store is not ready.",fname)
			ready = false
		}
	}

	//Check TRS

	alive,terr := (*glb.RFTloc).Alive()
    if (!alive || (terr != nil)) {
        logger.Log.Infof("%s: TRS not alive, err: %v",fname,err)
        ready = false
    }

	if (ready) {
		w.WriteHeader(http.StatusNoContent)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}

// GetHealth - Returns various health information
func GetHealth(w http.ResponseWriter, req *http.Request) {
	var err error
	var rspData healthRsp

	connected := "connected"
	unconnected := "not connected"
	responsive := "responsive"
	unresponsive := "not responsive"
	localmode := "local mode"
	remotemode := "remote mode"
	sep := ", "

	glb   := *domain.GLOB

	//Check KVStore

	if (glb.DSP == nil) {
		rspData.KvStore = unconnected
	} else {
		err = (*glb.DSP).Ping()
		if (err == nil) {
			rspData.KvStore = connected + sep + responsive
		} else {
			rspData.KvStore = connected + sep + unresponsive
		}
	}

	if (glb.DistLock == nil) {
		rspData.DistLocking = unconnected
	} else {
		err = (*glb.DistLock).Ping()
		if (err == nil) {
			rspData.DistLocking = connected + sep + responsive
		} else {
			rspData.DistLocking = connected + sep + unresponsive
		}
	}

	//Check HSM

	if (glb.HSM == nil) {
		rspData.StateManager = unconnected
	} else {
		err = (*glb.HSM).Ping()
		if (err == nil) {
			rspData.StateManager = connected + sep + responsive
		} else {
			rspData.StateManager = connected + sep + unresponsive
		}
	}

	//Check Vault

	if (glb.VaultEnabled) {
		if (glb.CS == nil) {
			rspData.Vault = unconnected
		} else {
			if ((*glb.CS).IsReady()) {
				rspData.Vault = connected + sep + responsive
			} else {
				rspData.Vault = connected + sep + unresponsive
			}
		}
	} else {
		rspData.Vault = "disabled"
	}

	//Check TRS

	if (glb.RFTloc == nil) {
		rspData.TaskRunner = unconnected
	} else {
		alive,terr := (*glb.RFTloc).Alive()
		if (terr != nil) {
			rspData.TaskRunner = unconnected + sep + unresponsive
		} else {
			if (!alive) {
				rspData.TaskRunner = connected + sep + unresponsive
			} else {
				rspData.TaskRunner = connected + sep + responsive
			}
		}

		ktype := reflect.TypeOf((*glb.RFTloc)).String()
		ktypeTL := strings.ToLower(ktype)
		if (strings.Contains(ktypeTL,"local")) {
			rspData.TaskRunner = rspData.TaskRunner + sep + localmode
		} else {
			rspData.TaskRunner = rspData.TaskRunner + sep + remotemode
		}
	}

	pb := model.Passback{StatusCode: http.StatusOK, Obj: rspData,}
	WriteHeaders(w,pb)
}

