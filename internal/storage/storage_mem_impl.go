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
	"sync"

	hmetcd "github.com/Cray-HPE/hms-hmetcd"
	"github.com/Cray-HPE/hms-power-control/internal/model"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

//This file contains the in-memory implementation of our object storage.
//Note that is really is just a wrapper around the ETCD implementation, which
//contains an in-memory implementation already.

type MEMStorage struct {
	Logger   *logrus.Logger
	mutex    *sync.Mutex
	kvHandle hmetcd.Kvi
}

func toETCDStorage(m *MEMStorage) *ETCDStorage {
	return &ETCDStorage{Logger: m.Logger, mutex: m.mutex, kvHandle: m.kvHandle}
}

func (m *MEMStorage) Init(Logger *logrus.Logger) error {
	var kverr error

	if Logger == nil {
		m.Logger = logrus.New()
	} else {
		m.Logger = Logger
	}

	m.mutex = &sync.Mutex{}
	m.Logger.Infof("Storage medium: memory, url: '%s'", kvUrlMemDefault)

	m.kvHandle, kverr = hmetcd.Open(kvUrlMemDefault, "")
	if kverr != nil {
		m.kvHandle = nil
		return fmt.Errorf("ERROR opening KV memory storage: %v", kverr)
	}
	m.Logger.Info("KV memory setup succeeded.")
	return nil
}

func (m *MEMStorage) Ping() error {
	e := toETCDStorage(m)
	return e.Ping()
}

func (m *MEMStorage) StorePowerStatus(p model.PowerStatusComponent) error {
	e := toETCDStorage(m)
	return e.StorePowerStatus(p)
}

func (m *MEMStorage) DeletePowerStatus(xname string) error {
	e := toETCDStorage(m)
	return e.DeletePowerStatus(xname)
}

func (m *MEMStorage) GetPowerStatus(xname string) (model.PowerStatusComponent, error) {
	e := toETCDStorage(m)
	return e.GetPowerStatus(xname)
}

func (m *MEMStorage) GetAllPowerStatus() (model.PowerStatus, error) {
	e := toETCDStorage(m)
	return e.GetAllPowerStatus()
}

func (m *MEMStorage) GetPowerStatusHierarchy(xname string) (model.PowerStatus, error) {
	e := toETCDStorage(m)
	return e.GetPowerStatusHierarchy(xname)
}

///////////////////////
// Power Capping
///////////////////////

func (m *MEMStorage) StorePowerCapTask(task model.PowerCapTask) error {
	e := toETCDStorage(m)
	return e.StorePowerCapTask(task)
}

func (m *MEMStorage) StorePowerCapOperation(op model.PowerCapOperation) error {
	e := toETCDStorage(m)
	return e.StorePowerCapOperation(op)
}

func (m *MEMStorage) GetPowerCapTask(taskID uuid.UUID) (model.PowerCapTask, error) {
	e := toETCDStorage(m)
	return e.GetPowerCapTask(taskID)
}

func (m *MEMStorage) GetPowerCapOperation(taskID uuid.UUID, opID uuid.UUID) (model.PowerCapOperation, error) {
	e := toETCDStorage(m)
	return e.GetPowerCapOperation(taskID, opID)
}

func (m *MEMStorage) GetAllPowerCapOperationsForTask(taskID uuid.UUID) ([]model.PowerCapOperation, error) {
	e := toETCDStorage(m)
	return e.GetAllPowerCapOperationsForTask(taskID)
}

func (m *MEMStorage) GetAllPowerCapTasks() ([]model.PowerCapTask, error) {
	e := toETCDStorage(m)
	return e.GetAllPowerCapTasks()
}

func (m *MEMStorage) DeletePowerCapTask(taskID uuid.UUID) error {
	e := toETCDStorage(m)
	return e.DeletePowerCapTask(taskID)
}

func (m *MEMStorage) DeletePowerCapOperation(taskID uuid.UUID, opID uuid.UUID) error {
	e := toETCDStorage(m)
	return e.DeletePowerCapOperation(taskID, opID)
}

///////////////////////
// Transition
///////////////////////

func (m *MEMStorage) StoreTransition(transition model.Transition) error {
	e := toETCDStorage(m)
	return e.StoreTransition(transition)
}

func (m *MEMStorage) StoreTransitionTask(op model.TransitionTask) error {
	e := toETCDStorage(m)
	return e.StoreTransitionTask(op)
}

func (m *MEMStorage) GetTransition(transitionID uuid.UUID) (model.Transition, error) {
	e := toETCDStorage(m)
	return e.GetTransition(transitionID)
}

func (m *MEMStorage) GetTransitionTask(transitionID uuid.UUID, taskID uuid.UUID) (model.TransitionTask, error) {
	e := toETCDStorage(m)
	return e.GetTransitionTask(transitionID, taskID)
}

func (m *MEMStorage) GetAllTasksForTransition(transitionID uuid.UUID) ([]model.TransitionTask, error) {
	e := toETCDStorage(m)
	return e.GetAllTasksForTransition(transitionID)
}

func (m *MEMStorage) GetAllTransitions() ([]model.Transition, error) {
	e := toETCDStorage(m)
	return e.GetAllTransitions()
}

func (m *MEMStorage) DeleteTransition(transitionID uuid.UUID) error {
	e := toETCDStorage(m)
	return e.DeleteTransition(transitionID)
}

func (m *MEMStorage) DeleteTransitionTask(transitionID uuid.UUID, taskID uuid.UUID) error {
	e := toETCDStorage(m)
	return e.DeleteTransitionTask(transitionID, taskID)
}

func (m *MEMStorage) WatchTransitionCB(transitionID uuid.UUID, cb TransitionWatchCBFunc, userdata interface{}) (WatchTransitionCBHandle, error) {
	e := toETCDStorage(m)
	return e.WatchTransitionCB(transitionID, cb, userdata)
}

func (m *MEMStorage) WatchTransitionCBCancel(cbh WatchTransitionCBHandle) {
	e := toETCDStorage(m)
	e.WatchTransitionCBCancel(cbh)
}

func (m *MEMStorage) TASTransition(transition model.Transition, testVal model.Transition) (bool, error) {
	e := toETCDStorage(m)
	return e.TASTransition(transition, testVal)
}