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
	"strconv"
)

// DriveSpeedTestResult - result of the drive speed test
type DriveSpeedTestResult struct {
	Version   string      `json:"version"`
	Endpoint  string      `json:"endpoint"`
	DrivePerf []DrivePerf `json:"drivePerf,omitempty"`

	Error string `json:"string,omitempty"`
}

// DrivePerf - result of drive speed test on 1 drive mounted at path
type DrivePerf struct {
	Path            string `json:"path"`
	ReadThroughput  uint64 `json:"readThroughput"`
	WriteThroughput uint64 `json:"writeThroughput"`

	Error string `json:"error,omitempty"`
}

// DriveSpeedTestOpts provide configurable options for drive speedtest
type DriveSpeedTestOpts struct {
	Serial    bool   // Run speed tests one drive at a time
	BlockSize uint64 // BlockSize for read/write (default 4MiB)
	FileSize  uint64 // Total fileSize to write and read (default 1GiB)
}

// DriveSpeedtest - perform drive speedtest on the MinIO servers
func (adm *AdminClient) DriveSpeedtest(ctx context.Context, opts DriveSpeedTestOpts) (chan DriveSpeedTestResult, error) {
	queryVals := make(url.Values)
	if opts.Serial {
		queryVals.Set("serial", "true")
	}
	queryVals.Set("blocksize", strconv.FormatUint(opts.BlockSize, 10))
	queryVals.Set("filesize", strconv.FormatUint(opts.FileSize, 10))
	resp, err := adm.executeMethod(ctx,
		http.MethodPost, requestData{
			relPath:     adminAPIPrefix + "/speedtest/drive",
			queryValues: queryVals,
		})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}
	ch := make(chan DriveSpeedTestResult)
	go func() {
		defer closeResponse(resp)
		defer close(ch)

		dec := json.NewDecoder(resp.Body)
		for {
			var result DriveSpeedTestResult
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
