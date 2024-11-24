// MIT License
//
// (C) Copyright [2021,2024] Hewlett Packard Enterprise Development LP
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

package trs_http_api

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
)

type SerializedRequest struct {
	Method           string               `json:",omitempty"`
	URL              *url.URL             `json:",omitempty"`
	Proto            string               `json:",omitempty"` // "HTTP/1.0"
	ProtoMajor       int                  `json:",omitempty"` // 1
	ProtoMinor       int                  `json:",omitempty"` // 0
	Header           http.Header          `json:",omitempty"`
	Body             []byte               `json:",omitempty"`
	ContentLength    int64                `json:",omitempty"`
	TransferEncoding []string             `json:",omitempty"`
	Close            bool                 `json:",omitempty"`
	Host             string               `json:",omitempty"`
	Form             url.Values           `json:",omitempty"`
	PostForm         url.Values           `json:",omitempty"` // Go 1.1
	MultipartForm    *multipart.Form      `json:",omitempty"`
	Trailer          http.Header          `json:",omitempty"`
	RemoteAddr       string               `json:",omitempty"`
	RequestURI       string               `json:",omitempty"`
	TLS              *tls.ConnectionState `json:",omitempty"`
}

type HttpKafkaTx struct {
	ID          uuid.UUID
	Request     SerializedRequest `json:",omitempty"`
	TimeStamp   string            `json:",omitempty"` // Time the request time.Now().String()
	Timeout     time.Duration     `json:",omitempty"`
	CPolicy     ClientPolicy
	ServiceName string
	Ignore      bool
}

type HttpKafkaRx struct {
	ID       uuid.UUID // message ID
	Response *SerializedResponse
	Err      *error
}

type RetryPolicy struct {
	Retries        int
	BackoffTimeout time.Duration
}

type HttpTxPolicy struct {
	Enabled                 bool    // Enable or disable the policy
	MaxIdleConns            int
	MaxIdleConnsPerHost     int
	IdleConnTimeout         time.Duration
	ResponseHeaderTimeout   time.Duration
	TLSHandshakeTimeout     time.Duration
	DisableKeepAlives       bool
}

type ClientPolicy struct {
	Retry    RetryPolicy
	Tx       HttpTxPolicy
}

type HttpTask struct {
	id            uuid.UUID // message id, likely monotonically increasing
	ServiceName   string    //name of the service
	Request       *http.Request
	TimeStamp     string // Time the request was created/sent RFC3339Nano
	Err           *error
	Timeout       time.Duration	// task's context timeout
	CPolicy       ClientPolicy
	Ignore        bool
	context       context.Context
	contextCancel context.CancelFunc
	forceInsecure bool
}

type SerializedResponse struct {
	Status           string // e.g. "200 OK"
	StatusCode       int    // e.g. 200
	Proto            string // e.g. "HTTP/1.0"
	ProtoMajor       int    // e.g. 1
	ProtoMinor       int    // e.g. 0
	Header           http.Header
	Body             []byte
	ContentLength    int64
	TransferEncoding []string
	Close            bool
	Uncompressed     bool
	Trailer          http.Header
	TLS              *tls.ConnectionState
}

func (ht HttpTask) Validate() (valid bool, err error) {
	valid = false
	if ht.TimeStamp == "" {
		err = errors.New("Timstamp is empty")
		return false, err
	} else if ht.Request == nil {
		err = errors.New("Request is nil")
		return false, err
	} else if ht.ServiceName == "" {
		err = errors.New("ServiceName is empty")
		return false, err
	} else if ht.id == uuid.Nil {
		err = errors.New("ID is nil")
		return false, err
	}

	valid = true
	return valid, err
}

func (ht* HttpTask) GetID() (uuid uuid.UUID) {
	uuid = ht.id
	return uuid
}

func (ht* HttpTask) SetIDIfNotPopulated() (uuid.UUID) {
	if ht.id == uuid.Nil {
		ht.id = uuid.New()
	}
	return ht.id
}

func (ht HttpTask) ToHttpKafkaTx() (tx HttpKafkaTx) {
	//Fill the data
	tx.ID = ht.id
	tx.Timeout = ht.Timeout
	tx.CPolicy = ht.CPolicy
	tx.TimeStamp = ht.TimeStamp
	tx.Request = ToSerializedRequest(*ht.Request)
	tx.ServiceName = ht.ServiceName
	tx.Ignore = ht.Ignore

	return tx
}

func (tx HttpKafkaTx) ToHttpTask() (ht HttpTask) {
	//Fill the service data
	ht.id = tx.ID
	ht.ServiceName = tx.ServiceName
	ht.CPolicy = tx.CPolicy
	ht.Timeout = tx.Timeout
	ht.TimeStamp = tx.TimeStamp
	ht.Ignore = tx.Ignore

	req := tx.Request.ToHttpRequest()
	ht.Request = &req

	return ht
}

func ToSerializedRequest(req http.Request) (sr SerializedRequest) {

	sr.Method = req.Method
	sr.URL = req.URL
	sr.Proto = req.Proto
	sr.ProtoMajor = req.ProtoMajor
	sr.ProtoMinor = req.ProtoMinor
	sr.Header = req.Header
	if req.Body != nil {
		sr.Body, _ = ioutil.ReadAll(req.Body)
	}
	sr.ContentLength = req.ContentLength
	sr.TransferEncoding = req.TransferEncoding
	sr.Close = req.Close
	sr.Host = req.Host
	sr.Form = req.Form
	sr.PostForm = req.PostForm
	sr.MultipartForm = req.MultipartForm
	sr.Trailer = req.Trailer
	sr.RemoteAddr = req.RemoteAddr
	sr.RequestURI = req.RequestURI
	sr.TLS = req.TLS

	return sr
}

func (sr SerializedRequest) ToHttpRequest() (req http.Request) {

	//GRAB REQUEST DATA
	req.Method = sr.Method
	req.URL = sr.URL
	req.Proto = sr.Proto
	req.ProtoMajor = sr.ProtoMajor
	req.ProtoMinor = sr.ProtoMinor
	req.Header = sr.Header
	if sr.Body != nil {
		req.Body = ioutil.NopCloser(bytes.NewBuffer(sr.Body))
	}
	req.ContentLength = sr.ContentLength
	req.TransferEncoding = sr.TransferEncoding
	req.Close = sr.Close
	req.Host = sr.Host
	req.Form = sr.Form
	req.PostForm = sr.PostForm
	req.MultipartForm = sr.MultipartForm
	req.Trailer = sr.Trailer
	req.RemoteAddr = sr.RemoteAddr
	req.RequestURI = sr.RequestURI
	req.TLS = sr.TLS

	return req
}

func (ht HttpTask) ToHttpKafkaRx() (rx HttpKafkaRx) {
	//Fill the data
	rx.ID = ht.id
	if ht.Err != nil {
		rx.Err = ht.Err
	}
	if ht.Request.Response != nil {
		var res SerializedResponse
		tmp := ToSerializedResponse(*ht.Request.Response)
		res = tmp
		rx.Response = &res
	}
	return rx
}

func ToSerializedResponse(resp http.Response) (sr SerializedResponse) {
	sr.Status = resp.Status
	sr.StatusCode = resp.StatusCode
	sr.Proto = resp.Proto
	sr.ProtoMajor = resp.ProtoMajor
	sr.ProtoMinor = resp.ProtoMinor
	sr.Header = resp.Header
	if resp.Body != nil {
		sr.Body, _ = ioutil.ReadAll(resp.Body)
	}
	sr.ContentLength = resp.ContentLength
	sr.TransferEncoding = resp.TransferEncoding
	sr.Close = resp.Close
	sr.Uncompressed = resp.Uncompressed
	sr.Trailer = resp.Trailer
	sr.TLS = resp.TLS

	return sr
}

func (sr SerializedResponse) ToHttpResponse() (resp http.Response) {
	resp.Status = sr.Status
	resp.StatusCode = sr.StatusCode
	resp.Proto = sr.Proto
	resp.ProtoMajor = sr.ProtoMajor
	resp.ProtoMinor = sr.ProtoMinor
	resp.Header = sr.Header
	if sr.Body != nil {
		resp.Body = ioutil.NopCloser(bytes.NewBuffer(sr.Body))
	}
	resp.ContentLength = sr.ContentLength
	resp.TransferEncoding = sr.TransferEncoding
	resp.Close = sr.Close
	resp.Uncompressed = sr.Uncompressed
	resp.Trailer = sr.Trailer
	resp.TLS = sr.TLS
	return resp
}

func (sr SerializedResponse) Equal(sr1 SerializedResponse) (equal bool) {
	equal = false
	if sr.Status == sr1.Status &&
		sr.StatusCode == sr1.StatusCode &&
		sr.ProtoMajor == sr1.ProtoMajor &&
		sr.Proto == sr1.Proto &&
		sr.ProtoMinor == sr.ProtoMinor &&
		sr.ContentLength == sr1.ContentLength &&
		sr.Close == sr1.Close &&
		sr.Uncompressed == sr1.Uncompressed &&
		//sr.Body == sr1.Body &&
		//sr.TransferEncoding == sr1.TransferEncoding &&
		//sr.Header == sr1.Header &&
		//sr.Trailer == sr1.Trailer &&
		sr.TLS == sr1.TLS {
		equal = true
	}

	return equal
}

func (sr SerializedRequest) Equal(sr1 SerializedRequest) (equal bool) {
	equal = false
	if sr.URL == sr1.URL &&
		sr.ProtoMajor == sr1.ProtoMajor &&
		sr.Proto == sr1.Proto &&
		sr.ProtoMinor == sr.ProtoMinor &&
		sr.ContentLength == sr1.ContentLength &&
		sr.Close == sr1.Close &&
		sr.Method == sr1.Method &&
		//sr.PostForm == sr1.PostForm &&
		//sr.Form == sr1.Form &&
		//sr.Header == sr1.Header &&
		//sr.Trailer == sr1.Trailer &&
		//sr.Body == sr1.Body &&
		sr.Host == sr1.Host &&
		sr.RemoteAddr == sr1.RemoteAddr &&
		sr.RequestURI == sr1.RequestURI &&
		sr.TLS == sr1.TLS {
		equal = true
	}

	return equal
}
