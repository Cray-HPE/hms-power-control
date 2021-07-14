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
	"testing"

	"github.com/stretchr/testify/suite"
)

type PassbackTS struct {
	suite.Suite
}

func (suite *PassbackTS) TestBuildErrorPassback() {
	var err error
	err = fmt.Errorf("ERROR: TestBuildError")
	pb := BuildErrorPassback(1200, err)
	suite.True(pb.IsError)
	suite.True(pb.StatusCode == 1200)
}

func (suite *PassbackTS) TestBuildSuccessPassback() {
	obj := []string{"a1", "a2", "a3"}
	pb := BuildSuccessPassback(200, obj)
	suite.False(pb.IsError)
	suite.True(pb.StatusCode == 200)
}

func TestPassbackSuite(t *testing.T) {

	suite.Run(t, new(PassbackTS))
}
