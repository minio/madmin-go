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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestClientPerfReturnsServerError checks that ClientPerf returns the server's
// error when either client speedtest route refuses, instead of a result built
// from the error body.
func TestClientPerfReturnsServerError(t *testing.T) {
	const refusal = `{"Code":"XMinioPaidTierLicenseRequired","Message":"This feature is not available for Free tier licenses."}`

	for _, refused := range []string{"/speedtest/client/devnull", "/speedtest/client/devnull/extratime"} {
		t.Run(refused, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, refused) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusMethodNotAllowed)
					w.Write([]byte(refusal))
					return
				}
				if strings.HasSuffix(r.URL.Path, "/extratime") {
					w.Write([]byte(`{"dur":1}`))
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			adm, err := New(server.URL[len("http://"):], "access", "secret", false)
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			_, err = adm.ClientPerf(context.Background(), time.Second)
			var errResp ErrorResponse
			if !errors.As(err, &errResp) || errResp.Code != "XMinioPaidTierLicenseRequired" {
				t.Fatalf("ClientPerf error = %v, want XMinioPaidTierLicenseRequired", err)
			}
		})
	}
}
