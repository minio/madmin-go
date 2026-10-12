//
// Copyright (c) 2015-2026 MinIO, Inc.
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
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// asErrorResponse is an error whose custom As method, not its Unwrap chain,
// yields an ErrorResponse.
type asErrorResponse struct{ resp ErrorResponse }

func (e asErrorResponse) Error() string { return "custom: " + e.resp.Message }

func (e asErrorResponse) As(target any) bool {
	t, ok := target.(*ErrorResponse)
	if ok {
		*t = e.resp
	}
	return ok
}

func TestToErrorResponse(t *testing.T) {
	lockout := ErrorResponse{Code: AdminSelfLockoutErrorCode, Message: AdminSelfLockoutMessage}
	other := ErrorResponse{Code: "XMinioAdminNoSuchUser", Message: "no user"}
	var nilResp *ErrorResponse

	for _, tc := range []struct {
		name string
		err  error
		want ErrorResponse
	}{
		{"nil", nil, ErrorResponse{}},
		{"plain error", errors.New("plain error"), ErrorResponse{}},
		{"value", lockout, lockout},
		{"pointer", &lockout, lockout},
		{"nil pointer", nilResp, ErrorResponse{}},
		{"wrapped value", fmt.Errorf("remove user: %w", lockout), lockout},
		{"wrapped pointer", fmt.Errorf("remove user: %w", &lockout), lockout},
		{"custom As", asErrorResponse{lockout}, lockout},
		{"joined", errors.Join(errors.New("plain error"), lockout), lockout},
		{"joined takes the first", errors.Join(other, lockout), other},
		{"joined pointer before value", errors.Join(&other, lockout), other},
		{"joined value before pointer", errors.Join(other, &lockout), other},
		{"joined after a nil pointer", errors.Join(nilResp, &lockout), lockout},
		{"nested", fmt.Errorf("outer: %w", errors.Join(errors.New("plain"), fmt.Errorf("inner: %w", &lockout))), lockout},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ToErrorResponse(tc.err); got != tc.want {
				t.Errorf("ToErrorResponse = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestIsAdminSelfLockout(t *testing.T) {
	lockout := ErrorResponse{Code: AdminSelfLockoutErrorCode, Message: AdminSelfLockoutMessage}
	other := ErrorResponse{Code: "XMinioAdminNoSuchUser", Message: "no user"}
	var nilResp *ErrorResponse

	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain error", errors.New("plain error"), false},
		{"nil pointer", nilResp, false},
		{"other code", other, false},
		{"other code pointer", &other, false},
		{"value", lockout, true},
		{"pointer", &lockout, true},
		{"wrapped value", fmt.Errorf("remove user: %w", lockout), true},
		{"wrapped pointer", fmt.Errorf("remove user: %w", &lockout), true},
		{"custom As", asErrorResponse{lockout}, true},
		{"joined after a plain error", errors.Join(errors.New("plain error"), nilResp, lockout), true},
		{"joined after another code", errors.Join(other, lockout), false},
		{"joined after another code pointer", errors.Join(&other, lockout), false},
		{"joined pointer first", errors.Join(&lockout, other), true},
		{"nested", fmt.Errorf("outer: %w", errors.Join(errors.New("plain"), fmt.Errorf("inner: %w", &lockout))), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsAdminSelfLockout(tc.err); got != tc.want {
				t.Errorf("IsAdminSelfLockout = %v, want %v", got, tc.want)
			}
			if got := ToErrorResponse(tc.err).Code == AdminSelfLockoutErrorCode; got != tc.want {
				t.Errorf("ToErrorResponse(err).Code is self-lockout = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsErrorCodeEmptyCode(t *testing.T) {
	if IsErrorCode(errors.New("plain error"), "") {
		t.Error("IsErrorCode matched an empty code on an error with no ErrorResponse")
	}
}

// TestAdminSelfLockoutResponse verifies that the 403 JSON body the server
// sends for a self-lockout refusal reaches the caller as an ErrorResponse that
// IsAdminSelfLockout recognizes, with the server's message intact.
func TestAdminSelfLockoutResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		if _, err := io.WriteString(w, `{"Code":"XMinioAdminSelfLockout",`+
			`"Message":"This operation would remove your own access",`+
			`"Resource":"/minio/admin/v4/remove-user","RequestId":"18B3D491BB8DCD15","HostId":"dd9025ba"}`); err != nil {
			t.Errorf("write reply: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client, err := New(mustParseHost(t, server.URL), "ak", "sk", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	err = fmt.Errorf("remove user: %w", client.RemoveUser(context.Background(), "alice"))

	if !IsAdminSelfLockout(err) {
		t.Fatalf("IsAdminSelfLockout(%v) = false, want true", err)
	}
	resp := ToErrorResponse(err)
	if resp.Message != AdminSelfLockoutMessage || resp.RequestID != "18B3D491BB8DCD15" {
		t.Errorf("ToErrorResponse = %+v, want the server's message and request id", resp)
	}
}

// countingReader counts the bytes read from it.
type countingReader struct {
	r io.Reader
	n int
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += n
	return n, err
}

// TestHTTPRespToErrorResponse verifies that an error body is decoded as JSON,
// or as XML when it starts with '<', and that a body that is neither is quoted
// in the error, cut to 1024 bytes.
func TestHTTPRespToErrorResponse(t *testing.T) {
	long := strings.Repeat("x", 2000)
	for _, tc := range []struct {
		body    string
		code    string
		message string
	}{
		{`{"Code": "XMinioAdminNoSuchUser", "Message": "no user"}`, "XMinioAdminNoSuchUser", "no user"},
		{"\n  <?xml version=\"1.0\"?><Error><Code>AccessDenied</Code><Message>denied</Message></Error>", "AccessDenied", "denied"},
		{`gateway timeout`, "504 Gateway Timeout", "Failed to parse server response (invalid character 'g' looking for beginning of value): gateway timeout"},
		{``, "504 Gateway Timeout", "Failed to parse server response (EOF): "},
		{long, "504 Gateway Timeout", "Failed to parse server response (invalid character 'x' looking for beginning of value): " + long[:1021] + "..."},
	} {
		resp := &http.Response{
			Status: "504 Gateway Timeout", StatusCode: http.StatusGatewayTimeout,
			Body: io.NopCloser(strings.NewReader(tc.body)),
		}
		got := ToErrorResponse(httpRespToErrorResponse(resp))
		if got.Code != tc.code || got.Message != tc.message || got.StatusCode != http.StatusGatewayTimeout {
			t.Errorf("%.40q: got %+v, want code %q, message %q", tc.body, got, tc.code, tc.message)
		}
	}
}

// TestDecodeErrorResponseLimit verifies that no more than errorBodyLimit bytes
// of an error body are decoded.
func TestDecodeErrorResponseLimit(t *testing.T) {
	body := &countingReader{r: strings.NewReader(`{"Message": "` + strings.Repeat("x", 4*errorBodyLimit) + `"}`)}
	resp := &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(body)}
	var errResp ErrorResponse
	if err := decodeErrorResponse(resp, &errResp, &errResp); err == nil {
		t.Fatal("a body cut at the limit should fail to decode")
	}
	if body.n > errorBodyLimit {
		t.Errorf("read %d bytes, want at most %d", body.n, errorBodyLimit)
	}
}
