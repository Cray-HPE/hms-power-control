// MIT License
//
// (C) Copyright [2022-2025] Hewlett Packard Enterprise Development LP
//
// Permission is hereby granted, free of charge, to any person obtaining a
// copy of this software and associated documentation files (the "Software"),
// to deal in the Software without restriction, including without limitation
// the rights to use, copy, modify, merge, publish, distribute, sublicense,
// and/or sell copies of the Software, and to permit persons to whom the
// Software is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included
// in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
// THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR
// OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
// ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
// OTHER DEALINGS IN THE SOFTWARE.

package hsm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	base "github.com/Cray-HPE/hms-base/v2"
	reservation "github.com/Cray-HPE/hms-smd/v2/pkg/service-reservations"
	"github.com/Cray-HPE/hms-smd/v2/pkg/sm"
	"github.com/sirupsen/logrus"
)

const (
	hsmLivenessPath                   = "/hsm/v2/service/liveness"
	hsmStateComponentsPath            = "/hsm/v2/State/Components"
	hsmStateComponentsQueryPath       = "/hsm/v2/State/Components/Query"
	hsmInventoryComponentEndpointPath = "/hsm/v2/Inventory/ComponentEndpoints"
	hsmReservationCheckPath           = "/hsm/v2/locks/service/reservations/check"
	hsmReservationPath                = "/hsm/v2/locks/service/reservations"
	hsmReservationReleasePath         = "/hsm/v2/locks/service/reservations/release"
	hsmPowerMapPath                   = "/hsm/v2/sysinfo/powermaps"

	HSM_MAX_COMPONENT_QUERY = 2000
)

//For some reason this is not defined in the HSM code base...

type CompQuery struct {
	ComponentIDs []string `json:"ComponentIDs"`
}


func (b *HSMv2) Init(globals *HSM_GLOBALS) error {
	b.HSMGlobals = HSM_GLOBALS{}
	b.HSMGlobals = *globals

	if b.HSMGlobals.Logger == nil {
		//Set up logger with defaults.
		b.HSMGlobals.Logger = logrus.New()
	}

	svcName := b.HSMGlobals.SvcName
	if svcName == "" {
		svcName = "PCS"
	}

	if b.HSMGlobals.LockEnabled {
		//Enable HSM component reservation, and set up the reservation
		//service's own logger.

		logy := logrus.New()
		logLevel := ""
		envstr := os.Getenv("SERVICE_RESERVATION_VERBOSITY")
		if envstr != "" {
			logLevel = strings.ToUpper(envstr)
		}

		switch logLevel {
		case "TRACE":
			logy.SetLevel(logrus.TraceLevel)
		case "DEBUG":
			logy.SetLevel(logrus.DebugLevel)
		case "INFO":
			logy.SetLevel(logrus.InfoLevel)
		case "WARN":
			logy.SetLevel(logrus.WarnLevel)
		case "ERROR":
			logy.SetLevel(logrus.ErrorLevel)
		case "FATAL":
			logy.SetLevel(logrus.FatalLevel)
		case "PANIC":
			logy.SetLevel(logrus.PanicLevel)
		default:
			logy.SetLevel(logrus.ErrorLevel)
		}

		Formatter := new(logrus.TextFormatter)
		Formatter.TimestampFormat = "2006-01-02T15:04:05.999999999Z07:00"
		Formatter.FullTimestamp = true
		Formatter.ForceColors = true
		logy.SetFormatter(Formatter)

		b.HSMGlobals.Reservation = &reservation.Production{}
		b.HSMGlobals.Reservation.InitInstance(b.HSMGlobals.SMUrl, "", 1, logy, svcName)
	}

	//Make sure certain things are set up

	if b.HSMGlobals.MaxComponentQuery == 0 {
		b.HSMGlobals.MaxComponentQuery = HSM_MAX_COMPONENT_QUERY
	}
	if b.HSMGlobals.SVCHttpClient == nil {
		return fmt.Errorf("ERROR: no microservice HTTP client is present.")
	}
	if b.HSMGlobals.SMUrl == "" {
		return fmt.Errorf("ERROR: no State Manager base URL is present.")
	}

	return nil
}

func (b *HSMv2) Ping() (err error) {
	finalURL := b.HSMGlobals.SMUrl + hsmLivenessPath

	req, err := http.NewRequest("GET", finalURL, nil)
	if err != nil {
		b.HSMGlobals.Logger.Error(err)
		return
	}

	reqContext, reqCtxCancel := context.WithTimeout(context.Background(), time.Second*5)

	req = req.WithContext(reqContext)

	rsp, err := b.HSMGlobals.SVCHttpClient.Do(req)

	// Always drain response bodies and close even if not looking at body
	base.DrainAndCloseResponseBody(rsp)

	reqCtxCancel() // Release resources and signal context timeout to stop

	if err != nil {
		b.HSMGlobals.Logger.Error(err)
		return
	}

	return
}

// Check each component in the given list. If it already has a reservation key,
// try to reacquire the reservation so the auto-renew thread will have it.
// If it has a deputy key, make sure it's valid. Any component without keys or
// with invalid keys, acquire a reservation for that component. The return array
// contains only components for which we aquired a reservation, as a convenience
// to the caller so they don't have to create this list themselves.
func (b *HSMv2) ReserveComponents(compList []ReservationData) ([]*ReservationData, error) {
	var retData []*ReservationData
	var aquireList []string
	var depKeys []reservation.Key
	var resKeys []reservation.Reservation

	if !b.HSMGlobals.LockEnabled {
		return retData, nil
	}
	if len(compList) == 0 {
		return retData, nil
	}

	// First check any component record with deputy keys. Any that are
	// valid, skip. If invalid or no deputy key, add to the list to 
	// reserve.

	compMap := make(map[string]*ReservationData)

	for ix, comp := range compList {
		compMap[comp.XName] = &compList[ix]
		compMap[comp.XName].needRsv = true
		compMap[comp.XName].ReservationOwner = false
		compMap[comp.XName].Error = nil
		if comp.ReservationKey != "" {
			res := reservation.Reservation{
				Xname: comp.XName,
				ReservationKey: comp.ReservationKey,
				DeputyKey: comp.DeputyKey,
			}
			resKeys = append(resKeys, res)
		} else if comp.DeputyKey != "" {
			depKeys = append(depKeys, reservation.Key{ID: comp.XName, Key: comp.DeputyKey})
		}
	}

	// Reacquire any reservations we may have previously held
	if len(resKeys) > 0 {
		resList, resErr := b.HSMGlobals.Reservation.Reacquire(resKeys, true)
		if resErr != nil {
			return retData, fmt.Errorf("ERROR reacquiring reservations: %v", resErr)
		}

		if len(resList.Success.ComponentIDs) > 0 {
			chkResp, chkErr := b.HSMGlobals.Reservation.FlexCheck(resList.Success.ComponentIDs)
			if resErr != nil {
				return retData, fmt.Errorf("ERROR checking reacquired reservations: %v", chkErr)
			}

			for _, comp := range chkResp.Success {
				compMap[comp.ID].ReservationOwner = true
				compMap[comp.ID].needRsv = false
				compMap[comp.ID].ReservationKey = comp.ReservationKey
				compMap[comp.ID].DeputyKey = comp.DeputyKey
				compMap[comp.ID].ExpirationTime = comp.ExpirationTime
				retData = append(retData, compMap[comp.ID])
			}
		}
	}

	if len(depKeys) > 0 {
		depList, depErr := b.HSMGlobals.Reservation.ValidateDeputyKeys(depKeys)
		if depErr != nil {
			return retData, fmt.Errorf("ERROR validating deputy keys: %v", depErr)
		}

		// Mark all components with valid deputy keys as NOT the 
		// reservation owner
		for _, comp := range depList.Success {
			compMap[comp.ID].ReservationOwner = false
			compMap[comp.ID].ReservationKey = ""
			compMap[comp.ID].ExpirationTime = ""
			compMap[comp.ID].needRsv = false
			retData = append(retData, compMap[comp.ID])
		}
	}

	// Whatever comps are left over, we need to aquire their reservations
	for k, v := range compMap {
		if v.needRsv {
			aquireList = append(aquireList, k)
		}
	}

	if len(aquireList) == 0 {
		return retData, nil
	}

	acList, acErr := b.HSMGlobals.Reservation.FlexAquire(aquireList)
	if acErr != nil {
		return retData, fmt.Errorf("ERROR aquiring needed reservations: %v", acErr)
	}

	for _, rr := range acList.Success {
		compMap[rr.ID].ReservationOwner = true
		compMap[rr.ID].needRsv = false
		compMap[rr.ID].ReservationKey = rr.ReservationKey
		compMap[rr.ID].DeputyKey = rr.DeputyKey
		compMap[rr.ID].ExpirationTime = rr.ExpirationTime
		retData = append(retData, compMap[rr.ID])
	}

	return retData, nil
}

// Check each component's deputy key for validity.  Any error results in
// all components being considered invalid (this would be for some sort of
// SM commmunication error).  Otherwise, each item in the list is checked
// and if it is not valid, it's Error field is set.
//
// NOTE: not having a deputy key associated with a component is not an error.
// This func is only checking non-nil keys for validity.
func (b *HSMv2) CheckDeputyKeys(compList []ReservationData) error {
	var keyList []reservation.Key
	cmap := make(map[string]*ReservationData)

	for ix, comp := range compList {
		cmap[comp.XName] = &compList[ix]
		compList[ix].Error = nil
		if comp.DeputyKey != "" {
			keyList = append(keyList, reservation.Key{ID: comp.XName, Key: comp.DeputyKey})
		}
	}

	if len(keyList) == 0 {
		return nil
	}

	checkList, cerr := b.HSMGlobals.Reservation.ValidateDeputyKeys(keyList)
	if cerr != nil {
		return fmt.Errorf("Error in deputy key check: %v", cerr)
	}

	for _, comp := range checkList.Success {
		cmap[comp.ID].ExpirationTime =  comp.ExpirationTime
		cmap[comp.ID].Error = nil
	}

	for _, comp := range checkList.Failure {
		cmap[comp.ID].Error = errors.New(comp.Reason)
	}

	return nil
}

//The passed-in list should only contain components that don't have a
//deputy key.  If no error is returned, caller must check all components
//to see if any of them failed.  The returned array contains components
//for who the release failed, for caller's convenience
func (b *HSMv2) ReleaseComponents(compList []ReservationData) ([]*ReservationData, error) {
	var retData []*ReservationData

	if !b.HSMGlobals.LockEnabled {
		return retData,nil
	}

	var clearList []string
	compMap := make(map[string]*ReservationData)

	for ix,comp := range(compList) {
		compMap[comp.XName] = &compList[ix]
		if comp.ReservationOwner {
			clearList = append(clearList, comp.XName)
		}
	}

	if len(clearList) == 0 {
		return retData, nil
	}

	rsv, rsvErr := b.HSMGlobals.Reservation.FlexRelease(clearList)
	if rsvErr != nil {
		return retData, fmt.Errorf("ERROR releasing reservations: %v", rsvErr)
	}

	for _, rr := range rsv.Success.ComponentIDs {
		compMap[rr].ReservationOwner = false
		compMap[rr].ReservationKey = ""
		compMap[rr].ExpirationTime = ""
		compMap[rr].DeputyKey = ""
		compMap[rr].Error = nil
	}
	for _, rr := range rsv.Failure {
		compMap[rr.ID].Error = errors.New(rr.Reason)
		retData = append(retData, compMap[rr.ID])
	}

	return retData, nil
}

// Given a map of previously-created HSM component data, populate
// endpoint information in each one.
//
// TODO: We currently don't return any components NOT found in HSM.
// Should we do so, and populate minimal data + the Error field?
func (b *HSMv2) FillComponentEndpointData(hd map[string]*HsmData) error {
	xnames := []string{}
	for _, comp := range(hd) {
		xnames = append(xnames, comp.BaseData.ID)
	}

	compIX := 0
	for numComps := len(hd); numComps > 0; numComps = numComps - b.HSMGlobals.MaxComponentQuery {
		idCount := numComps
		if idCount > b.HSMGlobals.MaxComponentQuery {
			idCount = b.HSMGlobals.MaxComponentQuery
		}

		//Construct the URL
		urlSuffix := ""
		for ix := 0; ix < idCount; ix ++ {
			urlSuffix = urlSuffix + "&id=" + xnames[compIX]
			compIX ++
		}
		smurl := b.HSMGlobals.SMUrl + hsmInventoryComponentEndpointPath +
					"?" + strings.TrimLeft(urlSuffix,"&")

		//Make the HSM call
		req, err := http.NewRequest(http.MethodGet, smurl, nil)
		if err != nil {
			return fmt.Errorf("ERROR creating HTTP request for '%s': %v",
				smurl, err)
		}

		reqContext, reqCtxCancel := context.WithTimeout(context.Background(), 40 * time.Second)

		req = req.WithContext(reqContext)

		rsp, rsperr := b.HSMGlobals.SVCHttpClient.Do(req)
		if rsperr != nil {
			// Always drain and close response bodies
			base.DrainAndCloseResponseBody(rsp)

			reqCtxCancel() // Release resources and signal context timeout to stop

			return fmt.Errorf("Error in http request '%s': %v", smurl, rsperr)
		}

		body, bderr := io.ReadAll(rsp.Body)

		// Always close response bodies
		base.DrainAndCloseResponseBody(rsp)

		reqCtxCancel() // Release resources and signal context timeout to stop

		if bderr != nil {
			return fmt.Errorf("Error reading response body for '%s': %v",
				smurl, bderr)
		}

		var jdata sm.ComponentEndpointArray
		bderr = json.Unmarshal(body, &jdata)
		if bderr != nil {
			return fmt.Errorf("Error unmarshalling response body for '%s': %v",
				smurl, bderr)
		}

		//Update the data from HSM into the HSM data map
		for _, comp := range jdata.ComponentEndpoints {
			_, ok := hd[comp.ID]
			if !ok {
				b.HSMGlobals.Logger.Warnf("HSM inventory/component item '%s' not found in internal map! Ignoring.",
					comp.ID)
				continue
			}

			//NOTE: the PowerURL field is only populated for components that
			//a BMC can serve power capping info for.  Thus, it's only for
			//nodes, really.  BMCs, etc. won't have this.
			switch (comp.ComponentEndpointType) {
				case sm.CompEPTypeChassis:	//chassis
					hd[comp.ID].RfFQDN = comp.RfEndpointFQDN
					hd[comp.ID].PowerStatusURI = comp.OdataID
					if comp.RedfishChassisInfo != nil {
						if comp.RedfishChassisInfo.Actions != nil {
							hd[comp.ID].PowerActionURI = comp.RedfishChassisInfo.Actions.ChassisReset.Target
							hd[comp.ID].AllowableActions = comp.RedfishChassisInfo.Actions.ChassisReset.AllowableValues
						}
					}

				case sm.CompEPTypeSystem:	//node
					hd[comp.ID].RfFQDN = comp.RfEndpointFQDN
					hd[comp.ID].PowerStatusURI = comp.OdataID
					if comp.RedfishSystemInfo != nil {
						if comp.RedfishSystemInfo.Actions != nil {
							hd[comp.ID].PowerActionURI = comp.RedfishSystemInfo.Actions.ComputerSystemReset.Target
							hd[comp.ID].AllowableActions = comp.RedfishSystemInfo.Actions.ComputerSystemReset.AllowableValues
						}
						hd[comp.ID] = extractPowerCapInfo(hd[comp.ID], comp)
					}

				case sm.CompEPTypeManager:	//BMC
					hd[comp.ID].RfFQDN = comp.RfEndpointFQDN
					hd[comp.ID].PowerStatusURI = comp.OdataID
					if comp.RedfishManagerInfo != nil &&
					   comp.RedfishManagerInfo.Actions != nil {
						hd[comp.ID].PowerActionURI = comp.RedfishManagerInfo.Actions.ManagerReset.Target
						hd[comp.ID].AllowableActions = comp.RedfishManagerInfo.Actions.ManagerReset.AllowableValues
					}

				case sm.CompEPTypePDU:		//PDU outlet collection
					hd[comp.ID].RfFQDN = comp.RfEndpointFQDN
					hd[comp.ID].PowerStatusURI = comp.OdataID

				case sm.CompEPTypeOutlet:	//PDU outlet/power connector
					hd[comp.ID].RfFQDN = comp.RfEndpointFQDN
					hd[comp.ID].PowerStatusURI = comp.OdataID
					if comp.RedfishOutletInfo != nil &&
					   comp.RedfishOutletInfo.Actions != nil {
						hd[comp.ID].PowerActionURI = comp.RedfishOutletInfo.Actions.PowerControl.Target
						hd[comp.ID].AllowableActions = comp.RedfishOutletInfo.Actions.PowerControl.AllowableValues
					}

				default:
					b.HSMGlobals.Logger.Errorf("Unknown HSM Inventory/ComponentEndpoint type '%s'.",
						comp.ComponentEndpointType)
			}
		}
	}

	return nil
}

// Turn the PowerCtl and Controls structs from HSM into something useful for
// PCS. This function constructs a map of PowerCap structs from PowerCtl[] or
// Controls[] in RedfishSystemInfo from HSM. This function manipulates the
// given compData and sets the following fields:
// - compData.PowerCapURI
// - compData.PowerCapControlsCount
// - compData.PowerCapCtlInfoCount
// - compData.PowerCapTargetURI - if rfSysInfo.PowerCtlInfo.PowerCtl[0].OEM.HPE.Target exists
// - compData.PowerCaps
func extractPowerCapInfo(compData *HsmData, compEP *sm.ComponentEndpoint) *HsmData {
	if compData == nil || compEP == nil || compEP.RedfishSystemInfo == nil {
		return compData
	}
	rfSysInfo := compEP.RedfishSystemInfo
	compData.PowerCapURI = rfSysInfo.PowerURL
	compData.PowerCapControlsCount = len(rfSysInfo.Controls)
	compData.PowerCapCtlInfoCount = len(rfSysInfo.PowerCtlInfo.PowerCtl)
	if compData.PowerCapControlsCount > 0 {
		compData.PowerCaps = make(map[string]PowerCap)
		for i, ctl := range rfSysInfo.Controls {
			compData.PowerCaps[ctl.Control.Name] = PowerCap{
				Name:        ctl.Control.Name,
				Path:        ctl.URL,
				Min:         ctl.Control.SettingRangeMin,
				Max:         ctl.Control.SettingRangeMax,
				PwrCtlIndex: i,
			}
		}
	} else if compData.PowerCapCtlInfoCount > 0 {
		pwrCtl := rfSysInfo.PowerCtlInfo.PowerCtl[0]
		if pwrCtl.OEM != nil && pwrCtl.OEM.HPE != nil && len(pwrCtl.OEM.HPE.Target) > 0 {
			compData.PowerCapTargetURI = pwrCtl.OEM.HPE.Target
		}
		compData.PowerCaps = make(map[string]PowerCap)
		isHpeApollo6500 := strings.Contains(compData.PowerCapURI, "AccPowerService/PowerLimit")
		isHpeServer := strings.Contains(compData.PowerCapURI, "Chassis/1/Power")
		for i, ctl := range rfSysInfo.PowerCtlInfo.PowerCtl {
			var min int = -1
			var max int = -1
			if isHpeApollo6500 || isHpeServer {
				ctl.Name = "Node Power Limit"
			}
			if ctl.OEM != nil {
				if ctl.OEM.Cray != nil {
					min = ctl.OEM.Cray.PowerLimit.Min
					max = ctl.OEM.Cray.PowerLimit.Max
				} else if ctl.OEM.HPE != nil {
					min = ctl.OEM.HPE.PowerLimit.Min
					max = ctl.OEM.HPE.PowerLimit.Max
				}
			}
			// remove any #fragment in the path
			var path string
			c := strings.Index(ctl.Oid, "#")
			if c < 0 {
				path = ctl.Oid
			} else {
				path = ctl.Oid[:c]
			}

			if path == "" {
				// A sane default fallback.
				path = rfSysInfo.PowerURL
			}
			compData.PowerCaps[ctl.Name] = PowerCap{
				Name:        ctl.Name,
				Path:        path,
				Min:         min,
				Max:         max,
				PwrCtlIndex: i,
			}
		}
	}
	return compData
}

// Fetch component info from HSM.
func (b *HSMv2) GetStateComponents(xnames []string) (base.ComponentArray, error) {
	var queryData CompQuery
	var retData base.ComponentArray

	smurl := b.HSMGlobals.SMUrl + hsmStateComponentsQueryPath
	queryData.ComponentIDs = xnames
	ba, baerr := json.Marshal(&queryData)
	if baerr != nil {
		return retData, fmt.Errorf("Error marshalling HSM component query data: %v",
			baerr)
	}

	req, err := http.NewRequest(http.MethodPost, smurl, bytes.NewBuffer(ba))
	if err != nil {
		return retData, fmt.Errorf("ERROR creating HTTP request for '%s': %v",
			smurl, err)
	}

	reqContext, reqCtxCancel := context.WithTimeout(context.Background(), 40 * time.Second)

	req = req.WithContext(reqContext)

	rsp, rsperr := b.HSMGlobals.SVCHttpClient.Do(req)
	if rsperr != nil {
		// Always drain and close response bodies
		base.DrainAndCloseResponseBody(rsp)

		reqCtxCancel() // Release resources and signal context timeout to stop

		return retData, fmt.Errorf("Error in http request '%s': %v", smurl, rsperr)
	}

	body, bderr := io.ReadAll(rsp.Body)

	// Always close response bodies
	base.DrainAndCloseResponseBody(rsp)

	reqCtxCancel() // Release resources and signal context timeout to stop

	if bderr != nil {
		return retData, fmt.Errorf("Error reading response body for '%s': %v",
			smurl, bderr)
	}

	bderr = json.Unmarshal(body, &retData)
	if bderr != nil {
		return retData, fmt.Errorf("Error unmarshalling response body for '%s': %v",
			smurl, bderr)
	}

	return retData, nil
}

// Fetch power map from HSM.
func (b *HSMv2) FillPowerMapData(hd map[string]*HsmData) error {
	var retData []sm.PowerMap

	smurl := b.HSMGlobals.SMUrl + hsmPowerMapPath

	req, err := http.NewRequest(http.MethodGet, smurl, nil)
	if err != nil {
		return fmt.Errorf("ERROR creating HTTP request for '%s': %v", smurl, err)
	}

	reqContext, reqCtxCancel := context.WithTimeout(context.Background(), 40 * time.Second)

	req = req.WithContext(reqContext)

	rsp, rsperr := b.HSMGlobals.SVCHttpClient.Do(req)
	if rsperr != nil {
		// Always drain and close response bodies
		base.DrainAndCloseResponseBody(rsp)

		reqCtxCancel() // Release resources and signal context timeout to stop

		return fmt.Errorf("Error in http request '%s': %v", smurl, rsperr)
	}

	body, bderr := io.ReadAll(rsp.Body)

	// Always close response bodies
	base.DrainAndCloseResponseBody(rsp)

	reqCtxCancel() // Release resources and signal context timeout to stop

	if bderr != nil {
		return fmt.Errorf("Error reading response body for '%s': %v", smurl, bderr)
	}

	bderr = json.Unmarshal(body, &retData)
	if bderr != nil {
		return fmt.Errorf("Error unmarshalling response body for '%s': %v", smurl, bderr)
	}

	for _, pm := range retData {
		_, ok := hd[pm.ID]
		if ok {
			hd[pm.ID].PoweredBy = pm.PoweredBy
		}
	}

	return nil
}

// Fill in various data for each requested component from HSM.
func (b *HSMv2) FillHSMData(xnames []string) (map[string]*HsmData, error) {
	hdata := make(map[string]*HsmData)

	//Get state/component data to fill in ID/Type/Role
	compArray, caerr := b.GetStateComponents(xnames)
	if caerr != nil {
		return hdata, fmt.Errorf("ERROR fetching State/Component data from HSM: %v",
			caerr)
	}

	//Populate map with what we have so far
	for _, comp := range compArray.Components {
		hdata[(*comp).ID] = &HsmData{BaseData: (*comp)}
	}

	//Get Inventory/ComponentEndpoint data to fill in Hostname/Domain/FQDN/URIs
	inverr := b.FillComponentEndpointData(hdata)
	if inverr != nil {
		return hdata, fmt.Errorf("ERROR fetching Inventory/ComponentEndpoints data from HSM: %v",
			inverr)
	}

	return hdata, nil
}
