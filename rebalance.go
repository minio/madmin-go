//
// Copyright (c) 2015-2022 MinIO, Inc.
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
	"io/ioutil"
	"net/http"
	"time"
)

// RebalPoolProgress contains metrics like number of objects, versions, etc rebalanced so far.
type RebalPoolProgress struct {
	NumObjects  uint64        `json:"objects"`
	NumVersions uint64        `json:"versions"`
	Bytes       uint64        `json:"bytes"`
	Bucket      string        `json:"bucket"`
	Object      string        `json:"object"`
	Elapsed     time.Duration `json:"elapsed"`
	ETA         time.Duration `json:"eta"`
}

// RebalancePoolStatus contains metrics of a rebalance operation on a given pool
type RebalancePoolStatus struct {
	ID       int               `json:"id"`                 // Pool index (zero-based)
	Status   string            `json:"status"`             // Active if rebalance is running, empty otherwise
	Used     float64           `json:"used"`               // Percentage used space
	Progress RebalPoolProgress `json:"progress,omitempty"` // is empty when rebalance is not running
}

// RebalanceStatus contains metrics and progress related information on all pools
type RebalanceStatus struct {
	ID        string                // identifies the ongoing rebalance operation by a uuid
	StoppedAt time.Time             `json:"stoppedAt,omitempty"`
	Pools     []RebalancePoolStatus `json:"pools"` // contains all pools, including inactive
}

// RebalanceStart starts a rebalance operation if one isn't in progress already
func (adm *AdminClient) RebalanceStart(ctx context.Context) (id string, err error) {
	// Execute POST on /minio/admin/v3/rebalance/start to start a rebalance operation.
	var resp *http.Response
	resp, err = adm.executeMethod(ctx,
		http.MethodPost,
		requestData{relPath: adminAPIPrefix + "/rebalance/start"})
	defer closeResponse(resp)
	if err != nil {
		return id, err
	}

	if resp.StatusCode != http.StatusOK {
		return id, httpRespToErrorResponse(resp)
	}

	var rebalInfo struct {
		ID string `json:"id"`
	}
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return id, err
	}

	err = json.Unmarshal(respBytes, &rebalInfo)
	if err != nil {
		return id, err
	}

	return rebalInfo.ID, nil
}

func (adm *AdminClient) RebalanceStatus(ctx context.Context) (r RebalanceStatus, err error) {
	// Execute GET on /minio/admin/v3/rebalance/status to get status of an ongoing rebalance operation.
	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{relPath: adminAPIPrefix + "/rebalance/status"})
	defer closeResponse(resp)
	if err != nil {
		return r, err
	}

	if resp.StatusCode != http.StatusOK {
		return r, httpRespToErrorResponse(resp)
	}

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return r, err
	}

	err = json.Unmarshal(respBytes, &r)
	if err != nil {
		return r, err
	}

	return r, nil
}

func (adm *AdminClient) RebalanceStop(ctx context.Context) error {
	// Execute POST on /minio/admin/v3/rebalance/stop to stop an ongoing rebalance operation.
	resp, err := adm.executeMethod(ctx,
		http.MethodPost,
		requestData{relPath: adminAPIPrefix + "/rebalance/stop"})
	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}

	return nil
}
