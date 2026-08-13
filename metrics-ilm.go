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

// ILMMetrics is the state of the lifecycle worker pools: what is queued, how fast
// it drains, and what is failing.
//
// Per-action object and version counts come from the scanner; bytes moved to or
// from a tier come from WarmTierMetrics. The error counts here are neither: a
// task can fail without reaching a tier, and a tier write can fail without
// failing the task.
type ILMMetrics struct {
	CollectedAt time.Time `json:"collected"`

	Nodes int `json:"nodes"`

	// Transition and Expiry are nil until the object layer initialises them.
	Transition *ILMQueueStats `json:"transition,omitempty"`
	Expiry     *ILMQueueStats `json:"expiry,omitempty"`

	// Restore is object restoration from a tier. It has no queue: it runs on the
	// caller's goroutine.
	Restore *ILMRestoreStats `json:"restore,omitempty"`

	LastDay *SegmentedILMMetrics `json:"lastDay,omitempty"`

	LastHour *SegmentedILMMetrics `json:"lastHour,omitempty"`
}

// Merge other into m.
func (m *ILMMetrics) Merge(other *ILMMetrics) {
	if other == nil {
		return
	}
	m.Nodes += other.Nodes
	if m.CollectedAt.Before(other.CollectedAt) {
		m.CollectedAt = other.CollectedAt
	}
	if other.Transition != nil {
		if m.Transition == nil {
			m.Transition = new(ILMQueueStats)
		}
		m.Transition.Merge(other.Transition)
	}
	if other.Expiry != nil {
		if m.Expiry == nil {
			m.Expiry = new(ILMQueueStats)
		}
		m.Expiry.Merge(other.Expiry)
	}
	if other.Restore != nil {
		if m.Restore == nil {
			m.Restore = new(ILMRestoreStats)
		}
		m.Restore.Merge(other.Restore)
	}
	if other.LastDay != nil {
		if m.LastDay == nil {
			m.LastDay = new(SegmentedILMMetrics)
		}
		m.LastDay.Add(other.LastDay)
	}
	if other.LastHour != nil {
		if m.LastHour == nil {
			m.LastHour = new(SegmentedILMMetrics)
		}
		m.LastHour.Add(other.LastHour)
	}
}

// ILMQueueStats is one lifecycle worker pool.
type ILMQueueStats struct {
	// Queued/Capacity is saturation at any scope. Capacity is not constant -- the
	// pools are resized at runtime -- so it is read per collect.
	Queued   int64 `json:"queued,omitempty"`
	Capacity int64 `json:"capacity,omitempty"`

	Workers int64 `json:"workers,omitempty"`

	// Active is tasks executing, so Active/Workers is pool busyness. Queued counts
	// only what is still waiting.
	Active int64 `json:"active,omitempty"`

	// Missed is tasks dropped because no worker was free. Unlike a full queue, which
	// is transient backpressure, missed work is discarded and those objects wait for
	// the next scan.
	Missed uint64 `json:"missed,omitempty"`

	// Tasks is completed tasks. Service time is measured from enqueue, so it includes
	// queue wait; Workers and Active explain it.
	Tasks TimedAction `json:"tasks,omitempty"`

	// Errors is keyed by failure class. Summing gives total failures.
	Errors map[string]uint64 `json:"errors,omitempty"`

	// HeadQueuedAt is the enqueue time of the most recently dequeued task: a strict
	// upper bound on the age of the queue head, which a channel cannot expose
	// directly. The lag is now - HeadQueuedAt, derived by the receiver.
	//
	// With Queued it separates throughput-bound from wedged: a deep queue with a
	// recent head is draining, with an old head is stuck. Merged oldest-wins so one
	// wedged pool stays visible. Zero means nothing has been dequeued.
	HeadQueuedAt time.Time `json:"head_queued_at,omitempty"`
}

// Merge other into q.
func (q *ILMQueueStats) Merge(other *ILMQueueStats) {
	if other == nil {
		return
	}
	q.Queued += other.Queued
	q.Capacity += other.Capacity
	q.Workers += other.Workers
	q.Active += other.Active
	q.Missed += other.Missed
	q.Tasks.Merge(other.Tasks)
	addMap(&q.Errors, other.Errors)
	// Oldest wins: one wedged pool must stay visible.
	if !other.HeadQueuedAt.IsZero() && (q.HeadQueuedAt.IsZero() || other.HeadQueuedAt.Before(q.HeadQueuedAt)) {
		q.HeadQueuedAt = other.HeadQueuedAt
	}
}

// ILMRestoreStats is object restoration from a remote tier.
type ILMRestoreStats struct {
	// Restores carries count, time and bytes, so mean duration and throughput are
	// derivable.
	Restores TimedAction `json:"restores,omitempty"`

	// Active is restorations in progress.
	Active int64 `json:"active,omitempty"`

	Errors map[string]uint64 `json:"errors,omitempty"`
}

// Merge other into r.
func (r *ILMRestoreStats) Merge(other *ILMRestoreStats) {
	if other == nil {
		return
	}
	r.Restores.Merge(other.Restores)
	r.Active += other.Active
	addMap(&r.Errors, other.Errors)
}

// SegmentedILMMetrics is a time-segmented view of lifecycle activity.
type SegmentedILMMetrics = Segmented[ILMSegment, *ILMSegment]

// ILMSegment is lifecycle activity over one time segment.
//
// Queue depth and worker counts are absent: they are instantaneous gauges, and a
// sum of samples across a segment means nothing.
type ILMSegment struct {
	TransitionTasks  uint64 `json:"transition_tasks,omitempty"`
	TransitionNanos  uint64 `json:"transition_nanos,omitempty"`
	TransitionErrors uint64 `json:"transition_errors,omitempty"`
	TransitionMissed uint64 `json:"transition_missed,omitempty"`

	ExpiryTasks  uint64 `json:"expiry_tasks,omitempty"`
	ExpiryNanos  uint64 `json:"expiry_nanos,omitempty"`
	ExpiryErrors uint64 `json:"expiry_errors,omitempty"`
	ExpiryMissed uint64 `json:"expiry_missed,omitempty"`

	RestoreCount  uint64 `json:"restore_count,omitempty"`
	RestoreBytes  uint64 `json:"restore_bytes,omitempty"`
	RestoreNanos  uint64 `json:"restore_nanos,omitempty"`
	RestoreErrors uint64 `json:"restore_errors,omitempty"`

	// N is contributing nodes.
	N int `json:"n"`
}

// Add other to s for the Segmenter interface.
func (s *ILMSegment) Add(other *ILMSegment) {
	if other == nil {
		return
	}
	s.TransitionTasks += other.TransitionTasks
	s.TransitionNanos += other.TransitionNanos
	s.TransitionErrors += other.TransitionErrors
	s.TransitionMissed += other.TransitionMissed
	s.ExpiryTasks += other.ExpiryTasks
	s.ExpiryNanos += other.ExpiryNanos
	s.ExpiryErrors += other.ExpiryErrors
	s.ExpiryMissed += other.ExpiryMissed
	s.RestoreCount += other.RestoreCount
	s.RestoreBytes += other.RestoreBytes
	s.RestoreNanos += other.RestoreNanos
	s.RestoreErrors += other.RestoreErrors
	s.N += other.N
}
