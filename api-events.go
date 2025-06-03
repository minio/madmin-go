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

// APIEventOpts represents the options for the APIEventOpts
type APIEventOpts struct {
	Node       string        `json:"node,omitempty"`
	API        string        `json:"api,omitempty"`
	Bucket     string        `json:"bucket,omitempty"`
	Object     string        `json:"object,omitempty"`
	StatusCode int           `json:"statusCode,omitempty"`
	Interval   time.Duration `json:"interval,omitempty"`
	Origin     event.Origin  `json:"origin,omitempty"`
	Type       event.APIType `json:"type,omitempty"`
}

// GetAPIEvents fetches the persisted API events from MinIO
func (adm AdminClient) GetAPIEvents(ctx context.Context, opts APIEventOpts) iter.Seq2[event.API, error] {
	return func(yield func(event.API, error) bool) {
		apiOpts, err := json.Marshal(opts)
		if err != nil {
			yield(event.API{}, err)
		}
		reqData := requestData{
			relPath: adminAPIPrefix + "/events/api",
			content: apiOpts,
		}
		resp, err := adm.executeMethod(ctx, http.MethodPost, reqData)
		if err != nil {
			closeResponse(resp)
			yield(event.API{}, err)
		}
		if resp.StatusCode != http.StatusOK {
			closeResponse(resp)
			return
		}
		dec := msgp.NewReader(resp.Body)
		for {
			var info event.API
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
