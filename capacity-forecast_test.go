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
	"strings"
	"testing"
)

func newCapacityForecastTestClient(t *testing.T, serverURL string) *AdminClient {
	t.Helper()
	client, err := New(mustParseHost(t, serverURL), "ak", "sk", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client
}

// TestCapacityForecastRequest verifies the client issues GET against the
// capacity-forecast admin path.
func TestCapacityForecastRequest(t *testing.T) {
	var capturedMethod, capturedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	if _, err := newCapacityForecastTestClient(t, server.URL).CapacityForecast(context.Background()); err != nil {
		t.Fatalf("CapacityForecast: %v", err)
	}

	if capturedMethod != http.MethodGet {
		t.Errorf("expected GET, got %s", capturedMethod)
	}
	if !strings.HasPrefix(capturedPath, "/minio/admin/") {
		t.Errorf("path missing /minio/admin/ prefix: %s", capturedPath)
	}
	if !strings.HasSuffix(capturedPath, "/capacity-forecast") {
		t.Errorf("path missing /capacity-forecast suffix: %s", capturedPath)
	}
}

// TestCapacityForecastDecode covers JSON decoding for both nil and non-nil
// pointer fields, including negative values for thresholds already crossed.
func TestCapacityForecastDecode(t *testing.T) {
	cases := []struct {
		name         string
		body         string
		wantNil80    bool
		wantNil100   bool
		wantVal80    float64
		wantNegative bool
	}{
		{
			name:       "days-until fields omitted produce nil pointers",
			body:       `{"currentUsedBytes":100,"currentTotalBytes":1000,"dailySnapshotCount":7}`,
			wantNil80:  true,
			wantNil100: true,
		},
		{
			name:      "positive days-until values decode as expected",
			body:      `{"daysUntil80Pct":42.5,"daysUntil100Pct":120}`,
			wantNil80: false,
			wantVal80: 42.5,
		},
		{
			name:         "negative days-until preserved (threshold already crossed)",
			body:         `{"daysUntil80Pct":-3.2,"daysUntil100Pct":-1}`,
			wantNil80:    false,
			wantVal80:    -3.2,
			wantNegative: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tc.body))
			}))
			defer server.Close()

			f, err := newCapacityForecastTestClient(t, server.URL).CapacityForecast(context.Background())
			if err != nil {
				t.Fatalf("CapacityForecast: %v", err)
			}

			if tc.wantNil80 {
				if f.DaysUntil80Pct != nil {
					t.Errorf("expected DaysUntil80Pct nil, got %v", *f.DaysUntil80Pct)
				}
			} else {
				if f.DaysUntil80Pct == nil {
					t.Fatal("expected DaysUntil80Pct non-nil")
				}
				if *f.DaysUntil80Pct != tc.wantVal80 {
					t.Errorf("DaysUntil80Pct: want %v, got %v", tc.wantVal80, *f.DaysUntil80Pct)
				}
				if tc.wantNegative && *f.DaysUntil80Pct >= 0 {
					t.Errorf("expected negative value, got %v", *f.DaysUntil80Pct)
				}
			}

			if tc.wantNil100 && f.DaysUntil100Pct != nil {
				t.Errorf("expected DaysUntil100Pct nil, got %v", *f.DaysUntil100Pct)
			}
		})
	}
}

// TestCapacityForecastHTTPError verifies non-200 responses produce an error
// rather than a zero-value struct.
func TestCapacityForecastHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	if _, err := newCapacityForecastTestClient(t, server.URL).CapacityForecast(context.Background()); err == nil {
		t.Fatal("expected error on 500, got nil")
	}
}
