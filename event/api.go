// Copyright (c) 2015-2025 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package event

import "time"

// API represents the api event
type API struct {
	Version      string    `json:"version"`
	DeploymentID string    `json:"deploymentid,omitempty"`
	SiteName     string    `json:"siteName,omitempty"`
	Time         time.Time `json:"time"`
	Event        string    `json:"event"`

	Type string `json:"type,omitempty"`

	API struct {
		Name       string `json:"name,omitempty"`
		Bucket     string `json:"bucket,omitempty"`
		Object     string `json:"object,omitempty"`
		StatusCode int    `json:"statusCode,omitempty"`
	} `json:"api"`

	Error string `json:"error,omitempty"`
}
