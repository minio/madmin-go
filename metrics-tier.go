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

// WarmTierMetrics is per-tier warm-storage request telemetry: operation counts,
// latency and failures against each configured remote tier.
//
// Carries no resident-inventory figure. Bytes and object counts living in a tier
// come from DataUsageInfo.TierStats, which is cluster-wide already. Every field
// here is node-local and sums.
type WarmTierMetrics struct {
	CollectedAt time.Time `json:"collected"`

	Nodes int `json:"nodes"`

	// Tiers is keyed by configured tier name.
	//
	// A configured tier that has never been used is present with zero counters and a
	// zero LastSuccess, which is how "configured but idle" stays distinct from "not
	// configured".
	Tiers map[string]WarmTierStat `json:"tiers,omitempty"`

	// LastDay is 24h in 15-minute segments, LastHour 1h in 1-minute segments. Both
	// are summed across tiers.
	LastDay *SegmentedWarmTierMetrics `json:"lastDay,omitempty"`

	LastHour *SegmentedWarmTierMetrics `json:"lastHour,omitempty"`
}

// Merge other into t.
func (t *WarmTierMetrics) Merge(other *WarmTierMetrics) {
	if other == nil {
		return
	}
	t.Nodes += other.Nodes
	if t.CollectedAt.Before(other.CollectedAt) {
		t.CollectedAt = other.CollectedAt
	}
	mergeMap(&t.Tiers, other.Tiers)
	if other.LastDay != nil {
		if t.LastDay == nil {
			t.LastDay = new(SegmentedWarmTierMetrics)
		}
		t.LastDay.Add(other.LastDay)
	}
	if other.LastHour != nil {
		if t.LastHour == nil {
			t.LastHour = new(SegmentedWarmTierMetrics)
		}
		t.LastHour.Add(other.LastHour)
	}
}

// WarmTierStat is one tier's activity.
type WarmTierStat struct {
	// N is the number of nodes reporting this tier.
	N int `json:"n"`

	// Type is the tier backend ("s3", "azure", "gcs", "minio"). A config echo: the
	// first reporting node supplies it, a later one must not blank it.
	Type string `json:"type,omitempty"`

	// Ops is keyed by operation: "put", "get", "delete". An operation never
	// performed is absent rather than zero. Open set, so a new one is a new key.
	//
	// "get" is timed to last byte and so includes the client's read time.
	Ops map[string]TimedAction `json:"ops,omitempty"`

	// GetTTFB is time to first byte: the remote call alone.
	//
	// Its Count is not redundant with Ops["get"].Count. TTFB is recorded when the
	// remote responds, the last-byte timing only when the body is closed, so the
	// difference is reads that started and never finished.
	GetTTFB TimedAction `json:"get_ttfb,omitempty"`

	// Errors is keyed by failure class: "not_found", "unreachable", "other". Open
	// set. Summing gives total failures, so no separate total is carried.
	//
	// "unreachable" means the tier itself is not answering, as opposed to a specific
	// object being gone.
	Errors map[string]uint64 `json:"errors,omitempty"`

	// InflightPut and InflightDelete are operations in progress.
	//
	// There is no InflightGet: a read's completion is signalled by closing the body,
	// and a read that fails before the body exists never signals, so the counter
	// would only ever climb.
	InflightPut    int64 `json:"inflight_put,omitempty"`
	InflightDelete int64 `json:"inflight_delete,omitempty"`

	// LastSuccess is the most recent successful operation. Together with LastError it
	// answers reachability from traffic that was going to happen anyway. A zero value
	// with zero counters means the tier has never been used.
	LastSuccess time.Time `json:"last_success,omitempty"`

	// LastError is the most recent failure, server-truncated. Both fields merge
	// latest-wins on LastErrorTime rather than summing.
	LastError     string    `json:"last_error,omitempty"`
	LastErrorTime time.Time `json:"last_error_time,omitempty"`
}

// Add other into t.
func (t *WarmTierStat) Add(other *WarmTierStat) {
	if other == nil {
		return
	}
	t.N += other.N
	if t.Type == "" {
		t.Type = other.Type
	}
	mergeMap(&t.Ops, other.Ops)
	t.GetTTFB.Merge(other.GetTTFB)
	addMap(&t.Errors, other.Errors)
	t.InflightPut += other.InflightPut
	t.InflightDelete += other.InflightDelete
	if t.LastSuccess.Before(other.LastSuccess) {
		t.LastSuccess = other.LastSuccess
	}
	takeLater(&t.LastErrorTime, &t.LastError, other.LastErrorTime, other.LastError)
}

// SegmentedWarmTierMetrics is a time-segmented view of tier activity.
type SegmentedWarmTierMetrics = Segmented[WarmTierSegment, *WarmTierSegment]

// WarmTierSegment is tier activity over one time segment. Per-tier detail is
// deliberately absent: it would multiply the payload by the tier count.
//
// Every field is a plain sum. Reading a window rolls it forward, so a field whose
// additive identity is not zero would be corrupted by the read.
type WarmTierSegment struct {
	PutCount    uint64 `json:"put_count,omitempty"`
	PutBytes    uint64 `json:"put_bytes,omitempty"`
	PutNanos    uint64 `json:"put_nanos,omitempty"`
	GetCount    uint64 `json:"get_count,omitempty"`
	GetBytes    uint64 `json:"get_bytes,omitempty"`
	GetNanos    uint64 `json:"get_nanos,omitempty"`
	DeleteCount uint64 `json:"delete_count,omitempty"`
	DeleteNanos uint64 `json:"delete_nanos,omitempty"`

	// Errors over the segment, summed across tiers: which tier failed is a
	// live-section question, when it started failing is this one.
	ErrorsNotFound    uint64 `json:"errors_not_found,omitempty"`
	ErrorsUnreachable uint64 `json:"errors_unreachable,omitempty"`
	ErrorsOther       uint64 `json:"errors_other,omitempty"`

	// N is contributing nodes.
	N int `json:"n"`
}

// Add other to s for the Segmenter interface.
func (s *WarmTierSegment) Add(other *WarmTierSegment) {
	if other == nil {
		return
	}
	s.PutCount += other.PutCount
	s.PutBytes += other.PutBytes
	s.PutNanos += other.PutNanos
	s.GetCount += other.GetCount
	s.GetBytes += other.GetBytes
	s.GetNanos += other.GetNanos
	s.DeleteCount += other.DeleteCount
	s.DeleteNanos += other.DeleteNanos
	s.ErrorsNotFound += other.ErrorsNotFound
	s.ErrorsUnreachable += other.ErrorsUnreachable
	s.ErrorsOther += other.ErrorsOther
	s.N += other.N
}
