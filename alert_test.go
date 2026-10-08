// Copyright (c) 2015-2026 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
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
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tinylib/msgp/msgp"
)

func encodeAlerts(t *testing.T, alerts ...Alert) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := msgp.NewWriter(&buf)
	for i := range alerts {
		if err := alerts[i].EncodeMsg(w); err != nil {
			t.Fatalf("EncodeMsg: %v", err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	return buf.Bytes()
}

func newAlertsTestServer(t *testing.T, header string, status int, body []byte) *AdminClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/alerts") {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if header != "" {
			w.Header().Set(AlertsInMemoryOnlyHeader, header)
		}
		w.WriteHeader(status)
		w.Write(body)
	}))
	t.Cleanup(server.Close)
	client, err := New(mustParseHost(t, server.URL), "ak", "sk", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client
}

// collectAlerts drains GetAlerts, stopping at the first error. It counts
// OnInMemoryOnly calls and fails if one arrives after an alert.
func collectAlerts(client *AdminClient, opts AlertLogOpts) (titles []string, inMemoryOnly int, err error) {
	var lateCallback bool
	opts.OnInMemoryOnly = func() {
		if len(titles) > 0 {
			lateCallback = true
		}
		inMemoryOnly++
	}
	for alert, err := range client.GetAlerts(context.Background(), opts) {
		if err != nil {
			return titles, inMemoryOnly, err
		}
		titles = append(titles, alert.Title)
	}
	if lateCallback {
		return titles, inMemoryOnly, errors.New("OnInMemoryOnly called after an alert")
	}
	return titles, inMemoryOnly, nil
}

func TestGetAlertsOnInMemoryOnly(t *testing.T) {
	body := encodeAlerts(t, Alert{Title: "a", DedupKey: "1"}, Alert{Title: "b", DedupKey: "2"})
	cases := []struct {
		name   string
		header string
		want   int
	}{
		{"header true", "true", 1},
		{"header absent", "", 0},
		{"header other value", "false", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newAlertsTestServer(t, tc.header, http.StatusOK, body)
			titles, inMemoryOnly, err := collectAlerts(client, AlertLogOpts{})
			if err != nil {
				t.Fatalf("GetAlerts: %v", err)
			}
			if inMemoryOnly != tc.want {
				t.Errorf("OnInMemoryOnly called %d times, want %d", inMemoryOnly, tc.want)
			}
			if strings.Join(titles, ",") != "a,b" {
				t.Errorf("titles = %v, want [a b]", titles)
			}
		})
	}
}

func TestGetAlertsInMemoryOnlyEmpty(t *testing.T) {
	client := newAlertsTestServer(t, "true", http.StatusOK, nil)
	titles, inMemoryOnly, err := collectAlerts(client, AlertLogOpts{})
	if err != nil {
		t.Fatalf("GetAlerts: %v", err)
	}
	if inMemoryOnly != 1 {
		t.Errorf("OnInMemoryOnly called %d times, want 1", inMemoryOnly)
	}
	if len(titles) != 0 {
		t.Errorf("expected no alerts, got %v", titles)
	}
}

func TestGetAlertsInMemoryOnlyErrorStatus(t *testing.T) {
	client := newAlertsTestServer(t, "true", http.StatusForbidden, []byte(`{"Code":"AccessDenied","Message":"denied"}`))
	_, inMemoryOnly, err := collectAlerts(client, AlertLogOpts{})
	if err == nil {
		t.Fatal("expected an error")
	}
	if inMemoryOnly != 0 {
		t.Errorf("OnInMemoryOnly called %d times on a failed request, want 0", inMemoryOnly)
	}
}

func TestGetAlertsOnInMemoryOnlyNotSent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if bytes.Contains(bytes.ToLower(body), []byte("inmemory")) {
			t.Errorf("OnInMemoryOnly leaked into the request body: %s", body)
		}
	}))
	t.Cleanup(server.Close)
	client, err := New(mustParseHost(t, server.URL), "ak", "sk", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, _, err := collectAlerts(client, AlertLogOpts{}); err != nil {
		t.Fatalf("GetAlerts: %v", err)
	}
}

func TestGetAlertsStreams(t *testing.T) {
	client := newAlertsTestServer(t, "true", http.StatusOK, encodeAlerts(t, Alert{Title: "a"}, Alert{Title: "b"}))
	var titles []string
	for alert, err := range client.GetAlerts(context.Background(), AlertLogOpts{}) {
		if err != nil {
			t.Fatalf("GetAlerts: %v", err)
		}
		titles = append(titles, alert.Title)
	}
	if strings.Join(titles, ",") != "a,b" {
		t.Errorf("titles = %v, want [a b]", titles)
	}
}

func TestGetAlertsErrorStatus(t *testing.T) {
	client := newAlertsTestServer(t, "", http.StatusForbidden, []byte(`{"Code":"AccessDenied","Message":"denied"}`))
	var errs int
	for alert, err := range client.GetAlerts(context.Background(), AlertLogOpts{}) {
		if alert != nil {
			t.Errorf("unexpected alert %+v", alert)
		}
		if err != nil {
			errs++
		}
	}
	if errs != 1 {
		t.Errorf("expected exactly one error, got %d", errs)
	}
}
