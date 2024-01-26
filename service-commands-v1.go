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
	"net/http"
	"net/url"
)

// Deprecated: ServiceRestart - restarts the MinIO cluster
func (adm *AdminClient) ServiceRestart(ctx context.Context) error {
	return adm.serviceCallAction(ctx, ServiceActionRestart)
}

// Deprecated: ServiceUnfreeze - un-freezes all incoming S3 API calls on MinIO cluster
func (adm *AdminClient) ServiceUnfreeze(ctx context.Context) error {
	return adm.serviceCallAction(ctx, ServiceActionUnfreeze)
}

// serviceCallAction - call service restart/update/stop API.
func (adm *AdminClient) serviceCallAction(ctx context.Context, action ServiceAction) error {
	queryValues := url.Values{}
	queryValues.Set("action", string(action))

	// Request API to Restart server
	resp, err := adm.executeMethod(ctx,
		http.MethodPost, requestData{
			relPath:     adminAPIPrefix + "/service",
			queryValues: queryValues,
		},
	)
	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}

	return nil
}
