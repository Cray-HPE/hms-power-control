/*
 * MIT License
 *
 * (C) Copyright [2022-2023] Hewlett Packard Enterprise Development LP
 *
 * Permission is hereby granted, free of charge, to any person obtaining a
 * copy of this software and associated documentation files (the "Software"),
 * to deal in the Software without restriction, including without limitation
 * the rights to use, copy, modify, merge, publish, distribute, sublicense,
 * and/or sell copies of the Software, and to permit persons to whom the
 * Software is furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included
 * in all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
 * THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR
 * OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
 * ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
 * OTHER DEALINGS IN THE SOFTWARE.
 */

package domain

import (
	"time"
	"sync"

	"github.com/Cray-HPE/hms-power-control/internal/credstore"
	"github.com/Cray-HPE/hms-power-control/internal/hsm"
	"github.com/Cray-HPE/hms-power-control/internal/logger"
	"github.com/Cray-HPE/hms-power-control/internal/storage"
	"github.com/Cray-HPE/hms-trs-app-api/pkg/trs_http_api"
	"github.com/Cray-HPE/hms-certs/pkg/hms_certs"
)

var GLOB *DOMAIN_GLOBALS

func Init(glob *DOMAIN_GLOBALS) {
	GLOB = glob
}

type DOMAIN_GLOBALS struct {
	CAUri             string
	BaseTRSTask       *trs_http_api.HttpTask
	RFTloc            *trs_http_api.TrsAPI
	HSMTloc           *trs_http_api.TrsAPI
	RFClientLock      *sync.RWMutex
	Running           *bool
	DSP               *storage.StorageProvider
	HSM               *hsm.HSMProvider
	RFHttpClient      *hms_certs.HTTPClientPair
	SVCHttpClient     *hms_certs.HTTPClientPair
	RFTransportReady  *bool
	VaultEnabled      bool
	CS                *credstore.CredStoreProvider
	DistLock          *storage.DistributedLockProvider
}

func (g *DOMAIN_GLOBALS) NewGlobals(base *trs_http_api.HttpTask,
                                    tlocRF *trs_http_api.TrsAPI,
                                    tlocSVC *trs_http_api.TrsAPI,
                                    clientRF *hms_certs.HTTPClientPair,
                                    clientSVC *hms_certs.HTTPClientPair,
                                    rfClientLock *sync.RWMutex,
                                    running *bool, dsp *storage.StorageProvider,
                                    hsm *hsm.HSMProvider, vaultEnabled bool,
                                    credStore *credstore.CredStoreProvider,
                                    distLock *storage.DistributedLockProvider) {
	g.BaseTRSTask = base
	g.RFTloc = tlocRF
	g.HSMTloc = tlocSVC
	g.RFHttpClient = clientRF
	g.SVCHttpClient = clientSVC
	g.RFClientLock = rfClientLock
	g.Running = running
	g.DSP = dsp
	g.HSM = hsm
	g.VaultEnabled = vaultEnabled
	g.CS = credStore
	g.DistLock = distLock
}

// Periodically runs functions to prune expired transitions and power-capping
// records and restart abandoned transitions.
func StartRecordsReaper() {
	go func() {
		logger.Log.Debug("Starting records reaper.")
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				transitionsReaper()
				powerCapReaper()
			}
		}
	}()
}
