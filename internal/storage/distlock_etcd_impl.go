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
	"github.com/sirupsen/logrus"
	hmetcd "github.com/Cray-HPE/hms-hmetcd"
)

//This file contains an ETCD-based implementation of a distributed timed
//locking mechanism, which can be used to synchronize activities amongst
//multiple instances of a microservice.

type ETCDLockProvider struct {
	Logger *logrus.Logger
	Duration int
	mutex *sync.Mutex
	kvHandle hmetcd.Kvi
}

func toStorageETCD(m *ETCDLockProvider) *ETCDStorage {
	return &ETCDStorage{Logger: m.Logger, mutex: m.mutex, kvHandle: m.kvHandle}
}

func fromStorageETCD(m *ETCDStorage) *ETCDLockProvider {
	return &ETCDLockProvider{Logger: m.Logger, mutex: m.mutex, kvHandle: m.kvHandle}
}

func (d *ETCDLockProvider) Init(Logger *logrus.Logger) error {
	e := toStorageETCD(d)
	return e.Init(Logger)
}

func (d *ETCDLockProvider) InitFromStorage(m interface{}, Logger *logrus.Logger) {
	ms := m.(*ETCDStorage)
	d.Logger = ms.Logger
	d.mutex = ms.mutex
	d.kvHandle = ms.kvHandle
	if (Logger == nil) {
		d.Logger = ms.Logger
	} else {
		d.Logger = Logger
	}
	if (d.Logger == nil) {
		d.Logger = logrus.New()
	}
}

func (d *ETCDLockProvider) Ping() error {
	e := toStorageETCD(d)
	return e.Ping()
}

func (d *ETCDLockProvider) DistributedTimedLock(maxSeconds int) error {
	if (maxSeconds < 1) {
		return fmt.Errorf("Error: lock duration request invalid (%d seconds) -- must be >= 1.",
					maxSeconds)
	}
	d.mutex.Lock()
	defer d.mutex.Unlock()
	err := d.kvHandle.DistTimedLock(maxSeconds)
	if (err != nil) {
		return err
	}
	d.Duration = maxSeconds

	return nil
}

func (d *ETCDLockProvider) Unlock() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.Duration = 0
	return d.kvHandle.DistUnlock()
}

func (d *ETCDLockProvider) GetDuration() int {
	return d.Duration
}

