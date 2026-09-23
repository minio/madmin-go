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
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"testing"
	"time"
)

// parseParams runs vals through ParseParams as a server would see them.
func parseParams(t *testing.T, vals url.Values) ServiceTraceOpts {
	t.Helper()
	req := httptest.NewRequest("GET", "/minio/admin/v3/trace?"+vals.Encode(), nil)
	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm() returned error = %v", err)
	}
	var parsed ServiceTraceOpts
	if err := parsed.ParseParams(req); err != nil {
		t.Fatalf("ParseParams() returned error = %v", err)
	}
	return parsed
}

// TestServiceTraceOptsLegacyParams checks that a type set through the Types
// bitfield still reaches servers that only understand the per-type params.
func TestServiceTraceOptsLegacyParams(t *testing.T) {
	for param, typ := range legacyTraceParams {
		t.Run(param, func(t *testing.T) {
			vals := make(url.Values)
			ServiceTraceOpts{Types: typ}.AddParams(vals)
			if got := vals.Get(param); got != "true" {
				t.Fatalf("AddParams() %s = %q, want true", param, got)
			}
			for other := range legacyTraceParams {
				if other == param {
					continue
				}
				if got := vals.Get(other); got != "false" {
					t.Fatalf("AddParams() %s = %q, want false", other, got)
				}
			}
		})
	}
}

// TestServiceTraceOptsRoundTrip checks that each single trace type survives
// AddParams -> ParseParams, including types with no legacy param.
func TestServiceTraceOptsRoundTrip(t *testing.T) {
	for typ := TraceType(1); typ <= TraceAll; typ <<= 1 {
		t.Run(typ.String(), func(t *testing.T) {
			vals := make(url.Values)
			ServiceTraceOpts{Types: typ}.AddParams(vals)
			if got := parseParams(t, vals).TraceTypes(); got != typ {
				t.Fatalf("round trip = %v (%d), want %v (%d)", got, got, typ, typ)
			}
		})
	}
}

// TestServiceTraceOptsDeprecatedRoundTrip checks that every deprecated bool
// field still selects the same trace types after a round trip.
func TestServiceTraceOptsDeprecatedRoundTrip(t *testing.T) {
	v := reflect.ValueOf(&ServiceTraceOpts{}).Elem()
	for i := range v.NumField() {
		f := v.Type().Field(i)
		if f.Type.Kind() != reflect.Bool || f.Name == "OnlyErrors" {
			continue
		}
		t.Run(f.Name, func(t *testing.T) {
			var opts ServiceTraceOpts
			reflect.ValueOf(&opts).Elem().Field(i).SetBool(true)
			want := opts.TraceTypes()
			if want == 0 {
				t.Fatalf("%s does not map to any trace type", f.Name)
			}
			vals := make(url.Values)
			opts.AddParams(vals)
			parsed := parseParams(t, vals)
			if got := parsed.TraceTypes(); got != want {
				t.Fatalf("round trip = %v, want %v", got, want)
			}
			if parsed.Types != want {
				t.Fatalf("ParseParams() Types = %v, want %v", parsed.Types, want)
			}
		})
	}
}

// oldClientAddParams is a frozen copy of AddParams from before the Types
// bitfield was introduced. Do not refactor it to share code with AddParams;
// its purpose is to pin the wire format an old client emits.
func oldClientAddParams(t ServiceTraceOpts, u url.Values) {
	u.Set("err", strconv.FormatBool(t.OnlyErrors))
	u.Set("threshold", t.Threshold.String())
	u.Set("threshold-ttfb", t.ThresholdTTFB.String())

	u.Set("s3", strconv.FormatBool(t.S3))
	u.Set("internal", strconv.FormatBool(t.Internal))
	u.Set("storage", strconv.FormatBool(t.Storage))
	u.Set("os", strconv.FormatBool(t.OS))
	u.Set("scanner", strconv.FormatBool(t.Scanner))
	u.Set("decommission", strconv.FormatBool(t.Decommission))
	u.Set("healing", strconv.FormatBool(t.Healing))
	u.Set("batch-replication", strconv.FormatBool(t.BatchAll || t.BatchReplication))
	u.Set("batch-keyrotation", strconv.FormatBool(t.BatchAll || t.BatchKeyRotation))
	u.Set("batch-expire", strconv.FormatBool(t.BatchAll || t.BatchExpire))
	u.Set("rebalance", strconv.FormatBool(t.Rebalance))
	u.Set("tables", strconv.FormatBool(t.Tables))
	u.Set("tables-scan", strconv.FormatBool(t.TablesScan))
	u.Set("replication-resync", strconv.FormatBool(t.ReplicationResync))
	u.Set("bootstrap", strconv.FormatBool(t.Bootstrap))
	u.Set("ftp", strconv.FormatBool(t.FTP))
	u.Set("ilm", strconv.FormatBool(t.ILM))
	u.Set("kms", strconv.FormatBool(t.KMS))
	u.Set("formatting", strconv.FormatBool(t.Formatting))
	u.Set("admin", strconv.FormatBool(t.Admin))
	u.Set("object", strconv.FormatBool(t.Object))
	u.Set("replication", strconv.FormatBool(t.Replication))
	u.Set("iam", strconv.FormatBool(t.IAM))
	u.Set("purgeondelete", strconv.FormatBool(t.PurgeOnDelete))
	u.Set("systeminventory", strconv.FormatBool(t.SystemInventory))
	u.Set("tables-compaction", strconv.FormatBool(t.TablesCompaction))
}

// oldServerParseParams is a frozen copy of ParseParams from before the Types
// bitfield was introduced; it knows nothing about the "types" param.
func oldServerParseParams(t *ServiceTraceOpts, r *http.Request) error {
	t.S3 = r.Form.Get("s3") == "true"
	t.OS = r.Form.Get("os") == "true"
	t.Scanner = r.Form.Get("scanner") == "true"
	t.Decommission = r.Form.Get("decommission") == "true"
	t.Healing = r.Form.Get("healing") == "true"
	t.BatchReplication = r.Form.Get("batch-replication") == "true"
	t.BatchKeyRotation = r.Form.Get("batch-keyrotation") == "true"
	t.BatchExpire = r.Form.Get("batch-expire") == "true"
	t.Rebalance = r.Form.Get("rebalance") == "true"
	t.Tables = r.Form.Get("tables") == "true"
	t.TablesScan = r.Form.Get("tables-scan") == "true"
	t.Storage = r.Form.Get("storage") == "true"
	t.Internal = r.Form.Get("internal") == "true"
	t.OnlyErrors = r.Form.Get("err") == "true"
	t.ReplicationResync = r.Form.Get("replication-resync") == "true"
	t.Bootstrap = r.Form.Get("bootstrap") == "true"
	t.FTP = r.Form.Get("ftp") == "true"
	t.ILM = r.Form.Get("ilm") == "true"
	t.KMS = r.Form.Get("kms") == "true"
	t.Formatting = r.Form.Get("formatting") == "true"
	t.Admin = r.Form.Get("admin") == "true"
	t.Object = r.Form.Get("object") == "true"
	t.Replication = r.Form.Get("replication") == "true"
	t.IAM = r.Form.Get("iam") == "true"
	t.PurgeOnDelete = r.Form.Get("purgeondelete") == "true"
	t.SystemInventory = r.Form.Get("systeminventory") == "true"
	t.TablesCompaction = r.Form.Get("tables-compaction") == "true"

	if th := r.Form.Get("threshold"); th != "" {
		d, err := time.ParseDuration(th)
		if err != nil {
			return err
		}
		t.Threshold = d
	}

	if thTTFB := r.Form.Get("threshold-ttfb"); thTTFB != "" {
		d, err := time.ParseDuration(thTTFB)
		if err != nil {
			return err
		}
		t.ThresholdTTFB = d
	}

	return nil
}

func request(t *testing.T, vals url.Values) *http.Request {
	t.Helper()
	req := httptest.NewRequest("GET", "/minio/admin/v3/trace?"+vals.Encode(), nil)
	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm() returned error = %v", err)
	}
	return req
}

// TestServiceTraceOptsNewClientOldServer feeds what the current client emits to
// a server that only understands the deprecated per-type params.
func TestServiceTraceOptsNewClientOldServer(t *testing.T) {
	for param, typ := range legacyTraceParams {
		t.Run(param, func(t *testing.T) {
			vals := make(url.Values)
			ServiceTraceOpts{Types: typ, OnlyErrors: true, Threshold: time.Second}.AddParams(vals)

			var old ServiceTraceOpts
			if err := oldServerParseParams(&old, request(t, vals)); err != nil {
				t.Fatalf("old server parse returned error = %v", err)
			}
			if got := old.TraceTypes(); got != typ {
				t.Fatalf("old server saw %v, want %v", got, typ)
			}
			if !old.OnlyErrors || old.Threshold != time.Second {
				t.Fatalf("old server lost err/threshold: %+v", old)
			}
		})
	}
}

// TestServiceTraceOptsOldClientNewServer feeds what a pre-Types client emits to
// the current server.
func TestServiceTraceOptsOldClientNewServer(t *testing.T) {
	v := reflect.ValueOf(&ServiceTraceOpts{}).Elem()
	for i := range v.NumField() {
		f := v.Type().Field(i)
		if f.Type.Kind() != reflect.Bool || f.Name == "OnlyErrors" {
			continue
		}
		t.Run(f.Name, func(t *testing.T) {
			var opts ServiceTraceOpts
			reflect.ValueOf(&opts).Elem().Field(i).SetBool(true)
			opts.OnlyErrors = true
			opts.Threshold = time.Second
			want := opts.TraceTypes()

			vals := make(url.Values)
			oldClientAddParams(opts, vals)
			if vals.Has("types") {
				t.Fatal("old client must not emit a types param")
			}

			var parsed ServiceTraceOpts
			if err := parsed.ParseParams(request(t, vals)); err != nil {
				t.Fatalf("ParseParams() returned error = %v", err)
			}
			if got := parsed.TraceTypes(); got != want {
				t.Fatalf("new server saw %v, want %v", got, want)
			}
			if parsed.Types != want {
				t.Fatalf("ParseParams() Types = %v, want %v", parsed.Types, want)
			}
			if !parsed.OnlyErrors || parsed.Threshold != time.Second {
				t.Fatalf("new server lost err/threshold: %+v", parsed)
			}
		})
	}
}

// TestServiceTraceOptsZeroTypes checks that a client sending types=0 alongside
// the deprecated params still selects traces via the deprecated params.
func TestServiceTraceOptsZeroTypes(t *testing.T) {
	vals := make(url.Values)
	oldClientAddParams(ServiceTraceOpts{S3: true}, vals)
	vals.Set("types", "0")

	var parsed ServiceTraceOpts
	if err := parsed.ParseParams(request(t, vals)); err != nil {
		t.Fatalf("ParseParams() returned error = %v", err)
	}
	if got := parsed.TraceTypes(); got != TraceS3 {
		t.Fatalf("types=0 with s3=true parsed as %v, want %v", got, TraceS3)
	}
}

func TestServiceTraceOptsRoundTripOther(t *testing.T) {
	opts := ServiceTraceOpts{
		Types:         TraceS3 | TraceMemory,
		OnlyErrors:    true,
		Threshold:     time.Second,
		ThresholdTTFB: 500 * time.Millisecond,
	}
	vals := make(url.Values)
	opts.AddParams(vals)
	if got := parseParams(t, vals); got != opts {
		t.Fatalf("round trip = %+v, want %+v", got, opts)
	}
}

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

func TestServiceTraceOptsTablesCompaction(t *testing.T) {
	opts := ServiceTraceOpts{TablesCompaction: true}
	if got := opts.TraceTypes(); !got.Contains(TraceTablesCompaction) {
		t.Fatalf("TraceTypes() missing TraceTablesCompaction: got %v", got)
	}

	vals := make(url.Values)
	opts.AddParams(vals)
	if got := vals.Get("tables-compaction"); got != "true" {
		t.Fatalf("AddParams() tables-compaction flag = %q, want true", got)
	}

	req := httptest.NewRequest("GET", "/minio/admin/v3/trace?tables-compaction=true", nil)
	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm() returned error = %v", err)
	}

	var parsed ServiceTraceOpts
	if err := parsed.ParseParams(req); err != nil {
		t.Fatalf("ParseParams() returned error = %v", err)
	}

	if !parsed.TablesCompaction {
		t.Fatalf("ParseParams() did not set TablesCompaction flag")
	}
}

func TestServiceTraceOptsSystemInventory(t *testing.T) {
	opts := ServiceTraceOpts{SystemInventory: true}
	if got := opts.TraceTypes(); !got.Contains(TraceSystemInventory) {
		t.Fatalf("TraceTypes() missing TraceSystemInventory: got %v", got)
	}

	vals := make(url.Values)
	opts.AddParams(vals)
	if got := vals.Get("systeminventory"); got != "true" {
		t.Fatalf("AddParams() systeminventory flag = %q, want true", got)
	}

	req := httptest.NewRequest("GET", "/minio/admin/v3/trace?systeminventory=true", nil)
	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm() returned error = %v", err)
	}

	var parsed ServiceTraceOpts
	if err := parsed.ParseParams(req); err != nil {
		t.Fatalf("ParseParams() returned error = %v", err)
	}

	if !parsed.SystemInventory {
		t.Fatalf("ParseParams() did not set SystemInventory flag")
	}
}
