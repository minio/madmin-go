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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListDistJobStatuses(t *testing.T) {
	want := []DistJobLeaderStatus{
		{
			JobID:   "job-1",
			JobType: DistJobTypeDecommission,
			PoolIdx: 1,
			Nodes: []DistJobNodeStatus{
				{Host: "node1:9000", IsLocal: true, Online: true, ItemsDone: 42},
			},
		},
	}

	var capturedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/distjob/status") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		capturedQuery = r.URL.Query().Get("job")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(want)
	}))
	defer server.Close()

	client, err := New(server.URL[7:], "access", "secret", false)
	if err != nil {
		t.Fatal(err)
	}

	got, err := client.ListDistJobStatuses(context.Background(), DistJobTypeDecommission)
	if err != nil {
		t.Fatalf("ListDistJobStatuses failed: %v", err)
	}
	if capturedQuery != "decommission" {
		t.Errorf("expected job=decommission in query, got %q", capturedQuery)
	}
	if len(got) != 1 || got[0].JobID != "job-1" || got[0].JobType != DistJobTypeDecommission {
		t.Errorf("unexpected result: %+v", got)
	}

	// DistJobTypeUnknown means "no filter" - the job query param must be absent.
	capturedQuery = "unset"
	if _, err := client.ListDistJobStatuses(context.Background(), DistJobTypeUnknown); err != nil {
		t.Fatalf("ListDistJobStatuses failed: %v", err)
	}
	if capturedQuery != "" {
		t.Errorf("expected no job query param, got %q", capturedQuery)
	}
}

func TestListDistJobStatusesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := New(server.URL[7:], "access", "secret", false)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := client.ListDistJobStatuses(context.Background(), DistJobTypeUnknown); err == nil {
		t.Fatal("expected an error for non-200 response, got nil")
	}
}

// TestDistJobTypeWire pins each job type's wire value and name, which a
// rolling upgrade relies on, and checks that names round-trip through
// ParseDistJobType and JSON.
func TestDistJobTypeWire(t *testing.T) {
	tests := []struct {
		typ   DistJobType
		value uint8
		name  string
	}{
		{DistJobTypeUnknown, 0, "unknown"},
		{DistJobTypeDecommission, 1, "decommission"},
		{DistJobTypeCompress, 2, "compress"},
		{DistJobTypeRebalance, 3, "rebalance"},
		{DistJobTypeClusterEncryption, 4, "encrypt-at-rest"},
	}
	for _, tc := range tests {
		if uint8(tc.typ) != tc.value {
			t.Errorf("%s = %d, want %d", tc.name, uint8(tc.typ), tc.value)
		}
		if got := tc.typ.String(); got != tc.name {
			t.Errorf("DistJobType(%d).String() = %q, want %q", tc.value, got, tc.name)
		}
		if got := ParseDistJobType(tc.name); got != tc.typ {
			t.Errorf("ParseDistJobType(%q) = %d, want %d", tc.name, got, tc.typ)
		}
		b, err := json.Marshal(tc.typ)
		if err != nil {
			t.Fatal(err)
		}
		var got DistJobType
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		if got != tc.typ {
			t.Errorf("JSON round trip of %s = %d, want %d", b, got, tc.typ)
		}
	}
	if got := DistJobType(0xff).String(); got != "unknown" {
		t.Errorf("DistJobType(0xff).String() = %q, want unknown", got)
	}
}
