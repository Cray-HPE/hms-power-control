/*
 * (C) Copyright [2021-2022] Hewlett Packard Enterprise Development LP
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

package model

import (
	"errors"
	"strings"
)

//This pattern is from : https://yourbasic.org/golang/iota/
//I think the only think we ever have to really worry about is ever changing the order of this (add/remove/re-order)
type PowerStateFilter int

const (
	PowerStateFilter_Nil       PowerStateFilter = iota - 1
	PowerStateFilter_On                         // on = 0
	PowerStateFilter_Off                        //  1
	PowerStateFilter_Undefined                  // 2
)

// TODO need to sanitize input!
// READING -> http://brandonokert.com/articles/json-management-patterns-in-go/
// ToPowerStateFilter - Will return a valid PowerStateFilter from string
func ToPowerStateFilter(psf string) (PSF PowerStateFilter, err error) {

	if len(psf) == 0 {
		err = errors.New("invalid powerStateFilter type: " + psf)
		PSF = PowerStateFilter_Nil
		return
	}
	if strings.ToLower(psf) == "on" {
		PSF = PowerStateFilter_On
		err = nil
	} else if strings.ToLower(psf) == "off" {
		PSF = PowerStateFilter_Off
		err = nil
	} else if strings.ToLower(psf) == "undefined" {
		PSF = PowerStateFilter_Undefined
		err = nil
	} else {
		err = errors.New("invalid powerStateFilter type: " + psf)
		PSF = PowerStateFilter_Nil
	}
	return
}

func (psf PowerStateFilter) String() string {
	if (int(psf) < 0) {
		return "invalid"
	}
	return [...]string{"on", "off", "undefined"}[psf]
}

func (psf PowerStateFilter) EnumIndex() int {
	return int(psf)
}

type ManagementStateFilter int

const (
	ManagementStateFilter_Nil         ManagementStateFilter = iota - 1
	ManagementStateFilter_available                         // available = 0
	ManagementStateFilter_unavailable                       //  1
	ManagementStateFilter_undefined                         //  2
)

func ToManagementStateFilter(msf string) (MSF ManagementStateFilter, err error) {

	if len(msf) == 0 {
		err = nil
		MSF = ManagementStateFilter_Nil
		return
	}

	if strings.ToLower(msf) == "available" {
		MSF = ManagementStateFilter_available
		err = nil
	} else if strings.ToLower(msf) == "unavailable" {
		MSF = ManagementStateFilter_unavailable
		err = nil
	} else {
		err = errors.New("invalid ManagementStateFilter type " + msf)
		MSF = ManagementStateFilter_Nil
	}
	return
}

func (msf ManagementStateFilter) String() string {
	if (int(msf) < 0) {
		return "invalid"
	}
	return [...]string{"available", "unavailable", "undefined"}[msf]
}

//https://levelup.gitconnected.com/implementing-enums-in-golang-9537c433d6e2
func (msf ManagementStateFilter) EnumIndex() int {
	return int(msf)
}

type PowerStatusComponent struct {
	XName                     string   `json:"xname"`
	PowerState                string   `json:"powerState"`
	ManagementState           string   `json:"managementState"`
	Error                     string   `json:"error"`
	SupportedPowerTransitions []string `json:"supportedPowerTransitions"`
	LastUpdated               string   `json:"LastUpdated"` //RFC3339Nano
}

type PowerStatus struct {
	Status []PowerStatusComponent `json:"status"`
}
