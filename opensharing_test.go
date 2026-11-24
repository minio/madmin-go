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
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type recordedRequest struct {
	method string
	path   string
	query  string
	body   string
	auth   string
}

// sharingTestServer answers every request with status and body, and records
// the last request.
func sharingTestServer(t *testing.T, status int, body string) (*httptest.Server, *recordedRequest) {
	t.Helper()
	rec := &recordedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*rec = recordedRequest{
			method: r.Method,
			path:   r.URL.EscapedPath(),
			query:  r.URL.RawQuery,
			body:   string(b),
			auth:   r.Header.Get("Authorization"),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

func testAdminClient(t *testing.T, srv *httptest.Server) *AdminClient {
	t.Helper()
	adm, err := New(strings.TrimPrefix(srv.URL, "http://"), "access", "secret", false)
	if err != nil {
		t.Fatal(err)
	}
	return adm
}

func TestSharingProviderRequests(t *testing.T) {
	ctx := context.Background()
	comment := "new comment"
	tests := []struct {
		name      string
		status    int
		resp      string
		call      func(*AdminClient) (any, error)
		method    string
		path      string
		query     string
		body      string
		checkResp func(t *testing.T, out any)
	}{
		{
			name:   "create share",
			status: http.StatusCreated,
			resp:   `{"id":"s1","name":"sales"}`,
			call: func(adm *AdminClient) (any, error) {
				return adm.CreateSharingShare(ctx, CreateSharingShareRequest{Name: "sales", Comment: "q3"})
			},
			method: http.MethodPost,
			path:   "/_delta-sharing/v1/provider/shares",
			body:   `{"name":"sales","comment":"q3"}`,
			checkResp: func(t *testing.T, out any) {
				if s := out.(*SharingShare); s.ID != "s1" || s.Name != "sales" {
					t.Fatalf("got %+v", s)
				}
			},
		},
		{
			name:   "list shares with paging",
			status: http.StatusOK,
			resp:   `{"items":[{"id":"s1","name":"sales"}],"nextPageToken":"p2"}`,
			call: func(adm *AdminClient) (any, error) {
				return adm.ListSharingShares(ctx, SharingListOptions{MaxResults: 10, PageToken: "p1"})
			},
			method: http.MethodGet,
			path:   "/_delta-sharing/v1/provider/shares",
			query:  "maxResults=10&pageToken=p1",
			checkResp: func(t *testing.T, out any) {
				if p := out.(*SharingSharesPage); len(p.Items) != 1 || p.NextPageToken != "p2" {
					t.Fatalf("got %+v", p)
				}
			},
		},
		{
			name:   "get share with data objects",
			status: http.StatusOK,
			resp:   `{"id":"s1","name":"a b","objects":[{"name":"bkt/docs","type":"VOLUME","sharedAs":"files.docs"}]}`,
			call: func(adm *AdminClient) (any, error) {
				return adm.GetSharingShare(ctx, "a b", true)
			},
			method: http.MethodGet,
			path:   "/_delta-sharing/v1/provider/shares/a%20b",
			query:  "include_shared_data=true",
			checkResp: func(t *testing.T, out any) {
				s := out.(*SharingShare)
				if len(s.Objects) != 1 || s.Objects[0].Type != SharingObjectVolume || s.Objects[0].SharedAs != "files.docs" {
					t.Fatalf("got %+v", s)
				}
			},
		},
		{
			name:   "update share",
			status: http.StatusOK,
			resp:   `{"id":"s1","name":"sales","comment":"new comment"}`,
			call: func(adm *AdminClient) (any, error) {
				return adm.UpdateSharingShare(ctx, "sales", UpdateSharingShareRequest{
					Comment: &comment,
					Updates: []SharingShareUpdate{{
						Action:     SharingUpdateAdd,
						DataObject: SharingDataObject{Type: SharingObjectVolume, Name: "bkt/docs", SharedAs: "files.docs"},
					}},
				})
			},
			method: http.MethodPatch,
			path:   "/_delta-sharing/v1/provider/shares/sales",
			body:   `{"comment":"new comment","updates":[{"action":"ADD","dataObject":{"name":"bkt/docs","type":"VOLUME","sharedAs":"files.docs"}}]}`,
		},
		{
			name:   "delete share",
			status: http.StatusNoContent,
			call: func(adm *AdminClient) (any, error) {
				return nil, adm.DeleteSharingShare(ctx, "sales")
			},
			method: http.MethodDelete,
			path:   "/_delta-sharing/v1/provider/shares/sales",
		},
		{
			name:   "list share grants",
			status: http.StatusOK,
			resp:   `{"items":[{"shareName":"sales","recipientName":"acme","privilege":"SELECT","grantedAt":"2026-11-24T10:00:00Z"}]}`,
			call: func(adm *AdminClient) (any, error) {
				return adm.ListSharingShareGrants(ctx, "sales", SharingListOptions{})
			},
			method: http.MethodGet,
			path:   "/_delta-sharing/v1/provider/shares/sales/permissions",
			checkResp: func(t *testing.T, out any) {
				p := out.(*SharingGrantsPage)
				if len(p.Items) != 1 || p.Items[0].RecipientName != "acme" || p.Items[0].GrantedAt.IsZero() {
					t.Fatalf("got %+v", p)
				}
			},
		},
		{
			name:   "grant and revoke",
			status: http.StatusOK,
			resp:   `{"items":[]}`,
			call: func(adm *AdminClient) (any, error) {
				return adm.UpdateSharingShareGrants(ctx, "sales", []SharingGrantChange{
					{RecipientName: "acme", Add: []string{SharingPrivilegeSelect}},
					{RecipientName: "globex", Remove: []string{SharingPrivilegeSelect}},
				})
			},
			method: http.MethodPatch,
			path:   "/_delta-sharing/v1/provider/shares/sales/permissions",
			body:   `{"changes":[{"recipientName":"acme","add":["SELECT"]},{"recipientName":"globex","remove":["SELECT"]}]}`,
		},
		{
			name:   "create recipient defaults to token authentication",
			status: http.StatusCreated,
			resp:   `{"id":"r1","name":"acme","authenticationType":"TOKEN","activationUrl":"https://x/_delta-sharing/v1/activations/c"}`,
			call: func(adm *AdminClient) (any, error) {
				return adm.CreateSharingRecipient(ctx, CreateSharingRecipientRequest{Name: "acme", TokenExpirationDays: 7})
			},
			method: http.MethodPost,
			path:   "/_delta-sharing/v1/provider/recipients",
			body:   `{"name":"acme","authenticationType":"TOKEN","tokenExpirationDays":7}`,
			checkResp: func(t *testing.T, out any) {
				if r := out.(*SharingRecipient); r.ActivationURL == "" {
					t.Fatalf("got %+v", r)
				}
			},
		},
		{
			name:   "list recipients",
			status: http.StatusOK,
			resp:   `{"items":[{"id":"r1","name":"acme","authenticationType":"TOKEN"}]}`,
			call: func(adm *AdminClient) (any, error) {
				return adm.ListSharingRecipients(ctx, SharingListOptions{MaxResults: 1})
			},
			method: http.MethodGet,
			path:   "/_delta-sharing/v1/provider/recipients",
			query:  "maxResults=1",
		},
		{
			name:   "get recipient",
			status: http.StatusOK,
			resp:   `{"id":"r1","name":"acme","authenticationType":"TOKEN"}`,
			call: func(adm *AdminClient) (any, error) {
				return adm.GetSharingRecipient(ctx, "acme")
			},
			method: http.MethodGet,
			path:   "/_delta-sharing/v1/provider/recipients/acme",
		},
		{
			name:   "update recipient",
			status: http.StatusOK,
			resp:   `{"id":"r1","name":"acme","authenticationType":"TOKEN","comment":"new comment"}`,
			call: func(adm *AdminClient) (any, error) {
				return adm.UpdateSharingRecipient(ctx, "acme", UpdateSharingRecipientRequest{Comment: &comment})
			},
			method: http.MethodPatch,
			path:   "/_delta-sharing/v1/provider/recipients/acme",
			body:   `{"comment":"new comment"}`,
		},
		{
			name:   "delete recipient",
			status: http.StatusNoContent,
			call: func(adm *AdminClient) (any, error) {
				return nil, adm.DeleteSharingRecipient(ctx, "acme")
			},
			method: http.MethodDelete,
			path:   "/_delta-sharing/v1/provider/recipients/acme",
		},
		{
			name:   "rotate token sends a zero grace period",
			status: http.StatusCreated,
			resp:   `{"tokenId":"t2","recipientName":"acme","activationUrl":"https://x/a/c2","expiresAt":"2026-12-24T10:00:00Z"}`,
			call: func(adm *AdminClient) (any, error) {
				return adm.RotateSharingRecipientToken(ctx, "acme", RotateSharingRecipientTokenRequest{TokenExpirationDays: 30})
			},
			method: http.MethodPost,
			path:   "/_delta-sharing/v1/provider/recipients/acme/rotate-token",
			body:   `{"tokenExpirationDays":30,"existingTokenExpireInSeconds":0}`,
			checkResp: func(t *testing.T, out any) {
				r := out.(*RotateSharingRecipientTokenResponse)
				if r.TokenID != "t2" || r.ExpiresAt == nil {
					t.Fatalf("got %+v", r)
				}
			},
		},
		{
			name:   "list recipient grants",
			status: http.StatusOK,
			resp:   `{"items":[]}`,
			call: func(adm *AdminClient) (any, error) {
				return adm.ListSharingRecipientGrants(ctx, "acme", SharingListOptions{PageToken: "p"})
			},
			method: http.MethodGet,
			path:   "/_delta-sharing/v1/provider/recipients/acme/share-permissions",
			query:  "pageToken=p",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, rec := sharingTestServer(t, tc.status, tc.resp)
			out, err := tc.call(testAdminClient(t, srv))
			if err != nil {
				t.Fatal(err)
			}
			if rec.method != tc.method || rec.path != tc.path || rec.query != tc.query {
				t.Fatalf("request %s %s?%s, want %s %s?%s", rec.method, rec.path, rec.query, tc.method, tc.path, tc.query)
			}
			if rec.body != tc.body {
				t.Fatalf("body %s, want %s", rec.body, tc.body)
			}
			if !strings.HasPrefix(rec.auth, "AWS4-HMAC-SHA256 ") {
				t.Fatalf("request not signed: %q", rec.auth)
			}
			if tc.checkResp != nil {
				tc.checkResp(t, out)
			}
		})
	}
}

func TestSharingProviderErrors(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantCode string
		wantMsg  string
	}{
		{"provider error", http.StatusNotFound, `{"errorCode":"RESOURCE_DOES_NOT_EXIST","message":"share not found"}`, "RESOURCE_DOES_NOT_EXIST", "share not found"},
		{"admin shaped error", http.StatusForbidden, `{"Code":"AccessDenied","Message":"Access Denied."}`, "AccessDenied", "Access Denied."},
		{"empty json", http.StatusConflict, `{}`, "409 Conflict", "409 Conflict"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := sharingTestServer(t, tc.status, tc.body)
			_, err := testAdminClient(t, srv).GetSharingRecipient(context.Background(), "acme")
			var resp ErrorResponse
			if !errors.As(err, &resp) {
				t.Fatalf("got %T %v, want ErrorResponse", err, err)
			}
			if resp.Code != tc.wantCode || resp.Message != tc.wantMsg || resp.StatusCode != tc.status {
				t.Fatalf("got %+v, want code %q message %q status %d", resp, tc.wantCode, tc.wantMsg, tc.status)
			}
		})
	}
}

func testSharingClient(t *testing.T, srv *httptest.Server) *SharingClient {
	t.Helper()
	c, err := NewSharingClient(DeltaSharingProfile{
		ShareCredentialsVersion: 1,
		Endpoint:                srv.URL + "/_delta-sharing/v1/",
		BearerToken:             "dapi-token",
	}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSharingClientRequests(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name   string
		resp   string
		call   func(*SharingClient) (any, error)
		method string
		path   string
		query  string
		check  func(t *testing.T, out any)
	}{
		{
			name:   "list shares",
			resp:   `{"items":[{"name":"sales","id":"s1"}],"nextPageToken":"n"}`,
			call:   func(c *SharingClient) (any, error) { return c.ListShares(ctx, SharingListOptions{MaxResults: 5}) },
			method: http.MethodGet,
			path:   "/_delta-sharing/v1/shares",
			query:  "maxResults=5",
			check: func(t *testing.T, out any) {
				if p := out.(*SharedPage[SharedShare]); len(p.Items) != 1 || p.NextPageToken != "n" {
					t.Fatalf("got %+v", p)
				}
			},
		},
		{
			name:   "list schemas escapes the share",
			resp:   `{"items":[]}`,
			call:   func(c *SharingClient) (any, error) { return c.ListSchemas(ctx, "a b", SharingListOptions{}) },
			method: http.MethodGet,
			path:   "/_delta-sharing/v1/shares/a%20b/schemas",
		},
		{
			name:   "list all tables",
			resp:   `{"items":[{"name":"t","schema":"s","share":"sales"}]}`,
			call:   func(c *SharingClient) (any, error) { return c.ListAllTables(ctx, "sales", SharingListOptions{}) },
			method: http.MethodGet,
			path:   "/_delta-sharing/v1/shares/sales/all-tables",
		},
		{
			name:   "list tables",
			resp:   `{"items":[]}`,
			call:   func(c *SharingClient) (any, error) { return c.ListTables(ctx, "sales", "s", SharingListOptions{}) },
			method: http.MethodGet,
			path:   "/_delta-sharing/v1/shares/sales/schemas/s/tables",
		},
		{
			name: "list all volumes",
			resp: `{"items":[{"name":"docs","schema":"files","share":"sales","storageLocation":"s3://bkt/docs"}]}`,
			call: func(c *SharingClient) (any, error) {
				return c.ListAllVolumes(ctx, "sales", SharingListOptions{PageToken: "p"})
			},
			method: http.MethodGet,
			path:   "/_delta-sharing/v1/shares/sales/all-volumes",
			query:  "pageToken=p",
			check: func(t *testing.T, out any) {
				p := out.(*SharedPage[SharedVolume])
				if len(p.Items) != 1 || p.Items[0].StorageLocation != "s3://bkt/docs" {
					t.Fatalf("got %+v", p)
				}
			},
		},
		{
			name:   "list volumes in a schema",
			resp:   `{"items":[]}`,
			call:   func(c *SharingClient) (any, error) { return c.ListVolumes(ctx, "sales", "files", SharingListOptions{}) },
			method: http.MethodGet,
			path:   "/_delta-sharing/v1/shares/sales/schemas/files/volumes",
		},
		{
			name:   "get volume",
			resp:   `{"name":"docs","schema":"files","share":"sales","storageLocation":"s3://bkt/docs"}`,
			call:   func(c *SharingClient) (any, error) { return c.GetVolume(ctx, "sales", "files", "docs") },
			method: http.MethodGet,
			path:   "/_delta-sharing/v1/shares/sales/schemas/files/volumes/docs",
		},
		{
			name: "volume credentials",
			resp: `{"awsTempCredentials":{"accessKeyId":"AK","secretAccessKey":"SK","sessionToken":"ST",` +
				`"endpoint":"https://aistor:9000","region":"us-east-1","pathStyleAccess":true},"expirationTime":1795000000000}`,
			call:   func(c *SharingClient) (any, error) { return c.GetVolumeCredentials(ctx, "sales", "files", "docs") },
			method: http.MethodPost,
			path:   "/_delta-sharing/v1/shares/sales/schemas/files/volumes/docs/temporary-volume-credentials",
			check: func(t *testing.T, out any) {
				c := out.(*SharingTemporaryCredentials)
				aws := c.AwsTempCredentials
				if aws == nil || aws.AccessKeyID != "AK" || aws.Endpoint != "https://aistor:9000" || !aws.PathStyleAccess {
					t.Fatalf("got %+v", aws)
				}
				if !c.Expiration().Equal(time.UnixMilli(1795000000000)) {
					t.Fatalf("expiration %v", c.Expiration())
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, rec := sharingTestServer(t, http.StatusOK, tc.resp)
			out, err := tc.call(testSharingClient(t, srv))
			if err != nil {
				t.Fatal(err)
			}
			if rec.method != tc.method || rec.path != tc.path || rec.query != tc.query {
				t.Fatalf("request %s %s?%s, want %s %s?%s", rec.method, rec.path, rec.query, tc.method, tc.path, tc.query)
			}
			if rec.auth != "Bearer dapi-token" {
				t.Fatalf("authorization %q", rec.auth)
			}
			if tc.check != nil {
				tc.check(t, out)
			}
		})
	}
}

func TestSharingClientVolumeSharingOff(t *testing.T) {
	srv, _ := sharingTestServer(t, http.StatusBadRequest,
		`{"errorCode":"INVALID_PARAMETER_VALUE","message":"volume sharing is not enabled on this server"}`)
	_, err := testSharingClient(t, srv).ListAllVolumes(context.Background(), "sales", SharingListOptions{})
	resp := ToErrorResponse(err)
	if resp.Code != "INVALID_PARAMETER_VALUE" || resp.StatusCode != http.StatusBadRequest ||
		!strings.Contains(resp.Message, "not enabled") {
		t.Fatalf("got %+v", resp)
	}
}

func TestSharingClientOAuth(t *testing.T) {
	var tokenCalls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/_delta-sharing/v1/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		tokenCalls.Add(1)
		id, secret, ok := r.BasicAuth()
		if !ok || id != "tok-id" || secret != "tok-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, `{"error":"invalid_client","error_description":"bad client"}`)
			return
		}
		if err := r.ParseForm(); err != nil || r.PostForm.Get("grant_type") != "client_credentials" {
			t.Errorf("form %v %v", r.PostForm, err)
		}
		json.NewEncoder(w).Encode(map[string]any{"access_token": "jwt", "expires_in": 3600, "token_type": "Bearer"})
	})
	mux.HandleFunc("/_delta-sharing/v1/shares", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer jwt" {
			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, `{"errorCode":"UNAUTHENTICATED","message":"no"}`)
			return
		}
		io.WriteString(w, `{"items":[]}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	profile := DeltaSharingProfile{
		ShareCredentialsVersion: 2,
		Type:                    "oauth_client_credentials",
		Endpoint:                srv.URL + "/_delta-sharing/v1",
		TokenEndpoint:           srv.URL + "/_delta-sharing/v1/oauth/token",
		ClientID:                "tok-id",
		ClientSecret:            "tok-secret",
	}
	c, err := NewSharingClient(profile, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := c.ListShares(context.Background(), SharingListOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	if n := tokenCalls.Load(); n != 1 {
		t.Fatalf("token fetched %d times, want once", n)
	}

	profile.ClientSecret = "wrong"
	c, err = NewSharingClient(profile, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.ListShares(context.Background(), SharingListOptions{})
	if resp := ToErrorResponse(err); resp.Code != "invalid_client" || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("got %+v", resp)
	}
}

func TestNewSharingClientRejectsIncompleteProfiles(t *testing.T) {
	tests := map[string]DeltaSharingProfile{
		"no endpoint":     {BearerToken: "t"},
		"bad scheme":      {Endpoint: "ftp://host/x", BearerToken: "t"},
		"no bearer token": {Endpoint: "https://host/_delta-sharing/v1"},
		"oauth without secret": {
			Endpoint: "https://host/_delta-sharing/v1", Type: "oauth_client_credentials",
			TokenEndpoint: "https://host/t", ClientID: "id",
		},
	}
	for name, profile := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := NewSharingClient(profile, nil); err == nil {
				t.Fatal("got nil error")
			}
		})
	}
}

func TestActivateSharingRecipient(t *testing.T) {
	var used atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/_delta-sharing/v1/activations/code1" || used.Swap(true) {
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, `{"errorCode":"RESOURCE_DOES_NOT_EXIST","message":"activation link not found or already used"}`)
			return
		}
		io.WriteString(w, `{"shareCredentialsVersion":1,"endpoint":"https://aistor/_delta-sharing/v1","bearerToken":"dapi","expirationTime":"2027-01-01T00:00:00Z"}`)
	}))
	defer srv.Close()

	link := srv.URL + "/_delta-sharing/v1/activations/code1"
	profile, err := ActivateSharingRecipient(context.Background(), srv.Client(), link)
	if err != nil {
		t.Fatal(err)
	}
	if profile.BearerToken != "dapi" || profile.ExpirationTime != "2027-01-01T00:00:00Z" {
		t.Fatalf("got %+v", profile)
	}
	_, err = ActivateSharingRecipient(context.Background(), srv.Client(), link)
	if resp := ToErrorResponse(err); resp.Code != "RESOURCE_DOES_NOT_EXIST" || resp.StatusCode != http.StatusNotFound {
		t.Fatalf("second activation: got %+v", resp)
	}
}

func TestSharingClientTokenWaitHonorsContext(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/oauth/token") {
			started <- struct{}{}
			<-release
			json.NewEncoder(w).Encode(map[string]any{"access_token": "jwt", "expires_in": 3600})
			return
		}
		io.WriteString(w, `{"items":[]}`)
	}))
	defer srv.Close()
	defer close(release)

	c, err := NewSharingClient(DeltaSharingProfile{
		Type:          "oauth_client_credentials",
		Endpoint:      srv.URL + "/_delta-sharing/v1",
		TokenEndpoint: srv.URL + "/_delta-sharing/v1/oauth/token",
		ClientID:      "id",
		ClientSecret:  "secret",
	}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	go c.ListShares(context.Background(), SharingListOptions{})
	<-started

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := c.ListShares(ctx, SharingListOptions{})
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("got %v, want context.DeadlineExceeded", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("caller waiting behind a token refresh ignored its context")
	}
}

func TestSharingClientTokenExpiryCountsFromRequest(t *testing.T) {
	var tokenCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/oauth/token") {
			tokenCalls.Add(1)
			time.Sleep(1500 * time.Millisecond)
			json.NewEncoder(w).Encode(map[string]any{"access_token": "jwt", "expires_in": 61})
			return
		}
		io.WriteString(w, `{"items":[]}`)
	}))
	defer srv.Close()

	c, err := NewSharingClient(DeltaSharingProfile{
		Type:          "oauth_client_credentials",
		Endpoint:      srv.URL + "/_delta-sharing/v1",
		TokenEndpoint: srv.URL + "/_delta-sharing/v1/oauth/token",
		ClientID:      "id",
		ClientSecret:  "secret",
	}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := c.ListShares(context.Background(), SharingListOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	// The token lives 61s from when it was requested, and the response took
	// 1.5s, so less than the one-minute margin is left: the second call must
	// fetch a new token.
	if n := tokenCalls.Load(); n != 2 {
		t.Fatalf("token fetched %d times, want 2", n)
	}
}

func TestSharingTraceOmitsActivationLink(t *testing.T) {
	const link = "https://aistor/_delta-sharing/v1/activations/secret-code"
	srv, _ := sharingTestServer(t, http.StatusCreated, `{"id":"r1","name":"acme","authenticationType":"TOKEN","activationUrl":"`+link+`"}`)
	adm := testAdminClient(t, srv)
	var trace strings.Builder
	adm.TraceOn(&trace)
	if _, err := adm.CreateSharingRecipient(context.Background(), CreateSharingRecipientRequest{Name: "acme"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(trace.String(), "secret-code") {
		t.Fatalf("trace holds the activation link:\n%s", trace.String())
	}

	srv, _ = sharingTestServer(t, http.StatusNotFound, `{"errorCode":"RESOURCE_DOES_NOT_EXIST","message":"recipient not found"}`)
	adm = testAdminClient(t, srv)
	trace.Reset()
	adm.TraceOn(&trace)
	_, _ = adm.GetSharingRecipient(context.Background(), "acme")
	if !strings.Contains(trace.String(), "recipient not found") {
		t.Fatalf("trace of an error lacks its body:\n%s", trace.String())
	}
}
