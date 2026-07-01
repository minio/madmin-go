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
	"encoding/json"
	"net/http"
	"net/url"
)

//msgp:tag json
//go:generate msgp -d clearomitted -d "timezone utc" -file $GOFILE

// DistJobType identifies a registered distributed job type.
//
// Values are stable across releases: append new types, never renumber
// existing ones — a rolling upgrade can have two server versions
// interpreting the same wire value at once.
type DistJobType uint8

const (
	// DistJobTypeUnknown is the zero value. No valid request carries it;
	// it doubles as the "no filter" sentinel for ListDistJobStatuses.
	DistJobTypeUnknown DistJobType = iota

	// DistJobTypeDecommission drains one pool that is being removed from
	// the cluster: every version of every object is copied out and the
	// pool is left empty.
	DistJobTypeDecommission
)

// String returns the wire/query-param representation of the job type, e.g.
// "decommission". Matches the ?job= value accepted by the distjob status API.
func (t DistJobType) String() string {
	switch t {
	case DistJobTypeDecommission:
		return "decommission"
	default:
		return "unknown"
	}
}

// MarshalJSON encodes the job type using its String() representation so
// admin API JSON responses carry a human-readable value instead of a bare
// integer.
func (t DistJobType) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

// UnmarshalJSON decodes a job type from its String() representation.
// Unrecognized values decode to DistJobTypeUnknown rather than erroring, so
// a newer server's job type is forward-compatible with an older client.
func (t *DistJobType) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*t = ParseDistJobType(s)
	return nil
}

// ParseDistJobType converts a job type's wire/query-param representation
// back into a DistJobType. Returns DistJobTypeUnknown if s does not match a
// known type.
func ParseDistJobType(s string) DistJobType {
	switch s {
	case DistJobTypeDecommission.String():
		return DistJobTypeDecommission
	default:
		return DistJobTypeUnknown
	}
}

// DistJobNodeStatus is the observable state of one node participating in a
// distributed job, as seen by the leader's poll loop.
type DistJobNodeStatus struct {
	// Host is the node's grid host address (host:port).
	Host string `json:"host"`
	// IsLocal is true only for the leader's own entry.
	IsLocal bool `json:"isLocal"`
	// Online is false once this node has missed enough consecutive status
	// polls that the leader stopped assigning it new work and re-queued
	// whatever it was processing to another node.
	Online bool `json:"online"`
	// ConsecutiveFails counts status poll failures to this node since its
	// last successful poll; it resets to 0 on any successful poll.
	ConsecutiveFails int `json:"consecutiveFails"`
	// CurrentSet and CurrentBucket identify the work item this node is
	// processing right now. CurrentBucket is empty when idle.
	CurrentSet    int    `json:"currentSet"`
	CurrentBucket string `json:"currentBucket,omitempty"`
	// ItemsDone, ItemsFailed, BytesDone, BytesFailed are this node's
	// cumulative counters across every work item completed so far in this
	// job run.
	ItemsDone   int64 `json:"itemsDone"`
	ItemsFailed int64 `json:"itemsFailed"`
	BytesDone   int64 `json:"bytesDone"`
	BytesFailed int64 `json:"bytesFailed"`
}

// DistJobLeaderStatus is a point-in-time snapshot of a running distributed
// job as seen by the leader node.
type DistJobLeaderStatus struct {
	JobID   string              `json:"jobID"`
	JobType DistJobType         `json:"jobType"`
	PoolIdx int                 `json:"poolIdx"`
	Nodes   []DistJobNodeStatus `json:"nodes"`
}

// ListDistJobStatuses returns the current state of all active distributed
// jobs. Pass a jobType other than DistJobTypeUnknown to filter to a
// specific job type. Pass DistJobTypeUnknown to return all active jobs.
func (adm *AdminClient) ListDistJobStatuses(ctx context.Context, jobType DistJobType) ([]DistJobLeaderStatus, error) {
	values := url.Values{}
	if jobType != DistJobTypeUnknown {
		values.Set("job", jobType.String())
	}
	resp, err := adm.executeMethod(ctx, http.MethodGet, requestData{
		relPath:     adminAPIPrefix + "/distjob/status",
		queryValues: values,
	})
	if err != nil {
		return nil, err
	}
	defer closeResponse(resp)
	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}
	var statuses []DistJobLeaderStatus
	if err = json.NewDecoder(resp.Body).Decode(&statuses); err != nil {
		return nil, err
	}
	return statuses, nil
}
