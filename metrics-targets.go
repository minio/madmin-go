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
//
// Nothing here is cumulative since process start. The per-target time bases are
// stated on TargetMetrics; log volume arrives as the two persisted segmented
// windows below.
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

	// Log lines emitted, by severity, over a segmented window.
	//
	// They are on the section rather than on a target because a line is counted
	// once on emission regardless of how many targets it fans out to -- and
	// because it is the only signal here that still works with no target
	// configured at all.
	//
	// LogVolumeLastDay is 96 x 15-minute segments, requested with the DayStats
	// flag; LogVolumeLastHour is 60 x 1-minute segments, requested with the
	// HourStats flag.
	LogVolumeLastDay  *SegmentedLogVolume `json:"logVolumeLastDay,omitempty"`
	LogVolumeLastHour *SegmentedLogVolume `json:"logVolumeLastHour,omitempty"`
}

// TargetSpillStats is the disk overflow store for a delivery queue.
type TargetSpillStats struct {
	Bytes int64 `json:"bytes,omitempty"`
	Files int64 `json:"files,omitempty"`
}

// TargetMetrics is one delivery target's state in this scope.
//
// Four time bases live here, and which one a field uses is stated on the field.
// No value is cumulative since process start: a lifetime counter needs two
// collections before it says anything, and resets invisibly on restart.
//
//   - Identity and configuration, echoed identically by every node: Subsystem,
//     Name, Type, Endpoint.
//   - Instantaneous, read live at collection: N, NodesOnline, NodesChecking,
//     QueueLength, QueueCapacity, Inflight, Workers.
//   - Sliding last minute, from in-memory accumulators, never persisted:
//     LastMinute.
//   - Segmented windows, persisted across restarts: LastHour, LastDay. Delivered
//     events, requests, failures and dropped events live only there, because a
//     one-minute view never catches events that arrive in bursts hours apart.
//
// LastError sits outside all four: it is the single most recent failure in
// scope, kept with its own timestamp.
//
// Every numeric field sums across hosts except LastMinute, which merges as a
// TimedAction, and the windows, which merge segment-wise. A per-node outlier --
// one node whose queue is full while five are idle -- shows up as a ByHost entry,
// and QueueLength against QueueCapacity is a correct saturation figure at either
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

	// Backpressure gauges.
	QueueLength   int64 `json:"queue_length,omitempty"`
	QueueCapacity int64 `json:"queue_capacity,omitempty"`
	Inflight      int64 `json:"inflight,omitempty"`
	Workers       int64 `json:"workers,omitempty"`

	// LastMinute is delivery over the last ~60s.
	//
	// Count is *requests*, not events: a batching target covers up to its batch
	// size per request, so the batch factor is a window's Events/Requests rather
	// than anything derivable here. The mean latency is AccTime/Count and the
	// delivery rate is Count/60 -- neither is sent. Bytes is populated only by the
	// writer path that knows the encoded payload size.
	LastMinute TimedAction `json:"last_minute"`

	// LastError is the most recent delivery failure anywhere in scope, kept with
	// its timestamp so Merge is order-independent: later wins, and equal
	// timestamps break on the lexicographically smaller message.
	LastError     string    `json:"last_error,omitempty"`
	LastErrorTime time.Time `json:"last_error_time,omitzero"`

	// LastDay is 96 x 15-minute segments, requested with the DayStats flag.
	LastDay *SegmentedTargetMetrics `json:"lastDay,omitempty"`

	// LastHour is 60 x 1-minute segments, requested with the HourStats flag.
	LastHour *SegmentedTargetMetrics `json:"lastHour,omitempty"`
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
	t.QueueLength += other.QueueLength
	t.QueueCapacity += other.QueueCapacity
	t.Inflight += other.Inflight
	t.Workers += other.Workers
	t.LastMinute.Merge(other.LastMinute)
	takeLater(&t.LastErrorTime, &t.LastError, other.LastErrorTime, other.LastError)
	// Presence survives the merge: a window that was requested and came back with
	// no completed segment is kept non-nil, so a reader can tell it apart from one
	// that was never asked for.
	if o := other.LastDay; o != nil {
		if t.LastDay == nil {
			t.LastDay = new(SegmentedTargetMetrics)
		}
		t.LastDay.Add(o)
	}
	if o := other.LastHour; o != nil {
		if t.LastHour == nil {
			t.LastHour = new(SegmentedTargetMetrics)
		}
		t.LastHour.Add(o)
	}
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
	// Presence survives the merge; see TargetMetrics.Add.
	if o := other.LogVolumeLastDay; o != nil {
		if d.LogVolumeLastDay == nil {
			d.LogVolumeLastDay = new(SegmentedLogVolume)
		}
		d.LogVolumeLastDay.Add(o)
	}
	if o := other.LogVolumeLastHour; o != nil {
		if d.LogVolumeLastHour == nil {
			d.LogVolumeLastHour = new(SegmentedLogVolume)
		}
		d.LogVolumeLastHour.Add(o)
	}
}

// SegmentedTargetMetrics is a time-segmented view of one target's delivery flow.
type SegmentedTargetMetrics = Segmented[TargetSegment, *TargetSegment]

// TargetSegment is one delivery target's flow over one time segment.
//
// Every field is a plain sum over the segment, so segments merge across nodes and
// rescale to a coarser interval by addition. Reading a window rolls it forward, so
// a field whose additive identity is not zero would be corrupted by the read.
//
// Queue depth, in-flight and worker counts are absent: they are instantaneous
// gauges, and a sum of samples across a segment means nothing.
type TargetSegment struct {
	// Events is events accepted for delivery and Requests the attempts they were
	// batched into, so Events/Requests is the batch factor. RequestNanos is the
	// summed delivery latency, so RequestNanos/Requests is the mean.
	Events       uint64 `json:"events,omitempty"`
	Requests     uint64 `json:"requests,omitempty"`
	RequestNanos uint64 `json:"request_nanos,omitempty"`

	// WriterErrors is every delivery attempt that failed to write. Attempts are
	// retried, so one event can contribute several.
	//
	// There is deliberately no separate failed-request count beside it: the two
	// move together on every non-batching path, and a batching target increments
	// only this one, so a second field would be the same value twice and would read
	// as zero for Kafka.
	WriterErrors uint64 `json:"writer_errors,omitempty"`

	// The reasons an event was discarded, as fixed fields rather than a map so the
	// segment stays a flat record of sums. They sum to the segment's dropped-event
	// count, so there is no separate total; DroppedOther keeps that sum exact when
	// the server cannot classify a drop.
	DroppedQueueFull        uint64 `json:"dropped_queue_full,omitempty"`
	DroppedRetriesExhausted uint64 `json:"dropped_retries_exhausted,omitempty"`
	DroppedShutdown         uint64 `json:"dropped_shutdown,omitempty"`
	DroppedOther            uint64 `json:"dropped_other,omitempty"`

	// N is contributing nodes.
	N int `json:"n"`
}

// Add other to s for the Segmenter interface.
func (s *TargetSegment) Add(other *TargetSegment) {
	if other == nil {
		return
	}
	s.Events += other.Events
	s.Requests += other.Requests
	s.RequestNanos += other.RequestNanos
	s.WriterErrors += other.WriterErrors
	s.DroppedQueueFull += other.DroppedQueueFull
	s.DroppedRetriesExhausted += other.DroppedRetriesExhausted
	s.DroppedShutdown += other.DroppedShutdown
	s.DroppedOther += other.DroppedOther
	s.N += other.N
}

// SegmentedLogVolume is a time-segmented view of log lines emitted.
type SegmentedLogVolume = Segmented[LogVolumeSegment, *LogVolumeSegment]

// LogVolumeSegment is the log lines emitted over one time segment, by severity.
//
// Every field is a plain sum over the segment, so segments merge across nodes and
// rescale to a coarser interval by addition. Reading a window rolls it forward, so
// a field whose additive identity is not zero would be corrupted by the read.
//
// The severities are fixed fields rather than a map: they are a bounded enum, and
// a map would be repeated in all 156 segments a node carries.
//
// These count lines *emitted*, so suppression happens before the count: a burst
// reported through LogOnceIf contributes one line plus its periodic rollups.
type LogVolumeSegment struct {
	ErrorLines   uint64 `json:"error_lines,omitempty"`
	WarningLines uint64 `json:"warning_lines,omitempty"`
	FatalLines   uint64 `json:"fatal_lines,omitempty"`
	InfoLines    uint64 `json:"info_lines,omitempty"`
	EventLines   uint64 `json:"event_lines,omitempty"`

	// N is contributing nodes.
	N int `json:"n"`
}

// Add other to s for the Segmenter interface.
func (s *LogVolumeSegment) Add(other *LogVolumeSegment) {
	if other == nil {
		return
	}
	s.ErrorLines += other.ErrorLines
	s.WarningLines += other.WarningLines
	s.FatalLines += other.FatalLines
	s.InfoLines += other.InfoLines
	s.EventLines += other.EventLines
	s.N += other.N
}
