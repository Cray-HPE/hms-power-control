This Go package wraps the ETCD Client V3 Go package.  

The reason for the wrapper is to reduce code bloat and repetition.  Common
K/V functions are made available and the Client V3 details hidden.

Functions available via this packages interface:

 o Open()           # Open a connection to ETCD
 o Close()          # Close a connection to ETCD
 o DistLock()       # Acquire distributed lock
 o DistTimedLock()  # Acquire lock but time out if already held
 o DistUnlock()     # Release dist'd lock
 o Store()          # Store a value for a key
 o Get()            # Get the value of a key
 o Delete()         # Delete a K/V
 o Transaction()    # Atomic if/then/else
 o TAS()            # Atomic test-and-set operation
 o Watch()          # Blocking watch for a key's value change or deletion
 o WatchWithCB()    # Watch for key val change/deletion in the background,
                    # call a callback function when change happens.
 o WatchCBCancel()  # Cancel a callback function watcher

IMPLEMENTATIONS:

There are 2 implementations to this interface:

 o Actual ETCD
 o Memory-backed

The actual ETCD interface will be used in production.   The memory-backed
interface can be used until the ETCD kubernetes pods are deployed and 
available.  It is also handy for local testing.


ETCD CLIENT V3 PACKAGE

Note that this is not necessary as it is part of our build tree.  It is noted
here for future reference.

To install the ETCD V3 client:

 o Make a directory and go to it.

 o GOPATH=`pwd` go get -v go.etcd.io/etcd/clientv3

 o cd go.etcd.io

 o mkdir ~/xxx/hms-services/go/src/vendor/go.etcd.io

 o cp -r * ~/xxx/hms-services/go/src/vendor/go.etcd.io

NOTE: this can't go under vendor/github.com because the code inside
go.etcd.io imports packages without the 'github.com' prefix.  Due to the
way GOPATH works this flat-out won't work.

One way to get it to work and be underneath github.com is to make a symlink
in the vendor directory to go.etcd.io, but it's not clear that can be 
checked into GIT.

GO DOCS

There is a rudimentary doc.go file that can be used by 'godoc' to generate
the documentation for this package.  To generate a static HTML file:

  cd go/src/hss/hmetcd
  GOPATH=`pwd`/../../.. godoc -html . > hmetcd.html

