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

// ReplDiffOpts holds options for `mc replicate diff` command
type ReplDiffOpts struct {
	ARN     string
	Verbose bool
	Prefix  string
}

// TgtDiffInfo returns status of unreplicated objects
// for the target ARN
type TgtDiffInfo struct {
	ReplicationStatus       string `json:"rStatus,omitempty"`  // target replication status
	DeleteReplicationStatus string `json:"drStatus,omitempty"` // target delete replication status
}

// DiffInfo represents relevant replication status and last attempt to replicate
// for the replication targets configured for the bucket
type DiffInfo struct {
	Object                  string                 `json:"object"`
	VersionID               string                 `json:"versionId"`
	Targets                 map[string]TgtDiffInfo `json:"targets,omitempty"`
	Err                     error                  `json:"error,omitempty"`
	ReplicationStatus       string                 `json:"rStatus,omitempty"` // overall replication status
	DeleteReplicationStatus string                 `json:"dStatus,omitempty"` // overall replication status of version delete
	ReplicationTimestamp    time.Time              `json:"replTimestamp,omitempty"`
	LastModified            time.Time              `json:"lastModified,omitempty"`
	IsDeleteMarker          bool                   `json:"deletemarker"`
}

// BucketReplicationDiff - gets diff for non-replicated entries.
func (adm *AdminClient) BucketReplicationDiff(ctx context.Context, bucketName string, opts ReplDiffOpts) <-chan DiffInfo {
	diffCh := make(chan DiffInfo)

	// start a routine to start reading line by line.
	go func(diffCh chan<- DiffInfo) {
		defer close(diffCh)
		queryValues := url.Values{}
		queryValues.Set("bucket", bucketName)

		if opts.Verbose {
			queryValues.Set("verbose", "true")
		}
		if opts.ARN != "" {
			queryValues.Set("arn", opts.ARN)
		}
		if opts.Prefix != "" {
			queryValues.Set("prefix", opts.Prefix)
		}

		reqData := requestData{
			relPath:     adminAPIPrefix + "/replication/diff",
			queryValues: queryValues,
		}

		// Execute PUT on /minio/admin/v3/diff to set quota for a bucket.
		resp, err := adm.executeMethod(ctx, http.MethodPost, reqData)
		if err != nil {
			diffCh <- DiffInfo{Err: err}
			return
		}
		defer closeResponse(resp)

		if resp.StatusCode != http.StatusOK {
			diffCh <- DiffInfo{Err: httpRespToErrorResponse(resp)}
			return
		}

		dec := json.NewDecoder(resp.Body)
		for {
			var di DiffInfo
			if err = dec.Decode(&di); err != nil {
				break
			}
			select {
			case <-ctx.Done():
				return
			case diffCh <- di:
			}
		}
	}(diffCh)
	// Returns the diff channel, for caller to start reading from.
	return diffCh
}
