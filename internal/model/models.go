/*
 * MIT License
 *
 * (C) Copyright [2020-2021] Hewlett Packard Enterprise Development LP
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
	"fmt"

	"github.com/google/uuid"
)

type IDList struct {
	IDs []uuid.UUID `json:"ids"`
}

type IDResp struct {
	ID uuid.UUID `json:"id"`
}

func UUIDSliceEquals(obj []uuid.UUID, other []uuid.UUID) bool {
	if len(obj) != len(other) {
		return false
	}
	objMap := make(map[uuid.UUID]int)
	otherMap := make(map[uuid.UUID]int)

	for _, objE := range obj {
		objMap[objE]++
	}
	for _, otherE := range other {
		otherMap[otherE]++
	}
	for objKey, objVal := range objMap {
		if otherMap[objKey] != objVal {
			return false
		}
	}
	return true
}

func StringSliceEquals(obj []string, other []string) bool {
	if len(obj) != len(other) {
		return false
	} else if len(obj) == 0 && len(other) == 0 {
		return true
	} else if obj == nil && other == nil {
		return true
	}
	objMap := make(map[string]int)
	otherMap := make(map[string]int)

	for _, objE := range obj {
		objMap[objE]++
	}
	for _, otherE := range other {
		otherMap[otherE]++
	}
	for objKey, objVal := range objMap {
		if otherMap[objKey] != objVal {
			return false
		}
	}
	return true
}

func NewInvalidInputError(Message string, CompIDs []string) (err error) {
	err = fmt.Errorf("%s: %v", Message, CompIDs)
	return err
}


func Find(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}
