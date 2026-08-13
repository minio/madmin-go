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
)

func TestMemVMStatMerge(t *testing.T) {
	a := &MemVMStat{SwapInBytes: 4096, MajorFaults: 10, OOMKill: 1, CompactStall: 3}
	b := &MemVMStat{SwapInBytes: 8192, MajorFaults: 5, WorkingsetRefault: 7}

	var got MemVMStat
	got.Merge(a)
	got.Merge(b)
	got.Merge(nil)

	want := MemVMStat{
		SwapInBytes: 12288, MajorFaults: 15, OOMKill: 1,
		WorkingsetRefault: 7, CompactStall: 3,
	}
	if got != want {
		t.Errorf("Merge = %+v, want %+v", got, want)
	}
}

func TestMemCgroupStatsMerge(t *testing.T) {
	a := &MemCgroupStats{Current: 100, Peak: 200, Max: 1000, Events: map[string]uint64{"high": 2}}
	b := &MemCgroupStats{Current: 300, Peak: 400, Max: 1000, Events: map[string]uint64{"high": 1, "oom_kill": 1}}

	var got MemCgroupStats
	got.Merge(a)
	got.Merge(b)

	if got.Current != 400 || got.Peak != 600 {
		t.Errorf("Current/Peak = %d/%d, want 400/600", got.Current, got.Peak)
	}
	// Max sums so the cluster's aggregate limit is comparable to its aggregate
	// Current.
	if got.Max != 2000 {
		t.Errorf("Max = %d, want 2000", got.Max)
	}
	if !maps.Equal(got.Events, map[string]uint64{"high": 3, "oom_kill": 1}) {
		t.Errorf("Events = %v", got.Events)
	}
	// The source maps must survive, since Merge is handed another node's report.
	if !maps.Equal(a.Events, map[string]uint64{"high": 2}) {
		t.Errorf("argument mutated: %v", a.Events)
	}
}

// "No ECC hardware" and "ECC hardware reporting zero errors" must stay
// distinguishable, which is what the DIMM counts are for.
func TestMemECCStatsMerge(t *testing.T) {
	healthy := &MemECCStats{Controllers: 1, DIMMs: 8}
	failing := &MemECCStats{
		Controllers: 1, DIMMs: 8,
		Corrected: 4096, DIMMsWithCorrected: 1,
		Uncorrected: 1, DIMMsWithUncorrected: 1,
		HardwareCorruptedBytes: 4096,
	}

	var got MemECCStats
	got.Merge(healthy)
	got.Merge(failing)

	if got.DIMMs != 16 || got.Controllers != 2 {
		t.Errorf("inventory = %d DIMMs on %d controllers, want 16/2", got.DIMMs, got.Controllers)
	}
	// 4096 errors on one of sixteen DIMMs is a stick to replace; the same total
	// spread over sixteen would be a different problem entirely.
	if got.Corrected != 4096 || got.DIMMsWithCorrected != 1 {
		t.Errorf("corrected = %d over %d DIMMs, want 4096/1", got.Corrected, got.DIMMsWithCorrected)
	}
	if got.HardwareCorruptedBytes != 4096 {
		t.Errorf("HardwareCorruptedBytes = %d, want 4096", got.HardwareCorruptedBytes)
	}
}

func TestMemFragStatsMerge(t *testing.T) {
	const page = 4096
	a := &MemFragStats{
		Zones: 2, PageSize: page, LargeOrderBytes: 2 << 20,
		FreeBytes: 1000, FreeBytesLarge: 800,
		ByZone: map[string]MemZoneFrag{
			"0/Normal": {FreeBytes: 900, FreeBytesLarge: 800, Orders: []uint64{1, 2, 3}},
			"0/DMA":    {FreeBytes: 100},
		},
	}
	b := &MemFragStats{
		Zones: 1, PageSize: page, LargeOrderBytes: 2 << 20,
		FreeBytes: 500, FreeBytesLarge: 100,
		ByZone: map[string]MemZoneFrag{
			// Longer Orders than a's: a merge must not truncate or panic.
			"0/Normal": {FreeBytes: 500, FreeBytesLarge: 100, Orders: []uint64{4, 5, 6, 7}},
		},
	}

	var got MemFragStats
	got.Merge(a)
	got.Merge(b)

	if got.Zones != 3 || got.PageSize != page {
		t.Errorf("Zones/PageSize = %d/%d, want 3/%d", got.Zones, got.PageSize, page)
	}
	if got.FreeBytes != 1500 || got.FreeBytesLarge != 900 {
		t.Errorf("free = %d/%d, want 1500/900", got.FreeBytes, got.FreeBytesLarge)
	}
	// The unusable-free-space index is derived, never carried: 1 - 900/1500.
	if idx := 1 - float64(got.FreeBytesLarge)/float64(got.FreeBytes); idx != 0.4 {
		t.Errorf("derived index = %v, want 0.4", idx)
	}
	normal := got.ByZone["0/Normal"]
	if !reflect.DeepEqual(normal.Orders, []uint64{5, 7, 9, 7}) {
		t.Errorf("Orders = %v, want [5 7 9 7]", normal.Orders)
	}
	if got.ByZone["0/DMA"].FreeBytes != 100 {
		t.Errorf("a zone only one host reported was lost: %+v", got.ByZone)
	}
}

// Orders is only interpretable while PageSize is known, so a merge across hosts
// with different page sizes must blank it rather than pick one.
func TestMemFragStatsMergeMixedPageSize(t *testing.T) {
	a := &MemFragStats{Zones: 1, PageSize: 4096, FreeBytes: 100}
	b := &MemFragStats{Zones: 1, PageSize: 65536, FreeBytes: 200}

	var got MemFragStats
	got.Merge(a)
	got.Merge(b)

	if got.PageSize != 0 {
		t.Errorf("PageSize = %d, want 0 once hosts disagree", got.PageSize)
	}
	// The byte totals stay valid across the disagreement.
	if got.FreeBytes != 300 {
		t.Errorf("FreeBytes = %d, want 300", got.FreeBytes)
	}
}

// collectRealtimeMetrics merges remote reports first and the local one last, so
// every merge has to be order-independent or the local node systematically wins.
func TestMemMergeIsOrderIndependent(t *testing.T) {
	a := &MemMetrics{
		Nodes:  1,
		VMStat: &MemVMStat{SwapInBytes: 4096, OOMKill: 1},
		Cgroup: &MemCgroupStats{Current: 10, Max: 100, Events: map[string]uint64{"oom": 1}},
		ECC:    &MemECCStats{DIMMs: 8, Corrected: 2, DIMMsWithCorrected: 1},
		Fragmentation: &MemFragStats{
			Zones: 1, PageSize: 4096, LargeOrderBytes: 2 << 20,
			FreeBytes: 100, FreeBytesLarge: 40,
			ByZone: map[string]MemZoneFrag{"0/Normal": {FreeBytes: 100, Orders: []uint64{1}}},
		},
	}
	// A host with no cgroup and no ECC hardware: nil sections must not blank a
	// real report from another host, in either order.
	b := &MemMetrics{
		Nodes:  1,
		VMStat: &MemVMStat{SwapInBytes: 8192},
		Fragmentation: &MemFragStats{
			Zones: 1, PageSize: 4096, LargeOrderBytes: 2 << 20,
			FreeBytes: 300, FreeBytesLarge: 200,
			ByZone: map[string]MemZoneFrag{"0/Normal": {FreeBytes: 300, Orders: []uint64{2, 3}}},
		},
	}

	var ab, ba MemMetrics
	ab.Merge(a)
	ab.Merge(b)
	ba.Merge(b)
	ba.Merge(a)

	if !reflect.DeepEqual(ab, ba) {
		t.Errorf("order dependent:\n%+v\n%+v", ab, ba)
	}
	if ab.Cgroup == nil || ab.Cgroup.Current != 10 {
		t.Errorf("a node without cgroup v2 blanked the section: %+v", ab.Cgroup)
	}
	if ab.ECC == nil || ab.ECC.DIMMs != 8 {
		t.Errorf("a node without ECC hardware blanked the section: %+v", ab.ECC)
	}
}

// windowed.SaveStats snapshots each element via Add, so a field missing from Add
// is silently never persisted even though msgp round-trips it.
func TestMemSegmentAddCoversAllFields(t *testing.T) {
	src := MemSegment{
		Used: 1, Free: 2, Available: 3, Limit: 4, N: 5,
		SwapInBytes: 6, SwapOutBytes: 7, MajorFaults: 8, WorkingsetRefault: 9,
		CompactStall: 10, OOMKill: 11,
		FragFreeBytes: 12, FragFreeBytesLarge: 13, FragN: 14,
	}

	var got MemSegment
	got.Add(&src)
	if got != src {
		t.Errorf("Add lost a field:\n got %+v\nwant %+v", got, src)
	}

	// Every field is a plain sum, so adding twice must double every one -- this
	// is what makes newFn() the additive identity with no sentinel convention.
	got.Add(&src)
	v := reflect.ValueOf(got)
	s := reflect.ValueOf(src)
	for i := range v.NumField() {
		name := v.Type().Field(i).Name
		switch v.Field(i).Kind() {
		case reflect.Uint64:
			if v.Field(i).Uint() != 2*s.Field(i).Uint() {
				t.Errorf("%s did not sum: %d", name, v.Field(i).Uint())
			}
		case reflect.Int:
			if v.Field(i).Int() != 2*s.Field(i).Int() {
				t.Errorf("%s did not sum: %d", name, v.Field(i).Int())
			}
		default:
			t.Errorf("%s has kind %v; a segment field must be a plain summable "+
				"counter", name, v.Field(i).Kind())
		}
	}
}
