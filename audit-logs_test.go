//
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
//

package madmin

import (
	"context"
	"io"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/minio/madmin-go/v4/log"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// auditLogStreamBody serves one encoded audit record and then reports EOF.
// Once the test marks reads as observed, every read counts as a read after
// the consumer stopped, so a drain of the stream fails the test.
type auditLogStreamBody struct {
	record       []byte
	served       bool
	observeReads atomic.Bool
	reads        atomic.Int32
	closed       atomic.Bool
}

func (b *auditLogStreamBody) Read(p []byte) (int, error) {
	if !b.served {
		b.served = true
		return copy(p, b.record), nil
	}
	if b.observeReads.Load() {
		b.reads.Add(1)
	}
	return 0, io.EOF
}

func (b *auditLogStreamBody) Close() error {
	b.closed.Store(true)
	return nil
}

// auditLogStaticTransport answers every request with status and body, without
// a network.
type auditLogStaticTransport struct {
	status int
	body   io.ReadCloser
}

func (rt *auditLogStaticTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		_ = req.Body.Close()
	}
	return &http.Response{
		StatusCode: rt.status,
		Header:     make(http.Header),
		Body:       rt.body,
		Request:    req,
	}, nil
}

// A consumer that stops reading audit logs early must close the response body
// without draining the rest of the stream; otherwise the connection leaks and
// an open stream can block the iterator.
func TestGetAuditLogsClosesResponseWhenStoppedEarly(t *testing.T) {
	var rec log.Audit
	encoded, err := rec.MarshalMsg(nil)
	if err != nil {
		t.Fatalf("MarshalMsg: %v", err)
	}
	body := &auditLogStreamBody{record: encoded}

	client, err := NewWithOptions("localhost:9000", &Options{
		Creds:     credentials.NewStaticV4("ak", "sk", ""),
		Transport: &auditLogStaticTransport{status: http.StatusOK, body: body},
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}

	count := 0
	for _, err := range client.GetAuditLogs(context.Background(), AuditLogOpts{}) {
		if err != nil {
			t.Fatalf("GetAuditLogs: %v", err)
		}
		count++
		body.observeReads.Store(true)
		break
	}
	if count != 1 {
		t.Fatalf("received %d audit records, want 1", count)
	}
	if !body.closed.Load() {
		t.Fatal("response body was not closed when the iterator stopped early")
	}
	if n := body.reads.Load(); n != 0 {
		t.Fatalf("the iterator read the stream %d more times after the consumer stopped", n)
	}
}
