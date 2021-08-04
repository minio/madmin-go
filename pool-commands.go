//
// MinIO Object Storage (c) 2021 MinIO, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package madmin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"
)

// Pool represents pool specific information
type Pool struct {
	SetCount     int    `json:"setCount"`
	DrivesPerSet int    `json:"drivesPerSet"`
	CmdLine      string `json:"cmdline"`
}

// PoolDrainInfo currently draining information
type PoolDrainInfo struct {
	StartTime   time.Time `json:"startTime"`
	StartSize   int64     `json:"startSize"`
	Duration    int64     `json:"duration"`
	CurrentSize int64     `json:"currentSize"`
	Complete    bool      `json:"complete"`
	Failed      bool      `json:"failed"`
}

// PoolInfo captures pool info
type PoolInfo struct {
	ID         int            `json:"id"`
	CmdLine    string         `json:"cmdline"`
	LastUpdate time.Time      `json:"lastUpdate"`
	Drain      *PoolDrainInfo `json:"drainInfo,omitempty"`
	Suspend    bool           `json:"suspend"`
}

// ResumePool - resume(allow) writes on previously suspended pool.
func (adm *AdminClient) ResumePool(ctx context.Context, pool string) error {
	values := url.Values{}
	values.Set("pool", pool)
	resp, err := adm.executeMethod(ctx, http.MethodPost, requestData{
		relPath:     adminAPIPrefix + "/pools/suspend", // POST <endpoint>/<admin-API>/pools/resume?pool=http://server{1...4}/disk{1...4}
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

// SuspendPool - suspend(disallow) writes on a pool.
func (adm *AdminClient) SuspendPool(ctx context.Context, pool string) error {
	values := url.Values{}
	values.Set("pool", pool)
	resp, err := adm.executeMethod(ctx, http.MethodPost, requestData{
		relPath:     adminAPIPrefix + "/pools/suspend", // POST <endpoint>/<admin-API>/pools/suspend?pool=http://server{1...4}/disk{1...4}
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

// DrainPool - starts moving data from specified pool to all other existing pools.
// Draining if successfully started this function will return `nil`, to check
// for on-going draining cycle use InfoPool.
func (adm *AdminClient) DrainPool(ctx context.Context, pool string) error {
	values := url.Values{}
	values.Set("pool", pool)
	resp, err := adm.executeMethod(ctx, http.MethodPost, requestData{
		relPath:     adminAPIPrefix + "/pools/drain", // POST <endpoint>/<admin-API>/pools/drain?pool=http://server{1...4}/disk{1...4}
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

// InfoPool return current status about pool, reports any draining activity in progress
// and elasped time.
func (adm *AdminClient) InfoPool(ctx context.Context, pool string) (PoolInfo, error) {
	values := url.Values{}
	values.Set("pool", pool)
	resp, err := adm.executeMethod(ctx, http.MethodPost, requestData{
		relPath:     adminAPIPrefix + "/pools/info", // GET <endpoint>/<admin-API>/pools/info?pool=http://server{1...4}/disk{1...4}
		queryValues: values,
	})
	if err != nil {
		return PoolInfo{}, err
	}
	defer closeResponse(resp)

	var info PoolInfo
	if err = json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return PoolInfo{}, err
	}

	return info, nil
}

// ListPools returns list of pools currently configured and being used
// on the cluster.
func (adm *AdminClient) ListPools(ctx context.Context) ([]Pool, error) {
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
	var pools []Pool
	if err = json.NewDecoder(resp.Body).Decode(&pools); err != nil {
		return nil, err
	}
	return pools, nil
}
