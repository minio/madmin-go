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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func newRunSnapshotExpiryTestClient(t *testing.T, serverURL string) *AdminClient {
	t.Helper()
	client, err := New(mustParseHost(t, serverURL), "ak", "sk", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client
}

// TestRunTableSnapshotExpiryRequest verifies the method, path, query parameters
// and JSON body sent for an on-demand run with both overrides set.
func TestRunTableSnapshotExpiryRequest(t *testing.T) {
	var (
		method string
		path   string
		query  url.Values
		body   RunTableSnapshotExpiryOptions
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path, query = r.Method, r.URL.Path, r.URL.Query()
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"snapshotsExpired":0}`))
	}))
	defer server.Close()

	maxAge, keep := 24, 3
	_, err := newRunSnapshotExpiryTestClient(t, server.URL).RunTableSnapshotExpiry(
		context.Background(), "wh", "ns.sub", "tbl",
		RunTableSnapshotExpiryOptions{MaxSnapshotAgeHours: &maxAge, MinSnapshotsToKeep: &keep})
	if err != nil {
		t.Fatalf("RunTableSnapshotExpiry: %v", err)
	}

	if method != http.MethodPost {
		t.Errorf("method = %s, want POST", method)
	}
	if !strings.HasPrefix(path, "/minio/admin/") || !strings.HasSuffix(path, "/tables/snapshot-expiry/run") {
		t.Errorf("unexpected path: %s", path)
	}
	if got := query.Get("warehouse"); got != "wh" {
		t.Errorf("warehouse = %q, want wh", got)
	}
	if got := query.Get("namespace"); got != "ns.sub" {
		t.Errorf("namespace = %q, want ns.sub", got)
	}
	if got := query.Get("table"); got != "tbl" {
		t.Errorf("table = %q, want tbl", got)
	}
	if body.MaxSnapshotAgeHours == nil || *body.MaxSnapshotAgeHours != maxAge {
		t.Errorf("MaxSnapshotAgeHours = %v, want %d", body.MaxSnapshotAgeHours, maxAge)
	}
	if body.MinSnapshotsToKeep == nil || *body.MinSnapshotsToKeep != keep {
		t.Errorf("MinSnapshotsToKeep = %v, want %d", body.MinSnapshotsToKeep, keep)
	}
}

// TestRunTableSnapshotExpiryOmitsUnsetOverrides verifies nil overrides are
// omitted from the body so the server applies its defaults.
func TestRunTableSnapshotExpiryOmitsUnsetOverrides(t *testing.T) {
	var rawBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		rawBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"snapshotsExpired":0}`))
	}))
	defer server.Close()

	if _, err := newRunSnapshotExpiryTestClient(t, server.URL).RunTableSnapshotExpiry(
		context.Background(), "wh", "ns", "tbl", RunTableSnapshotExpiryOptions{}); err != nil {
		t.Fatalf("RunTableSnapshotExpiry: %v", err)
	}
	if rawBody != "{}" {
		t.Errorf("body = %q, want {}", rawBody)
	}
}

// TestRunTableSnapshotExpiryDecode verifies the success response is decoded.
func TestRunTableSnapshotExpiryDecode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"snapshotsExpired":7}`))
	}))
	defer server.Close()

	got, err := newRunSnapshotExpiryTestClient(t, server.URL).RunTableSnapshotExpiry(
		context.Background(), "wh", "ns", "tbl", RunTableSnapshotExpiryOptions{})
	if err != nil {
		t.Fatalf("RunTableSnapshotExpiry: %v", err)
	}
	if got.SnapshotsExpired != 7 {
		t.Errorf("SnapshotsExpired = %d, want 7", got.SnapshotsExpired)
	}
}

// TestRunTableSnapshotExpiryError verifies a non-2xx response yields an error.
func TestRunTableSnapshotExpiryError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	if _, err := newRunSnapshotExpiryTestClient(t, server.URL).RunTableSnapshotExpiry(
		context.Background(), "wh", "ns", "tbl", RunTableSnapshotExpiryOptions{}); err == nil {
		t.Fatal("expected error on non-OK response, got nil")
	}
}
