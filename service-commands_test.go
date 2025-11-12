// MinIO, Inc. CONFIDENTIAL
//
// [2014] - [2025] MinIO, Inc. All Rights Reserved.
//
// NOTICE:  All information contained herein is, and remains the property
// of MinIO, Inc and its suppliers, if any.  The intellectual and technical
// concepts contained herein are proprietary to MinIO, Inc and its suppliers
// and may be covered by U.S. and Foreign Patents, patents in process, and are
// protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written
// permission is obtained from MinIO, Inc.

package madmin

import (
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestServiceTraceOptsTables(t *testing.T) {
	opts := ServiceTraceOpts{Tables: true}
	if got := opts.TraceTypes(); !got.Contains(TraceTables) {
		t.Fatalf("TraceTypes() missing TraceTables: got %v", got)
	}

	vals := make(url.Values)
	opts.AddParams(vals)
	if got := vals.Get("tables"); got != "true" {
		t.Fatalf("AddParams() tables flag = %q, want true", got)
	}

	req := httptest.NewRequest("GET", "/minio/admin/v3/trace?tables=true", nil)
	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm() returned error = %v", err)
	}

	var parsed ServiceTraceOpts
	if err := parsed.ParseParams(req); err != nil {
		t.Fatalf("ParseParams() returned error = %v", err)
	}

	if !parsed.Tables {
		t.Fatalf("ParseParams() did not set Tables flag")
	}
}
