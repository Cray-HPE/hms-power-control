# Comp Credentials Package

## Overview

The *hms-compcredentials* package provides an interface for storage and 
retrieval of credential information.  It is a key/value store which uses a 
Vault secure storage back-end.

The keys used are typically component XNames.  The values stored are
JSON-encoded data structures containing various types of sensitive information,
including admin account usernames and passwords.


## Data Types

There are two main data types used with this package; one is the object 
containing the plumbing information for the back-end storage, and the other
is the data structure containing the actual credential information.

This package implements an interface for the following data type.

```
type CompCredStore struct {
	CCPath string
	SS     sstorage.SecureStorage
}
```

Note: the 'SS' member of this data structure is contained in the 
*hms-securestorage* package.

The following data type holds the credential info.  Note that not all entries
need to be populated; only as many as are useful for the application.

```
type CompCredentials struct {
	Xname        string `json:"xname"`
	URL          string `json:"url"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	SNMPAuthPass string `json:"SNMPAuthPass,omitempty"`
	SNMPPrivPass string `json:"SNMPPrivPass,omitempty"`
}
```

## Key Spaces

Most key/value stores have the concept of key spaces, which provide a way to
group sets of key/value objects together.   The CompCreds package also provides
this.  Each CompCredStore "handle" is opened/initialized with a key space 
specified by the caller.  All calls using that handle will store and retrieve
objects within that key space only.   If multiple key spaces are needed,
multiple CompCredStore handles will be needed.


## Protecting Sensitive Data

The CompCredStore interface will always hide sensitive info when asked to
print things.   As a design practice, NEVER print out any sensitive information
in any source code, and NEVER store any sensitive information in source code!


## Methods

```
// Create a new CompCredStore struct that uses a SecureStorage backing store.
// All subsequent storage and retrieval of objects will be in the key space
// specified by 'keyPath'.

func NewCompCredStore(keyPath string, ss sstorage.SecureStorage) *CompCredStore


// Get the credentials for a component specified by xname from the secure store.

func (ccs *CompCredStore) GetCompCred(xname string) (CompCredentials, error)


// Get the credentials for all components in the secure store.

func (ccs *CompCredStore) GetAllCompCreds() (map[string]CompCredentials, error)


// Get the credentials for a list of components in the secure store.

func (ccs *CompCredStore) GetCompCreds(xnames []string) (map[string]CompCredentials, error)


// Store the credentials for a single component in the secure store.

func (ccs *CompCredStore) StoreCompCred(compCred CompCredentials) error


// Due to the sensitive nature of the data in CompCredentials, a custom 
// String function is provided to prevent passwords from being printed 
// directly (accidentally) to output.

func (compCred CompCredentials) String() string
```

## Usage

Typical usage of this package is shown in the following example.

```
...
import (
    sstorage "github.com/Cray-HPE/hms-securestorage"
    compcreds "github.com/Cray-HPE/hms-compcredentials"
)
...

    // Create the Vault adapter and connect to Vault

    ss, err := sstorage.NewVaultAdapter("secret")
    if err != nil {
        return fmt.Errorf("Error: %v", err)
    }

    // Initialize the CompCredStore struct with the Vault adapter.
	// Use the 'hms-creds' key space

    ccs := compcreds.NewCompCredStore("hms-creds", ss)

    // Create a new set of credentials for a component.

    compCred := compcreds.CompCredentials{
        Xname: "x0c0s21b0"
        URL: "10.4.0.8/redfish/v1/UpdateService"
        Username: "test"
        Password: "123"
    }

    // Store the credentials in the CompCredStore (backed by Vault).

    err = ccs.StoreCompCred(compCred)
    if err != nil {
        return fmt.Errorf("Error: %v", err)
        
    }

    // Read the credentials for a component from the CompCredStore
    // (backed by Vault).

    var ccred CompCredentials
    ccred, err = ccs.GetCompCred(compCred.Xname)
    if err != nil {
        return fmt.Errorf("Error: %v", err)
    }

    log.Printf("%v", ccred)
...
```



