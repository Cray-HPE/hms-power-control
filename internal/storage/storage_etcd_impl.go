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

package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	hmetcd "github.com/Cray-HPE/hms-hmetcd"
	"github.com/Cray-HPE/hms-power-control/internal/model"
	"github.com/Cray-HPE/hms-xname/xnametypes"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// This file contains interface functions for the ETCD implementation of PCS
// storage.   It will also be used for the in-memory implementation, indirectly,
// since the HMS ETCD package already provides both ETCD and in-memory
// implementations.

const (
	kvUrlMemDefault         = "mem:"
	kvUrlDefault            = kvUrlMemDefault //Default to in-memory implementation
	kvRetriesDefault        = 5
	keyPrefix               = "/pcs/"
	keySegPowerStatusMaster = "/powerstatusmaster"
	keySegPowerState        = "/powerstate"
	keySegPowerCap          = "/powercaptask"
	keySegPowerCapOp        = "/powercapop"
	keySegTransition        = "/transition"
	keySegTransitionPage    = "/transitionpage"
	keySegTransitionTask    = "/transitiontask"
	keySegTransitionStat    = "/transitionstat"
	keyMin                  = " "
	keyMax                  = "~"
)

type ETCDStorage struct {
	Logger   *logrus.Logger
	mutex    *sync.Mutex
	kvHandle hmetcd.Kvi
}

func (e *ETCDStorage) fixUpKey(k string) string {
	key := k
	if !strings.HasPrefix(k, keyPrefix) {
		key = keyPrefix
		if strings.HasPrefix(k, "/") {
			key += k[1:]
		} else {
			key += k
		}
	}
	return key
}

////// ETCD /////

func (e *ETCDStorage) kvStore(key string, val interface{}) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	data, err := json.Marshal(val)
	if err == nil {
		realKey := e.fixUpKey(key)
		err = e.kvHandle.Store(realKey, string(data))
	}
	return err
}

func (e *ETCDStorage) kvGet(key string, val interface{}) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	realKey := e.fixUpKey(key)
	v, exists, err := e.kvHandle.Get(realKey)
	if exists {
		// We have a key, so val is valid.
		err = json.Unmarshal([]byte(v), &val)
	} else if err == nil {
		// No key and no error.  We will return this condition as an error
		err = fmt.Errorf("Key %s does not exist", key)
	}
	return err
}

func (e *ETCDStorage) GetTransitionPages(transitionId string) ([]model.TransitionPage, error) {
	// todotodo
	e.mutex.Lock()
	defer e.mutex.Unlock()
	var pages []model.TransitionPage
	keyPrefix := fmt.Sprintf("%s/%s", keySegTransitionPage, transitionId)
	key := e.fixUpKey(keyPrefix)
	kvList, err := e.kvHandle.GetRange(key+keyMin, key+keyMax)
	if err == nil {
		e.sortTransitionPages(kvList)
		for _, kv := range kvList {
			var page model.TransitionPage
			err = json.Unmarshal([]byte(kv.Value), &page)
			if err != nil {
				e.Logger.Error(err)
			} else {
				pages = append(pages, page)
			}
		}
	} else {
		e.Logger.Error(err)
	}
	return pages, err
}

func (e *ETCDStorage) sortTransitionPages(list []hmetcd.Kvi_KV) {
	sort.Slice(list, func(i, j int) bool {
		key0 := list[i].Key
		key0Sufix := key0[strings.LastIndex(key0, "/")+1:]
		key1 := list[j].Key
		key1Sufix := key1[strings.LastIndex(key1, "/")+1:]
		key0Int, err := strconv.Atoi(key0Sufix)
		if err != nil {
			e.Logger.Errorf("Expected last part to be an int in %s. %s", key0, err)
		}
		key1Int, err := strconv.Atoi(key1Sufix)
		if err != nil {
			e.Logger.Errorf("Expected last part to be an int in %s, %s", key0, err)
		}
		return key0Int < key1Int
	})
}

// if a key doesnt exist, etcd doesn't return an error
func (e *ETCDStorage) kvDelete(key string) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	realKey := e.fixUpKey(key)
	e.Logger.Trace("delete" + realKey)
	return e.kvHandle.Delete(e.fixUpKey(key))
}

// Do an atomic Test-And-Set operation
func (e *ETCDStorage) kvTAS(key string, testVal interface{}, setVal interface{}) (bool, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	tdata, err := json.Marshal(testVal)
	if err != nil {
		return false, err
	}
	sdata, err := json.Marshal(setVal)
	if err != nil {
		return false, err
	}
	realKey := e.fixUpKey(key)
	ok, err := e.kvHandle.TAS(realKey, string(tdata), string(sdata))
	return ok, err
}

func (e *ETCDStorage) Init(Logger *logrus.Logger) error {
	var kverr error

	if Logger == nil {
		e.Logger = logrus.New()
	} else {
		e.Logger = Logger
	}

	e.mutex = &sync.Mutex{}
	retries := kvRetriesDefault
	host, hostExists := os.LookupEnv("ETCD_HOST")
	if !hostExists {
		e.kvHandle = nil
		return fmt.Errorf("No ETCD HOST specified, can't open ETCD.")
	}
	port, portExists := os.LookupEnv("ETCD_PORT")
	if !portExists {
		e.kvHandle = nil
		return fmt.Errorf("No ETCD PORT specified, can't open ETCD.")
	}

	kvURL := fmt.Sprintf("http://%s:%s", host, port)
	e.Logger.Info(kvURL)

	etcOK := false
	for ix := 1; ix <= retries; ix++ {
		e.kvHandle, kverr = hmetcd.Open(kvURL, "")
		if kverr != nil {
			e.Logger.Error("ERROR opening connection to ETCD (attempt ", ix, "):", kverr)
		} else {
			etcOK = true
			e.Logger.Info("ETCD connection succeeded.")
			break
		}
	}
	if !etcOK {
		e.kvHandle = nil
		return fmt.Errorf("ETCD connection attempts exhausted, can't connect.")
	}
	return nil
}

func (e *ETCDStorage) Ping() error {
	e.Logger.Debug("ETCD PING")
	key := fmt.Sprintf("/ping/%s", uuid.New().String())
	err := e.kvStore(key, "")
	if err == nil {
		err = e.kvDelete(key)
	}
	return err
}

func (e *ETCDStorage) GetPowerStatusMaster() (time.Time, error) {
	var lastUpdated time.Time
	key := fmt.Sprintf("%s", keySegPowerStatusMaster)

	err := e.kvGet(key, &lastUpdated)
	if err != nil {
		e.Logger.Error(err)
	}
	return lastUpdated, err
}

func (e *ETCDStorage) StorePowerStatusMaster(now time.Time) error {
	key := fmt.Sprintf("%s", keySegPowerStatusMaster)
	err := e.kvStore(key, now)
	if err != nil {
		e.Logger.Error(err)
	}
	return err
}

func (e *ETCDStorage) TASPowerStatusMaster(now time.Time, testVal time.Time) (bool, error) {
	key := fmt.Sprintf("%s", keySegPowerStatusMaster)
	ok, err := e.kvTAS(key, testVal, now)
	if err != nil {
		e.Logger.Error(err)
	}
	return ok, err
}

func (e *ETCDStorage) StorePowerStatus(p model.PowerStatusComponent) error {
	if !(xnametypes.IsHMSCompIDValid(p.XName)) {
		return fmt.Errorf("Error parsing '%s': invalid xname format.", p.XName)
	}
	key := fmt.Sprintf("%s/%s", keySegPowerState, p.XName)
	err := e.kvStore(key, p)
	if err != nil {
		e.Logger.Error(err)
	}
	return err
}

func (e *ETCDStorage) DeletePowerStatus(xname string) error {
	if !(xnametypes.IsHMSCompIDValid(xname)) {
		return fmt.Errorf("Error parsing '%s': invalid xname format.", xname)
	}
	key := fmt.Sprintf("%s/%s", keySegPowerState, xname)
	err := e.kvDelete(key)
	if err != nil {
		e.Logger.Error(err)
	}
	return err
}

func (e *ETCDStorage) GetPowerStatus(xname string) (model.PowerStatusComponent, error) {
	var pcomp model.PowerStatusComponent
	if !(xnametypes.IsHMSCompIDValid(xname)) {
		return pcomp, fmt.Errorf("Error parsing '%s': invalid xname format.", xname)
	}
	key := fmt.Sprintf("%s/%s", keySegPowerState, xname)

	err := e.kvGet(key, &pcomp)
	if err != nil {
		e.Logger.Error(err)
	}
	return pcomp, err
}

func (e *ETCDStorage) GetAllPowerStatus() (model.PowerStatus, error) {
	var pstats model.PowerStatus
	k := e.fixUpKey(keySegPowerState)
	kvl, err := e.kvHandle.GetRange(k+keyMin, k+keyMax)
	if err == nil {
		for _, kv := range kvl {
			var pcomp model.PowerStatusComponent
			err = json.Unmarshal([]byte(kv.Value), &pcomp)
			if err != nil {
				e.Logger.Error(err)
			} else {
				pstats.Status = append(pstats.Status, pcomp)
			}
		}
	} else {
		e.Logger.Error(err)
	}
	return pstats, err
}

func (e *ETCDStorage) GetPowerStatusHierarchy(xname string) (model.PowerStatus, error) {
	var pstats model.PowerStatus
	if !(xnametypes.IsHMSCompIDValid(xname)) {
		return pstats, fmt.Errorf("Error parsing '%s': invalid xname format.", xname)
	}
	key := fmt.Sprintf("%s/%s", keySegPowerState, xname)
	k := e.fixUpKey(key)
	kvl, err := e.kvHandle.GetRange(k, k+keyMax)
	if err == nil {
		for _, kv := range kvl {
			var pcomp model.PowerStatusComponent
			err = json.Unmarshal([]byte(kv.Value), &pcomp)
			if err != nil {
				e.Logger.Error(err)
			} else {
				pstats.Status = append(pstats.Status, pcomp)
			}
		}
	} else {
		e.Logger.Error(err)
	}
	return pstats, err
}

///////////////////////
// Power Capping
///////////////////////

func (e *ETCDStorage) StorePowerCapTask(task model.PowerCapTask) error {
	key := fmt.Sprintf("%s/%s", keySegPowerCap, task.TaskID.String())
	err := e.kvStore(key, task)
	if err != nil {
		e.Logger.Error(err)
	}
	return err
}

func (e *ETCDStorage) StorePowerCapOperation(op model.PowerCapOperation) error {
	// Store PowerCapOperations using their parent task's key so it will be
	// easier to get all of them when needed.
	key := fmt.Sprintf("%s/%s/%s", keySegPowerCapOp, op.TaskID.String(), op.OperationID.String())
	err := e.kvStore(key, op)
	if err != nil {
		e.Logger.Error(err)
	}
	return err
}

func (e *ETCDStorage) GetPowerCapTask(taskID uuid.UUID) (model.PowerCapTask, error) {
	var task model.PowerCapTask
	key := fmt.Sprintf("%s/%s", keySegPowerCap, taskID.String())

	err := e.kvGet(key, &task)
	if err != nil {
		e.Logger.Error(err)
	}
	return task, err
}

func (e *ETCDStorage) GetPowerCapOperation(taskID, opID uuid.UUID) (model.PowerCapOperation, error) {
	var op model.PowerCapOperation
	key := fmt.Sprintf("%s/%s/%s", keySegPowerCapOp, taskID.String(), opID.String())

	err := e.kvGet(key, &op)
	if err != nil {
		e.Logger.Error(err)
	}
	return op, err
}

func (e *ETCDStorage) GetAllPowerCapOperationsForTask(taskID uuid.UUID) ([]model.PowerCapOperation, error) {
	ops := []model.PowerCapOperation{}
	key := fmt.Sprintf("%s/%s", keySegPowerCapOp, taskID.String())
	k := e.fixUpKey(key)
	kvl, err := e.kvHandle.GetRange(k+keyMin, k+keyMax)
	if err == nil {
		for _, kv := range kvl {
			var op model.PowerCapOperation
			err = json.Unmarshal([]byte(kv.Value), &op)
			if err != nil {
				e.Logger.Error(err)
			} else {
				ops = append(ops, op)
			}
		}
	} else {
		e.Logger.Error(err)
	}
	return ops, err
}

func (e *ETCDStorage) GetAllPowerCapTasks() ([]model.PowerCapTask, error) {
	tasks := []model.PowerCapTask{}
	k := e.fixUpKey(keySegPowerCap)
	kvl, err := e.kvHandle.GetRange(k+keyMin, k+keyMax)
	if err == nil {
		for _, kv := range kvl {
			var task model.PowerCapTask
			err = json.Unmarshal([]byte(kv.Value), &task)
			if err != nil {
				e.Logger.Error(err)
			} else {
				tasks = append(tasks, task)
			}
		}
	} else {
		e.Logger.Error(err)
	}
	return tasks, err
}

func (e *ETCDStorage) DeletePowerCapTask(taskID uuid.UUID) error {
	key := fmt.Sprintf("%s/%s", keySegPowerCap, taskID.String())
	err := e.kvDelete(key)
	if err != nil {
		e.Logger.Error(err)
	}
	return err
}

func (e *ETCDStorage) DeletePowerCapOperation(taskID uuid.UUID, opID uuid.UUID) error {
	key := fmt.Sprintf("%s/%s/%s", keySegPowerCapOp, taskID.String(), opID.String())
	err := e.kvDelete(key)
	if err != nil {
		e.Logger.Error(err)
	}
	return err
}

///////////////////////
// Transitions
///////////////////////

type TransitionWatchCBFunc func(Transition model.Transition, wasDeleted bool, err error, userdata interface{}) bool

type WatchTransitionCBHandle struct {
	watchHandlePut    hmetcd.WatchCBHandle
	watchHandleDelete hmetcd.WatchCBHandle
}

func (e *ETCDStorage) StoreTransition(transition model.Transition) error {
	t, tPages := e.breakIntoPagesIfNeeded(transition)
	e.Logger.Infof("TRACE: task count: %d", len(t.Tasks))
	e.Logger.Infof("TRACE: location count: %d", len(t.Location))
	e.Logger.Infof("TRACE: page count: %d", len(tPages))

	key := fmt.Sprintf("%s/%s", keySegTransition, t.TransitionID.String())
	err := e.kvStore(key, t)
	if err != nil {
		e.Logger.Error(err)
	}

	for _, page := range tPages {

		e.Logger.Infof("TRACE: page %d count: %d", page.Index, len(page.Tasks))
		// Task pages
		key = fmt.Sprintf("%s/%s/%d", keySegTransitionPage, page.TransitionID.String(), page.Index)
		err = e.kvStore(key, page)
		if err != nil {
			e.Logger.Error(err)
		}
	}
	return err
}

func (e *ETCDStorage) breakIntoPagesIfNeeded(transition model.Transition) (model.Transition, []*model.TransitionPage) {
	// chunkSize := 1500
	chunkSize := 500
	e.Logger.Infof("TRACE: chunkSize: %d", chunkSize)
	if len(transition.Tasks) > chunkSize || len(transition.Location) > chunkSize {
		taskPages := e.pageTasks(transition, chunkSize)
		locationPages := e.pageLocations(transition, chunkSize)
		e.Logger.Infof("TRACE: tasks page count: %d", len(taskPages))
		e.Logger.Infof("TRACE: location page count: %d", len(locationPages))

		// pick the largest page count
		pageCount := 0
		if len(taskPages) > pageCount {
			pageCount = len(taskPages)
		}
		if len(locationPages) > pageCount {
			pageCount = len(locationPages)
		}

		// build pages
		var pages []*model.TransitionPage
		for i := 0; i < pageCount; i++ {
			index := i - 1
			id := fmt.Sprintf("%s_%d", transition.TransitionID.String(), index)
			page := model.TransitionPage{
				ID:           id,
				TransitionID: transition.TransitionID,
				Index:        index,
			}

			pages = append(pages, &page)
		}

		// fill in tasks on each page
		for i := 1; i < len(taskPages); i++ {
			pages[i-1].Tasks = taskPages[i]
		}

		// fill in locations on each page
		for i := 1; i < len(locationPages); i++ {
			pages[i-1].Location = locationPages[i]
		}
		return transition, pages
	} else {
		return transition, nil
	}
}

func (e *ETCDStorage) pageTasks(transition model.Transition, chunkSize int) [][]model.TransitionTaskResp {
	var pages [][]model.TransitionTaskResp
	if len(transition.Tasks) > chunkSize {
		for i := 0; i < len(transition.Tasks); i += chunkSize {
			end := i + chunkSize
			if end > len(transition.Tasks) {
				end = len(transition.Tasks)
			}
			pages = append(pages, transition.Tasks[i:end])
		}
	} else {
		pages = append(pages, transition.Tasks)
	}
	return pages
}

func (e *ETCDStorage) pageLocations(transition model.Transition, chunkSize int) [][]model.LocationParameter {
	var pages [][]model.LocationParameter
	if len(transition.Location) > chunkSize {
		for i := 0; i < len(transition.Location); i += chunkSize {
			end := i + chunkSize
			if end > len(transition.Location) {
				end = len(transition.Location)
			}
			pages = append(pages, transition.Location[i:end])
		}
	} else {
		pages = append(pages, transition.Location)
	}
	return pages
}

func (e *ETCDStorage) StoreTransitionTask(task model.TransitionTask) error {
	// Store TransitionTasks using their parent transition's key so it will be
	// easier to get all of them when needed.
	key := fmt.Sprintf("%s/%s/%s", keySegTransitionTask, task.TransitionID.String(), task.TaskID.String())
	err := e.kvStore(key, task)
	if err != nil {
		e.Logger.Error(err)
	}
	return err
}

func (e *ETCDStorage) GetTransition(transitionID uuid.UUID) (model.Transition, error) {
	var transition model.Transition
	key := fmt.Sprintf("%s/%s", keySegTransition, transitionID.String())

	err := e.kvGet(key, &transition)
	if err != nil {
		e.Logger.Error(err)
		return transition, err
	}

	pages, err := e.GetTransitionPages(transition.TransitionID.String())
	for _, page := range pages {
		transition.Tasks = append(transition.Tasks, page.Tasks...)
	}

	return transition, err
}

func (e *ETCDStorage) GetTransitionTask(transitionID, taskID uuid.UUID) (model.TransitionTask, error) {
	var task model.TransitionTask
	key := fmt.Sprintf("%s/%s/%s", keySegTransitionTask, transitionID.String(), taskID.String())

	err := e.kvGet(key, &task)
	if err != nil {
		e.Logger.Error(err)
	}
	return task, err
}

func (e *ETCDStorage) GetAllTasksForTransition(transitionID uuid.UUID) ([]model.TransitionTask, error) {
	tasks := []model.TransitionTask{}
	key := fmt.Sprintf("%s/%s", keySegTransitionTask, transitionID.String())
	k := e.fixUpKey(key)
	kvl, err := e.kvHandle.GetRange(k+keyMin, k+keyMax)
	if err == nil {
		for _, kv := range kvl {
			var task model.TransitionTask
			err = json.Unmarshal([]byte(kv.Value), &task)
			if err != nil {
				e.Logger.Error(err)
			} else {
				tasks = append(tasks, task)
			}
		}
	} else {
		e.Logger.Error(err)
	}
	return tasks, err
}

func (e *ETCDStorage) GetAllTransitions() ([]model.Transition, error) {
	transitions := []model.Transition{}
	key := fmt.Sprintf("%s/", keySegTransition)
	k := e.fixUpKey(key)
	kvl, err := e.kvHandle.GetRange(k+keyMin, k+keyMax)
	if err == nil {
		for _, kv := range kvl {
			var transition model.Transition
			err = json.Unmarshal([]byte(kv.Value), &transition)
			if err != nil {
				e.Logger.Error(err)
			} else {
				transitions = append(transitions, transition)
			}
		}
	} else {
		e.Logger.Error(err)
	}
	return transitions, err
}

func (e *ETCDStorage) DeleteTransition(transitionID uuid.UUID) error {
	key := fmt.Sprintf("%s/%s", keySegTransition, transitionID.String())
	var combinedErr error
	err := e.kvDelete(key)
	if err != nil {
		e.Logger.Error(err)
		combinedErr = wrapError(combinedErr, err)
	}
	pages, err := e.GetTransitionPages(transitionID.String())
	if err != nil {
		e.Logger.Error(err)
		combinedErr = wrapError(combinedErr, err)
	}
	for _, page := range pages {
		key = fmt.Sprintf("%s/%s/%d", keySegTransitionPage, transitionID.String(), page.Index)
		err = e.kvDelete(key)
		if err != nil {
			e.Logger.Error(err)
			combinedErr = wrapError(combinedErr, err)
		}
	}
	return combinedErr
}

func (e *ETCDStorage) DeleteTransitionTask(transitionID uuid.UUID, taskID uuid.UUID) error {
	key := fmt.Sprintf("%s/%s/%s", keySegTransitionTask, transitionID.String(), taskID.String())
	err := e.kvDelete(key)
	if err != nil {
		e.Logger.Error(err)
	}
	return err
}

func (e *ETCDStorage) TASTransition(transition model.Transition, testVal model.Transition) (bool, error) {
	key := fmt.Sprintf("%s/%s", keySegTransition, transition.TransitionID.String())
	ok, err := e.kvTAS(key, testVal, transition)
	if err != nil {
		e.Logger.Error(err)
	}
	return ok, err
}

func wrapError(err0 error, err1 error) error {
	if err0 != nil && err1 != nil {
		return fmt.Errorf("%s; %w", err0, err1)
	} else if err0 == nil && err1 != nil {
		return err1
	} else if err0 != nil && err1 == nil {
		return err0
	} else {
		// err0 == nil && err1 == nil
		return nil
	}
}
