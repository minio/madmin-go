package madmin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestListAccessKeysLDAPBulkConfig(t *testing.T) {
	for _, tc := range []struct {
		name       string
		configName string
	}{
		{"default config", ""},
		{"named config", "tcad"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			captured := make(chan url.Values, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				captured <- r.URL.Query()
				w.WriteHeader(http.StatusBadRequest)
			}))
			defer server.Close()
			adm, err := New(strings.TrimPrefix(server.URL, "http://"), "access", "secret", false)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			_, _ = adm.ListAccessKeysLDAPBulkWithConfig(context.Background(), []string{"uid=alice"}, "", false, tc.configName)
			got := <-captured
			if got.Get("listType") != AccessKeyListAll {
				t.Fatalf("listType = %q, want %q", got.Get("listType"), AccessKeyListAll)
			}
			if got.Get("userDNs") != "uid=alice" {
				t.Fatalf("userDNs = %q", got.Get("userDNs"))
			}
			if _, ok := got["configName"]; ok != (tc.configName != "") || got.Get("configName") != tc.configName {
				t.Fatalf("configName = %q (present %v), want %q", got.Get("configName"), ok, tc.configName)
			}
		})
	}
}

func TestListAccessKeysLDAPBulkAllConfigsUnsupported(t *testing.T) {
	var requested atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requested.Store(true)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	adm, err := New(strings.TrimPrefix(server.URL, "http://"), "access", "secret", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := adm.ListAccessKeysLDAPBulkWithOpts(context.Background(), nil, ListAccessKeysOpts{All: true, AllConfigs: true}); err == nil {
		t.Fatal("expected an error when allConfigs is set for LDAP")
	}
	if requested.Load() {
		t.Fatal("request with allConfigs reached the server")
	}
}
