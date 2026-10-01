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

// collectAlerts drains GetAlerts, stopping at the first error other than
// ErrAlertsInMemoryOnly.
func collectAlerts(client *AdminClient, opts AlertLogOpts) (titles []string, inMemoryOnly int, err error) {
	for alert, err := range client.GetAlerts(context.Background(), opts) {
		if errors.Is(err, ErrAlertsInMemoryOnly) {
			if len(titles) > 0 {
				return titles, inMemoryOnly, errors.New("ErrAlertsInMemoryOnly yielded after an alert")
			}
			inMemoryOnly++
			continue
		}
		if err != nil {
			return titles, inMemoryOnly, err
		}
		titles = append(titles, alert.Title)
	}
	return titles, inMemoryOnly, nil
}

func TestGetAlertsReportInMemoryOnly(t *testing.T) {
	body := encodeAlerts(t, Alert{Title: "a", DedupKey: "1"}, Alert{Title: "b", DedupKey: "2"})
	cases := []struct {
		name   string
		header string
		report bool
		want   int
	}{
		{"header true", "true", true, 1},
		{"header absent", "", true, 0},
		{"header other value", "false", true, 0},
		{"header true, not requested", "true", false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newAlertsTestServer(t, tc.header, http.StatusOK, body)
			titles, inMemoryOnly, err := collectAlerts(client, AlertLogOpts{ReportInMemoryOnly: tc.report})
			if err != nil {
				t.Fatalf("GetAlerts: %v", err)
			}
			if inMemoryOnly != tc.want {
				t.Errorf("ErrAlertsInMemoryOnly yielded %d times, want %d", inMemoryOnly, tc.want)
			}
			if strings.Join(titles, ",") != "a,b" {
				t.Errorf("titles = %v, want [a b]", titles)
			}
		})
	}
}

func TestGetAlertsInMemoryOnlyEmpty(t *testing.T) {
	client := newAlertsTestServer(t, "true", http.StatusOK, nil)
	titles, inMemoryOnly, err := collectAlerts(client, AlertLogOpts{ReportInMemoryOnly: true})
	if err != nil {
		t.Fatalf("GetAlerts: %v", err)
	}
	if inMemoryOnly != 1 {
		t.Errorf("ErrAlertsInMemoryOnly yielded %d times, want 1", inMemoryOnly)
	}
	if len(titles) != 0 {
		t.Errorf("expected no alerts, got %v", titles)
	}
}

func TestGetAlertsInMemoryOnlyStopEarly(t *testing.T) {
	client := newAlertsTestServer(t, "true", http.StatusOK, encodeAlerts(t, Alert{Title: "a"}))
	var calls int
	for range client.GetAlerts(context.Background(), AlertLogOpts{ReportInMemoryOnly: true}) {
		calls++
		break
	}
	if calls != 1 {
		t.Errorf("expected iteration to stop after the first yield, got %d", calls)
	}
}

func TestGetAlertsReportInMemoryOnlyNotSent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if bytes.Contains(bytes.ToLower(body), []byte("inmemory")) {
			t.Errorf("ReportInMemoryOnly leaked into the request body: %s", body)
		}
	}))
	t.Cleanup(server.Close)
	client, err := New(mustParseHost(t, server.URL), "ak", "sk", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, _, err := collectAlerts(client, AlertLogOpts{ReportInMemoryOnly: true}); err != nil {
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
