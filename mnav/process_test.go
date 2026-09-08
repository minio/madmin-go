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
	"strings"
	"testing"

	"github.com/minio/madmin-go/v4"
)

// Every process value on the wire is a sum over the processes that reported, so
// the fleet figure alone answers nothing: "File Descriptors 38,102" across 17
// processes is 2,241 each, and only the second number is comparable to a limit.
//
// The divisor is Count, so the mean is labelled per process rather than per node.
// The two are equal on a normal deployment and the section states both, but a
// host running more than one server would make "/node" a lie.
func TestProcessValuesCarryPerProcessMean(t *testing.T) {
	const nodes = 17
	nav := NewRealtimeMetricsNavigator(&madmin.RealtimeMetrics{
		Aggregated: madmin.Metrics{Process: &madmin.ProcessMetrics{
			Nodes: nodes, Count: nodes,
			// An hour of uptime each: the window every counter below is over.
			TotalRunningSecs: 3600 * nodes,
			TotalCPUPercent:  8.4 * nodes,
			TotalNumThreads:  248 * nodes,
			TotalNumFDs:      2241 * nodes,
			MemInfo:          madmin.ProcessMemoryInfo{RSS: 2 << 30 * nodes, Count: nodes},
			IOCounters: madmin.ProcessIOCounters{
				ReadBytes: 3_600_000_000 * nodes, Count: nodes,
			},
		}},
	})
	node, err := nav.Navigate("process")
	if err != nil {
		t.Fatalf("navigate process: %v", err)
	}
	data := node.GetLeafData()
	for _, want := range []struct{ label, value string }{
		{"Nodes", "17 node(s)"},
		{"Uptime", "1h (mean per process)"},
		{"CPU", "142.8% (8.4%/process)"},
		{"Threads", "4,216 (248/process)"},
		{"File Descriptors", "38,097 (2,241/process)"},
		{"Resident", "36 GB (2.1 GB/process)"},
		// Cumulative since process start, so it carries the rate over the uptime.
		{"Read", "61 GB (3.6 GB/process), 1.0 MB/s"},
	} {
		if got := leafValue(data, want.label); got != want.value {
			t.Errorf("%s = %q, want %q", want.label, got, want.value)
		}
	}

	// The old rows either stated a cluster sum as if it were one process's, or
	// said nothing at all.
	for _, gone := range []string{"Cumulative Uptime", "Total Read I/O", "Cluster Status", "Process Overview"} {
		if got := leafValue(data, gone); got != "" {
			t.Errorf("%s = %q, want it dropped", gone, got)
		}
	}
}

// A subsection reaches the uptime through its parent, since its own struct does
// not carry one, and without it a cumulative counter has no window to divide by.
func TestProcessSubsectionRatesUseParentUptime(t *testing.T) {
	nav := NewRealtimeMetricsNavigator(&madmin.RealtimeMetrics{
		Aggregated: madmin.Metrics{Process: &madmin.ProcessMetrics{
			Nodes: 4, Count: 4, TotalRunningSecs: 3600 * 4,
			IOCounters: madmin.ProcessIOCounters{
				ReadBytes: 3_600_000_000 * 4, ReadCount: 36000 * 4, Count: 4,
			},
		}},
	})
	node, err := nav.Navigate("process/io")
	if err != nil {
		t.Fatalf("navigate io: %v", err)
	}
	data := node.GetLeafData()
	for _, want := range []struct{ label, value string }{
		{"Read", "14 GB (3.6 GB/process), 1.0 MB/s"},
		{"Reads", "144,000 (36,000/process), 10 ops/s"},
		{"Mean Read", "100 kB"},
	} {
		if got := leafValue(data, want.label); got != want.value {
			t.Errorf("%s = %q, want %q", want.label, got, want.value)
		}
	}
}

// A segment's CPU seconds are summed over its samples -- one per process per
// tick -- so the utilisation is the per-process figure over the wall clock. The
// old form divided the whole sum by the window, scaling every percentage by the
// number of processes reporting.
func TestProcessSegmentCPUShareIsPerProcess(t *testing.T) {
	seg := madmin.ProcessSegment{
		N: 4, CPUPercent: 4 * 25, CPUUser: 200, RSS: 4 << 30,
		NumThreads: 4 * 100, ReadBytes: 4 * 900_000_000,
	}
	data := processSegmentRows(seg, 900, 1, "Covering 15m, until X.")
	for _, want := range []struct{ label, value string }{
		{"CPU", "25.00% per process"},
		// 200s over four processes is 50s each, and 50s of a 900s segment is
		// 5.56% of one core -- not the 22.2% the whole sum would give.
		{"CPU User", "50s (5.56% of one core)"},
		{"Resident", "1.1 GB per process"},
		{"Threads", "100 per process"},
		{"Read", "900 MB per process, 1.0 MB/s"},
	} {
		if got := leafValue(data, want.label); got != want.value {
			t.Errorf("%s = %q, want %q", want.label, got, want.value)
		}
	}
}

// The window lists an _ALL summary and a navigable child per segment, newest
// first and keyed by segment start like every other family, and every node in it keeps asking for
// the day window -- without the flag a refresh drops what it renders.
func TestProcessWindowNavigation(t *testing.T) {
	segs := make([]madmin.ProcessSegment, 3)
	for i := range segs {
		segs[i] = madmin.ProcessSegment{
			N: 2, CPUPercent: 2 * float64(10+i*5), RSS: 2 << 30, NumThreads: 2 * 90,
		}
	}
	nav := NewRealtimeMetricsNavigator(&madmin.RealtimeMetrics{
		Aggregated: madmin.Metrics{Process: &madmin.ProcessMetrics{
			Nodes: 2, Count: 2,
			LastDay: &madmin.SegmentedProcessMetrics{
				Interval: 900, FirstTime: dupFirstTime, Segments: segs,
			},
		}},
	})

	day, err := nav.Navigate("process/last_day")
	if err != nil {
		t.Fatalf("navigate last_day: %v", err)
	}
	// Newest first, behind _ALL, the way every family lists a window.
	got := childNames(day.GetChildren())
	want := []string{"_ALL", "10:30Z", "10:15Z", "10:00Z"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("children = %v, want %v", got, want)
	}
	if desc := childDesc(day.GetChildren(), "10:15Z"); !strings.Contains(desc, "CPU 15.0%") {
		t.Errorf("segment description = %q, want the segment's own CPU in it", desc)
	}

	for _, path := range []string{"process/last_day", "process/last_day/_ALL", "process/last_day/10:30Z"} {
		node, err := nav.Navigate(path)
		if err != nil {
			t.Fatalf("navigate %s: %v", path, err)
		}
		if flags := node.GetMetricFlags(); flags != madmin.MetricsDayStats {
			t.Errorf("%s flags = %v, want MetricsDayStats", path, flags)
		}
		if !node.ShouldPauseRefresh() {
			t.Errorf("%s refreshes while being read, want it paused", path)
		}
		if got := leafValue(node.GetLeafData(), "Coverage"); !strings.HasPrefix(got, "Covering ") {
			t.Errorf("%s Coverage = %q, want a coverage statement", path, got)
		}
	}

	// The whole window averages its segments rather than summing them.
	all, err := nav.Navigate("process/last_day/_ALL")
	if err != nil {
		t.Fatalf("navigate _ALL: %v", err)
	}
	if got, want := leafValue(all.GetLeafData(), "CPU"), "15.00% per process"; got != want {
		t.Errorf("_ALL CPU = %q, want %q", got, want)
	}
}

// A gauge is a level, so a window summary equals the segments it folded. A
// counter is each segment's own increase, so its window total is their sum while
// its rate stays the segments' own -- one sample covers one interval however many
// were folded in.
//
// The window used to be handed its whole span as the rate divisor, reading a
// 14-segment day at 136% of a core where every segment in it read 1900%.
func TestProcessWindowFoldsGaugesAndCountersApart(t *testing.T) {
	const nodes, count = 17, 14
	segs := make([]madmin.ProcessSegment, count)
	for i := range segs {
		segs[i] = madmin.ProcessSegment{
			N: nodes, CPUPercent: nodes * 1900, RSS: nodes * (2 << 30),
			CPUUser: nodes * 200, ReadBytes: nodes * 900_000_000,
		}
	}
	nav := NewRealtimeMetricsNavigator(&madmin.RealtimeMetrics{
		Aggregated: madmin.Metrics{Process: &madmin.ProcessMetrics{
			Nodes: nodes, Count: nodes,
			LastDay: &madmin.SegmentedProcessMetrics{
				Interval: 900, FirstTime: dupFirstTime, Segments: segs,
			},
		}},
	})

	all, err := nav.Navigate("process/last_day/_ALL")
	if err != nil {
		t.Fatalf("navigate _ALL: %v", err)
	}
	one, err := nav.Navigate("process/last_day/" + dupKey)
	if err != nil {
		t.Fatalf("navigate segment: %v", err)
	}
	allData, oneData := all.GetLeafData(), one.GetLeafData()

	for _, label := range []string{"CPU", "Resident", "Threads"} {
		got, want := leafValue(allData, label), leafValue(oneData, label)
		if want == "" {
			t.Fatalf("segment has no %s row to compare against", label)
		}
		if got != want {
			t.Errorf("_ALL %s = %q, want %q -- a level, not a sum", label, got, want)
		}
	}

	// 200 CPU-seconds of a 900-second interval is 22.2% of one core, and
	// fourteen of them are 46m40s across the window.
	for _, w := range []struct{ label, seg, all string }{
		{"CPU User", "3m20s (22.2% of one core)", "46m40s (22.2% of one core)"},
		{"Read", "900 MB per process, 1.0 MB/s", "13 GB per process, 1.0 MB/s"},
	} {
		if got := leafValue(oneData, w.label); got != w.seg {
			t.Errorf("segment %s = %q, want %q", w.label, got, w.seg)
		}
		if got := leafValue(allData, w.label); got != w.all {
			t.Errorf("_ALL %s = %q, want %q -- the sum of %d segments at the segments' own rate",
				w.label, got, w.all, count)
		}
	}

	// N counts samples, so the whole window carries one per process per segment;
	// only dividing that back down gives a process count.
	if got, want := leafValue(allData, "Processes"), "17 node(s) reporting"; got != want {
		t.Errorf("Processes = %q, want %q: N over the window is %d", got, want, nodes*count)
	}
}

// memNav builds a navigator over one merged memory sample. Values are given per
// node and summed here, the way MemInfo.Merge leaves them.
func memNav(nodes int, info madmin.MemInfo, day *madmin.SegmentedMemMetrics) MetricNavigator {
	scaled := madmin.MemInfo{
		Total: info.Total * uint64(nodes), Used: info.Used * uint64(nodes),
		Free: info.Free * uint64(nodes), Available: info.Available * uint64(nodes),
		Cache: info.Cache * uint64(nodes), Buffers: info.Buffers * uint64(nodes),
		Shared: info.Shared * uint64(nodes), Limit: info.Limit * uint64(nodes),
		SwapSpaceTotal: info.SwapSpaceTotal * uint64(nodes),
		SwapSpaceFree:  info.SwapSpaceFree * uint64(nodes),
	}
	return NewRealtimeMetricsNavigator(&madmin.RealtimeMetrics{
		Aggregated: madmin.Metrics{Mem: &madmin.MemMetrics{
			Nodes: nodes, Info: scaled, LastDay: day,
		}},
	})
}

// MemInfo.Merge sums every field, so the top page used to report a cluster's
// worth of RAM with one "Avg per Node" row bolted on and nothing per-node on any
// of the others.
func TestMemValuesCarryPerNodeMean(t *testing.T) {
	nav := memNav(17, madmin.MemInfo{
		Total: 128 << 30, Used: 94 << 30, Free: 6 << 30, Available: 34 << 30,
		Cache: 28 << 30, SwapSpaceTotal: 2 << 30, SwapSpaceFree: 2 << 30,
	}, nil)
	node, err := nav.Navigate("mem")
	if err != nil {
		t.Fatalf("navigate mem: %v", err)
	}
	data := node.GetLeafData()
	for _, want := range []struct{ label, value string }{
		{"Nodes", "17 node(s)"},
		{"Total", "2.3 TB (137 GB/node)"},
		{"Used", "1.7 TB (101 GB/node, 73.4%)"},
		{"Available", "621 GB (36 GB/node, 26.6%)"},
		{"Cache", "511 GB (30 GB/node, 21.9%)"},
	} {
		if got := leafValue(data, want.label); got != want.value {
			t.Errorf("%s = %q, want %q", want.label, got, want.value)
		}
	}
	// Rows that restated the node count, or offered advice instead of a
	// measurement.
	for _, gone := range []string{"Total Memory", "Avg per Node", "Avg Used", "Efficiency", "Pressure"} {
		if got := leafValue(data, gone); got != "" {
			t.Errorf("%s = %q, want it dropped", gone, got)
		}
	}
}

// The window was a flat list of rows with nothing to open, and never surfaced the
// vmstat deltas the wire already carries per segment.
func TestMemWindowNavigation(t *testing.T) {
	const nodes, count = 4, 3
	segs := make([]madmin.MemSegment, count)
	for i := range segs {
		segs[i] = madmin.MemSegment{
			N: nodes, Used: nodes * (90 << 30), Free: nodes * (10 << 30),
			Available: nodes * (30 << 30),
			// 1800 major faults a segment, per node: 2/s over 900s.
			MajorFaults: nodes * 1800, OOMKill: nodes * 1,
		}
	}
	nav := memNav(nodes, madmin.MemInfo{Total: 100 << 30}, &madmin.SegmentedMemMetrics{
		Interval: 900, FirstTime: dupFirstTime, Segments: segs,
	})

	day, err := nav.Navigate("mem/last_day")
	if err != nil {
		t.Fatalf("navigate last_day: %v", err)
	}
	got := childNames(day.GetChildren())
	want := []string{"_ALL", "10:30Z", "10:15Z", "10:00Z"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("children = %v, want %v", got, want)
	}
	if desc := childDesc(day.GetChildren(), "10:15Z"); !strings.Contains(desc, "Major Faults +1,800") {
		t.Errorf("segment description = %q, want the segment's own faults in it", desc)
	}

	// A gauge is a level, so the window mean equals each identical segment's.
	// A counter is a per-segment delta, so its window total is the sum while its
	// rate stays the segments' own.
	all, err := nav.Navigate("mem/last_day/_ALL")
	if err != nil {
		t.Fatalf("navigate _ALL: %v", err)
	}
	one, err := nav.Navigate("mem/last_day/" + dupKey)
	if err != nil {
		t.Fatalf("navigate segment: %v", err)
	}
	allData, oneData := all.GetLeafData(), one.GetLeafData()
	if got, want := leafValue(allData, "Used"), leafValue(oneData, "Used"); got != want {
		t.Errorf("_ALL Used = %q, want %q -- a level, not a sum", got, want)
	}
	if got, want := leafValue(allData, "Nodes"), "4 node(s) reporting"; got != want {
		t.Errorf("_ALL Nodes = %q, want %q: N over the window is %d", got, want, nodes*count)
	}
	if got, want := leafValue(oneData, "Major Faults"), "1,800 per node, 2.0/s"; got != want {
		t.Errorf("segment Major Faults = %q, want %q", got, want)
	}
	if got, want := leafValue(allData, "Major Faults"), "5,400 per node, 2.0/s"; got != want {
		t.Errorf("_ALL Major Faults = %q, want %q: three segments of 1,800 at the same rate", got, want)
	}
}

// The five process subsections and the four memory ones that no test reached.
// The CPU busy share, the mapped-memory breakdown, the voluntary/involuntary
// split, the swap ratio and the cgroup headroom are all derived here and nowhere
// else.
func TestProcessSubsectionsRenderDerivedRows(t *testing.T) {
	const n = 4
	nav := NewRealtimeMetricsNavigator(&madmin.RealtimeMetrics{
		Aggregated: madmin.Metrics{Process: &madmin.ProcessMetrics{
			Nodes: n, Count: n, TotalRunningSecs: 3600 * n,
			CPUTimes: madmin.ProcessCPUTimes{
				Count: n, User: 720 * n, System: 180 * n, Idle: 100 * n,
			},
			MemInfo: madmin.ProcessMemoryInfo{
				Count: n, RSS: 2 << 30 * n, VMS: 8 << 30 * n, HWM: 3 << 30 * n,
			},
			NumCtxSwitches: madmin.ProcessCtxSwitches{
				Count: n, Voluntary: 7500 * n, Involuntary: 2500 * n,
			},
			PageFaults: madmin.ProcessPageFaults{
				Count: n, MinorFaults: 99000 * n, MajorFaults: 1000 * n,
			},
			MemMaps: madmin.ProcessMemoryMaps{
				Count: n, TotalSize: 10 << 30 * n, TotalRSS: 2 << 30 * n,
				TotalPrivateDirty: 1 << 30 * n,
			},
		}},
	})
	for _, tc := range []struct{ path, label, want string }{
		// 900 CPU-seconds of the 3600 the process has been up, per process.
		{"process/cpu", "Busy", "25.0% of one core, per process"},
		{"process/cpu", "User", "48m0s (12m0s/process, 72.0%)"},
		{"process/memory", "Resident", "8.6 GB (2.1 GB/process)"},
		{"process/memory", "Peak Resident", "13 GB (3.2 GB/process)"},
		{"process/context_switches", "Voluntary", "30,000 (7,500/process, 75.0%)"},
		{"process/context_switches", "Involuntary", "10,000 (2,500/process, 25.0%)"},
		{"process/page_faults", "Minor", "396,000 (99,000/process, 99.0%)"},
		{"process/page_faults", "Major", "4,000 (1,000/process), 16/min"},
		{"process/mem_maps", "Mapped", "43 GB (11 GB/process)"},
		{"process/mem_maps", "Resident", "8.6 GB (2.1 GB/process, 20.0%)"},
		{"process/mem_maps", "Private Dirty", "4.3 GB (1.1 GB/process, 10.0%)"},
	} {
		node, err := nav.Navigate(tc.path)
		if err != nil {
			t.Fatalf("navigate %s: %v", tc.path, err)
		}
		if got := leafValue(node.GetLeafData(), tc.label); got != tc.want {
			t.Errorf("%s %s = %q, want %q", tc.path, tc.label, got, tc.want)
		}
	}
}

func TestMemSubsectionsRenderDerivedRows(t *testing.T) {
	nav := memNav(8, madmin.MemInfo{
		Total: 100 << 30, Used: 70 << 30, Free: 10 << 30, Available: 30 << 30,
		Cache: 18 << 30, Buffers: 2 << 30, Shared: 1 << 30,
		Limit: 80 << 30, SwapSpaceTotal: 50 << 30, SwapSpaceFree: 40 << 30,
	}, nil)
	for _, tc := range []struct{ path, label, want string }{
		{"mem/usage", "Available", "258 GB (32 GB/node, 30.0%)"},
		{"mem/usage", "Cache", "155 GB (19 GB/node, 18.0%)"},
		// Cache plus buffers is what the kernel hands back under pressure.
		{"mem/system", "Reclaimable", "172 GB (22 GB/node, 20.0%)"},
		{"mem/swap", "Used", "86 GB (11 GB/node, 20.0%)"},
		{"mem/swap", "Swap : RAM", "0.50 : 1"},
		{"mem/limits", "Limit", "687 GB (86 GB/node, 80.0%)"},
		{"mem/limits", "Headroom", "86 GB (11 GB/node, 12.5%)"},
	} {
		node, err := nav.Navigate(tc.path)
		if err != nil {
			t.Fatalf("navigate %s: %v", tc.path, err)
		}
		if got := leafValue(node.GetLeafData(), tc.label); got != tc.want {
			t.Errorf("%s %s = %q, want %q", tc.path, tc.label, got, tc.want)
		}
	}
}

// A window's slots are not all populated -- a restart, a startup or a node
// joining late leaves gaps -- and nothing accrued in the empty ones. Scaling the
// combined average by every slot invents activity that was never observed, and
// dividing N by every slot under-reports who was there.
func TestWindowTotalsSkipUnreportedSegments(t *testing.T) {
	const nodes = 2

	t.Run("process", func(t *testing.T) {
		// Two slots, one sample: 900 MB read and 200 CPU-seconds per process.
		segs := []madmin.ProcessSegment{
			{N: nodes, CPUPercent: nodes * 25, CPUUser: nodes * 200, ReadBytes: nodes * 900_000_000},
			{},
		}
		nav := NewRealtimeMetricsNavigator(&madmin.RealtimeMetrics{
			Aggregated: madmin.Metrics{Process: &madmin.ProcessMetrics{
				Nodes: nodes, Count: nodes,
				LastDay: &madmin.SegmentedProcessMetrics{
					Interval: 900, FirstTime: dupFirstTime, Segments: segs,
				},
			}},
		})
		all, err := nav.Navigate("process/last_day/_ALL")
		if err != nil {
			t.Fatalf("navigate _ALL: %v", err)
		}
		data := all.GetLeafData()
		for _, w := range []struct{ label, want string }{
			{"Read", "900 MB per process, 1.0 MB/s"},
			{"CPU User", "3m20s (22.2% of one core)"},
			{"Processes", "2 node(s) reporting"},
		} {
			if got := leafValue(data, w.label); got != w.want {
				t.Errorf("%s = %q, want %q: only one of the two slots reported",
					w.label, got, w.want)
			}
		}
	})

	t.Run("mem", func(t *testing.T) {
		segs := []madmin.MemSegment{
			{N: nodes, Used: nodes * (90 << 30), Free: nodes * (10 << 30), MajorFaults: nodes * 1800},
			{},
		}
		nav := memNav(nodes, madmin.MemInfo{Total: 100 << 30}, &madmin.SegmentedMemMetrics{
			Interval: 900, FirstTime: dupFirstTime, Segments: segs,
		})
		all, err := nav.Navigate("mem/last_day/_ALL")
		if err != nil {
			t.Fatalf("navigate _ALL: %v", err)
		}
		data := all.GetLeafData()
		for _, w := range []struct{ label, want string }{
			{"Major Faults", "1,800 per node, 2.0/s"},
			{"Nodes", "2 node(s) reporting"},
		} {
			if got := leafValue(data, w.label); got != w.want {
				t.Errorf("%s = %q, want %q: only one of the two slots reported",
					w.label, got, w.want)
			}
		}
	})
}
