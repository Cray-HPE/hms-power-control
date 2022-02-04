// MIT License
// 
// (C) Copyright [2022] Hewlett Packard Enterprise Development LP
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
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
	"github.com/sirupsen/logrus"
	"github.com/Cray-HPE/hms-base"
	"github.com/Cray-HPE/hms-smd/pkg/sm"
	reservation "github.com/Cray-HPE/hms-smd/pkg/service-reservations"
)

const (
	hsmLivenessPath = "/hsm/v2/service/liveness"
	hsmStateComponentsPath = "/hsm/v2/State/Components"
	hsmStateComponentsQueryPath = "/hsm/v2/State/Components/Query"
	hsmInventoryComponentEndpointPath = "/hsm/v2/Inventory/ComponentEndpoints"
	hsmReservationCheckPath = "/hsm/v2/locks/service/reservations/check"
	hsmReservationPath = "/hsm/v2/locks/service/reservations"
	hsmReservationReleasePath = "/hsm/v2/locks/service/reservations/release"
)

//For some reason this is not defined in the HSM code base...

type CompQuery struct {
	ComponentIDs []string `json:"ComponentIDs"`
}


func (b *HSMv0) Init(globals *HSM_GLOBALS) error {
	b.HSMGlobals = HSM_GLOBALS{}
	b.HSMGlobals = *globals

	if (b.HSMGlobals.Logger == nil) {
		//Set up logger with defaults.
		b.HSMGlobals.Logger = logrus.New()
	}

	svcName := b.HSMGlobals.SvcName
	if (svcName == "") {
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

//ZZZZ b.HSMGlobals.Logger.Errorf("HSMInit() logy: %p",logy)
		b.HSMGlobals.Reservation = &reservation.Production{}
		b.HSMGlobals.Reservation.InitInstance(b.HSMGlobals.SMUrl, "", 1, logy,svcName)
//ZZZZ b.HSMGlobals.Logger.Errorf("HSMInit() RSV: %p, '%v'",&b.HSMGlobals.Reservation,b.HSMGlobals.Reservation)
	}

	//Make sure certain things are set up

	if (b.HSMGlobals.SVCHttpClient == nil) {
		return fmt.Errorf("ERROR: no microservice HTTP client is present.")
	}
	if (b.HSMGlobals.SMUrl == "") {
		return fmt.Errorf("ERROR: no State Manager base URL is present.")
	}
//ZZZZ don't need this.
	if (b.HSMGlobals.MaxComponentQuery == 0) {
		b.HSMGlobals.MaxComponentQuery = 500
		b.HSMGlobals.Logger.Infof("Max component query length set to %d xnames.",
			b.HSMGlobals.MaxComponentQuery)
	}

	return nil
}

func (b *HSMv0) Ping() (err error) {
	finalURL := b.HSMGlobals.SMUrl + hsmLivenessPath

	req, err := http.NewRequest("GET", finalURL, nil)
	if err != nil {
		b.HSMGlobals.Logger.Error(err)
		return
	}

/**/
	tokstr := os.Getenv("TOKEN")
	if (tokstr != "") {
		req.Header.Add("Authorization",fmt.Sprintf("Bearer %s",tokstr))
		req.Header.Add("Content-Type","application/json")
	}
/**/

	reqContext, _ := context.WithTimeout(context.Background(), time.Second*5)
	req = req.WithContext(reqContext)
	if err != nil {
		b.HSMGlobals.Logger.Error(err)
		return
	}

	_, err = b.HSMGlobals.SVCHttpClient.Do(req)
	if err != nil {
		b.HSMGlobals.Logger.Error(err)
		return
	}
	return
}

func doReservationAquire(b *HSMv0, compList []ReservationData) error {
	if (!b.HSMGlobals.LockEnabled) {
		return nil
	}

	var aquireList []string
	for _,comp := range(compList) {
		if (b.HSMGlobals.Reservation.Check([]string{comp.XName}) == false) {
			aquireList = append(aquireList,comp.XName)
		}
	}

	err := b.HSMGlobals.Reservation.Aquire(aquireList)
	return err
}

//Take a list of components and (maybe) deputy keys.  For each component
//that has a deputy key, it is considered locked/reserved.  If there is no
//deputy key, then it will be locked/reserved here.

func (b *HSMv0) ReserveComponents(compList []ReservationData) ([]ReservationData,error) {
	rmap := make(map[string]*ReservationData)
	dmap := make(map[string]*ReservationData)

	if !b.HSMGlobals.LockEnabled {
		return []ReservationData{},nil
	}

	//Separate the components with deputy keys from the ones without.

	depList := []ReservationData{}
	freeList := []ReservationData{}

	for ix,comp := range compList {
		if ((comp.DeputyKey == "") || (comp.ReservationKey != "")) {
			freeList = append(freeList,comp)
			rmap[comp.XName] = &compList[ix]
b.HSMGlobals.Logger.Errorf(">>>>RSVCmp: freeList added '%s'",comp.XName)
		} else {
			depList = append(depList,comp)
			dmap[comp.XName] = &compList[ix]
b.HSMGlobals.Logger.Errorf(">>>>RSVCmp: depList added '%s'",comp.XName)
		}
	}

	//First check the components with deputy keys.  Those are considered
	//already locked/reserved.

	err := b.CheckDeputyKeys(depList)
	if (err != nil) {
		return []ReservationData{},fmt.Errorf("Error checking deputy keys: %v",err)
	}

	//If any deputy keys are invalid, fail the whole operation.

	ok := true
	for _,comp := range depList {
		if (comp.Error != "") {
			ok = false
			b.HSMGlobals.Logger.Errorf("Deputy key for '%s' check failed: %s.",
				comp.XName,comp.Error)
		}
	}
	if (!ok) {
		return []ReservationData{},fmt.Errorf("Deputy key(s) were invalid.")
	}

	//Acquire the reservations for the items that don't have deputy keys.

	err = doReservationAquire(b,freeList)
	if (err != nil) {
		return freeList,fmt.Errorf("Error aquiring reservations: %v",err)
	}

	ok = true
	for _,comp := range freeList {
		if (comp.Error != "") {
			ok = false
			b.HSMGlobals.Logger.Errorf("Deputy key for '%s' check failed: %s.",
				comp.XName,comp.Error)
			rmap[comp.XName].Error = comp.Error
		}
		rmap[comp.XName].ReservationKey = comp.ReservationKey
		rmap[comp.XName].DeputyKey = comp.DeputyKey
		rmap[comp.XName].ExpirationTime = comp.ExpirationTime
	}

	//We are returning the list of components for which we got a reservation
	//as a convenience to the caller so they don't have to plow through the
	//entire component list to find the ones they need.

	return freeList,nil
}

// Check each component's deputy key for validity.  Any error results in
// all components being considered invalid (this would be for some sort of
// SM commmunication error).  Otherwise, each item in the list is checked
// and if it is not valid, it's Error field is set.
//
// NOTE: not having a deputy key associated with a component is not an error.
// This func is only checking non-nil keys for validity.

func (b *HSMv0) CheckDeputyKeys(compList []ReservationData) error {
	var keyList []reservation.Key
	cmap := make(map[string]*ReservationData)

	for ix,comp := range(compList) {
		cmap[comp.XName] = &compList[ix]
		keyList = append(keyList,reservation.Key{ID: comp.XName, Key: comp.DeputyKey})
	}

	checkList,cerr := b.HSMGlobals.Reservation.ValidateDeputyKeys(keyList)
	if (cerr != nil) {
		return fmt.Errorf("Error in deputy key check: %v", cerr)
	}

	for _,comp := range(checkList.Success) {
		cmap[comp.ID].ExpirationTime =  comp.ExpirationTime
		cmap[comp.ID].Error = ""
	}

	for _,comp := range(checkList.Failure) {
		cmap[comp.ID].ExpirationTime = ""
		cmap[comp.ID].Error = comp.Reason
	}

	return nil
}


//The passed-in list should only contain components that don't have a
//deputy key.  If no error is returned, caller must check all components
//to see if any of them failed.

func (b *HSMv0) ReleaseComponents(compList []ReservationData) error {
	if !b.HSMGlobals.LockEnabled {
		return nil
	}

	var clearList []string
	for _,comp := range(compList) {
		if (b.HSMGlobals.Reservation.Check([]string{comp.XName}) == true) {
			clearList = append(clearList,comp.XName)
		}
	}

	if (len(clearList) == 0) {
		return nil
	}

	err := b.HSMGlobals.Reservation.Release(clearList)
	if (err != nil) {
		return err
	}

	for ix := 0; ix < len(compList); ix ++ {
		compList[ix].ReservationKey = ""
		compList[ix].DeputyKey = ""
		compList[ix].ExpirationTime = ""
		compList[ix].Error = ""
	}

	return nil
}

func (b *HSMv0) FillComponentEndpointData(hd map[string]*HsmData) error {
	//Slice this up into max-xname-chunk pieces and do multiple calls to HSM.
	//Speed shouldn't be an issue since no large power operation should ever
	//be super-huge.  If this gets to be a problem, we can use TRS to 
	//do the calls in parallel.

	xnames := []string{}
	for _,comp := range(hd) {
		xnames = append(xnames,comp.BaseData.ID)
	}

	compIX := 0
	for numComps := len(hd); numComps > 0; numComps = numComps - b.HSMGlobals.MaxComponentQuery {
		idCount := numComps
		if (idCount > b.HSMGlobals.MaxComponentQuery) {
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

		req,err := http.NewRequest(http.MethodGet,smurl,nil)
		if (err != nil) {
			return fmt.Errorf("ERROR creating HTTP request for '%s': %v",
				smurl,err)
		}
/**/
	tokstr := os.Getenv("TOKEN")
	if (tokstr != "") {
		req.Header.Add("Authorization",fmt.Sprintf("Bearer %s",tokstr))
		req.Header.Add("Content-Type","application/json")
	}
/**/

		reqContext,_ := context.WithTimeout(context.Background(), 40 * time.Second)
		req = req.WithContext(reqContext)

		rsp,rsperr := b.HSMGlobals.SVCHttpClient.Do(req)
		if (rsperr != nil) {
			return fmt.Errorf("Error in http request '%s': %v",smurl,rsperr)
		}

		body,bderr := ioutil.ReadAll(rsp.Body)
		if (bderr != nil) {
			return fmt.Errorf("Error reading response body for '%s': %v",
				smurl,bderr)
		}

		var jdata sm.ComponentEndpointArray
		bderr = json.Unmarshal(body,&jdata)
		if (bderr != nil) {
			return fmt.Errorf("Error unmarshalling response body for '%s': %v",
				smurl,bderr)
		}

		//Update the data from HSM into the HSM data map

		for _,comp := range(jdata.ComponentEndpoints) {
			_,ok := hd[comp.ID]
			if (!ok) {
				b.HSMGlobals.Logger.Warnf("HSM inventory/component item '%s' not found in internal map!  Ignoring.",
					comp.ID)
				continue
			}

			//NOTE: the PowerURL field is only populated for components that
			//a BMC can serve power capping info for.  Thus, it's only for
			//nodes, really.  BMCs, etc. won't have this.

			switch (comp.ComponentEndpointType) {
				case sm.CompEPTypeChassis:	//chassis
					hd[comp.ID].RfFQDN = comp.RfEndpointFQDN
					hd[comp.ID].PowerActionURI = comp.RedfishChassisInfo.Actions.ChassisReset.Target
					hd[comp.ID].PowerStatusURI = comp.OdataID
					hd[comp.ID].AllowableActions = comp.RedfishChassisInfo.Actions.ChassisReset.AllowableValues

				case sm.CompEPTypeSystem:	//node
					hd[comp.ID].RfFQDN = comp.RfEndpointFQDN
					hd[comp.ID].PowerActionURI = comp.RedfishSystemInfo.Actions.ComputerSystemReset.Target
					hd[comp.ID].AllowableActions = comp.RedfishSystemInfo.Actions.ComputerSystemReset.AllowableValues
					hd[comp.ID].PowerStatusURI = comp.OdataID
					hd[comp.ID].PowerCapURI = comp.RedfishSystemInfo.PowerURL

				case sm.CompEPTypeManager:	//BMC
					hd[comp.ID].RfFQDN = comp.RfEndpointFQDN
					hd[comp.ID].PowerActionURI = comp.RedfishManagerInfo.Actions.ManagerReset.Target
					hd[comp.ID].AllowableActions = comp.RedfishManagerInfo.Actions.ManagerReset.AllowableValues
					hd[comp.ID].PowerStatusURI = comp.OdataID

				case sm.CompEPTypePDU:		//PDU outlet collection
					hd[comp.ID].RfFQDN = comp.RfEndpointFQDN
					hd[comp.ID].PowerStatusURI = comp.OdataID

				case sm.CompEPTypeOutlet:	//PDU outlet
					hd[comp.ID].RfFQDN = comp.RfEndpointFQDN
					hd[comp.ID].PowerActionURI = comp.RedfishOutletInfo.Actions.PowerControl.Target
					hd[comp.ID].AllowableActions = comp.RedfishOutletInfo.Actions.PowerControl.AllowableValues
					hd[comp.ID].PowerStatusURI = comp.OdataID

				default:
					b.HSMGlobals.Logger.Errorf("Unknown HSM Inventory/ComponentEndpoint type '%s'.",
						comp.ComponentEndpointType)
			}
		}
	}

	return nil
}

func (b *HSMv0) GetStateComponents(xnames []string) (base.ComponentArray,error) {
	var queryData CompQuery
	var retData base.ComponentArray

	smurl := b.HSMGlobals.SMUrl + hsmStateComponentsQueryPath
	queryData.ComponentIDs = xnames
	ba,baerr := json.Marshal(&queryData)
	if (baerr != nil) {
		return retData,fmt.Errorf("Error marshalling HSM component query data: %v",
			baerr)
	}

	req,err := http.NewRequest(http.MethodPost,smurl,bytes.NewBuffer(ba))
	if (err != nil) {
		return retData,fmt.Errorf("ERROR creating HTTP request for '%s': %v",
			smurl,err)
	}
/**/
	tokstr := os.Getenv("TOKEN")
	if (tokstr != "") {
		req.Header.Add("Authorization",fmt.Sprintf("Bearer %s",tokstr))
		req.Header.Add("Content-Type","application/json")
	}
/**/

	reqContext,_ := context.WithTimeout(context.Background(), 40 * time.Second)
	req = req.WithContext(reqContext)

	rsp,rsperr := b.HSMGlobals.SVCHttpClient.Do(req)
	if (rsperr != nil) {
		return retData,fmt.Errorf("Error in http request '%s': %v",smurl,rsperr)
	}

	body,bderr := ioutil.ReadAll(rsp.Body)
	if (bderr != nil) {
		return retData,fmt.Errorf("Error reading response body for '%s': %v",
			smurl,bderr)
	}

	bderr = json.Unmarshal(body,&retData)
	if (bderr != nil) {
		return retData,fmt.Errorf("Error unmarshalling response body for '%s': %v",
			smurl,bderr)
	}

	return retData,nil
}

func (b *HSMv0) FillHSMData(xnames []string) (map[string]*HsmData,error) {
	hdata := make(map[string]*HsmData)

	//Get state/component data to fill in ID/Type/Role

	compArray,caerr := b.GetStateComponents(xnames)
	if (caerr != nil) {
		return hdata,fmt.Errorf("ERROR fetching State/Component data from HSM: %v",
			caerr)
	}

	//Populate map with what we have so far

	for _,comp := range(compArray.Components) {
		hdata[(*comp).ID] = &HsmData{BaseData: (*comp)}
	}

	//Get Inventory/ComponentEndpoint data to fill in Hostname/Domain/FQDN/URIs

	inverr := b.FillComponentEndpointData(hdata)
	if (inverr != nil) {
		return hdata,fmt.Errorf("ERROR fetching Inventory/ComponentEndpoints data from HSM: %v",
			inverr)
	}

	return hdata,nil
}

