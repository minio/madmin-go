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

import (
	"time"
)

//go:generate msgp $GOFILE

// Error represents the error event
type Error struct {
	Version   string            `json:"version"`
	API       string            `json:"apiName"`
	Time      time.Time         `json:"time"`
	Node      string            `json:"node"`
	Message   string            `json:"message"`
	RequestID string            `json:"requestID,omitempty"`
	UserAgent string            `json:"userAgent,omitempty"`
	Bucket    string            `json:"bucket,omitempty"`
	Object    string            `json:"object,omitempty"`
	VersionID string            `json:"versionId,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Trace     *Trace            `json:"trace,omitempty"`
}

// Trace represents the trace
type Trace struct {
	Source    []string               `json:"source,omitempty"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}
