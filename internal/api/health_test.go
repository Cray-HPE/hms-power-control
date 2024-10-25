// MIT License
// 
// (C) Copyright [2022-2023] Hewlett Packard Enterprise Development LP
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

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

    "github.com/Cray-HPE/hms-certs/pkg/hms_certs"
    "github.com/Cray-HPE/hms-power-control/internal/hsm"
	trsapi "github.com/Cray-HPE/hms-trs-app-api/v2/pkg/trs_http_api"
    "github.com/Cray-HPE/hms-power-control/internal/credstore"
    "github.com/Cray-HPE/hms-power-control/internal/storage"
    "github.com/Cray-HPE/hms-power-control/internal/domain"
    "github.com/Cray-HPE/hms-power-control/internal/logger"
    "github.com/sirupsen/logrus"
    "github.com/stretchr/testify/suite"
)


type Models_TS struct {
	suite.Suite
}

var glogger = logrus.New()

var (
	Running   bool
	TLOC_rf   trsapi.TrsAPI
	DSP       storage.StorageProvider
	HSM       hsm.HSMProvider
	CS        credstore.CredStoreProvider
	DLOCK     storage.DistributedLockProvider
	svcClient *hms_certs.HTTPClientPair
)


// Since we're not actually running PCS per se, we have to set up globals
// ourselves to connect to the other services.

func setupGlobals(suite *Models_TS)  {
	var err error

	Running = true
	logger.Init()
	logger.Log.Trace()

	smsServer := os.Getenv("SMS_SERVER")
	if (smsServer == "") {
		smsServer = "http://blah/blah"
	}
	svcClient,err = hms_certs.CreateRetryableHTTPClientPair("",10,2,4)
	suite.Assert().Equal(nil,err,
		"ERROR creating retryable client pair: %v",err)
	glb := hsm.HSM_GLOBALS{SvcName: "HSMLayerTest", Logger: glogger,
	                       LockEnabled: false, SMUrl: smsServer,
	                       SVCHttpClient: svcClient}
	HSM = &hsm.HSMv2{}
	err = HSM.Init(&glb)
	suite.Assert().Equal(err,nil,"ERROR calling Init(): %v",err)

	workerSec := &trsapi.TRSHTTPLocal{}
	TLOC_rf = workerSec
	TLOC_rf.Init("HealthTest", glogger)

	tmpStorageImplementation := &storage.MEMStorage{
		Logger: glogger,
	}
	DSP = tmpStorageImplementation
	tmpDistLockImplementation := &storage.MEMLockProvider{}
	DLOCK = tmpDistLockImplementation
	DSP.Init(glogger)
	DLOCK.Init(glogger)

	tmpCS := &credstore.VAULTv0{}

	ve := os.Getenv("VAULT_ENABLED")
	vkp := os.Getenv("VAULT_KEYPATH")
	CS = tmpCS
	if (ve != "") {
		var credStoreGlob credstore.CREDSTORE_GLOBALS
		credStoreGlob.NewGlobals(glogger, &Running, 600, vkp)
		CS.Init(&credStoreGlob)
	}

	var domainGlobals domain.DOMAIN_GLOBALS
	domainGlobals.NewGlobals(nil, &TLOC_rf, nil, nil, nil,
	                         nil, &Running, &DSP, &HSM, (ve != ""),
	                         &CS, &DLOCK, 20000, 1440)
	domain.Init(&domainGlobals)

}

//Convenience func to do HTTP requests to prevent code duplication.

func doHTTP(url string, method string, pld []byte) ([]byte,int,error) {
	var rdata []byte
	var req *http.Request
	var err error

	if (method == http.MethodGet) {
		req,err = http.NewRequest(method,url,nil)
	} else {
		req,err = http.NewRequest(method,url,bytes.NewBuffer(pld))
	}
	if (err != nil) {
		return rdata,0,fmt.Errorf("Error creating HTTP request: %v",err)
	}

	rsp,perr := svcClient.Do(req)
	if (perr != nil) {
		return rdata,0,fmt.Errorf("Error performing http %s: %v",method,perr)
	}

	rdata,err = ioutil.ReadAll(rsp.Body)
	if (err != nil) {
		return rdata,0,fmt.Errorf("Error reading http rsp body: %v",err)
	}

	return rdata,rsp.StatusCode,nil
}

func (suite *Models_TS) TestHealthStuff() {
	var rsp []byte
	var scode int
	var err error
	var hrsp healthRsp

	t := suite.T()

	//First, set up global stuff, which the health code uses.

	setupGlobals(suite)

	smServer := httptest.NewServer(http.HandlerFunc(GetLiveness))
	defer smServer.Close()
	_,scode,err = doHTTP(smServer.URL,http.MethodGet,nil)
	suite.Assert().Equal(nil,err,"ERROR doing HTTP call to /liveness API: %v",err)
	suite.Assert().Equal(http.StatusNoContent,scode,
		"Bad status code: %d, was expecting %d",scode,http.StatusNoContent)

	smServer2 := httptest.NewServer(http.HandlerFunc(GetReadiness))
	defer smServer2.Close()
	_,scode,err = doHTTP(smServer2.URL,http.MethodGet,nil)
	suite.Assert().Equal(nil,err,"ERROR doing HTTP call to /readiness API: %v",err)
	suite.Assert().Equal(http.StatusNoContent,scode,
		"Bad status code: %d, was expecting %d",scode,http.StatusNoContent)

	smServer3 := httptest.NewServer(http.HandlerFunc(GetHealth))
	defer smServer3.Close()
	rsp,scode,err = doHTTP(smServer3.URL,http.MethodGet,nil)
	suite.Assert().Equal(nil,err,"ERROR doing HTTP call to /health API: %v",err)
	suite.Assert().Equal(http.StatusOK,scode,
		"Bad status code: %d, was expecting %d",scode,http.StatusOK)

	err = json.Unmarshal(rsp,&hrsp)
	suite.Assert().Equal(nil,err,"ERROR unmarshalling /health response: %v",err)

	t.Logf("RSP: '%s'",string(rsp))

	crsp := "connected, responsive"
	suite.Assert().Equal(crsp,hrsp.KvStore,
		"Mismatching KVStore status, exp: '%s' got: '%s'",crsp,hrsp.KvStore)
	suite.Assert().Equal(crsp,hrsp.DistLocking,
		"Mismatching DistLocking status, exp: '%s' got: '%s'",
		crsp,hrsp.DistLocking)
	suite.Assert().Equal(crsp,hrsp.StateManager,
		"Mismatching StateManager status, exp: '%s' got: '%s'",
		crsp,hrsp.StateManager)
	suite.Assert().Equal(crsp,hrsp.Vault,
		"Mismatching Vault status, exp: '%s' got: '%s'",
		crsp,hrsp.Vault)
	suite.Assert().Equal(crsp+", local mode",hrsp.TaskRunner,
		"Mismatching TaskRunner status, exp: '%s' got: '%s'",
		crsp,hrsp.TaskRunner)
}


func Test_Stuff(t *testing.T) {
    suite.Run(t,new(Models_TS))
}

