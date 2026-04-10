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
)

// TablesReplicationStatus is the response for the tables replication status admin API.
type TablesReplicationStatus struct {
	Status string                          `json:"status"`
	Tables map[string]TableReplicationInfo `json:"tables"`
}

// TableReplicationInfo is the per-table replication status.
type TableReplicationInfo struct {
	Name             string   `json:"name"`
	Type             string   `json:"type"`
	VerifiedVersion  int      `json:"verifiedVersion"`
	LatestVersion    int      `json:"latestVersion"`
	VersionsBehind   int      `json:"versionsBehind"`
	MissingFiles     int      `json:"missingFiles"`
	MissingFileNames []string `json:"missingFileNames,omitempty"`
	RetriedFiles     int      `json:"retriedFiles"`
}

// TablesReplicationStatus returns the per-table replication tracking state
// maintained by the catalog scanner leader.
func (adm *AdminClient) TablesReplicationStatus(ctx context.Context) (TablesReplicationStatus, error) {
	var status TablesReplicationStatus

	reqData := requestData{
		relPath: adminAPIPrefix + "/tables/replication-status",
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

// TablesStartReplicaFailover signals the replica site to begin the failover
// and promotion process. This disables replication and completes the catalog
// scan so the replica can accept write traffic.
func (adm *AdminClient) TablesStartReplicaFailover(ctx context.Context) error {
	reqData := requestData{
		relPath: adminAPIPrefix + "/tables/start-failover",
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

// TablesReplicationResetCatalog signals the replica site to backup and delete
// its catalog so it can be rebuilt from scratch by the next scanner cycle.
func (adm *AdminClient) TablesReplicationResetCatalog(ctx context.Context) error {
	reqData := requestData{
		relPath: adminAPIPrefix + "/tables/reset-catalog",
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
