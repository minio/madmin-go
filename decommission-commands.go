//
// Copyright (c) 2015-2024 MinIO, Inc.
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
	"time"
)

// PoolDecommissionInfo currently draining information
type PoolDecommissionInfo struct {
	StartTime time.Time `json:"startTime"`
	// StartSize is the total pool capacity when decommissioning started
	StartSize int64 `json:"startSize"`
	// TotalSize is the total pool capacity (same as StartSize, kept for backward compatibility)
	TotalSize int64 `json:"totalSize"`
	// CurrentSize is the remaining pool free capacity (space not yet drained)
	CurrentSize int64 `json:"currentSize"`

	StartInodes   int64 `json:"startInodes"`
	TotalInodes   int64 `json:"totalInodes"`
	CurrentInodes int64 `json:"currentInodes"`

	Complete bool `json:"complete"`
	Failed   bool `json:"failed"`
	Canceled bool `json:"canceled"`

	ObjectsDecommissioned     int64 `json:"objectsDecommissioned"`
	ObjectsDecommissionFailed int64 `json:"objectsDecommissionedFailed"`
	BytesDone                 int64 `json:"bytesDecommissioned"`
	BytesFailed               int64 `json:"bytesDecommissionedFailed"`

	// Additional metrics (v4+)
	ElapsedSeconds     int64 `json:"elapsedSeconds,omitempty"`
	ItemsUnrecoverable int64 `json:"itemsUnrecoverable,omitempty"`
	BytesUnrecoverable int64 `json:"bytesUnrecoverable,omitempty"`
}

// DecommissionPoolOptions holds optional parameters for DecommissionPool.
type DecommissionPoolOptions struct {
	// SkipUnrecoverable when true skips objects with quorum loss instead of halting.
	SkipUnrecoverable bool
}

// PoolStatus captures current pool status
type PoolStatus struct {
	ID           int                   `json:"id"`
	CmdLine      string                `json:"cmdline"`
	LastUpdate   time.Time             `json:"lastUpdate"`
	Decommission *PoolDecommissionInfo `json:"decommissionInfo,omitempty"`
}

// DecommissionPool - starts moving data from specified pool to all other existing pools.
// Decommissioning if successfully started this function will return `nil`, to check
// for on-going draining cycle use StatusPool.
//
// Deprecated: Use DecommissionPoolWithOptions instead.
func (adm *AdminClient) DecommissionPool(ctx context.Context, pool string) error {
	return adm.DecommissionPoolWithOptions(ctx, pool, DecommissionPoolOptions{})
}

// DecommissionPoolWithOptions - starts moving data from specified pool to all other existing pools.
// Decommissioning if successfully started this function will return `nil`, to check
// for on-going draining cycle use StatusPool.
func (adm *AdminClient) DecommissionPoolWithOptions(ctx context.Context, pool string, opts DecommissionPoolOptions) error {
	values := url.Values{}
	values.Set("pool", pool)
	if opts.SkipUnrecoverable {
		values.Set("skipUnrecoverable", "true")
	}
	resp, err := adm.executeMethod(ctx, http.MethodPost, requestData{
		// POST <endpoint>/<admin-API>/pools/decommission?pool=http://server{1...4}/disk{1...4}
		relPath:     adminAPIPrefix + "/pools/decommission",
		queryValues: values,
	})
	if err != nil {
		return err
	}
	defer closeResponse(resp)
	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}
	return nil
}

// CancelDecommissionPool - cancels an on-going decommissioning process,
// this automatically makes the pool available for writing once canceled.
func (adm *AdminClient) CancelDecommissionPool(ctx context.Context, pool string) error {
	values := url.Values{}
	values.Set("pool", pool)
	resp, err := adm.executeMethod(ctx, http.MethodPost, requestData{
		// POST <endpoint>/<admin-API>/pools/cancel?pool=http://server{1...4}/disk{1...4}
		relPath:     adminAPIPrefix + "/pools/cancel",
		queryValues: values,
	})
	if err != nil {
		return err
	}
	defer closeResponse(resp)
	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}
	return nil
}

// StatusPool return current status about pool, reports any draining activity in progress
// and elapsed time.
func (adm *AdminClient) StatusPool(ctx context.Context, pool string) (PoolStatus, error) {
	values := url.Values{}
	values.Set("pool", pool)
	resp, err := adm.executeMethod(ctx, http.MethodGet, requestData{
		// GET <endpoint>/<admin-API>/pools/status?pool=http://server{1...4}/disk{1...4}
		relPath:     adminAPIPrefix + "/pools/status",
		queryValues: values,
	})
	if err != nil {
		return PoolStatus{}, err
	}
	defer closeResponse(resp)

	if resp.StatusCode != http.StatusOK {
		return PoolStatus{}, httpRespToErrorResponse(resp)
	}

	var info PoolStatus
	if err = json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return PoolStatus{}, err
	}

	return info, nil
}

// ListPoolsStatus returns list of pools currently configured and being used
// on the cluster.
func (adm *AdminClient) ListPoolsStatus(ctx context.Context) ([]PoolStatus, error) {
	resp, err := adm.executeMethod(ctx, http.MethodGet, requestData{
		relPath: adminAPIPrefix + "/pools/list", // GET <endpoint>/<admin-API>/pools/list
	})
	if err != nil {
		return nil, err
	}
	defer closeResponse(resp)
	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}
	var pools []PoolStatus
	if err = json.NewDecoder(resp.Body).Decode(&pools); err != nil {
		return nil, err
	}
	return pools, nil
}
