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
	"net/http"
	"time"

	"github.com/minio/madmin-go/v3/event"
)

// APIEventOpts represents the options for the APIEventOpts
type APIEventOpts struct {
	Node       string
	API        string
	Bucket     string
	Object     string
	StatusCode int
	Interval   time.Duration
}

// GetAPIEvents fetches the persisted API events from MinIO
func (adm AdminClient) GetAPIEvents(ctx context.Context, opts APIEventOpts) (<-chan event.API, error) {
	apiOpts, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}
	eventCh := make(chan event.API)
	go func(eventCh chan<- event.API) {
		defer close(eventCh)
		reqData := requestData{
			relPath: adminAPIPrefix + "/events/api",
			content: apiOpts,
		}
		resp, err := adm.executeMethod(ctx, http.MethodPost, reqData)
		if err != nil {
			closeResponse(resp)
			return
		}
		if resp.StatusCode != http.StatusOK {
			return
		}
		dec := json.NewDecoder(resp.Body)
		for {
			var info event.API
			if err = dec.Decode(&info); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				continue
			}
			select {
			case <-ctx.Done():
				return
			case eventCh <- info:
			}
		}
	}(eventCh)

	return eventCh, nil
}
