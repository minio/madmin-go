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
	"time"
)

// Pool represents pool specific information
type Pool struct {
	SetCount     int      `json:"setCount"`
	DrivesPerSet int      `json:"drivesPerSet"`
	CmdLine      string   `json:"cmdline"`
	Info         PoolInfo `json:"info"`
}

// PoolDrainInfo provides pool draining state.
type PoolDrainInfo struct {
	StartTime    time.Time `json:"startTime" msg:"st"`
	TotalSize    int64     `json:"totalSize" msg:"tsm"`
	TotalObjects int64     `json:"totalObjects" msg:"to"`
}

// PoolInfo represents pool specific info such as
// if pool is currently suspended, or being drained
// etc.
type PoolInfo struct {
	ID         int            `json:"id" msg:"id"`
	LastUpdate time.Time      `json:"lastUpdate" msg:"lu"`
	Drain      *PoolDrainInfo `json:"drainInfo,omitempty" msg:"dr"`
	Suspend    bool           `json:"suspend" msg:"sp"`
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
