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

package mnav

import (
	"reflect"
	"runtime/metrics"
	"strings"
	"testing"

	"github.com/minio/madmin-go/v4"
)

// goNav builds a navigator over one merged runtime sample. Values are given
// per node and summed here, the way the wire carries them.
func goNav(nodes int, uints map[string]uint64, floats map[string]float64) MetricNavigator {
	r := &madmin.RuntimeMetrics{
		N:            nodes,
		UintMetrics:  make(map[string]uint64, len(uints)),
		FloatMetrics: make(map[string]float64, len(floats)),
	}
	for k, v := range uints {
		r.UintMetrics[k] = v * uint64(nodes)
	}
	for k, v := range floats {
		r.FloatMetrics[k] = v * float64(nodes)
	}
	return NewRealtimeMetricsNavigator(&madmin.RealtimeMetrics{
		Aggregated: madmin.Metrics{Go: r},
	})
}

// Every runtime value on the wire is a sum across the nodes that reported, gauges
// included, so the fleet figure alone answers no question an operator has:
// "Active Goroutines 961,763" on a 17-node cluster is 56,574 per process.
func TestRuntimeValuesCarryPerNodeMean(t *testing.T) {
	const nodes = 17
	nav := goNav(nodes, map[string]uint64{
		mGoroutines:  56_574,
		mGomaxprocs:  64,
		mHeapInUse:   2 << 30,
		mHeapObjects: 1_000_000,
		mGCCycles:    500,
	}, map[string]float64{
		// 64 threads busy for an hour: the uptime every rate below derives from.
		mCPUTotal: 3600 * 64,
		mCPUUser:  3600 * 64 * 0.25,
	})

	root, err := nav.Navigate("go")
	if err != nil {
		t.Fatalf("navigate go: %v", err)
	}
	data := root.GetLeafData()
	for _, want := range []struct{ label, value string }{
		{"Nodes", "17 node(s)"},
		{"Goroutines", "961,758 (56,574/node)"},
		{"GOMAXPROCS", "1,088 (64/node)"},
		{"Heap In Use", "36 GB (2.1 GB/node)"},
		{"Uptime", "1h (derived, mean per node)"},
		// A counter is worth a rate, and the only window it has is the uptime.
		// Under one per second it is stated per minute: 500 collections in an
		// hour is 8.3 a minute, not "0.139 cycles/s".
		{"GC Cycles", "8,500 (500/node), 8.3 cycles/min"},
		{"User CPU", "272h0m0s CPU-time (16h0m0s/node, 25.0%)"},
	} {
		if got := leafValue(data, want.label); got != want.value {
			t.Errorf("%s = %q, want %q", want.label, got, want.value)
		}
	}

	// The counts of how many metric names the sample happened to carry said
	// nothing about the cluster and crowded out what did.
	for _, gone := range []string{"Uint64 Metrics", "Float64 Metrics", "Histogram Metrics", "Total Metrics", "Metric Categories"} {
		if got := leafValue(data, gone); got != "" {
			t.Errorf("%s = %q, want it dropped", gone, got)
		}
	}
}

// A reported uptime beats the one derived from the CPU accounting, and says so.
// Until the servers carry the field nothing reports one, so the derived value has
// to keep working -- and a partial rollout must divide by the nodes that answered
// rather than by the cluster, or the mean drops with the share still silent.
func TestRuntimeUptimePrefersReported(t *testing.T) {
	uptimeOf := func(reportedBy int, secs float64) string {
		nav := NewRealtimeMetricsNavigator(&madmin.RealtimeMetrics{
			Aggregated: madmin.Metrics{Go: &madmin.RuntimeMetrics{
				N:           10,
				UptimeSecs:  secs,
				UptimeNodes: reportedBy,
				UintMetrics: map[string]uint64{mGomaxprocs: 8 * 10},
				// The derivation would say two hours.
				FloatMetrics: map[string]float64{mCPUTotal: 7200 * 8 * 10},
			}},
		})
		node, err := nav.Navigate("go")
		if err != nil {
			t.Fatalf("navigate go: %v", err)
		}
		return leafValue(node.GetLeafData(), "Uptime")
	}
	for _, tc := range []struct {
		name       string
		reportedBy int
		secs       float64
		want       string
	}{
		{"nobody reports one", 0, 0, "2h (derived, mean per node)"},
		{"whole fleet reports", 10, 3600 * 10, "1h (mean per node)"},
		{"half the fleet reports", 4, 3600 * 4, "1h (mean of 4 of 10 nodes)"},
	} {
		if got := uptimeOf(tc.reportedBy, tc.secs); got != tc.want {
			t.Errorf("%s: Uptime = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// A single-node sample has no mean to add, and repeating the value would be pure
// noise.
func TestRuntimeSingleNodeHasNoMean(t *testing.T) {
	nav := goNav(1, map[string]uint64{mGoroutines: 400}, nil)
	root, err := nav.Navigate("go")
	if err != nil {
		t.Fatalf("navigate go: %v", err)
	}
	if got, want := leafValue(root.GetLeafData(), "Goroutines"), "400"; got != want {
		t.Errorf("Goroutines = %q, want %q", got, want)
	}
}

// The CPU classes are cumulative since start, so a share of the CPU time that was
// available is the only rate they can give -- and the sub-percent ones must not
// all round to the same "0.00%".
func TestRuntimeCPUClassesAreShares(t *testing.T) {
	nav := goNav(4, map[string]uint64{mGomaxprocs: 8}, map[string]float64{
		mCPUTotal:                       1000,
		mCPUUser:                        250,
		"/cpu/classes/idle:cpu-seconds": 749,
		mCPUGC:                          1,
		"/cpu/classes/gc/mark/dedicated:cpu-seconds": 0.05,
	})
	node, err := nav.Navigate("go/cpu_classes")
	if err != nil {
		t.Fatalf("navigate cpu_classes: %v", err)
	}
	data := node.GetLeafData()
	for _, want := range []struct{ label, value string }{
		{"User", "16m40s CPU-time (4m10s/node, 25.0%)"},
		{"Idle", "49m56s CPU-time (12m29s/node, 74.9%)"},
		{"GC Total", "4s CPU-time (1s/node, 0.10%)"},
		{"↳ Mark Dedicated", "200ms CPU-time (50ms/node, <0.01%)"},
	} {
		if got := leafValue(data, want.label); got != want.value {
			t.Errorf("%s = %q, want %q", want.label, got, want.value)
		}
	}
}

// A breakdown states the split, but the per-node mean is what the rest of the
// section carries and what an operator compares against one process. Showing only
// the share drops the magnitude the reader came for.
func TestRuntimeMemoryClassesCarryBothMeanAndShare(t *testing.T) {
	nav := goNav(8, map[string]uint64{
		mMemTotal:                         1000 << 20,
		mHeapInUse:                        700 << 20,
		"/memory/classes/heap/free:bytes": 300 << 20,
	}, nil)
	node, err := nav.Navigate("go/memory")
	if err != nil {
		t.Fatalf("navigate memory: %v", err)
	}
	data := node.GetLeafData()
	for _, want := range []struct{ label, value string }{
		{"Total", "8.4 GB (1.0 GB/node)"},
		{"Heap In Use", "5.9 GB (734 MB/node, 70.0%)"},
		{"Heap Free", "2.5 GB (315 MB/node, 30.0%)"},
	} {
		if got := leafValue(data, want.label); got != want.value {
			t.Errorf("%s = %q, want %q", want.label, got, want.value)
		}
	}
}

// Half of a real cluster's GC pauses landed under 64ns, which the shared
// duration helper floors to "0s" -- a percentile that reads as "instant" for
// every histogram is worse than none.
func TestRuntimeHistogramKeepsNanoseconds(t *testing.T) {
	h := metrics.Float64Histogram{
		Buckets: []float64{0, 64e-9, 128e-9, 1e-3, 4e-3},
		Counts:  []uint64{530, 100, 40, 2},
	}
	line, ok := histLine(h, 6)
	if !ok {
		t.Fatal("histLine returned no summary")
	}
	for _, want := range []string{"672 (112/node)", "p50 64ns", "p99 1ms"} {
		if !strings.Contains(line, want) {
			t.Errorf("histLine = %q, want it to contain %q", line, want)
		}
	}
}

// runtimeWindow builds a day window whose gauges move and whose counters climb,
// so both halves of the segment view have something to render.
func runtimeWindowNav(t *testing.T) MetricNavigator {
	t.Helper()
	const nodes = 3
	segs := make([]madmin.RuntimeSegment, 4)
	for i := range segs {
		segs[i] = madmin.RuntimeSegment{
			N: nodes,
			UintMetrics: map[string]uint64{
				mGoroutines: uint64(500+i*100) * nodes,
				mHeapInUse:  uint64(1<<30) * nodes,
				mGCCycles:   uint64(1000+i*10) * nodes,
				mAllocBytes: uint64(i+1) * (1 << 30) * nodes,
			},
		}
	}
	return NewRealtimeMetricsNavigator(&madmin.RealtimeMetrics{
		Aggregated: madmin.Metrics{Go: &madmin.RuntimeMetrics{
			N: nodes,
			LastDay: &madmin.SegmentedRuntimeMetrics{
				Interval: 900, FirstTime: dupFirstTime, Segments: segs,
			},
		}},
	})
}

// The window used to be a flat wall of rows with no total and nothing to open.
// It now lists an _ALL summary and a navigable child per segment, and the
// counters -- cumulative since process start on the wire -- are differenced into
// rates rather than shown as their lifetime level.
func TestRuntimeWindowNavigation(t *testing.T) {
	nav := runtimeWindowNav(t)

	day, err := nav.Navigate("go/last_day")
	if err != nil {
		t.Fatalf("navigate last_day: %v", err)
	}
	// Newest first, behind _ALL, the way every family lists a window.
	names := childNames(day.GetChildren())
	want := []string{"_ALL", "10:45Z", "10:30Z", "10:15Z", "10:00Z"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("children = %v, want %v", names, want)
	}
	// Each child says what happened in its segment, so the window page is the
	// aggregate and nothing else -- a day of segment rows buried the totals.
	if got := childDesc(day.GetChildren(), "10:30Z"); !strings.Contains(got, "GC Cycles +10") {
		t.Errorf("segment description = %q, want the segment's own movement in it", got)
	}
	for key := range day.GetLeafData() {
		if strings.Contains(key, "Z") && !strings.Contains(key, "Coverage") {
			t.Errorf("window leaf carries per-segment row %q, want the aggregate only", key)
		}
	}

	// _ALL ranges the gauges and totals the counters: three steps of ten cycles
	// across four samples. The rate is over the 45 minutes those three increases
	// were observed across, not the hour the window spans -- the first sample only
	// establishes a baseline, and nothing is attributable to it.
	all, err := nav.Navigate("go/last_day/_ALL")
	if err != nil {
		t.Fatalf("navigate _ALL: %v", err)
	}
	data := all.GetLeafData()
	for _, w := range []struct{ label, value string }{
		{"Segments", "4 of 4 reported"},
		{"Nodes", "3 node(s)"},
		{"Goroutines", "650/node avg (min 500, max 800)"},
		{"Heap In Use", "1.1 GB/node avg"},
		{"GC Cycles", "+30/node over the window, 0.7 cycles/min"},
		{"Allocated", "+3.2 GB/node over the window, 1.2 MB/s"},
	} {
		if got := leafValue(data, w.label); got != w.value {
			t.Errorf("_ALL %s = %q, want %q", w.label, got, w.value)
		}
	}

	// A segment states the level and what moved inside it, at the segment's own
	// 15-minute rate.
	seg, err := nav.Navigate("go/last_day/10:30Z")
	if err != nil {
		t.Fatalf("navigate segment: %v", err)
	}
	data = seg.GetLeafData()
	for _, w := range []struct{ label, value string }{
		{"Nodes", "3 node(s)"},
		{"Goroutines", "2,100 (700/node)"},
		{"GC Cycles", "3,060 (1,020/node), +10/node (0.7 cycles/min)"},
	} {
		if got := leafValue(data, w.label); got != w.value {
			t.Errorf("segment %s = %q, want %q", w.label, got, w.value)
		}
	}
	if got := leafValue(data, "Time Segment"); !strings.HasPrefix(got, "Covering 15m") {
		t.Errorf("Time Segment = %q, want it to cover 15m", got)
	}
}

// A node that restarted inside the window resets its cumulative counters, and the
// per-node mean drops with it. There is no way to recover the missing span from
// the sample, so that segment carries no rate rather than a negative one.
func TestRuntimeWindowSkipsCounterResets(t *testing.T) {
	const nodes = 2
	cycles := []uint64{1000, 1010, 4, 14}
	segs := make([]madmin.RuntimeSegment, len(cycles))
	for i, c := range cycles {
		segs[i] = madmin.RuntimeSegment{
			N:           nodes,
			UintMetrics: map[string]uint64{mGCCycles: c * nodes},
		}
	}
	w := newRuntimeWindow(&madmin.SegmentedRuntimeMetrics{
		Interval: 900, FirstTime: dupFirstTime, Segments: segs,
	})
	if got, ok := w.deltas[2][mGCCycles]; ok {
		t.Errorf("reset segment carries delta %v, want none", got)
	}
	// The span the surviving delta covers is measured from the segment that
	// last reported, so a gap does not inflate the rate derived from it.
	if got, want := w.deltas[3][mGCCycles].secs, 900; got != want {
		t.Errorf("delta span = %v, want %v", got, want)
	}
	if got, want := w.deltas[3][mGCCycles].value, 10.0; got != want {
		t.Errorf("delta after the reset = %v, want %v", got, want)
	}
	// The window total is the sum of the deltas it could trust, not last minus
	// first -- which would be negative here.
	if got, want := w.stats[mGCCycles].delta, 20.0; got != want {
		t.Errorf("window delta = %v, want %v", got, want)
	}
}

// A window that was never requested and one that is still filling are opposite
// answers, and neither is a measured zero.
func TestRuntimeWindowStates(t *testing.T) {
	nav := NewRealtimeMetricsNavigator(&madmin.RealtimeMetrics{
		Aggregated: madmin.Metrics{Go: &madmin.RuntimeMetrics{
			N:       1,
			LastDay: &madmin.SegmentedRuntimeMetrics{Interval: 900},
		}},
	})
	for _, tc := range []struct{ path, want string }{
		{"go/last_hour", "not collected"},
		{"go/last_day", "no data yet"},
	} {
		node, err := nav.Navigate(tc.path)
		if err != nil {
			t.Fatalf("navigate %s: %v", tc.path, err)
		}
		if got := node.GetLeafData()["Status"]; !strings.Contains(got, tc.want) {
			t.Errorf("%s status = %q, want it to contain %q", tc.path, got, tc.want)
		}
	}
}

// The window nodes ask for their own history; the root does not, so a continuous
// refresh does not pull the hour and the day on every tick.
func TestRuntimeWindowFlags(t *testing.T) {
	nav := runtimeWindowNav(t)
	for _, tc := range []struct {
		path string
		want madmin.MetricFlags
	}{
		{"go", 0},
		{"go/gc", 0},
		{"go/last_day", madmin.MetricsDayStats},
		{"go/last_day/_ALL", madmin.MetricsDayStats},
		{"go/last_day/10:00Z", madmin.MetricsDayStats},
		{"go/last_hour", madmin.MetricsHourStats},
	} {
		node, err := nav.Navigate(tc.path)
		if err != nil {
			t.Fatalf("navigate %s: %v", tc.path, err)
		}
		if got := node.GetMetricFlags(); got != tc.want {
			t.Errorf("%s flags = %v, want %v", tc.path, got, tc.want)
		}
		if got := node.GetOpts().Type; got&madmin.MetricsRuntime == 0 {
			t.Errorf("%s opts type = %v, want MetricsRuntime", tc.path, got)
		}
	}
}

// childDesc is the description listed for one child, or "" when it is absent.
func childDesc(children []MetricChild, name string) string {
	for _, c := range children {
		if c.Name == name {
			return c.Description
		}
	}
	return ""
}
