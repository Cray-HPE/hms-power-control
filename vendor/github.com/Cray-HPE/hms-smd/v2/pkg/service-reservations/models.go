// MIT License
//
// (C) Copyright [2020-2021] Hewlett Packard Enterprise Development LP
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

package service_reservations

const (
	CLProcessingModelRigid = "rigid"
	CLProcessingModelFlex  = "flexible"
)

//////////////////////////////////////////////
// Responses
//////////////////////////////////////////////

// Create (Serv)Res
type ReservationCreateSuccessResponse struct {
	ID             string `json:"ID"`
	DeputyKey      string `json:"DeputyKey"`
	ReservationKey string `json:"ReservationKey,omitempty"`
	ExpirationTime string `json:"ExpirationTime,omitempty"`
}

// Create (Serv)Res
type ReservationCheckSuccessResponse struct {
	ID             string `json:"ID"`
	DeputyKey      string `json:"DeputyKey"`
	ExpirationTime string `json:"ExpirationTime,omitempty"`
}

type FailureResponse struct {
	ID     string `json:"ID"`
	Reason string `json:"Reason"`
}
type ReservationCreateResponse struct {
	Success []ReservationCreateSuccessResponse `json:"Success"`
	Failure []FailureResponse                  `json:"Failure"`
}

type ReservationCheckResponse struct {
	Success []ReservationCheckSuccessResponse `json:"Success"`
	Failure []FailureResponse                 `json:"Failure"`
}

// Renew/Release ServRes, Release/Remove Res, Create/Unlock/Repair/Disable locks
type Counts struct {
	Total   int `json:"Total"`
	Success int `json:"Success"`
	Failure int `json:"Failure"`
}

type ComponentIDs struct {
	ComponentIDs []string `json:"ComponentIDs"`
}

// RELEASE/RENEW RESPONSE

type ReservationReleaseRenewResponse struct {
	Counts  Counts            `json:"Counts"`
	Success ComponentIDs      `json:"Success"`
	Failure []FailureResponse `json:"Failure"`
}

// RFC 7807 compliant error payload.  All fields are optional except the 'type' field.
type Problem7807 struct {
	Type_    string `json:"type"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
	Status   int    `json:"status,omitempty"`
	Title    string `json:"title,omitempty"`
}

//////////////////////////////////////////////
// Parameters
//////////////////////////////////////////////

// Create/Remove Res, Create ServRes, Check/Lock/Unlock/Repair/Disable Lock
type ReservationCreateParameters struct {
	ID                  []string `json:"ComponentIDs,omitempty"`
	NID                 []string `json:"nid,omitempty"`
	Type                []string `json:"type,omitempty"`
	State               []string `json:"state,omitempty"`
	Flag                []string `json:"flag,omitempty"`
	Enabled             []string `json:"enabled,omitempty"`
	SwStatus            []string `json:"softwarestatus,omitempty"`
	Role                []string `json:"role,omitempty"`
	SubRole             []string `json:"subrole,omitempty"`
	Subtype             []string `json:"subtype,omitempty"`
	Arch                []string `json:"arch,omitempty"`
	Class               []string `json:"class,omitempty"`
	Group               []string `json:"group,omitempty"`
	Partition           []string `json:"partition,omitempty"`
	ProcessingModel     string   `json:"ProcessingModel"`
	ReservationDuration int      `json:"ReservationDuration"`
}

// Release Res, Release/Renew ServRes
type Key struct {
	ID  string `json:"ID"`
	Key string `json:"Key"`
}
type ReservationRenewalParameters struct { // RENEWAL INPUT
	ReservationKeys     []Key  `json:"ReservationKeys"`
	ProcessingModel     string `json:"ProcessingModel"`
	ReservationDuration int    `json:"ReservationDuration"`
}

type ReservationReleaseParameters struct { // RELEASE INPUT
	ReservationKeys []Key  `json:"ReservationKeys"`
	ProcessingModel string `json:"ProcessingModel"`
}

// Check ServRes
type ReservationCheckParameters struct { // CHECK INPUT
	DeputyKeys []Key `json:"DeputyKeys"`
}
