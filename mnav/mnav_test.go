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
	"time"

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

func leafValue(data map[string]string, key string) string {
	for k, v := range data {
		if k == key || strings.HasSuffix(k, ":"+key) {
			return v
		}
	}
	return ""
}

func TestByTimeNavigation(t *testing.T) {
	t0 := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)
	seg := func(reqs int64, secs float64) madmin.APIStats {
		return madmin.APIStats{Requests: reqs, RequestTimeSecs: secs}
	}
	m := &madmin.RealtimeMetrics{Aggregated: madmin.Metrics{API: &madmin.APIMetrics{
		Nodes: 1,
		LastDayAPI: map[string]madmin.SegmentedAPIMetrics{
			// 3 segments starting at t0; the first is empty.
			"s3.GetObject": {Interval: 900, FirstTime: t0, Segments: []madmin.APIStats{seg(0, 0), seg(10, 1.0), seg(20, 2.0)}},
			// Starts one interval later, so its segment 0 aligns with GetObject's segment 1.
			"s3.PutObject": {Interval: 900, FirstTime: t0.Add(15 * time.Minute), Segments: []madmin.APIStats{seg(5, 0.5), seg(7, 0.7)}},
		},
	}}}
	nav := NewRealtimeMetricsNavigator(m)

	seg15 := t0.Add(15 * time.Minute).UTC().Format("15:04Z")
	seg30 := t0.Add(30 * time.Minute).UTC().Format("15:04Z")

	// _by_time lists non-empty segments, newest first (the empty t0 slot is skipped).
	bt, err := nav.Navigate("api/last_day/_by_time")
	if err != nil {
		t.Fatalf("navigate _by_time: %v", err)
	}
	if got, want := childNames(bt.GetChildren()), []string{seg30, seg15}; !reflect.DeepEqual(got, want) {
		t.Fatalf("_by_time segments = %v, want %v", got, want)
	}

	// A segment lists _ALL first, then the operations active in it, sorted.
	s15, err := nav.Navigate("api/last_day/_by_time/" + seg15)
	if err != nil {
		t.Fatalf("navigate segment %s: %v", seg15, err)
	}
	if got, want := childNames(s15.GetChildren()), []string{"_ALL", "s3.GetObject", "s3.PutObject"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("segment ops = %v, want %v", got, want)
	}

	// Per-op stats are matched by timestamp, not slot index: at seg15 PutObject must
	// use its own segment 0 (5 reqs), not grid slot index 1 (7 reqs).
	put15, err := nav.Navigate("api/last_day/_by_time/" + seg15 + "/s3.PutObject")
	if err != nil {
		t.Fatalf("navigate PutObject@%s: %v", seg15, err)
	}
	if got := leafValue(put15.GetLeafData(), "Total Requests"); got != "5" {
		t.Errorf("PutObject@%s Total Requests = %q, want 5 (timestamp match, not index)", seg15, got)
	}

	// Segment summary == sum of its operations (10+5 = 15), shown both via _ALL and
	// as the segment node's own leaf data.
	all15, err := nav.Navigate("api/last_day/_by_time/" + seg15 + "/_ALL")
	if err != nil {
		t.Fatalf("navigate _ALL@%s: %v", seg15, err)
	}
	if got := leafValue(all15.GetLeafData(), "Total Requests"); got != "15" {
		t.Errorf("_ALL@%s Total Requests = %q, want 15", seg15, got)
	}
	if got := leafValue(s15.GetLeafData(), "Total Requests"); got != "15" {
		t.Errorf("segment %s leaf Total Requests = %q, want 15", seg15, got)
	}

	// At seg30 the summary is 20+7 = 27 (index-based matching would drop PutObject -> 20).
	all30, err := nav.Navigate("api/last_day/_by_time/" + seg30 + "/_ALL")
	if err != nil {
		t.Fatalf("navigate _ALL@%s: %v", seg30, err)
	}
	if got := leafValue(all30.GetLeafData(), "Total Requests"); got != "27" {
		t.Errorf("_ALL@%s Total Requests = %q, want 27", seg30, got)
	}

	// Leaf carries API type + day-stats flag via opts propagation.
	if opts := put15.GetOpts(); opts.Type&madmin.MetricsAPI == 0 || opts.Flags&madmin.MetricsDayStats == 0 {
		t.Errorf("opts Type=%v Flags=%v, want MetricsAPI + MetricsDayStats", opts.Type, opts.Flags)
	}
}

// TestByTimeAllViews checks that every wired view exposes _by_time as its first
// last_day child and that the full segment -> operation -> stats path resolves.
func TestByTimeAllViews(t *testing.T) {
	t0 := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)
	m := &madmin.RealtimeMetrics{Aggregated: madmin.Metrics{
		RPC: &madmin.RPCMetrics{LastDay: map[string]madmin.SegmentedRPCMetrics{
			"storageRPC": {Interval: 900, FirstTime: t0, Segments: []madmin.RPCStats{{Requests: 3, RequestTimeSecs: 0.3}}},
		}},
		Disk: &madmin.DiskMetric{LastDaySegmented: map[string]madmin.SegmentedDiskActions{
			"WalkDir": {Interval: 900, FirstTime: t0, Segments: []madmin.DiskAction{{Count: 4, AccTime: 0.4}}},
		}},
		KMS: &madmin.KMSRtMetrics{LastDay: map[string]madmin.SegmentedKMSActions{
			"encrypt": {Interval: 900, FirstTime: t0, Segments: []madmin.KMSAction{{Count: 2}}},
		}},
	}}
	nav := NewRealtimeMetricsNavigator(m)
	segName := t0.UTC().Format("15:04Z")

	for _, tc := range []struct{ lastDay, op string }{
		{"rpc/last_day", "storageRPC"},
		{"drive/ops_last_day", "WalkDir"},
		{"kms/last_day", "encrypt"},
	} {
		ld, err := nav.Navigate(tc.lastDay)
		if err != nil {
			t.Fatalf("navigate %s: %v", tc.lastDay, err)
		}
		if got := childNames(ld.GetChildren()); len(got) == 0 || got[0] != "_by_time" {
			t.Errorf("%s first child = %v, want _by_time first", tc.lastDay, got)
		}
		for _, leaf := range []string{tc.op, "_ALL"} {
			path := tc.lastDay + "/_by_time/" + segName + "/" + leaf
			node, err := nav.Navigate(path)
			if err != nil {
				t.Fatalf("navigate %s: %v", path, err)
			}
			if len(node.GetLeafData()) == 0 {
				t.Errorf("%s: leaf has no data", path)
			}
		}
	}
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
