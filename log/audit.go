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

package log

import (
	"strings"
	"time"
)

//msgp:clearomitted
//msgp:timezone utc
//msgp:tag json

//go:generate msgp $GOFILE

// Audit represents the user triggered audit events
type Audit struct {
	Version    string                 `json:"version"`
	Time       time.Time              `json:"time"`
	Node       string                 `json:"node,omitempty"`
	APIName    string                 `json:"apiName,omitempty"`
	Bucket     string                 `json:"bucket,omitempty"`
	Tags       map[string]string      `json:"tags,omitempty"`
	RequestID  string                 `json:"requestID,omitempty"`
	ReqClaims  map[string]interface{} `json:"requestClaims,omitempty"`
	SourceHost string                 `json:"sourceHost,omitempty"`
	AccessKey  string                 `json:"accessKey,omitempty"`
	ParentUser string                 `json:"parentUser,omitempty"`
}

// String returns a canonical string for Audit
func (a Audit) String() string {
	values := []string{
		toString("version", a.Version),
		toTime("time", a.Time),
		toString("node", a.Node),
		toString("apiName", a.APIName),
		toString("bucket", a.Bucket),
		toMap("tags", a.Tags),
		toString("requestID", a.RequestID),
		toInterfaceMap("requestClaims", a.ReqClaims),
		toString("sourceHost", a.SourceHost),
		toString("accessKey", a.AccessKey),
		toString("parentUser", a.ParentUser),
	}
	values = filterAndSort(values)
	return strings.Join(values, ",")
}
