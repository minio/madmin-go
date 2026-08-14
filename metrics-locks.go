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

// LockMetrics is the state of this cluster's distributed locking.
//
// Three time bases live here, and which one a field uses is stated on the field.
// No value is cumulative since process start: a lifetime counter needs two
// collections before it says anything, and resets invisibly on restart.
//
//   - Instantaneous, read live at collection: Resources, Waiting.
//   - Sliding last minute, from in-memory accumulators, never persisted:
//     Acquire, Held, Release, AcquireFailed, TimedOut, Conflicts, Canceled,
//     ServerLatency.
//   - Segmented windows, persisted across restarts: LastHour, LastDay. These are
//     also the only place the rarer events appear -- rejections, lease expiries
//     and quorum losses -- because they are far too infrequent to show up in a
//     one-minute view.
type LockMetrics struct {
	CollectedAt time.Time `json:"collected"`

	Nodes int `json:"nodes"`

	// Resources counts distinct resources with a lock held right now, not locks --
	// one resource can carry many readers -- so it is not comparable with Purge's
	// reader and writer counts.
	Resources int64 `json:"resources,omitempty"`

	// Waiting is goroutines blocked on acquisition right now.
	Waiting int64 `json:"waiting,omitempty"`

	// Acquire is how long callers on this node waited before being granted a lock,
	// over the last minute -- the question a lock subsystem exists to answer. It
	// spans the whole acquisition, so it includes every retry and distributed
	// round-trip. Attempts that never got the lock are in AcquireFailed instead, so
	// AccTime/Count is the mean wait of a caller that did get it.
	Acquire LockOpStats `json:"acquire,omitzero"`

	// AcquireFailed is the attempts that never got the lock, over the last minute.
	// Count is how many and AccTime is the waiting they did before giving up, which
	// is work the cluster paid for and threw away. Why they failed is split across
	// TimedOut, Conflicts and Canceled.
	AcquireFailed LockOpStats `json:"acquire_failed,omitzero"`

	// TimedOut, Conflicts and Canceled break AcquireFailed down by cause, over the
	// same last minute: gave up waiting, refused at once because the caller would
	// not block, and ended because the caller's own context did. Only Conflicts is
	// routine under load. They sum to AcquireFailed's total count.
	TimedOut  uint64 `json:"timed_out,omitempty"`
	Conflicts uint64 `json:"conflicts,omitempty"`
	Canceled  uint64 `json:"canceled,omitempty"`

	// Held is how long a granted lock stayed held before its holder released it,
	// over the last minute. This is the input side of contention: a lock held long
	// makes every waiter wait. Counted once per lock on the node that took it, so
	// unlike Resources it does not scale with the number of servers a lock is
	// replicated to.
	Held LockOpStats `json:"held,omitzero"`

	// Release is how long releasing took, measured from the moment the hold ended,
	// so Held and Release do not overlap and together span the lock's life. Near
	// zero for plain distributed locks, which release fire-and-forget; real for the
	// bulk and coalesced paths, which wait for their release RPCs.
	Release LockOpStats `json:"release,omitzero"`

	// ServerLatency is what this node's own lock servers cost to talk to, as opposed
	// to what its callers waited.
	ServerLatency LockServerLatency `json:"server_latency,omitzero"`

	// Purge is sampled by the periodic cleanup pass rather than read live, so it has
	// its own timestamp and Readers+Writers does not reconcile with Resources.
	Purge *LockPurgeStats `json:"purge,omitempty"`

	// LastDay is 96 x 15-minute segments, requested with the DayStats flag.
	LastDay *SegmentedLockMetrics `json:"lastDay,omitempty"`

	// LastHour is 60 x 1-minute segments, requested with the HourStats flag.
	LastHour *SegmentedLockMetrics `json:"lastHour,omitempty"`
}

// Merge other into l.
func (l *LockMetrics) Merge(other *LockMetrics) {
	if other == nil {
		return
	}
	l.Nodes += other.Nodes
	if l.CollectedAt.Before(other.CollectedAt) {
		l.CollectedAt = other.CollectedAt
	}
	l.Resources += other.Resources
	l.Waiting += other.Waiting
	l.TimedOut += other.TimedOut
	l.Conflicts += other.Conflicts
	l.Canceled += other.Canceled
	l.Acquire.Merge(other.Acquire)
	l.AcquireFailed.Merge(other.AcquireFailed)
	l.Held.Merge(other.Held)
	l.Release.Merge(other.Release)
	l.ServerLatency.Merge(other.ServerLatency)
	if other.Purge != nil {
		if l.Purge == nil {
			l.Purge = new(LockPurgeStats)
		}
		l.Purge.Merge(other.Purge)
	}
	if other.LastDay != nil {
		if l.LastDay == nil {
			l.LastDay = new(SegmentedLockMetrics)
		}
		l.LastDay.Add(other.LastDay)
	}
	if other.LastHour != nil {
		if l.LastHour == nil {
			l.LastHour = new(SegmentedLockMetrics)
		}
		l.LastHour.Add(other.LastHour)
	}
}

// LockPurgeStats is what the periodic lock-cleanup pass observed. Grouped because
// every field shares one sample time.
type LockPurgeStats struct {
	// SampledAt is when the pass ran, merged oldest-wins so a node whose cleanup has
	// stalled is what the reader sees.
	SampledAt time.Time `json:"sampled_at,omitzero"`

	Readers int64 `json:"readers,omitempty"`
	Writers int64 `json:"writers,omitempty"`

	// Expired is what the last pass reclaimed, not a running total.
	Expired int64 `json:"expired,omitempty"`

	// OldestHeldAt is when the oldest still-held lock was acquired, or zero if none.
	// A timestamp rather than an age, merged oldest-wins so one stuck lock stays
	// visible.
	OldestHeldAt time.Time `json:"oldest_held_at,omitzero"`
}

// Merge other into p.
func (p *LockPurgeStats) Merge(other *LockPurgeStats) {
	if other == nil {
		return
	}
	p.Readers += other.Readers
	p.Writers += other.Writers
	p.Expired += other.Expired
	if !other.SampledAt.IsZero() && (p.SampledAt.IsZero() || other.SampledAt.Before(p.SampledAt)) {
		p.SampledAt = other.SampledAt
	}
	if !other.OldestHeldAt.IsZero() && (p.OldestHeldAt.IsZero() || other.OldestHeldAt.Before(p.OldestHeldAt)) {
		p.OldestHeldAt = other.OldestHeldAt
	}
}

// LockOpStats is one lock timing split by lock kind. Readers share a resource
// while writers exclude each other, so they queue for different reasons and a
// combined figure hides which of the two is slow.
type LockOpStats struct {
	Read  TimedAction `json:"read,omitempty"`
	Write TimedAction `json:"write,omitempty"`
}

// IsZero reports whether nothing has been recorded, for omitzero.
func (s LockOpStats) IsZero() bool { return s.Read.Count == 0 && s.Write.Count == 0 }

// Count is the combined number of operations recorded.
func (s LockOpStats) Count() uint64 { return s.Read.Count + s.Write.Count }

// Merge other into s.
func (s *LockOpStats) Merge(other LockOpStats) {
	s.Read.Merge(other.Read)
	s.Write.Merge(other.Write)
}

// LockServerLatency is how long this node's own lock servers took to service lock
// RPCs over the last minute, across every operation type. It is the locker's own
// cost; what a caller waited for a lock is LockMetrics.Acquire.
//
// An average and a maximum rather than a TimedAction: the underlying accumulator
// keeps one sample per one-second slot and discards the rest, so a sample count
// would be fabricated.
type LockServerLatency struct {
	// N is contributing lock servers, so AvgSumNanos divided by N is the mean.
	N int `json:"n,omitempty"`

	AvgSumNanos uint64 `json:"avg_sum_nanos,omitempty"`
	MaxNanos    uint64 `json:"max_nanos,omitempty"`
}

// IsZero reports whether nothing has been recorded, for omitzero.
func (l LockServerLatency) IsZero() bool { return l.N == 0 }

// Merge other into l. Averages sum against N; the maximum takes the worst server,
// so one slow lock server stays visible.
func (l *LockServerLatency) Merge(other LockServerLatency) {
	if other.N == 0 {
		return
	}
	l.N += other.N
	l.AvgSumNanos += other.AvgSumNanos
	l.MaxNanos = max(l.MaxNanos, other.MaxNanos)
}

// SegmentedLockMetrics is a time-segmented view of locking activity.
type SegmentedLockMetrics = Segmented[LockSegment, *LockSegment]

// LockSegment is locking activity over one time segment.
//
// Every field is a plain sum over the segment, so segments merge across nodes and
// rescale to a coarser interval by addition. Reads and writes are combined: the
// split belongs to the live view, where it is one object rather than 156.
//
// Instantaneous gauges are absent -- resource and waiter counts are samples, and
// summing samples over a segment means nothing.
type LockSegment struct {
	// AcquireCount is locks granted in the segment and AcquireNanos their summed
	// wait, so the mean wait is AcquireNanos/AcquireCount and the grant rate is
	// AcquireCount over the interval.
	AcquireCount uint64 `json:"acquire_count,omitempty"`
	AcquireNanos uint64 `json:"acquire_ns,omitempty"`

	// AcquireFailed is attempts that never got the lock, all causes, and
	// AcquireFailedNanos the waiting they wasted.
	AcquireFailed      uint64 `json:"acquire_failed,omitempty"`
	AcquireFailedNanos uint64 `json:"acquire_failed_ns,omitempty"`

	HeldCount uint64 `json:"held_count,omitempty"`
	HeldNanos uint64 `json:"held_ns,omitempty"`

	ReleaseCount uint64 `json:"release_count,omitempty"`
	ReleaseNanos uint64 `json:"release_ns,omitempty"`

	// Rejected is acquisitions refused at a lock server's wait limit, Expired is
	// locks reclaimed on lease expiry -- a holder died or was partitioned away --
	// and QuorumLost is locks this node gave up after a refresh missed quorum. All
	// three are rare enough that a segmented history is the only place they show.
	//
	// The first two are counted once per shard, not once per server in it, so they
	// are on the same scale as the live Purge block and as AcquireFailed rather than
	// multiplied by the shard's drive count.
	Rejected   uint64 `json:"rejected,omitempty"`
	Expired    uint64 `json:"expired,omitempty"`
	QuorumLost uint64 `json:"quorum_lost,omitempty"`

	// N is contributing nodes.
	N int `json:"n"`
}

// Add other into s.
func (s *LockSegment) Add(other *LockSegment) {
	if other == nil {
		return
	}
	s.AcquireCount += other.AcquireCount
	s.AcquireNanos += other.AcquireNanos
	s.AcquireFailed += other.AcquireFailed
	s.AcquireFailedNanos += other.AcquireFailedNanos
	s.HeldCount += other.HeldCount
	s.HeldNanos += other.HeldNanos
	s.ReleaseCount += other.ReleaseCount
	s.ReleaseNanos += other.ReleaseNanos
	s.Rejected += other.Rejected
	s.Expired += other.Expired
	s.QuorumLost += other.QuorumLost
	s.N += other.N
}
