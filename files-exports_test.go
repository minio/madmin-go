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
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

func newFilesExportsTestClient(t *testing.T, serverURL string) *AdminClient {
	t.Helper()
	client, err := New(mustParseHost(t, serverURL), "ak", "sk", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client
}

// TestFilesExportsQueryRequest verifies the method, path and query the client
// sends, and that it decodes a JSON reply in the Files admin API's format.
//
// The ids travel as one comma-separated exportId value, and the order asked for
// is the order the reply documents, so both are asserted rather than the fact
// that a request arrived.
//
// The reply is a literal in the Files API's keys rather than one encoded from
// the Go types, so a renamed key fails here. Decoding ignores the case of a key,
// so TestFilesWireKeys checks the casing.
func TestFilesExportsQueryRequest(t *testing.T) {
	const reply = `{
		"results": [
			{
				"node": "10.0.0.1:9000",
				"reach": "serving",
				"socketPath": "/run/aistor-files.sock",
				"exports": [
					{"exportId": 9, "status": {"exportId": 9, "held": true, "epoch": 7, "ownerId": "o1", "usedBytes": 1024}},
					{"exportId": 4, "notHeld": true}
				]
			},
			{"node": "10.0.0.2:9000", "reach": "no-daemon", "detail": "no daemon"}
		],
		"count": 2,
		"total": 3,
		"unreachableNodes": [{"node": "10.0.0.3:9000", "detail": "context deadline exceeded"}]
	}`

	var (
		method string
		path   string
		query  url.Values
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path, query = r.Method, r.URL.Path, r.URL.Query()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(reply))
	}))
	defer server.Close()

	got, err := newFilesExportsTestClient(t, server.URL).FilesExportsQuery(context.Background(), []uint64{9, 4})
	if err != nil {
		t.Fatalf("FilesExportsQuery: %v", err)
	}

	if method != http.MethodGet {
		t.Errorf("method = %s, want GET", method)
	}
	if wantPath := "/minio/admin/files/v1/gateway/exports"; path != wantPath {
		t.Errorf("path = %s, want %s", path, wantPath)
	}
	if ids := query.Get("exportId"); ids != "9,4" {
		t.Errorf("exportId = %q, want %q", ids, "9,4")
	}

	if got.Count != 2 || got.Total != 3 {
		t.Errorf("count/total = %d/%d, want 2/3", got.Count, got.Total)
	}
	if len(got.Results) != 2 {
		t.Fatalf("results = %d, want 2", len(got.Results))
	}
	if got.Results[0].Reach != FilesNodeServing || got.Results[1].Reach != FilesNodeNoDaemon {
		t.Errorf("reach = %q/%q, want %q/%q",
			got.Results[0].Reach, got.Results[1].Reach, FilesNodeServing, FilesNodeNoDaemon)
	}
	exports := got.Results[0].Exports
	if len(exports) != 2 {
		t.Fatalf("exports = %d, want 2", len(exports))
	}
	if exports[0].ExportID != 9 || exports[1].ExportID != 4 {
		t.Errorf("export ids = %d/%d, want 9/4", exports[0].ExportID, exports[1].ExportID)
	}
	if exports[0].Status == nil {
		t.Fatal("the held export must carry a status document")
	}
	if status := exports[0].Status; status.ExportID != 9 || status.OwnerID != "o1" ||
		status.Epoch != 7 || status.UsedBytes != 1024 {
		t.Errorf("status = %+v, want export 9 owned by o1 at epoch 7 with 1024 bytes used", status)
	}
	if exports[1].Status != nil || !exports[1].NotHeld {
		t.Errorf("unheld export = %+v, want a NotHeld entry with no status document", exports[1])
	}
	if len(got.UnreachableNodes) != 1 || got.UnreachableNodes[0].Node != "10.0.0.3:9000" ||
		got.UnreachableNodes[0].Detail != "context deadline exceeded" {
		t.Errorf("unreachableNodes = %+v, want one entry for 10.0.0.3:9000 with its detail", got.UnreachableNodes)
	}
}

// TestFilesExportsQueryRejectsIDCountsLocally verifies that an id list the
// server would refuse never reaches it, so a caller holding too many ids fails
// on the call rather than on a round trip.
func TestFilesExportsQueryRejectsIDCountsLocally(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newFilesExportsTestClient(t, server.URL)

	overTheCap := make([]uint64, MaxFilesExportIDsPerQuery+1)
	for idx := range overTheCap {
		overTheCap[idx] = uint64(idx)
	}

	for _, tc := range []struct {
		name      string
		exportIDs []uint64
	}{
		{name: "no_ids", exportIDs: nil},
		{name: "empty_slice", exportIDs: []uint64{}},
		{name: "over_the_cap", exportIDs: overTheCap},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := client.FilesExportsQuery(context.Background(), tc.exportIDs); err == nil {
				t.Fatal("FilesExportsQuery should reject this id list")
			}
		})
	}

	if requests != 0 {
		t.Errorf("the server received %d requests, want 0", requests)
	}
}

// TestFilesExportsQueryAtTheCap verifies the boundary is inclusive: exactly
// MaxFilesExportIDsPerQuery ids is a request the client sends.
func TestFilesExportsQueryAtTheCap(t *testing.T) {
	var ids string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids = r.URL.Query().Get("exportId")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(&FilesExportsQueryResponse{}); err != nil {
			t.Errorf("encode reply: %v", err)
		}
	}))
	defer server.Close()

	atTheCap := make([]uint64, MaxFilesExportIDsPerQuery)
	for idx := range atTheCap {
		atTheCap[idx] = uint64(idx)
	}

	if _, err := newFilesExportsTestClient(t, server.URL).FilesExportsQuery(context.Background(), atTheCap); err != nil {
		t.Fatalf("FilesExportsQuery: %v", err)
	}
	if got := len(strings.Split(ids, ",")); got != MaxFilesExportIDsPerQuery {
		t.Errorf("the server saw %d ids, want %d", got, MaxFilesExportIDsPerQuery)
	}
}

// TestFilesExportsQueryLeaseTimes verifies what a nil LastRenew means on the
// wire: a daemon that has never renewed a lease is distinguishable from one
// that renewed at an unknown time, without a sentinel age.
func TestFilesExportsQueryLeaseTimes(t *testing.T) {
	renewed := time.Date(2026, 9, 16, 10, 30, 0, 0, time.UTC)
	taken := time.Date(2026, 9, 16, 10, 30, 5, 0, time.UTC)

	want := FilesExportsQueryResponse{
		Results: []FilesNodeStatus{{
			Node:  "10.0.0.1:9000",
			Reach: FilesNodeServing,
			Exports: []FilesExportResult{
				{ExportID: 1, Status: &FilesExportStatus{
					ExportID: 1, Leasing: true, Held: true,
					LastRenew: &renewed, TTLSecs: 30, TS: taken,
				}},
				{ExportID: 2, Status: &FilesExportStatus{
					ExportID: 2, Leasing: true, TS: taken,
				}},
			},
		}},
		Count: 1,
		Total: 1,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(&want); err != nil {
			t.Errorf("encode reply: %v", err)
		}
	}))
	defer server.Close()

	got, err := newFilesExportsTestClient(t, server.URL).FilesExportsQuery(context.Background(), []uint64{1, 2})
	if err != nil {
		t.Fatalf("FilesExportsQuery: %v", err)
	}
	if len(got.Results) != 1 || len(got.Results[0].Exports) != 2 {
		t.Fatalf("results = %+v, want one node holding two exports", got.Results)
	}
	exports := got.Results[0].Exports

	renewedStatus := exports[0].Status
	if renewedStatus == nil || renewedStatus.LastRenew == nil {
		t.Fatalf("export 1 = %+v, want a status document carrying a renewal time", exports[0])
	}
	if !renewedStatus.LastRenew.Equal(renewed) {
		t.Errorf("lastRenew = %s, want %s", renewedStatus.LastRenew, renewed)
	}
	if !renewedStatus.TS.Equal(taken) {
		t.Errorf("ts = %s, want %s", renewedStatus.TS, taken)
	}

	neverRenewed := exports[1].Status
	if neverRenewed == nil {
		t.Fatal("export 2 must carry a status document")
	}
	if neverRenewed.LastRenew != nil {
		t.Errorf("lastRenew = %s, want nil for a lease that was never renewed", neverRenewed.LastRenew)
	}
	if !neverRenewed.Leasing {
		t.Error("leasing must survive the round trip, so a never-renewed lease is not read as an unleased export")
	}
}

// filesWireKey matches a camelCase JSON key in which every capital starts a
// word, so "exportId" passes and "exportID" does not.
var filesWireKey = regexp.MustCompile(`^[a-z][a-z0-9]*([A-Z][a-z0-9]+)*$`)

// TestFilesWireKeys verifies that every JSON key of the Files admin API types
// is camelCase, and that each cluster-wide reply names the nodes it missed in
// unreachableNodes. The server is expected to encode these types, so their keys
// are the API's wire format.
func TestFilesWireKeys(t *testing.T) {
	unreachableType := reflect.TypeOf(FilesUnreachableNode{})
	seen := make(map[reflect.Type]bool)

	var walk func(typ reflect.Type)
	walk = func(typ reflect.Type) {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct || typ == reflect.TypeOf(time.Time{}) || seen[typ] {
			return
		}
		seen[typ] = true
		for idx := range typ.NumField() {
			field := typ.Field(idx)
			if !field.IsExported() {
				continue
			}
			key, _, _ := strings.Cut(field.Tag.Get("json"), ",")
			if key == "-" {
				continue
			}
			if key == "" {
				key = field.Name
			}
			if !filesWireKey.MatchString(key) {
				t.Errorf("%s.%s is encoded as %q, which is not camelCase", typ.Name(), field.Name, key)
			}
			if field.Type.Kind() == reflect.Slice && field.Type.Elem() == unreachableType && key != "unreachableNodes" {
				t.Errorf("%s.%s names missed nodes as %q, want unreachableNodes", typ.Name(), field.Name, key)
			}
			walk(field.Type)
		}
	}
	for _, root := range []any{FilesExportsQueryResponse{}, FilesAuthResponse{}, FilesAuthSetRequest{}} {
		walk(reflect.TypeOf(root))
	}
}
