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
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"regexp"
	"slices"
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
	method  string
	path    string
	rawPath string
	query   url.Values
}

// newFilesJSONServer answers every request with status and body, and records
// each request it saw.
func newFilesJSONServer(t *testing.T, status int, body string) (*httptest.Server, *[]filesRequest) {
	t.Helper()
	var seen []filesRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, filesRequest{r.Method, r.URL.Path, r.URL.EscapedPath(), r.URL.Query()})
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
    {"name": "carol", "exportId": 104, "pseudo": "/home/carol", "node": "node03.example.com",
     "status": "serving", "usedBytes": 1288490188, "quotaBytes": 21474836480},
    {"name": "frank", "exportId": 107, "pseudo": "/home/frank", "node": "node02.example.com",
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
  "name": "carol", "exportId": 104, "pseudo": "/home/carol", "node": "node03.example.com",
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
			AccessRules: []FilesAccessRule{
				{Clients: []string{"10.20.9.0/24"}, AccessType: FilesAccessNone},
				{Clients: []string{"10.20.4.7", "10.20.4.8"}, AccessType: FilesAccessRW},
				{Clients: []string{"*.corp.example.com", "10.20.0.0/16"}, AccessType: FilesAccessRO},
			},
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
    {"name": "carol", "exportId": 104, "node": "node03.example.com", "status": "serving",
     "usedBytes": 1288490188, "limitBytes": 21474836480},
    {"name": "frank", "exportId": 107, "node": "node02.example.com", "status": "unreachable",
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
// retry so its connection is not leaked.
func TestAdmin426RetriesOnOldPrefix(t *testing.T) {
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
