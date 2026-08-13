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
	"testing"
)

func TestNetStackStatsMerge(t *testing.T) {
	dropsA, dropsB := uint64(3), uint64(4)
	onlyA := uint64(9)

	a := &NetStackStats{
		TCPRetransSegs: 10,
		TCPCurrEstab:   5,
		TCPListenDrops: &dropsA,
		TCPSynRetrans:  &onlyA,
	}
	b := &NetStackStats{
		TCPRetransSegs: 20,
		TCPCurrEstab:   7,
		TCPListenDrops: &dropsB,
		// TCPSynRetrans is nil: this kernel does not expose it.
	}

	got := *a
	got.Merge(b)

	if got.TCPRetransSegs != 30 {
		t.Errorf("TCPRetransSegs = %d, want 30", got.TCPRetransSegs)
	}
	// A gauge that counts sockets still sums to a meaningful cluster total.
	if got.TCPCurrEstab != 12 {
		t.Errorf("TCPCurrEstab = %d, want 12", got.TCPCurrEstab)
	}
	if got.TCPListenDrops == nil || *got.TCPListenDrops != 7 {
		t.Errorf("TCPListenDrops = %v, want 7", got.TCPListenDrops)
	}
	// A counter absent on one kernel must keep the other's value rather than
	// collapsing to zero.
	if got.TCPSynRetrans == nil || *got.TCPSynRetrans != 9 {
		t.Errorf("TCPSynRetrans = %v, want 9", got.TCPSynRetrans)
	}
	// A counter absent on both stays absent, so a reader can tell "not
	// exposed" from "zero".
	if got.TCPAbortOnData != nil {
		t.Errorf("TCPAbortOnData = %v, want nil", got.TCPAbortOnData)
	}

	// Merge must not mutate the source.
	if *b.TCPListenDrops != 4 || b.TCPSynRetrans != nil {
		t.Error("Merge mutated its argument")
	}
}

func TestNetStackStatsMergeIsOrderIndependent(t *testing.T) {
	x, y := uint64(1), uint64(2)
	a := NetStackStats{TCPRetransSegs: 10, TCPCurrEstab: 5, TCPListenDrops: &x}
	b := NetStackStats{TCPRetransSegs: 20, TCPCurrEstab: 7, TCPSynRetrans: &y}

	fwd := a
	fwd.Merge(&b)
	rev := b
	rev.Merge(&a)

	if fwd.TCPRetransSegs != rev.TCPRetransSegs || fwd.TCPCurrEstab != rev.TCPCurrEstab ||
		*fwd.TCPListenDrops != *rev.TCPListenDrops || *fwd.TCPSynRetrans != *rev.TCPSynRetrans {
		t.Errorf("order dependent: %+v vs %+v", fwd, rev)
	}
}

func TestNetConnStatsMerge(t *testing.T) {
	a := &NetConnStats{
		States:  map[string]uint64{"ESTABLISHED": 10, "TIME_WAIT": 2},
		Backlog: &NetListenerBacklogStats{N: 2, DepthSum: 6, DepthMax: 4, LimitMin: 4096},
	}
	b := &NetConnStats{
		States:  map[string]uint64{"ESTABLISHED": 5, "CLOSE_WAIT": 1},
		Backlog: &NetListenerBacklogStats{N: 1, DepthSum: 40, DepthMax: 40, LimitMin: 511},
	}

	got := NetConnStats{}
	got.Merge(a)
	got.Merge(b)

	want := map[string]uint64{"ESTABLISHED": 15, "TIME_WAIT": 2, "CLOSE_WAIT": 1}
	if !maps.Equal(got.States, want) {
		t.Errorf("States = %v, want %v", got.States, want)
	}
	if got.Backlog.N != 3 || got.Backlog.DepthSum != 46 {
		t.Errorf("Backlog = %+v, want N=3 DepthSum=46", got.Backlog)
	}
	// The worst queue and the tightest limit are what an operator needs.
	if got.Backlog.DepthMax != 40 || got.Backlog.LimitMin != 511 {
		t.Errorf("Backlog = %+v, want DepthMax=40 LimitMin=511", got.Backlog)
	}

	rev := NetConnStats{}
	rev.Merge(b)
	rev.Merge(a)
	if !maps.Equal(rev.States, got.States) || *rev.Backlog != *got.Backlog {
		t.Error("NetConnStats.Merge is order dependent")
	}
}

// A zero limit means "not reported" and must never win the minimum.
func TestNetListenerBacklogStatsZeroLimit(t *testing.T) {
	got := NetListenerBacklogStats{}
	got.Add(&NetListenerBacklogStats{N: 1, DepthSum: 1, LimitMin: 0})
	got.Add(&NetListenerBacklogStats{N: 1, DepthSum: 1, LimitMin: 4096})

	if got.LimitMin != 4096 {
		t.Errorf("LimitMin = %d, want 4096", got.LimitMin)
	}
}

func TestNetLinkStatsMerge(t *testing.T) {
	a := &NetLinkStats{
		N:              2,
		OperStates:     map[string]int{"up": 2},
		Duplex:         map[string]int{"full": 2},
		SpeedMbps:      map[int64]int{100000: 2},
		MTU:            map[uint32]int{9000: 2},
		CarrierChanges: 3,
	}
	b := &NetLinkStats{
		N:              2,
		OperStates:     map[string]int{"up": 1, "down": 1},
		Duplex:         map[string]int{"full": 1, "half": 1},
		SpeedMbps:      map[int64]int{100000: 1, 1000: 1},
		MTU:            map[uint32]int{9000: 1, 1500: 1},
		CarrierChanges: 4,
	}

	got := NetLinkStats{}
	got.Merge(a)
	got.Merge(b)

	if got.N != 4 {
		t.Errorf("N = %d, want 4", got.N)
	}
	if !maps.Equal(got.OperStates, map[string]int{"up": 3, "down": 1}) {
		t.Errorf("OperStates = %v", got.OperStates)
	}
	// The odd rail stays visible as its own key across the whole cluster, which
	// is the point of counting values rather than listing interfaces.
	if !maps.Equal(got.SpeedMbps, map[int64]int{100000: 3, 1000: 1}) {
		t.Errorf("SpeedMbps = %v", got.SpeedMbps)
	}
	if !maps.Equal(got.MTU, map[uint32]int{9000: 3, 1500: 1}) {
		t.Errorf("MTU = %v", got.MTU)
	}
	if got.CarrierChanges != 7 {
		t.Errorf("CarrierChanges = %d, want 7", got.CarrierChanges)
	}
	// The link total is N and the down count is derived, so neither is a field.
	if down := got.N - got.OperStates["up"]; down != 1 {
		t.Errorf("derived down count = %d, want 1", down)
	}
}

func TestNetStackSegmentAdd(t *testing.T) {
	a := NetStackSegment{RetransSegs: 5, CurrEstabSum: 100, CurrEstabMax: 60, N: 1}
	b := NetStackSegment{RetransSegs: 7, CurrEstabSum: 20, CurrEstabMax: 20, N: 1}

	got := a
	got.Add(&b)

	if got.RetransSegs != 12 || got.N != 2 {
		t.Errorf("got %+v, want retrans 12 N 2", got)
	}
	if got.CurrEstabSum != 120 {
		t.Errorf("CurrEstabSum = %d, want 120", got.CurrEstabSum)
	}
	// A gauge's max identifies the node or bucket that spiked; the mean is
	// Sum/N and is not sent.
	if got.CurrEstabMax != 60 {
		t.Errorf("CurrEstabMax = %d, want 60", got.CurrEstabMax)
	}

	rev := b
	rev.Add(&a)
	if rev != got {
		t.Errorf("order dependent: %+v vs %+v", rev, got)
	}
}

// The zero value must be an Add identity, which internal/windowed requires:
// newFn() followed by Add(v) has to reproduce v.
func TestNetStackSegmentZeroIsAddIdentity(t *testing.T) {
	v := NetStackSegment{RetransSegs: 3, CurrEstabSum: 9, CurrEstabMax: 9, N: 2}

	var got NetStackSegment
	got.Add(&v)
	if got != v {
		t.Errorf("zero+Add(%+v) = %+v", v, got)
	}

	got.Add(nil)
	got.Add(&NetStackSegment{})
	if got != v {
		t.Errorf("Add(nil)/Add(empty) mutated: %+v, want %+v", got, v)
	}
}

func TestNetMetricsMergeAggregates(t *testing.T) {
	nodeA := &NetMetrics{
		Stack: &NetStackStats{TCPRetransSegs: 10},
		Conns: &NetConnStats{States: map[string]uint64{"ESTABLISHED": 4}},
		Links: &NetLinkStats{N: 1, OperStates: map[string]int{"up": 1}},
	}
	nodeB := &NetMetrics{
		Stack: &NetStackStats{TCPRetransSegs: 5},
		Conns: &NetConnStats{States: map[string]uint64{"ESTABLISHED": 6}},
		Links: &NetLinkStats{N: 1, OperStates: map[string]int{"down": 1}},
	}

	var got NetMetrics
	got.Merge(nodeA)
	got.Merge(nodeB)

	if got.Stack == nil || got.Stack.TCPRetransSegs != 15 {
		t.Errorf("Stack = %+v, want retrans 15", got.Stack)
	}
	if got.Conns == nil || got.Conns.States["ESTABLISHED"] != 10 {
		t.Errorf("Conns = %+v, want 10 established", got.Conns)
	}
	if got.Links == nil || got.Links.N != 2 {
		t.Errorf("Links = %+v, want N=2", got.Links)
	}
}

// A node reporting nothing must not blank what other nodes reported.
func TestNetMetricsMergeNilAggregates(t *testing.T) {
	full := &NetMetrics{
		Stack: &NetStackStats{TCPRetransSegs: 10},
		Conns: &NetConnStats{States: map[string]uint64{"ESTABLISHED": 4}},
		Links: &NetLinkStats{N: 1},
	}

	var got NetMetrics
	got.Merge(full)
	got.Merge(&NetMetrics{})

	if got.Stack == nil || got.Stack.TCPRetransSegs != 10 {
		t.Errorf("Stack = %+v, want preserved", got.Stack)
	}
	if got.Conns == nil || got.Links == nil {
		t.Error("Conns/Links were blanked by a node that reported neither")
	}
}
