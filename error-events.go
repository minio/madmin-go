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

	"github.com/minio/madmin-go/v4/event"
	"github.com/tinylib/msgp/msgp"
)

// ErrorEventOpts represents the options for the ErrorEvents
type ErrorEventOpts struct {
	Node     string        `json:"node,omitempty"`
	API      string        `json:"api,omitempty"`
	Bucket   string        `json:"bucket,omitempty"`
	Object   string        `json:"object,omitempty"`
	Interval time.Duration `json:"interval,omitempty"`
}

// GetErrorEvents fetches the persisted error events from MinIO
func (adm AdminClient) GetErrorEvents(ctx context.Context, opts ErrorEventOpts) iter.Seq2[event.Error, error] {
	return func(yield func(event.Error, error) bool) {
		errOpts, err := json.Marshal(opts)
		if err != nil {
			yield(event.Error{}, err)
			return
		}
		reqData := requestData{
			relPath: adminAPIPrefix + "/events/error",
			content: errOpts,
		}
		resp, err := adm.executeMethod(ctx, http.MethodPost, reqData)
		if err != nil {
			yield(event.Error{}, err)
			return
		}
		if resp.StatusCode != http.StatusOK {
			yield(event.Error{}, httpRespToErrorResponse(resp))
			return
		}
		dec := msgp.NewReader(resp.Body)
		for {
			var info event.Error
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
