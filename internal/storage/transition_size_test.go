// MIT License
//
// (C) Copyright [2024-2025] Hewlett Packard Enterprise Development LP
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
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Cray-HPE/hms-power-control/internal/model"
)

type EtcdTestSettings struct {
	DisableSizeChecks bool
	PageSize          int
	MaxMessageLen     int
	MaxEtcdObjectSize int
}

var DEFAULT_TEST_SETTINGS *EtcdTestSettings // nil means default settings

func createProviders(t *testing.T, settings *EtcdTestSettings) (StorageProvider, DistributedLockProvider, bool) {
	var ms StorageProvider
	var ds DistributedLockProvider
	isInMemoryProvider := true

	if (os.Getenv("ETCD_HOST") != "") && (os.Getenv("ETCD_PORT") != "") {
		t.Logf("Using ETCD backing store.")
		ms = &ETCDStorage{}
		ds = &ETCDLockProvider{}
		isInMemoryProvider = false
	} else {
		t.Logf("Using In-Memory backing store.")
		ms = &MEMStorage{}
		ds = &MEMLockProvider{}
	}

	err := ms.Init(nil)
	if err != nil {
		t.Errorf("Storage Init() failed: %v", err)
	}

	ds.InitFromStorage(ms, nil)
	if err != nil {
		t.Errorf("DistLock InitFromStorage() failed: %v", err)
	}

	if e, ok := ms.(*ETCDStorage); ok {
		if settings != nil {
			e.PageSize = settings.PageSize
			e.MaxMessageLen = settings.MaxMessageLen
			e.MaxEtcdObjectSize = settings.MaxEtcdObjectSize
		}
		t.Logf("ETCD Storage test settings, page_size: %d, message_size: %d, object_size: %d", e.PageSize, e.MaxMessageLen, e.MaxEtcdObjectSize)
	}
	return ms, ds, isInMemoryProvider
}

func TestWriteTransitions(t *testing.T) {
	ms, _, _ := createProviders(t, DEFAULT_TEST_SETTINGS)

	params := newParameters("x7000c0s0b0", 1, "on")
	transition := newTransition(params)
	transition = addTasks(transition, 10, 0)

	ms.StoreTransition(transition)

	storedTransition, _, err := ms.GetTransition(transition.TransitionID)
	if err != nil {
		t.Errorf("Failed to read stored transtion. TransitionID: %s, Error: %s", transition.TransitionID, err)
	}

	if storedTransition.TransitionID != transition.TransitionID {
		t.Errorf("TestWriteTransitions: stored transition wrong id, expected: %s, actaual: %s", transition.TransitionID, storedTransition.TransitionID)
		storedLen := len(storedTransition.Tasks)
		originalLen := len(transition.Tasks)
		if storedLen != originalLen {
			t.Errorf("TestWriteTransitions: stored transition had wrong number of tasks, expected: %d, actaual: %d", originalLen, storedLen)
		}
	}
}

func TestMaxTransitionFailure(t *testing.T) {
	// max object size is 2097152 (2 x 1024 x 1024 bytes)
	settings := &EtcdTestSettings{
		DisableSizeChecks: false,
		PageSize:          100000,
		MaxMessageLen:     1000,
		MaxEtcdObjectSize: 4000000,
	}
	ms, _, isInMemoryProvider := createProviders(t, settings)

	// skip the test if using the memory based prover.
	// The memory based provider does not have the size limits that the etcd provider has.
	if isInMemoryProvider {
		t.SkipNow()
	}

	params := newParameters("x6000c0s10b0", 10000, "on")
	transition := newTransition(params)
	transition = addTasks(transition, 150, 150)

	size, err := getSize(transition)
	if err != nil {
		t.Errorf("Unexpected error marshalling transition to json. TransitionID: %s, Error: %s", transition.TransitionID, err)
	}
	t.Logf("TestMaxTransitionFailure: TransitionID: %s, size: %d", transition.TransitionID, size)

	err = ms.StoreTransition(transition)

	if err == nil {
		t.Errorf("Expected an error when writing a large transtion. TransitionID: %s, size: %d", transition.TransitionID, size)
	}
}

func TestMaxTransition(t *testing.T) {
	// max object size is 2097152 (2 x 1024 x 1024 bytes)
	ms, _, _ := createProviders(t, DEFAULT_TEST_SETTINGS)

	xnameCount := 6000
	messageLen := 150
	params := newParameters("x6000c0s10b0", xnameCount, "on")
	transition := newTransition(params)
	transition = addTasks(transition, messageLen, messageLen)

	size, err := getSize(transition)
	if err != nil {
		t.Errorf("Unexpected error marshalling transition to json. TransitionID: %s, Error: %s", transition.TransitionID, err)
	}
	t.Logf("TestMaxTransition: TransitionID: %s, xnameCount: %d, messageLen: %d, object size: %d",
		transition.TransitionID, xnameCount, messageLen, size)

	err = ms.StoreTransition(transition)
	if err != nil {
		t.Errorf("Failed to write large transtion. TransitionID: %s, size: %d, Error: %s", transition.TransitionID, size, err)
	}

	storedTransition, _, err := ms.GetTransition(transition.TransitionID)
	if err != nil {
		t.Errorf("Failed to read large transtion. TransitionID: %s, size: %d, Error: %s", transition.TransitionID, size, err)
	}
	storedSize, _ := getSize(storedTransition)
	t.Logf("TestMaxTransition: TransitionID: %s, storedSize: %d", transition.TransitionID, storedSize)
}

func TestTransitionSizes(t *testing.T) {
	xnameCount := 6000
	messageLen := 150
	errorLen := 150
	params := newParameters("x6000c0s10b0", xnameCount, "on")
	transition := newTransition(params)
	transition = addTasks(transition, messageLen, errorLen)

	size, err := getSize(transition)
	if err != nil {
		t.Errorf("Unexpected error marshalling transition to json. TransitionID: %s, Error: %s", transition.TransitionID, err)
	}
	t.Logf("TestTransitionSizes: TransitionID: %s, xnameCount: %d, messageLen: %d, errorMessageLen: %d, object size: %d",
		transition.TransitionID, xnameCount, messageLen, errorLen, size)

	if size < 1570000 {
		t.Errorf("Unexpected transtion size: %d. Expected size to be over 1570000 for 6000 nodes, with 150 long messages", size)
	}

	messageLen = 100
	errorLen = 150
	transitionSmall := newTransition(params)
	transition = addTasks(transition, messageLen, errorLen)

	size, err = getSize(transitionSmall)
	if err != nil {
		t.Errorf("Unexpected error marshalling transition to json. TransitionID: %s, Error: %s", transitionSmall.TransitionID, err)
	}
	t.Logf("TestTransitionSizes: TransitionID: %s, xnameCount: %d, messageLen: %d, errorMessageLen: %d, object size: %d",
		transition.TransitionID, xnameCount, messageLen, errorLen, size)

	maxObjectSize := 1570000
	if size > maxObjectSize {
		t.Errorf("Unexpected transtion size: %d. Expected size to be over %d. node count: %d, message len %d, error len: %d",
			size, maxObjectSize, xnameCount, messageLen, errorLen)
	}
}

func getSize(transition model.Transition) (int, error) {
	sdata, err := json.Marshal(transition)
	if err != nil {
		return -1, err
	}
	size := len(sdata)
	return size, nil
}

func newTransition(parameters model.TransitionParameter) model.Transition {
	transition, _ := model.ToTransition(parameters, 2000)
	return transition
}

func newParameters(xnamePrefix string, count int, operation string) model.TransitionParameter {
	params := model.TransitionParameter{
		Operation: operation,
	}
	for i := 0; i < count; i++ {
		params.Location = append(params.Location,
			model.LocationParameter{
				Xname: fmt.Sprintf("%sn%d", xnamePrefix, i),
			})

	}
	return params
}

func addTasks(transition model.Transition, messageLen, errMessageLen int) model.Transition {
	for _, location := range transition.Location {
		task := model.TransitionTaskResp{
			Xname:          location.Xname,
			TaskStatus:     model.TransitionStatusInProgress,
			TaskStatusDesc: maxString("message", strOfLen(messageLen)),
			Error:          maxString("error message", strOfLen(errMessageLen)),
		}
		transition.Tasks = append(transition.Tasks, task)
	}
	return transition
}

func maxString(str string, maxStr string) string {
	result := str
	length := len(str)
	maxLength := len(maxStr)
	if length < maxLength {
		result = str + " " + maxStr[length+1:]
	} else if length > maxLength {
		result = str[:maxLength]
	}
	return result
}

func TestMaxString(t *testing.T) {
	n := "TestMaxString"

	m := maxString("junk", strOfLen(1000))
	assertEqualInt(t, n+"1", "Wrong length", 1000, len(m))

	m = maxString("junk", strOfLen(0))
	assertEqualInt(t, n+"2", fmt.Sprintf("Wrong length: '%s'", m), 0, len(m))

	m = maxString("junk", strOfLen(4))
	assertEqualInt(t, n+"3", fmt.Sprintf("Wrong length: '%s'", m), 4, len(m))

	m = maxString("junk", strOfLen(5))
	assertEqualInt(t, n+"4", fmt.Sprintf("Wrong length: '%s'", m), 5, len(m))

	m = maxString("junk", strOfLen(6))
	assertEqualInt(t, n+"5", fmt.Sprintf("Wrong length: '%s'", m), 6, len(m))
}

func strOfLen(length int) string {
	l := length/10 + 1
	str := strings.Repeat("0123456789 ", l)
	return str[:length]
}

func TestStrOfLen(t *testing.T) {
	s := strOfLen(1000)
	t.Logf("Long string: %s", s)
	if len(s) != 1000 {
		t.Errorf("Wrong length: %s", s)
	}
}

func assertEqualInt(t *testing.T, funcName, message string, expected, actual int) {
	if expected != actual {
		t.Errorf("%s: %s, expected: %d, actaual: %d", funcName, message, expected, actual)
	}
}
