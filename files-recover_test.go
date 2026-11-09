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
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package madmin

import (
	"context"
	"net/http"
	"testing"
	"time"
)

const filesRecoverRunning = `{"jobID": "b1.5f0c1e2d", "name": "carol", "exportID": 104,
	"node": "node03.example.com", "state": "running", "step": "reloading",
	"started": "2026-11-09T10:00:00Z"}`

// TestRecoverFilesExportRequest verifies the method, path and generation a
// recover sends, that it sends no body, and that it decodes the job.
func TestRecoverFilesExportRequest(t *testing.T) {
	server, seen := newFilesJSONServer(t, http.StatusOK, filesRecoverRunning)
	client := newFilesExportsTestClient(t, server.URL)

	job, err := client.RecoverFilesExport(context.Background(), "104", "5f2a91")
	if err != nil {
		t.Fatalf("RecoverFilesExport: %v", err)
	}
	req := (*seen)[0]
	if want := "/minio/admin/files/v1/exports/104/recover"; req.method != http.MethodPost || req.path != want {
		t.Errorf("sent %s %s, want POST %s", req.method, req.path, want)
	}
	if req.ifMatch != `"5f2a91"` {
		t.Errorf("If-Match = %q", req.ifMatch)
	}
	if len(req.body) != 0 || req.contentType != "" {
		t.Errorf("sent body %q with Content-Type %q, want none", req.body, req.contentType)
	}
	started := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	if job.JobID != "b1.5f0c1e2d" || job.ExportID != 104 || job.Node != "node03.example.com" ||
		job.State != FilesJobRunning || job.Step != FilesRecoverReloading || !job.Started.Equal(started) || job.Finished != nil {
		t.Errorf("job = %+v", job)
	}
}

// TestRecoverFilesExportRefusesLocally verifies that a recover naming no
// export or no usable generation is not sent.
func TestRecoverFilesExportRefusesLocally(t *testing.T) {
	server, seen := newFilesJSONServer(t, http.StatusOK, filesRecoverRunning)
	client := newFilesExportsTestClient(t, server.URL)

	for _, tc := range []struct {
		export     string
		generation FilesGeneration
	}{
		{"", "5f2a91"},
		{"stats", "5f2a91"},
		{"104", ""},
		{"104", `5f"2a`},
	} {
		if _, err := client.RecoverFilesExport(context.Background(), tc.export, tc.generation); err == nil {
			t.Errorf("RecoverFilesExport(%q, %q) succeeded", tc.export, tc.generation)
		}
	}
	if len(*seen) != 0 {
		t.Errorf("sent %d requests, want none", len(*seen))
	}
}

// TestRecoverFilesExportRepeatsUnknownOutcome verifies that a recover whose
// outcome is unknown is repeated, since the server answers a repeat with the
// job already running, that the repeat carries the first request's If-Match
// and no body, and that one the server refused is not.
func TestRecoverFilesExportRepeatsUnknownOutcome(t *testing.T) {
	ok := filesScriptedReply{http.StatusOK, filesRecoverRunning}
	for _, tc := range []struct {
		name   string
		first  filesScriptedReply
		writes int32
		code   string
	}{
		{"dropped_connection", filesDropped, 2, ""},
		{"bad_gateway", filesScriptedReply{http.StatusBadGateway, `bad gateway`}, 2, ""},
		{"export_open", filesScriptedReply{http.StatusConflict, `{"code": "ExportOpen", "message": "open"}`}, 1, FilesErrExportOpen},
		// A recover is not recorded, so a node that did not answer is not
		// waited for by a repeat either.
		{"node_unreachable", filesScriptedReply{http.StatusServiceUnavailable, `{"code": "NodeUnreachable", "message": "no answer"}`}, 1, FilesErrNodeUnreachable},
		{"not_implemented", filesScriptedReply{http.StatusNotImplemented, `{"code": "NotImplemented", "message": "old"}`}, 1, FilesErrNotImplemented},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server, writes, _, seen := newFilesRecordingScriptedServer(t, filesCarol, tc.first, ok)
			job, err := newFilesExportsTestClient(t, server.URL).RecoverFilesExport(context.Background(), "104", "5f2a91")
			if got := writes.Load(); got != tc.writes {
				t.Errorf("sent %d writes, want %d", got, tc.writes)
			}
			sent := seen()
			for i, req := range sent {
				if want := "/minio/admin/files/v1/exports/104/recover"; req.method != http.MethodPost || req.path != want {
					t.Errorf("write %d sent %s %s, want POST %s", i, req.method, req.path, want)
				}
				if req.ifMatch != sent[0].ifMatch || req.ifMatch != `"5f2a91"` {
					t.Errorf("write %d If-Match = %q, want %q as first sent", i, req.ifMatch, `"5f2a91"`)
				}
				if len(req.body) != 0 || req.contentType != "" {
					t.Errorf("write %d sent body %q with Content-Type %q, want none", i, req.body, req.contentType)
				}
			}
			if tc.code == "" {
				if err != nil || job.JobID != "b1.5f0c1e2d" {
					t.Errorf("RecoverFilesExport = %+v, %v; want the running job", job, err)
				}
				return
			}
			if code := ToErrorResponse(err).Code; code != tc.code {
				t.Errorf("error code = %q (%v), want %q", code, err, tc.code)
			}
		})
	}
}

// TestFilesRecoverJobStatusRequest verifies the path a job read sends, with
// the job id escaped as one segment, and that it decodes a finished job and a
// failed one.
func TestFilesRecoverJobStatusRequest(t *testing.T) {
	for _, tc := range []struct {
		name  string
		reply string
		check func(FilesRecoverJob) bool
	}{
		{
			"done", `{"jobID": "b1/5f", "name": "carol", "exportID": 104, "state": "done", "step": "rebuilding",
			"started": "2026-11-09T10:00:00Z", "finished": "2026-11-09T10:04:00Z", "rebuilt": true}`,
			func(j FilesRecoverJob) bool {
				return j.State == FilesJobDone && j.Rebuilt && j.Finished != nil && j.Error == nil
			},
		},
		{
			"failed", `{"jobID": "b1/5f", "name": "carol", "exportID": 104, "state": "failed", "step": "rebuilding",
			"started": "2026-11-09T10:00:00Z", "finished": "2026-11-09T10:01:00Z",
			"error": {"code": "RecoverFailed", "message": "the lease is held by node02"}}`,
			func(j FilesRecoverJob) bool {
				return j.State == FilesJobFailed && !j.Rebuilt && j.Error != nil &&
					j.Error.Code == FilesErrRecoverFailed && j.Error.Message == "the lease is held by node02"
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server, seen := newFilesJSONServer(t, http.StatusOK, tc.reply)
			job, err := newFilesExportsTestClient(t, server.URL).FilesRecoverJobStatus(context.Background(), "carol", "b1/5f")
			if err != nil {
				t.Fatalf("FilesRecoverJobStatus: %v", err)
			}
			req := (*seen)[0]
			if want := "/minio/admin/files/v1/exports/carol/recover/b1%2F5f"; req.method != http.MethodGet || req.rawPath != want {
				t.Errorf("sent %s %s, want GET %s", req.method, req.rawPath, want)
			}
			if !tc.check(job) {
				t.Errorf("job = %+v", job)
			}
		})
	}
}

// TestFilesRecoverJobStatusErrors verifies that a job id naming no job is
// refused before a request is sent, and that a job the node forgot carries
// the server's code.
func TestFilesRecoverJobStatusErrors(t *testing.T) {
	server, seen := newFilesJSONServer(t, http.StatusNotFound, `{"code": "JobNotFound", "message": "the gateway on node03 restarted"}`)
	client := newFilesExportsTestClient(t, server.URL)

	for _, jobID := range []string{"", ".", ".."} {
		if _, err := client.FilesRecoverJobStatus(context.Background(), "carol", jobID); err == nil {
			t.Errorf("FilesRecoverJobStatus(%q) succeeded", jobID)
		}
	}
	if len(*seen) != 0 {
		t.Fatalf("sent %d requests for refused job ids", len(*seen))
	}

	_, err := client.FilesRecoverJobStatus(context.Background(), "carol", "b1.5f")
	if code := ToErrorResponse(err).Code; code != FilesErrJobNotFound {
		t.Errorf("error code = %q (%v), want %q", code, err, FilesErrJobNotFound)
	}
}
