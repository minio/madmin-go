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
	"reflect"
	"testing"
)

// setAdminAPIPrefixes sets the current and previous admin API version prefixes
// for the rest of the test, so the test does not depend on MADMIN_API_VERSION.
func setAdminAPIPrefixes(t *testing.T, current, previous string) {
	t.Helper()
	savedCurrent, savedPrevious := adminAPIPrefix, adminAPIOldPrefix
	adminAPIPrefix, adminAPIOldPrefix = current, previous
	t.Cleanup(func() { adminAPIPrefix, adminAPIOldPrefix = savedCurrent, savedPrevious })
}

// upgradeRequiredServer starts a server that answers every request with a 426
// and records the path of each request it receives.
func upgradeRequiredServer(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.WriteHeader(http.StatusUpgradeRequired)
		_, _ = w.Write([]byte(`{"Code":"XMinioAdminVersionMismatch","Message":"upgrade"}`))
	}))
	t.Cleanup(server.Close)
	return server, &paths
}

// sendUpgradeRequired sends a GET for relPath to server and returns the
// response, which the test closes.
func sendUpgradeRequired(t *testing.T, server *httptest.Server, relPath string) *http.Response {
	t.Helper()
	resp, err := newFilesExportsTestClient(t, server.URL).executeMethod(context.Background(), http.MethodGet,
		requestData{relPath: relPath})
	if err != nil {
		t.Fatalf("executeMethod: %v", err)
	}
	t.Cleanup(func() { closeResponse(resp) })
	if resp.StatusCode != http.StatusUpgradeRequired {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUpgradeRequired)
	}
	return resp
}

// A 426 on a path under the current admin API version is retried once on the
// previous version, and a 426 from the previous version is returned.
func TestUpgradeRequiredRetriesOnceOnThePreviousVersion(t *testing.T) {
	setAdminAPIPrefixes(t, "/v4", "/v3")
	server, paths := upgradeRequiredServer(t)

	sendUpgradeRequired(t, server, "/v4/info")

	if want := []string{"/minio/admin/v4/info", "/minio/admin/v3/info"}; !reflect.DeepEqual(*paths, want) {
		t.Errorf("the server received %q, want %q", *paths, want)
	}
}

// Only the leading version is an admin API version. A later segment that holds
// the same text is kept: the retry of a heal of bucket v4data still names
// v4data, and a Files path with an export named v4home has no leading version,
// so it is sent unchanged, once.
func TestUpgradeRequiredKeepsAVersionLaterInThePath(t *testing.T) {
	setAdminAPIPrefixes(t, "/v4", "/v3")

	for _, tc := range []struct {
		name    string
		relPath string
		want    []string
	}{
		{
			name:    "bucket_under_the_current_version",
			relPath: "/v4/heal/v4data",
			want:    []string{"/minio/admin/v4/heal/v4data", "/minio/admin/v3/heal/v4data"},
		},
		{
			name:    "export_under_no_version",
			relPath: filesAPIPrefix + "/exports/v4home",
			want:    []string{"/minio/admin/files/v1/exports/v4home"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server, paths := upgradeRequiredServer(t)

			sendUpgradeRequired(t, server, tc.relPath)

			if !reflect.DeepEqual(*paths, tc.want) {
				t.Errorf("the server received %q, want %q", *paths, tc.want)
			}
		})
	}
}

// When the current version is already the previous one, as it is with
// MADMIN_API_VERSION=v3, there is nothing to fall back to, so the 426 is
// returned after one request.
func TestUpgradeRequiredWithNoOlderVersionIsNotRetried(t *testing.T) {
	setAdminAPIPrefixes(t, "/v3", "/v3")
	server, paths := upgradeRequiredServer(t)

	sendUpgradeRequired(t, server, "/v3/info")

	if want := []string{"/minio/admin/v3/info"}; !reflect.DeepEqual(*paths, want) {
		t.Errorf("the server received %q, want %q", *paths, want)
	}
}

// A 426 on the last attempt is returned with a body the caller can still
// decode, so the caller sees the server's error code.
func TestUpgradeRequiredOnTheLastAttemptKeepsTheErrorBody(t *testing.T) {
	setAdminAPIPrefixes(t, "/v4", "/v3")
	savedMaxRetry := MaxRetry
	MaxRetry = 1
	t.Cleanup(func() { MaxRetry = savedMaxRetry })
	server, paths := upgradeRequiredServer(t)

	resp := sendUpgradeRequired(t, server, "/v4/info")

	if want := []string{"/minio/admin/v4/info"}; !reflect.DeepEqual(*paths, want) {
		t.Errorf("the server received %q, want %q", *paths, want)
	}
	if code := ToErrorResponse(httpRespToErrorResponse(resp)).Code; code != "XMinioAdminVersionMismatch" {
		t.Errorf("code = %q, want the server's XMinioAdminVersionMismatch", code)
	}
}
