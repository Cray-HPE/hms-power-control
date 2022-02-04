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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"github.com/Cray-HPE/hms-smd/pkg/sm"
	smreservations "github.com/Cray-HPE/hms-smd/pkg/service-reservations"
	"github.com/Cray-HPE/hms-certs/pkg/hms_certs"
	"github.com/sirupsen/logrus"
)

var glogger = logrus.New()

func TestInit(t *testing.T) {
	svcClient,err := hms_certs.CreateRetryableHTTPClientPair("",10,10,4)
	if (err != nil) {
		t.Errorf("ERROR creating retryable client pair: %v",err)
	}
	glb := HSM_GLOBALS{SvcName: "HSMLayerTest", Logger: glogger,
	                   LockEnabled: false, SMUrl: "http://blah/blah",
	                   SVCHttpClient: svcClient}
	HSM := &HSMv0{}
	err = HSM.Init(&glb)
	if (err != nil) {
		t.Errorf("ERROR calling Init(): %v",err)
	}

	//Try error stuff

	glb2 := glb
	glb2.SMUrl = ""
	err = HSM.Init(&glb2)
	if (err == nil) {
		t.Errorf("ERROR Init(1) should have failed, did not.")
	}

	glb2 = glb
	glb2.SVCHttpClient = nil
	err = HSM.Init(&glb2)
	if (err == nil) {
		t.Errorf("ERROR Init(2) should have failed, did not.")
	}
}

func smLivenessHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func smReserveCheckHandler(w http.ResponseWriter, r *http.Request) {
	var jdata sm.CompLockV2DeputyKeyArray
	var retData sm.CompLockV2ReservationResult
	fname := "smReserveCheckHandler()"

	body,err := ioutil.ReadAll(r.Body)
	if (err != nil) {
		glogger.Errorf("%s: Error reading request body: %v",fname,err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
glogger.Errorf(">> ReserveCheckHandler, inbound ba: '%s'",string(body))
	err = json.Unmarshal(body,&jdata)
	if (err != nil) {
		glogger.Errorf("%s: Error unmarshalling request body: %v",fname,err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	//For each inbound array element, populate the response.
//	//Simulate half success, half failure.

	for ix := 0; ix < len(jdata.DeputyKeys); ix ++ {
		if (jdata.DeputyKeys[ix].Key == "") {
			retData.Failure = append(retData.Failure,
				sm.CompLockV2Failure{ID: jdata.DeputyKeys[ix].ID,
			                     Reason: fmt.Sprintf("RSVKey_%d_failed",ix),})
		} else {
			retData.Success = append(retData.Success,
				sm.CompLockV2Success{ID: jdata.DeputyKeys[ix].ID,
			                     DeputyKey: jdata.DeputyKeys[ix].Key})
		}
	}


/*	fhalf := len(jdata.DeputyKeys) / 2
	for ix := 0; ix < fhalf; ix ++ {
		retData.Success = append(retData.Success,
			sm.CompLockV2Success{ID: jdata.DeputyKeys[ix].ID,
			                     DeputyKey: jdata.DeputyKeys[ix].Key,
			                     ReservationKey: fmt.Sprintf("RSVKey_%d",ix),
			                     CreationTime: "Then",
			                     ExpirationTime: "Whenever",})
	}
	for ix := fhalf; ix < len(jdata.DeputyKeys); ix ++ {
		retData.Failure = append(retData.Failure,
			sm.CompLockV2Failure{ID: jdata.DeputyKeys[ix].ID,
			                     Reason: fmt.Sprintf("RSVKey_%d_failed",ix),})
	}
*/

	ba,baerr := json.Marshal(&retData)
	if (baerr != nil) {
		glogger.Errorf("%s: Error marshalling response data: %v",fname,err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
glogger.Errorf(">> ReserveCheckHandler, return ba: '%s'",string(ba))

	w.Header().Set("Content-Type","application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(ba)
}

func smReservationHandler(w http.ResponseWriter, r *http.Request) {
	//For now, just return http.StatusOK.  If we want to get fancy, we can
	//mimic what is done in  smd/pkg/service-reservations/interface.go, in
	//the function Aquire().  Or, we can choose to just return an error to test
	//the error paths. //ZZZZ

	fname := "smReservationHandler()"
	var jdata smreservations.ReservationCreateParameters
	var rsp smreservations.ReservationCreateResponse

	body,_ := ioutil.ReadAll(r.Body)
	err := json.Unmarshal(body,&jdata)
	if (err != nil) {
		glogger.Errorf("%s: Error unmarshalling req data: %v",fname,err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	//Note that HSM always returns successes only.

	for ix,comp := range(jdata.ID) {
		rsp.Success = append(rsp.Success,
				smreservations.ReservationCreateSuccessResponse{ID: comp,
			        ReservationKey: fmt.Sprintf("RSVKey_%d",ix),
					DeputyKey: fmt.Sprintf("DEPKey_%d",ix),
					ExpirationTime: fmt.Sprintf("ExpTime_%d",ix)})
	}

/*	for ix,comp := range(gkeyList) {
		//Only create reservation key if there is no deputy key

		rsp.Success = append(rsp.Success,
				smreservations.ReservationCreateSuccessResponse{ID: comp.XName,
			        ReservationKey: fmt.Sprintf("RSVKey_%d",ix),
					DeputyKey: comp.DeputyKey,
					ExpirationTime: fmt.Sprintf("ExpTime_%d",ix)})
	}
*/

	ba,baerr := json.Marshal(&rsp)
	if (baerr != nil) {
		glogger.Errorf("%s: Error marshalling response data: %v",fname,baerr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type","application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(ba)
}

func smReservationReleaseHandler(w http.ResponseWriter, r *http.Request) {
	var relList smreservations.ReservationReleaseParameters
	var retData smreservations.ReservationReleaseRenewResponse
	fname := "smReservationReleaseHandler()"

	body,_ := ioutil.ReadAll(r.Body)
glogger.Errorf(">> ReserveReleaseHandler, inbound ba: '%s'",string(body))
	err := json.Unmarshal(body,&relList)
	if (err != nil) {
		glogger.Errorf("%s: Error unmarshalling req data: %v",fname,err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	//Blindly make all releases OK

	for _,comp := range(relList.ReservationKeys) {
		retData.Success.ComponentIDs = append(retData.Success.ComponentIDs,
			comp.ID)
	}
	retData.Counts.Total = len(relList.ReservationKeys)
	retData.Counts.Success = len(relList.ReservationKeys)

	ba,baerr := json.Marshal(&retData)
	if (baerr != nil) {
		glogger.Errorf("%s: Error marshalling response data: %v",fname,baerr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
glogger.Errorf(">> ReserveReleaseHandler, outbound ba: '%s'",string(ba))

	w.Header().Set("Content-Type","application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(ba)
}

func TestPing(t *testing.T) {
	svcClient,err := hms_certs.CreateRetryableHTTPClientPair("",10,10,4)
	if (err != nil) {
		t.Errorf("ERROR creating retryable client pair: %v",err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(hsmLivenessPath,http.HandlerFunc(smLivenessHandler))
	smServer := httptest.NewServer(mux)
	defer smServer.Close()

	glb := HSM_GLOBALS{SvcName: "HSMLayerTest", Logger: glogger,
	                   LockEnabled: false, SMUrl: smServer.URL,
	                   SVCHttpClient: svcClient}
	HSM := &HSMv0{}
	err = HSM.Init(&glb)
	if (err != nil) {
		t.Errorf("ERROR calling Init(): %v",err)
	}

	err = HSM.Ping()
	if (err != nil) {
		t.Errorf("Ping() failed: %v",err)
	}
}

var gkeyList = []ReservationData{
				{XName: "x0c0s0b0n0",   DeputyKey: "aaaaaa"},
				{XName: "x1c1s1b1n1",   DeputyKey: "bbbbbb"},
				{XName: "x100c0s0b0n0"/*, DeputyKey: "yyyyyy"*/},
				{XName: "x101c0s0b0n0"/*, DeputyKey: "zzzzzz"*/},}

func TestCheckDeputyKeys(t *testing.T) {
	keyList := make([]ReservationData,len(gkeyList))

	if ((len(gkeyList) %2) != 0) {
		t.Fatalf("TEST ERROR: keylist length must be multiple of 2.")
	}

	copy(keyList,gkeyList)

	svcClient,err := hms_certs.CreateRetryableHTTPClientPair("",10,10,4)
	if (err != nil) {
		t.Errorf("ERROR creating retryable client pair: %v",err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc(hsmLivenessPath,http.HandlerFunc(smLivenessHandler))
	mux.HandleFunc(hsmReservationCheckPath,http.HandlerFunc(smReserveCheckHandler))
	mux.HandleFunc(hsmReservationPath,http.HandlerFunc(smReservationHandler))
	smServer := httptest.NewServer(mux)
	defer smServer.Close()

	glb := HSM_GLOBALS{SvcName: "HSMLayerTest", Logger: glogger,
	                   LockEnabled: true, SMUrl: smServer.URL,
	                   SVCHttpClient: svcClient}
	HSM := &HSMv0{}
	err = HSM.Init(&glb)
	if (err != nil) {
		t.Errorf("ERROR calling Init(): %v",err)
	}

	err = HSM.Ping()
	if (err != nil) {
		t.Errorf("Ping() failed: %v",err)
	}
	err = HSM.CheckDeputyKeys(keyList)
	if (err != nil) {
		t.Errorf("CheckDeputyKeys() failed: %v",err)
	}

	oks := 0
	bads := 0
	oklen := 0
	badlen := 0
	for _,key := range(gkeyList) {
		if (key.DeputyKey != "") {
			oklen ++
		} else {
			badlen ++
		}
	}

	for ix,key := range(keyList) {
		t.Logf("Key[%d]: '%v'",ix,key)
		if (key.Error == "") {
			oks ++
		} else {
			bads ++
		}
	}
	if ((oks != oklen) || (bads != badlen)) {
		t.Errorf("Deputy key ok/bad incorrect, expecting %d success, %d failure, got success: %d, failure: %d",
			oklen,badlen,oks,bads)
	}
}


func TestReserveComponents(t *testing.T) {
	keyList := make([]ReservationData,len(gkeyList))

	if ((len(keyList) %2) != 0) {
		t.Fatalf("TEST ERROR: keylist length must be multiple of 2.")
	}

	copy(keyList,gkeyList)

	svcClient,err := hms_certs.CreateRetryableHTTPClientPair("",10,10,4)
	if (err != nil) {
		t.Errorf("ERROR creating retryable client pair: %v",err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc(hsmLivenessPath,http.HandlerFunc(smLivenessHandler))
	mux.HandleFunc(hsmReservationCheckPath,http.HandlerFunc(smReserveCheckHandler))
	//mux.HandleFunc(smreservations.HSM_DEFAULT_RESERVATION_PATH,http.HandlerFunc(smReservationHandler))
	//mux.HandleFunc(smreservations.HSM_DEFAULT_RESERVATION_PATH+"/release",
	//		http.HandlerFunc(smReservationReleaseHandler))
	mux.HandleFunc(hsmReservationPath,http.HandlerFunc(smReservationHandler))
	mux.HandleFunc(hsmReservationReleasePath,
			http.HandlerFunc(smReservationReleaseHandler))
	smServer := httptest.NewServer(mux)
	defer smServer.Close()

	glb := HSM_GLOBALS{SvcName: "HSMLayerTest", Logger: glogger,
	                   LockEnabled: true, SMUrl: smServer.URL,
	                   SVCHttpClient: svcClient}
	HSM := &HSMv0{}
	err = HSM.Init(&glb)
	if (err != nil) {
		t.Errorf("ERROR calling Init(): %v",err)
	}

	unlockedList,rerr := HSM.ReserveComponents(keyList)
	if (rerr != nil) {
		t.Errorf("ReserveComponents() failed: %v",rerr)
	}

	if (len(unlockedList) != (len(keyList) / 2)) {
		t.Errorf("Unlocked list expected length: %d, got %d",
			len(keyList)/2, len(unlockedList))
	}

	for ix,key := range(unlockedList) {
		t.Logf("Now-locked Key[%d]: '%v'",ix,key)
		if (key.Error != "") {
			t.Logf("   ERROR: '%s'",key.Error)
		} else {
			t.Logf("   OK.")
		}
	}

	//Now release the keys we own

	err = HSM.ReleaseComponents(unlockedList)
	if (err != nil) {
		t.Errorf("ReleaseComponents() failed: %v",err)
	}

	for ix,key := range(unlockedList) {
		t.Logf("Released Key[%d]: '%v'",ix,key)
	}
}

