// MIT License
// 
// (C) Copyright [2022-2023] Hewlett Packard Enterprise Development LP
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

package hsm

import (
	"github.com/sirupsen/logrus"
	"github.com/Cray-HPE/hms-certs/pkg/hms_certs"
	"github.com/Cray-HPE/hms-base"
	reservation "github.com/Cray-HPE/hms-smd/v2/pkg/service-reservations"
)

type HSM_GLOBALS struct {
	SvcName           string
	Logger            *logrus.Logger
	Reservation       *reservation.Production
	Running           *bool
	LockEnabled       bool
	SMUrl             string
	SVCHttpClient     *hms_certs.HTTPClientPair
	MaxComponentQuery int
}

// There are 2 modes of operation for locking.  One is that we use a deputy
// key passed into the PCS API.  In that case we don't have the reservation
// key, just the deputy key.  The other is that we are not given a deputy key,
// and we get the reservation ourselves.  In that case we get both the 
// reservation key and a deputy key.  We must release the reservation for these
// when we're done.  In any given ReservationData object, if the ReservationKey
// field is not empty, that tells us we got the lock ourselves.

type ReservationData struct {
	XName            string
	ReservationOwner bool	//true == we got the rsv, not just using deputy key
	ReservationKey   string	//only valid if we had to get the reservation
	ExpirationTime   string	//Ditto.
	DeputyKey        string	//Can be empty, filled in when getting reservation
	Error            error

	//Private stuff, used only internally
	needRsv      bool
}

type HsmData struct {
	BaseData         base.Component `json:"baseData"`

	RfFQDN           string   `json:"RfFQDN"`
	PowerActionURI   string   `json:"actionURI"`
	AllowableActions []string `json:"allowableActions"`
	PowerStatusURI   string   `json:"statusURI"`
	Error            error    `json:"error"`

// For PCS power capping
	PowerCapURI           string              `json:"powerCapURI"`
	PowerCapTargetURI     string              `json:"powerCapTargetURI"`
	PowerCapControlsCount int                 `json:"powerCapControlsCount"`
	PowerCapCtlInfoCount  int                 `json:"powerCapCtlInfoCount"`
	PowerCaps             map[string]PowerCap `json:"powerCaps"`

// For PCS transitions
	PoweredBy []string
}

// PowerCap defines values useful to PCS for power capping.
type PowerCap struct {
	Name        string
	Path        string
	Min         int
	Max         int
	PwrCtlIndex int
}
