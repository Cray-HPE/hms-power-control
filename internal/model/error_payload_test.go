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
	"errors"
	"github.com/stretchr/testify/suite"
	"net/http"
	"testing"
)

type ErrorPayloadTS struct {
	suite.Suite
}

func (suite *ErrorPayloadTS) TestErrorPayload_Equality() {

	errMsg := errors.New("Whoops, you slipped on a chip!")
	var err1 Problem7807
	err1.Status = http.StatusNotFound
	err1.Type_ = "about:blank"
	err1.Instance = ""
	err1.Detail = errMsg.Error()
	err1.Title = http.StatusText(http.StatusNotFound)

	err2 := GetFormattedErrorMessage(errMsg, http.StatusNotFound)

	suite.True(err1.Equals(err2))
	suite.True(err1.Equals(err1))
	suite.True(err2.Equals(err2))
	suite.True(err2.Equals(err1))

}

func (suite *ErrorPayloadTS) TestErrorPayload_Nil() {

	errMsg := errors.New("unknown error - could not parse from error object")
	var err1 Problem7807
	err1.Status = http.StatusNotFound
	err1.Type_ = "about:blank"
	err1.Instance = ""
	err1.Detail = errMsg.Error()
	err1.Title = http.StatusText(http.StatusNotFound)

	err2 := GetFormattedErrorMessage(nil, http.StatusNotFound)

	suite.True(err1.Equals(err2))
	suite.True(err1.Equals(err1))
	suite.True(err2.Equals(err2))
	suite.True(err2.Equals(err1))

}

func TestModelErrorPayloadSuite(t *testing.T) {

	suite.Run(t, new(ErrorPayloadTS))
}
