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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
	StartTime      time.Time `json:"startTime"`
	TotalNodes     int       `json:"totalNodes"`
	RestartedNodes int       `json:"restartedNodes"`
	UpgradedNodes  int       `json:"upgradedNodes"`
	OfflineNodes   int       `json:"offlineNodes"`
	ErrorNodes     int       `json:"errorNodes"`
	ETA            int       `json:"eta,omitempty"` // in seconds
	Err            error     `json:"-"`
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
	resp, err := adm.executeMethod(
		ctx,
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

// MaxServerUpdateFileSize is the largest update file that servers take in
// ServerUpdateFromFile.
const MaxServerUpdateFileSize = 512 << 20

// ServerUpdateFromFileOpts specifies how ServerUpdateFromFile installs its
// release, as ServerUpdateOpts does for ServerUpdate.
type ServerUpdateFromFileOpts struct {
	DryRun              bool
	Rolling             bool
	RollingGracefulWait time.Duration
	ByNode              bool
}

// ServerUpdateFromFile installs the labeled release in an update file on
// every server of the cluster and restarts them, as ServerUpdate does after
// a download. The update file, of size bytes read from r, is a
// gzip-compressed tar archive of one platform and variant of a release:
// release.json and the binary, with their minisign signatures. The server
// checks it as it would a download, and uploading it names the release.
//
// The file is read twice from r's current offset, first to sign its SHA-256:
// servers refuse an update file that the request's signature does not
// cover. The request is not retried, and ServerUpdateStatus follows a
// rolling restart.
func (adm *AdminClient) ServerUpdateFromFile(ctx context.Context, opts ServerUpdateFromFileOpts, r io.ReadSeeker, size int64) (us ServerUpdateStatus, err error) {
	if size <= 0 || size > MaxServerUpdateFileSize {
		return us, fmt.Errorf("update file size %d is not between 1 and %d bytes", size, MaxServerUpdateFileSize)
	}
	start, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return us, err
	}
	h := sha256.New()
	if _, err = io.CopyN(h, r, size); err != nil {
		return us, fmt.Errorf("reading the update file: %w", err)
	}
	if _, err = r.Seek(start, io.SeekStart); err != nil {
		return us, err
	}
	contentSHA256 := hex.EncodeToString(h.Sum(nil))

	queryValues := url.Values{}
	queryValues.Set("dry-run", strconv.FormatBool(opts.DryRun))
	if opts.Rolling {
		queryValues.Set("wait", strconv.FormatInt(int64(opts.RollingGracefulWait), 10))
	}
	queryValues.Set("by-node", strconv.FormatBool(opts.ByNode))

	resp, err := adm.executeMethod(
		ctx,
		http.MethodPost, requestData{
			relPath:       adminAPIPrefix + "/update-from-file",
			queryValues:   queryValues,
			customHeaders: http.Header{"Content-Type": []string{"application/gzip"}},
			contentReader: io.LimitReader(r, size),
			contentLength: size,
			contentSHA256: contentSHA256,
			// The file has been read: another attempt would send less.
			retry: func(*http.Response, []byte, error) bool { return false },
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

// CancelServerUpdate will cancel an ongoing update and the rolling restart if already engaged
func (adm *AdminClient) CancelServerUpdate(ctx context.Context) error {
	resp, err := adm.executeMethod(
		ctx,
		http.MethodPost, requestData{
			relPath: adminAPIPrefix + "/cancel-update",
		},
	)
	defer closeResponse(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}
	return nil
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
	resp, err := adm.executeMethod(
		ctx,
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
	resp, err := adm.executeMethod(
		ctx,
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
