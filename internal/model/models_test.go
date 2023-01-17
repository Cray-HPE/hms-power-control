/*
 * (C) Copyright [2021-2023] Hewlett Packard Enterprise Development LP
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
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

type ModelsTS struct {
	suite.Suite
}

func (suite *ModelsTS) TestUUIDSliceEquals() {
	u1 := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	u2 := []uuid.UUID{uuid.New(), uuid.New()}
	u3 := []uuid.UUID{uuid.New(), uuid.New()}
	suite.True(UUIDSliceEquals(u1, u1))
	suite.True(UUIDSliceEquals(u2, u2))
	suite.False(UUIDSliceEquals(u1, u2))
	suite.False(UUIDSliceEquals(u3, u2))
}

func (suite *ModelsTS) TestStringSliceEquals() {
	s1 := []string{"a1", "a2", "a3"}
	s2 := []string{"a3", "a2", "a1"}
	s3 := []string{"a3", "a2"}
	s4 := []string{"a3", "a2", "a4"}
	suite.True(StringSliceEquals(s1, s2))
	suite.True(StringSliceEquals(s2, s1))
	suite.False(StringSliceEquals(s3, s1))
	suite.False(StringSliceEquals(s3, s4))
	suite.False(StringSliceEquals(s1, s4))
}

func (suite *ModelsTS) TestNewInvalidInputError() {
	err := NewInvalidInputError("Message", []string{"a1", "a2", "a3"})
	suite.True(strings.Contains(err.Error(), "Message"))
	suite.True(strings.Contains(err.Error(), "[a1 a2 a3]"))
}

func TestModelsSuite(t *testing.T) {

	suite.Run(t, new(ModelsTS))
}
