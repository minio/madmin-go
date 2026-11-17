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
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// TestAccountSummaryRequest verifies that AccountSummary asks for version 2 of
// the account info API and decodes its three-field reply. The reply is a
// literal in the server's keys, so a renamed field fails here.
func TestAccountSummaryRequest(t *testing.T) {
	const reply = `{
		"AccountName": "alice",
		"Server": {"Type": 2, "GatewayOnline": false},
		"Policy": {"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:*"],"Resource":["arn:aws:s3:::*"]}]}
	}`

	type request struct {
		method string
		path   string
		query  url.Values
	}
	captured := make(chan request, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured <- request{r.Method, r.URL.Path, r.URL.Query()}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(reply))
	}))
	defer server.Close()

	adm, err := New(mustParseHost(t, server.URL), "access", "secret", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := adm.AccountSummary(context.Background())
	if err != nil {
		t.Fatalf("AccountSummary: %v", err)
	}

	req := <-captured
	if req.method != http.MethodGet {
		t.Errorf("method = %q, want GET", req.method)
	}
	if want := "/minio/admin/v4/accountinfo"; req.path != want {
		t.Errorf("path = %q, want %q", req.path, want)
	}
	if req.query.Get("v") != "2" {
		t.Errorf("v = %q, want 2", req.query.Get("v"))
	}
	if _, ok := req.query["prefix-usage"]; ok {
		t.Errorf("prefix-usage sent, want absent")
	}

	if got.AccountName != "alice" {
		t.Errorf("AccountName = %q, want alice", got.AccountName)
	}
	if got.Server.Type != 2 {
		t.Errorf("Server.Type = %v, want 2", got.Server.Type)
	}
	if len(got.Policy) == 0 || got.Policy[0] != '{' {
		t.Errorf("Policy = %q, want the raw policy object", got.Policy)
	}
}

// TestAccountSummaryError verifies that a non-200 reply becomes an error, as
// an older server that rejects the request would send.
func TestAccountSummaryError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	adm, err := New(mustParseHost(t, server.URL), "access", "secret", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := adm.AccountSummary(context.Background()); err == nil {
		t.Fatal("AccountSummary: got nil error for a 403 reply")
	}
}
