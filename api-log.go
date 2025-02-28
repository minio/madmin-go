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

// LogMask is a bit mask for log types.
type LogMask uint64

const (
	LogMaskFatal LogMask = 1 << iota
	LogMaskWarning
	LogMaskError
	LogMaskEvent
	LogMaskInfo

	// LogMaskAll must be the last.
	LogMaskAll LogMask = (1 << iota) - 1
)

// Mask returns the LogMask as uint64
func (m LogMask) Mask() uint64 {
	return uint64(m)
}

// Contains returns whether all flags in other is present in t.
func (m LogMask) Contains(other LogMask) bool {
	return m&other == other
}

// LogKind specifies the kind of error log
type LogKind string

const (
	LogKindFatal   LogKind = "FATAL"
	LogKindWarning LogKind = "WARNING"
	LogKindError   LogKind = "ERROR"
	LogKindEvent   LogKind = "EVENT"
	LogKindInfo    LogKind = "INFO"
)

// LogMask returns the mask based on the kind.
func (l LogKind) LogMask() LogMask {
	switch l {
	case LogKindFatal:
		return LogMaskFatal
	case LogKindWarning:
		return LogMaskWarning
	case LogKindError:
		return LogMaskError
	case LogKindEvent:
		return LogMaskEvent
	case LogKindInfo:
		return LogMaskInfo
	}
	return LogMaskAll
}

func (l LogKind) String() string {
	return string(l)
}

// LogInfo holds console log messages
type LogInfo struct {
	logEntry
	ConsoleMsg string
	NodeName   string `json:"node"`
	Err        error  `json:"-"`
}

// GetLogs - listen on console log messages.
func (adm AdminClient) GetLogs(ctx context.Context, node string, lineCnt int, logKind string) <-chan LogInfo {
	logCh := make(chan LogInfo, 1)

	// Only success, start a routine to start reading line by line.
	go func(logCh chan<- LogInfo) {
		defer close(logCh)
		urlValues := make(url.Values)
		urlValues.Set("node", node)
		urlValues.Set("limit", strconv.Itoa(lineCnt))
		urlValues.Set("logType", logKind)
		for {
			reqData := requestData{
				relPath:     adminAPIPrefixV4 + "/log",
				queryValues: urlValues,
			}
			// Execute GET to call log handler
			resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
			if err != nil {
				closeResponse(resp)
				return
			}

			if resp.StatusCode != http.StatusOK {
				logCh <- LogInfo{Err: httpRespToErrorResponse(resp)}
				return
			}
			dec := json.NewDecoder(resp.Body)
			for {
				var info LogInfo
				if err = dec.Decode(&info); err != nil {
					break
				}
				select {
				case <-ctx.Done():
					return
				case logCh <- info:
				}
			}

		}
	}(logCh)

	// Returns the log info channel, for caller to start reading from.
	return logCh
}

// Mask returns the mask based on the error level.
func (l LogInfo) Mask() uint64 {
	return l.LogKind.LogMask().Mask()
}
