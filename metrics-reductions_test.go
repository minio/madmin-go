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

import (
	"maps"
	"reflect"
	"testing"
	"time"
)

func TestGaugeStatsAdd(t *testing.T) {
	a := GaugeStats{N: 2, Sum: 30, Min: 10, Max: 20}
	b := GaugeStats{N: 3, Sum: 60, Min: 5, Max: 40}

	got := a
	got.Add(&b)
	want := GaugeStats{N: 5, Sum: 90, Min: 5, Max: 40}
	if got != want {
		t.Fatalf("Add: got %+v, want %+v", got, want)
	}
}

func TestGaugeStatsAddIsOrderIndependent(t *testing.T) {
	a := GaugeStats{N: 2, Sum: 30, Min: 10, Max: 20}
	b := GaugeStats{N: 3, Sum: 60, Min: 5, Max: 40}
	c := GaugeStats{N: 1, Sum: 7, Min: 7, Max: 7}

	forward := a
	forward.Add(&b)
	forward.Add(&c)

	reverse := c
	reverse.Add(&b)
	reverse.Add(&a)

	if forward != reverse {
		t.Fatalf("order dependent: %+v != %+v", forward, reverse)
	}
}

// A zero GaugeStats must be the identity for Add, which is what
// internal/windowed's Adder contract requires: newFn() followed by Add(v)
// must reproduce v.
func TestGaugeStatsZeroIsAddIdentity(t *testing.T) {
	for _, v := range []GaugeStats{
		{N: 1, Sum: 5, Min: 5, Max: 5},
		{N: 4, Sum: 0, Min: 0, Max: 0}, // a legitimately all-zero sample
		{N: 3, Sum: -9, Min: -5, Max: -1},
	} {
		var got GaugeStats
		got.Add(&v)
		if got != v {
			t.Errorf("zero+Add(%+v) = %+v, want %+v", v, got, v)
		}
	}
}

func TestGaugeStatsAddEmptyAndNil(t *testing.T) {
	want := GaugeStats{N: 2, Sum: 30, Min: 10, Max: 20}

	got := want
	got.Add(nil)
	if got != want {
		t.Errorf("Add(nil) mutated: got %+v, want %+v", got, want)
	}

	// N == 0 is the unset sentinel: an empty sample must not drag Min to 0.
	got = want
	got.Add(&GaugeStats{})
	if got != want {
		t.Errorf("Add(empty) mutated: got %+v, want %+v", got, want)
	}
}

func TestGaugeStatsMean(t *testing.T) {
	if got := (GaugeStats{}).Mean(); got != 0 {
		t.Errorf("Mean of zero value = %v, want 0", got)
	}
	if got := (GaugeStats{N: 4, Sum: 10}).Mean(); got != 2.5 {
		t.Errorf("Mean = %v, want 2.5", got)
	}
}

func TestAddMap(t *testing.T) {
	var dst map[string]uint64
	addMap(&dst, nil)
	if dst != nil {
		t.Fatalf("addMap(nil) allocated: %v", dst)
	}

	addMap(&dst, map[string]uint64{"a": 1, "b": 2})
	addMap(&dst, map[string]uint64{"b": 3, "c": 4})
	want := map[string]uint64{"a": 1, "b": 5, "c": 4}
	if !maps.Equal(dst, want) {
		t.Fatalf("addMap: got %v, want %v", dst, want)
	}
}

// Non-string keys are supported because metrics.go generates with
// -d "maps binkeys"; DStateStats.DwellBuckets relies on it.
func TestAddMapIntKeys(t *testing.T) {
	var dst map[int]int
	addMap(&dst, map[int]int{5: 1, 10: 2})
	addMap(&dst, map[int]int{10: 3, 30: 1})
	want := map[int]int{5: 1, 10: 5, 30: 1}
	if !maps.Equal(dst, want) {
		t.Fatalf("addMap: got %v, want %v", dst, want)
	}
}

func TestMergeMap(t *testing.T) {
	var dst map[string]GaugeStats
	mergeMap(&dst, nil)
	if dst != nil {
		t.Fatalf("mergeMap(nil) allocated: %v", dst)
	}

	mergeMap(&dst, map[string]GaugeStats{
		"cpu_some": {N: 1, Sum: 10, Min: 10, Max: 10},
	})
	mergeMap(&dst, map[string]GaugeStats{
		"cpu_some": {N: 1, Sum: 4, Min: 4, Max: 4},
		"io_full":  {N: 1, Sum: 2, Min: 2, Max: 2},
	})

	want := map[string]GaugeStats{
		"cpu_some": {N: 2, Sum: 14, Min: 4, Max: 10},
		"io_full":  {N: 1, Sum: 2, Min: 2, Max: 2},
	}
	if !maps.Equal(dst, want) {
		t.Fatalf("mergeMap: got %v, want %v", dst, want)
	}
}

func TestMergeMapDoesNotMutateSource(t *testing.T) {
	src := map[string]TimedAction{"put": {Count: 1, AccTime: 100}}
	srcCopy := maps.Clone(src)
	dst := map[string]TimedAction{"put": {Count: 2, AccTime: 50}}

	mergeMap(&dst, src)

	if !maps.Equal(src, srcCopy) {
		t.Fatalf("source mutated: got %v, want %v", src, srcCopy)
	}
	if got := dst["put"]; got.Count != 3 || got.AccTime != 150 {
		t.Fatalf("dst = %+v, want Count 3 AccTime 150", got)
	}
}

// nil must mean "this kernel does not expose the counter", never zero.
func TestAddOptCounter(t *testing.T) {
	var dst *uint64

	addOptCounter(&dst, nil)
	if dst != nil {
		t.Fatalf("nil+nil became %v, want nil", *dst)
	}

	src := uint64(5)
	addOptCounter(&dst, &src)
	if dst == nil || *dst != 5 {
		t.Fatalf("nil+5 = %v, want 5", dst)
	}
	if src != 5 {
		t.Fatalf("source mutated to %d", src)
	}

	src2 := uint64(7)
	addOptCounter(&dst, &src2)
	if *dst != 12 {
		t.Fatalf("5+7 = %d, want 12", *dst)
	}

	// An absent counter on the other side must not reset what we have.
	addOptCounter(&dst, nil)
	if *dst != 12 {
		t.Fatalf("12+nil = %d, want 12", *dst)
	}
}

func TestAddOptCounterDoesNotAliasSource(t *testing.T) {
	var dst *uint64
	src := uint64(5)
	addOptCounter(&dst, &src)

	src = 99
	if *dst != 5 {
		t.Fatalf("dst aliases src: got %d, want 5", *dst)
	}
}

func TestTakeLater(t *testing.T) {
	t0 := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Minute)

	tests := []struct {
		name    string
		dstAt   time.Time
		dstMsg  string
		srcAt   time.Time
		srcMsg  string
		wantAt  time.Time
		wantMsg string
	}{
		{"empty source is ignored", t0, "old", time.Time{}, "", t0, "old"},
		{"empty dest adopts source", time.Time{}, "", t0, "new", t0, "new"},
		{"later source wins", t0, "old", t1, "new", t1, "new"},
		{"earlier source loses", t1, "new", t0, "old", t1, "new"},
		{"tie breaks on smaller message", t0, "b", t0, "a", t0, "a"},
		{"tie keeps smaller message", t0, "a", t0, "b", t0, "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			at, msg := tt.dstAt, tt.dstMsg
			takeLater(&at, &msg, tt.srcAt, tt.srcMsg)
			if !at.Equal(tt.wantAt) || msg != tt.wantMsg {
				t.Fatalf("got (%v, %q), want (%v, %q)", at, msg, tt.wantAt, tt.wantMsg)
			}
		})
	}
}

func TestTakeLaterIsOrderIndependent(t *testing.T) {
	t0 := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	type sample struct {
		at  time.Time
		msg string
	}
	samples := []sample{
		{t0.Add(time.Minute), "b"},
		{t0, "a"},
		{t0.Add(time.Minute), "a"}, // ties with the first on time
	}

	fold := func(order []int) sample {
		var out sample
		for _, i := range order {
			takeLater(&out.at, &out.msg, samples[i].at, samples[i].msg)
		}
		return out
	}

	forward := fold([]int{0, 1, 2})
	reverse := fold([]int{2, 1, 0})
	shuffled := fold([]int{1, 2, 0})

	if !reflect.DeepEqual(forward, reverse) || !reflect.DeepEqual(forward, shuffled) {
		t.Fatalf("order dependent: %+v / %+v / %+v", forward, reverse, shuffled)
	}
	// The later timestamp with the smaller message must win.
	if forward.msg != "a" || !forward.at.Equal(t0.Add(time.Minute)) {
		t.Fatalf("got %+v, want ('a', %v)", forward, t0.Add(time.Minute))
	}
}

// MetricFlags.String() previously omitted the three Top* flags.
func TestMetricFlagsString(t *testing.T) {
	all := MetricsDayStats | MetricsByHost | MetricsByDisk | MetricsLegacyDiskIO |
		MetricsByDiskSet | MetricsSMART | MetricsHourStats |
		MetricsTopWarehouses | MetricsTopNamespaces | MetricsTopTables |
		MetricsTablesCatalog

	want := "DayStats,ByHost,ByDisk,LegacyIO,ByDiskSet,SMART,HourStats," +
		"TopWarehouses,TopNamespaces,TopTables,TablesCatalog"
	if got := all.String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}

	if got := MetricFlags(0).String(); got != "" {
		t.Fatalf("String() of no flags = %q, want empty", got)
	}
	if got := MetricsTopTables.String(); got != "TopTables" {
		t.Fatalf("String() = %q, want %q", got, "TopTables")
	}

	// Flag values travel as an integer, so appending must not move an existing
	// bit. MetricsTablesCatalog is the first bit added after MetricsTopTables.
	if MetricsTablesCatalog != 1<<10 {
		t.Fatalf("MetricsTablesCatalog = %d, want %d: a flag bit moved",
			MetricsTablesCatalog, 1<<10)
	}
	if MetricsTopTables != 1<<9 || MetricsDayStats != 1<<0 {
		t.Fatal("an existing MetricFlags bit moved")
	}
}

// The three ProcessMetrics additions each use a different reduction, so the
// merge is worth pinning: ThreadStates and DwellBuckets/ByWchan sum by key,
// Pressure merges per line, and WindowSecs takes the max.
func TestProcessMetricsMergeThreadsPressureDState(t *testing.T) {
	nodeA := &ProcessMetrics{
		Nodes:        1,
		ThreadStates: map[string]int{"R": 2, "D": 1},
		Pressure: map[string]PSIStall{
			"io_some": {N: 1, StallUS: 100, Avg10Sum: 4, Avg10Max: 4},
		},
		DState: &DStateStats{
			WindowSecs:   55,
			DwellBuckets: map[int]int{0: 1, 5: 1, 10: 0},
			ByWchan:      map[string]int{"io_schedule": 1},
		},
	}
	nodeB := &ProcessMetrics{
		Nodes:        1,
		ThreadStates: map[string]int{"R": 3, "S": 7},
		Pressure: map[string]PSIStall{
			"io_some":  {N: 1, StallUS: 50, Avg10Sum: 40, Avg10Max: 40},
			"cpu_some": {N: 1, StallUS: 10, Avg10Sum: 1, Avg10Max: 1},
		},
		DState: &DStateStats{
			WindowSecs:   50,
			DwellBuckets: map[int]int{0: 2, 5: 1, 10: 1},
			ByWchan:      map[string]int{"io_schedule": 1, "msleep": 1},
		},
	}

	merge := func(a, b *ProcessMetrics) *ProcessMetrics {
		var out ProcessMetrics
		out.Merge(a)
		out.Merge(b)
		return &out
	}

	got := merge(nodeA, nodeB)

	if !maps.Equal(got.ThreadStates, map[string]int{"R": 5, "D": 1, "S": 7}) {
		t.Errorf("ThreadStates = %v", got.ThreadStates)
	}

	io := got.Pressure["io_some"]
	if io.N != 2 || io.StallUS != 150 || io.Avg10Sum != 44 || io.Avg10Max != 40 {
		t.Errorf("Pressure[io_some] = %+v, want N=2 StallUS=150 Sum=44 Max=40", io)
	}
	if cpu := got.Pressure["cpu_some"]; cpu.N != 1 {
		t.Errorf("Pressure[cpu_some] = %+v, want the single-node entry carried through", cpu)
	}

	if got.DState == nil {
		t.Fatal("DState = nil")
	}
	// WindowSecs is a config echo: max, never summed.
	if got.DState.WindowSecs != 55 {
		t.Errorf("WindowSecs = %d, want 55", got.DState.WindowSecs)
	}
	if !maps.Equal(got.DState.DwellBuckets, map[int]int{0: 3, 5: 2, 10: 1}) {
		t.Errorf("DwellBuckets = %v", got.DState.DwellBuckets)
	}
	if !maps.Equal(got.DState.ByWchan, map[string]int{"io_schedule": 2, "msleep": 1}) {
		t.Errorf("ByWchan = %v", got.DState.ByWchan)
	}

	// collectRealtimeMetrics merges remote before local, so order must not matter.
	rev := merge(nodeB, nodeA)
	if !maps.Equal(rev.ThreadStates, got.ThreadStates) ||
		!maps.Equal(rev.DState.DwellBuckets, got.DState.DwellBuckets) ||
		!maps.Equal(rev.DState.ByWchan, got.DState.ByWchan) ||
		rev.DState.WindowSecs != got.DState.WindowSecs ||
		rev.Pressure["io_some"] != got.Pressure["io_some"] {
		t.Error("ProcessMetrics.Merge is order dependent")
	}
}

// A node that reports nothing must not blank what other nodes reported.
func TestProcessMetricsMergeNilAdditions(t *testing.T) {
	full := &ProcessMetrics{
		Nodes:        1,
		ThreadStates: map[string]int{"R": 2},
		Pressure:     map[string]PSIStall{"io_some": {N: 1, Avg10Sum: 4, Avg10Max: 4}},
		DState:       &DStateStats{WindowSecs: 55, ByWchan: map[string]int{"io_schedule": 1}},
	}

	var out ProcessMetrics
	out.Merge(full)
	out.Merge(&ProcessMetrics{Nodes: 1})

	if !maps.Equal(out.ThreadStates, map[string]int{"R": 2}) {
		t.Errorf("ThreadStates = %v, want preserved", out.ThreadStates)
	}
	if out.DState == nil || out.DState.WindowSecs != 55 {
		t.Errorf("DState = %+v, want preserved", out.DState)
	}
	if out.Pressure["io_some"].N != 1 {
		t.Errorf("Pressure = %v, want preserved", out.Pressure)
	}
}
