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

//go:generate go tool msgp -d clearomitted -d "timezone utc" -file $GOFILE

// DistJobType identifies a registered distributed job type. The zero value,
// DistJobTypeUnknown, doubles as the "no filter" sentinel for
// ListDistJobStatuses.
//
// Values are stable across releases: append new types, never renumber
// existing ones — a rolling upgrade can have two server versions
// interpreting the same wire value at once.
type DistJobType uint8

const (
	DistJobTypeUnknown DistJobType = iota
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

// MarshalJSON encodes the job type as its String() form, not a bare integer.
func (t DistJobType) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

// UnmarshalJSON decodes unrecognized values as DistJobTypeUnknown rather
// than erroring, so an older client stays forward-compatible with a newer
// server's job type.
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
	Host    string `json:"host"                    msg:"h"`
	IsLocal bool   `json:"isLocal"                 msg:"il"`
	// Online goes false after enough consecutive status poll failures;
	// the leader then stops assigning this node work and re-queues
	// whatever it had to another node.
	Online           bool `json:"online"                  msg:"on"`
	ConsecutiveFails int  `json:"consecutiveFails"        msg:"cf"`
	// CurrentSet/CurrentBucket identify the work item in progress now;
	// CurrentBucket is empty when idle.
	CurrentSet    int    `json:"currentSet"              msg:"cs"`
	CurrentBucket string `json:"currentBucket,omitempty" msg:"cb"`
	// Cumulative counters across every work item completed so far in this run.
	ItemsDone   int64 `json:"itemsDone"   msg:"id"`
	ItemsFailed int64 `json:"itemsFailed" msg:"if"`
	BytesDone   int64 `json:"bytesDone"   msg:"bd"`
	BytesFailed int64 `json:"bytesFailed" msg:"bf"`
}

// DistJobLeaderStatus is a point-in-time snapshot of a running distributed
// job as seen by the leader node.
type DistJobLeaderStatus struct {
	JobID   string              `json:"jobID"   msg:"id"`
	JobType DistJobType         `json:"jobType" msg:"jt"`
	PoolIdx int                 `json:"poolIdx" msg:"pi"`
	Nodes   []DistJobNodeStatus `json:"nodes"   msg:"n"`
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
