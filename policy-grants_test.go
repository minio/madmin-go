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
	"reflect"
	"testing"
	"time"
)

func samplePolicyGrants() PolicyGrants {
	return PolicyGrants{
		Objects: []ObjectGrant{
			{Access: ObjectAccessReadWrite, Bucket: "analytics", Prefix: "reports/2026/", Browse: true},
			{Access: ObjectAccessRead, Bucket: "team-a-*"},
		},
		Tables: []TableGrant{
			{Access: TableAccessQuery, Warehouse: "sales-warehouse", Namespace: "orders.eu", Table: "daily"},
			{Access: TableAccessManage, Warehouse: "lake-*"},
		},
		BucketAdmin: []BucketAdmin{
			{Bucket: "uploads", Settings: true, AccessPolicy: true, ObjectLock: true, Delete: true},
		},
		TableBucketAdmin: []TableBucketAdmin{
			{Warehouse: "sales-warehouse", Settings: true, Delete: true},
		},
		CreateBuckets:      []string{"team-a-*"},
		CreateTableBuckets: []string{"lake-*"},
	}
}

func TestPolicyGrantsJSON(t *testing.T) {
	testCases := []struct {
		name string
		g    PolicyGrants
		want string
	}{
		{
			name: "empty",
			g:    PolicyGrants{},
			want: `{}`,
		},
		{
			name: "minimal",
			g: PolicyGrants{
				Objects:          []ObjectGrant{{Access: ObjectAccessWrite, Bucket: "b"}},
				Tables:           []TableGrant{{Access: TableAccessQueryWrite, Warehouse: "w"}},
				BucketAdmin:      []BucketAdmin{{Bucket: "b"}},
				TableBucketAdmin: []TableBucketAdmin{{Warehouse: "w"}},
			},
			want: `{"objects":[{"access":"write","bucket":"b"}],"tables":[{"access":"query-write","warehouse":"w"}],"bucketAdmin":[{"bucket":"b"}],"tableBucketAdmin":[{"warehouse":"w"}]}`,
		},
		{
			name: "full",
			g:    samplePolicyGrants(),
			want: `{"objects":[{"access":"readwrite","bucket":"analytics","prefix":"reports/2026/","browse":true},{"access":"read","bucket":"team-a-*"}],` +
				`"tables":[{"access":"query","warehouse":"sales-warehouse","namespace":"orders.eu","table":"daily"},{"access":"manage","warehouse":"lake-*"}],` +
				`"bucketAdmin":[{"bucket":"uploads","settings":true,"accessPolicy":true,"objectLock":true,"delete":true}],` +
				`"tableBucketAdmin":[{"warehouse":"sales-warehouse","settings":true,"delete":true}],` +
				`"createBuckets":["team-a-*"],"createTableBuckets":["lake-*"]}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf, err := json.Marshal(tc.g)
			if err != nil {
				t.Fatal(err)
			}
			if string(buf) != tc.want {
				t.Fatalf("marshal:\nwant %s\ngot  %s", tc.want, buf)
			}
			var got PolicyGrants
			if err := json.Unmarshal(buf, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.g) {
				t.Fatalf("round trip: want %+v, got %+v", tc.g, got)
			}
		})
	}
}

func TestPolicyGrantsIsEmpty(t *testing.T) {
	if !(PolicyGrants{}).IsEmpty() {
		t.Error("zero PolicyGrants should be empty")
	}
	if !(PolicyGrants{Objects: []ObjectGrant{}, CreateBuckets: []string{}}).IsEmpty() {
		t.Error("PolicyGrants with empty slices should be empty")
	}
	nonEmpty := []PolicyGrants{
		{Objects: []ObjectGrant{{}}},
		{Tables: []TableGrant{{}}},
		{BucketAdmin: []BucketAdmin{{}}},
		{TableBucketAdmin: []TableBucketAdmin{{}}},
		{CreateBuckets: []string{"b"}},
		{CreateTableBuckets: []string{"w"}},
	}
	for i, g := range nonEmpty {
		if g.IsEmpty() {
			t.Errorf("case %d: %+v should not be empty", i, g)
		}
	}
}

func TestPolicyGrantsCompileResultJSON(t *testing.T) {
	testCases := []struct {
		name string
		r    PolicyGrantsCompileResult
		want string
	}{
		{
			name: "empty",
			r:    PolicyGrantsCompileResult{},
			want: `{}`,
		},
		{
			name: "compiled",
			r: PolicyGrantsCompileResult{
				Policy:  json.RawMessage(`{"Version":"2012-10-17","Statement":[]}`),
				Summary: []PolicySummaryLine{{Kind: PolicySummaryObject, Text: "List objects in analytics"}},
				Warnings: []PolicyWarning{
					{Code: PolicyWarningBucketNotVersioned, Bucket: "analytics", Message: "analytics is not versioned"},
					{Code: PolicyWarningBucketPattern, Bucket: "team-a-*", Matches: []string{"team-a-logs"}, Message: "pattern"},
				},
				SuggestedName: "rw-analytics-reports-2026",
			},
			want: `{"policy":{"Version":"2012-10-17","Statement":[]},` +
				`"summary":[{"kind":"object","text":"List objects in analytics"}],` +
				`"warnings":[{"code":"BucketNotVersioned","bucket":"analytics","message":"analytics is not versioned"},` +
				`{"code":"BucketPattern","bucket":"team-a-*","matches":["team-a-logs"],"message":"pattern"}],` +
				`"suggestedName":"rw-analytics-reports-2026"}`,
		},
		{
			name: "invalid",
			r: PolicyGrantsCompileResult{
				Warnings: []PolicyWarning{{Code: PolicyWarningDeleteBucket, Message: "m"}},
				Errors:   []PolicyGrantsFieldError{{Field: "objects[0].bucket", Message: "required"}},
			},
			want: `{"warnings":[{"code":"DeleteBucket","message":"m"}],"errors":[{"field":"objects[0].bucket","message":"required"}]}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf, err := json.Marshal(tc.r)
			if err != nil {
				t.Fatal(err)
			}
			if string(buf) != tc.want {
				t.Fatalf("marshal:\nwant %s\ngot  %s", tc.want, buf)
			}
			var got PolicyGrantsCompileResult
			if err := json.Unmarshal(buf, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.r) {
				t.Fatalf("round trip: want %+v, got %+v", tc.r, got)
			}
		})
	}
}

func TestPolicyGrantsCompileResultErr(t *testing.T) {
	if err := (PolicyGrantsCompileResult{}).Err(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	r := PolicyGrantsCompileResult{Errors: []PolicyGrantsFieldError{
		{Field: "objects[1].prefix", Message: "must not contain *"},
		{Field: "tables[0].warehouse", Message: "required"},
	}}
	err := r.Err()
	var pge *PolicyGrantsError
	if !errors.As(err, &pge) {
		t.Fatalf("expected *PolicyGrantsError, got %T", err)
	}
	if !reflect.DeepEqual(pge.Errors, r.Errors) {
		t.Fatalf("errors: want %+v, got %+v", r.Errors, pge.Errors)
	}
	want := "invalid policy rules: objects[1].prefix: must not contain *; tables[0].warehouse: required"
	if err.Error() != want {
		t.Fatalf("message: want %q, got %q", want, err.Error())
	}
}

func TestPolicyInfoGrantsJSON(t *testing.T) {
	policy := json.RawMessage(`{"Version":"2012-10-17","Statement":[]}`)
	grants := &PolicyGrants{Objects: []ObjectGrant{{Access: ObjectAccessRead, Bucket: "b"}}}
	summary := []PolicySummaryLine{{Kind: PolicySummaryObject, Text: "Download objects in b"}}
	grantsJSON := `"grants":{"objects":[{"access":"read","bucket":"b"}]},"grantsVersion":1,"grantsCurrent":true,` +
		`"summary":[{"kind":"object","text":"Download objects in b"}]`

	testCases := []struct {
		name string
		pi   PolicyInfo
		want string
	}{
		{
			name: "no-dates-with-grants",
			pi: PolicyInfo{
				PolicyName: "p", Policy: policy,
				Grants: grants, GrantsVersion: 1, GrantsCurrent: true, Summary: summary,
			},
			want: `{"PolicyName":"p","Policy":{"Version":"2012-10-17","Statement":[]},` + grantsJSON + `}`,
		},
		{
			name: "dates-with-grants",
			pi: PolicyInfo{
				PolicyName: "p", Policy: policy,
				CreateDate: time.Date(2026, time.October, 1, 10, 0, 0, 0, time.UTC),
				UpdateDate: time.Date(2026, time.October, 2, 10, 0, 0, 0, time.UTC),
				Grants:     grants, GrantsVersion: 1, GrantsCurrent: true, Summary: summary,
			},
			want: `{"PolicyName":"p","Policy":{"Version":"2012-10-17","Statement":[]},` +
				`"CreateDate":"2026-10-01T10:00:00Z","UpdateDate":"2026-10-02T10:00:00Z",` + grantsJSON + `}`,
		},
		{
			name: "dates-without-grants",
			pi: PolicyInfo{
				PolicyName: "p", Policy: policy,
				CreateDate: time.Date(2026, time.October, 1, 10, 0, 0, 0, time.UTC),
				UpdateDate: time.Date(2026, time.October, 2, 10, 0, 0, 0, time.UTC),
			},
			want: `{"PolicyName":"p","Policy":{"Version":"2012-10-17","Statement":[]},` +
				`"CreateDate":"2026-10-01T10:00:00Z","UpdateDate":"2026-10-02T10:00:00Z"}`,
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

type capturedRequest struct {
	method string
	path   string
	query  string
	body   []byte
}

func newCaptureServer(t *testing.T, status int, respBody string) (*AdminClient, <-chan capturedRequest) {
	t.Helper()
	ch := make(chan capturedRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		ch <- capturedRequest{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: body}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, respBody)
	}))
	t.Cleanup(server.Close)

	adm, err := New(mustParseHost(t, server.URL), "access", "secret", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return adm, ch
}

func TestCompilePolicyGrants(t *testing.T) {
	g := samplePolicyGrants()

	checkRequest := func(t *testing.T, ch <-chan capturedRequest) {
		t.Helper()
		got := <-ch
		if got.method != http.MethodPost {
			t.Errorf("method: want %s, got %s", http.MethodPost, got.method)
		}
		if want := "/minio/admin/v4/policy-grants/compile"; got.path != want {
			t.Errorf("path: want %s, got %s", want, got.path)
		}
		var req PolicyGrantsCompileRequest
		if err := json.Unmarshal(got.body, &req); err != nil {
			t.Fatalf("request body %q: %v", got.body, err)
		}
		if !reflect.DeepEqual(req.Grants, g) {
			t.Errorf("request grants: want %+v, got %+v", g, req.Grants)
		}
	}

	t.Run("compiled", func(t *testing.T) {
		want := PolicyGrantsCompileResult{
			Policy:        json.RawMessage(`{"Version":"2012-10-17","Statement":[]}`),
			Summary:       []PolicySummaryLine{{Kind: PolicySummaryTable, Text: "Query tables in sales-warehouse"}},
			Warnings:      []PolicyWarning{{Code: PolicyWarningBucketNotFound, Bucket: "analytics", Message: "missing"}},
			SuggestedName: "rw-analytics-reports-2026",
		}
		respBody, err := json.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		adm, ch := newCaptureServer(t, http.StatusOK, string(respBody))

		got, err := adm.CompilePolicyGrants(context.Background(), g)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		checkRequest(t, ch)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("result: want %+v, got %+v", want, got)
		}
		if err := got.Err(); err != nil {
			t.Fatalf("Err: want nil, got %v", err)
		}
	})

	t.Run("validation-errors", func(t *testing.T) {
		respBody := `{"errors":[{"field":"objects[0].prefix","message":"must not contain *"},{"field":"createBuckets[0]","message":"invalid bucket name"}]}`
		adm, ch := newCaptureServer(t, http.StatusOK, respBody)

		got, err := adm.CompilePolicyGrants(context.Background(), g)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		checkRequest(t, ch)
		if len(got.Policy) != 0 || len(got.Summary) != 0 || got.SuggestedName != "" {
			t.Fatalf("expected empty policy, summary and name, got %+v", got)
		}
		gotErr := got.Err()
		if gotErr == nil {
			t.Fatal("Err: want error, got nil")
		}
		want := "invalid policy rules: objects[0].prefix: must not contain *; createBuckets[0]: invalid bucket name"
		if gotErr.Error() != want {
			t.Fatalf("Err: want %q, got %q", want, gotErr.Error())
		}
	})

	t.Run("admin-error", func(t *testing.T) {
		respBody := `{"Code":"AccessDenied","Message":"Access Denied."}`
		adm, ch := newCaptureServer(t, http.StatusForbidden, respBody)

		_, err := adm.CompilePolicyGrants(context.Background(), g)
		checkRequest(t, ch)
		var errResp ErrorResponse
		if !errors.As(err, &errResp) {
			t.Fatalf("expected ErrorResponse, got %T: %v", err, err)
		}
		if errResp.StatusCode != http.StatusForbidden || errResp.Code != "AccessDenied" {
			t.Fatalf("unexpected error response: %+v", errResp)
		}
	})
}

func TestInfoCannedPolicyGrants(t *testing.T) {
	respBody := `{"Version":1,"policy":{"Version":"2012-10-17","Statement":[]},` +
		`"CreateDate":"2026-10-01T10:00:00Z","UpdateDate":"2026-10-02T10:00:00Z",` +
		`"grants":{"objects":[{"access":"readwrite","bucket":"analytics","prefix":"reports/2026/","browse":true}]},` +
		`"grantsVersion":1,"grantsCurrent":true,` +
		`"summary":[{"kind":"object","text":"List, download, upload and delete objects in analytics/reports/2026/"}]}`
	adm, ch := newCaptureServer(t, http.StatusOK, respBody)

	got, err := adm.InfoCannedPolicy(context.Background(), "rw-analytics")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req := <-ch
	if req.method != http.MethodGet || req.path != "/minio/admin/v4/info-canned-policy" || req.query != "name=rw-analytics&v=2" {
		t.Fatalf("unexpected request %s %s?%s", req.method, req.path, req.query)
	}

	want := &PolicyInfo{
		Policy:     json.RawMessage(`{"Version":"2012-10-17","Statement":[]}`),
		CreateDate: time.Date(2026, time.October, 1, 10, 0, 0, 0, time.UTC),
		UpdateDate: time.Date(2026, time.October, 2, 10, 0, 0, 0, time.UTC),
		Grants: &PolicyGrants{Objects: []ObjectGrant{
			{Access: ObjectAccessReadWrite, Bucket: "analytics", Prefix: "reports/2026/", Browse: true},
		}},
		GrantsVersion: 1,
		GrantsCurrent: true,
		Summary: []PolicySummaryLine{
			{Kind: PolicySummaryObject, Text: "List, download, upload and delete objects in analytics/reports/2026/"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %+v, got %+v", want, got)
	}
}
