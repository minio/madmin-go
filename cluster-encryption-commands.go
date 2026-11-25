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
	"net/url"
	"time"
)

// ClusterEncryptionNodeState is the state of the drive key that one node serves.
type ClusterEncryptionNodeState string

// Node states that ClusterEncryptionNode.State reports.
const (
	ClusterEncryptionNodeLoaded  ClusterEncryptionNodeState = "loaded"
	ClusterEncryptionNodeAbsent  ClusterEncryptionNodeState = "absent"
	ClusterEncryptionNodeLoading ClusterEncryptionNodeState = "loading"
	ClusterEncryptionNodeOffline ClusterEncryptionNodeState = "offline"
	ClusterEncryptionNodeError   ClusterEncryptionNodeState = "error"
)

// ClusterEncryptionJobState is the state of the encryption job.
type ClusterEncryptionJobState string

// Job states that ClusterEncryptionJob.State reports.
const (
	ClusterEncryptionJobNone                  ClusterEncryptionJobState = "none"
	ClusterEncryptionJobRunning               ClusterEncryptionJobState = "running"
	ClusterEncryptionJobPaused                ClusterEncryptionJobState = "paused"
	ClusterEncryptionJobCompleted             ClusterEncryptionJobState = "completed"
	ClusterEncryptionJobCompletedWithFailures ClusterEncryptionJobState = "completedWithFailures"
	ClusterEncryptionJobFailed                ClusterEncryptionJobState = "failed"
	ClusterEncryptionJobCanceled              ClusterEncryptionJobState = "canceled"
)

// ClusterEncryptionJobPoolState is the state of the encryption job on one pool.
type ClusterEncryptionJobPoolState string

// Pool states that ClusterEncryptionJobPool.State reports.
const (
	ClusterEncryptionJobPoolPending ClusterEncryptionJobPoolState = "pending"
	ClusterEncryptionJobPoolRunning ClusterEncryptionJobPoolState = "running"
	ClusterEncryptionJobPoolDone    ClusterEncryptionJobPoolState = "done"
	ClusterEncryptionJobPoolSkipped ClusterEncryptionJobPoolState = "skipped"
)

// ClusterEncryptionObjectState is the cluster encryption of one object version.
type ClusterEncryptionObjectState string

// Object states that ClusterEncryptionObjectInfo.State reports.
const (
	ClusterEncryptionObjectEncrypted    ClusterEncryptionObjectState = "encrypted"
	ClusterEncryptionObjectPlaintext    ClusterEncryptionObjectState = "plaintext"
	ClusterEncryptionObjectDeleteMarker ClusterEncryptionObjectState = "deleteMarker"
	// ClusterEncryptionObjectRemote is a version whose data moved to a remote
	// tier, which cluster encryption does not apply to.
	ClusterEncryptionObjectRemote ClusterEncryptionObjectState = "remote"
)

// ClusterEncryptionStatus is the reply of the cluster encryption status and
// change APIs. Cluster encryption is an AIStor extension.
type ClusterEncryptionStatus struct {
	Enabled bool `json:"enabled"`

	// KeyExists reports whether the KMS store holds a drive key.
	KeyExists bool `json:"keyExists"`

	// RootKeyProtected is false while the root key is sealed by hsm:minio:zero,
	// whose key is public.
	RootKeyProtected bool `json:"rootKeyProtected"`

	LatestKeyVersion     uint32    `json:"latestKeyVersion,omitempty"`
	LatestKeyCreatedAt   time.Time `json:"latestKeyCreatedAt,omitzero"`
	KeyVersions          int       `json:"keyVersions,omitempty"`
	KeyVersionsRemaining int       `json:"keyVersionsRemaining,omitempty"`
	StateUpdatedAt       time.Time `json:"stateUpdatedAt,omitzero"`

	// APIVersionReady reports whether the cluster API version supports
	// cluster encryption, which enabling it requires.
	APIVersionReady bool `json:"apiVersionReady"`

	// Nodes holds the drive key state of each node, the answering node first.
	Nodes []ClusterEncryptionNode `json:"nodes"`

	Job ClusterEncryptionJob `json:"job"`

	// JobStopped is set in the disable reply when the disable stopped a running
	// or paused encryption job.
	JobStopped bool `json:"jobStopped,omitempty"`

	// Sets holds the plaintext versions the scanner counted in each erasure set
	// at its last scan.
	Sets []ClusterEncryptionSet `json:"sets"`
}

// ClusterEncryptionNode is the drive key state that one node serves. A node
// that serves something other than the KMS store holds is out of sync.
type ClusterEncryptionNode struct {
	Node               string                     `json:"node"`
	State              ClusterEncryptionNodeState `json:"state"`
	Enabled            bool                       `json:"enabled"`
	LatestKeyVersion   uint32                     `json:"latestKeyVersion,omitempty"`
	LatestKeyCreatedAt time.Time                  `json:"latestKeyCreatedAt,omitzero"`
	StateUpdatedAt     time.Time                  `json:"stateUpdatedAt,omitzero"`
	InSync             bool                       `json:"inSync"`
	Error              string                     `json:"error,omitempty"`
}

// ClusterEncryptionSet holds the versions of an erasure set not encrypted at
// rest, as the scanner of that set counted them at its last completed scan.
type ClusterEncryptionSet struct {
	Pool              int       `json:"pool"`
	Set               int       `json:"set"`
	PlaintextVersions uint64    `json:"plaintextVersions"`
	PlaintextSize     uint64    `json:"plaintextSize"`
	LastPassStartedAt time.Time `json:"lastPassStartedAt,omitzero"`

	// TimedOut reports that the usage cache of the set did not load in time, so
	// its counts are unknown and zero.
	TimedOut bool `json:"timedOut,omitempty"`
}

// ClusterEncryptionJob is the record of the encryption job, which encrypts the
// data written while cluster encryption was disabled.
type ClusterEncryptionJob struct {
	ID       string                    `json:"id"`
	State    ClusterEncryptionJobState `json:"state"`
	Error    string                    `json:"error,omitempty"`
	Started  time.Time                 `json:"started,omitzero"`
	Updated  time.Time                 `json:"updated,omitzero"`
	Finished time.Time                 `json:"finished,omitzero"`

	Pools []ClusterEncryptionJobPool `json:"pools"`

	// Totals sums the counts of Pools.
	Totals ClusterEncryptionJobCounts `json:"totals"`

	// DataGen counts the writes that changed encrypted data or annotation
	// records.
	DataGen uint64 `json:"dataGen,omitempty"`
}

// ClusterEncryptionJobPool is the state of the encryption job on one pool.
type ClusterEncryptionJobPool struct {
	// Pool is the command line of the pool, which a pool removal does not change.
	Pool  string                        `json:"pool"`
	State ClusterEncryptionJobPoolState `json:"state"`

	// SkipReason is why the job passed over a skipped pool.
	SkipReason string `json:"skipped,omitempty"`

	// Current is the bucket the job works on while the pool runs.
	Current string `json:"current,omitempty"`

	Counts ClusterEncryptionJobCounts `json:"counts"`
}

// ClusterEncryptionJobCounts counts versions. A version is encrypted when its
// data was rewritten, restaged when only its annotation records were, and
// skipped when it needed nothing or was left alone for a reason.
type ClusterEncryptionJobCounts struct {
	Encrypted           int64 `json:"encrypted"`
	AnnotationsRestaged int64 `json:"annotationsRestaged"`
	Skipped             int64 `json:"skipped"`
	Failed              int64 `json:"failed"`
	BytesEncrypted      int64 `json:"bytesEncrypted"`
}

// ClusterEncryptionObjectInfo is the cluster encryption of one object version.
// The key fields are set only when the version is encrypted. Cluster encryption
// is an AIStor extension.
type ClusterEncryptionObjectInfo struct {
	Bucket     string                       `json:"bucket"`
	Object     string                       `json:"object"`
	VersionID  string                       `json:"versionId"`
	State      ClusterEncryptionObjectState `json:"state"`
	KeyVersion uint32                       `json:"keyVersion,omitempty"`

	// KeyCreatedAt is the creation date of the key version that wraps the object
	// key. It is omitted when the server cannot look up that key version, which
	// can happen for an encrypted version.
	KeyCreatedAt time.Time `json:"keyCreatedAt,omitzero"`
	Cipher       string    `json:"cipher,omitempty"`
}

// ClusterEncryptionEnable enables cluster encryption of new writes on the whole
// cluster, creating the drive key if there is none, and returns the status. A
// second call changes nothing.
func (adm *AdminClient) ClusterEncryptionEnable(ctx context.Context) (ClusterEncryptionStatus, error) {
	var st ClusterEncryptionStatus
	err := adm.clusterEncryptionCall(ctx, http.MethodPost, "/cluster/encryption/enable", nil, nil, &st)
	return st, err
}

// ClusterEncryptionDisable makes new writes plaintext on the whole cluster and
// stops the encryption job, and returns the status. Encrypted data stays
// readable.
func (adm *AdminClient) ClusterEncryptionDisable(ctx context.Context) (ClusterEncryptionStatus, error) {
	var st ClusterEncryptionStatus
	err := adm.clusterEncryptionCall(ctx, http.MethodPost, "/cluster/encryption/disable", nil, nil, &st)
	return st, err
}

// ClusterEncryptionRotateKey adds a version to the drive key, which wraps the
// object keys of new writes from then on, and returns the status.
func (adm *AdminClient) ClusterEncryptionRotateKey(ctx context.Context) (ClusterEncryptionStatus, error) {
	var st ClusterEncryptionStatus
	err := adm.clusterEncryptionCall(ctx, http.MethodPost, "/cluster/encryption/rotate", nil, clusterEncryptionNoRepeat, &st)
	return st, err
}

// ClusterEncryptionStatus returns the cluster encryption status, with the drive
// key state of every node.
func (adm *AdminClient) ClusterEncryptionStatus(ctx context.Context) (ClusterEncryptionStatus, error) {
	var st ClusterEncryptionStatus
	err := adm.clusterEncryptionCall(ctx, http.MethodGet, "/cluster/encryption/status", nil, nil, &st)
	return st, err
}

// ClusterEncryptionObject returns the cluster encryption of one version of an
// object, read from the metadata of the version, not its data. An empty
// versionID selects the latest version.
func (adm *AdminClient) ClusterEncryptionObject(ctx context.Context, bucket, object, versionID string) (ClusterEncryptionObjectInfo, error) {
	query := url.Values{}
	query.Set("bucket", bucket)
	query.Set("object", object)
	if versionID != "" {
		query.Set("versionId", versionID)
	}
	var info ClusterEncryptionObjectInfo
	err := adm.clusterEncryptionCall(ctx, http.MethodGet, "/cluster/encryption/object", query, nil, &info)
	return info, err
}

// ClusterEncryptionJobStart starts the encryption job and returns its record. It
// fails while cluster encryption is disabled, and while a job runs or is paused.
func (adm *AdminClient) ClusterEncryptionJobStart(ctx context.Context) (ClusterEncryptionJob, error) {
	var job ClusterEncryptionJob
	err := adm.clusterEncryptionCall(ctx, http.MethodPost, "/cluster/encryption/job/start", nil, clusterEncryptionNoRepeat, &job)
	return job, err
}

// ClusterEncryptionJobStop stops the running or paused encryption job, which
// ends in the state canceled, and returns its record. A canceled job is not
// resumed: a later start begins a new one. It fails when no job runs or is
// paused.
func (adm *AdminClient) ClusterEncryptionJobStop(ctx context.Context) (ClusterEncryptionJob, error) {
	var job ClusterEncryptionJob
	err := adm.clusterEncryptionCall(ctx, http.MethodPost, "/cluster/encryption/job/stop", nil, clusterEncryptionNoRepeat, &job)
	return job, err
}

// ClusterEncryptionJobPause pauses the running encryption job, which resumes
// from the buckets it finished, and returns its record.
func (adm *AdminClient) ClusterEncryptionJobPause(ctx context.Context) (ClusterEncryptionJob, error) {
	var job ClusterEncryptionJob
	err := adm.clusterEncryptionCall(ctx, http.MethodPost, "/cluster/encryption/job/pause", nil, clusterEncryptionNoRepeat, &job)
	return job, err
}

// ClusterEncryptionJobResume resumes a paused or failed encryption job and
// returns its record.
func (adm *AdminClient) ClusterEncryptionJobResume(ctx context.Context) (ClusterEncryptionJob, error) {
	var job ClusterEncryptionJob
	err := adm.clusterEncryptionCall(ctx, http.MethodPost, "/cluster/encryption/job/resume", nil, clusterEncryptionNoRepeat, &job)
	return job, err
}

// ClusterEncryptionJobStatus returns the record of the encryption job. A cluster
// that never started one reports the state ClusterEncryptionJobNone.
func (adm *AdminClient) ClusterEncryptionJobStatus(ctx context.Context) (ClusterEncryptionJob, error) {
	var job ClusterEncryptionJob
	err := adm.clusterEncryptionCall(ctx, http.MethodGet, "/cluster/encryption/job/status", nil, nil, &job)
	return job, err
}

// clusterEncryptionNoRepeat is the retry hook of a call that changes state. The
// server may act before it answers 503, so a repeat could apply the change
// twice.
func clusterEncryptionNoRepeat(*http.Response, []byte, error) bool { return false }

// clusterEncryptionCall sends one call with the query values and decodes its
// reply. A nil retry leaves the repeats to executeMethod.
func (adm *AdminClient) clusterEncryptionCall(ctx context.Context, method, path string, query url.Values,
	retry func(*http.Response, []byte, error) bool, reply any,
) error {
	resp, err := adm.executeMethod(ctx, method, requestData{relPath: adminAPIPrefix + path, queryValues: query, retry: retry})
	if err != nil {
		return err
	}
	defer closeResponse(resp)
	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}
	return json.NewDecoder(resp.Body).Decode(reply)
}
