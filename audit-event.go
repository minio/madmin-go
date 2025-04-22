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
	"fmt"
	"net/http"
	"net/url"

	"github.com/minio/madmin-go/v3/event"
)

func (adm AdminClient) GetAPIEvents(ctx context.Context, node string, api string) <-chan event.API {
	logCh := make(chan event.API)

	// Only success, start a routine to start reading line by line.
	go func(logCh chan<- event.API) {
		defer close(logCh)
		urlValues := make(url.Values)
		urlValues.Set("node", node)
		urlValues.Set("api", api)
		// for {
		reqData := requestData{
			relPath:     adminAPIPrefix + "/events/api",
			queryValues: urlValues,
		}
		// Execute GET to call log handler
		resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
		if err != nil {
			closeResponse(resp)
			return
		}

		if resp.StatusCode != http.StatusOK {
			// logCh <- LogInfo{Err: httpRespToErrorResponse(resp)}
			return
		}
		dec := json.NewDecoder(resp.Body)
		for {
			var info event.API
			if err = dec.Decode(&info); err != nil {
				fmt.Println(err)
				break
			}
			select {
			case <-ctx.Done():
				return
			case logCh <- info:
			}
		}
		// }

	}(logCh)

	// Returns the log info channel, for caller to start reading from.
	return logCh
}
