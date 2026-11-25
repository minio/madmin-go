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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

// clusterEncryptionStatusJSON is a status reply of the server: node2 missed a
// key reload, node3 is offline, and a job runs.
const clusterEncryptionStatusJSON = `{
  "enabled": true,
  "keyExists": true,
  "rootKeyProtected": false,
  "latestKeyVersion": 2,
  "latestKeyCreatedAt": "2026-11-20T14:03:00Z",
  "keyVersions": 2,
  "keyVersionsRemaining": 254,
  "stateUpdatedAt": "2026-11-20T10:00:00Z",
  "apiVersionReady": true,
  "nodes": [
    {"node": "node1:9000", "state": "loaded", "enabled": true, "latestKeyVersion": 2, "latestKeyCreatedAt": "2026-11-20T14:03:00Z", "stateUpdatedAt": "2026-11-20T10:00:00Z", "inSync": true},
    {"node": "node2:9000", "state": "loaded", "enabled": true, "latestKeyVersion": 1, "latestKeyCreatedAt": "2026-11-18T09:00:00Z", "stateUpdatedAt": "2026-11-20T10:00:00Z", "inSync": false},
    {"node": "node3:9000", "state": "offline", "enabled": false, "inSync": false, "error": "connection refused"}
  ],
  "job": ` + clusterEncryptionJobJSON + `,
  "sets": [
    {"pool": 0, "set": 0, "plaintextVersions": 0, "plaintextSize": 0, "lastPassStartedAt": "2026-11-20T09:00:00Z"},
    {"pool": 1, "set": 0, "plaintextVersions": 12, "plaintextSize": 4096, "lastPassStartedAt": "2026-11-20T09:30:00Z"}
  ]
}`

// clusterEncryptionJobJSON is the record of a running encryption job.
const clusterEncryptionJobJSON = `{
  "id": "job-1",
  "state": "running",
  "started": "2026-11-21T08:00:00Z",
  "updated": "2026-11-21T08:05:00Z",
  "pools": [
    {"pool": "http://node{1...4}/data{1...4}", "state": "done", "counts": {"encrypted": 10, "annotationsRestaged": 2, "skipped": 1, "failed": 0, "bytesEncrypted": 4096}},
    {"pool": "http://node{5...8}/data{1...4}", "state": "running", "current": "photos", "counts": {"encrypted": 3, "annotationsRestaged": 0, "skipped": 0, "failed": 0, "bytesEncrypted": 512}}
  ],
  "totals": {"encrypted": 13, "annotationsRestaged": 2, "skipped": 1, "failed": 0, "bytesEncrypted": 4608},
  "dataGen": 7
}`

func TestClusterEncryptionStatusWire(t *testing.T) {
	var st ClusterEncryptionStatus
	if err := json.Unmarshal([]byte(clusterEncryptionStatusJSON), &st); err != nil {
		t.Fatal(err)
	}
	if !st.Enabled || !st.KeyExists || st.RootKeyProtected || !st.APIVersionReady {
		t.Errorf("flags = enabled %v, keyExists %v, rootKeyProtected %v, apiVersionReady %v",
			st.Enabled, st.KeyExists, st.RootKeyProtected, st.APIVersionReady)
	}
	if st.LatestKeyVersion != 2 || st.KeyVersions != 2 || st.KeyVersionsRemaining != 254 {
		t.Errorf("key versions = %d latest, %d total, %d remaining", st.LatestKeyVersion, st.KeyVersions, st.KeyVersionsRemaining)
	}
	keyAt := time.Date(2026, 11, 20, 14, 3, 0, 0, time.UTC)
	if !st.LatestKeyCreatedAt.Equal(keyAt) {
		t.Errorf("LatestKeyCreatedAt = %v, want %v", st.LatestKeyCreatedAt, keyAt)
	}

	if len(st.Nodes) != 3 {
		t.Fatalf("len(Nodes) = %d, want 3", len(st.Nodes))
	}
	if n := st.Nodes[0]; !n.LatestKeyCreatedAt.Equal(keyAt) {
		t.Errorf("Nodes[0].LatestKeyCreatedAt = %v, want %v", n.LatestKeyCreatedAt, keyAt)
	}
	if n := st.Nodes[1]; n.Node != "node2:9000" || n.State != ClusterEncryptionNodeLoaded || n.InSync || n.LatestKeyVersion != 1 ||
		!n.LatestKeyCreatedAt.Equal(time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("Nodes[1] = %+v, want node2 loaded at version 1 and out of sync", n)
	}
	if n := st.Nodes[2]; n.State != ClusterEncryptionNodeOffline || n.Error != "connection refused" {
		t.Errorf("Nodes[2] = %+v, want offline with its error", n)
	}

	if st.Job.ID != "job-1" || st.Job.State != ClusterEncryptionJobRunning || st.Job.DataGen != 7 {
		t.Errorf("Job = %+v, want job-1 running at dataGen 7", st.Job)
	}
	if len(st.Job.Pools) != 2 {
		t.Fatalf("len(Job.Pools) = %d, want 2", len(st.Job.Pools))
	}
	if p := st.Job.Pools[0]; p.State != ClusterEncryptionJobPoolDone || p.Counts.BytesEncrypted != 4096 || p.Counts.AnnotationsRestaged != 2 {
		t.Errorf("Job.Pools[0] = %+v, want done with its counts", p)
	}
	if p := st.Job.Pools[1]; p.State != ClusterEncryptionJobPoolRunning || p.Current != "photos" {
		t.Errorf("Job.Pools[1] = %+v, want running on bucket photos", p)
	}
	if st.Job.Totals.Encrypted != 13 || st.Job.Totals.BytesEncrypted != 4608 {
		t.Errorf("Job.Totals = %+v, want 13 encrypted and 4608 bytes", st.Job.Totals)
	}

	if len(st.Sets) != 2 {
		t.Fatalf("len(Sets) = %d, want 2", len(st.Sets))
	}
	want := time.Date(2026, 11, 20, 9, 30, 0, 0, time.UTC)
	if s := st.Sets[1]; s.Pool != 1 || s.PlaintextVersions != 12 || s.PlaintextSize != 4096 || !s.LastPassStartedAt.Equal(want) {
		t.Errorf("Sets[1] = %+v, want pool 1 with 12 plaintext versions", s)
	}
}

func TestClusterEncryptionWireValues(t *testing.T) {
	tests := []struct {
		got  string
		want string
	}{
		{string(ClusterEncryptionNodeLoaded), "loaded"},
		{string(ClusterEncryptionNodeAbsent), "absent"},
		{string(ClusterEncryptionNodeLoading), "loading"},
		{string(ClusterEncryptionNodeOffline), "offline"},
		{string(ClusterEncryptionNodeError), "error"},
		{string(ClusterEncryptionJobNone), "none"},
		{string(ClusterEncryptionJobRunning), "running"},
		{string(ClusterEncryptionJobPaused), "paused"},
		{string(ClusterEncryptionJobCompleted), "completed"},
		{string(ClusterEncryptionJobCompletedWithFailures), "completedWithFailures"},
		{string(ClusterEncryptionJobFailed), "failed"},
		{string(ClusterEncryptionJobCanceled), "canceled"},
		{string(ClusterEncryptionJobPoolPending), "pending"},
		{string(ClusterEncryptionJobPoolRunning), "running"},
		{string(ClusterEncryptionJobPoolDone), "done"},
		{string(ClusterEncryptionJobPoolSkipped), "skipped"},
		{string(ClusterEncryptionObjectEncrypted), "encrypted"},
		{string(ClusterEncryptionObjectPlaintext), "plaintext"},
		{string(ClusterEncryptionObjectDeleteMarker), "deleteMarker"},
		{string(ClusterEncryptionObjectRemote), "remote"},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("state = %q, want %q", tc.got, tc.want)
		}
	}
}

func TestClusterEncryptionStatusCalls(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		call   func(*AdminClient) (ClusterEncryptionStatus, error)
	}{
		{"enable", http.MethodPost, "/cluster/encryption/enable", func(c *AdminClient) (ClusterEncryptionStatus, error) {
			return c.ClusterEncryptionEnable(context.Background())
		}},
		{"disable", http.MethodPost, "/cluster/encryption/disable", func(c *AdminClient) (ClusterEncryptionStatus, error) {
			return c.ClusterEncryptionDisable(context.Background())
		}},
		{"rotate", http.MethodPost, "/cluster/encryption/rotate", func(c *AdminClient) (ClusterEncryptionStatus, error) {
			return c.ClusterEncryptionRotateKey(context.Background())
		}},
		{"status", http.MethodGet, "/cluster/encryption/status", func(c *AdminClient) (ClusterEncryptionStatus, error) {
			return c.ClusterEncryptionStatus(context.Background())
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := newClusterEncryptionServer(t, tc.method, "/minio/admin/v4"+tc.path, nil, clusterEncryptionStatusJSON)
			defer server.Close()
			client := newClusterEncryptionClient(t, server.URL)

			st, err := tc.call(client)
			if err != nil {
				t.Fatal(err)
			}
			if !st.Enabled || len(st.Nodes) != 3 || st.Job.ID != "job-1" {
				t.Errorf("status = %+v, want the reply of the server", st)
			}
		})
	}
}

func TestClusterEncryptionJobCalls(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		call   func(*AdminClient) (ClusterEncryptionJob, error)
	}{
		{"job start", http.MethodPost, "/cluster/encryption/job/start", func(c *AdminClient) (ClusterEncryptionJob, error) {
			return c.ClusterEncryptionJobStart(context.Background())
		}},
		{"job pause", http.MethodPost, "/cluster/encryption/job/pause", func(c *AdminClient) (ClusterEncryptionJob, error) {
			return c.ClusterEncryptionJobPause(context.Background())
		}},
		{"job resume", http.MethodPost, "/cluster/encryption/job/resume", func(c *AdminClient) (ClusterEncryptionJob, error) {
			return c.ClusterEncryptionJobResume(context.Background())
		}},
		{"job status", http.MethodGet, "/cluster/encryption/job/status", func(c *AdminClient) (ClusterEncryptionJob, error) {
			return c.ClusterEncryptionJobStatus(context.Background())
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := newClusterEncryptionServer(t, tc.method, "/minio/admin/v4"+tc.path, nil, clusterEncryptionJobJSON)
			defer server.Close()
			client := newClusterEncryptionClient(t, server.URL)

			job, err := tc.call(client)
			if err != nil {
				t.Fatal(err)
			}
			if job.ID != "job-1" || job.State != ClusterEncryptionJobRunning || len(job.Pools) != 2 {
				t.Errorf("job = %+v, want the reply of the server", job)
			}
		})
	}
}

// A stop is a POST to its own route, and its reply is the canceled record.
func TestClusterEncryptionJobStop(t *testing.T) {
	const canceled = `{"id":"job-1","state":"canceled","finished":"2026-11-21T09:00:00Z","pools":[],"dataGen":8}`
	server := newClusterEncryptionServer(t, http.MethodPost, "/minio/admin/v4/cluster/encryption/job/stop", nil, canceled)
	defer server.Close()
	client := newClusterEncryptionClient(t, server.URL)

	job, err := client.ClusterEncryptionJobStop(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if job.ID != "job-1" || job.State != ClusterEncryptionJobCanceled || job.Finished.IsZero() || job.DataGen != 8 {
		t.Errorf("job = %+v, want job-1 canceled and finished at dataGen 8", job)
	}
}

// The disable reply reports a stopped job with jobStopped, and a reply without
// the field, such as a status, reads it as false.
func TestClusterEncryptionDisableJobStopped(t *testing.T) {
	const stopped = `{"enabled":false,"keyExists":true,"rootKeyProtected":true,"apiVersionReady":true,"jobStopped":true}`
	server := newClusterEncryptionServer(t, http.MethodPost, "/minio/admin/v4/cluster/encryption/disable", nil, stopped)
	defer server.Close()
	client := newClusterEncryptionClient(t, server.URL)

	st, err := client.ClusterEncryptionDisable(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Enabled || !st.JobStopped {
		t.Errorf("status = enabled %v, jobStopped %v, want disabled with jobStopped", st.Enabled, st.JobStopped)
	}

	var plain ClusterEncryptionStatus
	if err := json.Unmarshal([]byte(clusterEncryptionStatusJSON), &plain); err != nil {
		t.Fatal(err)
	}
	if plain.JobStopped {
		t.Error("JobStopped = true for a reply without jobStopped, want false")
	}
	out, err := json.Marshal(plain)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "jobStopped") {
		t.Errorf("marshaled status %s carries jobStopped, want it omitted when false", out)
	}
}

// The object call sends the bucket and the object name, and the version only
// when one is named, then decodes the state of the version.
func TestClusterEncryptionObject(t *testing.T) {
	const versionID = "3f2a9c1e-7b4d-4e8a-9c0f-1d2e3f4a5b6c"
	keyAt := time.Date(2026, 11, 20, 14, 3, 0, 0, time.UTC)
	tests := []struct {
		name      string
		versionID string
		query     url.Values
		reply     string
		want      ClusterEncryptionObjectInfo
	}{
		{
			name:      "encrypted version",
			versionID: versionID,
			query:     url.Values{"bucket": {"photos"}, "object": {"albums/cat 1.jpg"}, "versionId": {versionID}},
			reply:     `{"bucket":"photos","object":"albums/cat 1.jpg","versionId":"` + versionID + `","state":"encrypted","keyVersion":2,"keyCreatedAt":"2026-11-20T14:03:00Z","cipher":"AES-128-CTR"}`,
			want: ClusterEncryptionObjectInfo{
				Bucket: "photos", Object: "albums/cat 1.jpg", VersionID: versionID,
				State: ClusterEncryptionObjectEncrypted, KeyVersion: 2, KeyCreatedAt: keyAt, Cipher: "AES-128-CTR",
			},
		},
		{
			name:  "latest version without a version id",
			query: url.Values{"bucket": {"photos"}, "object": {"albums/cat 1.jpg"}},
			reply: `{"bucket":"photos","object":"albums/cat 1.jpg","versionId":"` + versionID + `","state":"plaintext"}`,
			want:  ClusterEncryptionObjectInfo{Bucket: "photos", Object: "albums/cat 1.jpg", VersionID: versionID, State: ClusterEncryptionObjectPlaintext},
		},
		{
			name:      "delete marker",
			versionID: versionID,
			query:     url.Values{"bucket": {"photos"}, "object": {"albums/cat 1.jpg"}, "versionId": {versionID}},
			reply:     `{"bucket":"photos","object":"albums/cat 1.jpg","versionId":"` + versionID + `","state":"deleteMarker"}`,
			want:      ClusterEncryptionObjectInfo{Bucket: "photos", Object: "albums/cat 1.jpg", VersionID: versionID, State: ClusterEncryptionObjectDeleteMarker},
		},
		{
			name:      "remote version",
			versionID: versionID,
			query:     url.Values{"bucket": {"photos"}, "object": {"albums/cat 1.jpg"}, "versionId": {versionID}},
			reply:     `{"bucket":"photos","object":"albums/cat 1.jpg","versionId":"` + versionID + `","state":"remote"}`,
			want:      ClusterEncryptionObjectInfo{Bucket: "photos", Object: "albums/cat 1.jpg", VersionID: versionID, State: ClusterEncryptionObjectRemote},
		},
		{
			name:      "encrypted version without a key creation date",
			versionID: versionID,
			query:     url.Values{"bucket": {"photos"}, "object": {"albums/cat 1.jpg"}, "versionId": {versionID}},
			reply:     `{"bucket":"photos","object":"albums/cat 1.jpg","versionId":"` + versionID + `","state":"encrypted","keyVersion":2,"cipher":"AES-128-CTR"}`,
			want: ClusterEncryptionObjectInfo{
				Bucket: "photos", Object: "albums/cat 1.jpg", VersionID: versionID,
				State: ClusterEncryptionObjectEncrypted, KeyVersion: 2, Cipher: "AES-128-CTR",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := newClusterEncryptionServer(t, http.MethodGet, "/minio/admin/v4/cluster/encryption/object", tc.query, tc.reply)
			defer server.Close()
			client := newClusterEncryptionClient(t, server.URL)

			info, err := client.ClusterEncryptionObject(context.Background(), "photos", "albums/cat 1.jpg", tc.versionID)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(info, tc.want) {
				t.Errorf("info = %+v, want %+v", info, tc.want)
			}
		})
	}
}

// A version or object that does not exist is refused with the code of the server.
func TestClusterEncryptionObjectNotFound(t *testing.T) {
	const noSuchKey = `{"Code":"NoSuchKey","Message":"The specified key does not exist."}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(noSuchKey))
	}))
	defer server.Close()
	client := newClusterEncryptionClient(t, server.URL)

	_, err := client.ClusterEncryptionObject(context.Background(), "photos", "missing.jpg", "")
	if err == nil {
		t.Fatal("ClusterEncryptionObject returned no error for a missing object")
	}
	if code := ToErrorResponse(err).Code; code != "NoSuchKey" {
		t.Errorf("ClusterEncryptionObject code = %q, want NoSuchKey", code)
	}
}

// A set whose usage cache timed out is reported with TimedOut and no counts,
// and a counted set leaves TimedOut unset.
func TestClusterEncryptionSetTimedOutWire(t *testing.T) {
	var sets []ClusterEncryptionSet
	if err := json.Unmarshal([]byte(`[{"pool":2,"set":1,"timedOut":true},{"pool":0,"set":0,"plaintextVersions":3,"plaintextSize":512}]`), &sets); err != nil {
		t.Fatal(err)
	}
	if len(sets) != 2 {
		t.Fatalf("len(sets) = %d, want 2", len(sets))
	}
	if s := sets[0]; !s.TimedOut || s.Pool != 2 || s.Set != 1 || s.PlaintextVersions != 0 || s.PlaintextSize != 0 {
		t.Errorf("sets[0] = %+v, want pool 2 set 1 timed out with no counts", s)
	}
	if s := sets[1]; s.TimedOut || s.PlaintextVersions != 3 || s.PlaintextSize != 512 {
		t.Errorf("sets[1] = %+v, want 3 versions and 512 bytes, not timed out", s)
	}
}

// A 503 from a state-changing call is not repeated, because the server may have
// acted before it answered. The LockTimeout code is what the server sends when
// the lock for the change is busy.
func TestClusterEncryptionStateChangeIsNotRepeated(t *testing.T) {
	setMaxRetry(t, 3)
	for _, tc := range []struct {
		name string
		call func(*AdminClient) error
	}{
		{"rotate", func(c *AdminClient) error {
			_, err := c.ClusterEncryptionRotateKey(context.Background())
			return err
		}},
		{"job start", func(c *AdminClient) error {
			_, err := c.ClusterEncryptionJobStart(context.Background())
			return err
		}},
		{"job stop", func(c *AdminClient) error {
			_, err := c.ClusterEncryptionJobStop(context.Background())
			return err
		}},
		{"job pause", func(c *AdminClient) error {
			_, err := c.ClusterEncryptionJobPause(context.Background())
			return err
		}},
		{"job resume", func(c *AdminClient) error {
			_, err := c.ClusterEncryptionJobResume(context.Background())
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server, attempts := countingServer(t, http.StatusServiceUnavailable, `{"Code":"LockTimeout"}`)
			client := newClusterEncryptionClient(t, server.URL)

			err := tc.call(client)
			if code := ToErrorResponse(err).Code; code != "LockTimeout" {
				t.Errorf("code = %q, want LockTimeout (error %v)", code, err)
			}
			if n := attempts.Load(); n != 1 {
				t.Errorf("the server received %d requests, want 1", n)
			}
		})
	}
}

// An idempotent call repeats a 503 up to MaxRetry times.
func TestClusterEncryptionStatusRepeatsUnavailable(t *testing.T) {
	setMaxRetry(t, 2)
	server, attempts := countingServer(t, http.StatusServiceUnavailable, `{"Code":"LockTimeout"}`)
	client := newClusterEncryptionClient(t, server.URL)

	if _, err := client.ClusterEncryptionStatus(context.Background()); err == nil {
		t.Fatal("ClusterEncryptionStatus returned no error for a 503")
	}
	if n := attempts.Load(); n != 2 {
		t.Errorf("the server received %d requests, want 2", n)
	}
}

func TestClusterEncryptionCallError(t *testing.T) {
	const notEnabled = `{"Code":"XMinioAdminClusterEncryptionNotEnabled","Message":"Cluster encryption is not enabled."}`
	for _, tc := range []struct {
		name string
		body string
		// code is the error code: the server's, or the status line when the body
		// does not decode.
		code string
	}{
		{name: "no_body", body: "", code: "409 Conflict"},
		{name: "json_body", body: notEnabled, code: "XMinioAdminClusterEncryptionNotEnabled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			client := newClusterEncryptionClient(t, server.URL)

			_, err := client.ClusterEncryptionJobStart(context.Background())
			if err == nil {
				t.Fatal("ClusterEncryptionJobStart returned no error for a conflict")
			}
			if code := ToErrorResponse(err).Code; code != tc.code {
				t.Errorf("ClusterEncryptionJobStart code = %q, want %q", code, tc.code)
			}

			_, err = client.ClusterEncryptionStatus(context.Background())
			if err == nil {
				t.Fatal("ClusterEncryptionStatus returned no error for a conflict")
			}
			if code := ToErrorResponse(err).Code; code != tc.code {
				t.Errorf("ClusterEncryptionStatus code = %q, want %q", code, tc.code)
			}
		})
	}
}

func TestDataUsageAtRestPlaintextWire(t *testing.T) {
	var usage DataUsageInfo
	if err := json.Unmarshal([]byte(`{"objectsTotalAtRestPlaintext":3,"objectsTotalAtRestPlaintextSize":4096}`), &usage); err != nil {
		t.Fatal(err)
	}
	if usage.ObjectsTotalAtRestPlaintext != 3 || usage.ObjectsTotalAtRestPlaintextSize != 4096 {
		t.Errorf("DataUsageInfo at-rest plaintext = %d versions, %d bytes; want 3 and 4096",
			usage.ObjectsTotalAtRestPlaintext, usage.ObjectsTotalAtRestPlaintextSize)
	}

	var bucket BucketUsageInfo
	if err := json.Unmarshal([]byte(`{"atRestPlaintextCount":2,"atRestPlaintextSize":512}`), &bucket); err != nil {
		t.Fatal(err)
	}
	if bucket.AtRestPlaintextCount != 2 || bucket.AtRestPlaintextSize != 512 {
		t.Errorf("BucketUsageInfo at-rest plaintext = %d versions, %d bytes; want 2 and 512",
			bucket.AtRestPlaintextCount, bucket.AtRestPlaintextSize)
	}
}

// newClusterEncryptionServer replies reply to one call, and reports a request
// whose method, path or query values differ from the ones the call must send.
// A nil query means the call sends none.
func newClusterEncryptionServer(t *testing.T, method, path string, query url.Values, reply string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method || r.URL.Path != path {
			t.Errorf("request = %s %s, want %s %s", r.Method, r.URL.Path, method, path)
		}
		if got := r.URL.Query(); len(got) != len(query) || (len(query) > 0 && !reflect.DeepEqual(got, query)) {
			t.Errorf("query = %v, want %v", got, query)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(reply))
	}))
}

func newClusterEncryptionClient(t *testing.T, url string) *AdminClient {
	t.Helper()
	client, err := New(strings.TrimPrefix(url, "http://"), "access", "secret", false)
	if err != nil {
		t.Fatal(err)
	}
	return client
}
