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
	"time"
)

// UntierStatus is the status of an untier session for a bucket.
type UntierStatus string

const (
	// UntierStatusStarted indicates the session is running.
	UntierStatusStarted UntierStatus = "started"
	// UntierStatusCompleted indicates the session completed successfully.
	UntierStatusCompleted UntierStatus = "completed"
	// UntierStatusFailed indicates the session failed.
	UntierStatusFailed UntierStatus = "failed"
	// UntierStatusStopped indicates the session was stopped by the operator.
	UntierStatusStopped UntierStatus = "stopped"
)

// UntierStartOptions contains options for starting an untier session.
type UntierStartOptions struct {
	Bucket string `json:"bucket"`
}

// UntierStopOptions contains options for stopping a running untier session.
type UntierStopOptions struct {
	Bucket string `json:"bucket"`
}

// UntierBucketStatus contains per-bucket progress metrics for an untier session.
type UntierBucketStatus struct {
	ID              string       `json:"id"`
	Bucket          string       `json:"bucket"`
	Status          UntierStatus `json:"status"`
	StartTime       time.Time    `json:"startTime"`
	EndTime         time.Time    `json:"endTime,omitempty"`
	StoppedAt       time.Time    `json:"stoppedAt,omitempty"`
	Object          string       `json:"object,omitempty"`
	LastKey         string       `json:"lastKey,omitempty"`
	NumObjectsTotal uint64       `json:"numObjectsTotal"`
	NumObjects      uint64       `json:"numObjects"`
	NumVersions     uint64       `json:"numVersions"`
	BytesDone       uint64       `json:"bytesDone"`
	BytesTotal      uint64       `json:"bytesTotal"`
	ETASeconds      *int64       `json:"etaSeconds,omitempty"`
}

// UntierStatusInfo contains the status of all active or completed untier sessions.
type UntierStatusInfo struct {
	Buckets []UntierBucketStatus `json:"buckets"`
}

// UntierStart starts an untier operation for the given bucket.
func (adm *AdminClient) UntierStart(ctx context.Context, opts UntierStartOptions) error {
	body, err := json.Marshal(opts)
	if err != nil {
		return err
	}
	resp, err := adm.executeMethod(ctx,
		http.MethodPost,
		requestData{
			relPath: adminAPIPrefix + "/untier/start",
			content: body,
		})
	defer closeResponse(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}
	return nil
}

// UntierStatus returns the status of ongoing or completed untier sessions.
// If bucket is non-empty only that bucket's status is returned.
func (adm *AdminClient) UntierStatus(ctx context.Context, bucket string) (UntierStatusInfo, error) {
	var info UntierStatusInfo
	queryValues := url.Values{}
	if bucket != "" {
		queryValues.Set("bucket", bucket)
	}
	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{
			relPath:     adminAPIPrefix + "/untier/status",
			queryValues: queryValues,
		})
	defer closeResponse(resp)
	if err != nil {
		return info, err
	}
	if resp.StatusCode != http.StatusOK {
		return info, httpRespToErrorResponse(resp)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return info, err
	}
	if err = json.Unmarshal(b, &info); err != nil {
		return info, err
	}
	return info, nil
}

// UntierStop stops a running untier session for the given bucket.
func (adm *AdminClient) UntierStop(ctx context.Context, opts UntierStopOptions) error {
	body, err := json.Marshal(opts)
	if err != nil {
		return err
	}
	resp, err := adm.executeMethod(ctx,
		http.MethodPost,
		requestData{
			relPath: adminAPIPrefix + "/untier/stop",
			content: body,
		})
	defer closeResponse(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}
	return nil
}
