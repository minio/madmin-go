//
// Copyright (c) 2015-2022 MinIO, Inc.
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
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"unicode/utf8"
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

	defer closeResponse(resp)
	// Limit to 100K
	body, err := ioutil.ReadAll(io.LimitReader(resp.Body, 100<<10))
	if err != nil {
		return ErrorResponse{
			Code:    resp.Status,
			Message: fmt.Sprintf("Failed to read server response: %s.", err),
		}
	}

	var errResp ErrorResponse
	// Decode the json error
	err = json.Unmarshal(body, &errResp)
	if err != nil {
		// We might get errors as XML, try that.
		xmlErr := xml.Unmarshal(body, &errResp)

		if xmlErr != nil {
			bodyString := string(body)
			if !utf8.Valid(body) {
				bodyString = hex.EncodeToString(body)
			}
			if len(bodyString) > 1024 {
				bodyString = bodyString[:1021] + "..."
			}
			return ErrorResponse{
				Code:    resp.Status,
				Message: fmt.Sprintf("Failed to parse server response (%s): %s", err.Error(), bodyString),
			}
		}
	}
	return errResp
}

// ToErrorResponse - Returns parsed ErrorResponse struct from body and
// http headers.
//
// For example:
//
//	import admin "github.com/minio/madmin-go/v3"
//	...
//	...
//	ss, err := adm.ServiceStatus(...)
//	if err != nil {
//	   resp := admin.ToErrorResponse(err)
//	}
//	...
func ToErrorResponse(err error) ErrorResponse {
	switch err := err.(type) {
	case ErrorResponse:
		return err
	default:
		return ErrorResponse{}
	}
}

// ErrInvalidArgument - Invalid argument response.
func ErrInvalidArgument(message string) error {
	return ErrorResponse{
		Code:      "InvalidArgument",
		Message:   message,
		RequestID: "minio",
	}
}
