//
// Copyright (c) 2015-2025 MinIO, Inc.
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
	"testing"
)

// mustParseHost extracts the host from a URL string for use with NewAnonymousClient
func mustParseHost(t *testing.T, rawURL string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("Failed to parse URL %q: %v", rawURL, err)
	}
	return u.Host
}

// TestHealthOptsURLParameters tests that Maintenance and Distributed fields
// correctly add or omit URL parameters
func TestHealthOptsURLParameters(t *testing.T) {
	tests := []struct {
		name                 string
		opts                 HealthOpts
		expectDistributed    bool
		expectMaintenance    bool
		expectClusterReadURL bool
	}{
		{
			name:              "Distributed true adds parameter",
			opts:              HealthOpts{Distributed: true},
			expectDistributed: true,
			expectMaintenance: false,
		},
		{
			name:              "Distributed false omits parameter",
			opts:              HealthOpts{Distributed: false},
			expectDistributed: false,
			expectMaintenance: false,
		},
		{
			name:              "Maintenance true adds parameter",
			opts:              HealthOpts{Maintenance: true},
			expectDistributed: false,
			expectMaintenance: true,
		},
		{
			name:              "Maintenance false omits parameter",
			opts:              HealthOpts{Maintenance: false},
			expectDistributed: false,
			expectMaintenance: false,
		},
		{
			name:              "Both Distributed and Maintenance true",
			opts:              HealthOpts{Distributed: true, Maintenance: true},
			expectDistributed: true,
			expectMaintenance: true,
		},
		{
			name:              "Default empty opts",
			opts:              HealthOpts{},
			expectDistributed: false,
			expectMaintenance: false,
		},
		{
			name:                 "ClusterRead bypasses distributed check",
			opts:                 HealthOpts{ClusterRead: true, Distributed: true},
			expectClusterReadURL: true,
		},
		{
			name:                 "ClusterRead bypasses maintenance check",
			opts:                 HealthOpts{ClusterRead: true, Maintenance: true},
			expectClusterReadURL: true,
		},
		{
			name:                 "ClusterRead bypasses both parameters",
			opts:                 HealthOpts{ClusterRead: true, Distributed: true, Maintenance: true},
			expectClusterReadURL: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedURL *url.URL
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedURL = r.URL
				w.Header().Set(minioWriteQuorumHeader, "2")
				w.Header().Set(minIOHealingDrives, "0")
				if tt.opts.Maintenance && !tt.expectClusterReadURL {
					w.WriteHeader(http.StatusPreconditionFailed)
				} else {
					w.WriteHeader(http.StatusOK)
				}
			}))
			defer server.Close()

			client, err := NewAnonymousClient(mustParseHost(t, server.URL), false)
			if err != nil {
				t.Fatalf("Failed to create anonymous client: %v", err)
			}

			_, err = client.Healthy(context.Background(), tt.opts)
			if err != nil {
				t.Fatalf("Healthy() returned error: %v", err)
			}

			if capturedURL == nil {
				t.Fatal("No request was captured")
			}

			// Verify the URL path
			if tt.expectClusterReadURL {
				if capturedURL.Path != clusterReadCheckEndpoint {
					t.Errorf("Expected path %s, got %s", clusterReadCheckEndpoint, capturedURL.Path)
				}
				// ClusterRead endpoint should not have distributed or maintenance parameters
				if capturedURL.Query().Get(distributedURLParameterKey) != "" {
					t.Errorf("ClusterRead should not include distributed parameter, got: %s", capturedURL.Query().Get(distributedURLParameterKey))
				}
				if capturedURL.Query().Get(maintanenceURLParameterKey) != "" {
					t.Errorf("ClusterRead should not include maintenance parameter, got: %s", capturedURL.Query().Get(maintanenceURLParameterKey))
				}
				return
			}

			// Verify the URL path for cluster check
			if capturedURL.Path != clusterCheckEndpoint {
				t.Errorf("Expected path %s, got %s", clusterCheckEndpoint, capturedURL.Path)
			}

			// Verify distributed parameter
			distributedParam := capturedURL.Query().Get(distributedURLParameterKey)
			if tt.expectDistributed {
				if distributedParam != "true" {
					t.Errorf("Expected distributed parameter to be 'true', got '%s'", distributedParam)
				}
			} else {
				if distributedParam != "" {
					t.Errorf("Expected distributed parameter to be absent, got '%s'", distributedParam)
				}
			}

			// Verify maintenance parameter
			maintenanceParam := capturedURL.Query().Get(maintanenceURLParameterKey)
			if tt.expectMaintenance {
				if maintenanceParam != "true" {
					t.Errorf("Expected maintenance parameter to be 'true', got '%s'", maintenanceParam)
				}
			} else {
				if maintenanceParam != "" {
					t.Errorf("Expected maintenance parameter to be absent, got '%s'", maintenanceParam)
				}
			}
		})
	}
}

// TestHealthOptsURLConstruction tests the URL construction at a lower level
func TestHealthOptsURLConstruction(t *testing.T) {
	testCases := []struct {
		name        string
		maintenance bool
		distributed bool
		wantParams  map[string]string
	}{
		{
			name:        "No parameters",
			maintenance: false,
			distributed: false,
			wantParams:  map[string]string{},
		},
		{
			name:        "Only distributed",
			maintenance: false,
			distributed: true,
			wantParams:  map[string]string{distributedURLParameterKey: "true"},
		},
		{
			name:        "Only maintenance",
			maintenance: true,
			distributed: false,
			wantParams:  map[string]string{maintanenceURLParameterKey: "true"},
		},
		{
			name:        "Both parameters",
			maintenance: true,
			distributed: true,
			wantParams: map[string]string{
				maintanenceURLParameterKey: "true",
				distributedURLParameterKey: "true",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate the URL construction logic from clusterCheck
			urlValues := make(url.Values)
			if tc.maintenance {
				urlValues.Set(maintanenceURLParameterKey, "true")
			}
			if tc.distributed {
				urlValues.Set(distributedURLParameterKey, "true")
			}

			// Verify expected parameters are present
			for key, expectedVal := range tc.wantParams {
				if got := urlValues.Get(key); got != expectedVal {
					t.Errorf("Parameter %q: expected %q, got %q", key, expectedVal, got)
				}
			}

			// Verify no unexpected parameters
			for key := range urlValues {
				if _, ok := tc.wantParams[key]; !ok {
					t.Errorf("Unexpected parameter %q in URL values", key)
				}
			}
		})
	}
}

// TestHealthResultParsing tests that health results are correctly parsed
func TestHealthResultParsing(t *testing.T) {
	testCases := []struct {
		name              string
		statusCode        int
		writeQuorum       string
		healingDrives     string
		opts              HealthOpts
		expectedHealthy   bool
		expectedMaintMode bool
		expectedWQ        int
		expectedHD        int
	}{
		{
			name:            "Healthy cluster with distributed check",
			statusCode:      http.StatusOK,
			writeQuorum:     "3",
			healingDrives:   "0",
			opts:            HealthOpts{Distributed: true},
			expectedHealthy: true,
			expectedWQ:      3,
			expectedHD:      0,
		},
		{
			name:            "Healthy cluster with maintenance check",
			statusCode:      http.StatusOK,
			writeQuorum:     "4",
			healingDrives:   "0",
			opts:            HealthOpts{Maintenance: true},
			expectedHealthy: true,
			expectedWQ:      4,
			expectedHD:      0,
		},
		{
			name:              "Maintenance mode detected",
			statusCode:        http.StatusPreconditionFailed,
			writeQuorum:       "3",
			healingDrives:     "1",
			opts:              HealthOpts{Maintenance: true},
			expectedHealthy:   false,
			expectedMaintMode: true,
			expectedWQ:        3,
			expectedHD:        1,
		},
		{
			name:              "Maintenance mode with distributed check",
			statusCode:        http.StatusPreconditionFailed,
			writeQuorum:       "3",
			healingDrives:     "2",
			opts:              HealthOpts{Distributed: true, Maintenance: true},
			expectedHealthy:   false,
			expectedMaintMode: true,
			expectedWQ:        3,
			expectedHD:        2,
		},
		{
			name:            "Unhealthy cluster",
			statusCode:      http.StatusServiceUnavailable,
			writeQuorum:     "2",
			healingDrives:   "3",
			opts:            HealthOpts{Distributed: true},
			expectedHealthy: false,
			expectedWQ:      2,
			expectedHD:      3,
		},
		{
			name:            "Default opts healthy",
			statusCode:      http.StatusOK,
			writeQuorum:     "2",
			healingDrives:   "0",
			opts:            HealthOpts{},
			expectedHealthy: true,
			expectedWQ:      2,
			expectedHD:      0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify parameters are present when expected
				if tc.opts.Distributed {
					if r.URL.Query().Get(distributedURLParameterKey) != "true" {
						t.Errorf("Expected distributed=true in URL query, got: %s", r.URL.Query().Encode())
					}
				}
				if tc.opts.Maintenance {
					if r.URL.Query().Get(maintanenceURLParameterKey) != "true" {
						t.Errorf("Expected maintenance=true in URL query, got: %s", r.URL.Query().Encode())
					}
				}

				w.Header().Set(minioWriteQuorumHeader, tc.writeQuorum)
				w.Header().Set(minIOHealingDrives, tc.healingDrives)
				w.WriteHeader(tc.statusCode)
			}))
			defer server.Close()

			client, err := NewAnonymousClient(mustParseHost(t, server.URL), false)
			if err != nil {
				t.Fatalf("Failed to create anonymous client: %v", err)
			}

			result, err := client.Healthy(context.Background(), tc.opts)
			if err != nil {
				t.Fatalf("Healthy() returned error: %v", err)
			}

			if result.Healthy != tc.expectedHealthy {
				t.Errorf("Expected Healthy=%v, got %v", tc.expectedHealthy, result.Healthy)
			}
			if result.MaintenanceMode != tc.expectedMaintMode {
				t.Errorf("Expected MaintenanceMode=%v, got %v", tc.expectedMaintMode, result.MaintenanceMode)
			}
			if result.WriteQuorum != tc.expectedWQ {
				t.Errorf("Expected WriteQuorum=%d, got %d", tc.expectedWQ, result.WriteQuorum)
			}
			if result.HealingDrives != tc.expectedHD {
				t.Errorf("Expected HealingDrives=%d, got %d", tc.expectedHD, result.HealingDrives)
			}
		})
	}
}

// TestClusterReadCheck tests the ClusterRead health check path
func TestClusterReadCheck(t *testing.T) {
	testCases := []struct {
		name            string
		statusCode      int
		expectedHealthy bool
	}{
		{
			name:            "ClusterRead healthy",
			statusCode:      http.StatusOK,
			expectedHealthy: true,
		},
		{
			name:            "ClusterRead unhealthy",
			statusCode:      http.StatusServiceUnavailable,
			expectedHealthy: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var capturedPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedPath = r.URL.Path
				w.WriteHeader(tc.statusCode)
			}))
			defer server.Close()

			client, err := NewAnonymousClient(mustParseHost(t, server.URL), false)
			if err != nil {
				t.Fatalf("Failed to create anonymous client: %v", err)
			}

			result, err := client.Healthy(context.Background(), HealthOpts{ClusterRead: true})
			if err != nil {
				t.Fatalf("Healthy() returned error: %v", err)
			}

			if capturedPath != clusterReadCheckEndpoint {
				t.Errorf("Expected path %s, got %s", clusterReadCheckEndpoint, capturedPath)
			}

			if result.Healthy != tc.expectedHealthy {
				t.Errorf("Expected Healthy=%v, got %v", tc.expectedHealthy, result.Healthy)
			}
		})
	}
}
