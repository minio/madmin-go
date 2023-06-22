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
	"net/http"
	"net/url"
	"time"
)

// NetperfNodeResult - stats from each server
type NetperfNodeResult struct {
	Endpoint  string `json:"endpoint"`
	TX        uint64 `json:"tx"`
	TxDurMs   uint64 `json:"txDurMs"`
	RX        uint64 `json:"rx"`
	RxDurMs   uint64 `json:"rxDurMs"`
	TotalConn uint64 `json:"totalConn"`
	Error     string `json:"error,omitempty"`
}

// NetperfResult - aggregate results from all servers
type NetperfResult struct {
	NodeResults []NetperfNodeResult `json:"nodeResults"`
}

// Netperf - perform netperf on the MinIO servers
func (adm *AdminClient) Netperf(ctx context.Context, duration time.Duration) (result NetperfResult, err error) {
	queryVals := make(url.Values)
	queryVals.Set("duration", duration.String())

	resp, err := adm.executeMethod(ctx,
		http.MethodPost, requestData{
			relPath:     adminAPIPrefix + "/speedtest/net",
			queryValues: queryVals,
		})
	if err != nil {
		return result, err
	}
	if resp.StatusCode != http.StatusOK {
		return result, httpRespToErrorResponse(resp)
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}
