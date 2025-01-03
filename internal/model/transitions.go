/*
 * (C) Copyright [2021-2024] Hewlett Packard Enterprise Development LP
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
	"time"

	"github.com/google/uuid"
)

///////////////////////////
// Transitions Definitions
///////////////////////////

const (
	TransitionStatusNew           = "new"
	TransitionStatusInProgress    = "in-progress"
	TransitionStatusCompleted     = "completed"
	TransitionStatusAborted       = "aborted"
	TransitionStatusAbortSignaled = "abort-signaled"
)

const (
	TransitionTaskStatusNew         = "new"
	TransitionTaskStatusInProgress  = "in-progress"
	TransitionTaskStatusFailed      = "failed"
	TransitionTaskStatusSucceeded   = "succeeded"
	TransitionTaskStatusUnsupported = "unsupported"
)

const DefaultTaskDeadline = 5
const TransitionKeepAliveInterval = 10

///////////////////////////
//INPUT - Generally from the API layer
///////////////////////////

type TransitionParameter struct {
	Operation    string              `json:"operation"`
	TaskDeadline *int                `json:"taskDeadlineMinutes"`
	Location     []LocationParameter `json:"location"`
}

type LocationParameter struct {
	Xname     string `json:"xname"`
	DeputyKey string `json:"deputyKey,omitempty"`
}

func ToTransition(parameter TransitionParameter, expirationTimeMins int) (TR Transition, err error) {
	TR.TransitionID = uuid.New()
	TR.Operation, err = ToOperationFilter(parameter.Operation)
	if parameter.TaskDeadline != nil {
		TR.TaskDeadline = *parameter.TaskDeadline
	} else {
		TR.TaskDeadline = DefaultTaskDeadline
	}
	TR.Location = parameter.Location
	TR.CreateTime = time.Now()
	TR.AutomaticExpirationTime = time.Now().Add(time.Minute * time.Duration(expirationTimeMins))
	TR.LastActiveTime = time.Now()
	TR.Status = TransitionStatusNew
	TR.TaskIDs = []uuid.UUID{}
	return
}

//////////////
// INTERNAL - Generally passed around /internal/* packages
//////////////

type Transition struct {
	TransitionID            uuid.UUID           `json:"transitionID"`
	Operation               Operation           `json:"operation"`
	TaskDeadline            int                 `json:"taskDeadlineMinutes"`
	Location                []LocationParameter `json:"location"`
	CreateTime              time.Time           `json:"createTime"`
	LastActiveTime          time.Time           `json:"lastActiveTime"`
	AutomaticExpirationTime time.Time           `json:"automaticExpirationTime"`
	Status                  string              `json:"transitionStatus"`
	TaskIDs                 []uuid.UUID

	// Only populated when the task is completed
	IsCompressed bool                 `json:"isCompressed"`
	TaskCounts   TransitionTaskCounts `json:"taskCounts"`
	Tasks        []TransitionTaskResp `json:"tasks,omitempty"`
}

type TransitionPage struct {
	ID           string               `json:"ID"`
	TransitionID uuid.UUID            `json:"transitionID"`
	Index        int                  `json:"index"`
	Location     []LocationParameter  `json:"location"`
	Tasks        []TransitionTaskResp `json:"tasks,omitempty"`
	TaskIDs      []uuid.UUID
}

type TransitionTask struct {
	TaskID         uuid.UUID `json:"taskID"`
	TransitionID   uuid.UUID `json:"transitionID"`
	Operation      Operation `json:"operation"` // != Transition.Operation Tasks the redfish power command being issued (for recovery purposes)
	State          TaskState `json:"TaskState"`
	Xname          string    `json:"xname"`
	ReservationKey string    `json:"reservationKey,omitempty"`
	DeputyKey      string    `json:"deputyKey,omitempty"`
	Status         string    `json:"taskStatus"`
	StatusDesc     string    `json:"taskStatusDescription"`
	Error          string    `json:"error,omitempty"`
}

//////////////
// OUTPUT - Generally passed back to the API layer.
//////////////

type TransitionCreation struct {
	TransitionID uuid.UUID `json:"transitionID"`
	Operation    string    `json:"operation"`
}

type TransitionRespArray struct {
	Transitions []TransitionResp `json:"transitions"`
}

type TransitionResp struct {
	TransitionID            uuid.UUID            `json:"transitionID"`
	Operation               string               `json:"operation"`
	CreateTime              time.Time            `json:"createTime"`
	AutomaticExpirationTime time.Time            `json:"automaticExpirationTime"`
	TransitionStatus        string               `json:"transitionStatus"`
	TaskCounts              TransitionTaskCounts `json:"taskCounts"`
	Tasks                   []TransitionTaskResp `json:"tasks,omitempty"`
}

type TransitionTaskCounts struct {
	Total       int `json:"total"`
	New         int `json:"new"`
	InProgress  int `json:"in-progress"`
	Failed      int `json:"failed"`
	Succeeded   int `json:"succeeded"`
	Unsupported int `json:"un-supported"`
}

type TransitionTaskResp struct {
	Xname          string `json:"xname"`
	TaskStatus     string `json:"taskStatus"`
	TaskStatusDesc string `json:"taskStatusDescription"`
	Error          string `json:"error,omitempty"`
}

type TransitionAbortResp struct {
	AbortStatus string `json:"abortStatus"`
}

// Assembles a TransitionResp struct from a transition and an array of its tasks.
// If 'full' == true, full task information is included (xname, taskStatus, errors, etc).
func ToTransitionResp(transition Transition, tasks []TransitionTask, full bool) TransitionResp {
	// Build the response struct
	rsp := TransitionResp{
		TransitionID:            transition.TransitionID,
		Operation:               transition.Operation.String(),
		CreateTime:              transition.CreateTime,
		AutomaticExpirationTime: transition.AutomaticExpirationTime,
		TransitionStatus:        transition.Status,
	}

	// Is a compressed record
	if transition.IsCompressed {
		rsp.TaskCounts = transition.TaskCounts
		if full {
			rsp.Tasks = transition.Tasks
		}
		return rsp
	}

	counts := TransitionTaskCounts{}
	for _, task := range tasks {
		// Get the count of tasks with each status type.
		switch task.Status {
		case TransitionTaskStatusNew:
			counts.New++
		case TransitionTaskStatusInProgress:
			counts.InProgress++
		case TransitionTaskStatusFailed:
			counts.Failed++
		case TransitionTaskStatusSucceeded:
			counts.Succeeded++
		case TransitionTaskStatusUnsupported:
			counts.Unsupported++
		}
		counts.Total++
		// Include information about individual tasks if full == true
		if full {
			taskRsp := TransitionTaskResp{
				Xname:          task.Xname,
				TaskStatus:     task.Status,
				TaskStatusDesc: task.StatusDesc,
				Error:          task.Error,
			}
			rsp.Tasks = append(rsp.Tasks, taskRsp)
		}
	}
	rsp.TaskCounts = counts
	return rsp
}

//////////////
// FUNCTIONS
//////////////

func NewTransitionTask(transitionID uuid.UUID, op Operation) TransitionTask {
	return TransitionTask{
		TaskID:       uuid.New(),
		TransitionID: transitionID,
		Operation:    op,
		State:        TaskState_GatherData,
		Status:       TransitionTaskStatusNew,
	}
}

// ToOperationFilter - Will return a valid Operation from string
func ToOperationFilter(op string) (OP Operation, err error) {
	if len(op) == 0 {
		err = errors.New("invalid Operation type " + op)
		OP = Operation_Nil
		return
	}
	operation := strings.ToLower(op)
	switch operation {
	case "on":
		OP = Operation_On
		err = nil
	case "off":
		OP = Operation_Off
		err = nil
	case "soft-restart":
		OP = Operation_SoftRestart
		err = nil
	case "hard-restart":
		OP = Operation_HardRestart
		err = nil
	case "init":
		OP = Operation_Init
		err = nil
	case "force-off":
		OP = Operation_ForceOff
		err = nil
	case "soft-off":
		OP = Operation_SoftOff
		err = nil
	default:
		err = errors.New("invalid Operation type " + op)
		OP = Operation_Nil
	}
	return
}

// This pattern is from : https://yourbasic.org/golang/iota/
// I think the only think we ever have to really worry about is ever changing the order of this (add/remove/re-order)
type Operation int

const (
	Operation_Nil         Operation = iota - 1
	Operation_On                    // On = 0
	Operation_Off                   // 1 GracfulShutdown/Off->ForceOff
	Operation_SoftRestart           // 2 GracefulRestart->ForceRestart Or GracfulShutdown/Off->ForceOff->On
	Operation_HardRestart           // 3 GracfulShutdown/Off->ForceOff->On
	Operation_Init                  // 4 GracfulShutdown/Off->ForceOff->On does not require the initial power state to be "on"
	Operation_ForceOff              // 5 ForceOff
	Operation_SoftOff               // 6 GracfulShutdown/Off
)

func (op Operation) String() string {
	return [...]string{"On", "Off", "Soft-Restart", "Hard-Restart", "Init", "Force-Off", "Soft-Off"}[op]
}

func (op Operation) EnumIndex() int {
	return int(op)
}

type TaskState int

const (
	TaskState_Nil        TaskState = iota - 1
	TaskState_GatherData           // GatherData = 0
	TaskState_Sending              // 1 Command MAY have been sent. Can't confirm it was received.
	TaskState_Waiting              // 2 Command received. Waiting to confirm power state
	TaskState_Confirmed            // 3 Power state confirmed
)

func (ts TaskState) String() string {
	return [...]string{"Gathering Data", "Sending Command", "Waiting to Confirm", "Confirmed Transition", "Failed", "Complete"}[ts]
}

func (ts TaskState) EnumIndex() int {
	return int(ts)
}

func CopyTransition(transition Transition) Transition {
	result := transition

	result.Tasks = make([]TransitionTaskResp, 0)
	copy(result.Tasks, transition.Tasks)

	result.Location = make([]LocationParameter, 0)
	copy(result.Location, transition.Location)

	copy(result.TaskIDs, transition.TaskIDs)
	result.TaskIDs = make([]uuid.UUID, 0)

	return result

}
