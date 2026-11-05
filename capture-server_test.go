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
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type capturedRequest struct {
	method string
	path   string
	query  string
	body   []byte
}

// newCaptureServer returns a client of a test server that answers every
// request with status and respBody, and a channel that receives each request.
func newCaptureServer(t *testing.T, status int, respBody string) (*AdminClient, <-chan capturedRequest) {
	t.Helper()
	ch := make(chan capturedRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		ch <- capturedRequest{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: body}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, respBody)
	}))
	t.Cleanup(server.Close)

	adm, err := New(mustParseHost(t, server.URL), "access", "secret", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return adm, ch
}
