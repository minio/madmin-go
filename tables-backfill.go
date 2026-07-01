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
	"time"
)

// BackfillStatus is the response for the tables backfill status admin API.
type BackfillStatus struct {
	Status     string          `json:"status"` // "running", "completed", "failed"
	Result     *BackfillResult `json:"result,omitempty"`
	StartedAt  time.Time       `json:"startedAt,omitempty"`
	FinishedAt time.Time       `json:"finishedAt,omitempty"`
	Error      string          `json:"error,omitempty"`
}

// BackfillResult holds the counts from a catalog identity backfill run.
type BackfillResult struct {
	Warehouses int      `json:"warehouses"`
	Namespaces int      `json:"namespaces"`
	Tables     int      `json:"tables"`
	Views      int      `json:"views"`
	Warnings   []string `json:"warnings,omitempty"`
}

// TablesBackfillStart triggers a catalog identity backfill on the leader node.
func (adm *AdminClient) TablesBackfillStart(ctx context.Context) error {
	reqData := requestData{
		relPath: adminAPIPrefix + "/tables/backfill",
	}

	resp, err := adm.executeMethod(ctx, http.MethodPost, reqData)
	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}

	return nil
}

// TablesBackfillStatus returns the status of the most recent backfill run.
func (adm *AdminClient) TablesBackfillStatus(ctx context.Context) (BackfillStatus, error) {
	var status BackfillStatus

	reqData := requestData{
		relPath: adminAPIPrefix + "/tables/backfill",
	}

	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
	defer closeResponse(resp)
	if err != nil {
		return status, err
	}

	if resp.StatusCode != http.StatusOK {
		return status, httpRespToErrorResponse(resp)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return status, err
	}
	if err = json.Unmarshal(b, &status); err != nil {
		return status, err
	}

	return status, nil
}

// TablesBackfillCancel cancels a running backfill job.
func (adm *AdminClient) TablesBackfillCancel(ctx context.Context) error {
	reqData := requestData{
		relPath: adminAPIPrefix + "/tables/backfill",
	}

	resp, err := adm.executeMethod(ctx, http.MethodDelete, reqData)
	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}

	return nil
}
