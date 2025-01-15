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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	base "github.com/Cray-HPE/hms-base/v2"
	"github.com/Cray-HPE/hms-certs/pkg/hms_certs"
	"github.com/Cray-HPE/hms-xname/xnametypes"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/suite"
)

type Models_TS struct {
	suite.Suite
}

var glogger = logrus.New()

func (suite *Models_TS) TestInit() {
	svcClient,err := hms_certs.CreateRetryableHTTPClientPair("",10,10,4)
	suite.Assert().Equal(nil,err,"ERROR creating retryable client pair: %v",err)
	glb := HSM_GLOBALS{SvcName: "HSMLayerTest", Logger: glogger,
	                   LockEnabled: false, SMUrl: "http://blah/blah",
	                   SVCHttpClient: svcClient}
	HSM := &HSMv2{}
	err = HSM.Init(&glb)
	suite.Assert().Equal(err,nil,"ERROR calling Init(): %v",err)

	//Try error stuff

	glb2 := glb
	glb2.SMUrl = ""
	err = HSM.Init(&glb2)
	suite.Assert().NotEqual(err,nil,"ERROR Init(1) should have failed, did not.")

	glb2 = glb
	glb2.SVCHttpClient = nil
	err = HSM.Init(&glb2)
	suite.Assert().NotEqual(err,nil,"ERROR Init(2) should have failed, did not.")
}

func getSMURL() string {
	envstr := os.Getenv("SMS_SERVER")
	return envstr
}

func (suite *Models_TS) TestPing() {
	t := suite.T()
	svcClient,err := hms_certs.CreateRetryableHTTPClientPair("",10,10,4)
	suite.Assert().Equal(err,nil,"ERROR creating retryable client pair: %v",err)

	smURL := getSMURL()
	if (smURL == "") {
		t.Logf("INFO: No SM URL specified, nothing to do.")
		return
	}
	glb := HSM_GLOBALS{SvcName: "HSMLayerTest", Logger: glogger,
	                   LockEnabled: false, SMUrl: smURL,
	                   SVCHttpClient: svcClient}
	HSM := &HSMv2{}
	err = HSM.Init(&glb)
	suite.Assert().Equal(err,nil,"ERROR calling Init(): %v",err)

	err = HSM.Ping()
	suite.Assert().Equal(err,nil,"Ping() failed: %v",err)
}

//Get a list of discovered components, and return a list of components
//of the given type(s), specified as e.g. "Node,NodeBMC,ChassisBMC".  
//"" means keep select for all discovered components.

func getDiscoveredComponents(ctype string) ([]string,error) {
	var comps []string
	var qstr string

	smURL := getSMURL()
	if (smURL == "") {
		return comps,fmt.Errorf("INFO: Can't get SM URL from env, nothing to do.")
	}

	if (ctype != "") {
		qs := strings.Split(ctype,",")
		qs2 := "&type=" + strings.Join(qs,"&type=")
		qstr = strings.Replace(qs2,"&","?",1)
	}

	rsp,scode,err := doHTTP(smURL+"/hsm/v2/State/Components"+qstr,
		http.MethodGet,nil)
	if (err != nil) {
		return comps,fmt.Errorf("Error in HTTP request for HSM components: %v",
			err)
	}
	if (scode != http.StatusOK) {
		return comps,fmt.Errorf("Error getting components, bad rsp code: %d",scode)
	}

	var compData base.ComponentArray
	err = json.Unmarshal(rsp,&compData)
	if (err != nil) {
		return comps,fmt.Errorf("Error unmarshalling component data: %v",err)
	}

	for _,ep := range(compData.Components) {
		comps = append(comps,ep.ID)
	}

	return comps,nil
}

//Convenience func to do HTTP requests to prevent code duplication.

func doHTTP(url string, method string, pld []byte) ([]byte,int,error) {
	var rdata []byte
	var req *http.Request

	svcClient,err := hms_certs.CreateRetryableHTTPClientPair("",10,10,4)
	if (err != nil) {
		return rdata,0,fmt.Errorf("ERROR creating retryable client pair: %v",err)
	}

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
		// Always drain and close response bodies
		if rsp != nil && rsp.Body != nil {
			_, _ = io.Copy(io.Discard, rsp.Body)
			rsp.Body.Close()
		}

		return rdata,0,fmt.Errorf("Error performing http %s: %v",method,perr)
	}

	rdata,err = io.ReadAll(rsp.Body)

	// Always close response bodies
	if rsp != nil && rsp.Body != nil {
		rsp.Body.Close()
	}

	if (err != nil) {
		return rdata,0,fmt.Errorf("Error reading http rsp body: %v",err)
	}

	return rdata,rsp.StatusCode,nil
}

func (suite *Models_TS) TestReserveComponents() {
	t := suite.T()
	var keyList []ReservationData
	var tr bool

	//Get a list of discovered components from HSM, make a list of all
	//BMC and/or controller types.  Get reservations for
	//all of them.  Call CheckDeputyKeys using the dep keys returned.  Then
	//release the reservations.

	comps,cerr := getDiscoveredComponents(xnametypes.CabinetPDUController.String()+","+
										  xnametypes.CabinetPDUOutlet.String()+","+
										  xnametypes.Chassis.String()+","+
										  xnametypes.ChassisBMC.String()+","+
										  xnametypes.NodeBMC.String()+","+
										  xnametypes.RouterBMC.String()+","+
										  xnametypes.Node.String())
	if ((cerr != nil) || (len(comps) == 0)) {
		t.Log("INFO: Can't get components from env, nothing to do.")
		return
	}

	smURL := getSMURL()
	if (smURL == "") {
		t.Logf("INFO: Can't get SM URL from env, nothing to do.")
		return
	}

	for _,cc := range(comps) {
		keyList = append(keyList,ReservationData{XName: cc})
	}

	svcClient,err := hms_certs.CreateRetryableHTTPClientPair("",10,10,4)
	suite.Assert().Equal(err,nil,"ERROR creating retryable client pair: %v",err)

	glogger.SetLevel(logrus.TraceLevel)
	glb := HSM_GLOBALS{SvcName: "HSMLayerTest", Logger: glogger,
	                   LockEnabled: true, SMUrl: smURL,
	                   SVCHttpClient: svcClient}
	HSM := &HSMv2{}
	err = HSM.Init(&glb)
	suite.Assert().Equal(err,nil,"ERROR calling Init(): %v",err)

	unlockedList,rerr := HSM.ReserveComponents(keyList)
	suite.Assert().Equal(rerr,nil,"ReserveComponents() failed: %v",rerr)

	suite.Assert().Equal(len(unlockedList),len(keyList),
		"Unlocked list expected length: %d, got %d",
			len(keyList), len(unlockedList))

	for ix,key := range(unlockedList) {
		t.Logf("Now-locked Key[%d]: '%v'",ix,key)
		tr = suite.Assert().Equal(key.Error,nil,"   ERROR: '%v'",key.Error)
		if (tr) {
			t.Logf("   OK.")
		}
	}

	//Check the deputy keys for validity

	err = HSM.CheckDeputyKeys(keyList)
	suite.Assert().Equal(err,nil,"CheckDeputyKeys() failed: %v",err)

	//Display the dep keys results

	for ix,key := range(keyList) {
		t.Logf("Reservation[%d]: '%v'",ix,key)
		tr = suite.Assert().Equal(key.Error,nil,"   ERROR: '%v'",key.Error)
		if (tr) {
			t.Logf("   OK.")
		}
	}

	//Now release the keys we own

	relList,relErr := HSM.ReleaseComponents(keyList)
	suite.Assert().Equal(relErr,nil,"ReleaseComponents() failed: %v",relErr)

	suite.Assert().Equal(len(relList),0,"ReleaseComponents() didn't release all components!")

	for ix,key := range(unlockedList) {
		t.Logf("Released Key[%d]: '%v'",ix,key)
	}
}


func (suite *Models_TS) TestFillHSMData() {
	t := suite.T()
	var tr bool

	//Get discovered components.  Keep all of them
	compList,cerr := getDiscoveredComponents("")
	if ((cerr != nil) || (len(compList) == 0)) {
		t.Log("Can't get components from env, nothing to do.")
		return
	}

	smURL := getSMURL()
	tr = suite.Assert().NotEqual(smURL,"","Can't get SM URL from env, nothing to do.")
	if (!tr) {
		return
	}

	svcClient,err := hms_certs.CreateRetryableHTTPClientPair("",10,10,4)
	suite.Assert().Equal(err,nil,"ERROR creating retryable client pair: %v",err)

	glb := HSM_GLOBALS{SvcName: "HSMLayerTest", Logger: glogger,
	                   LockEnabled: true, SMUrl: smURL,
	                   SVCHttpClient: svcClient, MaxComponentQuery: 1}
	HSM := &HSMv2{}
	err = HSM.Init(&glb)
	suite.Assert().Equal(err,nil,"ERROR calling Init(): %v",err)

	compMap,err := HSM.FillHSMData(compList)
	suite.Assert().Equal(err,nil,"ERROR FillHSMData(): %v",err)
	suite.Assert().Equal(len(compMap),len(compList),
		"Length mismatch of returned map, exp: %d, got: %d",
			len(compList),len(compMap))

	for _,comp := range(compList) {
		ok := false
		for k,_ := range(compMap) {
			if (k == comp) {
				ok = true
				break
			}
		}
		suite.Assert().True(ok,"ERROR: No match found for '%s'",comp)
	}
}

func Test_Stuff(t *testing.T) {
	suite.Run(t,new(Models_TS))
}
