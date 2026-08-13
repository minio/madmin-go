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

// Distributed-job states.
const (
	DistJobStateRunning  = "running"
	DistJobStatePaused   = "paused"
	DistJobStateComplete = "complete"
	DistJobStateFailed   = "failed"
	DistJobStateCanceled = "canceled"
)

// DistJobMetrics carries the distributed (server-pool) jobs running anywhere in
// the cluster: pool decommission today, rebalance and future pool operations on
// the same framework.
//
// Distinct from BatchJobMetrics, which covers replicate/expire/keyrotate/catalog
// batch jobs. These are server-pool operations with their own persisted state.
type DistJobMetrics struct {
	CollectedAt time.Time `json:"collected"`

	// Jobs is keyed by job ID. A map rather than a slice because Merge must
	// select per job -- the same choice BatchJobMetrics.Jobs makes.
	Jobs map[string]DistJobProgress `json:"jobs,omitempty"`
}

// DistJobProgress is one distributed job's aggregate progress plus its per-node
// breakdown.
//
// Nothing here is summed across hosts. Exactly one node runs and owns each job,
// but which node that is changes on failover and a former leader can still hold
// stale state, so Merge selects per job ID by UpdatedAt. Summing would
// double-count every object the job ever moved.
//
// Percent complete, rate and ETA are all deliberately absent: they are
// BytesDone/BytesTotal and BytesDone over the elapsed time, and computing them
// here would bake each job type's definition of its own denominator into a
// shared struct.
type DistJobProgress struct {
	ID      string      `json:"id"`
	Type    DistJobType `json:"type"`
	PoolIdx int         `json:"poolIdx,omitempty"`

	// State is one of the DistJobState constants. A single string rather than
	// separate booleans: the states are mutually exclusive, and a new one must
	// not require a new wire field.
	State string `json:"state,omitempty"`

	StartTime time.Time `json:"startTime,omitzero"`
	// UpdatedAt is when the leader last refreshed this report, and is the Merge
	// selector. Elapsed time is UpdatedAt-StartTime, so no elapsed field.
	UpdatedAt time.Time `json:"updatedAt,omitzero"`
	// EndTime is set once State became terminal, so elapsed remains computable
	// for a finished job.
	EndTime time.Time `json:"endTime,omitzero"`

	// Items is whatever the job type counts -- objects for decommission,
	// versions for rebalance -- using the same vocabulary as
	// DistJobNodeStatus, so no job carries another job's structurally-zero
	// counter.
	ItemsDone    int64 `json:"itemsDone,omitempty"`
	ItemsFailed  int64 `json:"itemsFailed,omitempty"`
	ItemsSkipped int64 `json:"itemsSkipped,omitempty"`

	BytesDone   int64 `json:"bytesDone,omitempty"`
	BytesFailed int64 `json:"bytesFailed,omitempty"`
	// BytesTotal is the job type's own denominator: for decommission the pool
	// data to drain, for rebalance the bytes to move to reach the free-space
	// goal. Zero when it is not yet measurable.
	BytesTotal int64 `json:"bytesTotal,omitempty"`

	// The Unrecoverable pair is the possible-data-loss subset of the failures,
	// not an addition to them.
	ItemsUnrecoverable int64 `json:"itemsUnrecoverable,omitempty"`
	BytesUnrecoverable int64 `json:"bytesUnrecoverable,omitempty"`

	// Where the job is working right now; empty when idle or finished.
	CurrentBucket string `json:"currentBucket,omitempty"`
	CurrentObject string `json:"currentObject,omitempty"`

	// Nodes is the leader's per-node breakdown. Empty for a job type not yet
	// ported to the distributed-job framework, in which case only the aggregate
	// above is populated.
	Nodes []DistJobNodeStatus `json:"nodes,omitempty"`
}

// Merge other into d, selecting per job ID.
func (d *DistJobMetrics) Merge(other *DistJobMetrics) {
	if other == nil {
		return
	}
	if d.CollectedAt.Before(other.CollectedAt) {
		d.CollectedAt = other.CollectedAt
	}
	if len(other.Jobs) == 0 {
		return
	}
	if d.Jobs == nil {
		d.Jobs = make(map[string]DistJobProgress, len(other.Jobs))
	}
	for id, o := range other.Jobs {
		if cur, ok := d.Jobs[id]; !ok || o.fresherThan(cur) {
			d.Jobs[id] = o
		}
	}
}

// fresherThan reports whether p is the more authoritative report of the same job
// than q: the later update wins, then more progress. The trailing comparisons
// exist only to make the selection a strict total order, so Merge is independent
// of merge order even when two leaders report identically.
func (p DistJobProgress) fresherThan(q DistJobProgress) bool {
	if !p.UpdatedAt.Equal(q.UpdatedAt) {
		return p.UpdatedAt.After(q.UpdatedAt)
	}
	if p.ItemsDone != q.ItemsDone {
		return p.ItemsDone > q.ItemsDone
	}
	if p.BytesDone != q.BytesDone {
		return p.BytesDone > q.BytesDone
	}
	return p.State < q.State
}
