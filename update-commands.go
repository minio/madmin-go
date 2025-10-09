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
	"time"
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

	Error string `json:"error,omitempty"`
}

// UpdateProgress reports progress of a rolling server update
type UpdateProgress struct {
	StartTime     time.Time `json:"startTime"`
	UpgradedNodes int       `json:"upgradedNodes"`
	OfflineNodes  int       `json:"offlineNodes"`
	PendingNodes  int       `json:"pendingNodes"`
	ErrorNodes    int       `json:"errorNodes"`
	ETA           int       `json:"eta,omitempty"` // in seconds
	Err           error     `json:"-"`
}

// ServerUpdateOpts specifies the URL (optionally to download the binary from)
// also allows a dry-run, the new API is idempotent which means you can
// run it as many times as you want and any server that is not upgraded
// automatically does get upgraded eventually to the relevant version.
type ServerUpdateOpts struct {
	UpdateURL           string
	DryRun              bool
	Rolling             bool
	RollingGracefulWait time.Duration
	ByNode              bool
}

// ServerUpdate - updates and restarts the MinIO cluster to latest version.
// optionally takes an input URL to specify a custom update binary link
func (adm *AdminClient) ServerUpdate(ctx context.Context, opts ServerUpdateOpts) (us ServerUpdateStatus, err error) {
	queryValues := url.Values{}
	queryValues.Set("type", "2")
	queryValues.Set("updateURL", opts.UpdateURL)
	queryValues.Set("dry-run", strconv.FormatBool(opts.DryRun))
	if opts.Rolling {
		queryValues.Set("wait", strconv.FormatInt(int64(opts.RollingGracefulWait), 10))
	}
	queryValues.Set("by-node", strconv.FormatBool(opts.ByNode))

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

// NodeBumpVersionResp is the result of BumpVersion API in a single node
type NodeBumpVersionResp struct {
	Done    bool   `json:"done"`
	Offline bool   `json:"offline"`
	Error   string `json:"error,omitempty"`
}

// ClusterBumpVersionResp is the result of BumpVersion API of the cluster
type ClusterBumpVersionResp struct {
	Nodes map[string]NodeBumpVersionResp `json:"nodes,omitempty"`
	Error string                         `json:"error,omitempty"`
}

// BumpVersion asks the cluster to use the newest internal backend format and internode APIs that is supported by the current binary
func (adm *AdminClient) BumpVersion(ctx context.Context, dryRun bool) (r ClusterBumpVersionResp, err error) {
	values := url.Values{}
	values.Set("dry-run", strconv.FormatBool(dryRun))
	resp, err := adm.executeMethod(ctx,
		http.MethodPost,
		requestData{
			queryValues: values,
			relPath:     adminAPIPrefix + "/bump-version",
		},
	)
	defer closeResponse(resp)
	if err != nil {
		return r, err
	}
	if resp.StatusCode != http.StatusOK {
		return r, httpRespToErrorResponse(resp)
	}
	err = json.NewDecoder(resp.Body).Decode(&r)
	return r, err
}

// APIDesc describes the backend format version and the node API version of a single node
type APIDesc struct {
	BackendVersion Version `json:"backendVersion"`
	NodeAPIVersion uint32  `json:"nodeAPIVersion"`
	Error          string  `json:"error,omitempty"`
}

// ClusterAPIDesc describes the backend format version and the node API version of all nodes in the cluster
type ClusterAPIDesc struct {
	Nodes map[string]APIDesc `json:"nodes,omitempty"`
	Error string             `json:"error,omitempty"`
}

// GetAPIDesc returns the backend format version and the node API version of all nodes in the cluster
func (adm *AdminClient) GetAPIDesc(ctx context.Context) (r ClusterAPIDesc, err error) {
	values := url.Values{}
	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{
			queryValues: values,
			relPath:     adminAPIPrefix + "/api-desc",
		},
	)
	defer closeResponse(resp)
	if err != nil {
		return r, err
	}
	if resp.StatusCode != http.StatusOK {
		return r, httpRespToErrorResponse(resp)
	}
	err = json.NewDecoder(resp.Body).Decode(&r)
	return r, err
}

// ServerUpdateStatus streams progress updates for an ongoing server update
func (adm *AdminClient) ServerUpdateStatus(ctx context.Context) <-chan UpdateProgress {
	progressCh := make(chan UpdateProgress)
	go func(progressCh chan<- UpdateProgress) {
		defer close(progressCh)

		reqData := requestData{
			relPath: adminAPIPrefix + "/update-status",
		}

		resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
		if err != nil {
			progressCh <- UpdateProgress{Err: err}
			return
		}

		if resp.StatusCode != http.StatusOK {
			closeResponse(resp)
			progressCh <- UpdateProgress{Err: httpRespToErrorResponse(resp)}
			return
		}

		dec := json.NewDecoder(resp.Body)
		for {
			var progress UpdateProgress
			if err = dec.Decode(&progress); err != nil {
				closeResponse(resp)
				return
			}
			select {
			case <-ctx.Done():
				closeResponse(resp)
				return
			case progressCh <- progress:
			}
		}
	}(progressCh)

	return progressCh
}
