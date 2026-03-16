//
// Copyright (c) 2015-2026 MinIO, Inc.
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
)

//msgp:tag json
//go:generate msgp -d clearomitted -d "timezone utc" -file $GOFILE

// NotificationTarget holds information about a single configured notification target.
type NotificationTarget struct {
	ARN     string   `json:"arn"`
	Type    string   `json:"type"`
	ID      string   `json:"id"`
	Status  string   `json:"status"`
	Buckets []string `json:"buckets,omitempty"`
}

// NotificationTargetsInfo is the response from ListNotificationTargets.
type NotificationTargetsInfo struct {
	Targets []NotificationTarget `json:"targets"`
}

// ListNotificationTargets returns all configured notification targets on the server,
// including their online/offline status and which buckets are subscribed to each.
func (adm *AdminClient) ListNotificationTargets(ctx context.Context) (NotificationTargetsInfo, error) {
	resp, err := adm.executeMethod(ctx, http.MethodGet, requestData{
		relPath: adminAPIPrefix + "/notification-targets",
	})
	defer closeResponse(resp)
	if err != nil {
		return NotificationTargetsInfo{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return NotificationTargetsInfo{}, httpRespToErrorResponse(resp)
	}

	var info NotificationTargetsInfo
	if err = json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return NotificationTargetsInfo{}, err
	}
	return info, nil
}
