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
	"context"
	"fmt"
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
	ctx context.Context
	ctxCancelFunc context.CancelFunc
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

func (d *ETCDLockProvider) DistributedTimedLock(maxLockTime time.Duration) (context.Context,error) {
	if (maxLockTime < 1) {
		return nil,fmt.Errorf("Error: lock duration request invalid (%s seconds) -- must be >= 1.",
					maxLockTime.String())
	}
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.ctx,d.ctxCancelFunc = context.WithTimeout(context.Background(),maxLockTime)
	err := d.kvHandle.DistTimedLock(int(maxLockTime.Seconds()))
	if (err != nil) {
		d.ctxCancelFunc()
		return nil,err
	}
	d.Duration = maxLockTime

	return d.ctx,nil
}

func (d *ETCDLockProvider) Unlock() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.Duration = 0
	d.kvHandle.DistUnlock()
	d.ctxCancelFunc()
	return nil
}

func (d *ETCDLockProvider) GetDuration() time.Duration {
	return d.Duration
}

