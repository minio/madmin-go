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
	"time"
)

//go:generate msgp $GOFILE

// Error represents the error event
type Error struct {
	Version string            `json:"version"`
	Node    string            `json:"node"`
	Time    time.Time         `json:"time"`
	Message string            `json:"message"`
	API     string            `json:"apiName"`
	Trace   *Trace            `json:"trace,omitempty"`
	Tags    map[string]string `json:"tags,omitempty"`
	XXHash  uint64            `json:"xxhash,omitempty"`
}

// Trace represents the call trace
type Trace struct {
	Source    []string          `json:"source,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
}

// GetTagValByKey gets the tag value by key
func (e Error) GetTagValByKey(key string) string {
	return e.Tags[key]
}
