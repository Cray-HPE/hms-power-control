/*
 * MIT License
 *
 * (C) Copyright [2021] Hewlett Packard Enterprise Development LP
 *
 * Permission is hereby granted, free of charge, to any person obtaining a
 * copy of this software and associated documentation files (the "Software"),
 * to deal in the Software without restriction, including without limitation
 * the rights to use, copy, modify, merge, publish, distribute, sublicense,
 * and/or sell copies of the Software, and to permit persons to whom the
 * Software is furnished to do so, subject to the following conditions:
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
 *
 */

package model

import (
	"errors"
	"github.com/google/uuid"
	"strings"
)

//INPUT
type TransitionParameter struct {
	Operation string              `json:"operation"`
	Location  []LocationParameter `json:"location"`
}

type Transition struct {
	Operation Operation           `json:"operation"`
	Location  []LocationParameter `json:"location"`
}

type LocationParameter struct {
	Xname     string    `json:"xname"`
	DeputyKey uuid.UUID `json:"deputyKey,omitempty"`
}

func ToTransition(parameter TransitionParameter) (TR Transition, err error) {
	TR.Location = parameter.Location
	TR.Operation, err = ToOperationFilter(parameter.Operation)
	return
}

//OUTPUT

type TransitionCreation struct {
	TransitionID uuid.UUID `json:"transitionID"`
	Operation    string    `json:"operation"`
}

// ToOperationFilter - Will return a valid Operation from string
func ToOperationFilter(op string) (OP Operation, err error) {
	if len(op) == 0 {
		err = errors.New("invalid Operation type " + op)
		OP = Operation_Nil
		return
	}
	if strings.ToLower(op) == "on" {
		OP = Operation_On
		err = nil
	} else if strings.ToLower(op) == "off" {
		OP = Operation_Off
		err = nil
	} else if strings.ToLower(op) == "softrestart" {
		OP = Operation_SoftRestart
		err = nil
	} else if strings.ToLower(op) == "hardrestart" {
		OP = Operation_HardRestart
		err = nil
	} else if strings.ToLower(op) == "init" {
		OP = Operation_Init
		err = nil
	} else if strings.ToLower(op) == "forceoff" {
		OP = Operation_ForceOff
		err = nil
	} else {
		err = errors.New("invalid Operation type " + op)
		OP = Operation_Nil
	}
	return
}

//This pattern is from : https://yourbasic.org/golang/iota/
//I think the only think we ever have to really worry about is ever changing the order of this (add/remove/re-order)
type Operation int

const (
	Operation_Nil         Operation = iota - 1
	Operation_On                    // On = 0
	Operation_Off                   //  1
	Operation_SoftRestart           // 2
	Operation_HardRestart           //  3
	Operation_Init                  // 4
	Operation_ForceOff              // 5
)

func (op Operation) String() string {
	return [...]string{"On", "Off", "SoftRestart", "HardRestart", "Init", "ForceOff"}[op]
}

func (op Operation) EnumIndex() int {
	return int(op)
}
