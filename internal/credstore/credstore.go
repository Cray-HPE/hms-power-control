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

package credstore

import (
	"errors"
	"time"

	"github.com/Cray-HPE/hms-base"
	"github.com/Cray-HPE/hms-compcredentials"
	"github.com/Cray-HPE/hms-securestorage"
	"github.com/sirupsen/logrus"
)

func (b *VAULTv0) Init(globals *CREDSTORE_GLOBALS) {
	b.CredStoreGlobals = CREDSTORE_GLOBALS{}
	b.CredStoreGlobals = *globals

	if (b.CredStoreGlobals.Logger == nil) {
		// Set up logger with defaults.
		b.CredStoreGlobals.Logger = logrus.New()
	}

	b.CredStoreGlobals.credsCacheMap = make(map[string]CompCredCached)
	b.CredStoreGlobals.credStoreReady = false
	for *b.CredStoreGlobals.Running {
		if secureStorage, err := securestorage.NewVaultAdapter(""); err != nil {
			b.CredStoreGlobals.Logger.Errorf("Unable to connect to Vault, err: %s! Trying again in 1 second...", err)
			time.Sleep(1 * time.Second)
		} else {
			b.CredStoreGlobals.Logger.Info("Connected to Vault.")

			b.CredStoreGlobals.credStore = compcredentials.NewCompCredStore(b.CredStoreGlobals.VaultKeypath, secureStorage)
			b.CredStoreGlobals.credStoreReady = true
			return
		}
	}

	return
}

func (b *VAULTv0) IsReady() bool {
	return b.CredStoreGlobals.credStoreReady
}

// Get the credentials for a specified xname. This will only go to vault if the
// credentials are unknown or have been cached for too long.
func (b *VAULTv0) GetCredentials(xname string) (user string, pw string, err error) {
	compType := base.GetHMSType(xname)
	if compType == base.HMSTypeInvalid {
		err = errors.New("Invalid xname for GetCredentials(), " + xname)
		return
	}

	if creds, ok := b.CredStoreGlobals.credsCacheMap[xname]; ok && creds.Expire.After(time.Now()) {
		// Use what is cached if we have something and it hasn't expired
		user = creds.User
		pw = creds.Pw
		return
	}
	// Get credentials from Vault
	if b.CredStoreGlobals.credStoreReady {
		credentials, credErr := b.CredStoreGlobals.credStore.GetCompCred(xname)
		if credErr != nil {
			err = credErr
			return
		} else {
			user = credentials.Username
			pw = credentials.Password
			expire := time.Duration(b.CredStoreGlobals.CredCacheDuration) * time.Second
			b.CredStoreGlobals.credsCacheMap[xname] = CompCredCached{
				User: user,
				Pw: pw,
				Expire: time.Now().Add(expire),
			}
		}
	} else {
		err = errors.New("Credentials store not ready")
	}
	return
}

// Get the credentials for the controller of a specified xname. This will only
// go to vault if the credentials are unknown or have been cached for too long.
func (b *VAULTv0) GetControllerCredentials(xname string) (user string, pw string, err error) {
	// Only get and cache controller credentials. This way, fewer vault calls are needed.
	compType := base.GetHMSType(xname)
	if compType == base.HMSTypeInvalid {
		err = errors.New("Invalid xname for GetControllerCredentials(), " + xname)
		return
	}

	// Loop until we get a BMC
	id := xname
	for {
		compType = base.GetHMSType(id)
		if compType == base.HMSTypeInvalid {
			// Ended up not finding a parent controller.
			// Just use the xname to query Vault
			id = xname
			break
		}
		if base.IsHMSTypeController(compType) {
			break
		} else {
			id = base.GetHMSCompParent(id)
		}
	}
	user, pw, err = b.GetCredentials(id)
	return
}
