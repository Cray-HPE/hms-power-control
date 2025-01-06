// MIT License
//
// (C) Copyright [2024-2025] Hewlett Packard Enterprise Development LP
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

package storage

import (
	"fmt"
	"testing"

	"github.com/Cray-HPE/hms-power-control/internal/logger"
	"github.com/Cray-HPE/hms-power-control/internal/model"
)

func TestPageLocations(t *testing.T) {
	e := newETCDStorageForTesting(t)

	transition := newTransitionWithTasks("x1000c0s0b0", 50, 50)
	e.PageSize = 10
	pageLocations := e.pageLocations(transition, e.PageSize)
	if len(pageLocations) != 5 {
		t.Error(tmessage("Expected five pages", 5, len(pageLocations)))
	}
	expectedLength := 10
	for i, page := range pageLocations {
		if len(page) != expectedLength {
			t.Error(tmessage(
				fmt.Sprintf("Page %d has the wrong length for locations,", i),
				expectedLength,
				len(page)))
		}
	}

	e.PageSize = 100
	pageLocations = e.pageLocations(transition, e.PageSize)
	if len(pageLocations) != 1 {
		t.Error(tmessage("Expected only one page", 1, len(pageLocations)))
	}
	for i, page := range pageLocations {
		if len(page) != 50 {
			t.Error(tmessage(
				fmt.Sprintf("Page %d has the wrong length for locations,", i),
				50,
				len(page)))
		}
	}

	transition = newTransitionWithTasks("x1001c0s0b0", 51, 51)
	e.PageSize = 10
	pageLocations = e.pageLocations(transition, e.PageSize)
	if len(pageLocations) != 6 {
		t.Error(tmessage("Expected six pages", 6, len(pageLocations)))
	}
	for i, page := range pageLocations {
		if i == 5 {
			if len(page) != 1 {
				t.Error(tmessage(
					fmt.Sprintf("Page %d has the wrong length for locations,", i),
					1,
					len(page)))
			}
		} else {
			if len(page) != 10 {
				t.Error(tmessage(
					fmt.Sprintf("Page %d has the wrong length for locations,", i),
					10,
					len(page)))
			}
		}
	}

	var emptyTransition model.Transition
	e.PageSize = 10
	pageLocations = e.pageLocations(emptyTransition, e.PageSize)
	if len(pageLocations) != 0 {
		t.Error(tmessage("Expected zero pages for empty transition", 0, len(pageLocations)))
	}
}

func TestPageTasks(t *testing.T) {
	e := newETCDStorageForTesting(t)

	transition := newTransitionWithTasks("x2000c0s0b0", 50, 50)
	e.PageSize = 10
	pageTasks := e.pageTasks(transition, e.PageSize)
	if len(pageTasks) != 5 {
		t.Error(tmessage("Expected five pages", 5, len(pageTasks)))
	}
	expectedLength := 10
	for i, page := range pageTasks {
		if len(page) != expectedLength {
			t.Error(tmessage(
				fmt.Sprintf("Page %d has the wrong length for tasks,", i),
				expectedLength,
				len(page)))
		}
	}

	e.PageSize = 100
	pageTasks = e.pageTasks(transition, e.PageSize)
	if len(pageTasks) != 1 {
		t.Error(tmessage("Expected only one page", 1, len(pageTasks)))
	}
	for i, page := range pageTasks {
		if len(page) != 50 {
			t.Error(tmessage(
				fmt.Sprintf("Page %d has the wrong length for tasks,", i),
				50,
				len(page)))
		}
	}

	transition = newTransitionWithTasks("x2001c0s0b0", 51, 51)
	e.PageSize = 10
	pageTasks = e.pageTasks(transition, e.PageSize)
	if len(pageTasks) != 6 {
		t.Error(tmessage("Expected six pages", 6, len(pageTasks)))
	}
	for i, page := range pageTasks {
		if i == 5 {
			if len(page) != 1 {
				t.Error(tmessage(
					fmt.Sprintf("Page %d has the wrong length for tasks,", i),
					1,
					len(page)))
			}
		} else {
			if len(page) != 10 {
				t.Error(tmessage(
					fmt.Sprintf("Page %d has the wrong length for tasks,", i),
					10,
					len(page)))
			}
		}
	}

	var emptyTransition model.Transition
	e.PageSize = 10
	pageTasks = e.pageTasks(emptyTransition, e.PageSize)

	if len(pageTasks) != 0 {
		t.Error(tmessage("Expected zero pages for empty transition", 0, len(pageTasks)))
	}
}

func tmessage(message string, expected interface{}, actual interface{}) string {
	return fmt.Sprintf("%s: Expected: %v, Actual: %v", message, expected, actual)
}

func newETCDStorageForTesting(t *testing.T) *ETCDStorage {
	t.Logf("Using empty etcdStorage.")
	etcdStorage := &ETCDStorage{
		Logger: logger.Log,
	}
	return etcdStorage
}

func newTransitionWithTasks(xnamePrefix string, locationCount, taskCount int) model.Transition {
	transition := newTransition(newParameters(xnamePrefix, locationCount, "on"))
	for i := 0; i < taskCount; i++ {
		task := model.TransitionTaskResp{
			Xname: fmt.Sprintf("%s%d", xnamePrefix, i),
		}
		transition.Tasks = append(transition.Tasks, task)
	}
	return transition
}
