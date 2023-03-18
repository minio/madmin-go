//
// Copyright (c) 2015-2022 MinIO, Inc.
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
	"math/bits"
	"net/http"
	"time"
)

//go:generate stringer -type=TraceType -trimprefix=Trace $GOFILE

// TraceType indicates the type of the tracing Info
type TraceType uint64

const (
	// TraceOS tracing (Golang os package calls)
	TraceOS TraceType = 1 << iota
	// TraceStorage tracing (MinIO Storage Layer)
	TraceStorage
	// TraceS3 provides tracing of S3 API calls
	TraceS3
	// TraceInternal tracing internal (.minio.sys/...) HTTP calls
	TraceInternal
	// TraceScanner will trace scan operations.
	TraceScanner
	// TraceDecommission will trace decommission operations.
	TraceDecommission
	// TraceHealing will trace healing operations.
	TraceHealing
	// TraceBatchReplication will trace batch replication operations.
	TraceBatchReplication
	// TraceBatchKeyRotation will trace batch keyrotation operations.
	TraceBatchKeyRotation
	// TraceRebalance will trace rebalance operations
	TraceRebalance
	// TraceReplicationResync will trace replication resync operations.
	TraceReplicationResync
	// TraceBootstrap will trace events during MinIO cluster bootstrap
	TraceBootstrap
	// Add more here...

	// TraceAll contains all valid trace modes.
	// This *must* be the last entry.
	TraceAll TraceType = (1 << iota) - 1
)

// Contains returns whether all flags in other is present in t.
func (t TraceType) Contains(other TraceType) bool {
	return t&other == other
}

// Overlaps returns whether any flags in t overlaps with other.
func (t TraceType) Overlaps(other TraceType) bool {
	return t&other != 0
}

// SingleType returns whether t has a single type set.
func (t TraceType) SingleType() bool {
	// Include
	return bits.OnesCount64(uint64(t)) == 1
}

// Merge will merge other into t.
func (t *TraceType) Merge(other TraceType) {
	*t = *t | other
}

// SetIf will add other if b is true.
func (t *TraceType) SetIf(b bool, other TraceType) {
	if b {
		*t = *t | other
	}
}

// Mask returns the trace type as uint32.
func (t TraceType) Mask() uint64 {
	return uint64(t)
}

// TraceInfo - represents a trace record, additionally
// also reports errors if any while listening on trace.
type TraceInfo struct {
	TraceType TraceType `json:"type"`

	NodeName string        `json:"nodename"`
	FuncName string        `json:"funcname"`
	Time     time.Time     `json:"time"`
	Path     string        `json:"path"`
	Duration time.Duration `json:"dur"`

	Message    string            `json:"msg,omitempty"`
	Error      string            `json:"error,omitempty"`
	Custom     map[string]string `json:"custom,omitempty"`
	HTTP       *TraceHTTPStats   `json:"http,omitempty"`
	HealResult *HealResultItem   `json:"healResult,omitempty"`
}

// Mask returns the trace type as uint32.
func (t TraceInfo) Mask() uint64 {
	return t.TraceType.Mask()
}

// traceInfoLegacy - represents a trace record, additionally
// also reports errors if any while listening on trace.
// For minio versions before July 2022.
type traceInfoLegacy struct {
	TraceInfo

	ReqInfo   *TraceRequestInfo  `json:"request"`
	RespInfo  *TraceResponseInfo `json:"response"`
	CallStats *TraceCallStats    `json:"stats"`

	StorageStats *struct {
		Path     string        `json:"path"`
		Duration time.Duration `json:"duration"`
	} `json:"storageStats"`
	OSStats *struct {
		Path     string        `json:"path"`
		Duration time.Duration `json:"duration"`
	} `json:"osStats"`
}

type TraceHTTPStats struct {
	ReqInfo   TraceRequestInfo  `json:"request"`
	RespInfo  TraceResponseInfo `json:"response"`
	CallStats TraceCallStats    `json:"stats"`
}

// TraceCallStats records request stats
type TraceCallStats struct {
	InputBytes  int `json:"inputbytes"`
	OutputBytes int `json:"outputbytes"`
	// Deprecated: Use TraceInfo.Duration (June 2022)
	Latency         time.Duration `json:"latency"`
	TimeToFirstByte time.Duration `json:"timetofirstbyte"`
}

// TraceRequestInfo represents trace of http request
type TraceRequestInfo struct {
	Time     time.Time   `json:"time"`
	Proto    string      `json:"proto"`
	Method   string      `json:"method"`
	Path     string      `json:"path,omitempty"`
	RawQuery string      `json:"rawquery,omitempty"`
	Headers  http.Header `json:"headers,omitempty"`
	Body     []byte      `json:"body,omitempty"`
	Client   string      `json:"client"`
}

// TraceResponseInfo represents trace of http request
type TraceResponseInfo struct {
	Time       time.Time   `json:"time"`
	Headers    http.Header `json:"headers,omitempty"`
	Body       []byte      `json:"body,omitempty"`
	StatusCode int         `json:"statuscode,omitempty"`
}
