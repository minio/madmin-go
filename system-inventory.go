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
)

// SystemInventoryTableStatus is the user-visible state of a bucket's system
// inventory table. The values mirror the server's inventory table status. An
// empty value means the source bucket is missing or transiently unreachable.
type SystemInventoryTableStatus string

const (
	SystemInventoryDisabled    SystemInventoryTableStatus = "Disabled"
	SystemInventoryBackfilling SystemInventoryTableStatus = "Backfilling"
	SystemInventoryActive      SystemInventoryTableStatus = "Active"
)

// SystemInventoryBucketStatus is the system inventory state of a single bucket.
type SystemInventoryBucketStatus struct {
	Bucket string                     `json:"bucket"`
	Status SystemInventoryTableStatus `json:"status"`
}

// SystemInventoryStatus reports whether the per-bucket system inventory feature
// is enabled and the inventory table state for every eligible bucket.
type SystemInventoryStatus struct {
	Enabled bool                          `json:"enabled"`
	Buckets []SystemInventoryBucketStatus `json:"buckets"`
}

// SystemInventoryStatus returns the system inventory feature state and the
// per-bucket inventory table status for every eligible bucket on the cluster.
func (adm *AdminClient) SystemInventoryStatus(ctx context.Context) (SystemInventoryStatus, error) {
	var result SystemInventoryStatus

	resp, err := adm.executeMethod(ctx, http.MethodGet,
		requestData{
			relPath: adminAPIPrefix + "/system-inventory/status",
		},
	)
	if err != nil {
		return result, err
	}
	defer closeResponse(resp)

	if resp.StatusCode != http.StatusOK {
		return result, httpRespToErrorResponse(resp)
	}

	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return result, err
	}

	return result, nil
}
