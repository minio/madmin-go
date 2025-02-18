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
	"time"
)

//msgp:clearomitted
//msgp:tag json
//go:generate msgp
// LicenseInfo is a structure containing MinIO license information.

type LicenseInfo struct {
	ID           string    `json:"ID"`           // The license ID
	Organization string    `json:"Organization"` // Name of the organization using the license
	Plan         string    `json:"Plan"`         // License plan. E.g. "ENTERPRISE-PLUS"
	IssuedAt     time.Time `json:"IssuedAt"`     // Point in time when the license was issued
	ExpiresAt    time.Time `json:"ExpiresAt"`    // Point in time when the license expires
	Trial        bool      `json:"Trial"`        // Whether the license is on trial
	APIKey       string    `json:"APIKey"`       // Subnet account API Key
}

// GetLicenseInfo - returns the license info
func (adm *AdminClient) GetLicenseInfo(ctx context.Context) (*LicenseInfo, error) {
	// Execute GET on /minio/admin/v3/licenseinfo to get license info.
	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{
			relPath: adminAPIPrefixV3 + "/license-info",
		})
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	l := LicenseInfo{}
	err = json.NewDecoder(resp.Body).Decode(&l)
	if err != nil {
		return nil, err
	}
	return &l, nil
}
