//
// Copyright (c) 2015-2025 MinIO, Inc.
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
	"io"
	"iter"
	"net/http"
	"time"

	"github.com/minio/madmin-go/v4/log"
	"github.com/tinylib/msgp/msgp"
)

// ErrorLogOpts represents the options for the ErrorLogs.
//
// Wildcard syntax on Nodes / APIs / Buckets entries (case-insensitive):
//
//	"xyz"   → exact match
//	"xyz*"  → prefix match
//	"*xyz"  → suffix match
//	"*xyz*" → contains match
//	"*"     → matches anything
//
// Values within a single field OR-combine; across fields filters AND.
type ErrorLogOpts struct {
	Nodes    []string      `json:"nodes,omitempty"`
	APIs     []string      `json:"apis,omitempty"`
	Buckets  []string      `json:"buckets,omitempty"`
	Prefixes []string      `json:"prefixes,omitempty"`
	Interval time.Duration `json:"interval,omitempty"`
	Limit    int           `json:"limit,omitempty"`

	// Deprecated: use Nodes.
	Node string `json:"node,omitempty"`
	// Deprecated: use APIs.
	API string `json:"api,omitempty"`
	// Deprecated: use Buckets.
	Bucket string `json:"bucket,omitempty"`
	// Deprecated: use Prefixes.
	Prefix string `json:"prefix,omitempty"`
	// Deprecated: use Limit.
	MaxPerNode int `json:"maxPerNode,omitempty"`
}

// GetErrorLogs fetches the persisted error logs from MinIO
func (adm AdminClient) GetErrorLogs(ctx context.Context, opts ErrorLogOpts) iter.Seq2[log.Error, error] {
	return func(yield func(log.Error, error) bool) {
		errOpts, err := json.Marshal(opts)
		if err != nil {
			yield(log.Error{}, err)
			return
		}
		reqData := requestData{
			relPath: adminAPIPrefix + "/logs/error",
			content: errOpts,
		}
		resp, err := adm.executeMethod(ctx, http.MethodPost, reqData)
		if err != nil {
			yield(log.Error{}, err)
			return
		}
		if resp.StatusCode != http.StatusOK {
			yield(log.Error{}, httpRespToErrorResponse(resp))
			return
		}
		dec := msgp.NewReader(resp.Body)
		for {
			var info log.Error
			if err = info.DecodeMsg(dec); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				continue
			}
			select {
			case <-ctx.Done():
				return
			default:
				yield(info, nil)
			}
		}
	}
}
