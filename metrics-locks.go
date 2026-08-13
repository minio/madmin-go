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
type LockMetrics struct {
	CollectedAt time.Time `json:"collected"`

	Nodes int `json:"nodes"`

	// Resources counts distinct resources with a lock held, not locks -- one resource
	// can carry many readers -- so it is not comparable with Purge's reader and
	// writer counts.
	Resources int64 `json:"resources,omitempty"`

	// Waiting is goroutines blocked on acquisition; Rejected is acquisitions refused
	// at the wait limit. Rising Rejected means contention has become a throughput
	// problem.
	Waiting  int64  `json:"waiting,omitempty"`
	Rejected uint64 `json:"rejected,omitempty"`

	// ExpiredTotal is a monotonic count of locks reclaimed on lease expiry, meaning a
	// holder died or was partitioned away.
	ExpiredTotal uint64 `json:"expired_total,omitempty"`

	// QuorumLost counts locks force-released after a refresh missed quorum: this node
	// giving up on a lock it holds, as opposed to reclaiming one a peer abandoned.
	QuorumLost uint64 `json:"quorum_lost,omitempty"`

	// Purge is sampled by the periodic cleanup pass rather than read live, so it has
	// its own timestamp and Readers+Writers does not reconcile with Resources.
	Purge *LockPurgeStats `json:"purge,omitempty"`
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
	l.Rejected += other.Rejected
	l.ExpiredTotal += other.ExpiredTotal
	l.QuorumLost += other.QuorumLost
	if other.Purge != nil {
		if l.Purge == nil {
			l.Purge = new(LockPurgeStats)
		}
		l.Purge.Merge(other.Purge)
	}
}

// LockPurgeStats is what the periodic lock-cleanup pass observed. Grouped because
// every field shares one sample time.
type LockPurgeStats struct {
	// SampledAt is when the pass ran, merged oldest-wins so a node whose cleanup has
	// stalled is what the reader sees.
	SampledAt time.Time `json:"sampled_at,omitempty"`

	Readers int64 `json:"readers,omitempty"`
	Writers int64 `json:"writers,omitempty"`

	// Expired is what the last pass reclaimed, not a running total.
	Expired int64 `json:"expired,omitempty"`

	// OldestHeldAt is when the oldest still-held lock was acquired, or zero if none.
	// A timestamp rather than an age, merged oldest-wins so one stuck lock stays
	// visible.
	OldestHeldAt time.Time `json:"oldest_held_at,omitempty"`
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
