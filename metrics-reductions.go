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

// Reduction classes for the realtime-metrics wire format.
//
// RealtimeMetrics.Aggregated is produced by merging every node's Metrics,
// while RealtimeMetrics.Merge overwrites ByHost[host] so per-host entries stay
// pristine. Merge is therefore the only definition of "cluster view" this API
// has, and a field's merge behaviour is its meaning. Every value belongs to
// exactly one of six classes:
//
//  1. plain-sum -- cumulative counters and gauges that count things (sockets,
//     threads, queued items, bytes). Merge is "+=". Counter versus gauge
//     matters for interpretation, not for reduction.
//     Precedent: APIStats.Merge, MemInfo.Merge.
//
//  2. TimedAction -- any latency or throughput accumulator. Carries Count +
//     AccTime + MinTime + MaxTime + Bytes, so one summable value yields the
//     mean (AccTime/Count), the tails, and the rate. Never ship a
//     server-computed average beside one. Precedent: TimedAction.Merge.
//
//  3. enum-map -- a value whose domain is a bounded enumeration, carried as
//     map[value]count and summed by key. One bounded map yields the
//     distribution, the mean and the outliers, and absorbs new enum values
//     with no wire change. Use it for link speed, duplex, operational state,
//     MTU, TCP connection state, thread scheduling state, error categories,
//     drop reasons and credential-vending outcomes. Enum maps are never
//     time-segmented: a map inside 156 segments is the one shape the segment
//     budget cannot absorb. Precedent: CPUByModel, PowerSourceCounts.
//
//  4. sum+N+max gauge -- a genuinely continuous, non-summable gauge (a stall
//     percentage, an age in seconds, a queue occupancy), carried as
//     GaugeStats. Sum/N is the mean and Max is the worst single sample in
//     scope, so the ratio between them separates "one bad node" from "the
//     whole cluster" even when the caller did not request MetricsByHost.
//     Precedent: CPUMetrics' PowerNodes/TotalWatts/MaxNodeWatts trio.
//
//  5. cluster-replicated singleton -- state every node holds identically (an
//     IAM cache inventory, a catalog count). Merge takes the copy with the
//     later sample timestamp and NEVER sums: summing multiplies every count by
//     the node count. Always behind a pointer, so nil means "no node
//     reported it". Precedent: SiteResyncMetrics.Merge.
//
//  6. leader-owned -- state exactly one node owns, where which node that is
//     changes on failover and a demoted leader keeps stale totals in memory.
//     Merge selects per key by a progress field and never sums.
//     Precedent: BatchJobMetrics.Merge.
//
// Two rules keep Aggregated correct:
//
// A struct never mixes classes. Anything that does not reduce with "+=" goes
// in its own pointer sub-struct with its own documented Merge, or is an
// explicitly named member of a class-4 trio (*Sum / *Min / *Max / N), or is a
// non-numeric identity or config echo merged with max or first-wins. Numeric
// class-5 and class-6 values are always their own sub-struct, no exceptions:
// that is the one mix that fails silently, because a summed replicated count
// looks plausible and is wrong by a factor of the node count.
//
// Never send a value the receiver can compute from other values on the wire:
// no averages, no percentages, no ETAs, no elapsed times beside two
// timestamps, no totals beside a complete enum-map.
//
// Segment structs (ProcessSegment, MemSegment, PowerSegment, ...) are the one
// carve-out for the first rule: a segment is a flat record whose per-field
// reduction is encoded in the field name, and whose single Add is both the
// within-bucket time collapse and the across-host merge.
//
// One corollary worth stating: a class-4 gauge carries its own N rather than
// reusing the section's node count, because a gauge can be absent on a host
// where the section is present (PSI and buddyinfo are Linux-only) and zero
// must distinguish "no data" from "no stall".

// GaugeStats carries a continuous, non-summable gauge as sum + sample count +
// extremes. The unit is documented by the field that holds it
// (OldestHeldSecs, QueuedAgeSecs, ...) rather than by this type.
//
// Min is populated only where the low side is actionable, and shares
// TimedAction.MinTime's limitation: with N > 0 a Min of 0 cannot be
// distinguished from "not tracked". That is accepted for consistency with
// TimedAction and PowerSegment rather than making this the only
// optional-pointer numeric on the wire.
type GaugeStats struct {
	N   int     `json:"n,omitempty"`   // samples contributing to Sum
	Sum float64 `json:"sum,omitempty"` // the mean is Sum/N
	Min float64 `json:"min,omitempty"`
	Max float64 `json:"max,omitempty"`
}

// Add other into g. Satisfies Segmenter so a GaugeStats can be time
// segmented.
//
// Both extremes use N == 0 as the unset sentinel, matching
// TimedAction.Merge and PowerSegment.Add. Max is guarded as well as Min so
// the zero value is an unconditional identity for Add -- max(0, v) is not v
// for a negative v, which would otherwise break internal/windowed's Adder
// contract for any gauge that can go below zero.
func (g *GaugeStats) Add(other *GaugeStats) {
	if other == nil || other.N == 0 {
		return
	}
	if g.N == 0 {
		g.Min, g.Max = other.Min, other.Max
	} else {
		g.Min = min(g.Min, other.Min)
		g.Max = max(g.Max, other.Max)
	}
	g.Sum += other.Sum
	g.N += other.N
}

// Mean returns Sum/N, or 0 when no samples contributed.
func (g GaugeStats) Mean() float64 {
	if g.N == 0 {
		return 0
	}
	return g.Sum / float64(g.N)
}

// SegmentedGauges are time segmented continuous gauges.
type SegmentedGauges = Segmented[GaugeStats, *GaugeStats]

// addMap accumulates src into *dst by key, allocating dst on first use.
// This is the class enum-map reduction.
func addMap[K comparable, V addable](dst *map[K]V, src map[K]V) {
	if len(src) == 0 {
		return
	}
	if *dst == nil {
		*dst = make(map[K]V, len(src))
	}
	for k, v := range src {
		(*dst)[k] += v
	}
}

// mergeMap accumulates src into *dst by key using PV.Add, for maps whose
// values are themselves mergeable (TimedAction, GaugeStats, ...).
func mergeMap[K comparable, V any, PV interface {
	*V
	Add(*V)
}](dst *map[K]V, src map[K]V,
) {
	if len(src) == 0 {
		return
	}
	if *dst == nil {
		*dst = make(map[K]V, len(src))
	}
	for k, v := range src {
		cur := (*dst)[k]
		PV(&cur).Add(&v)
		(*dst)[k] = cur
	}
}

// addOptCounter accumulates an availability-optional counter: nil in src
// leaves dst untouched, nil in dst adopts src's value. Used for kernel
// counters a given kernel may not expose, where nil must not read as zero.
func addOptCounter(dst **uint64, src *uint64) {
	if src == nil {
		return
	}
	if *dst == nil {
		v := *src
		*dst = &v
		return
	}
	**dst += *src
}

// takeLater keeps the later of two (timestamp, message) pairs. Ties break on
// the lexicographically smaller message so Merge stays independent of merge
// order.
func takeLater(dstAt *time.Time, dstMsg *string, srcAt time.Time, srcMsg string) {
	if srcMsg == "" && srcAt.IsZero() {
		return
	}
	if dstAt.IsZero() && *dstMsg == "" {
		*dstAt, *dstMsg = srcAt, srcMsg
		return
	}
	if srcAt.After(*dstAt) || (srcAt.Equal(*dstAt) && srcMsg < *dstMsg) {
		*dstAt, *dstMsg = srcAt, srcMsg
	}
}
