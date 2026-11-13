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
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package madmin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// updateFromFileRequest is what the test server received.
type updateFromFileRequest struct {
	method, path, contentType, payloadHash, authorization string
	query                                                 map[string]string
	contentLength                                         int64
	body                                                  []byte
}

// updateFromFileServer answers every request with status and body, and
// records the requests.
func updateFromFileServer(t *testing.T, status int, body string) (*httptest.Server, *[]updateFromFileRequest, *atomic.Int32) {
	t.Helper()
	var (
		reqs     []updateFromFileRequest
		attempts atomic.Int32
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		b, _ := io.ReadAll(r.Body)
		q := map[string]string{}
		for k := range r.URL.Query() {
			q[k] = r.URL.Query().Get(k)
		}
		reqs = append(reqs, updateFromFileRequest{
			method: r.Method, path: r.URL.Path, contentType: r.Header.Get("Content-Type"),
			payloadHash: r.Header.Get("X-Amz-Content-Sha256"), authorization: r.Header.Get("Authorization"),
			query: q, contentLength: r.ContentLength, body: b,
		})
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server, &reqs, &attempts
}

// TestServerUpdateFromFile checks the request ServerUpdateFromFile sends:
// the file as the body, signed when the reader can be read twice, and the
// options as for ServerUpdate.
func TestServerUpdateFromFile(t *testing.T) {
	file := []byte("an update file")
	sum := sha256.Sum256(file)
	want := ServerUpdateStatus{DryRun: true, Results: []ServerPeerUpdateStatus{{Host: "node1:9000", CurrentVersion: "11.0.0-stable", UpdatedVersion: "11.0.0-stable.hotfix-acme"}}}
	reply, _ := json.Marshal(want)

	t.Run("request", func(t *testing.T) {
		server, reqs, _ := updateFromFileServer(t, http.StatusOK, string(reply))
		us, err := newFilesExportsTestClient(t, server.URL).ServerUpdateFromFile(context.Background(),
			ServerUpdateFromFileOpts{DryRun: true, Rolling: true, RollingGracefulWait: 2 * time.Minute, ByNode: true},
			bytes.NewReader(file), int64(len(file)))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(us, want) {
			t.Errorf("ServerUpdateFromFile() = %+v, want %+v", us, want)
		}
		if len(*reqs) != 1 {
			t.Fatalf("%d requests, want 1", len(*reqs))
		}
		got := (*reqs)[0]
		if got.method != http.MethodPost || got.path != "/minio/admin"+adminAPIPrefix+"/update-from-file" {
			t.Errorf("request %s %s", got.method, got.path)
		}
		for k, v := range map[string]string{"dry-run": "true", "wait": "120000000000", "by-node": "true"} {
			if got.query[k] != v {
				t.Errorf("query %s = %q, want %q", k, got.query[k], v)
			}
		}
		if !bytes.Equal(got.body, file) || got.contentLength != int64(len(file)) || got.contentType != "application/gzip" {
			t.Errorf("body %q, length %d, type %q", got.body, got.contentLength, got.contentType)
		}
		// The signature covers the file: its SHA-256 is a signed header.
		if got.payloadHash != hex.EncodeToString(sum[:]) {
			t.Errorf("X-Amz-Content-Sha256 = %q, want the file's SHA-256 %x", got.payloadHash, sum)
		}
		_, signed, _ := strings.Cut(got.authorization, "SignedHeaders=")
		signed, _, _ = strings.Cut(signed, ",")
		if !slices.Contains(strings.Split(signed, ";"), "x-amz-content-sha256") {
			t.Errorf("x-amz-content-sha256 not signed: %q", got.authorization)
		}
	})

	t.Run("reads size bytes from the current offset", func(t *testing.T) {
		server, reqs, _ := updateFromFileServer(t, http.StatusOK, string(reply))
		r := bytes.NewReader(append([]byte("skip"), append(bytes.Clone(file), "rest"...)...))
		r.Seek(4, io.SeekStart)
		if _, err := newFilesExportsTestClient(t, server.URL).ServerUpdateFromFile(context.Background(),
			ServerUpdateFromFileOpts{}, r, int64(len(file))); err != nil {
			t.Fatal(err)
		}
		if got := (*reqs)[0]; !bytes.Equal(got.body, file) || got.payloadHash != hex.EncodeToString(sum[:]) {
			t.Errorf("body %q, payload hash %q", got.body, got.payloadHash)
		}
	})

	t.Run("not retried", func(t *testing.T) {
		server, _, attempts := updateFromFileServer(t, http.StatusServiceUnavailable,
			`{"Code":"XMinioServerNotInitialized","Message":"not ready"}`)
		_, err := newFilesExportsTestClient(t, server.URL).ServerUpdateFromFile(context.Background(),
			ServerUpdateFromFileOpts{}, bytes.NewReader(file), int64(len(file)))
		if err == nil || !strings.Contains(err.Error(), "not ready") {
			t.Errorf("error = %v, want the server's", err)
		}
		if n := attempts.Load(); n != 1 {
			t.Errorf("%d attempts, want 1", n)
		}
	})

	t.Run("not sent again on the previous API version", func(t *testing.T) {
		// A server that asks for an upgrade gets the request once: the file
		// has been sent, and an older server has no update-from-file API.
		setAdminAPIPrefixes(t, "/v4", "/v3")
		server, reqs, _ := updateFromFileServer(t, http.StatusUpgradeRequired,
			`{"Code":"XMinioAdminVersionMismatch","Message":"upgrade"}`)
		_, err := newFilesExportsTestClient(t, server.URL).ServerUpdateFromFile(context.Background(),
			ServerUpdateFromFileOpts{}, bytes.NewReader(file), int64(len(file)))
		if ToErrorResponse(err).Code != "XMinioAdminVersionMismatch" {
			t.Errorf("error = %v, want XMinioAdminVersionMismatch", err)
		}
		if len(*reqs) != 1 || (*reqs)[0].path != "/minio/admin/v4/update-from-file" {
			paths := make([]string, 0, len(*reqs))
			for _, r := range *reqs {
				paths = append(paths, r.path)
			}
			t.Errorf("requests %v, want one to /v4", paths)
		}
	})

	t.Run("server error", func(t *testing.T) {
		server, _, _ := updateFromFileServer(t, http.StatusBadRequest,
			`{"Code":"XMinioAdminUpdateInvalidFile","Message":"the update file holds no binary"}`)
		_, err := newFilesExportsTestClient(t, server.URL).ServerUpdateFromFile(context.Background(),
			ServerUpdateFromFileOpts{}, bytes.NewReader(file), int64(len(file)))
		if ToErrorResponse(err).Code != "XMinioAdminUpdateInvalidFile" {
			t.Errorf("error = %v, want XMinioAdminUpdateInvalidFile", err)
		}
	})

	t.Run("size out of range", func(t *testing.T) {
		client := newFilesExportsTestClient(t, "http://127.0.0.1:1")
		for _, size := range []int64{0, MaxServerUpdateFileSize + 1} {
			if _, err := client.ServerUpdateFromFile(context.Background(), ServerUpdateFromFileOpts{}, bytes.NewReader(file), size); err == nil {
				t.Errorf("size %d accepted", size)
			}
		}
	})
}
