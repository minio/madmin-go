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

import "time"

//go:generate go tool msgp -unexported -d clearomitted -d "tag json" -d "timezone utc" -d "maps binkeys" -file $GOFILE

// DeliveryTargetMetrics is the state of every asynchronous delivery queue in
// scope: bucket-event notification targets, audit-log targets and
// system/error-log targets.
//
// All three ride the same internal queue implementation, so they share
// TargetMetrics. They are separate maps because the operator configures them as
// separate subsystems and a key is only unique within its class -- the same
// target name can exist under both a legacy and a current audit subsystem.
//
// One MetricType bit covers all three rather than one bit each: collection is
// atomic loads either way, so selectivity is legitimately a client-side concern,
// and three bits would triple the collect, merge, navigation and test surface
// for one concept.
type DeliveryTargetMetrics struct {
	CollectedAt time.Time `json:"collected"`
	Nodes       int       `json:"nodes"`

	// Keys are "<config-subsystem>:<target-name>", e.g.
	// "notify_webhook:primary", "audit_kafka:1", "logger_webhook:main".
	//
	// The subsystem is part of the key because a bare name is not unique across
	// subsystems, and it is the identity an operator already sees in
	// `mc admin config get` -- which makes the key actionable rather than
	// opaque. The class is structurally known from which map you read, so it is
	// not repeated in the key: a reader should never have to parse one.
	Notification map[string]TargetMetrics `json:"notification,omitempty"`
	Audit        map[string]TargetMetrics `json:"audit,omitempty"`
	Logs         map[string]TargetMetrics `json:"logs,omitempty"`

	// Spill is the on-disk overflow store shared by the audit and log queues.
	Spill *TargetSpillStats `json:"spill,omitempty"`

	// LogVolume maps a log severity ("error", "warning", "fatal", "info",
	// "event") to the number of lines emitted since process start.
	//
	// It is on the section rather than on a target because a line is counted
	// once on emission regardless of how many targets it fans out to -- and
	// because it is the only signal here that still works with no target
	// configured at all.
	LogVolume map[string]uint64 `json:"log_volume,omitempty"`
}

// TargetSpillStats is the disk overflow store for a delivery queue.
type TargetSpillStats struct {
	Bytes int64 `json:"bytes,omitempty"`
	Files int64 `json:"files,omitempty"`
}

// TargetMetrics is one delivery target's state in this scope.
//
// Every numeric field sums across hosts except LastMinute, which merges as a
// TimedAction, and Drops, which sums by key. A per-node outlier -- one node
// whose queue is full while five are idle -- shows up as a ByHost entry, and
// QueueLength against QueueCapacity is a correct saturation figure at either
// scope because both sum.
type TargetMetrics struct {
	// N is the number of nodes reporting this target. Nodes are expected to
	// agree on configuration, so N is the denominator for "online on
	// NodesOnline of N nodes".
	N int `json:"n"`

	// Identity and configuration, identical on every node.
	Subsystem string `json:"subsystem,omitempty"`
	Name      string `json:"name,omitempty"`
	Type      string `json:"type,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`

	// Readiness expressed as node counts, so they sum.
	NodesOnline   int `json:"nodes_online,omitempty"`
	NodesChecking int `json:"nodes_checking,omitempty"`

	// Cumulative flow since process start; resets on restart.
	TotalEvents    uint64 `json:"total_events,omitempty"`
	TotalRequests  uint64 `json:"total_requests,omitempty"`
	FailedRequests uint64 `json:"failed_requests,omitempty"`
	WriterErrors   uint64 `json:"writer_errors,omitempty"`

	// Drops maps the reason an event was discarded to a count: "queue_full",
	// "retries_exhausted", "shutdown", or "other".
	//
	// The sum over this map is the total dropped-event count, so there is no
	// separate total field. "other" exists so that sum stays exact even when the
	// server cannot classify a drop.
	Drops map[string]uint64 `json:"drops,omitempty"`

	// Backpressure gauges.
	QueueLength   int64 `json:"queue_length,omitempty"`
	QueueCapacity int64 `json:"queue_capacity,omitempty"`
	Inflight      int64 `json:"inflight,omitempty"`
	Workers       int64 `json:"workers,omitempty"`

	// LastMinute is delivery over the last ~60s.
	//
	// Count is *requests*, not events: a batching target covers up to its batch
	// size per request, so TotalEvents/Count is the batch factor rather than a
	// rate. The mean latency is AccTime/Count and the delivery rate is
	// Count/60 -- neither is sent. Bytes is populated only by the writer path
	// that knows the encoded payload size.
	LastMinute TimedAction `json:"last_minute"`

	// LastError is the most recent delivery failure anywhere in scope, kept with
	// its timestamp so Merge is order-independent: later wins, and equal
	// timestamps break on the lexicographically smaller message.
	LastError     string    `json:"last_error,omitempty"`
	LastErrorTime time.Time `json:"last_error_time,omitzero"`
}

// Add other into t.
func (t *TargetMetrics) Add(other *TargetMetrics) {
	if other == nil {
		return
	}
	if t.N == 0 {
		t.Subsystem, t.Name, t.Type, t.Endpoint = other.Subsystem, other.Name, other.Type, other.Endpoint
	}
	t.N += other.N
	t.NodesOnline += other.NodesOnline
	t.NodesChecking += other.NodesChecking
	t.TotalEvents += other.TotalEvents
	t.TotalRequests += other.TotalRequests
	t.FailedRequests += other.FailedRequests
	t.WriterErrors += other.WriterErrors
	addMap(&t.Drops, other.Drops)
	t.QueueLength += other.QueueLength
	t.QueueCapacity += other.QueueCapacity
	t.Inflight += other.Inflight
	t.Workers += other.Workers
	t.LastMinute.Merge(other.LastMinute)
	takeLater(&t.LastErrorTime, &t.LastError, other.LastErrorTime, other.LastError)
}

// Merge other into d.
func (d *DeliveryTargetMetrics) Merge(other *DeliveryTargetMetrics) {
	if other == nil {
		return
	}
	if d.CollectedAt.Before(other.CollectedAt) {
		d.CollectedAt = other.CollectedAt
	}
	d.Nodes += other.Nodes
	mergeMap(&d.Notification, other.Notification)
	mergeMap(&d.Audit, other.Audit)
	mergeMap(&d.Logs, other.Logs)
	if other.Spill != nil {
		if d.Spill == nil {
			d.Spill = &TargetSpillStats{}
		}
		d.Spill.Bytes += other.Spill.Bytes
		d.Spill.Files += other.Spill.Files
	}
	addMap(&d.LogVolume, other.LogVolume)
}
