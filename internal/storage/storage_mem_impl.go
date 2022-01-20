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

	if (Logger == nil) {
		m.Logger = logrus.New()
	} else {
		m.Logger = Logger
	}

	m.mutex = &sync.Mutex{}
	m.Logger.Infof("Storage medium: memory, url: '%s'",kvUrlMemDefault)

	m.kvHandle, kverr = hmetcd.Open(kvUrlMemDefault, "")
	if kverr != nil {
		m.kvHandle = nil
		return fmt.Errorf("ERROR opening KV memory storage: %v",kverr)
	}
	m.Logger.Info("KV memory setup succeeded.")
	return nil
}

func (m *MEMStorage) Ping() error {
	e := toETCDStorage(m)
	return e.Ping()
}

func (m *MEMStorage) StorePowerStatus(p PowerStatusComponent) error {
	e := toETCDStorage(m)
	return e.StorePowerStatus(p)
}

func (m *MEMStorage) DeletePowerStatus(xname string)error {
	e := toETCDStorage(m)
	return e.DeletePowerStatus(xname)
}

func (m *MEMStorage) GetPowerStatus(xname string) (PowerStatusComponent, error) {
	e := toETCDStorage(m)
	return e.GetPowerStatus(xname)
}

func (m *MEMStorage) GetAllPowerStatus() (PowerStatus, error) {
	e := toETCDStorage(m)
	return e.GetAllPowerStatus()
}

