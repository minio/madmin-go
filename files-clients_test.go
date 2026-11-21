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
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package madmin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

// TestFilesExportClientsRequest verifies the path FilesExportClients sends, a
// name or an id escaped as one segment, and that it decodes a reply in the
// Files admin API's keys.
func TestFilesExportClientsRequest(t *testing.T) {
	server, seen := newFilesJSONServer(t, http.StatusOK, `{
  "exportID": 4,
  "nodes": {
    "node1:9000": {"nfsClients": {"65530a1c00000001": {"address": "10.0.0.7",
      "owner": "Linux NFSv4.1 compute-07", "minorVersion": 1, "leaseAgeSecs": 3,
      "opens": 2, "holdsState": true, "onExport": true}}},
    "node2:9000": {"truncated": true, "nfsClients": {"65530a1c00000002": {"address": "10.0.0.8",
      "owner": "Linux NFSv4.2 compute-08", "minorVersion": 2, "leaseAgeSecs": 41,
      "opens": 0, "holdsState": false, "onExport": false}}},
    "node3:9000": {"unreachable": "connection refused"},
    "node4:9000": {}
  }
}`)
	client := newFilesExportsTestClient(t, server.URL)

	for _, tc := range []struct{ export, rawPath string }{
		{"4", "/minio/admin/files/v1/exports/4/clients"},
		{"a b?", "/minio/admin/files/v1/exports/a%20b%3F/clients"},
	} {
		if _, err := client.FilesExportClients(context.Background(), tc.export); err != nil {
			t.Fatalf("FilesExportClients(%q): %v", tc.export, err)
		}
		req := (*seen)[len(*seen)-1]
		if req.method != http.MethodGet || req.rawPath != tc.rawPath || len(req.query) != 0 {
			t.Errorf("FilesExportClients(%q) sent %s %s?%v, want GET %s", tc.export, req.method, req.rawPath, req.query, tc.rawPath)
		}
	}
	if _, err := client.FilesExportClients(context.Background(), "stats"); err == nil {
		t.Error("FilesExportClients sent the reserved name stats")
	}

	got, err := client.FilesExportClients(context.Background(), "4")
	if err != nil {
		t.Fatalf("FilesExportClients: %v", err)
	}
	want := FilesClientsResponse{
		ExportID: 4,
		Nodes: map[string]FilesNodeNFSClients{
			"node1:9000": {NFSClients: map[string]FilesNFSClient{
				"65530a1c00000001": {
					Address: "10.0.0.7", Owner: "Linux NFSv4.1 compute-07", MinorVersion: 1,
					LeaseAgeSecs: 3, Opens: 2, HoldsState: true, OnExport: true,
				},
			}},
			"node2:9000": {Truncated: true, NFSClients: map[string]FilesNFSClient{
				"65530a1c00000002": {
					Address: "10.0.0.8", Owner: "Linux NFSv4.2 compute-08", MinorVersion: 2,
					LeaseAgeSecs: 41,
				},
			}},
			"node3:9000": {Unreachable: "connection refused"},
			"node4:9000": {},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FilesExportClients decoded\n%+v\nwant\n%+v", got, want)
	}
}

// TestEvictFilesClientRequest verifies that an eviction is one DELETE on the
// export's clients, naming its node and each client id in the query, and that
// it decodes each client's outcome.
func TestEvictFilesClientRequest(t *testing.T) {
	server, seen := newFilesJSONServer(t, http.StatusOK, `{"exportID": 4, "node": "node1:9000", "nfsClients": {
  "65530a1c00000001": {"evicting": true},
  "65530a1c00000002": {"code": "ClientNotFound", "detail": "no NFS client 65530a1c00000002"}}}`)
	client := newFilesExportsTestClient(t, server.URL)

	got, err := client.EvictFilesClient(context.Background(), "carol", "node1:9000", "65530a1c00000001", "65530a1c00000002")
	if err != nil {
		t.Fatalf("EvictFilesClient: %v", err)
	}
	want := FilesEvictResult{ExportID: 4, Node: "node1:9000", NFSClients: map[string]FilesEvictOutcome{
		"65530a1c00000001": {Evicting: true},
		"65530a1c00000002": {Code: FilesErrClientNotFound, Detail: "no NFS client 65530a1c00000002"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("EvictFilesClient decoded %+v, want %+v", got, want)
	}
	if len(*seen) != 1 {
		t.Fatalf("EvictFilesClient sent %d requests, want 1", len(*seen))
	}
	req := (*seen)[0]
	wantQuery := url.Values{"node": {"node1:9000"}, "client": {"65530a1c00000001", "65530a1c00000002"}}
	if req.method != http.MethodDelete || req.rawPath != "/minio/admin/files/v1/exports/carol/clients" || !reflect.DeepEqual(req.query, wantQuery) {
		t.Errorf("EvictFilesClient sent %s %s?%v", req.method, req.rawPath, req.query)
	}
}

// TestEvictFilesClientAll verifies that an eviction naming no client asks for
// all of them with all=true and names no client.
func TestEvictFilesClientAll(t *testing.T) {
	server, seen := newFilesJSONServer(t, http.StatusOK, `{"exportID": 4, "node": "node1:9000", "nfsClients": {}}`)
	client := newFilesExportsTestClient(t, server.URL)

	if _, err := client.EvictFilesClient(context.Background(), "carol", "node1:9000"); err != nil {
		t.Fatalf("EvictFilesClient: %v", err)
	}
	if len(*seen) != 1 {
		t.Fatalf("EvictFilesClient sent %d requests, want 1", len(*seen))
	}
	req := (*seen)[0]
	wantQuery := url.Values{"node": {"node1:9000"}, "all": {"true"}}
	if req.method != http.MethodDelete || req.rawPath != "/minio/admin/files/v1/exports/carol/clients" || !reflect.DeepEqual(req.query, wantQuery) {
		t.Errorf("EvictFilesClient sent %s %s?%v", req.method, req.rawPath, req.query)
	}
}

// TestEvictFilesClientRefusesBadArguments verifies that a missing node, a
// misspelled client id or too many ids are refused before anything is sent.
func TestEvictFilesClientRefusesBadArguments(t *testing.T) {
	server, seen := newFilesJSONServer(t, http.StatusOK, `{}`)
	client := newFilesExportsTestClient(t, server.URL)

	tooMany := make([]string, FilesMaxEvictClients+1)
	for i := range tooMany {
		tooMany[i] = "65530a1c00000001"
	}
	for _, tc := range []struct {
		node string
		ids  []string
	}{
		{"", []string{"65530a1c00000001"}},
		{"", nil},
		{"node1:9000", []string{""}},
		{"node1:9000", []string{"65530A1C00000001"}},
		{"node1:9000", []string{"65530a1c0000001"}},
		{"node1:9000", []string{"65530a1c00000001", "../../../x000000"}},
		{"node1:9000", tooMany},
	} {
		if _, err := client.EvictFilesClient(context.Background(), "carol", tc.node, tc.ids...); err == nil {
			t.Errorf("EvictFilesClient(%q, %d ids) was not refused", tc.node, len(tc.ids))
		}
	}
	if len(*seen) != 0 {
		t.Errorf("a refused eviction sent %d requests", len(*seen))
	}
}

// TestFilesEvictResultEncodesEmptyClients verifies that a result with no
// clients names an empty nfsClients object rather than null.
func TestFilesEvictResultEncodesEmptyClients(t *testing.T) {
	b, err := json.Marshal(FilesEvictResult{ExportID: 4, Node: "node1:9000"})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"exportID":4,"node":"node1:9000","nfsClients":{}}`; string(b) != want {
		t.Errorf("encoded %s, want %s", b, want)
	}
}
