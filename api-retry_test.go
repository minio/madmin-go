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
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
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

// noJitterSource makes the client's random numbers zero, so every backoff
// waits its full length and a test can tell how many waits there were.
type noJitterSource struct{}

func (noJitterSource) Int63() int64 { return 0 }
func (noJitterSource) Seed(int64)   {}

// setMaxRetry sets MaxRetry for the rest of the test.
func setMaxRetry(t *testing.T, n int) {
	t.Helper()
	saved := MaxRetry
	MaxRetry = n
	t.Cleanup(func() { MaxRetry = saved })
}

// countingServer starts a server that answers every request with status and
// body, and counts the requests it receives.
func countingServer(t *testing.T, status int, body string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server, &attempts
}

// The retry timer yields once per attempt and waits only between attempts:
// maxRetry attempts take the first maxRetry-1 backoffs, and no wait follows
// the last one.
func TestRetryTimerWaitsOnlyBetweenAttempts(t *testing.T) {
	const unit = 20 * time.Millisecond
	adm := AdminClient{random: rand.New(noJitterSource{})}
	for maxRetry := 1; maxRetry <= 4; maxRetry++ {
		// The waits are unit, 2*unit, 4*unit, ..., so maxRetry-1 of them add
		// up to (2^(maxRetry-1) - 1) * unit, and the wait a timer would add
		// after the last attempt is 2^(maxRetry-1) * unit.
		between := time.Duration(1<<(maxRetry-1)-1) * unit
		afterLast := time.Duration(1<<(maxRetry-1)) * unit

		start := time.Now()
		attempts := 0
		for range adm.newRetryTimer(context.Background(), maxRetry, unit, time.Hour, NoJitter) {
			attempts++
		}
		elapsed := time.Since(start)

		if attempts != maxRetry {
			t.Errorf("maxRetry %d: %d attempts, want %d", maxRetry, attempts, maxRetry)
		}
		if elapsed < between {
			t.Errorf("maxRetry %d: took %v, want at least the %v of backoff between attempts", maxRetry, elapsed, between)
		}
		if elapsed >= between+afterLast {
			t.Errorf("maxRetry %d: took %v, want less than %v: it waited after the last attempt", maxRetry, elapsed, between+afterLast)
		}
	}
}

// An admin call whose every answer is retryable is sent MaxRetry times, with
// a backoff between attempts and none after the last, so the caller gets the
// last answer as soon as it arrives.
func TestExecuteMethodWaitsOnlyBetweenAttempts(t *testing.T) {
	setMaxRetry(t, 2)
	server, attempts := countingServer(t, http.StatusServiceUnavailable, `{"Code":"XMinioServerNotInitialized"}`)
	client := newFilesExportsTestClient(t, server.URL)
	client.random = rand.New(noJitterSource{})

	start := time.Now()
	resp, err := client.executeMethod(context.Background(), http.MethodGet, requestData{relPath: "/v4/info"})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("executeMethod: %v", err)
	}
	defer closeResponse(resp)

	if n := attempts.Load(); n != 2 {
		t.Errorf("the server received %d requests, want 2", n)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
	// One full backoff of DefaultRetryUnit separates the two attempts. A wait
	// after the second would add another 2*DefaultRetryUnit.
	if elapsed < DefaultRetryUnit {
		t.Errorf("took %v, want at least the %v backoff between the attempts", elapsed, DefaultRetryUnit)
	}
	if elapsed >= 2*DefaultRetryUnit {
		t.Errorf("took %v, want less than %v: it waited after the last attempt", elapsed, 2*DefaultRetryUnit)
	}
}

// Every success status ends the call after one attempt with its body intact,
// even when the body happens to carry a retryable code.
func TestExecuteMethodSuccessStatusIsFinal(t *testing.T) {
	setMaxRetry(t, 2)
	const body = `{"Code":"SlowDown"}`
	for _, status := range []int{
		http.StatusOK,
		http.StatusCreated,
		http.StatusAccepted,
		http.StatusNoContent,
		http.StatusPartialContent,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server, attempts := countingServer(t, status, body)

			start := time.Now()
			resp, err := newFilesExportsTestClient(t, server.URL).executeMethod(context.Background(), http.MethodGet,
				requestData{relPath: "/v4/info"})
			elapsed := time.Since(start)
			if err != nil {
				t.Fatalf("executeMethod: %v", err)
			}
			defer closeResponse(resp)

			if n := attempts.Load(); n != 1 {
				t.Errorf("the server received %d requests, want 1", n)
			}
			if elapsed >= DefaultRetryUnit/2 {
				t.Errorf("took %v, want no backoff", elapsed)
			}
			if resp.StatusCode != status {
				t.Errorf("status = %d, want %d", resp.StatusCode, status)
			}
			got, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			want := body
			if status == http.StatusNoContent {
				want = ""
			}
			if string(got) != want {
				t.Errorf("body = %q, want %q", got, want)
			}
		})
	}
}
