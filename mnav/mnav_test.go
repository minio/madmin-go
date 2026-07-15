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
	"reflect"
	"strings"
	"testing"

	"github.com/minio/madmin-go/v4"
)

func TestDiskSetNavigation(t *testing.T) {
	m := &madmin.RealtimeMetrics{
		ByDiskSet: map[int]map[int]madmin.DiskMetric{
			2: {0: {NDisks: 4}, 1: {NDisks: 4}},
			0: {0: {NDisks: 8}},
			1: {0: {NDisks: 2}, 1: {NDisks: 2}, 2: {NDisks: 2}},
		},
	}
	nav := NewRealtimeMetricsNavigator(m)

	// Pools are listed in numeric order with zero-padded names.
	bds, err := nav.Navigate("by_drive_set")
	if err != nil {
		t.Fatalf("navigate by_drive_set: %v", err)
	}
	if got, want := childNames(bds.GetChildren()), []string{"pool_00", "pool_01", "pool_02"}; !reflect.DeepEqual(got, want) {
		t.Errorf("pool order = %v, want %v", got, want)
	}

	// Inside a pool: _ALL first, then sets in numeric order with zero-padded names.
	pool, err := nav.Navigate("by_drive_set/pool_01")
	if err != nil {
		t.Fatalf("navigate pool_01: %v", err)
	}
	if got, want := childNames(pool.GetChildren()), []string{"_ALL", "set_0000", "set_0001", "set_0002"}; !reflect.DeepEqual(got, want) {
		t.Errorf("set order = %v, want %v", got, want)
	}

	// A set node carries both its pool and set index in the refresh opts.
	set, err := nav.Navigate("by_drive_set/pool_02/set_0001")
	if err != nil {
		t.Fatalf("navigate set_0001: %v", err)
	}
	if opts := set.GetOpts(); !reflect.DeepEqual(opts.PoolIdx, []int{2}) || !reflect.DeepEqual(opts.DriveSetIdx, []int{1}) {
		t.Errorf("set opts PoolIdx=%v DriveSetIdx=%v, want [2] [1]", opts.PoolIdx, opts.DriveSetIdx)
	}

	// _ALL node is scoped to the whole pool: pool index set, no set restriction.
	all, err := nav.Navigate("by_drive_set/pool_02/_ALL")
	if err != nil {
		t.Fatalf("navigate _ALL: %v", err)
	}
	if opts := all.GetOpts(); !reflect.DeepEqual(opts.PoolIdx, []int{2}) || len(opts.DriveSetIdx) != 0 {
		t.Errorf("_ALL opts PoolIdx=%v DriveSetIdx=%v, want [2] and empty", opts.PoolIdx, opts.DriveSetIdx)
	}
}

func childNames(children []MetricChild) []string {
	out := make([]string, len(children))
	for i, c := range children {
		out[i] = c.Name
	}
	return out
}

func TestDiskSetLeafData(t *testing.T) {
	m := &madmin.RealtimeMetrics{
		ByDiskSet: map[int]map[int]madmin.DiskMetric{
			0: {
				0: {
					NDisks:        4,
					IOStatsMinute: madmin.DiskIOStats{N: 4, ReadIOs: 3000, WriteIOs: 3000, ReadSectors: 60000, WriteSectors: 60000},
					Cache:         &madmin.CacheStats{N: 4, Capacity: 1 << 30, Used: 1 << 29, Hits: 80, Misses: 20},
				},
			},
		},
	}
	nav := NewRealtimeMetricsNavigator(m)

	// Pool summary reports both IO rate and bandwidth.
	bds, err := nav.Navigate("by_drive_set")
	if err != nil {
		t.Fatalf("navigate by_drive_set: %v", err)
	}
	if pool0 := bds.GetLeafData()["Pool 0"]; !strings.Contains(pool0, "IO/s") || !strings.Contains(pool0, "MB/s") {
		t.Errorf("Pool 0 summary = %q, want both IO/s and MB/s", pool0)
	}

	// _ALL and individual sets surface cache capacity + hit rate.
	for _, path := range []string{"by_drive_set/pool_00/_ALL", "by_drive_set/pool_00/set_0000"} {
		node, err := nav.Navigate(path)
		if err != nil {
			t.Fatalf("navigate %s: %v", path, err)
		}
		d := node.GetLeafData()
		if _, ok := d["Cache Capacity"]; !ok {
			t.Errorf("%s: missing Cache Capacity", path)
		}
		if got := d["Cache Hit Rate"]; got != "80.0%" {
			t.Errorf("%s: Cache Hit Rate = %q, want 80.0%%", path, got)
		}
	}
}
