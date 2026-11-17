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
	"time"
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

// TestBucketsInfoRequest verifies the path and query BucketsInfo sends, with
// and without a bucket, and that it decodes the bucket info reply. The reply
// is a literal in the server's keys, so a renamed field fails here.
func TestBucketsInfoRequest(t *testing.T) {
	const reply = `{
		"buckets": [
			{
				"name": "photos",
				"created": "2026-11-01T10:00:00Z",
				"size": 4096,
				"objects": 3,
				"versions": 5,
				"deleteMarkers": 1,
				"objectHistogram": {"LESS_THAN_1024_B": 3},
				"objectsVersionsHistogram": {"UNDER_10_VERSIONS": 3},
				"details": {"versioning": true, "locking": true, "quota": {"size": 1048576}}
			},
			{"name": "logs", "created": "2026-11-02T10:00:00Z"},
			{"name": "archive", "created": "2026-11-03T10:00:00Z", "nameOnly": true}
		]
	}`

	for _, tc := range []struct {
		name   string
		bucket string
	}{
		{"all buckets", ""},
		{"one bucket", "photos"},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
			got, err := adm.BucketsInfo(context.Background(), BucketsInfoOpts{Bucket: tc.bucket})
			if err != nil {
				t.Fatalf("BucketsInfo: %v", err)
			}

			req := <-captured
			if req.method != http.MethodGet {
				t.Errorf("method = %q, want GET", req.method)
			}
			if want := "/minio/admin/v4/bucket-info"; req.path != want {
				t.Errorf("path = %q, want %q", req.path, want)
			}
			if _, ok := req.query["bucket"]; ok != (tc.bucket != "") || req.query.Get("bucket") != tc.bucket {
				t.Errorf("bucket = %q (present %v), want %q", req.query.Get("bucket"), ok, tc.bucket)
			}

			if len(got) != 3 {
				t.Fatalf("got %d buckets, want 3", len(got))
			}
			b := got[0]
			if b.NameOnly {
				t.Errorf("photos: NameOnly = true, want false")
			}
			if b.Name != "photos" || b.Size != 4096 || b.Objects != 3 || b.Versions != 5 || b.DeleteMarkers != 1 {
				t.Errorf("counts = %+v", b)
			}
			if want := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC); !b.Created.Equal(want) {
				t.Errorf("Created = %v, want %v", b.Created, want)
			}
			if b.ObjectSizesHistogram["LESS_THAN_1024_B"] != 3 {
				t.Errorf("ObjectSizesHistogram = %v", b.ObjectSizesHistogram)
			}
			if b.ObjectVersionsHistogram["UNDER_10_VERSIONS"] != 3 {
				t.Errorf("ObjectVersionsHistogram = %v", b.ObjectVersionsHistogram)
			}
			if b.Details == nil || !b.Details.Versioning || !b.Details.Locking || b.Details.Quota == nil || b.Details.Quota.Size != 1048576 {
				t.Errorf("Details = %+v", b.Details)
			}
			if got[1].Name != "logs" || got[1].Details != nil || got[1].NameOnly {
				t.Errorf("second bucket = %+v, want logs with no details, not name-only", got[1])
			}
			n := got[2]
			if n.Name != "archive" || !n.NameOnly {
				t.Errorf("third bucket = %+v, want archive with NameOnly", n)
			}
			if want := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC); !n.Created.Equal(want) {
				t.Errorf("archive: Created = %v, want %v", n.Created, want)
			}
			if n.Size != 0 || n.Objects != 0 || n.ObjectSizesHistogram != nil || n.Details != nil {
				t.Errorf("archive = %+v, want no usage or details", n)
			}
		})
	}
}

// TestBucketsInfoError verifies that a non-200 reply becomes an error.
func TestBucketsInfoError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	adm, err := New(mustParseHost(t, server.URL), "access", "secret", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := adm.BucketsInfo(context.Background(), BucketsInfoOpts{}); err == nil {
		t.Fatal("BucketsInfo: got nil error for a 403 reply")
	}
}
