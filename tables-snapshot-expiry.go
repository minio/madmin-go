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
	"io"
	"net/http"
	"net/url"
)

// RunTableSnapshotExpiryOptions carries optional overrides for an on-demand
// snapshot-expiration run. A nil field inherits the server default.
type RunTableSnapshotExpiryOptions struct {
	MaxSnapshotAgeHours *int `json:"maxSnapshotAgeHours,omitempty"`
	MinSnapshotsToKeep  *int `json:"minSnapshotsToKeep,omitempty"`
}

// RunTableSnapshotExpiryResult reports the outcome of RunTableSnapshotExpiry.
type RunTableSnapshotExpiryResult struct {
	SnapshotsExpired int `json:"snapshotsExpired"`
}

// RunTableSnapshotExpiry runs snapshot expiration for a single table
// synchronously, regardless of the table's maintenance schedule or
// enabled/disabled status. It expires snapshot references from table metadata
// only; referenced files are reclaimed separately by unreferenced-file removal.
// namespace uses dot ('.') as the level delimiter.
func (adm *AdminClient) RunTableSnapshotExpiry(ctx context.Context, warehouse, namespace, table string, opts RunTableSnapshotExpiryOptions) (RunTableSnapshotExpiryResult, error) {
	var result RunTableSnapshotExpiryResult

	body, err := json.Marshal(opts)
	if err != nil {
		return result, err
	}

	values := make(url.Values)
	values.Set("warehouse", warehouse)
	values.Set("namespace", namespace)
	values.Set("table", table)

	reqData := requestData{
		relPath:     adminAPIPrefix + "/tables/snapshot-expiry/run",
		queryValues: values,
		content:     body,
	}

	resp, err := adm.executeMethod(ctx, http.MethodPost, reqData)
	defer closeResponse(resp)
	if err != nil {
		return result, err
	}

	if resp.StatusCode != http.StatusOK {
		return result, httpRespToErrorResponse(resp)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}
	if err = json.Unmarshal(b, &result); err != nil {
		return result, err
	}

	return result, nil
}
