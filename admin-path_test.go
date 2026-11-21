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
	"path"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAdminPathSegment(t *testing.T) {
	for _, value := range []string{
		"tier-name",
		"TIER._~+",
		"TIER:A=B@C$D&E",
		"TIER!*()',;",
		"TIER%41",
	} {
		got, err := adminPathSegment("name", value)
		if err != nil {
			t.Fatalf("adminPathSegment(%q): %v", value, err)
		}
		if got != value {
			t.Errorf("adminPathSegment(%q) = %q, want value unchanged", value, got)
		}
	}

	for _, value := range []string{
		"",
		".",
		"..",
		"../service",
		"name/child",
		`name\child`,
		"name?force=true",
		"name#fragment",
	} {
		if _, err := adminPathSegment("name", value); err == nil {
			t.Errorf("adminPathSegment(%q) should fail", value)
		}
	}
}

func TestTierPathPreservesSupportedNames(t *testing.T) {
	prefix := path.Join(adminAPIPrefix, tierAPI) + "/"
	for _, tierName := range []string{
		"TIER._~-",
		"TIER+A:B=C@D$E&F",
		"TIER!*()',;",
		"TIER%41",
	} {
		got, err := tierPath(tierName)
		if err != nil {
			t.Fatalf("tierPath(%q): %v", tierName, err)
		}
		if want := prefix + tierName; got != want {
			t.Errorf("tierPath(%q) = %q, want %q", tierName, got, want)
		}
	}
}

func TestTierAPIsRejectRouteChangingNamesBeforeRequest(t *testing.T) {
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
		"edit": func() error {
			return client.EditTier(ctx, "tier?force=true", TierCreds{})
		},
		"remove": func() error {
			return client.RemoveTier(ctx, "tier?force=true")
		},
		"remove v2": func() error {
			return client.RemoveTierV2(ctx, "tier#fragment", RemoveTierOpts{Force: true})
		},
		"verify": func() error {
			return client.VerifyTier(ctx, "../service")
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

func TestTierRequestsPreserveSupportedNames(t *testing.T) {
	requestURIs := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURIs <- r.RequestURI
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := New(mustParseHost(t, server.URL), "ak", "sk", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, tierName := range []string{
		"TIER._~-",
		"TIER+A:B=C@D$E&F",
		"TIER!*()',;",
		"TIER%41",
	} {
		if err = client.RemoveTier(context.Background(), tierName); err != nil {
			t.Fatalf("RemoveTier(%q): %v", tierName, err)
		}
		if requestURI := <-requestURIs; !strings.HasSuffix(requestURI, "/tier/"+tierName) {
			t.Errorf("RemoveTier(%q) request URI = %q", tierName, requestURI)
		}
	}
}
