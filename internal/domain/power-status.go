/*
 * MIT License
 *
 * (C) Copyright [2022] Hewlett Packard Enterprise Development LP
 *
 * Permission is hereby granted, free of charge, to any person obtaining a
 * copy of this software and associated documentation files (the "Software"),
 * to deal in the Software without restriction, including without limitation
 * the rights to use, copy, modify, merge, publish, distribute, sublicense,
 * and/or sell copies of the Software, and to permit persons to whom the
 * Software is furnished to do so, subject to the following conditions:
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
 *
 */

package domain

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/Cray-HPE/hms-power-control/internal/credstore"
	"github.com/Cray-HPE/hms-power-control/internal/storage"
	pcshsm "github.com/Cray-HPE/hms-power-control/internal/hsm"
	pcsmodel "github.com/Cray-HPE/hms-power-control/internal/model"
	rf "github.com/Cray-HPE/hms-smd/pkg/redfish"
	trsapi "github.com/Cray-HPE/hms-trs-app-api/pkg/trs_http_api"
	"github.com/Cray-HPE/hms-xname/xnametypes"

	"github.com/sirupsen/logrus"
)


// Power monitoring framework.  The general flow of this framework:
//
// o Read all components and supporting info from HSM (includes various RF
//   URLs), and store them in an internal memory-cached map.
// o Periodically use this map to figure out which BMCs to query for HW state.
// o Query BMCs for HW state, and store the results both in the in-memory
//   cached component map and also in ETCD storage.
// o If user asks for power status on a set of components, it is fetched from
//   the ETCD-stored data.  This makes it possible for any instance of a 
//   multi-instance PCS service to be able to serve data retrieved by any other
//   instance.


// This is what gets stored in the in-memory component map.

type componentPowerInfo struct {
	PSComp      pcsmodel.PowerStatusComponent
	HSMData     pcshsm.HsmData
	BmcUsername string
	BmcPassword string
}

const (
	PSTATE_KEYRANGE_START = "x0"
	PSTATE_KEYRANGE_END   = "xz"
)

var hwStateMap = make(map[string]*componentPowerInfo)
var glogger *logrus.Logger
var hsmHandle *pcshsm.HSMProvider
var kvStore *storage.StorageProvider
var ccStore *credstore.CredStoreProvider
var distLocker *storage.DistributedLockProvider
var tloc *trsapi.TrsAPI
var pmSampleInterval time.Duration
var distLockMaxTime time.Duration
var pstateMonitorRunning bool
var serviceRunning *bool
var vaultEnabled = false
var httpTimeout = 30 //seconds


/////////////////////////////////////////////////////////////////////////////
//                P U B L I C   F U N C T I O N S
/////////////////////////////////////////////////////////////////////////////


// Initialize and start power state monitoring.
//
// domGlb:            Domain global object (holds lots of handles)
// distLockMaxTimeIn: Max time to hold dist'd lock before forfeiting
// loggerIn:          Logrus logger object.  Will create if nil.
// sampleInterval:    Time between HW status sampling.
// Return:            nil on success, else error message.

func PowerStatusMonitorInit(domGlb *DOMAIN_GLOBALS,
                            distLockMaxTimeIn time.Duration,
                            loggerIn *logrus.Logger,
                            sampleInterval time.Duration) error {
	if (loggerIn == nil) {
		glogger = logrus.New()
	} else {
		glogger = loggerIn
	}

	if (domGlb.HSM == nil) {
		return fmt.Errorf("ERROR: must supply valid HSM handle.")
	}
	if (domGlb.DSP == nil) {
		return fmt.Errorf("ERROR: must supply valid storage object.")
	}
	if (domGlb.CS == nil) {
		if (domGlb.VaultEnabled) {
			return fmt.Errorf("ERROR: must supply valid component cred store handle.")
		}
	}
	if (domGlb.DistLock == nil) {
		return fmt.Errorf("ERROR: must supply valid distributed lock object.")
	}
	if (distLockMaxTimeIn < time.Second) {
		return fmt.Errorf("ERROR: distributed lock max time must be >= 1 second.")
	}
	if (domGlb.RFTloc == nil) {
		return fmt.Errorf("ERROR: must supply valid Task Runner Service handle.")
	}
	if (sampleInterval < time.Second) {
		return fmt.Errorf("ERROR: power monitor sample interval must be >= 1 second.")
	}

	hsmHandle = domGlb.HSM
	kvStore = domGlb.DSP
	ccStore = domGlb.CS
	distLocker = domGlb.DistLock
	tloc = domGlb.RFTloc
	distLockMaxTime = distLockMaxTimeIn
	pmSampleInterval = sampleInterval
	serviceRunning = domGlb.Running
	vaultEnabled = domGlb.VaultEnabled

	go monitorHW()
	return nil
}

// Stop power status monitoring.

func PowerStatusMonitorStop() {
	pstateMonitorRunning = false
}

// Change the power status monitoring interval, on the fly.

func PowerStatusMonitorChangeInterval(newInterval time.Duration) error {
	if (newInterval < time.Second) {
		return fmt.Errorf("ERROR: power monitor sample interval must be >= 1 second.")
	}
	pmSampleInterval = newInterval
	return nil
}

// Get power status for given components.  Filter by power state and 
// management state.  Any undefined filter results in all states for
// the state category.
//
// xnames:          Array of component names for which to get power data
// pwrStateFilter:  Power state filter.
// mgmtStateFilter: Management state filter.  
// Return:          Passback object populated with model.PowerStatus object.

func GetPowerStatus(xnames []string,
                    pwrStateFilter pcsmodel.PowerStateFilter,
                    mgmtStateFilter pcsmodel.ManagementStateFilter) (pb pcsmodel.Passback) {
	//Grab all KVs from ETCD, cycle through them to make a map, then
	//match the xnames in the passed-in array.  Grab pertinent data and create
	//a pcsmodel.PowerStatus object 'pstatus', and return it.

	statusObj,err := (*kvStore).GetAllPowerStatus()
	if (err != nil) {
		//TODO: we don't have an HTTP status code from a failed 
		//GetAllPowerStatus() call; might need to pass that back in a 
		//future mod.
		return pcsmodel.BuildErrorPassback(http.StatusInternalServerError,err)
	}

	//Make a map of all returned components for xnames array matching.

	compMap := make(map[string]*pcsmodel.PowerStatusComponent)

	for ix,comp := range(statusObj.Status) {
		compMap[comp.XName] = &statusObj.Status[ix]
	}

	//Build a return object, filtering by xname.

	var rcomps pcsmodel.PowerStatus
	var robj pcsmodel.Passback

	psUndef := ((pwrStateFilter == pcsmodel.PowerStateFilter_Nil) ||
			    (pwrStateFilter == pcsmodel.PowerStateFilter_Undefined))
	msUndef := ((mgmtStateFilter == pcsmodel.ManagementStateFilter_Nil) ||
			    (mgmtStateFilter == pcsmodel.ManagementStateFilter_undefined))

	for _,name := range(xnames) {
		stateMatch := true
		mp,mapok := compMap[name]
		if (!mapok) {
			//Get the type.  If it has no support for power status, make the
			//error message reflect that.  Otherwise give a generic error.
			pcomp := pcsmodel.PowerStatusComponent{XName: name}
			htype := xnametypes.GetHMSType(name)
			switch (htype) {
				case xnametypes.Chassis:    fallthrough
				case xnametypes.ChassisBMC: fallthrough
				case xnametypes.NodeBMC:    fallthrough
				case xnametypes.RouterBMC:  fallthrough
				case xnametypes.Node:       fallthrough
				case xnametypes.CabinetPDUOutlet:
					pcomp.Error = "Component not found in component map."

				case xnametypes.HMSTypeInvalid:
					pcomp.Error = "Invalid component name."

				default:
					pcomp.Error = "Component can not have power state and managment state data"
			}
			rcomps.Status = append(rcomps.Status,pcomp)
			continue
		}
		//Filter by pwrstate and mgmtstate
		if (!psUndef) {
			if (strings.ToLower(pwrStateFilter.String()) != strings.ToLower(mp.PowerState)) {
				stateMatch = false
			}
		}
		if (!msUndef) {
			if (strings.ToLower(mgmtStateFilter.String()) != strings.ToLower(mp.ManagementState)) {
				stateMatch = false
			}
		}

		if (stateMatch) {
			var cmp pcsmodel.PowerStatusComponent
			cmp.SupportedPowerTransitions = make([]string,len(mp.SupportedPowerTransitions))
			cmp.XName = name
			cmp.PowerState = mp.PowerState
			cmp.ManagementState = mp.ManagementState
			copy(cmp.SupportedPowerTransitions,mp.SupportedPowerTransitions)
			rcomps.Status = append(rcomps.Status,cmp)
		}
	}

	robj = pcsmodel.BuildSuccessPassback(200,rcomps)
	return robj
}

/////////////////////////////////////////////////////////////////////////////
//                I N T E R N A L   F U N C T I O N S
/////////////////////////////////////////////////////////////////////////////

// Update the cached HW state map using HSM.  This will add new components
// to the map but not the actual HW states.

func updateComponentMap() error {
	fname := "updateComponentMap()"

	//Get all components in HSM

	compMap,err := (*hsmHandle).FillHSMData([]string{"all"})
	if (err != nil) {
		return fmt.Errorf("Error fetching HSM data: %v",err)
	}

	if (len(compMap) == 0) {
		return fmt.Errorf("HSM returned empty list of components!")
	}

	//TODO: Not sure if this is kosher... we'll remove any entry from
	//our in-memory component map if it is not returned by HSM.  That way
	//the in-memory map is "in sync" with HSM.   If for whatever reason
	//components are removed from HSM, we'll remove them from our internal
	//map too, so we don't keep trying to contact BMCs that may not exist.

	for k,_ := range(hwStateMap) {
		_,ok := compMap[k]
		if (!ok) {
			glogger.Infof("Removing '%s' from local map (no longer in HSM component list).", k)
			delete(hwStateMap,k)
			(*kvStore).DeletePowerStatus(k)
		}
	}

	//Filter on all pertinent component types and add to the component map.

	for _,v := range(compMap) {
		switch (xnametypes.HMSType(v.BaseData.Type)) {
			case xnametypes.Chassis:    fallthrough
			case xnametypes.ChassisBMC: fallthrough
			case xnametypes.NodeBMC:    fallthrough
			case xnametypes.RouterBMC:  fallthrough
			case xnametypes.Node:       fallthrough
			case xnametypes.CabinetPDUOutlet:
				_,ok := hwStateMap[v.BaseData.ID]
				if (!ok) {
					//New component.
					newComp := componentPowerInfo{}
					newComp.PSComp.XName = v.BaseData.ID
					newComp.PSComp.PowerState = pcsmodel.PowerStateFilter_Undefined.String()
					newComp.PSComp.ManagementState = pcsmodel.ManagementStateFilter_undefined.String()
					newComp.PSComp.SupportedPowerTransitions = v.AllowableActions
					newComp.HSMData.RfFQDN = v.RfFQDN
					newComp.HSMData.PowerStatusURI = v.PowerStatusURI
					newComp.HSMData.PowerActionURI = v.PowerActionURI
					newComp.HSMData.PowerCapURI = v.PowerCapURI
					newComp.PSComp.LastUpdated = time.Now().Format(time.RFC3339)
					hwStateMap[v.BaseData.ID] = &newComp
				}
			default:
				glogger.Tracef("%s: Component type not handled: %s",
					fname,string(v.BaseData.Type))
		}
	}

	return nil
}

// Check the comp map and populate the creds of any entries that have no
// cred info.  If a previous RF access failed due to bad creds, those creds
// will be deleted from the HW map entry, causing them to get re-populated
// here.

func updateVaultCreds() error {
	tmpMap := make(map[string]*componentPowerInfo)

	for k,v := range(hwStateMap) {
		if ((v.BmcUsername == "") || (v.BmcPassword != "")) {
			tmpMap[k] = v
		}
	}

	err := getVaultCredsAll(tmpMap)
	return err
}

func getVaultCredsAll(compMap map[string]*componentPowerInfo) error {
	var un,pw string
	var err error
	fname := "getVaultCredsAll()"

	if (!vaultEnabled || (ccStore == nil)) {
		glogger.Warnf("%s: Vault is disabled.",fname)
		return nil
	}

	//The credstore layer caches creds, so this should be fast.

	for k,v := range(compMap) {
		un,pw,err = (*ccStore).GetControllerCredentials(k)
		if (err != nil) {
			return fmt.Errorf("ERROR: Can't get BMC creds for '%s': %v",
						k,err)
		}
		if ((un == "") || (pw == "")) {
			glogger.Warnf("%s: Missing/empty creds for '%s'",fname,k)
		}
		v.BmcUsername = un
		v.BmcPassword = pw
	}

	return nil
}

// Return the HTTP status code from a completed TRS task.  This is a little
// funky... if a valid transaction yields no response, there will be no
// Response structure.  This would basically be a 204.  But if there is an
// error, you also get no Response struct, and thus no return code.  The
// "Err" field should have something in it, but the messages don't give good
// details.
//
// So for now, if there is no Response data and Err is nil, we will consider
// that a 204.  If there is no Response and Err is populated, it will be a 500.

func getStatusCode(tp *trsapi.HttpTask) int {
	ecode := 600
	if tp.Request.Response != nil {
		ecode = int(tp.Request.Response.StatusCode)
	} else {
		if tp.Err != nil {
			glogger.Tracef("getStatusCode, no response, err: '%v'", *tp.Err)
			ecode = int(http.StatusInternalServerError)
		} else {
			ecode = int(http.StatusNoContent)
		}
	}
	return ecode
}

// Fetch actual hardware status from hardware.

func getHWStatesFromHW() error {
	var url string
	var powerState pcsmodel.PowerStateFilter

	fname := "getHWStatesFromHW"
	hashXName := http.CanonicalHeaderKey("XName")
	hashCType := http.CanonicalHeaderKey("CType")
	sourceTL := trsapi.HttpTask{Timeout: time.Duration(httpTimeout) * time.Second,}

	//Get vault creds where needed

	cerr := updateVaultCreds()
	if (cerr != nil) {
		return cerr
	}

	//TODO: maybe ask HBTD for current HB status of node elements, and if 
	//they're heartbeating, set the status to ON and don't ask the HW.  This
	//can have a pretty large window of error however.

	//Use TRS to get all HW states.  Create a map so the TRS task completion
	//notifications can map back to an XName.

	taskList := (*tloc).CreateTaskList(&sourceTL,len(hwStateMap))
	activeTasks := 0

	for k,v := range(hwStateMap) {	//key is component XName, val is HSM RF EP info
		if (v.PSComp.ManagementState == pcsmodel.ManagementStateFilter_unavailable.String()) {
			continue
		}

		ctype := xnametypes.GetHMSType(k)
		switch (ctype) {
			case xnametypes.NodeBMC:    fallthrough
			case xnametypes.RouterBMC:  fallthrough
			case xnametypes.ChassisBMC: fallthrough
			case xnametypes.Node:       fallthrough
			case xnametypes.Chassis:    fallthrough
			case xnametypes.CabinetPDUOutlet:
				url = "https://" + v.HSMData.RfFQDN + v.HSMData.PowerStatusURI
				taskList[activeTasks].Request,_ = http.NewRequest(http.MethodGet,url,nil)
				taskList[activeTasks].Request.SetBasicAuth(v.BmcUsername,v.BmcPassword)
				//Hack alert: set the xname and comp type in the req header 
				//so we can use it when processing the responses.
				taskList[activeTasks].Request.Header.Add(hashXName,k)
				taskList[activeTasks].Request.Header.Add(hashCType,string(ctype))
				activeTasks ++

			default:
				glogger.Errorf("%s: Component type not handled: %s",
					fname,string(ctype))
				taskList[activeTasks].Ignore = true
		}
	}

	//Launch

	rchan,err := (*tloc).Launch(&taskList)
	if (err != nil) {
		return fmt.Errorf("%s: TRS Launch() error: %v.",fname,err)
	}

	//Pick off responses.

	nDone := 0
	for {
		task := <-rchan
		glogger.Debugf("%s: Task complete, URL: '%s', status code: %d",
			fname,task.Request.URL.Path,getStatusCode(task))
		nDone++
		if (nDone >= activeTasks) {
			break
		}
	}

	//For each response, get the XName via Request.Header["XName"].
	//Get it's type via Request.Header["CType"].

	for ii,_ := range(taskList) {
		if (taskList[ii].Ignore) {
			continue
		}

		//Grab the XName and component type from the header (put into
		//place in the requests above).
		needState := false
		xnameArr := taskList[ii].Request.Header[hashXName]
		ctypeArr := taskList[ii].Request.Header[hashCType]
		if ((len(xnameArr) == 0) || (len(ctypeArr) == 0)) {
			return fmt.Errorf("Internal error: response headers for xname and/or ctype are empty: '%v', '%v'",
				xnameArr,ctypeArr)
		}
		xname := xnameArr[0]
		ctype := ctypeArr[0]

		if (taskList[ii].Request.Response == nil) {
			//TODO: should this "ride through" transient failures?
			updateHWState(xname,pcsmodel.PowerStateFilter_Undefined,
				pcsmodel.ManagementStateFilter_undefined,"No response from target")
			continue
		}

		scode := getStatusCode(&taskList[ii])

		switch (scode) {
			case http.StatusBadRequest:          fallthrough
			case http.StatusNotFound:            fallthrough
			case http.StatusMethodNotAllowed:    fallthrough
			case http.StatusForbidden:           fallthrough
			case http.StatusNotImplemented:      fallthrough
			case http.StatusInternalServerError: fallthrough
			case http.StatusBadGateway:          fallthrough
			case http.StatusServiceUnavailable:
				glogger.Errorf("%s: Bad response from '%s', power state undefined: %d/%s",
					fname,xname,scode,http.StatusText(scode))
				updateHWState(xname,pcsmodel.PowerStateFilter_Undefined,
					pcsmodel.ManagementStateFilter_undefined,
					taskList[ii].Request.Response.Status)

			case http.StatusUnauthorized:
				updateHWState(xname,pcsmodel.PowerStateFilter_Undefined,
					pcsmodel.ManagementStateFilter_undefined,
					taskList[ii].Request.Response.Status)
				//Insure the next sweep gets new creds from Vault.
				hwStateMap[xname].BmcUsername = ""
				hwStateMap[xname].BmcPassword = ""

			default:
				if (scode >= 206) {
					updateHWState(xname,pcsmodel.PowerStateFilter_Undefined,
						pcsmodel.ManagementStateFilter_undefined,
						taskList[ii].Request.Response.Status)
				} else {
					needState = true
				}
		}

		glogger.Tracef("%s: %s needState: %t",fname,xname,needState)
		if (!needState) {
			continue
		}

		//At this point we have to actually decode the returned state info.

		rqURL := taskList[ii].Request.URL.Path
		stsBody, stserr := ioutil.ReadAll(taskList[ii].Request.Response.Body)

		if (stserr != nil) {
			glogger.Errorf("ERROR reading response body for '%s' '%s': %v",
				xname,rqURL,stserr)
			//Power state unknown, but got a response, so mgmt state OK
			updateHWState(xname,pcsmodel.PowerStateFilter_Undefined,
				pcsmodel.ManagementStateFilter_available,
				"Unable to read response body")
			continue
		}

		switch (xnametypes.HMSType(ctype)) {
			case xnametypes.NodeBMC:    fallthrough
			case xnametypes.RouterBMC:  fallthrough
			case xnametypes.ChassisBMC:
				//Any valid response means ON
				updateHWState(xname,pcsmodel.PowerStateFilter_On,
					pcsmodel.ManagementStateFilter_available,"")

			case xnametypes.Node:
				//Nodes: look for "PowerState" in response payload.
				var info rf.ComputerSystem
				err = json.Unmarshal(stsBody, &info)
				if (err != nil) {
					glogger.Errorf("ERROR unmarshalling power payload for '%s': %v",
						rqURL,err)
					//Power state unknown, but got a response, so mgmt state OK
					updateHWState(xname,pcsmodel.PowerStateFilter_Undefined,
						pcsmodel.ManagementStateFilter_available,
						"Unable to unmarshal power payload")
					break
				}
				powerState,err = pcsmodel.ToPowerStateFilter(info.PowerState)
				if (err != nil) {
					glogger.Errorf("Invalid power state from HW: '%s', setting to undefined.",
						info.PowerState)
					powerState = pcsmodel.PowerStateFilter_Undefined
				}
				updateHWState(xname,powerState,pcsmodel.ManagementStateFilter_available,"")

			case xnametypes.Chassis:
				//Mt Chassis: PowerState is for rect status
				var info rf.Chassis
				err = json.Unmarshal(stsBody, &info)
				if (err != nil) {
					glogger.Errorf("ERROR unmarshalling power payload for '%s': %v",
						rqURL,err)
					//Power state unknown, but got a response, so mgmt state OK
					updateHWState(xname,pcsmodel.PowerStateFilter_Undefined,
						pcsmodel.ManagementStateFilter_available,
						"Unable to unmarshal power payload")
					break
				}
				powerState,_ = pcsmodel.ToPowerStateFilter(info.PowerState)
				updateHWState(xname,powerState,
					pcsmodel.ManagementStateFilter_available,"")

			case xnametypes.CabinetPDUPowerConnector:
				var info rf.Outlet
				err = json.Unmarshal(stsBody, &info)
				if (err != nil) {
					glogger.Errorf("ERROR unmarshalling power payload for '%s': %v",
						rqURL,err)
					//Power state unknown, but got a response, so mgmt state OK
					updateHWState(xname,pcsmodel.PowerStateFilter_Undefined,
						pcsmodel.ManagementStateFilter_available,
						"Unable to unmarshal power payload")
					break
				}
				powerState,_ = pcsmodel.ToPowerStateFilter(info.PowerState)
				updateHWState(xname,powerState,pcsmodel.ManagementStateFilter_available,"")
			default:
				glogger.Errorf("Error: %s: unknown component type.", ctype)
				updateHWState(xname,pcsmodel.PowerStateFilter_Undefined,
					pcsmodel.ManagementStateFilter_available,
					"Unknown component type")
		}
	}

	return nil
}

// Goroutine for monitoring HW and updating the HW state database.  For now,
// we'll do all of the checking in a single instance rather than sharding.
// Since we are using TRS that should be OK -- small systems thread off the
// work, large systems will use remote/worker mode which will shard things for
// us.

func monitorHW() {
	//Make sure we only start this thread once.

	if (pstateMonitorRunning) {
		return
	}

	pstateMonitorRunning = true
	for {
		//Check for exit conditions
		if (!pstateMonitorRunning || !(*serviceRunning)) {
			return
		}

		time.Sleep(pmSampleInterval)

		//Get dist'd ETCD lock.

		lckerr := (*distLocker).DistributedTimedLock(distLockMaxTime)
		if (lckerr != nil) {
			//Lock is already held.  This means someone else is doing the check,
			//so we don't have to.

			glogger.Debugf("HB checker being done elsewhere, skipping.")
			glogger.Debugf("  (returned: %v)",lckerr)
			continue
		}

		//Get map of all components in HSM and their BMCs.  This maybe only 
		//needs to be done once in a while.

		err := updateComponentMap()
		if (err != nil) {
			glogger.Errorf("Error getting component list from HSM: %v",err)
			err = (*distLocker).Unlock()
			if (err != nil) {
				glogger.Errorf("ERROR releasing distributed lock: %v",err)
			}
			continue
		}

		//Update the current power states of all components in the component
		//map by reading the actual hardware.

		err = getHWStatesFromHW()

		if (err != nil) {
			//This means probably nothing got anywhere, so ignore it.
			glogger.Errorf("ERROR getting HW states: %v",err)
			err = (*distLocker).Unlock()
			if (err != nil) {
				glogger.Errorf("ERROR releasing distributed lock: %v",err)
			}
			continue
		}

		err = (*distLocker).Unlock()
		if (err != nil) {
			glogger.Errorf("ERROR releasing distributed lock: %v",err)
		}
	}
}

// Update the HW state of all components, in our backing store.

func updateHWState(xname string, hwState pcsmodel.PowerStateFilter,
                   mgmtState pcsmodel.ManagementStateFilter, errInfo string) {
	funcname := "updateHWState()"

	comp,ok := hwStateMap[xname]
	if (!ok) {
		//Something's wrong...
		glogger.Errorf("%s: INTERNAL ERROR: no HW map entry for '%s'",
			funcname,xname)
		return
	}

	//See if the HW state has changed, and if so, update the ETCD record.

	hwStateStr   := strings.ToLower(hwState.String())
	mgmtStateStr := strings.ToLower(mgmtState.String())

	if ((hwStateStr == strings.ToLower(comp.PSComp.PowerState)) && (mgmtStateStr == strings.ToLower(comp.PSComp.ManagementState))) {
		return
	}

	//Update local map

	comp.PSComp.PowerState = hwStateStr
	comp.PSComp.ManagementState = mgmtStateStr
	comp.PSComp.LastUpdated = time.Now().Format(time.RFC3339Nano)
	comp.PSComp.Error = errInfo

	//Update stored map

	var psc pcsmodel.PowerStatusComponent
	psc.XName = xname
	psc.PowerState = hwStateStr
	psc.ManagementState = mgmtStateStr
	psc.Error = errInfo
	psc.SupportedPowerTransitions = comp.PSComp.SupportedPowerTransitions
	psc.LastUpdated = comp.PSComp.LastUpdated

	err := (*kvStore).StorePowerStatus(psc)
	if (err != nil) {
		glogger.Errorf("%s: ERROR storing component state for '%s': %v",
			funcname,xname,err)
		return
	}
}

