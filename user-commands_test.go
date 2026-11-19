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
	"sync/atomic"
	"testing"
	"time"
)

// TestAccountSummaryRequest checks the request and the decoded reply.
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

// TestAccountSummaryError checks that a non-200 reply is an error.
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

// TestBucketsInfoRequest checks the query for each option and the decoded reply.
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
		],
		"nextContinuationToken": "tok-2"
	}`

	for _, tc := range []struct {
		name  string
		opts  BucketsInfoOpts
		query url.Values
	}{
		{"first page", BucketsInfoOpts{}, url.Values{}},
		{"zero or less limit left to server", BucketsInfoOpts{Limit: -1}, url.Values{}},
		{"limit", BucketsInfoOpts{Limit: 50}, url.Values{"limit": {"50"}}},
		{
			"next page",
			BucketsInfoOpts{Limit: 50, ContinuationToken: "tok-1"},
			url.Values{"limit": {"50"}, "continuation-token": {"tok-1"}},
		},
		{"one bucket", BucketsInfoOpts{Bucket: "photos"}, url.Values{"bucket": {"photos"}}},
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
			page, err := adm.BucketsInfo(context.Background(), tc.opts)
			if err != nil {
				t.Fatalf("BucketsInfo: %v", err)
			}
			if page.NextContinuationToken != "tok-2" {
				t.Errorf("NextContinuationToken = %q, want tok-2", page.NextContinuationToken)
			}
			got := page.Buckets

			req := <-captured
			if req.method != http.MethodGet {
				t.Errorf("method = %q, want GET", req.method)
			}
			if want := "/minio/admin/v4/bucket-info"; req.path != want {
				t.Errorf("path = %q, want %q", req.path, want)
			}
			if !reflect.DeepEqual(req.query, tc.query) {
				t.Errorf("query = %v, want %v", req.query, tc.query)
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

// TestBucketsInfoError checks that a non-200 reply is an error.
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

// TestBucketsInfoRefusesPagingOneBucket checks that Bucket with paging options
// fails without sending a request.
func TestBucketsInfoRefusesPagingOneBucket(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`{"buckets":[]}`))
	}))
	defer server.Close()

	adm, err := New(mustParseHost(t, server.URL), "access", "secret", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for _, opts := range []BucketsInfoOpts{
		{Bucket: "photos", Limit: 10},
		{Bucket: "photos", Limit: -1},
		{Bucket: "photos", ContinuationToken: "tok-1"},
	} {
		if _, err := adm.BucketsInfo(context.Background(), opts); err == nil {
			t.Errorf("BucketsInfo(%+v): got nil error", opts)
		}
	}
	if n := requests.Load(); n != 0 {
		t.Errorf("sent %d requests, want 0", n)
	}
}

// TestBucketsInfoRespEmptyList checks that an empty page encodes as
// {"buckets":[]}.
func TestBucketsInfoRespEmptyList(t *testing.T) {
	b, err := json.Marshal(BucketsInfoResp{})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if want := `{"buckets":[]}`; string(b) != want {
		t.Errorf("Marshal = %s, want %s", b, want)
	}
}
