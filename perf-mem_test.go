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
	"testing"
	"time"
)

func newMemPerfTestClient(t *testing.T, serverURL string) *AdminClient {
	t.Helper()
	client, err := New(mustParseHost(t, serverURL), "ak", "sk", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client
}

// TestMemPerfRequest verifies the client issues POST against the speedtest/mem
// admin path.
func TestMemPerfRequest(t *testing.T) {
	var capturedMethod, capturedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	if _, err := newMemPerfTestClient(t, server.URL).MemPerf(context.Background(), MemPerfOpts{}); err != nil {
		t.Fatalf("MemPerf: %v", err)
	}

	if capturedMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", capturedMethod)
	}
	if !strings.HasPrefix(capturedPath, "/minio/admin/") {
		t.Errorf("path missing /minio/admin/ prefix: %s", capturedPath)
	}
	if !strings.HasSuffix(capturedPath, "/speedtest/mem") {
		t.Errorf("path missing /speedtest/mem suffix: %s", capturedPath)
	}
}

// TestMemPerfQueryValues covers option serialization, and that zero options are
// omitted entirely so the server applies its own defaults.
func TestMemPerfQueryValues(t *testing.T) {
	tests := []struct {
		name string
		opts MemPerfOpts
		want map[string]string
		omit []string
	}{
		{
			name: "all options set",
			opts: MemPerfOpts{Duration: 12 * time.Second, Threads: 64, BufferSize: 1 << 26},
			want: map[string]string{"duration": "12s", "threads": "64", "buffersize": "67108864"},
		},
		{
			name: "zero options omitted",
			opts: MemPerfOpts{},
			omit: []string{"duration", "threads", "buffersize"},
		},
		{
			name: "partial options",
			opts: MemPerfOpts{Threads: 8},
			want: map[string]string{"threads": "8"},
			omit: []string{"duration", "buffersize"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var q url.Values
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				q = r.URL.Query()
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{}`))
			}))
			defer server.Close()

			if _, err := newMemPerfTestClient(t, server.URL).MemPerf(context.Background(), tc.opts); err != nil {
				t.Fatalf("MemPerf: %v", err)
			}
			for k, want := range tc.want {
				if got := q.Get(k); got != want {
					t.Errorf("%s = %q, want %q", k, got, want)
				}
			}
			for _, k := range tc.omit {
				if _, ok := q[k]; ok {
					t.Errorf("%s should be omitted when zero, got %q", k, q.Get(k))
				}
			}
		})
	}
}

// TestMemPerfNegativeOpts checks that a negative option is rejected before any
// request is issued, so a caller mistake cannot start a saturating run.
func TestMemPerfNegativeOpts(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts MemPerfOpts
	}{
		{"negative duration", MemPerfOpts{Duration: -time.Second}},
		{"negative threads", MemPerfOpts{Threads: -1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{}`))
			}))
			defer server.Close()

			if _, err := newMemPerfTestClient(t, server.URL).MemPerf(context.Background(), tc.opts); err == nil {
				t.Fatal("expected an error for a negative option")
			}
			if called {
				t.Error("no request should be issued when validation fails")
			}
		})
	}
}

// TestMemPerfDecode covers decoding of a populated multi-node response.
func TestMemPerfDecode(t *testing.T) {
	body := `{"nodeResults":[
      {"endpoint":"node-1:9000","bandwidthBps":335000000000,"perNumaBps":[168000000000,167000000000],
       "threads":256,"bufferBytes":67108864,"duration":5000000000,
       "theoreticalBps":460800000000,"topologySource":"dmi","channels":12,
       "configuredMTs":4800,"maxSpeedMTs":5600,"numaNodes":2,
       "dimms":[{"locator":"DIMM_A1","sizeBytes":68719476736,"maxSpeedMTs":5600,"configuredMTs":4800}]},
      {"endpoint":"node-2:9000","bandwidthBps":120000000000,"topologySource":"edac","channels":8},
      {"endpoint":"node-3:9000","error":"perf unavailable"}]}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
	defer server.Close()

	res, err := newMemPerfTestClient(t, server.URL).MemPerf(context.Background(), MemPerfOpts{})
	if err != nil {
		t.Fatalf("MemPerf: %v", err)
	}
	if len(res.NodeResults) != 3 {
		t.Fatalf("expected 3 node results, got %d", len(res.NodeResults))
	}

	n := res.NodeResults[0]
	if n.BandwidthBps != 335000000000 {
		t.Errorf("bandwidth = %d", n.BandwidthBps)
	}
	if n.TopologySource != MemPerfTopologyDMI {
		t.Errorf("topologySource = %q", n.TopologySource)
	}
	if n.Channels != 12 || n.NUMANodes != 2 {
		t.Errorf("channels = %d, numaNodes = %d", n.Channels, n.NUMANodes)
	}
	if n.Duration != 5*time.Second {
		t.Errorf("duration = %v, want 5s", n.Duration)
	}
	if len(n.PerNUMABps) != 2 {
		t.Errorf("perNumaBps len = %d", len(n.PerNUMABps))
	}
	if len(n.DIMMs) != 1 || n.DIMMs[0].Locator != "DIMM_A1" {
		t.Errorf("dimms = %+v", n.DIMMs)
	}
	if !n.Downclocked() {
		t.Error("4800 configured against 5600 rated should report downclocked")
	}

	if res.NodeResults[2].Error != "perf unavailable" {
		t.Errorf("per-node error not decoded: %q", res.NodeResults[2].Error)
	}
}

// TestMemPerfHTTPError checks a non-200 response surfaces as an error.
func TestMemPerfHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	if _, err := newMemPerfTestClient(t, server.URL).MemPerf(context.Background(), MemPerfOpts{}); err == nil {
		t.Fatal("expected an error for a 500 response")
	}
}

// TestMemPerfMalformedJSON checks a truncated body surfaces as an error rather
// than an empty result.
func TestMemPerfMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"nodeResults":[{"endpoint":`))
	}))
	defer server.Close()

	if _, err := newMemPerfTestClient(t, server.URL).MemPerf(context.Background(), MemPerfOpts{}); err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
}

// TestMemPerfContextCancelled checks a cancelled context aborts the call.
func TestMemPerfContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := newMemPerfTestClient(t, server.URL).MemPerf(ctx, MemPerfOpts{}); err == nil {
		t.Fatal("expected an error for a cancelled context")
	}
}

// TestMemPerfEfficiency pins the contract that a ratio is only reported when a
// topology source actually backs the theoretical figure.
func TestMemPerfEfficiency(t *testing.T) {
	tests := []struct {
		name string
		res  MemPerfNodeResult
		want float64
	}{
		{
			name: "dmi topology",
			res:  MemPerfNodeResult{BandwidthBps: 200, TheoreticalBps: 400, TopologySource: MemPerfTopologyDMI},
			want: 0.5,
		},
		{
			name: "edac topology without a theoretical figure",
			res:  MemPerfNodeResult{BandwidthBps: 200, TopologySource: MemPerfTopologyEDAC},
			want: 0,
		},
		{
			name: "unavailable topology never reports a ratio",
			res:  MemPerfNodeResult{BandwidthBps: 200, TheoreticalBps: 400, TopologySource: MemPerfTopologyUnavailable},
			want: 0,
		},
		{
			name: "unset topology never reports a ratio",
			res:  MemPerfNodeResult{BandwidthBps: 200, TheoreticalBps: 400},
			want: 0,
		},
		{
			name: "zero theoretical",
			res:  MemPerfNodeResult{BandwidthBps: 200, TopologySource: MemPerfTopologyDMI},
			want: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.res.Efficiency(); got != tc.want {
				t.Errorf("Efficiency() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestMemPerfDownclocked covers the rated-versus-configured comparison, which
// stays meaningful even on a node showing good efficiency.
func TestMemPerfDownclocked(t *testing.T) {
	tests := []struct {
		name string
		res  MemPerfNodeResult
		want bool
	}{
		{"configured below rated", MemPerfNodeResult{ConfiguredMTs: 4800, MaxSpeedMTs: 5600}, true},
		{"running at rated", MemPerfNodeResult{ConfiguredMTs: 5600, MaxSpeedMTs: 5600}, false},
		{"rated unknown", MemPerfNodeResult{ConfiguredMTs: 4800}, false},
		{"configured unknown", MemPerfNodeResult{MaxSpeedMTs: 5600}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.res.Downclocked(); got != tc.want {
				t.Errorf("Downclocked() = %v, want %v", got, tc.want)
			}
		})
	}
}
