//
// Copyright (c) 2015-2024 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.
//

package madmin

import (
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
)

/* **** SAMPLE ERROR RESPONSE ****
<?xml version="1.0" encoding="UTF-8"?>
<Error>
   <Code>AccessDenied</Code>
   <Message>Access Denied</Message>
   <BucketName>bucketName</BucketName>
   <Key>objectName</Key>
   <RequestId>F19772218238A85A</RequestId>
   <HostId>GuWkjyviSiGHizehqpmsD1ndz5NClSP19DOT+s2mv7gXGQ8/X1lhbDGiIJEXpGFD</HostId>
</Error>
*/

// ErrorResponse - Is the typed error returned by all API operations.
type ErrorResponse struct {
	XMLName    xml.Name `xml:"Error" json:"-"`
	Code       string
	Message    string
	BucketName string
	Key        string
	RequestID  string `xml:"RequestId"`
	HostID     string `xml:"HostId"`

	// Region where the bucket is located. This header is returned
	// only in HEAD bucket and ListObjects response.
	Region string

	// StatusCode is the HTTP status the error was answered with. It is zero
	// for an error no response carried. A 4xx says the server did not act on
	// the request.
	StatusCode int `xml:"-" json:"-"`
}

// Error - Returns HTTP error string
func (e ErrorResponse) Error() string {
	return e.Message
}

const (
	reportIssue = "Please report this issue at https://github.com/minio/minio/issues."
)

// httpRespToErrorResponse returns a new encoded ErrorResponse
// structure as error.
func httpRespToErrorResponse(resp *http.Response) error {
	if resp == nil || resp.Body == nil {
		msg := "Response is empty. " + reportIssue
		return ErrInvalidArgument(msg)
	}

	// Close without draining: decodeErrorResponse reads at most errorBodyLimit.
	defer resp.Body.Close()
	var errResp ErrorResponse
	if err := decodeErrorResponse(resp, &errResp, &errResp); err != nil {
		return err
	}
	errResp.StatusCode = resp.StatusCode
	return errResp
}

// errorBodyLimit is the most of an error body that is decoded.
const errorBodyLimit = 100 << 10

// decodeErrorResponse decodes the error body of resp as it streams: into
// jsonV when it is JSON, and into xmlV when it starts with '<', as MinIO's
// S3 errors do. Only the first 1 KiB is held, to quote in the error returned
// when the body fails to decode.
func decodeErrorResponse(resp *http.Response, jsonV any, xmlV *ErrorResponse) error {
	br := bufio.NewReader(io.LimitReader(resp.Body, errorBodyLimit))
	head, _ := br.Peek(1024)
	quoted := fmt.Sprintf("%q", head) // decoding reuses head's bytes
	var err error
	if trimmed := bytes.TrimLeft(head, " \t\r\n"); len(trimmed) > 0 && trimmed[0] == '<' {
		err = xml.NewDecoder(br).Decode(xmlV)
	} else {
		err = json.NewDecoder(br).Decode(jsonV)
	}
	if err != nil {
		return ErrorResponse{
			Code:       resp.Status,
			Message:    fmt.Sprintf("Failed to parse server response (%s): %s", err, quoted),
			StatusCode: resp.StatusCode,
		}
	}
	return nil
}

// ToErrorResponse - Returns parsed ErrorResponse struct from body and
// http headers.
//
// For example:
//
//	import admin "github.com/minio/madmin-go/v4"
//	...
//	...
//	ss, err := adm.ServiceStatus(...)
//	if err != nil {
//	   resp := admin.ToErrorResponse(err)
//	}
//	...
//
// The first ErrorResponse, or non-nil *ErrorResponse, in err's tree is
// returned.
func ToErrorResponse(err error) ErrorResponse {
	resp, _ := findErrorResponse(err)
	return resp
}

// findErrorResponse returns the first ErrorResponse or non-nil *ErrorResponse
// in err's tree, walked in the order errors.As uses.
func findErrorResponse(err error) (ErrorResponse, bool) {
	switch e := err.(type) {
	case ErrorResponse:
		return e, true
	case *ErrorResponse:
		if e != nil {
			return *e, true
		}
	}
	if x, ok := err.(interface{ As(any) bool }); ok {
		var resp ErrorResponse
		if x.As(&resp) {
			return resp, true
		}
		var ptr *ErrorResponse
		if x.As(&ptr) && ptr != nil {
			return *ptr, true
		}
	}
	switch x := err.(type) {
	case interface{ Unwrap() error }:
		return findErrorResponse(x.Unwrap())
	case interface{ Unwrap() []error }:
		for _, child := range x.Unwrap() {
			if resp, ok := findErrorResponse(child); ok {
				return resp, true
			}
		}
	}
	return ErrorResponse{}, false
}

// IsErrorCode reports whether the ErrorResponse that ToErrorResponse finds in
// err carries code.
func IsErrorCode(err error, code string) bool {
	return code != "" && ToErrorResponse(err).Code == code
}

// The error codes and messages the admin API answers with.
const (
	// AdminSelfLockoutErrorCode is the ErrorResponse.Code that the server
	// sends, with HTTP 403, when an IAM change would remove the caller's own
	// IAM admin access.
	AdminSelfLockoutErrorCode = "XMinioAdminSelfLockout"

	// AdminSelfLockoutMessage is the message the server sends with
	// AdminSelfLockoutErrorCode. A caller can show it when
	// ErrorResponse.Message is empty.
	AdminSelfLockoutMessage = "This operation would remove your own access"
)

// IsAdminSelfLockout reports whether err carries AdminSelfLockoutErrorCode.
func IsAdminSelfLockout(err error) bool {
	return IsErrorCode(err, AdminSelfLockoutErrorCode)
}

// ErrInvalidArgument - Invalid argument response.
func ErrInvalidArgument(message string) error {
	return ErrorResponse{
		Code:      "InvalidArgument",
		Message:   message,
		RequestID: "minio",
	}
}
