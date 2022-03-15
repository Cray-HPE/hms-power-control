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
	"os"
	"sync"
	"time"
	"github.com/sirupsen/logrus"
	hmetcd "github.com/Cray-HPE/hms-hmetcd"
)

//This file contains an ETCD-based implementation of a distributed timed
//locking mechanism, which can be used to synchronize activities amongst
//multiple instances of a microservice.

type ETCDLockProvider struct {
	Logger *logrus.Logger
	Duration time.Duration
	mutex *sync.Mutex
	kvHandle hmetcd.Kvi
}

func toStorageETCD(m *ETCDLockProvider) *ETCDStorage {
	return &ETCDStorage{Logger: m.Logger, mutex: m.mutex, kvHandle: m.kvHandle}
}

func fromStorageETCD(m *ETCDStorage) *ETCDLockProvider {
	return &ETCDLockProvider{Logger: m.Logger, mutex: m.mutex, kvHandle: m.kvHandle}
}

func (e *ETCDLockProvider) Init(Logger *logrus.Logger) error {
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

func (d *ETCDLockProvider) DistributedTimedLock(maxLockTime time.Duration) error {
	if (maxLockTime < 1) {
		return fmt.Errorf("Error: lock duration request invalid (%s seconds) -- must be >= 1.",
					maxLockTime.String())
	}
	d.mutex.Lock()
	defer d.mutex.Unlock()
	err := d.kvHandle.DistTimedLock(int(maxLockTime.Seconds()))
	if (err != nil) {
		return err
	}
	d.Duration = maxLockTime

	return nil
}

func (d *ETCDLockProvider) Unlock() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.Duration = 0
	return d.kvHandle.DistUnlock()
}

func (d *ETCDLockProvider) GetDuration() time.Duration {
	return d.Duration
}

