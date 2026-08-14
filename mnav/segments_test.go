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
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package mnav

import (
	"strings"
	"testing"
	"time"

	"github.com/minio/madmin-go/v4"
)

// dupFirstTime is the start of a 97-slot quarter-hour timeline, so slot 0 and slot
// 96 are both named 10:00Z.
var dupFirstTime = time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)

const dupKey = "10:00Z"

// The newest instant owns a shared key even when the slots arrive out of order.
func TestSegmentTimeOwners(t *testing.T) {
	times := []time.Time{
		dupFirstTime.Add(24 * time.Hour), // 10:00Z, one day on
		dupFirstTime.Add(15 * time.Minute),
		dupFirstTime, // 10:00Z
	}
	owners := segmentTimeOwners(times)
	if got, want := len(owners), 2; got != want {
		t.Fatalf("owners = %v, want %d distinct keys", owners, want)
	}
	if got, want := owners[dupKey], 0; got != want {
		t.Errorf("owner of %s = %d, want slot %d (the newest instant)", dupKey, got, want)
	}
}

// dupWindow is a day window whose first and last slot collide on one key, carrying
// old in the oldest slot and new in the newest.
func dupWindow[T any, PT segPtr[T]](old, recent T) madmin.Segmented[T, PT] {
	segments := make([]T, 97)
	segments[0] = old
	segments[96] = recent
	return madmin.Segmented[T, PT]{Interval: 900, FirstTime: dupFirstTime, Segments: segments}
}

// A key is a bare wall-clock time, so a window that reaches 24 hours -- what
// Segmented.Add yields when two contributors' FirstTime differ by one interval --
// has two slots wanting one name. Every family sharing the key format must list it
// once and resolve it to the newest slot.
func TestDuplicateSegmentKeysAcrossFamilies(t *testing.T) {
	rpcDay := dupWindow[madmin.RPCStats, *madmin.RPCStats](
		madmin.RPCStats{Requests: 7, RequestTimeSecs: 0.7},
		madmin.RPCStats{Requests: 11, RequestTimeSecs: 1.1},
	)
	replDay := dupWindow[madmin.ReplicationStats, *madmin.ReplicationStats](
		madmin.ReplicationStats{Nodes: 1, Events: 7},
		madmin.ReplicationStats{Nodes: 1, Events: 11},
	)
	tableReads := make([]int64, 97)
	tableReads[0], tableReads[96] = 7, 11

	for _, tt := range []struct {
		name string
		node MetricNode
		// dup is the key both slots want, "" for the start-of-segment default.
		dup  string
		key  string
		want string
	}{
		{
			name: "tables",
			node: &tableSegmentedNode{
				seg: &madmin.SegmentedTableIO{
					IntervalSecs: 900, FirstTime: dupFirstTime, Reads: tableReads,
				},
				windowSecs: 86400, flag: madmin.MetricsDayStats,
			},
			key: "Reads", want: "11",
		},
		{
			name: "rpc handler",
			node: &RPCLastDayHandlerNode{handlerName: "storage.ReadAll", segmented: rpcDay},
			key:  "Total Requests", want: "11",
		},
		{
			name: "rpc all handlers",
			node: &RPCLastDayAllNode{rpc: &madmin.RPCMetrics{
				LastDay: map[string]madmin.SegmentedRPCMetrics{"storage.ReadAll": rpcDay},
			}},
			key: "Total Requests", want: "11",
		},
		{
			name: "replication target",
			node: NewReplicationLastDayNode("arn:target", &replDay, nil, "replication/target/last_day"),
			key:  "Total Events", want: "11",
		},
		{
			name: "replication aggregated",
			node: NewReplicationLastDayAggregatedNode(&madmin.ReplicationMetrics{
				Nodes:   1,
				Targets: map[string]madmin.ReplicationTargetStats{"arn:target": {Nodes: 1, LastDay: &replDay}},
			}, nil, "replication/last_day"),
			key: "Total Events", want: "11",
		},
		{
			name: "healing",
			node: func() MetricNode {
				w := dupWindow[madmin.HealingCounts, *madmin.HealingCounts](
					madmin.HealingCounts{Started: 7}, madmin.HealingCounts{Started: 11},
				)
				return NewHealingLastDayNode(&w, nil, "healing/last_day")
			}(),
			key: "Started", want: "11",
		},
		{
			// Named by the instant a segment ends, so its keys sit one interval on.
			name: "process",
			node: func() MetricNode {
				w := dupWindow[madmin.ProcessSegment, *madmin.ProcessSegment](
					madmin.ProcessSegment{N: 1, CPUPercent: 7}, madmin.ProcessSegment{N: 1, CPUPercent: 11},
				)
				return NewProcessLastDayNode(&w, nil, "process/last_day")
			}(),
			dup: "10:15Z", key: "CPU Usage", want: "11.00% average",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dup := tt.dup
			if dup == "" {
				dup = dupKey
			}
			names := childNames(tt.node.GetChildren())
			seen := map[string]int{}
			for _, name := range names {
				seen[name]++
				if _, err := tt.node.GetChild(name); err != nil {
					t.Errorf("GetChild(%q) = %v, want a node: everything listed must resolve", name, err)
				}
			}
			for name, n := range seen {
				if n != 1 {
					t.Errorf("child %q listed %d times in %v, want exactly once", name, n, names)
				}
			}
			if seen[dup] == 0 {
				t.Fatalf("children = %v, want the shared key %s among them", names, dup)
			}

			child, err := tt.node.GetChild(dup)
			if err != nil {
				t.Fatalf("GetChild(%q): %v", dup, err)
			}
			if got := leafValue(child.GetLeafData(), tt.key); got != tt.want {
				t.Errorf("%s = %q, want %q from the newest slot", tt.key, got, tt.want)
			}
		})
	}
}

// A row key is a UTC start and nothing else. It is what an operator greps a log
// with, so it must not drift with the reader's zone, and it must not carry an end
// time: every segment in a window is the same fixed size, stated once by the
// Coverage row, and a full span wraps the narrow column and squeezes the value.
//
// Families used to format this themselves, and cpu keyed the reader's local clock
// while its siblings keyed UTC -- the same instant under two names, and ambiguous
// across a DST fold.
func TestSegmentRowKeyIsUTCStartOnly(t *testing.T) {
	// 00:30 UTC, which is a different day in the reader's zone if that zone is far
	// enough east, so a key built from local time cannot coincidentally match.
	start := time.Date(2026, 8, 12, 0, 30, 0, 0, time.UTC)
	end := start.Add(15 * time.Minute)

	for _, tz := range []string{"UTC", "Australia/Sydney", "America/Los_Angeles"} {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			t.Skipf("zone %s unavailable: %v", tz, err)
		}
		t.Run(tz, func(t *testing.T) {
			// segmentRowKey must read the instant, not the zone it is carried in.
			got := segmentRowKey(3, start.In(loc), end.In(loc))
			if want := "03: 00:30Z"; got != want {
				t.Errorf("segmentRowKey = %q, want %q", got, want)
			}
		})
	}

	// The key and the navigation name must name the same instant, or a row cannot be
	// matched to the child that drills into it.
	if key, name := segmentRowKey(0, start, end), segmentKey(start); !strings.HasSuffix(key, name) {
		t.Errorf("row key %q does not end in the navigation name %q", key, name)
	}
}

// The helper above is only worth as much as the families using it, so this checks
// a real one end to end. CPU is the family that used to format its own key from
// the reader's local clock.
func TestCPUWindowRowKeyIsUTC(t *testing.T) {
	loc, err := time.LoadLocation("Australia/Sydney")
	if err != nil {
		t.Skipf("zone unavailable: %v", err)
	}
	start := time.Date(2026, 8, 12, 0, 30, 0, 0, time.UTC)

	nav := NewRealtimeMetricsNavigator(&madmin.RealtimeMetrics{
		Aggregated: madmin.Metrics{CPU: &madmin.CPUMetrics{
			LastHour: &madmin.SegmentedCPUMetrics{
				Interval:  60,
				FirstTime: start.In(loc),
				Segments:  []madmin.CPUSegment{{N: 1, User: 10, System: 5, Idle: 85}},
			},
		}},
	})
	node, err := nav.Navigate("cpu/last_hour")
	if err != nil {
		t.Fatalf("navigate cpu/last_hour: %v", err)
	}
	data := node.GetLeafData()
	var found bool
	for key := range data {
		if !strings.Contains(key, ":") || !strings.Contains(key, "0:30") && !strings.Contains(key, "10:30") {
			continue
		}
		found = true
		if !strings.HasSuffix(key, "00:30Z") {
			t.Errorf("row key %q, want it to end in the UTC start 00:30Z: a local-clock "+
				"key names the same instant differently for every reader", key)
		}
	}
	if !found {
		t.Errorf("no segment row found in %v", data)
	}
}

// A segment's N is one sample per node, so an _ALL entry -- which sums every
// segment in the window -- carries one per node per segment. Rendering that raw
// reported a 5-node cluster over a 60-segment hour as "312 node sample(s)", which
// reads as a node count and is off by the segment count.
func TestFormatNodeCountDividesByMergeCount(t *testing.T) {
	for _, tc := range []struct {
		name     string
		n        int
		segments int
		want     string
	}{
		{"single segment", 5, 1, "5 node(s)"},
		// The reported case: 60 one-minute segments, 5 nodes throughout.
		{"whole hour, stable cluster", 300, 60, "5 node(s)"},
		// A node that joined or left mid-window contributed to only some segments, so
		// the average is fractional and that is real information, not noise.
		{"whole hour, node joined midway", 312, 60, "5.2 node(s) avg"},
		// Defensive: a merge count of zero must not divide by zero.
		{"unknown merge count", 7, 0, "7 node(s)"},
	} {
		if got := formatNodeCount(tc.n, tc.segments); got != tc.want {
			t.Errorf("%s: formatNodeCount(%d, %d) = %q, want %q",
				tc.name, tc.n, tc.segments, got, tc.want)
		}
	}
}

// The same invariant end to end: an _ALL leaf must report the node count, not the
// node count times the number of segments it summed.
func TestWindowAllLeafReportsNodeCount(t *testing.T) {
	const nodes, segments = 5, 12
	segs := make([]madmin.LockSegment, segments)
	for i := range segs {
		segs[i] = madmin.LockSegment{N: nodes, AcquireCount: 10, AcquireNanos: 1_000_000}
	}
	nav := lockNav(t, &madmin.LockMetrics{
		LastHour: &madmin.SegmentedLockMetrics{
			Interval: 60, FirstTime: lockFirstTime, Segments: segs,
		},
	})
	all, err := nav.Navigate("locks/last_hour/_ALL")
	if err != nil {
		t.Fatalf("navigate _ALL: %v", err)
	}
	d := all.GetLeafData()
	if got, want := leafValue(d, "Nodes"), "5 node(s)"; got != want {
		t.Errorf("Nodes = %q, want %q: summing N over %d segments gives %d",
			got, want, segments, nodes*segments)
	}
	// The counters really are cross-segment sums, so those must NOT be divided.
	if got, want := leafValue(d, "Acquires"), "120"; got != want {
		t.Errorf("Acquires = %q, want %q: a counter still sums over the window", got, want)
	}
}

// Microsecond precision is what a sub-second latency needs and pure noise on a
// figure of minutes, which is how a mean failed-wait rendered as
// "13m19.506473s".
func TestDurationOfScalesPrecision(t *testing.T) {
	for _, tc := range []struct {
		acc, count uint64
		want       string
	}{
		{0, 0, "n/a"},
		{583_000, 1, "583µs"},
		{20_599_000, 1, "20.599ms"},
		// A second or more keeps milliseconds, not microseconds.
		{1_131_456_789, 1, "1.131s"},
		// A minute or more keeps whole seconds: this is the reported case, which used
		// to render as "13m19.506473s".
		{799_506_473_000, 1, "13m20s"},
		// Still an average, so the division happens first.
		{799_506_473_000 * 4, 4, "13m20s"},
	} {
		if got := durationOf(tc.acc, tc.count); got != tc.want {
			t.Errorf("durationOf(%d, %d) = %q, want %q", tc.acc, tc.count, got, tc.want)
		}
	}
}
