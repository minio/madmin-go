//
// Copyright (c) 2015-2024 MinIO, Inc.
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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"
)

var (
	withCreateDate    = []byte(`{"PolicyName":"readwrite","Policy":{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["admin:*"]},{"Effect":"Allow","Action":["s3:*"],"Resource":["arn:aws:s3:::*"]}]},"CreateDate":"2020-03-15T10:10:10Z","UpdateDate":"2021-03-15T10:10:10Z"}`)
	withoutCreateDate = []byte(`{"PolicyName":"readwrite","Policy":{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["admin:*"]},{"Effect":"Allow","Action":["s3:*"],"Resource":["arn:aws:s3:::*"]}]}}`)
)

func TestPolicyInfo(t *testing.T) {
	testCases := []struct {
		pi          *PolicyInfo
		expectedBuf []byte
	}{
		{
			&PolicyInfo{
				PolicyName: "readwrite",
				Policy:     []byte(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["admin:*"]},{"Effect":"Allow","Action":["s3:*"],"Resource":["arn:aws:s3:::*"]}]}`),
				CreateDate: time.Date(2020, time.March, 15, 10, 10, 10, 0, time.UTC),
				UpdateDate: time.Date(2021, time.March, 15, 10, 10, 10, 0, time.UTC),
			},
			withCreateDate,
		},
		{
			&PolicyInfo{
				PolicyName: "readwrite",
				Policy:     []byte(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["admin:*"]},{"Effect":"Allow","Action":["s3:*"],"Resource":["arn:aws:s3:::*"]}]}`),
			},
			withoutCreateDate,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run("", func(t *testing.T) {
			buf, err := json.Marshal(testCase.pi)
			if err != nil {
				t.Error(err)
			}
			if !bytes.Equal(buf, testCase.expectedBuf) {
				t.Errorf("expected %s, got %s", string(testCase.expectedBuf), string(buf))
			}
		})
	}
}

// TestCannedPolicyRequests verifies the query parameters that add, override
// and reset requests send to the add-canned-policy endpoint.
func TestCannedPolicyRequests(t *testing.T) {
	policy := []byte(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:*"],"Resource":["arn:aws:s3:::*"]}]}`)

	type request struct {
		method string
		path   string
		query  url.Values
		body   []byte
	}

	testCases := []struct {
		name     string
		call     func(ctx context.Context, adm *AdminClient) error
		want     url.Values
		wantBody []byte
	}{
		{
			name: "add",
			call: func(ctx context.Context, adm *AdminClient) error {
				return adm.AddCannedPolicy(ctx, "readwrite", policy)
			},
			want:     url.Values{"name": {"readwrite"}},
			wantBody: policy,
		},
		{
			name: "add-without-override",
			call: func(ctx context.Context, adm *AdminClient) error {
				return adm.AddCannedPolicyWithOpts(ctx, "readwrite", policy, AddCannedPolicyOpts{})
			},
			want:     url.Values{"name": {"readwrite"}},
			wantBody: policy,
		},
		{
			name: "add-with-override",
			call: func(ctx context.Context, adm *AdminClient) error {
				return adm.AddCannedPolicyWithOpts(ctx, "readwrite", policy, AddCannedPolicyOpts{OverrideBuiltin: true})
			},
			want:     url.Values{"name": {"readwrite"}, "overrideBuiltin": {"true"}},
			wantBody: policy,
		},
		{
			name: "reset",
			call: func(ctx context.Context, adm *AdminClient) error {
				return adm.ResetCannedPolicy(ctx, "readwrite")
			},
			want: url.Values{"name": {"readwrite"}, "resetBuiltin": {"true"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Hand the captured request back over a buffered channel so the
			// read below does not race with the server goroutine.
			capturedCh := make(chan request, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				capturedCh <- request{method: r.Method, path: r.URL.Path, query: r.URL.Query(), body: body}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			adm, err := New(mustParseHost(t, server.URL), "access", "secret", false)
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			if err := tc.call(context.Background(), adm); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := <-capturedCh
			if got.method != http.MethodPut {
				t.Errorf("method: expected %s, got %s", http.MethodPut, got.method)
			}
			if wantPath := "/minio/admin/v4/add-canned-policy"; got.path != wantPath {
				t.Errorf("path: expected %s, got %s", wantPath, got.path)
			}
			if got.query.Encode() != tc.want.Encode() {
				t.Errorf("query: expected %s, got %s", tc.want.Encode(), got.query.Encode())
			}
			if !bytes.Equal(got.body, tc.wantBody) {
				t.Errorf("body: expected %q, got %q", tc.wantBody, got.body)
			}
		})
	}
}

func TestPolicyInfoTypeJSON(t *testing.T) {
	policy := json.RawMessage(`{"Version":"2012-10-17","Statement":[]}`)
	testCases := []struct {
		name string
		pi   PolicyInfo
		want string
	}{
		{
			name: "system-no-dates",
			pi:   PolicyInfo{PolicyName: "Admin", Policy: policy, Type: PolicyTypeSystem},
			want: `{"PolicyName":"Admin","Policy":{"Version":"2012-10-17","Statement":[]},"type":"System"}`,
		},
		{
			name: "legacy-no-dates",
			pi: PolicyInfo{
				PolicyName: "consoleAdmin", Policy: policy,
				Type: PolicyTypeLegacy, Legacy: true, ReplacedBy: "Admin",
			},
			want: `{"PolicyName":"consoleAdmin","Policy":{"Version":"2012-10-17","Statement":[]},` +
				`"type":"Legacy","legacy":true,"replacedBy":"Admin"}`,
		},
		{
			name: "conflict-with-dates",
			pi: PolicyInfo{
				PolicyName: "Admin", Policy: policy,
				CreateDate: time.Date(2026, time.October, 1, 10, 0, 0, 0, time.UTC),
				UpdateDate: time.Date(2026, time.October, 2, 10, 0, 0, 0, time.UTC),
				Type:       PolicyTypeUserDefined, Conflict: true,
			},
			want: `{"PolicyName":"Admin","Policy":{"Version":"2012-10-17","Statement":[]},` +
				`"CreateDate":"2026-10-01T10:00:00Z","UpdateDate":"2026-10-02T10:00:00Z",` +
				`"type":"UserDefined","conflict":true}`,
		},
		{
			name: "modified-legacy-with-dates",
			pi: PolicyInfo{
				PolicyName: "readwrite", Policy: policy,
				CreateDate: time.Date(2026, time.October, 1, 10, 0, 0, 0, time.UTC),
				UpdateDate: time.Date(2026, time.October, 2, 10, 0, 0, 0, time.UTC),
				Type:       PolicyTypeUserDefined, Legacy: true, ReplacedBy: "ObjectsReadWrite",
			},
			want: `{"PolicyName":"readwrite","Policy":{"Version":"2012-10-17","Statement":[]},` +
				`"CreateDate":"2026-10-01T10:00:00Z","UpdateDate":"2026-10-02T10:00:00Z",` +
				`"type":"UserDefined","legacy":true,"replacedBy":"ObjectsReadWrite"}`,
		},
		{
			name: "no-type",
			pi:   PolicyInfo{PolicyName: "p", Policy: policy},
			want: `{"PolicyName":"p","Policy":{"Version":"2012-10-17","Statement":[]}}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf, err := json.Marshal(tc.pi)
			if err != nil {
				t.Fatal(err)
			}
			if string(buf) != tc.want {
				t.Fatalf("marshal:\nwant %s\ngot  %s", tc.want, buf)
			}
			var got PolicyInfo
			if err := json.Unmarshal(buf, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.pi) {
				t.Fatalf("round trip: want %+v, got %+v", tc.pi, got)
			}
		})
	}
}

func TestPolicyListEntryJSON(t *testing.T) {
	testCases := []struct {
		name string
		e    PolicyListEntry
		want string
	}{
		{
			name: "system-no-dates",
			e:    PolicyListEntry{Name: "Admin", Type: PolicyTypeSystem},
			want: `{"name":"Admin","type":"System"}`,
		},
		{
			name: "legacy-no-dates",
			e:    PolicyListEntry{Name: "readonly", Type: PolicyTypeLegacy, Legacy: true, ReplacedBy: "ObjectsRead"},
			want: `{"name":"readonly","type":"Legacy","legacy":true,"replacedBy":"ObjectsRead"}`,
		},
		{
			name: "conflict-with-dates",
			e: PolicyListEntry{
				Name: "Admin", Type: PolicyTypeUserDefined, Conflict: true,
				CreateDate: time.Date(2026, time.October, 1, 10, 0, 0, 0, time.UTC),
				UpdateDate: time.Date(2026, time.October, 2, 10, 0, 0, 0, time.UTC),
			},
			want: `{"name":"Admin","type":"UserDefined","conflict":true,` +
				`"CreateDate":"2026-10-01T10:00:00Z","UpdateDate":"2026-10-02T10:00:00Z"}`,
		},
		{
			name: "no-type",
			e:    PolicyListEntry{Name: "p"},
			want: `{"name":"p"}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf, err := json.Marshal(tc.e)
			if err != nil {
				t.Fatal(err)
			}
			if string(buf) != tc.want {
				t.Fatalf("marshal:\nwant %s\ngot  %s", tc.want, buf)
			}
			var got PolicyListEntry
			if err := json.Unmarshal(buf, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.e) {
				t.Fatalf("round trip: want %+v, got %+v", tc.e, got)
			}
		})
	}
}

func TestListCannedPolicyInfo(t *testing.T) {
	created := time.Date(2026, time.October, 1, 10, 0, 0, 0, time.UTC)
	testCases := []struct {
		name string
		body string
		want []PolicyListEntry
	}{
		{
			name: "v2-array",
			body: `[{"name":"readonly","type":"Legacy","legacy":true,"replacedBy":"ObjectsRead"},` +
				`{"name":"Admin","type":"UserDefined","conflict":true,"CreateDate":"2026-10-01T10:00:00Z","UpdateDate":"2026-10-01T10:00:00Z"},` +
				`{"name":"ObjectsRead","type":"System"}]`,
			want: []PolicyListEntry{
				{Name: "Admin", Type: PolicyTypeUserDefined, Conflict: true, CreateDate: created, UpdateDate: created},
				{Name: "ObjectsRead", Type: PolicyTypeSystem},
				{Name: "readonly", Type: PolicyTypeLegacy, Legacy: true, ReplacedBy: "ObjectsRead"},
			},
		},
		{
			name: "v2-empty",
			body: `[]`,
			want: []PolicyListEntry{},
		},
		{
			name: "old-server-object",
			body: "\n  " + `{"readwrite":{"Version":"2012-10-17","Statement":[]},"diagnostics":{"Version":"2012-10-17","Statement":[]},"consoleAdmin":{"Version":"2012-10-17","Statement":[]}}`,
			want: []PolicyListEntry{{Name: "consoleAdmin"}, {Name: "diagnostics"}, {Name: "readwrite"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			adm, ch := newCaptureServer(t, http.StatusOK, tc.body)
			got, err := adm.ListCannedPolicyInfo(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			req := <-ch
			if req.method != http.MethodGet {
				t.Errorf("method: want %s, got %s", http.MethodGet, req.method)
			}
			if want := "/minio/admin/v4/list-canned-policies"; req.path != want {
				t.Errorf("path: want %s, got %s", want, req.path)
			}
			if req.query != "v=2" {
				t.Errorf("query: want v=2, got %s", req.query)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("entries:\nwant %+v\ngot  %+v", tc.want, got)
			}
		})
	}

	t.Run("error", func(t *testing.T) {
		adm, _ := newCaptureServer(t, http.StatusForbidden,
			`<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>Access Denied.</Message></Error>`)
		if _, err := adm.ListCannedPolicyInfo(context.Background()); err == nil {
			t.Fatal("expected an error")
		}
	})
}
