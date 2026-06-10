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

// DistJobNodeStatus is the observable state of one node participating in a
// distributed job, as reported by the leader.
type DistJobNodeStatus struct {
	Host             string `json:"host"`
	IsLocal          bool   `json:"isLocal"`
	Online           bool   `json:"online"`
	ConsecutiveFails int    `json:"consecutiveFails"`
	AssignedSets     []int  `json:"assignedSets"`
	ItemsDone        int64  `json:"itemsDone"`
	ItemsFailed      int64  `json:"itemsFailed"`
	BytesDone        int64  `json:"bytesDone"`
	BytesFailed      int64  `json:"bytesFailed"`
}

// DistJobLeaderStatus is a point-in-time snapshot of a running distributed
// job as seen by the leader node.
type DistJobLeaderStatus struct {
	JobName       string              `json:"jobName"`
	PoolIdx       int                 `json:"poolIdx"`
	CurrentBucket string              `json:"currentBucket"`
	Generation    uint64              `json:"generation"`
	Nodes         []DistJobNodeStatus `json:"nodes"`
}

// ListDistJobStatuses returns the current state of all active distributed
// jobs. Pass a non-empty jobName to filter to a specific job type
// (e.g. "decommission"). Pass empty string to return all active jobs.
func (adm *AdminClient) ListDistJobStatuses(ctx context.Context, jobName string) ([]DistJobLeaderStatus, error) {
	values := url.Values{}
	if jobName != "" {
		values.Set("job", jobName)
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
