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
	"time"
)

//go:generate msgp -file $GOFILE

// ReplDiffOpts holds options for `mc replicate diff` command
//
//msgp:ignore ReplDiffOpts
type ReplDiffOpts struct {
	ARN     string
	Verbose bool
	Prefix  string
}

// TgtDiffInfo returns status of unreplicated objects
// for the target ARN
//msgp:ignore TgtDiffInfo

type TgtDiffInfo struct {
	ReplicationStatus       string `json:"rStatus,omitempty"`  // target replication status
	DeleteReplicationStatus string `json:"drStatus,omitempty"` // target delete replication status
}

// DiffInfo represents relevant replication status and last attempt to replicate
// for the replication targets configured for the bucket
//msgp:ignore DiffInfo

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
			relPath:     adminAPIPrefixV4 + "/replication/diff",
			queryValues: queryValues,
		}

		// Execute PUT on /minio/admin/v4/diff to set quota for a bucket.
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

// ReplicationMRF represents MRF backlog for a bucket
type ReplicationMRF struct {
	NodeName   string `json:"nodeName" msg:"n"`
	Bucket     string `json:"bucket" msg:"b"`
	Object     string `json:"object" msg:"o"`
	VersionID  string `json:"versionId" msg:"v"`
	RetryCount int    `json:"retryCount" msg:"rc"`
	Err        string `json:"error,omitempty" msg:"err"`
}

// BucketReplicationMRF - gets MRF entries for bucket and node. Return MRF across buckets if bucket is empty, across nodes
// if node is `all`
func (adm *AdminClient) BucketReplicationMRF(ctx context.Context, bucketName string, node string) <-chan ReplicationMRF {
	mrfCh := make(chan ReplicationMRF)

	// start a routine to start reading line by line.
	go func(mrfCh chan<- ReplicationMRF) {
		defer close(mrfCh)
		queryValues := url.Values{}
		queryValues.Set("bucket", bucketName)
		if node != "" {
			queryValues.Set("node", node)
		}
		reqData := requestData{
			relPath:     adminAPIPrefixV4 + "/replication/mrf",
			queryValues: queryValues,
		}

		// Execute GET on /minio/admin/v4/replication/mrf to get mrf backlog for a bucket.
		resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
		if err != nil {
			mrfCh <- ReplicationMRF{Err: err.Error()}
			return
		}
		defer closeResponse(resp)

		if resp.StatusCode != http.StatusOK {
			mrfCh <- ReplicationMRF{Err: httpRespToErrorResponse(resp).Error()}
			return
		}
		dec := json.NewDecoder(resp.Body)
		for {
			var bk ReplicationMRF
			if err = dec.Decode(&bk); err != nil {
				break
			}
			select {
			case <-ctx.Done():
				return
			case mrfCh <- bk:
			}
		}
	}(mrfCh)
	// Returns the mrf backlog channel, for caller to start reading from.
	return mrfCh
}

// LatencyStat represents replication link latency statistics
type LatencyStat struct {
	Curr time.Duration `json:"curr"`
	Avg  time.Duration `json:"avg"`
	Max  time.Duration `json:"max"`
}

// TimedErrStats has failed replication stats across time windows
type TimedErrStats struct {
	LastMinute RStat `json:"lastMinute"`
	LastHour   RStat `json:"lastHour"`
	Totals     RStat `json:"totals"`
	// ErrCounts is a map of error codes to count of errors since server start - tracks
	// only AccessDenied errors for now.
	ErrCounts map[string]int `json:"errCounts,omitempty"`
}

// Add - adds two TimedErrStats
func (te TimedErrStats) Add(o TimedErrStats) TimedErrStats {
	m := make(map[string]int)
	for k, v := range te.ErrCounts {
		m[k] = v
	}
	for k, v := range o.ErrCounts {
		m[k] += v
	}
	return TimedErrStats{
		LastMinute: te.LastMinute.Add(o.LastMinute),
		LastHour:   te.LastHour.Add(o.LastHour),
		Totals:     te.Totals.Add(o.Totals),
		ErrCounts:  m,
	}
}

// RStat represents count and bytes replicated/failed
type RStat struct {
	Count float64 `json:"count"`
	Bytes int64   `json:"bytes"`
}

// Add - adds two RStats
func (r RStat) Add(r1 RStat) RStat {
	return RStat{
		Count: r.Count + r1.Count,
		Bytes: r.Bytes + r1.Bytes,
	}
}

// DowntimeInfo captures the downtime information
type DowntimeInfo struct {
	Duration StatRecorder `json:"duration"`
	Count    StatRecorder `json:"count"`
}

// RecordCount records the value
func (d *DowntimeInfo) RecordCount(value int64) {
	d.Count.Record(value)
}

// RecordDuration records the value
func (d *DowntimeInfo) RecordDuration(value int64) {
	d.Duration.Record(value)
}

// StatRecorder records and calculates the aggregates
type StatRecorder struct {
	Total int64 `json:"total"`
	Avg   int64 `json:"avg"`
	Max   int64 `json:"max"`
	count int64 `json:"-"`
}

// Record will record the value and calculates the aggregates on the fly
func (s *StatRecorder) Record(value int64) {
	s.Total += value
	if s.count == 0 || value > s.Max {
		s.Max = value
	}
	s.count++
	s.Avg = s.Total / s.count
}
