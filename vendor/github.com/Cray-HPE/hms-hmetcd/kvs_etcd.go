// MIT License
//
// (C) Copyright [2021,2025] Hewlett Packard Enterprise Development LP
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

// Package hmetcd provides an interface to ETCD along with an
// alternate implementation using memory/map storage.
package hmetcd

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

// Distributed lock stuff

type KvcLockResult int

const (
	KVC_LOCK_RSLT_INVALID KvcLockResult = 0
	KVC_LOCK_SET_FAIL     KvcLockResult = 1
	KVC_LOCK_OCCUPIED     KvcLockResult = 2
	KVC_LOCK_GRANTED      KvcLockResult = 3
	KVC_LOCK_RELEASE_FAIL KvcLockResult = 4
	KVC_LOCK_RELEASED     KvcLockResult = 5
)

// Key/value change types, used for watches

const (
	KVC_KEYCHANGE_INVAL  = 0
	KVC_KEYCHANGE_PUT    = 1
	KVC_KEYCHANGE_DELETE = 2
)

const (
	DIST_LOCK_KEY = "hbtd_dist_lock"
)

type WatchCBFunc func(key string, val string, op int, userdata interface{}) bool

// Interface data type for actual ETC-based interface methods

type Kvs_etcd struct {
	client     *clientv3.Client
	mutex      *sync.Mutex          //for local locking only
	dist_lock  *concurrency.Mutex   //for dist'd locking only, not exported
	cc_session *concurrency.Session //for dist'd locking only, not exported
}

// Interface data type for memory-based interface methods

type Kvs_mem struct {
	mutex *sync.Mutex
}

// Quick K/V container

type Kvi_KV struct {
	Key   string
	Value string
}

// Interface for the ETC KV store

type Kvi interface {
	DistLock() error
	DistTimedLock(tosec int) error
	DistUnlock() error
	Store(key string, value string) error
	TempKey(key string) error
	Get(key string) (string, bool, error)
	GetRange(keystart string, keyend string) ([]Kvi_KV, error)
	Delete(key string) error
	Transaction(key, op, value, thenkey, thenval, elsekey, elseval string) (bool, error)
	TAS(key string, testval string, setval string) (bool, error)
	Watch(key string) (string, int)
	WatchWithCB(key string, op int, cb WatchCBFunc, userdata interface{}) (WatchCBHandle, error)
	WatchCBCancel(cbh WatchCBHandle)
	Close() error
}

// Handle used for K/V watch via callback function

type WatchCBHandle struct {
	key      string //Don't export these!
	op       int
	killme   chan int
	cb       WatchCBFunc
	userdata interface{}
	hnd_etcd *Kvs_etcd
	hnd_mem  *Kvs_mem
}

//Backing for the MEM interface.  We'll keep it local/global in order to
//mimic the ETCD backing, which remains after a close.  It won't survive
//an application restart, however.

var memStorage = make(map[string]string)

/////////////////////////////////////////////////////////////////////////////
// FUNCTIONS
/////////////////////////////////////////////////////////////////////////////

// Get application basename (not full path).
//
// Args:   None
//
// Return: Name of the currently running application.

func getAppBase() string {
	return path.Base(os.Args[0])
}

// (ETCD) Get the value of a key.  There are several possible return scenarios:
//
//	o There is an error fetching the key.  This will result in an empty key,
//	  non-nil error string and the key-exists indicator set to false.
//	o The key exists but has an empty-string value.  The return values will
//	  be an empty string, key-exists flag set to true, and a nil error.
//	o The key exists and has a non-empty value.  The return value will be
//	  the key's value, key-exissts flag set to true, and a nil error.
//
// key(in):  Key to get the value of.
//
// Return:   see above
func (kvs *Kvs_etcd) Get(key string) (string, bool, error) {
	kvc := clientv3.NewKV(kvs.client)
	lctx, lctx_cancel := context.WithTimeout(context.Background(), 5*time.Second)
	kval, err := kvc.Get(lctx, key)
	lctx_cancel()

	if err != nil {
		return "", false, err
	}

	if kval.Count == 0 {
		return "", false, nil
	}
	if kval.Count > 1 {
		err := fmt.Errorf("WARNING: fetched %d keys for '%s', should only be 1",
			kval.Count, key)
		//for _,ev := range kval.Kvs {
		//    log.Printf("Key: '%s', val: '%s'\n",ev.Key,ev.Value)
		//}
		return "", false, err
	}

	return string(kval.Kvs[0].Value), true, nil
}

// (MEM) Get the value of a key.  There are several possible return scenarios:
//
//	o There is an error fetching the key.  This will result in an empty key,
//	  non-nil error string and the key-exists indicator set to false.
//	o The key exists but has an empty-string value.  The return values will
//	  be an empty string, key-exists flag set to true, and a nil error.
//	o The key exists and has a non-empty value.  The return value will be
//	  the key's value, key-exissts flag set to true, and a nil error.
//
// key(in):  Key to get the value of.
//
// Return:   see above
func (kvs *Kvs_mem) Get(key string) (string, bool, error) {
	kvs.mutex.Lock()
	defer kvs.mutex.Unlock()
	val, ok := memStorage[key]
	if !ok {
		return "", false, nil
	}
	return val, true, nil
}

// (ETCD) Get a range of keys.  All keys returned will be lexicographically
// constrained by 'keystart' and 'keyend'.  Note that ETCD doesn't have a
// way to just grab an arbitrary list of keys -- they must be a "range"
// where (returned_keys >= keystart) && (returned_keys <= keyend))
//
// NOTE!!!  The returned list of keys is NOT SORTED.  This is to save time
// when callers don't care.
//
// Also note: if there are no matches, an empty set is returned; this is not an
// error.
//
// keystart(in): Key range start.
//
// keyend(in):   Key range end.
//
// Return:       Unsorted array of key/value structs on success;
// nil on success, error string on error
func (kvs *Kvs_etcd) GetRange(keystart string, keyend string) ([]Kvi_KV, error) {
	var svals []Kvi_KV
	kvc := clientv3.NewKV(kvs.client)
	lctx, lctx_cancel := context.WithTimeout(context.Background(), 5*time.Second)
	kval, err := kvc.Get(lctx, keystart, clientv3.WithFromKey(), clientv3.WithRange(keyend))
	lctx_cancel()

	if err != nil {
		return svals, err
	}

	if kval.Count > 0 {
		for _, ev := range kval.Kvs {
			svals = append(svals, Kvi_KV{string(ev.Key), string(ev.Value)})
		}
	}

	return svals, nil
}

// (MEM) Get a range of keys.  All keys returned will be lexicographically
// constrained by 'keystart' and 'keyend'.  Note that ETCD doesn't have a
// way to just grab an arbitrary list of keys -- they must be a "range"
// where (returned_keys >= keystart) && (returned_keys <= keyend))
//
// NOTE!!!  The returned list of keys is NOT SORTED.  This is to save time
// when callers don't care.
//
// Also note: if there are no matches, an empty set is returned; this is not an
// error.
//
// keystart(in): Key range start.
//
// keyend(in):   Key range end.
//
// Return:       Unsorted array of key/value structs on success;
// nil on success, error string on error
func (kvs *Kvs_mem) GetRange(keystart string, keyend string) ([]Kvi_KV, error) {
	var svals []Kvi_KV

	for key, val := range memStorage {
		if (key >= keystart) && (key <= keyend) {
			svals = append(svals, Kvi_KV{key, val})
		}
	}

	return svals, nil
}

// (ETCD) Store a value of a key.  If the key does not exist it is created.
//
// key(in):  Key to create or update.
//
// val(in):  Value to assign to they key.
//
// Return:   nil on success, else error string.
func (kvs *Kvs_etcd) Store(key string, val string) error {
	kvc := clientv3.NewKV(kvs.client)
	lctx, lctx_cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, err := kvc.Put(lctx, key, val)
	lctx_cancel()

	if err != nil {
		return err
	}
	return nil
}

// (MEM) Store a value of a key.  If the key does not exist it is created.
//
// key(in):  Key to create or update.
//
// val(in):  Value to assign to they key.
//
// Return:   nil on success, else error string.
func (kvs *Kvs_mem) Store(key string, val string) error {
	kvs.mutex.Lock()
	defer kvs.mutex.Unlock()
	memStorage[key] = val
	return nil
}

// (ETCD) Delete a key.  Note that deleting a non-existent key is not an
// error.
//
// key(in):  Key to delete.
//
// Return:   nil on success, error string on error.
func (kvs *Kvs_etcd) Delete(key string) error {
	kvc := clientv3.NewKV(kvs.client)
	lctx, lctx_cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, err := kvc.Delete(lctx, key)
	lctx_cancel()

	if err != nil {
		fmt.Printf("ERROR deleting key %s:%s", key, err)
		return err
	}

	return nil
}

// (MEM) Delete a key.  Note that deleting a non-existent key is not an
// error.
//
// key(in):  Key to delete.
//
// Return:   nil on success, error string on error.
func (kvs *Kvs_mem) Delete(key string) error {
	kvs.mutex.Lock()
	defer kvs.mutex.Unlock()
	_, ok := memStorage[key]
	if ok {
		delete(memStorage, key)
	}

	return nil
}

// (ETCD) Perform a Transaction (atomic if... then... else...).
// Does the following case sensitive check:
//
// if (value_of_key op value) then key.val = thenval else key.val = elseval
//
// key(in):     Key to check, drives the transaction.
//
// op(in):      Operator in "if" portion -- supports =, !=, <, and >.
//
// value(in):   Value to check against key's value in "if" portion.
//
// thenkey(in): Key to change if "if" evaluates to TRUE.
//
// thenval(in): Value to assign to key if "if" evaluates to TRUE.
//
// elsekey(in): Key to change if "if" evaluates to FALSE.
//
// elseval(in): Value to assign to key if "if" evaluates to FALSE.
//
// Return:      true if the 'if' portion succeeded, else 'false';
// nil on success, error string if transaction fails to execute.
// if err != nil then no transaction took place
func (kvs *Kvs_etcd) Transaction(key, op, value, thenkey, thenval, elsekey, elseval string) (bool, error) {
	kvc := clientv3.NewKV(kvs.client)
	lctx, lctx_cancel := context.WithTimeout(context.Background(), 5*time.Second)

	rsp, err := kvc.Txn(lctx).
		If(clientv3.Compare(clientv3.Value(key), op, value)).
		Then(clientv3.OpPut(thenkey, thenval)).
		Else(clientv3.OpPut(elsekey, elseval)).
		Commit()
	lctx_cancel()
	if err == nil {
		return rsp.Succeeded, nil
	}
	return false, err
}

// (MEM) Perform a Transaction (atomic if... then... else...).
// Does the following case sensitive check:
//
// if (value_of_key op value) then key.val = thenval else key.val = elseval
//
// key(in):     Key to check, drives the transaction.
//
// op(in):      Operator in "if" portion -- supports =, !=, <, and >.
//
// value(in):   Value to check against key's value in "if" portion.
//
// thenkey(in): Key to change if "if" evaluates to TRUE.
//
// thenval(in): Value to assign to key if "if" evaluates to TRUE.
//
// elsekey(in): Key to change if "if" evaluates to FALSE.
//
// elseval(in): Value to assign to key if "if" evaluates to FALSE.
//
// Return:      true if the 'if' portion succeeded, else 'false';
// nil on success, error string if transaction fails to execute.
// if err != nil then no transaction took place
func (kvs *Kvs_mem) Transaction(key, op, value, thenkey, thenval, elsekey, elseval string) (bool, error) {
	kvs.mutex.Lock()
	defer kvs.mutex.Unlock()
	thenop := false

	if (op == "=") && (memStorage[key] == value) {
		thenop = true
	} else if (op == "<") && (memStorage[key] < value) {
		thenop = true
	} else if (op == ">") && (memStorage[key] > value) {
		thenop = true
	} else if (op == "!=") && (memStorage[key] != value) {
		thenop = true
	}

	if thenop {
		memStorage[thenkey] = thenval
	} else {
		memStorage[elsekey] = elseval
	}

	return thenop, nil
}

// (ETCD) Do an atomic Test-And-Set operation:
//
// key(in):     Key to test.
//
// testval(in): Test value.
//
// setval(in):  Value to set if key value == test value.
//
// Return:      true if the set happened, else false; nil, unless an
// error occurred.
func (kvs *Kvs_etcd) TAS(key string, testval string, setval string) (bool, error) {
	kvc := clientv3.NewKV(kvs.client)
	lctx, lctx_cancel := context.WithTimeout(context.Background(), 5*time.Second)

	rsp, err := kvc.Txn(lctx).
		If(clientv3.Compare(clientv3.Value(key), "=", testval)).
		Then(clientv3.OpPut(key, setval)).
		Commit()
	lctx_cancel()

	if err != nil {
		return false, err
	}
	if rsp.Succeeded {
		return true, nil
	}
	return false, nil
}

// (MEM) Do an atomic Test-And-Set operation:
//
// key(in):     Key to test.
//
// testval(in): Test value.
//
// setval(in):  Value to set if key value == test value.
//
// Return:      true if the set happened, else false; nil, unless an
// error occurred.
func (kvs *Kvs_mem) TAS(key string, testval string, setval string) (bool, error) {
	kvs.mutex.Lock()
	defer kvs.mutex.Unlock()

	val, vok := memStorage[key]
	if !vok {
		memStorage[key] = setval
		return true, nil
	}

	if val == testval {
		memStorage[key] = setval
		return true, nil
	}

	return false, nil
}

// (ETCD) Acquire a distributed lock.  This is an atomic operation.  The
// lock lives within ETCD and can be used by any number of other applications.
//
// NOTE: if this lock is used multiple times within an application,
// it will just appear to acquire the lock multiple times.  This is not
// the lock to use within an application; use plain mutexes instead.
// This lock is specifically designed to only work ACROSS applications,
// even multiple copies of the same application.
//
// Args: None
//
// Return: nil on success, error string on error.
func (kvs *Kvs_etcd) DistLock() error {
	lctx, lctx_cancel := context.WithTimeout(context.Background(),
		time.Duration(1000000)*time.Second)
	err := kvs.dist_lock.Lock(lctx)
	lctx_cancel()
	if err != nil {
		return err
	}
	return nil
}

// (MEM) Acquire a lock.  Note that without ETCD backing, this won't work.
// That is OK because it makes no sense to use a memory-backed implementation
// in a horizontally scaled application!  So, do nothing.
//
// Args: None
//
// Return: nil on success, error string on error.
func (kvs *Kvs_mem) DistLock() error {
	return nil
}

// (ETCD) Acquire a distributed lock, but time out if it remains held by
// some other actor.  This can effectively be used as a "TryLock" by using
// a short timeout.
//
// NOTE: if this lock is used multiple times within an application,
// it will just appear to acquire the lock multiple times.  This is not
// the lock to use within an application; use plain mutexes instead.
// This lock is specifically designed to only work across applications,
// even multiple copies of the same application.
//
// tosec(in): Number of seconds to wait for lock.
//
// Return:    nil on success, error string on error (including timeout).
func (kvs *Kvs_etcd) DistTimedLock(tosec int) error {
	var err error

	if kvs.cc_session != nil {
		err = fmt.Errorf("ERROR: distributed lock already held by this process")
		return err
	}

	lctx, lctx_cancel := context.WithTimeout(context.Background(),
		time.Duration(tosec)*time.Second)
	defer lctx_cancel()
	lresp, grerr := kvs.client.Lease.Grant(lctx, int64(tosec))
	if grerr != nil {
		return grerr
	}

	//Create a session for this lock

	kvs.cc_session, err = concurrency.NewSession(kvs.client,
		concurrency.WithLease(lresp.ID))
	if err != nil {
		return err
	}
	//Lease acquired.  If app dies, lease will expire and session will
	//die releasing the lock.

	kvs.dist_lock = concurrency.NewMutex(kvs.cc_session, DIST_LOCK_KEY)
	err = kvs.dist_lock.Lock(lctx)

	if err != nil {
		//TODO: distinguish between timeout and function call failure
		//Get rid of the session and set everything back to nil
		kvs.cc_session.Close()
		kvs.cc_session = nil
		return err
	}
	return nil
}

// (MEM) Acquire a distributed lock, but time out eventually.  NOTE: THIS
// DOES NOT WORK -- mem: based instances can't do dist'd locks and are
// assumed to be single instance implementations.  So, do nothing.
//
// tosec(in): Number of seconds to wait for lock. (IGNORED)
//
// Return:    nil
func (kvs *Kvs_mem) DistTimedLock(tosec int) error {
	return nil
}

// (ETCD) Unlock a distributed lock.
//
// NOTE: if this lock is unlocked multiple times within an application,
// it will just appear to work.  This is not the lock to use within an
// application; use plain mutexes instead.  This lock is specifically
// designed to only work across applications, even multiple copies of
// the same application.
//
// Args:   None
//
// Return: nil on success, error string on error.
func (kvs *Kvs_etcd) DistUnlock() error {
	var err error
	if kvs.cc_session != nil {
		lctx, lctx_cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer lctx_cancel()
		err = kvs.dist_lock.Unlock(lctx)
		if err != nil {
			log.Println("ERROR releasing sess lock:", err)
		}
		kvs.cc_session.Close()
		kvs.cc_session = nil
		return err
	}
	return nil
}

// (MEM) Unlock a distributed lock.  Since mem: based instances can't do
// dist'd locks, we do nothing.
//
// Args:   None
//
// Return: nil on success, error string on error.
func (kvs *Kvs_mem) DistUnlock() error {
	return nil
}

// (ETCD) Watch a key and block until it either changes value, gets created,
// or gets deleted.
//
// key(in):  Key to watch.
//
// Return:   New key value if the value changed;
// Operation, either KVC_KEYCHANGE_PUT or KVC_KEYCHANGE_DELETE
func (kvs *Kvs_etcd) Watch(key string) (string, int) {
	var rch clientv3.WatchChan
	var kcval int = KVC_KEYCHANGE_INVAL
	var keyval string = ""

	//TODO: does this need a timeout?  Does it need canceling?  Docs
	//don't seem to indicate that it needs canceling.

	rch = kvs.client.Watch(context.Background(), key)
	wresp := <-rch
	ev := wresp.Events[0]

	switch ev.Type {
	case mvccpb.PUT:
		kcval = KVC_KEYCHANGE_PUT
		keyval = string(ev.Kv.Value) //string(ev.PrevKv.Value)) ??

	case mvccpb.DELETE:
		kcval = KVC_KEYCHANGE_DELETE

	default:
		log.Printf("ERROR: Unknown trip type: '%s'\n",
			string(ev.Type))
	}

	return keyval, kcval
}

// Memory watch convience routine used by the MEM interface.
//
// NOTE: this routine is not guaranteed to catch every change.
// It is polling for changes, and there can be multiple changes
// between polls.
//
// kvs(in):  Memory interface data descriptor.
//
// key(in):  Key to watch
//
// Return:   New key value if the value changed;
// Operation, either KVC_KEYCHANGE_PUT or KVC_KEYCHANGE_DELETE
func mem_watch(kvs *Kvs_mem, key string) (string, int) {
	kvs.mutex.Lock()
	pval, ok := memStorage[key]
	kvs.mutex.Unlock()

	for {
		kvs.mutex.Lock()
		thisval, tok := memStorage[key]
		kvs.mutex.Unlock()

		if !ok && tok {
			//key came into being.  Report a PUT
			return thisval, KVC_KEYCHANGE_PUT
		} else if ok && !tok {
			return "", KVC_KEYCHANGE_DELETE
		} else if ok && tok {
			if thisval != pval {
				return thisval, KVC_KEYCHANGE_PUT
			}
		}

		time.Sleep(500000 * time.Microsecond)
	}
}

// (MEM) Watch a key and block until it either changes value, gets created,
// or gets deleted.
//
// key(in):  Key to watch.
//
// Return:   New key value if the value changed;
// Operation, either KVC_KEYCHANGE_PUT or KVC_KEYCHANGE_DELETE
func (kvs *Kvs_mem) Watch(key string) (string, int) {
	val, op := mem_watch(kvs, key)
	return val, op
}

// Convience goroutine for continuous watching of a key using actual ETCD.
// Runs until either the application cancels it via WatchCBCancel(), or by
// the registered callback function returning FALSE.
//
// hnd(in):  K/V watch info.
//
// Return:   None.
func watch_helper_etcd(hnd WatchCBHandle) {
	var rv bool

	rch := hnd.hnd_etcd.client.Watch(context.Background(), hnd.key)

	for {
		rv = true
		select {
		case wresp := <-rch:
			ev := wresp.Events[0]
			if (ev.Type == mvccpb.PUT) && (hnd.op == KVC_KEYCHANGE_PUT) {
				rv = hnd.cb(string(ev.Kv.Key), string(ev.Kv.Value), hnd.op, hnd.userdata)
			} else if (ev.Type == mvccpb.DELETE) && (hnd.op == KVC_KEYCHANGE_DELETE) {
				hnd.cb(string(ev.Kv.Key), "", hnd.op, hnd.userdata)
				rv = false //TODO: should we keep watching a deleted key?
			}
		case <-hnd.killme:
			return
		}

		if !rv {
			return
		}
	}
}

// Convience goroutine for continuous watching of a key using the memory
// interface.  Runs until either the application cancels it via
// WatchCBCancel(), or by the registered callback function returning FALSE.
//
// hnd(in):  K/V watch info.
//
// Return:   None.
func watch_helper_mem(hnd WatchCBHandle) {
	var rv bool
	for {
		rv = true
		select {
		case <-hnd.killme:
			return
		default:
			val, op := mem_watch(hnd.hnd_mem, hnd.key)
			if op == hnd.op {
				if op == KVC_KEYCHANGE_PUT {
					rv = hnd.cb(hnd.key, val, op, hnd.userdata)
				} else if op == KVC_KEYCHANGE_DELETE {
					hnd.cb(hnd.key, "", op, hnd.userdata)
					rv = false // TODO: should we keep watching a del'd key?
				}
			}
		}

		if !rv {
			return
		}
		time.Sleep(500000 * time.Microsecond)
	}
}

// (ETCD) Set up a watcher goroutine for a key and call a callback function
// when there is a change.  The callback func will get called when a key's
// value either changes or when the key is deleted.
//
// Cancelation:
//
// The watcher will run until the one of the following takes place:
//
//	o The key watched key is deleted,
//	o The callback function returns 'false'
//	o The watcher is manually cancelled by calling WatchCBCancel().
//
// key(in):      Key to watch.
//
// op(in):       Key change operation to watch for: KVC_KEYCHANGE_PUT, KVC_KEYCHANGE_DELETE
//
// cb(in):       Function to call when specified key changes as specified by 'op'.
//
// userdata(in): Arbitrary data to pass to callback func.
//
// Return:       Handle (used for cancellation); nil on success, error string on error.
func (kvs *Kvs_etcd) WatchWithCB(key string, op int, cb WatchCBFunc, userdata interface{}) (WatchCBHandle, error) {
	var wh = WatchCBHandle{}

	if (op != KVC_KEYCHANGE_PUT) && (op != KVC_KEYCHANGE_DELETE) {
		return wh, fmt.Errorf("invalid key watch operation: %d", op)
	}

	wh = WatchCBHandle{key, op, make(chan int, 2), cb, userdata, kvs, nil}

	go watch_helper_etcd(wh)
	return wh, nil
}

// (MEM) Set up a watcher goroutine for a key and call a callback function
// when there is a change.  The callback func will get called when a key's
// value either changes or when the key is deleted.
//
// Cancelation:
//
// The watcher will run until the one of the following takes place:
//
//	o The key watched key is deleted,
//	o The callback function returns 'false'
//	o The watcher is manually cancelled by calling WatchCBCancel().
//
// key(in):      Key to watch.
//
// op(in):       Key change operation to watch for: KVC_KEYCHANGE_PUT,KVC_KEYCHANGE_DELETE
//
// cb(in):       Function to call when specified key changes as specified by 'op'.
//
// userdata(in): Arbitrary data to pass to callback function.
//
// Return:       Handle (used for cancellation); nil on success, error
// string on error.
func (kvs *Kvs_mem) WatchWithCB(key string, op int, cb WatchCBFunc, userdata interface{}) (WatchCBHandle, error) {
	var wh = WatchCBHandle{}
	if (op != KVC_KEYCHANGE_PUT) && (op != KVC_KEYCHANGE_DELETE) {
		return wh, fmt.Errorf("invalid key watch operation: %d", op)
	}
	wh = WatchCBHandle{key, op, make(chan int, 2), cb, userdata, nil, kvs}
	go watch_helper_mem(wh)
	return wh, nil
}

// (ETCD) Cancel a K/V watch-with-callback.
//
// cbh(in):  Watch handle from WatchWithCB().
//
// Return:   None.
func (kvs *Kvs_etcd) WatchCBCancel(cbh WatchCBHandle) {
	cbh.killme <- 1
}

// (MEM) Cancel a K/V watch-with-callback.
//
// cbh(in):  Watch handle from WatchWithCB().
//
// Return:   None.
func (kvs *Kvs_mem) WatchCBCancel(cbh WatchCBHandle) {
	cbh.killme <- 1
}

// (ETCD) Close an ETCD connection.
//
// Args:   None
//
// Return: nil (reserved for future expansion)
func (kvs *Kvs_etcd) Close() error {
	kvs.client.Close()
	return nil
}

// (MEM) Close an MEM connection.  Note that we don't delete the underlying
// storage!  This will more closely mimic the ETCD backing which remains
// after a close.
//
// Args:   None
//
// Return: nil (reserved for future expansion)
func (kvs *Kvs_mem) Close() error {
	//Nothing to do.
	return nil
}

// Opens an ETCD/MEM interface.
//
// endpoint(in): ETCD endpoint URL, e.g. "http://10.2.3.4:2379"
// MEM  endpoint URL, e.g. "mem:"
//
// options(in):  Array of option strings.  Ignored for now.  Someday there
// may be RBAC stuff, etc. in the options.
//
// Return: ETCD or MEM data descriptor;  nil on success, error string on error
func Open(endpoint string, options string) (Kvi, error) {
	/*    for _, opt := range options {
	          switch strings.ToLower(opt) {
	          case "insecure":
	              insecure = true
	          case "debug":
	              Debug = true
	          }
	      }
	*/

	//Is this an in-memory variety?

	x := strings.Split(endpoint, ":")
	if x[0] == "mem" {
		memStorage[getAppBase()] = getAppBase()
		kvs := &Kvs_mem{&sync.Mutex{}}
		return kvs, nil
	}

	//This is a real ETCD based interface.

	var kvs *Kvs_etcd = &Kvs_etcd{}

	//Default endpoint during testing on a local etcd daemon
	//was http://localhost:2380

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{endpoint},
		DialTimeout: 10 * time.Second,
	})

	if err != nil {
		log.Println("ERROR opening connection to ETCD:", err)
		return kvs, err
	}

	/*    lctx,lctx_cancel := context.WithTimeout(context.Background(),
	                                              5*time.Second)
	      sess,serr := concurrency.NewSession(cli,concurrency.WithContext(lctx))
	      if (serr != nil) {
	          log.Println("ERROR creating concurrency session:",serr)
	          return kvs,serr
	      }
	      lctx_cancel()

	      cc_key := fmt.Sprintf("%s_lock",getAppBase())
	      dl := concurrency.NewMutex(sess,cc_key)
	*/

	(*kvs).client = cli
	(*kvs).mutex = &sync.Mutex{}
	//kvs.cc_session = sess
	//kvs.dist_lock = dl
	return kvs, nil
}

// Convenience goroutine to keep a lease alive.  The idea is to periodically
// send a keep-alive to the specified key's lease so that it continues
// to exist.  If the program exits or aborts, the lease will expire and
// ETCD will delete the key.
//
// Note that it is possible for things to get so busy that the lease
// expires before we can do a keep-alive.  We will handle that by re-creating
// the lease.  This is not ideal depending on how the key is used, because
// this means that the key goes away and comes back.  This should only
// happen in very extreme cases of overload, at which point there will
// be bigger problems anyway!
//
// kvs(in):   ETCD connection descriptor
// key(in):   Key to maintain lease for
// lid(in):   Lease ID to maintain.
// Return:    None.
func lease_keep_alive(kvs *Kvs_etcd, key string, lid clientv3.LeaseID) {
	localLid := lid

	for {
		if kvs.client == nil {
			return
		}
		time.Sleep(2 * time.Second)
		lctx, lctx_cancel := context.WithTimeout(context.Background(),
			5*time.Second)
		_, perr := kvs.client.KeepAliveOnce(lctx, localLid)
		lctx_cancel()
		if perr != nil {
			var llerr error
			fmt.Println("ERROR renewing temp key lease, key:", key, ":", perr)
			fmt.Printf("Recreating lease...\n")
			localLid, llerr = create_tk_lease(kvs, key)
			if llerr != nil {
				fmt.Println("ERROR trying to re-create temp key lease:", llerr)
				return
			} else {
				fmt.Printf("New temp key lease acquired.\n")
			}
		}
	}
}

// Convenience function to create a temporary key lease.
//
// kvs(in):   ETCD connection descriptor
// key(in):   Key to maintain lease for
// Return:    Lease ID on success; nil on success, error string on error
func create_tk_lease(kvs *Kvs_etcd, key string) (clientv3.LeaseID, error) {
	var lid clientv3.LeaseID
	lctx, lctx_cancel := context.WithTimeout(context.Background(), 5*time.Second)
	rsp, err := kvs.client.Grant(lctx, 10) //TODO: is this right?
	if err != nil {
		lctx_cancel()
		return lid, err
	}
	lid = rsp.ID
	_, perr := kvs.client.Put(lctx, key, "1", clientv3.WithLease(lid))
	lctx_cancel()
	if perr != nil {
		return lid, perr
	}

	return lid, nil
}

// Create a temporary key.  It will exist only for the life of the application.
// Once the app dies or aborts unexpectedly, the lease will expire and ETCD
// will delete the key.  This is useful for horizontally scaled applications
// so that any given copy can know how many copies are running, as one
// example.  When the key is created, some arbitrary token value is placed
// in the key (not specifyable by the caller).
//
// key(in):  Temporary key to create.
// Return:   nil on success, error string on error.
func (kvs *Kvs_etcd) TempKey(key string) error {
	leaseID, err := create_tk_lease(kvs, key)
	if err != nil {
		return err
	}

	//Spin up a goroutine to keep this alive

	go lease_keep_alive(kvs, key, leaseID)
	return nil
}

// Create a temporary key.  Note that this does NOT mimic the ETCD
// implementation!!  It will create the key, but it is up to the application
// developer to fake out the behavior of temp keys created by other
// instances.
//
// key(in):  Temporary key to create.
// Return:   nil on success, error string on error.
func (kvs *Kvs_mem) TempKey(key string) error {
	kvs.mutex.Lock()
	defer kvs.mutex.Unlock()
	memStorage[key] = "1"
	return nil
}

//TODO:
//  o RBAC?
