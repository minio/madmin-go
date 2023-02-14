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
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// SpeedTestStatServer - stats of a server
type SpeedTestStatServer struct {
	Endpoint         string `json:"endpoint"`
	ThroughputPerSec uint64 `json:"throughputPerSec"`
	ObjectsPerSec    uint64 `json:"objectsPerSec"`
	Err              string `json:"err"`
}

// SpeedTestStats - stats of all the servers
type SpeedTestStats struct {
	ThroughputPerSec uint64                `json:"throughputPerSec"`
	ObjectsPerSec    uint64                `json:"objectsPerSec"`
	Response         Timings               `json:"responseTime"`
	TTFB             Timings               `json:"ttfb,omitempty"`
	Servers          []SpeedTestStatServer `json:"servers"`
}

// SpeedTestResult - result of the speedtest() call
type SpeedTestResult struct {
	Version    string `json:"version"`
	Servers    int    `json:"servers"`
	Disks      int    `json:"disks"`
	Size       int    `json:"size"`
	Concurrent int    `json:"concurrent"`
	PUTStats   SpeedTestStats
	GETStats   SpeedTestStats
}

// SpeedtestOpts provide configurable options for speedtest
type SpeedtestOpts struct {
	Size         int           // Object size used in speed test
	Concurrency  int           // Concurrency used in speed test
	Duration     time.Duration // Total duration of the speed test
	Autotune     bool          // Enable autotuning
	StorageClass string        // Choose type of storage-class to be used while performing I/O
	Bucket       string        // Choose a custom bucket name while performing I/O
	KeepData     bool          // Avoid cleanup after running an object speed test
}

// Speedtest - perform speedtest on the MinIO servers
func (adm *AdminClient) Speedtest(ctx context.Context, opts SpeedtestOpts) (chan SpeedTestResult, error) {
	if !opts.Autotune {
		if opts.Duration <= time.Second {
			return nil, errors.New("duration must be greater a second")
		}
		if opts.Size <= 0 {
			return nil, errors.New("size must be greater than 0 bytes")
		}
		if opts.Concurrency <= 0 {
			return nil, errors.New("concurrency must be greater than 0")
		}
	}

	queryVals := make(url.Values)
	if opts.Size > 0 {
		queryVals.Set("size", strconv.Itoa(opts.Size))
	}
	if opts.Duration > 0 {
		queryVals.Set("duration", opts.Duration.String())
	}
	if opts.Concurrency > 0 {
		queryVals.Set("concurrent", strconv.Itoa(opts.Concurrency))
	}
	if opts.Bucket != "" {
		queryVals.Set("bucket", opts.Bucket)
	}
	if opts.Autotune {
		queryVals.Set("autotune", "true")
	}
	if opts.KeepData {
		queryVals.Set("keep-data", "true")
	}
	resp, err := adm.executeMethod(ctx,
		http.MethodPost, requestData{
			relPath:     adminAPIPrefix + "/speedtest",
			queryValues: queryVals,
		})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}
	ch := make(chan SpeedTestResult)
	go func() {
		defer closeResponse(resp)
		defer close(ch)
		dec := json.NewDecoder(resp.Body)
		for {
			var result SpeedTestResult
			if err := dec.Decode(&result); err != nil {
				return
			}
			select {
			case ch <- result:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}
