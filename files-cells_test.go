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
	"reflect"
	"testing"
	"time"
)

// TestFilesCellsRequest verifies the path and query FilesCells sends, and that
// it decodes a reply in the Files admin API's keys: a cell that serves away
// from its home, one its node does not run, and one on a node that could not
// be read.
func TestFilesCellsRequest(t *testing.T) {
	server, seen := newFilesJSONServer(t, http.StatusOK, `{
  "exportID": 4, "generation": 12,
  "cells": [
    {"cell": 0, "node": "node2:9000", "home": "node1:9000", "atHome": false, "state": "serving",
     "health": {"tookOver": true, "servingSince": "2026-11-12T10:00:00Z", "leasing": true, "held": true,
       "fenced": false, "epoch": 9, "lastRenew": "2026-11-12T10:20:00Z", "paging": true,
       "usedBytes": 1048576, "usedInodes": 42, "shipping": true, "unshippedSince": "2026-11-12T10:19:58Z",
       "shipFailed": false, "drDegraded": false, "lockGrace": true, "salvageWaits": 1,
       "salvageSince": "2026-11-12T10:00:00Z", "ts": "2026-11-12T10:20:01Z"},
     "alsoOn": ["node1:9000"]},
    {"cell": 1, "node": "node3:9000", "home": "node3:9000", "atHome": true, "state": "not-serving",
     "detail": "waiting for the cell"},
    {"cell": 2, "node": "node4:9000", "home": "node4:9000", "atHome": true, "state": "unreported",
     "detail": "connection refused"}
  ],
  "unreachableNodes": [{"node": "node4:9000", "detail": "connection refused"}]
}`)
	client := newFilesExportsTestClient(t, server.URL)

	got, err := client.FilesCells(context.Background(), 4)
	if err != nil {
		t.Fatalf("FilesCells: %v", err)
	}
	req := (*seen)[0]
	if req.method != http.MethodGet || req.path != "/minio/admin/files/v1/cells/status" || req.query.Get("exportID") != "4" || len(req.query) != 1 {
		t.Errorf("FilesCells sent %s %s?%v, want GET /minio/admin/files/v1/cells/status?exportID=4", req.method, req.path, req.query)
	}

	at := func(s string) *time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return &v
	}
	waits := 1
	want := FilesCellsResponse{
		ExportID: 4, Generation: 12,
		Cells: []FilesCell{
			{
				Cell: 0, Node: "node2:9000", Home: "node1:9000", State: FilesCellServing,
				Health: &FilesCellHealth{
					TookOver: true, ServingSince: at("2026-11-12T10:00:00Z"), Leasing: true, Held: true, Epoch: 9,
					LastRenew: at("2026-11-12T10:20:00Z"), Paging: true, UsedBytes: 1048576, UsedInodes: 42,
					Shipping: true, UnshippedSince: at("2026-11-12T10:19:58Z"), LockGrace: true,
					SalvageWaits: &waits, SalvageSince: at("2026-11-12T10:00:00Z"), TS: *at("2026-11-12T10:20:01Z"),
				},
				AlsoOn: []string{"node1:9000"},
			},
			{Cell: 1, Node: "node3:9000", Home: "node3:9000", AtHome: true, State: FilesCellNotServing, Detail: "waiting for the cell"},
			{Cell: 2, Node: "node4:9000", Home: "node4:9000", AtHome: true, State: FilesCellUnreported, Detail: "connection refused"},
		},
		UnreachableNodes: []FilesUnreachableNode{{Node: "node4:9000", Detail: "connection refused"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FilesCells decoded\n%+v\nwant\n%+v", got, want)
	}
}

// TestFilesCellsEncodeEmptyAsArray verifies that a reply with no cell reads
// as an empty list, not null.
func TestFilesCellsEncodeEmptyAsArray(t *testing.T) {
	b, err := json.Marshal(FilesCellsResponse{ExportID: 4})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if cells, ok := doc["cells"].([]any); !ok || len(cells) != 0 {
		t.Errorf("cells encoded as %v, want []", doc["cells"])
	}
}
