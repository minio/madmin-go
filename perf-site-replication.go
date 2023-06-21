//
// Copyright (c) 2015-2023 MinIO, Inc.
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

// SiteReplicationperfNodeResult - stats from each server
type SiteReplicationPerfNodeResult struct {
	Endpoint string `json:"endpoint"`
	TX       uint64 `json:"tx"`
	RX       uint64 `json:"rx"`
	Error    string `json:"error,omitempty"`
}

// SiteReplicationperfResult - aggregate results from all servers
type SiteReplicationPerfResult struct {
	NodeResults []SiteReplicationperfNodeResult `json:"nodeResults"`
}

// SiteReplicationPerf - perform site-replication on the MinIO servers
func (adm *AdminClient) SiteReplicationPerf(ctx context.Context, duration time.Duration) (result SiteReplicationperfResult, err error) {
	queryVals := make(url.Values)
	queryVals.Set("duration", duration.String())

	resp, err := adm.executeMethod(ctx,
		http.MethodPost, requestData{
			relPath:     adminAPIPrefix + "/speedtest/site-replication",
			queryValues: queryVals,
		})
	if err != nil {
		return result, err
	}
	defer closeResponse(resp)
	if resp.StatusCode != http.StatusOK {
		return result, httpRespToErrorResponse(resp)
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}
