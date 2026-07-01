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
	"strings"
	"testing"
)

func newSystemInventoryTestClient(t *testing.T, serverURL string) *AdminClient {
	t.Helper()
	client, err := New(mustParseHost(t, serverURL), "ak", "sk", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client
}

func TestSystemInventoryStatusRequest(t *testing.T) {
	var capturedMethod, capturedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"enabled":false,"buckets":[]}`))
	}))
	defer server.Close()

	if _, err := newSystemInventoryTestClient(t, server.URL).SystemInventoryStatus(context.Background()); err != nil {
		t.Fatalf("SystemInventoryStatus: %v", err)
	}

	if capturedMethod != http.MethodGet {
		t.Errorf("expected GET, got %s", capturedMethod)
	}
	if !strings.HasPrefix(capturedPath, "/minio/admin/") {
		t.Errorf("path missing /minio/admin/ prefix: %s", capturedPath)
	}
	if !strings.HasSuffix(capturedPath, "/system-inventory/status") {
		t.Errorf("path missing /system-inventory/status suffix: %s", capturedPath)
	}
}

func TestSystemInventoryStatusDecode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"enabled":true,"buckets":[{"bucket":"active","status":"Active"},{"bucket":"backfill","status":"Backfilling"},{"bucket":"off","status":"Disabled"}]}`))
	}))
	defer server.Close()

	got, err := newSystemInventoryTestClient(t, server.URL).SystemInventoryStatus(context.Background())
	if err != nil {
		t.Fatalf("SystemInventoryStatus: %v", err)
	}

	if !got.Enabled {
		t.Fatal("Enabled = false, want true")
	}
	if len(got.Buckets) != 3 {
		t.Fatalf("Buckets len = %d, want 3", len(got.Buckets))
	}
	want := []SystemInventoryBucketStatus{
		{Bucket: "active", Status: SystemInventoryActive},
		{Bucket: "backfill", Status: SystemInventoryBackfilling},
		{Bucket: "off", Status: SystemInventoryDisabled},
	}
	for i := range want {
		if got.Buckets[i] != want[i] {
			t.Errorf("Buckets[%d] = %+v, want %+v", i, got.Buckets[i], want[i])
		}
	}
}

func TestSystemInventoryStatusHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	if _, err := newSystemInventoryTestClient(t, server.URL).SystemInventoryStatus(context.Background()); err == nil {
		t.Fatal("expected error on 500, got nil")
	}
}
