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
	"net/http"
	"net/url"
	"strconv"
)

// ServerPeerUpdateStatus server update peer binary update result
type ServerPeerUpdateStatus struct {
	Host           string                `json:"host"`
	Err            string                `json:"err,omitempty"`
	CurrentVersion string                `json:"currentVersion"`
	UpdatedVersion string                `json:"updatedVersion"`
	WaitingDrives  map[string]DiskStatus `json:"waitingDrives,omitempty"`
}

// ServerUpdateStatus server update status
type ServerUpdateStatus struct {
	DryRun  bool                     `json:"dryRun"`
	Results []ServerPeerUpdateStatus `json:"results,omitempty"`
}

// ServerUpdateOpts specifies the URL (optionally to download the binary from)
// also allows a dry-run, the new API is idempotent which means you can
// run it as many times as you want and any server that is not upgraded
// automatically does get upgraded eventually to the relevant version.
type ServerUpdateOpts struct {
	UpdateURL string
	DryRun    bool
}

// ServerUpdate - updates and restarts the MinIO cluster to latest version.
// optionally takes an input URL to specify a custom update binary link
func (adm *AdminClient) ServerUpdate(ctx context.Context, opts ServerUpdateOpts) (us ServerUpdateStatus, err error) {
	queryValues := url.Values{}
	queryValues.Set("type", "2")
	queryValues.Set("updateURL", opts.UpdateURL)
	queryValues.Set("dry-run", strconv.FormatBool(opts.DryRun))

	// Request API to Restart server
	resp, err := adm.executeMethod(ctx,
		http.MethodPost, requestData{
			relPath:     adminAPIPrefix + "/update",
			queryValues: queryValues,
		},
	)
	defer closeResponse(resp)
	if err != nil {
		return us, err
	}

	if resp.StatusCode != http.StatusOK {
		return us, httpRespToErrorResponse(resp)
	}

	if err = json.NewDecoder(resp.Body).Decode(&us); err != nil {
		return us, err
	}

	return us, nil
}
