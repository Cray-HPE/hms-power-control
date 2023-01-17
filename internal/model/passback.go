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

type Passback struct {
	StatusCode int
	Obj        interface{}
	Error      Problem7807
	IsError    bool
}

func BuildErrorPassback(StatusCode int, Err error) (passback Passback) {
	passback.StatusCode = StatusCode
	passback.Obj = nil
	passback.Error = GetFormattedErrorMessage(Err, StatusCode)
	passback.IsError = true
	return passback
}

func BuildSuccessPassback(StatusCode int, Obj interface{}) (passback Passback) {
	passback.StatusCode = StatusCode
	passback.Obj = Obj
	passback.IsError = false
	return passback
}
