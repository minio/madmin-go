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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/minio/madmin-go/v4/filesaccess"
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
// The ids travel as one comma-separated exportID value, and the order asked for
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
					{"exportID": 9, "status": {"exportID": 9, "held": true, "epoch": 7, "ownerId": "o1", "usedBytes": 1024}},
					{"exportID": 4, "notHeld": true}
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
	if ids := query.Get("exportID"); ids != "9,4" {
		t.Errorf("exportID = %q, want %q", ids, "9,4")
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

// TestFilesGatewayExportsRequest verifies that the listing sends no query at
// all, and that each node's listing decodes. A node with no daemon carries no
// listing, and a listing without a time carries no TS.
func TestFilesGatewayExportsRequest(t *testing.T) {
	const reply = `{
		"results": [
			{
				"node": "10.0.0.1:9000",
				"reach": "serving",
				"daemon": {"exports": [1, 4], "bootId": "3f9c", "ts": "2026-10-03T10:00:00Z"}
			},
			{"node": "10.0.0.2:9000", "reach": "no-daemon", "detail": "no daemon"},
			{"node": "10.0.0.3:9000", "reach": "serving", "daemon": {"exports": []}},
			{"node": "10.0.0.4:9000", "reach": "serving", "daemon": {"exports": [1, 4], "truncated": true}}
		],
		"count": 4,
		"total": 4
	}`

	server, seen := newFilesJSONServer(t, http.StatusOK, reply)

	got, err := newFilesExportsTestClient(t, server.URL).FilesGatewayExports(context.Background())
	if err != nil {
		t.Fatalf("FilesGatewayExports: %v", err)
	}

	if len(*seen) != 1 {
		t.Fatalf("the server saw %d requests, want 1", len(*seen))
	}
	if req := (*seen)[0]; req.method != http.MethodGet || req.path != "/minio/admin/files/v1/gateway/exports" || len(req.query) != 0 {
		t.Errorf("request = %s %s %v, want GET on the gateway exports path with no query", req.method, req.path, req.query)
	}

	if len(got.Results) != 4 {
		t.Fatalf("results = %d, want 4", len(got.Results))
	}
	daemon := got.Results[0].Daemon
	if daemon == nil || !slices.Equal(daemon.Exports, []uint64{1, 4}) || daemon.BootID != "3f9c" ||
		daemon.TS == nil || !daemon.TS.Equal(time.Date(2026, 10, 3, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("daemon = %+v, want exports [1 4] under boot id 3f9c at 10:00 UTC", daemon)
	}
	if got.Results[1].Daemon != nil {
		t.Errorf("a node with no daemon carried a listing: %+v", got.Results[1].Daemon)
	}
	// A gateway serving nothing still answered, which is what separates it from
	// a node with no daemon.
	if empty := got.Results[2].Daemon; empty == nil || len(empty.Exports) != 0 || empty.TS != nil {
		t.Errorf("empty listing = %+v, want a listing with no exports and no time", empty)
	}
	if cut := got.Results[3].Daemon; cut == nil || !cut.Truncated {
		t.Errorf("truncated listing = %+v, want Truncated", cut)
	}
}

// TestFilesExportsQueryAtTheCap verifies the boundary is inclusive: exactly
// MaxFilesExportIDsPerQuery ids is a request the client sends.
func TestFilesExportsQueryAtTheCap(t *testing.T) {
	var ids string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids = r.URL.Query().Get("exportID")
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
// word or belongs to the initialism ID, so "exportID" passes and "ExportID"
// and "export_id" do not.
var filesWireKey = regexp.MustCompile(`^[a-z][a-z0-9]*([A-Z][a-z0-9]+|ID)*$`)

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
		// A type that encodes itself, such as filesaccess.Rule, owns its
		// wire form, and its own tests check it.
		if reflect.PointerTo(typ).Implements(reflect.TypeFor[json.Marshaler]()) {
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
	for _, root := range []any{
		FilesExportsQueryResponse{},
		FilesAuthResponse{},
		FilesAuthSetRequest{},
		FilesExportList{},
		FilesExport{},
		FilesStatsList{},
	} {
		walk(reflect.TypeOf(root))
	}
}

// filesRequest is what a test server saw of one request.
type filesRequest struct {
	method      string
	path        string
	rawPath     string
	query       url.Values
	contentType string
	ifMatch     string
	body        []byte
}

// newFilesJSONServer answers every request with status and body, and records
// each request it saw.
func newFilesJSONServer(t *testing.T, status int, body string) (*httptest.Server, *[]filesRequest) {
	t.Helper()
	var seen []filesRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		seen = append(seen, filesRequest{r.Method, r.URL.Path, r.URL.EscapedPath(), r.URL.Query(), r.Header.Get("Content-Type"), r.Header.Get("If-Match"), sent})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if _, err := io.WriteString(w, body); err != nil {
			t.Errorf("write reply: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	return server, &seen
}

// TestListFilesExportsRequest verifies the path and filters ListFilesExports
// sends, and that it decodes the sample reply of MANAGEMENT-API.md §2: an
// export on a node that did not answer is still listed, with no usage.
func TestListFilesExportsRequest(t *testing.T) {
	server, seen := newFilesJSONServer(t, http.StatusOK, `{
  "exports": [
    {"name": "carol", "exportID": 104, "pseudo": "/home/carol", "node": "node03.example.com",
     "status": "serving", "usedBytes": 1288490188, "quotaBytes": 21474836480},
    {"name": "frank", "exportID": 107, "pseudo": "/home/frank", "node": "node02.example.com",
     "status": "unreachable", "quotaBytes": 10737418240}
  ],
  "unreachableNodes": [
    {"node": "node02.example.com", "detail": "filesgw: no gateway daemon on this node"}
  ]
}`)
	client := newFilesExportsTestClient(t, server.URL)

	got, err := client.ListFilesExports(context.Background(), FilesListOptions{Node: "node03", Status: FilesExportServing})
	if err != nil {
		t.Fatalf("ListFilesExports: %v", err)
	}
	req := (*seen)[0]
	if req.method != http.MethodGet {
		t.Errorf("method = %s, want GET", req.method)
	}
	if want := "/minio/admin/files/v1/exports"; req.path != want {
		t.Errorf("path = %s, want %s", req.path, want)
	}
	if n, s := req.query.Get("node"), req.query.Get("status"); n != "node03" || s != "serving" {
		t.Errorf("node/status = %q/%q, want node03/serving", n, s)
	}

	if len(got.Exports) != 2 {
		t.Fatalf("exports = %+v, want 2", got.Exports)
	}
	carol, frank := got.Exports[0], got.Exports[1]
	if carol.Name != "carol" || carol.ExportID != 104 || carol.Status != FilesExportServing ||
		carol.QuotaBytes != 21474836480 || carol.UsedBytes == nil || *carol.UsedBytes != 1288490188 {
		t.Errorf("carol = %+v", carol)
	}
	if frank.Status != FilesExportUnreachable || frank.UsedBytes != nil {
		t.Errorf("frank = %+v, want unreachable with no usage", frank)
	}
	if len(got.UnreachableNodes) != 1 || got.UnreachableNodes[0].Node != "node02.example.com" ||
		got.UnreachableNodes[0].Detail == "" {
		t.Errorf("unreachableNodes = %+v", got.UnreachableNodes)
	}

	// No option narrows nothing, so no filter is sent.
	if _, err := client.ListFilesExports(context.Background(), FilesListOptions{}); err != nil {
		t.Fatalf("ListFilesExports: %v", err)
	}
	if q := (*seen)[1].query; len(q) != 0 {
		t.Errorf("query = %v, want none", q)
	}
}

// TestGetFilesExportRequest verifies that a name and an id are both sent as
// given, and that the sample reply of MANAGEMENT-API.md §3 decodes with its
// access rules in evaluation order.
func TestGetFilesExportRequest(t *testing.T) {
	server, seen := newFilesJSONServer(t, http.StatusOK, `{
  "name": "carol", "exportID": 104, "pseudo": "/home/carol", "node": "node03.example.com",
  "accessType": "rw", "squash": "root", "quotaBytes": 21474836480,
  "accessRules": [
    {"clients": ["10.20.9.0/24"], "accessType": "none"},
    {"clients": ["10.20.4.7", "10.20.4.8"], "accessType": "rw"},
    {"clients": ["*.corp.example.com", "10.20.0.0/16"], "accessType": "ro"}
  ],
  "status": "serving", "usedBytes": 1288490188
}`)
	client := newFilesExportsTestClient(t, server.URL)

	for _, tc := range []struct{ export, rawPath string }{
		{"carol", "/minio/admin/files/v1/exports/carol"},
		{"104", "/minio/admin/files/v1/exports/104"},
		{"a b?", "/minio/admin/files/v1/exports/a%20b%3F"},
		// An unescaped / would route to another path.
		{"a/b", "/minio/admin/files/v1/exports/a%2Fb"},
	} {
		got, err := client.GetFilesExport(context.Background(), tc.export)
		if err != nil {
			t.Fatalf("GetFilesExport(%q): %v", tc.export, err)
		}
		req := (*seen)[len(*seen)-1]
		if req.method != http.MethodGet || req.rawPath != tc.rawPath || len(req.query) != 0 {
			t.Errorf("GetFilesExport(%q) sent %s %s?%v, want GET %s", tc.export, req.method, req.rawPath, req.query, tc.rawPath)
		}

		want := FilesExport{
			Name: "carol", ExportID: 104, Pseudo: "/home/carol", Node: "node03.example.com",
			Status: FilesExportServing, AccessType: FilesAccessRW, Squash: FilesSquashRoot, QuotaBytes: 21474836480,
			AccessRules: mustFilesRules("10.20.9.0/24 none\n" +
				"10.20.4.7,10.20.4.8 rw\n" +
				"*.corp.example.com,10.20.0.0/16 ro\n").All(),
		}
		used := got.UsedBytes
		got.UsedBytes = nil
		if !reflect.DeepEqual(got, want) {
			t.Errorf("GetFilesExport(%q) = %+v, want %+v", tc.export, got, want)
		}
		if used == nil || *used != 1288490188 {
			t.Errorf("usedBytes = %v, want 1288490188", used)
		}
	}
}

// TestGetFilesExportRefusesLocally verifies that a token naming no export
// never reaches the server. "stats" would reach the fleet stats endpoint and
// decode into an empty export rather than fail.
func TestGetFilesExportRefusesLocally(t *testing.T) {
	server, seen := newFilesJSONServer(t, http.StatusOK, `{}`)
	client := newFilesExportsTestClient(t, server.URL)

	for _, export := range []string{"", "stats"} {
		if _, err := client.GetFilesExport(context.Background(), export); err == nil {
			t.Errorf("GetFilesExport(%q) should fail", export)
		}
	}
	if len(*seen) != 0 {
		t.Errorf("the server received %d requests, want 0", len(*seen))
	}
}

// TestFilesExportStatsRequest verifies both stats endpoints, and that the
// sample reply of MANAGEMENT-API.md §6 decodes.
func TestFilesExportStatsRequest(t *testing.T) {
	server, seen := newFilesJSONServer(t, http.StatusOK, `{
  "stats": [
    {"name": "carol", "exportID": 104, "node": "node03.example.com", "status": "serving",
     "usedBytes": 1288490188, "limitBytes": 21474836480},
    {"name": "frank", "exportID": 107, "node": "node02.example.com", "status": "unreachable",
     "limitBytes": 10737418240}
  ],
  "unreachableNodes": [{"node": "node02.example.com", "detail": "down"}]
}`)
	client := newFilesExportsTestClient(t, server.URL)

	got, err := client.FilesExportStats(context.Background(), "", FilesStatsOptions{Node: "node03"})
	if err != nil {
		t.Fatalf("FilesExportStats: %v", err)
	}
	req := (*seen)[0]
	if want := "/minio/admin/files/v1/exports/stats"; req.method != http.MethodGet || req.path != want {
		t.Errorf("fleet stats sent %s %s, want GET %s", req.method, req.path, want)
	}
	if n := req.query.Get("node"); n != "node03" {
		t.Errorf("node = %q, want node03", n)
	}
	if len(got.Stats) != 2 || got.Stats[0].UsedBytes == nil || *got.Stats[0].UsedBytes != 1288490188 ||
		got.Stats[0].LimitBytes != 21474836480 || got.Stats[1].UsedBytes != nil ||
		got.Stats[1].Status != FilesExportUnreachable || len(got.UnreachableNodes) != 1 {
		t.Errorf("stats = %+v", got)
	}

	if _, err := client.FilesExportStats(context.Background(), "104", FilesStatsOptions{}); err != nil {
		t.Fatalf("FilesExportStats: %v", err)
	}
	req = (*seen)[1]
	if want := "/minio/admin/files/v1/exports/104/stats"; req.path != want || len(req.query) != 0 {
		t.Errorf("per-export stats sent %s?%v, want %s", req.path, req.query, want)
	}

	// The per-export form is neither narrowed nor paged, so each option is
	// refused before a request is sent.
	for _, opts := range []FilesStatsOptions{{Node: "node03"}, {Limit: 10}, {ContinuationToken: "t"}} {
		if _, err := client.FilesExportStats(context.Background(), "104", opts); err == nil {
			t.Errorf("FilesExportStats with an export and %+v succeeded, want an error", opts)
		}
	}
	if len(*seen) != 2 {
		t.Errorf("the server received %d requests, want 2", len(*seen))
	}
}

// TestFilesListsEncodeEmptyAsArray verifies that a server listing nothing
// replies with [], as the samples of MANAGEMENT-API.md show, rather than null.
func TestFilesListsEncodeEmptyAsArray(t *testing.T) {
	for _, tc := range []struct {
		v   any
		key string
	}{
		{FilesExportList{}, `"exports":[]`},
		{&FilesExportList{}, `"exports":[]`},
		{FilesStatsList{}, `"stats":[]`},
		{&FilesStatsList{}, `"stats":[]`},
		{FilesDaemonExports{}, `"exports":[]`},
		{&FilesNodeStatus{Daemon: &FilesDaemonExports{}}, `"exports":[]`},
	} {
		body, err := json.Marshal(tc.v)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), tc.key) {
			t.Errorf("%T encodes as %s, want %s", tc.v, body, tc.key)
		}
	}
}

// TestFilesReadErrorCode verifies that the Files error envelope reaches the
// caller as an ErrorResponse carrying its code, so a caller switches on the
// code rather than the message.
func TestFilesReadErrorCode(t *testing.T) {
	server, _ := newFilesJSONServer(t, http.StatusNotFound,
		`{"code": "ExportNotFound", "message": "no export named carol", "requestId": "18B3D491BB8DCD15"}`)
	client := newFilesExportsTestClient(t, server.URL)

	for name, call := range map[string]func() error{
		"list": func() error { _, err := client.ListFilesExports(context.Background(), FilesListOptions{}); return err },
		"get":  func() error { _, err := client.GetFilesExport(context.Background(), "carol"); return err },
		"stats": func() error {
			_, err := client.FilesExportStats(context.Background(), "carol", FilesStatsOptions{})
			return err
		},
		"query": func() error {
			_, err := client.FilesExportsQuery(context.Background(), []uint64{4})
			return err
		},
		"gateway listing": func() error { _, err := client.FilesGatewayExports(context.Background()); return err },
	} {
		err := call()
		var errResp ErrorResponse
		if !errors.As(err, &errResp) {
			t.Fatalf("%s: err = %v (%T), want an ErrorResponse", name, err, err)
		}
		if errResp.Code != FilesErrExportNotFound || errResp.RequestID != "18B3D491BB8DCD15" {
			t.Errorf("%s: error = %+v, want code %s", name, errResp, FilesErrExportNotFound)
		}
	}
}

// TestFilesReservedExportNames verifies that a name that would reach a route
// other than the export's is refused before a request is sent, by both methods
// that take an export.
func TestFilesReservedExportNames(t *testing.T) {
	server, seen := newFilesJSONServer(t, http.StatusOK, `{}`)
	client := newFilesExportsTestClient(t, server.URL)

	for _, name := range []string{"stats", ".", ".."} {
		if _, err := client.GetFilesExport(context.Background(), name); err == nil {
			t.Errorf("GetFilesExport(%q) succeeded, want an error", name)
		}
		if _, err := client.FilesExportStats(context.Background(), name, FilesStatsOptions{}); err == nil {
			t.Errorf("FilesExportStats(%q) succeeded, want an error", name)
		}
	}
	if len(*seen) != 0 {
		t.Errorf("the server received %d requests, want 0", len(*seen))
	}
}

// TestFilesRead426IsNotRetried verifies that a 426 on a Files path, which has
// no admin API version to downgrade, is returned after one request. A server
// with no Files routes answers so. An export name holding the admin API
// version, such as v4home, is not rewritten either.
func TestFilesRead426IsNotRetried(t *testing.T) {
	for _, name := range []string{"carol", "v4home"} {
		server, seen := newFilesJSONServer(t, http.StatusUpgradeRequired,
			`{"Code": "XMinioAdminVersionMismatch", "Message": "upgrade"}`)

		_, err := newFilesExportsTestClient(t, server.URL).GetFilesExport(context.Background(), name)
		if ToErrorResponse(err).Code != "XMinioAdminVersionMismatch" {
			t.Errorf("%s: err = %v, want XMinioAdminVersionMismatch", name, err)
		}
		if len(*seen) != 1 {
			t.Errorf("%s: the server received %d requests, want 1", name, len(*seen))
		}
		if want := "/minio/admin/files/v1/exports/" + name; (*seen)[0].path != want {
			t.Errorf("%s: path = %s, want %s", name, (*seen)[0].path, want)
		}
	}
}

// closeTracker is a RoundTripper that records whether each response body it
// returns was closed, and whether the earlier ones were closed by the time the
// next request was sent.
type closeTracker struct {
	next   http.RoundTripper
	bodies []*trackedBody
	// openAtRetry is set when a request is sent while an earlier response
	// body is still open.
	openAtRetry bool
}

type trackedBody struct {
	io.ReadCloser
	closed bool
}

func (b *trackedBody) Close() error {
	b.closed = true
	return b.ReadCloser.Close()
}

func (c *closeTracker) RoundTrip(r *http.Request) (*http.Response, error) {
	for _, body := range c.bodies {
		if !body.closed {
			c.openAtRetry = true
		}
	}
	resp, err := c.next.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	body := &trackedBody{ReadCloser: resp.Body}
	c.bodies = append(c.bodies, body)
	resp.Body = body
	return resp, nil
}

// TestAdmin426RetriesOnOldPrefix verifies that a 426 on an admin API path is
// retried once on the old version, that only the leading version is rewritten,
// not one later in the path, and that the 426 response is closed before the
// retry so its connection is not leaked. The prefixes are pinned, because
// MADMIN_API_VERSION=v3 leaves no older version to retry on.
func TestAdmin426RetriesOnOldPrefix(t *testing.T) {
	setAdminAPIPrefixes(t, "/v4", "/v3")
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Path)
		if strings.HasPrefix(r.URL.Path, "/minio/admin"+adminAPIPrefix) {
			w.WriteHeader(http.StatusUpgradeRequired)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	client := newFilesExportsTestClient(t, server.URL)
	tracker := &closeTracker{next: client.httpClient.Transport}
	client.httpClient.Transport = tracker

	resp, err := client.executeMethod(context.Background(),
		http.MethodGet, requestData{relPath: adminAPIPrefix + "/probe" + adminAPIPrefix})
	if err != nil {
		t.Fatalf("executeMethod: %v", err)
	}
	want := []string{
		"/minio/admin" + adminAPIPrefix + "/probe" + adminAPIPrefix,
		"/minio/admin" + adminAPIOldPrefix + "/probe" + adminAPIPrefix,
	}
	if !slices.Equal(seen, want) {
		t.Errorf("paths = %v, want %v", seen, want)
	}
	if len(tracker.bodies) != 2 {
		t.Fatalf("the transport returned %d responses, want 2", len(tracker.bodies))
	}
	if tracker.openAtRetry {
		t.Error("the 426 response was not closed before the retry")
	}
	closeResponse(resp)
}

// TestFilesExportUsedBytesOnTheWire verifies what the server encodes: usage
// that is unknown is absent, and a usage of zero is present, so neither is
// read as the other.
func TestFilesExportUsedBytesOnTheWire(t *testing.T) {
	zero := uint64(0)
	for _, tc := range []struct {
		used    *uint64
		present bool
	}{{nil, false}, {&zero, true}} {
		body, err := json.Marshal(FilesExport{Name: "carol", UsedBytes: tc.used})
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(string(body), `"usedBytes"`); got != tc.present {
			t.Errorf("%s: usedBytes present = %v, want %v", body, got, tc.present)
		}
	}
}

// newFilesPagedServer serves pages of a fleet read at path, one per
// continuation token: pages[0] answers a request without one, and pages[i]
// the token "p<i>". Each page but the last carries the token of the next. It
// records the query of each request.
func newFilesPagedServer(t *testing.T, path, key string, pages [][]string) (*httptest.Server, *[]url.Values) {
	t.Helper()
	var seen []url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			http.Error(w, `{"code": "NotFound"}`, http.StatusNotFound)
			return
		}
		seen = append(seen, r.URL.Query())
		idx := 0
		if token := r.URL.Query().Get("continuation-token"); token != "" {
			if _, err := fmt.Sscanf(token, "p%d", &idx); err != nil || idx < 1 || idx >= len(pages) {
				http.Error(w, `{"code": "InvalidRequest"}`, http.StatusBadRequest)
				return
			}
		}
		entries := make([]string, len(pages[idx]))
		for i, name := range pages[idx] {
			entries[i] = fmt.Sprintf(`{"name": %q}`, name)
		}
		body := fmt.Sprintf(`{%q: [%s]`, key, strings.Join(entries, ","))
		if idx+1 < len(pages) {
			body += fmt.Sprintf(`, "nextContinuationToken": "p%d"`, idx+1)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, body+"}")
	}))
	t.Cleanup(server.Close)
	return server, &seen
}

// TestListFilesExportsPages verifies that the limit and the token are sent,
// that a limit of zero or less is left to the server, and that a caller
// following NextContinuationToken reads every page, including an empty one a
// status filter leaves, and stops only when it is empty.
func TestListFilesExportsPages(t *testing.T) {
	server, seen := newFilesPagedServer(t, "/minio/admin/files/v1/exports", "exports",
		[][]string{{"carol", "dave"}, {}, {"gina"}})
	client := newFilesExportsTestClient(t, server.URL)

	var names []string
	opts := FilesListOptions{Status: FilesExportServing, Limit: 2}
	for {
		page, err := client.ListFilesExports(context.Background(), opts)
		if err != nil {
			t.Fatalf("ListFilesExports(%+v): %v", opts, err)
		}
		for _, e := range page.Exports {
			names = append(names, e.Name)
		}
		if page.NextContinuationToken == "" {
			break
		}
		opts.ContinuationToken = page.NextContinuationToken
	}
	if !slices.Equal(names, []string{"carol", "dave", "gina"}) || len(*seen) != 3 {
		t.Fatalf("%d pages listed %v, want 3 pages of carol, dave, gina", len(*seen), names)
	}
	for idx, q := range *seen {
		want := url.Values{"status": {"serving"}, "limit": {"2"}}
		if idx > 0 {
			want.Set("continuation-token", fmt.Sprintf("p%d", idx))
		}
		if !reflect.DeepEqual(q, want) {
			t.Errorf("page %d sent %v, want %v", idx+1, q, want)
		}
	}

	for _, limit := range []int{0, -5} {
		if _, err := client.ListFilesExports(context.Background(), FilesListOptions{Limit: limit}); err != nil {
			t.Fatalf("ListFilesExports(limit %d): %v", limit, err)
		}
		if q := (*seen)[len(*seen)-1]; q.Has("limit") {
			t.Errorf("limit %d sent %v, want it left to the server", limit, q)
		}
	}
}

// TestFilesExportStatsPages verifies that the fleet stats page as the list
// does.
func TestFilesExportStatsPages(t *testing.T) {
	server, seen := newFilesPagedServer(t, "/minio/admin/files/v1/exports/stats", "stats",
		[][]string{{"carol"}, {"frank"}})
	client := newFilesExportsTestClient(t, server.URL)

	first, err := client.FilesExportStats(context.Background(), "", FilesStatsOptions{Node: "node03", Limit: 1})
	if err != nil {
		t.Fatalf("FilesExportStats: %v", err)
	}
	if len(first.Stats) != 1 || first.Stats[0].Name != "carol" || first.NextContinuationToken != "p1" {
		t.Fatalf("page 1 = %+v, want carol and a token", first)
	}
	last, err := client.FilesExportStats(context.Background(), "",
		FilesStatsOptions{Node: "node03", Limit: 1, ContinuationToken: first.NextContinuationToken})
	if err != nil {
		t.Fatalf("FilesExportStats: %v", err)
	}
	if len(last.Stats) != 1 || last.Stats[0].Name != "frank" || last.NextContinuationToken != "" {
		t.Fatalf("page 2 = %+v, want frank and no token", last)
	}
	want := url.Values{"node": {"node03"}, "limit": {"1"}, "continuation-token": {"p1"}}
	if q := (*seen)[1]; !reflect.DeepEqual(q, want) {
		t.Errorf("page 2 sent %v, want %v", q, want)
	}
}

// jsonKeys returns the top-level keys of a JSON object.
func jsonKeys(t *testing.T, body []byte) []string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("decode body %q: %v", body, err)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// TestAddFilesExportRequest verifies that AddFilesExport sends a PUT carrying
// only the fields MANAGEMENT-API.md §1 lists, since the server refuses an
// unknown one, and leaves a default out rather than sending a zero. It decodes
// the 202 sample reply.
func TestAddFilesExportRequest(t *testing.T) {
	server, seen := newFilesJSONServer(t, http.StatusAccepted, `{
  "name": "carol", "exportID": 104, "pseudo": "/home/carol", "node": "node03.example.com",
  "status": "pending", "accessType": "rw", "squash": "root", "quotaBytes": 10737418240
}`)
	client := newFilesExportsTestClient(t, server.URL)

	id := uint64(104)
	got, err := client.AddFilesExport(context.Background(), FilesExportSpec{
		Name: "carol", Pseudo: "/home/carol", AccessType: FilesAccessRW, Squash: FilesSquashRoot,
		QuotaBytes: 10737418240, Node: "node03.example.com", ExportID: &id,
	})
	if err != nil {
		t.Fatalf("AddFilesExport: %v", err)
	}
	req := (*seen)[0]
	if want := "/minio/admin/files/v1/exports"; req.method != http.MethodPut || req.path != want {
		t.Errorf("sent %s %s, want PUT %s", req.method, req.path, want)
	}
	if req.contentType != "application/json" {
		t.Errorf("content type = %q, want application/json", req.contentType)
	}
	if keys, want := jsonKeys(t, req.body), []string{"accessType", "exportID", "name", "node", "pseudo", "quotaBytes", "squash"}; !slices.Equal(keys, want) {
		t.Errorf("body keys = %v, want %v", keys, want)
	}
	want := FilesExport{
		Name: "carol", ExportID: 104, Pseudo: "/home/carol", Node: "node03.example.com", Status: FilesExportPending,
		AccessType: FilesAccessRW, Squash: FilesSquashRoot, QuotaBytes: 10737418240,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("reply = %+v, want %+v", got, want)
	}

	if _, err := client.AddFilesExport(context.Background(), FilesExportSpec{Name: "dave", Pseudo: "/home/dave"}); err != nil {
		t.Fatalf("AddFilesExport: %v", err)
	}
	if keys, want := jsonKeys(t, (*seen)[1].body), []string{"name", "pseudo"}; !slices.Equal(keys, want) {
		t.Errorf("body keys = %v, want %v: a default is left to the server", keys, want)
	}
}

// mustFilesRules parses a rules file, and panics on an error: the tests pass
// only valid ones.
func mustFilesRules(file string) filesaccess.Rules {
	rules, err := filesaccess.Parse(strings.NewReader(file), "test")
	if err != nil {
		panic(err)
	}
	return rules
}

// filesWrites calls each Files write once against client.
func filesWrites(ctx context.Context, client *AdminClient) map[string]func() error {
	rules := mustFilesRules("* rw")
	return map[string]func() error{
		"add": func() error {
			_, err := client.AddFilesExport(ctx, FilesExportSpec{Name: "carol", Pseudo: "/home/carol"})
			return err
		},
		"quota":  func() error { _, err := client.SetFilesExportQuota(ctx, "carol", 7, 1); return err },
		"access": func() error { _, err := client.SetFilesExportAccess(ctx, "carol", 7, rules); return err },
		"clear":  func() error { _, err := client.ClearFilesExportAccess(ctx, "carol", 7); return err },
		"remove": func() error {
			_, err := client.RemoveFilesExport(ctx, "carol", 7, FilesRemoveOptions{Purge: true})
			return err
		},
	}
}

// TestFilesWriteSendsIfMatch verifies that every per-export write names its
// generation in If-Match as a quoted entity tag, and that an add, which has no
// generation yet, names none.
func TestFilesWriteSendsIfMatch(t *testing.T) {
	server, seen := newFilesJSONServer(t, http.StatusOK, `{}`)
	for name, call := range filesWrites(context.Background(), newFilesExportsTestClient(t, server.URL)) {
		before := len(*seen)
		if err := call(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		want := `"7"`
		if name == "add" {
			want = ""
		}
		if got := (*seen)[before].ifMatch; got != want {
			t.Errorf("%s: If-Match = %q, want %q", name, got, want)
		}
	}
}

// TestFilesWriteRefusesZeroGeneration verifies that a per-export write without
// a generation is refused before it is sent, since generations start at 1.
func TestFilesWriteRefusesZeroGeneration(t *testing.T) {
	server, seen := newFilesJSONServer(t, http.StatusOK, `{}`)
	client := newFilesExportsTestClient(t, server.URL)
	ctx := context.Background()
	rules := mustFilesRules("* rw")

	for name, call := range map[string]func() error{
		"quota":  func() error { _, err := client.SetFilesExportQuota(ctx, "carol", 0, 1); return err },
		"access": func() error { _, err := client.SetFilesExportAccess(ctx, "carol", 0, rules); return err },
		"clear":  func() error { _, err := client.ClearFilesExportAccess(ctx, "carol", 0); return err },
		"remove": func() error { _, err := client.RemoveFilesExport(ctx, "carol", 0, FilesRemoveOptions{}); return err },
	} {
		if err := call(); err == nil {
			t.Errorf("%s with generation 0 should fail", name)
		}
	}
	if len(*seen) != 0 {
		t.Errorf("the server received %d requests, want 0", len(*seen))
	}
}

// filesScriptedReply is one answer of newFilesScriptedServer. A zero status
// drops the connection instead, so the outcome is unknown to the client.
type filesScriptedReply struct {
	status int
	body   string
}

// newFilesScriptedServer answers the n-th write, counting from 0, with
// writes[n], and every later one with the last entry. A GET is answered with
// read. It returns the server and its write and read counts.
func newFilesScriptedServer(t *testing.T, read filesScriptedReply, writes ...filesScriptedReply) (*httptest.Server, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	var nWrites, nReads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reply := read
		if r.Method == http.MethodGet {
			nReads.Add(1)
		} else {
			n := int(nWrites.Add(1)) - 1
			reply = writes[min(n, len(writes)-1)]
		}
		if reply.status == 0 {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("hijack: %v", err)
				return
			}
			conn.Close()
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(reply.status)
		io.WriteString(w, reply.body)
	}))
	t.Cleanup(server.Close)
	return server, &nWrites, &nReads
}

var (
	filesDropped  = filesScriptedReply{}
	filesModified = filesScriptedReply{http.StatusPreconditionFailed, `{"code": "ExportModified", "message": "changed"}`}
	filesExists   = filesScriptedReply{http.StatusConflict, `{"code": "ExportAlreadyExists", "message": "taken"}`}
	filesNotFound = filesScriptedReply{http.StatusNotFound, `{"code": "ExportNotFound", "message": "gone"}`}
	filesCarol    = filesScriptedReply{http.StatusOK, `{"name": "carol", "exportID": 104, "generation": 9,
		"pseudo": "/home/carol", "accessType": "rw", "squash": "root", "quotaBytes": 1,
		"accessRules": [{"clients": ["*"], "accessType": "rw"}]}`}
	filesCarolOther = filesScriptedReply{http.StatusOK, `{"name": "carol", "exportID": 104, "generation": 9,
		"pseudo": "/home/other", "accessType": "ro", "squash": "root", "quotaBytes": 2}`}
)

// TestFilesWriteRetriesOnUnknownOutcome verifies that every write is repeated
// after an answer that leaves its outcome unknown, since a repeat cannot apply
// it twice.
func TestFilesWriteRetriesOnUnknownOutcome(t *testing.T) {
	for _, tc := range []struct {
		name  string
		first filesScriptedReply
	}{
		{"dropped_connection", filesDropped},
		{"bad_gateway", filesScriptedReply{http.StatusBadGateway, `bad gateway`}},
		{"slow_down", filesScriptedReply{http.StatusServiceUnavailable, `{"code": "SlowDown", "message": "slow down"}`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server, writes, _ := newFilesScriptedServer(t, filesCarol, tc.first, filesScriptedReply{http.StatusOK, `{}`})
			for name, call := range filesWrites(context.Background(), newFilesExportsTestClient(t, server.URL)) {
				writes.Store(0)
				if err := call(); err != nil {
					t.Errorf("%s: %v", name, err)
				}
				if n := writes.Load(); n != 2 {
					t.Errorf("%s: the server received %d writes, want 2", name, n)
				}
			}
		})
	}
}

// TestFilesWriteFirstAnswerIsFinal verifies that a conflict on the first
// attempt means what it says: it is returned without a repeat or a read.
func TestFilesWriteFirstAnswerIsFinal(t *testing.T) {
	for _, reply := range []filesScriptedReply{filesModified, filesExists, filesNotFound} {
		server, writes, reads := newFilesScriptedServer(t, filesCarol, reply)
		for name, call := range filesWrites(context.Background(), newFilesExportsTestClient(t, server.URL)) {
			writes.Store(0)
			if err := call(); err == nil {
				t.Errorf("%s: %d answer should fail", name, reply.status)
			}
			if n := writes.Load(); n != 1 {
				t.Errorf("%s: the server received %d writes, want 1", name, n)
			}
		}
		if n := reads.Load(); n != 0 {
			t.Errorf("%d answer: the client read the export %d times, want 0", reply.status, n)
		}
	}
}

// TestFilesWriteReconciles verifies what a repeat does with the answer the
// earlier attempt could have caused: it reads the export, and succeeds when the
// export already holds what the write asked for, without the previous value it
// cannot know. When the export holds something else, the answer is returned.
func TestFilesWriteReconciles(t *testing.T) {
	ctx := context.Background()
	rules := mustFilesRules("* rw")
	for _, tc := range []struct {
		name   string
		read   filesScriptedReply
		answer filesScriptedReply
		call   func(*AdminClient) error
	}{
		{"quota", filesCarol, filesModified, func(c *AdminClient) error {
			got, err := c.SetFilesExportQuota(ctx, "carol", 7, 1)
			if err == nil && (got.PreviousBytes != nil || got.CurrentBytes != 1 || got.Generation != 9 || got.ExportID != 104) {
				t.Errorf("quota: reply = %+v", got)
			}
			return err
		}},
		{"access", filesCarol, filesModified, func(c *AdminClient) error {
			got, err := c.SetFilesExportAccess(ctx, "carol", 7, rules)
			if err == nil && (got.PreviousRuleCount != nil || got.RuleCount != 1 || got.Generation != 9) {
				t.Errorf("access: reply = %+v", got)
			}
			return err
		}},
		{"clear", filesCarolOther, filesModified, func(c *AdminClient) error {
			got, err := c.ClearFilesExportAccess(ctx, "carol", 7)
			if err == nil && (got.PreviousRuleCount != nil || got.RuleCount != 0 || got.Generation != 9) {
				t.Errorf("clear: reply = %+v", got)
			}
			return err
		}},
		{"add", filesCarol, filesExists, func(c *AdminClient) error {
			got, err := c.AddFilesExport(ctx, FilesExportSpec{Name: "carol", Pseudo: "/home/carol", QuotaBytes: 1})
			if err == nil && (got.ExportID != 104 || got.Generation != 9) {
				t.Errorf("add: reply = %+v", got)
			}
			return err
		}},
		{"remove", filesCarol, filesNotFound, func(c *AdminClient) error {
			got, err := c.RemoveFilesExport(ctx, "carol", 7, FilesRemoveOptions{Purge: true})
			if err == nil && !got.AlreadyRemoved {
				t.Errorf("remove: reply = %+v, want AlreadyRemoved", got)
			}
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server, writes, _ := newFilesScriptedServer(t, tc.read, filesDropped, tc.answer)
			if err := tc.call(newFilesExportsTestClient(t, server.URL)); err != nil {
				t.Fatalf("err = %v, want the write reconciled", err)
			}
			if n := writes.Load(); n != 2 {
				t.Errorf("the server received %d writes, want 2", n)
			}
		})
	}

	for _, tc := range []struct {
		name string
		code string
		call func(*AdminClient) error
	}{
		{"quota", FilesErrExportModified, func(c *AdminClient) error {
			_, err := c.SetFilesExportQuota(ctx, "carol", 7, 1)
			return err
		}},
		{"access", FilesErrExportModified, func(c *AdminClient) error {
			_, err := c.SetFilesExportAccess(ctx, "carol", 7, rules)
			return err
		}},
		{"clear", FilesErrExportModified, func(c *AdminClient) error {
			_, err := c.ClearFilesExportAccess(ctx, "carol", 7)
			return err
		}},
		{"add", FilesErrExportAlreadyExists, func(c *AdminClient) error {
			_, err := c.AddFilesExport(ctx, FilesExportSpec{Name: "carol", Pseudo: "/home/carol", QuotaBytes: 1})
			return err
		}},
	} {
		t.Run(tc.name+"_held_by_another", func(t *testing.T) {
			t.Parallel()
			answer := filesModified
			if tc.code == FilesErrExportAlreadyExists {
				answer = filesExists
			}
			// The quota and access cases read an export another caller changed.
			read := filesCarolOther
			if tc.name == "clear" {
				read = filesCarol
			}
			server, _, reads := newFilesScriptedServer(t, read, filesDropped, answer)
			if err := tc.call(newFilesExportsTestClient(t, server.URL)); ToErrorResponse(err).Code != tc.code {
				t.Errorf("err = %v, want code %s", err, tc.code)
			}
			if n := reads.Load(); n != 1 {
				t.Errorf("the client read the export %d times, want 1", n)
			}
		})
	}
}

// TestAddFilesExportAcceptsAny2xx verifies that an add answered with any 2xx
// status is reported as the success it is.
func TestAddFilesExportAcceptsAny2xx(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusCreated, http.StatusAccepted} {
		server, seen := newFilesJSONServer(t, status, `{"name": "carol", "exportID": 104, "status": "pending"}`)
		got, err := newFilesExportsTestClient(t, server.URL).AddFilesExport(context.Background(),
			FilesExportSpec{Name: "carol", Pseudo: "/home/carol"})
		if err != nil || got.ExportID != 104 {
			t.Errorf("status %d: reply = %+v, err = %v", status, got, err)
		}
		if len(*seen) != 1 {
			t.Errorf("status %d: the server received %d requests, want 1", status, len(*seen))
		}
	}
}

// TestFilesWriteAcceptsBodylessSuccess verifies that a 2xx reply without a
// body, such as a 204, is a success rather than a decode error, and that a body
// cut short still fails.
func TestFilesWriteAcceptsBodylessSuccess(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusOK, http.StatusAccepted} {
		server, _ := newFilesJSONServer(t, status, ``)
		if _, err := newFilesExportsTestClient(t, server.URL).ClearFilesExportAccess(context.Background(), "carol", 7); err != nil {
			t.Errorf("status %d without a body: %v", status, err)
		}
	}

	server, _ := newFilesJSONServer(t, http.StatusOK, `{"name": "carol"`)
	if _, err := newFilesExportsTestClient(t, server.URL).ClearFilesExportAccess(context.Background(), "carol", 7); err == nil {
		t.Error("a truncated body should fail")
	}
}

// TestAddFilesExportRefusesReservedNames verifies that an export is never
// created under a name no other method can reach it by.
func TestAddFilesExportRefusesReservedNames(t *testing.T) {
	server, seen := newFilesJSONServer(t, http.StatusAccepted, `{}`)
	client := newFilesExportsTestClient(t, server.URL)
	for _, name := range []string{"stats", ".", ".."} {
		if _, err := client.AddFilesExport(context.Background(), FilesExportSpec{Name: name, Pseudo: "/home/carol"}); err == nil {
			t.Errorf("AddFilesExport(%q) should fail", name)
		}
	}
	if len(*seen) != 0 {
		t.Errorf("the server received %d requests, want 0", len(*seen))
	}
}

// TestSetFilesExportQuotaRequest verifies that a quota of zero, which clears
// the limit, is sent rather than omitted, and that the §4 sample reply decodes.
func TestSetFilesExportQuotaRequest(t *testing.T) {
	server, seen := newFilesJSONServer(t, http.StatusOK,
		`{"name": "carol", "exportID": 104, "previousBytes": 10737418240, "currentBytes": 0, "usedBytes": 1288490188}`)

	got, err := newFilesExportsTestClient(t, server.URL).SetFilesExportQuota(context.Background(), "carol", 7, 0)
	if err != nil {
		t.Fatalf("SetFilesExportQuota: %v", err)
	}
	req := (*seen)[0]
	if want := "/minio/admin/files/v1/exports/carol/quota"; req.method != http.MethodPut || req.path != want {
		t.Errorf("sent %s %s, want PUT %s", req.method, req.path, want)
	}
	if string(req.body) != `{"quotaBytes":0}` {
		t.Errorf("body = %s, want {\"quotaBytes\":0}", req.body)
	}
	if got.ExportID != 104 || got.PreviousBytes == nil || *got.PreviousBytes != 10737418240 || got.CurrentBytes != 0 ||
		got.UsedBytes == nil || *got.UsedBytes != 1288490188 {
		t.Errorf("reply = %+v", got)
	}
}

// TestSetFilesExportAccessRequest verifies that the rule list reaches the
// server exactly as given, in order and never sorted, since the order is the
// policy.
func TestSetFilesExportAccessRequest(t *testing.T) {
	server, seen := newFilesJSONServer(t, http.StatusOK,
		`{"name": "carol", "exportID": 104, "previousRuleCount": 0, "ruleCount": 3}`)
	client := newFilesExportsTestClient(t, server.URL)

	rules := mustFilesRules("10.20.9.0/24 none\n" +
		"10.20.4.8,10.20.4.7 rw\n" +
		"* ro\n")
	got, err := client.SetFilesExportAccess(context.Background(), "104", 7, rules)
	if err != nil {
		t.Fatalf("SetFilesExportAccess: %v", err)
	}
	req := (*seen)[0]
	if want := "/minio/admin/files/v1/exports/104/access"; req.method != http.MethodPut || req.path != want {
		t.Errorf("sent %s %s, want PUT %s", req.method, req.path, want)
	}
	if keys := jsonKeys(t, req.body); !slices.Equal(keys, []string{"rules"}) {
		t.Errorf("body keys = %v, want [rules]", keys)
	}
	var sent filesAccessBody
	if err := json.Unmarshal(req.body, &sent); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !slices.EqualFunc(sent.Rules.All(), rules.All(), FilesAccessRule.Equal) {
		t.Errorf("the server received %+v, want %+v", sent.Rules.All(), rules.All())
	}
	if got.PreviousRuleCount == nil || *got.PreviousRuleCount != 0 || got.RuleCount != 3 {
		t.Errorf("reply = %+v", got)
	}

	// An empty list is not a way to clear: ClearFilesExportAccess is.
	if _, err := client.SetFilesExportAccess(context.Background(), "carol", 7, filesaccess.Rules{}); err == nil {
		t.Error("SetFilesExportAccess with no rules should fail")
	}
	if len(*seen) != 1 {
		t.Errorf("the server received %d requests, want 1", len(*seen))
	}
}

// TestClearFilesExportAccessRequest verifies that clearing sends a DELETE with
// no body.
func TestClearFilesExportAccessRequest(t *testing.T) {
	server, seen := newFilesJSONServer(t, http.StatusOK,
		`{"name": "carol", "exportID": 104, "previousRuleCount": 3, "ruleCount": 0}`)

	got, err := newFilesExportsTestClient(t, server.URL).ClearFilesExportAccess(context.Background(), "carol", 7)
	if err != nil {
		t.Fatalf("ClearFilesExportAccess: %v", err)
	}
	req := (*seen)[0]
	if want := "/minio/admin/files/v1/exports/carol/access"; req.method != http.MethodDelete || req.path != want || len(req.body) != 0 {
		t.Errorf("sent %s %s with body %q, want DELETE %s with none", req.method, req.path, req.body, want)
	}
	if got.PreviousRuleCount == nil || *got.PreviousRuleCount != 3 || got.RuleCount != 0 {
		t.Errorf("reply = %+v", got)
	}
}

// TestRemoveFilesExportRequest verifies that force and purge are sent only
// when asked for.
func TestRemoveFilesExportRequest(t *testing.T) {
	server, seen := newFilesJSONServer(t, http.StatusOK, `{"name": "carol", "exportID": 104}`)
	client := newFilesExportsTestClient(t, server.URL)

	for _, tc := range []struct {
		opts  FilesRemoveOptions
		query url.Values
	}{
		{FilesRemoveOptions{}, url.Values{}},
		{FilesRemoveOptions{Force: true}, url.Values{"force": {"true"}}},
		{FilesRemoveOptions{Force: true, Purge: true}, url.Values{"force": {"true"}, "purge": {"true"}}},
	} {
		got, err := client.RemoveFilesExport(context.Background(), "carol", 7, tc.opts)
		if err != nil {
			t.Fatalf("RemoveFilesExport(%+v): %v", tc.opts, err)
		}
		req := (*seen)[len(*seen)-1]
		if want := "/minio/admin/files/v1/exports/carol"; req.method != http.MethodDelete || req.path != want {
			t.Errorf("sent %s %s, want DELETE %s", req.method, req.path, want)
		}
		if !reflect.DeepEqual(req.query, tc.query) {
			t.Errorf("RemoveFilesExport(%+v) query = %v, want %v", tc.opts, req.query, tc.query)
		}
		if got.Name != "carol" || got.ExportID != 104 {
			t.Errorf("reply = %+v", got)
		}
	}
}

// TestFilesWriteRefusesLocally verifies that no write reaches the server for a
// token that names no export.
func TestFilesWriteRefusesLocally(t *testing.T) {
	server, seen := newFilesJSONServer(t, http.StatusOK, `{}`)
	client := newFilesExportsTestClient(t, server.URL)
	ctx := context.Background()
	rules := mustFilesRules("* rw")

	for _, export := range []string{"", "stats"} {
		for name, call := range map[string]func() error{
			"quota":  func() error { _, err := client.SetFilesExportQuota(ctx, export, 7, 1); return err },
			"access": func() error { _, err := client.SetFilesExportAccess(ctx, export, 7, rules); return err },
			"clear":  func() error { _, err := client.ClearFilesExportAccess(ctx, export, 7); return err },
			"remove": func() error { _, err := client.RemoveFilesExport(ctx, export, 7, FilesRemoveOptions{}); return err },
		} {
			if err := call(); err == nil {
				t.Errorf("%s(%q) should fail", name, export)
			}
		}
	}
	if len(*seen) != 0 {
		t.Errorf("the server received %d requests, want 0", len(*seen))
	}
}

// TestFilesWriteErrorCode verifies that a refused write reaches the caller
// with the server's code.
func TestFilesWriteErrorCode(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		code string
		call func(*AdminClient) error
	}{
		{FilesErrExportAlreadyExists, func(c *AdminClient) error {
			_, err := c.AddFilesExport(ctx, FilesExportSpec{Name: "carol", Pseudo: "/home/carol"})
			return err
		}},
		{FilesErrExportInUse, func(c *AdminClient) error {
			_, err := c.RemoveFilesExport(ctx, "carol", 7, FilesRemoveOptions{})
			return err
		}},
	} {
		server, _ := newFilesJSONServer(t, http.StatusConflict, `{"code": "`+tc.code+`", "message": "refused"}`)
		if err := tc.call(newFilesExportsTestClient(t, server.URL)); ToErrorResponse(err).Code != tc.code {
			t.Errorf("err = %v, want code %s", err, tc.code)
		}
	}
}
