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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
