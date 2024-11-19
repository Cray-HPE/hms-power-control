# HMS ETCD Interface Package

## Overview

The *hms-hmetcd* package wraps the Golang ETCD Client V3 package.  

This provides a simplified ETCD interface to reduce code bloat and repetition.
Common K/V functions are provided which abstract the details of the Golang 
Client V3 package.  Another advantage to using this package is the elimination 
of refactoring microservice code if the underlying ETCD interface is swapped 
out.


## Interface Implementations

There are 2 implementations to this interface:

* Actual ETCD
* In-Memory

The actual ETCD interface will be used in production.   The in-memory
interface can be used for testing.  In theory it could also be used in 
production as long as there are not more than one instance of a given service
running, since the different instances can't see each other's memory space.


## Methods

This interface provides methods for: 

* Opening/closing an ETCD handle 
* Inter-process distributed locking
* K/V Get/Store/Delete operations
* Test-And-Set operations
* K/V value-change watch mechanisms


## Typical Usage Flow

Typically an application will begin by opening a handle to the ETCD K/V store.

Once that is done, the application can store, fetch, and delete K/V pairs
as needed.

It is possible to fetch a range of K/V pairs using start/end key 
specifications.

Test-And-Set operations are handy for atomic value changes.

Transaction operations provide an 'if-then-else' construct for setting or
changing K/V values.

Watchers can be set up to monitor a K/V pair and notify the application when
they change.


## Distributed Locks

Multi-instance applications run multiple copies of themselves.  This is 
common in Kubernetes environments for redundancy and high-availability.  Such
applications may encounter situations where only one instance
is permitted to do a particular task at a time.   This requires that an
instance acquire a distributed lock, preventing other instances of that service
(or any other process using the same ETCD instance) from doing a particular
task.

This package provides such a locking mechanism.  It is possible to request
and use a distributed lock in a very similar fashion to any other mutex-type
lock.   The difference is that the lock itself lives in ETCD so it is visible
to any process that is connected to a particular running ETCD instance.

Distributed locks can be acquired indefinitely or by specifying a maximum
hold time.

Distributed locks are released when they are no longer needed.


## Interface Functions

These functions are implemented for both in-memory and "real ETCD" backing
stores.  The specifications below will show the *Kvs_etcd* implementation;
*Kvs_mem* can also be used.


```
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

// Key/value change types, used for watches

const (
	KVC_KEYCHANGE_INVAL  = 0
	KVC_KEYCHANGE_PUT    = 1
	KVC_KEYCHANGE_DELETE = 2
)


/////////////////////////////////////////////////////////////////////////////
//                       OPEN/CLOSE
/////////////////////////////////////////////////////////////////////////////

// Open a handle to a K/V store.  Returns an interface object.
//
// endpoint(in): K/V store URI, e.g. "https://cray-hmnfd-etcd-client:2379" (ETCD)
//               or "mem:" (in-memory)
// options(in):  Currently unused, reserved for future enhancements.
// Return:       K/V interface handle
//               nil on success, error specification if an error occurred.

func Open(endpoint string, options string) (Kvi, error)

// Close a K/V store handle
//
// Return: nil on success, error string on error.

func (kvs *Kvs_etcd) Close() error


/////////////////////////////////////////////////////////////////////////////
//                       GET/STORE/DELETE
/////////////////////////////////////////////////////////////////////////////

// Get the value of the specified key.
//
// key(in): Key to search for.
// Return:  String value associated with key on success, empty on error
//          true if key exists, false if not
//          nil on success, error specification if an error occurred.

func (kvs *Kvs_etcd) Get(key string) (string, bool, error)


// Fetch a range of K/V pairs.
//
// keystart(in): Start pattern of key range, e.g. "x1"
// keyend(in):   End pattern of key range, e.g. "x9"
// Return:       Slice of K/V objects on success
//               nil on success, error specification on error

func (kvs *Kvs_etcd) GetRange(keystart string, keyend string) ([]Kvi_KV, error)


// Store a value for a key.
//
// key(in):    Key to associate value with.
// value(in(): String value.
// Return:     nil on success, error specification if an error occurred.

func (kvs *Kvs_etcd) Store(key string, value string) error


// Create a temporary key.  It will exist only for the life of the application.
// Once the app dies or aborts unexpectedly, the lease will expire and ETCD
// will delete the key.  This is useful for horizontally scaled applications
// so that any given copy can know how many copies are running, as one
// example.  When the key is created, some arbitrary token value is placed
// in the key (not specifyable by the caller).
//
// key(in):  Temporary key to create.
// Return:   nil on success, error string on error.

func (kvs *Kvs_etcd) TempKey(key string) error


// D) Delete a key.  Note that deleting a non-existent key is not an
// error.
//
// key(in):  Key to delete.
// Return:   nil on success, error specification if an error occurred.

func (kvs *Kvs_etcd) Delete(key string) error


/////////////////////////////////////////////////////////////////////////////
//                       TEST-AND-SET
/////////////////////////////////////////////////////////////////////////////


// Perform a Transaction (atomic if... then... else...).
// Does the following case sensitive check:
//
// if (value_of_key op value) then key.val = thenval else key.val = elseval
//
// key(in):     Key to check, drives the transaction.
// op(in):      Operator in "if" portion -- supports =, !=, <, and >.
// value(in):   Value to check against key's value in "if" portion.
// thenkey(in): Key to change if "if" evaluates to TRUE.
// thenval(in): Value to assign to key if "if" evaluates to TRUE.
// elsekey(in): Key to change if "if" evaluates to FALSE.
// elseval(in): Value to assign to key if "if" evaluates to FALSE.
// Return:      true if the 'if' portion succeeded, else 'false';
//              nil on success, error string if transaction fails to execute.
//                if err != nil then no transaction took place

func (kvs *Kvs_etcd) Transaction(key, op, value, thenkey, thenval, elsekey, elseval string) (bool, error)


// Do an atomic Test-And-Set operation:
//
//  if (key.value == testval) then 
//      key.value = setval
//  end
//
// key(in):     Key to test.
// testval(in): Test value.
// setval(in):  Value to set if key value == test value.
// Return:      true if the set happened, else false; 
//              nil on success, error specification if an error occurred.

func (kvs *Kvs_etcd) TAS(key string, testval string, setval string) (bool, error)


/////////////////////////////////////////////////////////////////////////////
//                    DISTRIBUTED LOCKING
/////////////////////////////////////////////////////////////////////////////


// Acquire a distributed lock.  This is an atomic operation.  The lock
// lives within ETCD and can be used by any number of other applications.
//
// NOTE: if this lock is used multiple times within an application,
// it will just appear to acquire the lock multiple times.  This is not
// the lock to use within an application; use plain mutexes instead.
// This lock is specifically designed to only work ACROSS applications,
// even multiple copies of the same application.
//
// Args: None
// Return: nil on success, error string on error.

func (kvs *Kvs_etcd) DistLock() error


// Acquire a distributed lock, but time out if it remains held by some
// other actor.  This can effectively be used as a "TryLock" by using
// a short timeout.
//
// NOTE: if this lock is used multiple times within an application,
// it will just appear to acquire the lock multiple times.  This is not
// the lock to use within an application; use plain mutexes instead.
// This lock is specifically designed to only work across applications,
// even multiple copies of the same application.
//
// tosec(in): Number of seconds to wait for lock.
// Return:    nil on success, error string on error (including timeout).

func (kvs *Kvs_etcd) DistTimedLock(tosec int) error


// Unlock a distributed lock.
//
// NOTE: if this lock is unlocked multiple times within an application,
// it will just appear to work.  This is not the lock to use within an
// application; use plain mutexes instead.  This lock is specifically
// designed to only work across applications, even multiple copies of
// the same application.
//
// Args:   None
// Return: nil on success, error string on error.

func (kvs *Kvs_etcd) DistUnlock() error


/////////////////////////////////////////////////////////////////////////////
//                     	K/V CHANGE WATCHING
/////////////////////////////////////////////////////////////////////////////


// Watch a key and block until it either changes value, gets created,
// or gets deleted.
//
// key(in):  Key to watch.
// Return:   New key value if the value changed;
//           Operation, either KVC_KEYCHANGE_PUT or KVC_KEYCHANGE_DELETE

func (kvs *Kvs_etcd) Watch(key string) (string, int)


// Set up a watcher goroutine for a key and call a callback function
// when there is a change.  The callback func will get called when a key's
// value either changes or when the key is deleted.
//
// Cancelation:
//
// The watcher will run until the one of the following takes place:
//    o The key watched key is deleted,
//    o The callback function returns 'false'
//    o The watcher is manually cancelled by calling WatchCBCancel().
//
// key(in):      Key to watch.
// op(in):       Key change operation to watch for: KVC_KEYCHANGE_PUT, KVC_KEYCHANGE_DELETE
// cb(in):       Function to call when specified key changes as specified by 'op'.
// userdata(in): Arbitrary data to pass to callback func.
// Return:       Handle (used for cancellation)
//               nil on success, error string on error.

func (kvs *Kvs_etcd) WatchWithCB(key string, op int, cb WatchCBFunc, userdata interface{}) (WatchCBHandle, error)


// (ETCD) Cancel a K/V watch-with-callback.
//
// cbh(in):  Watch handle from WatchWithCB().
// Return:   None.

func (kvs *Kvs_etcd) WatchCBCancel(cbh WatchCBHandle)
```

## Common Use Case Examples

### Open/Close K/V Store Handle

```
import (
	"github.com/Cray-HPE/hms-hmetcd"
...
	kvHandle,kverr := hmetcd.Open(KV_URL,"")
	if (kverr != nil) {
		log.Printf("ERROR opening K/V handle: %v",kverr)
		return
	}

	... //Do stuff

	kverr = kvHandle.Close()
	if (kverr != nil) {
		log.Printf("ERROR closing K/V handle: %v",kverr)
	}
...
```

### Get and Store K/V Pairs

```
...
	key := "my_key"
	initialVal := "The rain in spain"

	//Store a K/V pair

	sterr := kvHandle.Store(key,initialVal)
	if (sterr != nil) {
		log.Printf("ERROR storing value for key '%s': %v",key,sterr)
		return
	}

	//Get K/V pair

	val,ok,err := kvHandle.Get(key)
	if (err != nil) {
		log.Printf("ERROR fetching value for '%s': %v",key,err)
		return
	}
	if (!ok) {
		log.Printf("ERROR: key '%s' does not exist.",key)
		return
	}
	log.Printf("Key: '%s', value: '%s'",key,val)

	//Delete K/V pair

	err = kvHandle.Delete(key)
	if (err != nil) {
		log.Printf("ERROR deleting key '%s': %v",key,err)
		return
	}
...
```


### Fetch A Range Of K/V Pairs

```
...
	keyStart := "x0"
	keyEnd   := "xz"

	kvList,err := kvHandle.GetRange(keyStart,keyEnd)
	if (err != nil) {
		log.Printf("ERROR fetching key range: %v",err)
		return
	}

	//Iterate over the keys and print out their values

	for k,v := range(kvList) {
		log.Printf("Key: '%s', value: '%s'",k,v)
	}
...
```


### Distributed Locking

```
...
	//Acquire a lock for a max time, block until it's available.

	maxLockSecs := 60
	lckerr := kvHandle.DistTimedLock(maxLockSecs)
	if (lckerr != nil) {
		log.Printf("ERROR acquiring distributed timed lock: %v",lckerr)
		return
	}

	... // Do stuff in here

	lckerr = kvHandle.DistUnlock()
	if (lckerr != nil) {
		log.Printf("ERROR: did not release distributed timed lock! %v",lckerr)
	}
...
```

