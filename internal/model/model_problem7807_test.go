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
	"github.com/stretchr/testify/suite"
	"testing"
)

type Problem7807TS struct {
	suite.Suite
}

func (suite *Problem7807TS) TestProblem7807Equals_HappyPath() {

	var err1 Problem7807
	var err2 Problem7807

	err1 = GetFormattedErrorMessage(nil, 500)
	err2 = GetFormattedErrorMessage(nil, 500)

	suite.True(err1.Equals(err2))
	suite.True(err2.Equals(err1))
	suite.True(err1.Equals(err1))
	suite.True(err2.Equals(err2))

	errMsg := errors.New("Whoops, you slipped on a chip!")

	err1 = GetFormattedErrorMessage(errMsg, 500)
	err2 = GetFormattedErrorMessage(errMsg, 500)

	suite.True(err1.Equals(err2))
	suite.True(err2.Equals(err1))
	suite.True(err1.Equals(err1))
	suite.True(err2.Equals(err2))

}

func (suite *Problem7807TS) TestProblem7807Equals_NotEqual() {

	var err1 Problem7807
	var err2 Problem7807
	errMsg := errors.New("Whoops, you slipped on a chip!")

	err1 = GetFormattedErrorMessage(errMsg, 500)
	err2 = GetFormattedErrorMessage(nil, 500)

	suite.False(err1.Equals(err2))
	suite.False(err2.Equals(err1))
	suite.True(err1.Equals(err1))
	suite.True(err2.Equals(err2))

}

func TestModelProblem7807Suite(t *testing.T) {

	suite.Run(t, new(Problem7807TS))
}
