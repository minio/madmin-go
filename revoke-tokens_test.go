//
// Copyright (c) 2015-2026 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.
//

package madmin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestRevokeTokensProviders(t *testing.T) {
	type request struct {
		path  string
		query url.Values
	}
	for _, tc := range []struct {
		name     string
		call     func(adm *AdminClient) error
		provider string
		issuer   string
	}{
		{"builtin", func(adm *AdminClient) error {
			return adm.RevokeTokens(context.Background(), RevokeTokensReq{User: "alice", FullRevoke: true})
		}, BuiltinProvider, ""},
		{"ldap", func(adm *AdminClient) error {
			return adm.RevokeTokensLDAP(context.Background(), RevokeTokensReq{User: "alice", FullRevoke: true})
		}, LDAPProvider, ""},
		{"openid with an issuer", func(adm *AdminClient) error {
			return adm.RevokeTokensOpenID(context.Background(), RevokeTokensReq{User: "alice", FullRevoke: true}, "https://idp.example")
		}, OpenIDProvider, "https://idp.example"},
		{"openid with the only issuer", func(adm *AdminClient) error {
			return adm.RevokeTokensOpenID(context.Background(), RevokeTokensReq{User: "alice", TokenRevokeType: "t"}, "")
		}, OpenIDProvider, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			captured := make(chan request, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				captured <- request{path: r.URL.Path, query: r.URL.Query()}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			adm, err := New(strings.TrimPrefix(server.URL, "http://"), "access", "secret", false)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if err := tc.call(adm); err != nil {
				t.Fatalf("revoke: %v", err)
			}
			got := <-captured
			if want := "/revoke-tokens/" + tc.provider; !strings.HasSuffix(got.path, want) {
				t.Fatalf("path %q, want suffix %q", got.path, want)
			}
			if got.query.Get("user") != "alice" {
				t.Fatalf("user = %q", got.query.Get("user"))
			}
			if _, ok := got.query["issuer"]; ok != (tc.issuer != "") || got.query.Get("issuer") != tc.issuer {
				t.Fatalf("issuer = %q (present %v), want %q", got.query.Get("issuer"), ok, tc.issuer)
			}
		})
	}
}
