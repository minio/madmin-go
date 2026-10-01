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

func TestListAlertsInMemoryOnly(t *testing.T) {
	body := encodeAlerts(t, Alert{Title: "a", DedupKey: "1"}, Alert{Title: "b", DedupKey: "2"})
	cases := []struct {
		name   string
		header string
		want   bool
	}{
		{"header true", "true", true},
		{"header absent", "", false},
		{"header other value", "false", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newAlertsTestServer(t, tc.header, http.StatusOK, body)
			res, err := client.ListAlerts(context.Background(), AlertLogOpts{})
			if err != nil {
				t.Fatalf("ListAlerts: %v", err)
			}
			if res.InMemoryOnly != tc.want {
				t.Errorf("InMemoryOnly = %v, want %v", res.InMemoryOnly, tc.want)
			}
			if len(res.Alerts) != 2 || res.Alerts[0].Title != "a" || res.Alerts[1].Title != "b" {
				t.Errorf("unexpected alerts: %+v", res.Alerts)
			}
		})
	}
}

func TestListAlertsEmpty(t *testing.T) {
	client := newAlertsTestServer(t, "true", http.StatusOK, nil)
	res, err := client.ListAlerts(context.Background(), AlertLogOpts{})
	if err != nil {
		t.Fatalf("ListAlerts: %v", err)
	}
	if !res.InMemoryOnly {
		t.Error("InMemoryOnly = false, want true")
	}
	if len(res.Alerts) != 0 {
		t.Errorf("expected no alerts, got %d", len(res.Alerts))
	}
}

func TestListAlertsErrorStatus(t *testing.T) {
	client := newAlertsTestServer(t, "", http.StatusForbidden, []byte(`{"Code":"AccessDenied","Message":"denied"}`))
	res, err := client.ListAlerts(context.Background(), AlertLogOpts{})
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
	if len(res.Alerts) != 0 || res.InMemoryOnly {
		t.Errorf("expected zero result on error, got %+v", res)
	}
}

func TestListAlertsTruncatedStream(t *testing.T) {
	full := encodeAlerts(t, Alert{Title: "a", DedupKey: "1"}, Alert{Title: "b", DedupKey: "2"})
	first := encodeAlerts(t, Alert{Title: "a", DedupKey: "1"})
	client := newAlertsTestServer(t, "true", http.StatusOK, full[:len(first)+(len(full)-len(first))/2])
	res, err := client.ListAlerts(context.Background(), AlertLogOpts{})
	if err == nil {
		t.Fatal("expected decode error for truncated stream")
	}
	if len(res.Alerts) != 1 || res.Alerts[0].Title != "a" {
		t.Errorf("expected the one fully decoded alert, got %+v", res.Alerts)
	}
	if !res.InMemoryOnly {
		t.Error("InMemoryOnly = false, want true")
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
