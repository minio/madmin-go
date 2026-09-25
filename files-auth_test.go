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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7/pkg/credentials"
)

// SetFilesAuth sends the material encrypted with the caller's secret key, so
// the server can decrypt it back to the exact assets named, and it decodes the
// JSON reply.
func TestSetFilesAuthRequest(t *testing.T) {
	keytab := []byte{0x05, 0x02, 0x00, 0x2a}
	sent := []FilesAuthMaterial{
		{Kind: FilesAuthKrb5Keytab, Bytes: keytab},
		{Kind: FilesAuthTLSCABundle, Bytes: []byte("-----BEGIN CERTIFICATE-----\n")},
	}
	want := FilesAuthResponse{
		SetDigest: "d1",
		Assets:    []FilesAuthAsset{{Kind: FilesAuthKrb5Keytab, SHA256: "k1", Bytes: 4}},
		Results: []FilesAuthNodeStatus{{
			Node:    "10.0.0.1:9000",
			Outcome: FilesAuthApplied,
			Gateway: &FilesAuthGateway{
				BootID:     "b1",
				Subsystems: []FilesAuthSubsystem{{Name: "krb5", Outcome: "applied", Seq: 2}},
				Assets:     []FilesAuthAsset{{Kind: FilesAuthKrb5Keytab, SHA256: "k1", InForceSHA256: "k1"}},
			},
			Converged: true,
		}},
		Count:       1,
		Total:       2,
		Unreachable: []FilesUnreachableNode{{Node: "10.0.0.2:9000", Error: "down"}},
	}

	var (
		method, path string
		got          FilesAuthSetRequest
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		if bytes.Contains(body, keytab) {
			t.Error("the keytab travels in the clear")
		}
		plain, err := DecryptData("sk", bytes.NewReader(body))
		if err != nil {
			t.Errorf("decrypt body: %v", err)
		}
		if err := json.Unmarshal(plain, &got); err != nil {
			t.Errorf("decode body %q: %v", plain, err)
		}
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(&want); err != nil {
			t.Errorf("encode reply: %v", err)
		}
	}))
	defer server.Close()

	reply, err := newFilesExportsTestClient(t, server.URL).SetFilesAuth(context.Background(), sent)
	if err != nil {
		t.Fatalf("SetFilesAuth: %v", err)
	}
	if method != http.MethodPut {
		t.Errorf("method = %s, want PUT", method)
	}
	if wantPath := "/minio/admin/files/v1/auth"; path != wantPath {
		t.Errorf("path = %s, want %s", path, wantPath)
	}
	if len(got.Assets) != 2 || got.Assets[0].Kind != FilesAuthKrb5Keytab || !bytes.Equal(got.Assets[0].Bytes, keytab) {
		t.Errorf("the server received %+v", got.Assets)
	}
	if len(reply.Results) != 1 || !reply.Results[0].Converged || reply.Results[0].Gateway == nil ||
		reply.Results[0].Gateway.Assets[0].InForceSHA256 != "k1" || reply.Total != 2 {
		t.Errorf("reply = %+v", reply)
	}
}

// The client refuses a set the server would refuse, before anything is sent.
func TestSetFilesAuthRefusesABadSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("a bad set reached the server")
	}))
	defer server.Close()
	client := newFilesExportsTestClient(t, server.URL)

	for name, set := range map[string][]FilesAuthMaterial{
		"no asset":           nil,
		"an unknown kind":    {{Kind: "ssh_host_key", Bytes: []byte("x")}},
		"an empty asset":     {{Kind: FilesAuthKrb5Keytab}},
		"a kind named twice": {{Kind: FilesAuthKrb5Keytab, Bytes: []byte("x")}, {Kind: FilesAuthKrb5Keytab, Bytes: []byte("y")}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := client.SetFilesAuth(context.Background(), set); err == nil {
				t.Fatal("the set was accepted")
			}
		})
	}
}

// FilesAuthInfo is a GET on the same path, and a server error reaches the
// caller as an error rather than as an empty reply.
func TestFilesAuthInfoRequest(t *testing.T) {
	var method, path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"Code":"AccessDenied","Message":"denied"}`))
	}))
	defer server.Close()

	if _, err := newFilesExportsTestClient(t, server.URL).FilesAuthInfo(context.Background()); ToErrorResponse(err).Code != "AccessDenied" {
		t.Fatalf("err = %v, want the server's AccessDenied", err)
	}
	if method != http.MethodGet || path != "/minio/admin/files/v1/auth" {
		t.Errorf("request = %s %s", method, path)
	}
}

// flakyCreds fails its first lookup and succeeds after that.
type flakyCreds struct {
	credentials.Expiry
	calls int
}

func (f *flakyCreds) Retrieve() (credentials.Value, error) { return f.RetrieveWithCredContext(nil) }

func (f *flakyCreds) RetrieveWithCredContext(*credentials.CredContext) (credentials.Value, error) {
	if f.calls++; f.calls == 1 {
		return credentials.Value{}, errors.New("sts down")
	}
	return credentials.Value{AccessKeyID: "ak", SecretAccessKey: "sk", SignerType: credentials.SignatureV4}, nil
}

func (f *flakyCreds) IsExpired() bool { return true }

// SetFilesAuth never sends material when the credential lookup fails, so the
// material is never encrypted under an empty key.
func TestSetFilesAuthCredentialFailure(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewWithOptions(mustParseHost(t, server.URL), &Options{Creds: credentials.New(&flakyCreds{})})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	set := []FilesAuthMaterial{{Kind: FilesAuthKrb5Keytab, Bytes: []byte{0x05, 0x02}}}
	if _, err := client.SetFilesAuth(context.Background(), set); err == nil {
		t.Fatal("a failed credential lookup was not reported")
	}
	if requests != 0 {
		t.Errorf("the server received %d requests", requests)
	}
}

// A 426 on a Files path, which carries no admin API version to downgrade,
// reaches the caller as an error after one request instead of being retried.
func TestFilesAuthUpgradeRequiredIsNotRetried(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusUpgradeRequired)
		_, _ = w.Write([]byte(`{"Code":"XMinioAdminVersionMismatch","Message":"upgrade"}`))
	}))
	defer server.Close()

	_, err := newFilesExportsTestClient(t, server.URL).FilesAuthInfo(context.Background())
	if ToErrorResponse(err).Code != "XMinioAdminVersionMismatch" {
		t.Fatalf("err = %v, want the server's XMinioAdminVersionMismatch", err)
	}
	if requests != 1 {
		t.Errorf("the server received %d requests, want 1", requests)
	}
}

// FilesAuthInfo decodes the JSON reply the server writes.
func TestFilesAuthInfoDecodesReply(t *testing.T) {
	want := FilesAuthResponse{
		SetDigest: "abc",
		Assets:    []FilesAuthAsset{{Kind: FilesAuthKrb5Keytab, SHA256: "abc", Bytes: 4}},
		Results: []FilesAuthNodeStatus{{
			Node:    "node1:9000",
			Outcome: FilesAuthApplied,
			Gateway: &FilesAuthGateway{
				BootID:     "boot",
				Subsystems: []FilesAuthSubsystem{{Name: "krb5", Outcome: FilesAuthSubsystemApplied, Seq: 3}},
			},
			Converged: true,
		}},
		Count: 1,
		Total: 1,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := json.NewEncoder(w).Encode(&want); err != nil {
			t.Errorf("encode reply: %v", err)
		}
	}))
	defer server.Close()

	got, err := newFilesExportsTestClient(t, server.URL).FilesAuthInfo(context.Background())
	if err != nil {
		t.Fatalf("FilesAuthInfo: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("reply = %+v, want %+v", got, want)
	}
}

// rotatingCreds returns a new credential on every lookup: ak1/sk1, ak2/sk2, ...
type rotatingCreds struct {
	credentials.Expiry
	calls int
}

func (r *rotatingCreds) Retrieve() (credentials.Value, error) { return r.RetrieveWithCredContext(nil) }

func (r *rotatingCreds) RetrieveWithCredContext(*credentials.CredContext) (credentials.Value, error) {
	r.calls++
	return credentials.Value{
		AccessKeyID:     fmt.Sprintf("ak%d", r.calls),
		SecretAccessKey: fmt.Sprintf("sk%d", r.calls),
		SignerType:      credentials.SignatureV4,
	}, nil
}

func (r *rotatingCreds) IsExpired() bool { return true }

// SetFilesAuth signs with the credential that encrypted the body, even when the
// provider hands out a new one between the two, so the server can decrypt the
// body with the secret of the signing key.
func TestSetFilesAuthSignsWithTheEncryptingCredential(t *testing.T) {
	var got FilesAuthSetRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		_, rest, _ := strings.Cut(auth, "Credential=ak")
		n, _, _ := strings.Cut(rest, "/")
		body, _ := io.ReadAll(r.Body)
		plain, err := DecryptData("sk"+n, bytes.NewReader(body))
		if err != nil {
			t.Errorf("the body does not decrypt with the signing key's secret: %v (%s)", err, auth)
		} else if err := json.Unmarshal(plain, &got); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client, err := NewWithOptions(mustParseHost(t, server.URL), &Options{Creds: credentials.New(&rotatingCreds{})})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	set := []FilesAuthMaterial{{Kind: FilesAuthKrb5Keytab, Bytes: []byte{0x05, 0x02}}}
	if _, err := client.SetFilesAuth(context.Background(), set); err != nil {
		t.Fatalf("SetFilesAuth: %v", err)
	}
	if len(got.Assets) != 1 || got.Assets[0].Kind != FilesAuthKrb5Keytab {
		t.Errorf("the server received %+v", got.Assets)
	}
}
