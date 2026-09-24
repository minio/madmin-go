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
