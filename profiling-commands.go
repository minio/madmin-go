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
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"
)

// ProfilerType represents the profiler type
// passed to the profiler subsystem.
type ProfilerType string

// Different supported profiler types.
const (
	ProfilerCPU        ProfilerType = "cpu"        // represents CPU profiler type
	ProfilerCPUIO      ProfilerType = "cpuio"      // represents CPU with IO (fgprof) profiler type
	ProfilerMEM        ProfilerType = "mem"        // represents MEM profiler type
	ProfilerBlock      ProfilerType = "block"      // represents Block profiler type
	ProfilerMutex      ProfilerType = "mutex"      // represents Mutex profiler type
	ProfilerTrace      ProfilerType = "trace"      // represents Trace profiler type
	ProfilerThreads    ProfilerType = "threads"    // represents ThreadCreate profiler type
	ProfilerGoroutines ProfilerType = "goroutines" // represents Goroutine dumps.
	ProfilerRuntime    ProfilerType = "runtime"    // Include runtime metrics
)

// StartProfilingResult holds the result of starting
// profiler result in a given node.
type StartProfilingResult struct {
	NodeName string `json:"nodeName"`
	Success  bool   `json:"success"`
	Error    string `json:"error"`
}

// Profile makes an admin call to remotely start profiling on a standalone
// server or the whole cluster in  case of a distributed setup for a specified duration.
func (adm *AdminClient) Profile(ctx context.Context, profiler ProfilerType, duration time.Duration) (io.ReadCloser, error) {
	v := url.Values{}
	v.Set("profilerType", string(profiler))
	v.Set("duration", duration.String())
	resp, err := adm.executeMethod(ctx,
		http.MethodPost, requestData{
			relPath:     adminAPIPrefix + "/profile",
			queryValues: v,
		},
	)
	if err != nil {
		closeResponse(resp)
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	if resp.Body == nil {
		return nil, errors.New("body is nil")
	}
	return resp.Body, nil
}
