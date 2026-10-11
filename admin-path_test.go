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
	"strings"
	"sync/atomic"
	"testing"
)

func TestAdminPathSegment(t *testing.T) {
	for _, value := range []string{"tier-name", "name with space", "name?force=true", "name#fragment", "日本語"} {
		got, err := adminPathSegment("name", value)
		if err != nil {
			t.Fatalf("adminPathSegment(%q): %v", value, err)
		}
		if want := url.PathEscape(value); got != want {
			t.Errorf("adminPathSegment(%q) = %q, want %q", value, got, want)
		}
	}

	for _, value := range []string{"", ".", "..", "../service", "name/child", `name\child`} {
		if _, err := adminPathSegment("name", value); err == nil {
			t.Errorf("adminPathSegment(%q) should fail", value)
		}
	}
}

func TestAdminAPIsRejectInvalidPathSegmentsBeforeRequest(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(mustParseHost(t, server.URL), "ak", "sk", false)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	tests := map[string]func() error{
		"tier": func() error {
			return client.RemoveTier(ctx, "../service")
		},
		"scanner bucket": func() error {
			_, err := client.BucketScanInfo(ctx, "bucket/status")
			return err
		},
		"inventory ID": func() error {
			_, err := client.CancelInventoryJob(ctx, "bucket", "../job")
			return err
		},
		"node hostname": func() error {
			_, err := client.NodeInfo(ctx, `node\metrics`)
			return err
		},
	}
	for name, call := range tests {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("invalid path segment should fail")
			}
		})
	}
	if got := requests.Load(); got != 0 {
		t.Errorf("server received %d requests, want 0", got)
	}
}

func TestTierPathEscapesQueryAndFragmentDelimiters(t *testing.T) {
	var requestURI string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURI = r.RequestURI
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := New(mustParseHost(t, server.URL), "ak", "sk", false)
	if err != nil {
		t.Fatal(err)
	}
	if err = client.RemoveTier(context.Background(), "tier?force=true#fragment"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(requestURI, "/tier/tier%3Fforce=true%23fragment") {
		t.Errorf("request URI = %q, want escaped tier name", requestURI)
	}
}
