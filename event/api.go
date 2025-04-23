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
	Version      string                 `json:"version"`
	Type         string                 `json:"type,omitempty"`
	DeploymentID string                 `json:"deploymentid,omitempty"`
	SiteName     string                 `json:"siteName,omitempty"`
	Time         time.Time              `json:"time"`
	Node         string                 `json:"node,omitempty"`
	Name         string                 `json:"name,omitempty"`
	Bucket       string                 `json:"bucket,omitempty"`
	Object       string                 `json:"object,omitempty"`
	VersionID    string                 `json:"versionId,omitempty"`
	External     bool                   `json:"external,omitempty"`
	Tags         map[string]interface{} `json:"tags,omitempty"`
	CallInfo     *CallInfo              `json:"callInfo,omitempty"`
}

// CallInfo represents the info for the external call
type CallInfo struct {
	StatusCode      int                    `json:"statusCode,omitempty"`
	InputBytes      int64                  `json:"rx"`
	OutputBytes     int64                  `json:"tx"`
	HeaderBytes     int64                  `json:"txHeaders,omitempty"`
	TimeToFirstByte string                 `json:"timeToFirstByte,omitempty"`
	TimeToResponse  string                 `json:"timeToResponse,omitempty"`
	SourceHost      string                 `json:"sourceHost,omitempty"`
	RequestID       string                 `json:"requestID,omitempty"`
	UserAgent       string                 `json:"userAgent,omitempty"`
	ReqPath         string                 `json:"requestPath,omitempty"`
	ReqHost         string                 `json:"requestHost,omitempty"`
	ReqClaims       map[string]interface{} `json:"requestClaims,omitempty"`
	ReqQuery        map[string]string      `json:"requestQuery,omitempty"`
	ReqHeader       map[string]string      `json:"requestHeader,omitempty"`
	RespHeader      map[string]string      `json:"responseHeader,omitempty"`
	AccessKey       string                 `json:"accessKey,omitempty"`
	ParentUser      string                 `json:"parentUser,omitempty"`
}
