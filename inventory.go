//
// Copyright (c) 2015-2025 MinIO, Inc.
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
	"fmt"
	"net/http"
)

// InventoryJobControlStatus represents the status of an inventory job control operation
type InventoryJobControlStatus string

const (
	InventoryJobStatusCanceled  InventoryJobControlStatus = "canceled"
	InventoryJobStatusSuspended InventoryJobControlStatus = "suspended"
	InventoryJobStatusResumed   InventoryJobControlStatus = "resumed"
)

// InventoryJobControlResponse represents the response from inventory job control operations
type InventoryJobControlResponse struct {
	Status      InventoryJobControlStatus `json:"status"`
	Bucket      string                    `json:"bucket"`
	InventoryID string                    `json:"inventoryId"`
	Message     string                    `json:"message,omitempty"`
}

// CancelInventoryJob cancels a running inventory job
func (adm *AdminClient) CancelInventoryJob(ctx context.Context, bucket, inventoryID string) (*InventoryJobControlResponse, error) {
	return adm.controlInventoryJob(ctx, bucket, inventoryID, "cancel")
}

// SuspendInventoryJob suspends a running inventory job
func (adm *AdminClient) SuspendInventoryJob(ctx context.Context, bucket, inventoryID string) (*InventoryJobControlResponse, error) {
	return adm.controlInventoryJob(ctx, bucket, inventoryID, "suspend")
}

// ResumeInventoryJob resumes a suspended inventory job
func (adm *AdminClient) ResumeInventoryJob(ctx context.Context, bucket, inventoryID string) (*InventoryJobControlResponse, error) {
	return adm.controlInventoryJob(ctx, bucket, inventoryID, "resume")
}

// controlInventoryJob is the common function for all inventory job control operations
func (adm *AdminClient) controlInventoryJob(ctx context.Context, bucket, inventoryID, action string) (*InventoryJobControlResponse, error) {
	if bucket == "" {
		return nil, fmt.Errorf("bucket name cannot be empty")
	}
	if inventoryID == "" {
		return nil, fmt.Errorf("inventory ID cannot be empty")
	}

	resp, err := adm.executeMethod(ctx, http.MethodPost,
		requestData{
			relPath: adminAPIPrefix + fmt.Sprintf("/inventory/%s/%s/%s", bucket, inventoryID, action),
		},
	)
	if err != nil {
		return nil, err
	}
	defer closeResponse(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	// Decode the response
	var result InventoryJobControlResponse
	dec := json.NewDecoder(resp.Body)
	if err = dec.Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}
