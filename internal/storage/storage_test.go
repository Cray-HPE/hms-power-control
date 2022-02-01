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

package storage

import (
	"fmt"
	"github.com/Cray-HPE/hms-power-control/internal/model"
	"os"
	"sort"
	"testing"
	"time"
)

//Tests will use the memory interface, since it exercises both the memory and
//ETCD implementations.

func pcompEqual(a, b *model.PowerStatusComponent) bool {
	if a.XName != b.XName {
		return false
	}
	if a.PowerState != b.PowerState {
		return false
	}
	if a.ManagementState != b.ManagementState {
		return false
	}
	if a.Error != b.Error {
		return false
	}
	if len(a.SupportedPowerTransitions) != len(b.SupportedPowerTransitions) {
		return false
	}

	asrt := a.SupportedPowerTransitions
	bsrt := b.SupportedPowerTransitions
	sort.Strings(asrt)
	sort.Strings(bsrt)
	for ix := 0; ix < len(asrt); ix++ {
		if asrt[ix] != bsrt[ix] {
			return false
		}
	}
	return true
}

func TestPowerStatusStorage(t *testing.T) {
	var ms StorageProvider
	var ds DistributedLockProvider

	if (os.Getenv("ETCD_HOST") != "") && (os.Getenv("ETCD_PORT") != "") {
		t.Logf("Using ETCD backing store.")
		ms = &ETCDStorage{}
		ds = &ETCDLockProvider{}
	} else {
		t.Logf("Using In-Memory backing store.")
		ms = &MEMStorage{}
		ds = &MEMLockProvider{}
	}

	err := ms.Init(nil)
	if err != nil {
		t.Errorf("Storage Init() failed: %v", err)
	}

	err = ms.Ping()
	if err != nil {
		t.Errorf("Storage Ping() failed: %v", err)
	}

	ds.InitFromStorage(ms, nil)
	if err != nil {
		t.Errorf("DistLock InitFromStorage() failed: %v", err)
	}

	err = ds.Ping()
	if err != nil {
		t.Errorf("DistLock Ping() failed: %v", err)
	}

	pstat := model.PowerStatusComponent{XName: "x0c0s0b0n0",
		PowerState:                "on",
		ManagementState:           "available",
		Error:                     "OK",
		SupportedPowerTransitions: []string{"On", "Off"},
	}

	err = ms.StorePowerStatus(pstat)
	if err != nil {
		t.Errorf("StorePowerStatus() failed: %v", err)
	}

	pcomp, perr := ms.GetPowerStatus("x0c0s0b0n0")
	if perr != nil {
		t.Errorf("GetPowerStatus() failed: %v", perr)
	}
	if !pcompEqual(&pcomp, &pstat) {
		t.Errorf("GetPowerStatus returned incorrect data: Exp: '%v', Act: '%v'",
			pstat, pcomp)
	}

	pcomp, perr = ms.GetPowerStatus("x1c2s1b1n1")
	if perr == nil {
		t.Errorf("GetPowerStatus() should have failed (bad xname), did not.")
	}

	err = ms.DeletePowerStatus("x0c0s0b0n0")
	if err != nil {
		t.Errorf("StorePowerStatus() failed: %v", err)
	}

	pcomp, perr = ms.GetPowerStatus("x0c0s0b0n0")
	if perr == nil {
		t.Errorf("GetPowerStatus() should have failed (non-existent xname), did not.")
	}

	maxIX := 3

	for ix := 0; ix <= maxIX; ix++ {
		pst := pstat
		pst.XName = fmt.Sprintf("x%dc%ds%db%dn%d", ix, ix, ix, ix, ix)
		err := ms.StorePowerStatus(pst)
		if err != nil {
			t.Errorf("StorePowerStatus() failed (%s): %v", pst.XName, err)
		}
	}

	parr, paerr := ms.GetAllPowerStatus()
	if paerr != nil {
		t.Errorf("GetAllPowerStatus() failed: %v", paerr)
	}
	paMap := make(map[string]*model.PowerStatusComponent)
	for ix := 0; ix < len(parr.Status); ix++ {
		t.Logf("Fetched power status element[%d]: '%v'", ix, parr.Status[ix])
		paMap[parr.Status[ix].XName] = &parr.Status[ix]
	}

	for ix := 0; ix <= maxIX; ix++ {
		xn := fmt.Sprintf("x%dc%ds%db%dn%d", ix, ix, ix, ix, ix)
		if paMap[xn].XName != xn {
			t.Errorf("GetAllPowerStatus() array mismatch, exp: '%s', got: '%s'",
				xn, paMap[xn].XName)
		}
	}

	//Some error tests

	pErrComp := pstat
	pErrComp.XName = "xyzzy"
	err = ms.StorePowerStatus(pErrComp)
	if err == nil {
		t.Errorf("StorePowerStatus() with bad XName should have failed, did not.")
	}
	err = ms.DeletePowerStatus(pErrComp.XName)
	if err == nil {
		t.Errorf("DeletePowerStatus() with bad XName should have failed, did not.")
	}
	_, err = ms.GetPowerStatus(pErrComp.XName)
	if err == nil {
		t.Errorf("GetPowerStatus() with bad XName should have failed, did not.")
	}

	//For distributed timed locks, the memory-based implementation does nothing
	//for locking, so all we can exercise is the function calls.

	lockDur := 10 * time.Second
	err = ds.DistributedTimedLock(lockDur)
	if err != nil {
		t.Errorf("DistributedTimedLock() failed: %v", err)
	}
	time.Sleep(1 * time.Second)
	if ds.GetDuration() != lockDur {
		t.Errorf("Lock duration readout failed, expecting %s, got %s",
			lockDur.String(), ds.GetDuration().String())
	}
	err = ds.Unlock()
	if err != nil {
		t.Errorf("Error releasing timed lock (outer): %v", err)
	}
	if ds.GetDuration() != 0 {
		t.Errorf("Lock duration readout failed, expecting 0s, got %s",
			ds.GetDuration().String())
	}
}
