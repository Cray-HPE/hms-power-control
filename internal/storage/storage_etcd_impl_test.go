// MIT License
//
// (C) Copyright [2024] Hewlett Packard Enterprise Development LP
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

func TestPageTasks(t *testing.T) {
	e := newETCDStorageForTesting()
	//
	transition := createTransition(50, 50)
	pageTasks := e.pageTasks(transition, 10)
	assertTrue(t, "Expected five pages", len(pageTasks) == 5, 5, len(pageTasks))
	expectedLength := 10
	for i, page := range pageTasks {
		assertTrue(t,
			fmt.Sprintf("Page %d has the wrong length for tasks,", i),
			len(page) == expectedLength,
			expectedLength,
			len(page))
	}

	pageTasks = e.pageTasks(transition, 100)
	assertTrue(t, "Expected only one page", len(pageTasks) == 1, 1, len(pageTasks))
	for i, page := range pageTasks {
		assertTrue(t,
			fmt.Sprintf("Page %d has the wrong length for tasks,", i),
			len(page) == 50,
			50,
			len(page))
	}

	transition = createTransition(51, 51)
	pageTasks = e.pageTasks(transition, 10)
	assertTrue(t, "Expected six pages", len(pageTasks) == 6, 6, len(pageTasks))
	for i, page := range pageTasks {
		if i == 5 {
			assertTrue(t,
				fmt.Sprintf("Page %d has the wrong length for tasks,", i),
				len(page) == 1,
				1,
				len(page))
		} else {
			assertTrue(t,
				fmt.Sprintf("Page %d has the wrong length for tasks,", i),
				len(page) == 10,
				10,
				len(page))
		}
	}

	var emptyTransition model.Transition
	pageTasks = e.pageTasks(emptyTransition, 10)
	assertTrue(t, "Expected zero pages for empty transition", len(pageTasks) == 0, 0, len(pageTasks))
}

func assertTrue(t *testing.T, message string, condition bool, expected interface{}, actual interface{}) {
	if !condition {
		t.Errorf("%s: Expected: %v, Actual: %v", message, expected, actual)
	}
}

func newETCDStorageForTesting() *ETCDStorage {
	etcdStorage := &ETCDStorage{
		Logger: logger.Log,
	}

	return etcdStorage
}

func createTransition(locationCount, taskCount int) model.Transition {
	var transition model.Transition
	for i := 0; i < locationCount; i++ {
		location := model.LocationParameter{Xname: fmt.Sprintf("x8000c0s0b0n%d", i)}
		transition.Location = append(transition.Location, location)
	}
	for i := 0; i < taskCount; i++ {
		task := model.TransitionTaskResp{}
		transition.Tasks = append(transition.Tasks, task)
	}
	return transition
}
