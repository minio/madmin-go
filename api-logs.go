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

// APILogOpts represents the options for the APILogOpts
type APILogOpts struct {
	Nodes        []string      `json:"nodes,omitempty"`   // entries ending with "*" are prefix matches, otherwise exact match
	APIs         []string      `json:"apis,omitempty"`    // entries ending with "*" are prefix matches, otherwise exact match
	Buckets      []string      `json:"buckets,omitempty"` // entries ending with "*" are prefix matches, otherwise exact match
	Prefix       string        `json:"prefix,omitempty"`
	StatusCodes  []int         `json:"statusCodes,omitempty"`
	StatusRanges []string      `json:"statusRanges,omitempty"` // e.g. "2xx", "4xx", "5xx"
	Interval     time.Duration `json:"interval,omitempty"`
	Origins      []log.Origin  `json:"origins,omitempty"`
	Types        []log.APIType `json:"types,omitempty"`
	MaxPerNode   int           `json:"maxPerNode,omitempty"` // Deprecated
	Limit        int           `json:"limit,omitempty"`
}

// GetAPILogs fetches the persisted API logs from MinIO
func (adm AdminClient) GetAPILogs(ctx context.Context, opts APILogOpts) iter.Seq2[log.API, error] {
	return func(yield func(log.API, error) bool) {
		apiOpts, err := json.Marshal(opts)
		if err != nil {
			yield(log.API{}, err)
			return
		}
		reqData := requestData{
			relPath: adminAPIPrefix + "/logs/api",
			content: apiOpts,
		}
		resp, err := adm.executeMethod(ctx, http.MethodPost, reqData)
		if err != nil {
			yield(log.API{}, err)
			return
		}
		if resp.StatusCode != http.StatusOK {
			yield(log.API{}, httpRespToErrorResponse(resp))
			return
		}
		dec := msgp.NewReader(resp.Body)
		for {
			var info log.API
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
