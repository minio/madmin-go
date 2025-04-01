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
	"io"
	"net/http"
	"time"
)

//msgp:clearomitted
//go:generate msgp

// BucketScanInfo contains information of a bucket scan in a given pool/set
type BucketScanInfo struct {
	Pool        int         `msg:"pool"`
	Set         int         `msg:"set"`
	Cycle       uint64      `msg:"cycle"`
	Ongoing     bool        `msg:"ongoing"`
	LastUpdate  time.Time   `msg:"last_update"`
	LastStarted time.Time   `msg:"last_started"`
	Completed   []time.Time `msg:"completed,omitempty"`
}

// BucketScanInfo returns information of a bucket scan in all pools/sets
func (adm *AdminClient) BucketScanInfo(ctx context.Context, bucket string) ([]BucketScanInfo, error) {
	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{relPath: adminAPIPrefix + "/scanner/status/" + bucket})
	if err != nil {
		return nil, err
	}
	defer closeResponse(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var info []BucketScanInfo
	err = json.Unmarshal(respBytes, &info)
	if err != nil {
		return nil, err
	}

	return info, nil
}
