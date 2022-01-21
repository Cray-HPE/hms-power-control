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

//This file contains an in-memory implementation of a distributed locking
//mechanism.   NOTE: THIS MECHANISM DOESN'T REALLY DO ANY DIST'D LOCKING,
//since there is no IPC or actual backing store.  It is a wrapper around the
//ETCD locking mechanism, which contains an in-memory implementation.  This
//implementation is just to satisfy the dist'd lock interface.


type MEMLockProvider struct {
	Logger *logrus.Logger
	Duration time.Duration
	mutex *sync.Mutex
	ctx context.Context
	ctxCancelFunc context.CancelFunc
	kvHandle hmetcd.Kvi
}

func toStorageMEM(m *MEMLockProvider) *MEMStorage {
	return &MEMStorage{Logger: m.Logger, mutex: m.mutex, kvHandle: m.kvHandle}
}

func toDistLockETCD(m *MEMLockProvider) *ETCDLockProvider {
	return &ETCDLockProvider{Logger: m.Logger, Duration: m.Duration,
	                         mutex: m.mutex, ctx: m.ctx,
	                         ctxCancelFunc: m.ctxCancelFunc,
	                         kvHandle: m.kvHandle}
}


func (d *MEMLockProvider) Init(Logger *logrus.Logger) error {
	e := toStorageMEM(d)
	return e.Init(Logger)
}

func (d *MEMLockProvider) InitFromStorage(m interface{}, Logger *logrus.Logger) {
	ms := m.(*MEMStorage)
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

func (d *MEMLockProvider) Ping() error {
	e := toStorageMEM(d)
	return e.Ping()
}

func (d *MEMLockProvider) DistributedTimedLock(maxLockTime time.Duration) (context.Context,error) {
	if (maxLockTime < 1) {
		return nil,fmt.Errorf("Error: lock duration request invalid (%s seconds) -- must be >= 1.",
					maxLockTime.String())
	}
	d.Duration = maxLockTime
	e := toDistLockETCD(d)
	c,ce := e.DistributedTimedLock(maxLockTime)
	d.ctx = e.ctx
	d.ctxCancelFunc = e.ctxCancelFunc
	return c,ce
}

func (d *MEMLockProvider) Unlock() error {
	e := toDistLockETCD(d)
	err := e.Unlock()
	d.Duration = 0
	return err
}

func (d *MEMLockProvider) GetDuration() time.Duration {
	return d.Duration
}

