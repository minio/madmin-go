//
// Copyright (c) 2015-2025 MinIO, Inc.
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
	"reflect"
	"testing"
	"time"

	"github.com/prometheus/procfs"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/load"
)

// TestScannerMetricsMerge tests ScannerMetrics.Merge functionality
func TestScannerMetricsMerge(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour)

	tests := []struct {
		name   string
		base   *ScannerMetrics
		other  *ScannerMetrics
		verify func(t *testing.T, result *ScannerMetrics)
	}{
		{
			name: "merge nil other",
			base: &ScannerMetrics{
				CollectedAt:    now,
				OngoingBuckets: 5,
			},
			other: nil,
			verify: func(t *testing.T, result *ScannerMetrics) {
				if result.OngoingBuckets != 5 {
					t.Errorf("OngoingBuckets = %d, want 5", result.OngoingBuckets)
				}
			},
		},
		{
			name: "merge with later timestamp",
			base: &ScannerMetrics{
				CollectedAt:    now,
				OngoingBuckets: 5,
			},
			other: &ScannerMetrics{
				CollectedAt:    later,
				OngoingBuckets: 10,
			},
			verify: func(t *testing.T, result *ScannerMetrics) {
				if !result.CollectedAt.Equal(later) {
					t.Errorf("CollectedAt = %v, want %v", result.CollectedAt, later)
				}
				if result.OngoingBuckets != 10 {
					t.Errorf("OngoingBuckets = %d, want 10", result.OngoingBuckets)
				}
			},
		},
		{
			name: "merge lifetime ops",
			base: &ScannerMetrics{
				LifeTimeOps: map[string]uint64{"op1": 100, "op2": 200},
			},
			other: &ScannerMetrics{
				LifeTimeOps: map[string]uint64{"op1": 50, "op3": 300},
			},
			verify: func(t *testing.T, result *ScannerMetrics) {
				expected := map[string]uint64{"op1": 150, "op2": 200, "op3": 300}
				if !reflect.DeepEqual(result.LifeTimeOps, expected) {
					t.Errorf("LifeTimeOps = %v, want %v", result.LifeTimeOps, expected)
				}
			},
		},
		{
			name: "merge excessive prefixes",
			base: &ScannerMetrics{
				ExcessivePrefixes: []string{"prefix1", "prefix2"},
			},
			other: &ScannerMetrics{
				ExcessivePrefixes: []string{"prefix2", "prefix3"},
			},
			verify: func(t *testing.T, result *ScannerMetrics) {
				// Should be deduplicated and sorted
				if len(result.ExcessivePrefixes) != 3 {
					t.Errorf("ExcessivePrefixes length = %d, want 3", len(result.ExcessivePrefixes))
				}
				// Check sorted order
				expected := []string{"prefix1", "prefix2", "prefix3"}
				if !reflect.DeepEqual(result.ExcessivePrefixes, expected) {
					t.Errorf("ExcessivePrefixes = %v, want %v", result.ExcessivePrefixes, expected)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.base.Merge(tt.other)
			tt.verify(t, tt.base)
		})
	}
}

// TestDiskMetricMerge tests DiskMetric.Merge functionality
func TestDiskMetricMerge(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour)

	tests := []struct {
		name   string
		base   *DiskMetric
		other  *DiskMetric
		verify func(t *testing.T, result *DiskMetric)
	}{
		{
			name:  "merge nil other",
			base:  &DiskMetric{NDisks: 5},
			other: nil,
			verify: func(t *testing.T, result *DiskMetric) {
				if result.NDisks != 5 {
					t.Errorf("NDisks = %d, want 5", result.NDisks)
				}
			},
		},
		{
			name:  "merge empty base",
			base:  &DiskMetric{},
			other: &DiskMetric{NDisks: 10, Offline: 2},
			verify: func(t *testing.T, result *DiskMetric) {
				if result.NDisks != 10 {
					t.Errorf("NDisks = %d, want 10", result.NDisks)
				}
				if result.Offline != 2 {
					t.Errorf("Offline = %d, want 2", result.Offline)
				}
			},
		},
		{
			name: "merge disk counts",
			base: &DiskMetric{
				CollectedAt: now,
				NDisks:      5,
				Offline:     1,
				Healing:     2,
				Hanging:     1,
			},
			other: &DiskMetric{
				CollectedAt: later,
				NDisks:      3,
				Offline:     2,
				Healing:     1,
				Hanging:     2,
			},
			verify: func(t *testing.T, result *DiskMetric) {
				if !result.CollectedAt.Equal(later) {
					t.Errorf("CollectedAt = %v, want %v", result.CollectedAt, later)
				}
				if result.NDisks != 8 {
					t.Errorf("NDisks = %d, want 8", result.NDisks)
				}
				if result.Offline != 3 {
					t.Errorf("Offline = %d, want 3", result.Offline)
				}
				if result.Healing != 3 {
					t.Errorf("Healing = %d, want 3", result.Healing)
				}
				if result.Hanging != 3 {
					t.Errorf("Hanging = %d, want 3", result.Hanging)
				}
			},
		},
		{
			name: "merge io stats",
			base: &DiskMetric{
				NDisks:        1, // Need non-zero NDisks to actually merge
				IOStatsMinute: DiskIOStats{N: 1, ReadIOs: 100, WriteIOs: 200},
			},
			other: &DiskMetric{
				NDisks:        1,
				IOStatsMinute: DiskIOStats{N: 1, ReadIOs: 50, WriteIOs: 100},
			},
			verify: func(t *testing.T, result *DiskMetric) {
				if result.NDisks != 2 {
					t.Errorf("NDisks = %d, want 2", result.NDisks)
				}
				if result.IOStatsMinute.N != 2 {
					t.Errorf("IOStatsMinute.N = %d, want 2", result.IOStatsMinute.N)
				}
				if result.IOStatsMinute.ReadIOs != 150 {
					t.Errorf("IOStatsMinute.ReadIOs = %d, want 150", result.IOStatsMinute.ReadIOs)
				}
				if result.IOStatsMinute.WriteIOs != 300 {
					t.Errorf("IOStatsMinute.WriteIOs = %d, want 300", result.IOStatsMinute.WriteIOs)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.base.Merge(tt.other)
			tt.verify(t, tt.base)
		})
	}
}

// TestOSMetricsMerge tests OSMetrics.Merge functionality
func TestOSMetricsMerge(t *testing.T) {
	tests := []struct {
		name   string
		base   *OSMetrics
		other  *OSMetrics
		verify func(t *testing.T, result *OSMetrics)
	}{
		{
			name:  "merge nil other",
			base:  &OSMetrics{},
			other: nil,
			verify: func(_ *testing.T, _ *OSMetrics) {
				// Should not panic
			},
		},
		{
			name: "merge lifetime ops",
			base: &OSMetrics{
				LifeTimeOps: map[string]uint64{"read": 1000, "write": 2000},
			},
			other: &OSMetrics{
				LifeTimeOps: map[string]uint64{"read": 500, "delete": 100},
			},
			verify: func(t *testing.T, result *OSMetrics) {
				expected := map[string]uint64{"read": 1500, "write": 2000, "delete": 100}
				if !reflect.DeepEqual(result.LifeTimeOps, expected) {
					t.Errorf("LifeTimeOps = %v, want %v", result.LifeTimeOps, expected)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.base.Merge(tt.other)
			tt.verify(t, tt.base)
		})
	}
}

// TestMemMetricsMerge tests MemMetrics.Merge functionality
func TestMemMetricsMerge(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour)

	tests := []struct {
		name   string
		base   *MemMetrics
		other  *MemMetrics
		verify func(t *testing.T, result *MemMetrics)
	}{
		{
			name:  "merge nil other",
			base:  &MemMetrics{},
			other: nil,
			verify: func(_ *testing.T, _ *MemMetrics) {
				// Should not panic
			},
		},
		{
			name: "merge memory values",
			base: &MemMetrics{
				CollectedAt: now,
				Nodes:       2,
				Info: MemInfo{
					Total:          8000,
					Used:           2000,
					Free:           6000,
					Available:      4000,
					Shared:         100,
					Cache:          500,
					Buffers:        300,
					SwapSpaceTotal: 2000,
					SwapSpaceFree:  1000,
					Limit:          8000,
				},
			},
			other: &MemMetrics{
				CollectedAt: later,
				Nodes:       3,
				Info: MemInfo{
					Total:          8000,
					Used:           3000,
					Free:           5000,
					Available:      3000,
					Shared:         150,
					Cache:          400,
					Buffers:        200,
					SwapSpaceTotal: 2000,
					SwapSpaceFree:  500,
					Limit:          8000,
				},
			},
			verify: func(t *testing.T, result *MemMetrics) {
				if !result.CollectedAt.Equal(later) {
					t.Errorf("CollectedAt = %v, want %v", result.CollectedAt, later)
				}
				if result.Nodes != 5 {
					t.Errorf("Nodes = %d, want 5", result.Nodes)
				}
				// Verify all MemInfo fields are merged (summed)
				if result.Info.Total != 16000 {
					t.Errorf("Info.Total = %d, want 16000", result.Info.Total)
				}
				if result.Info.Used != 5000 {
					t.Errorf("Info.Used = %d, want 5000", result.Info.Used)
				}
				if result.Info.Free != 11000 {
					t.Errorf("Info.Free = %d, want 11000", result.Info.Free)
				}
				if result.Info.Available != 7000 {
					t.Errorf("Info.Available = %d, want 7000", result.Info.Available)
				}
				if result.Info.Shared != 250 {
					t.Errorf("Info.Shared = %d, want 250", result.Info.Shared)
				}
				if result.Info.Cache != 900 {
					t.Errorf("Info.Cache = %d, want 900", result.Info.Cache)
				}
				if result.Info.Buffers != 500 {
					t.Errorf("Info.Buffers = %d, want 500", result.Info.Buffers)
				}
				if result.Info.SwapSpaceTotal != 4000 {
					t.Errorf("Info.SwapSpaceTotal = %d, want 4000", result.Info.SwapSpaceTotal)
				}
				if result.Info.SwapSpaceFree != 1500 {
					t.Errorf("Info.SwapSpaceFree = %d, want 1500", result.Info.SwapSpaceFree)
				}
				if result.Info.Limit != 16000 {
					t.Errorf("Info.Limit = %d, want 16000", result.Info.Limit)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.base.Merge(tt.other)
			tt.verify(t, tt.base)
		})
	}
}

// TestCPUMetricsMerge tests CPUMetrics.Merge functionality
func TestCPUMetricsMerge(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour)

	tests := []struct {
		name   string
		base   *CPUMetrics
		other  *CPUMetrics
		verify func(t *testing.T, result *CPUMetrics)
	}{
		{
			name:  "merge nil other",
			base:  &CPUMetrics{},
			other: nil,
			verify: func(_ *testing.T, _ *CPUMetrics) {
				// Should not panic
			},
		},
		{
			name: "merge cpu stats",
			base: &CPUMetrics{
				CollectedAt: now,
				Nodes:       2,
				TimesStat: &cpu.TimesStat{
					User:   100,
					System: 50,
					Idle:   1000,
				},
				LoadStat: &load.AvgStat{
					Load1:  1.5,
					Load5:  2.0,
					Load15: 1.8,
				},
				CPUCount: 4,
			},
			other: &CPUMetrics{
				CollectedAt: later,
				Nodes:       3,
				TimesStat: &cpu.TimesStat{
					User:   50,
					System: 25,
					Idle:   500,
				},
				LoadStat: &load.AvgStat{
					Load1:  0.5,
					Load5:  1.0,
					Load15: 0.8,
				},
				CPUCount: 4,
			},
			verify: func(t *testing.T, result *CPUMetrics) {
				if !result.CollectedAt.Equal(later) {
					t.Errorf("CollectedAt = %v, want %v", result.CollectedAt, later)
				}
				if result.Nodes != 5 {
					t.Errorf("Nodes = %d, want 5", result.Nodes)
				}
				if result.TimesStat.User != 150 {
					t.Errorf("TimesStat.User = %f, want 150", result.TimesStat.User)
				}
				if result.TimesStat.System != 75 {
					t.Errorf("TimesStat.System = %f, want 75", result.TimesStat.System)
				}
				if result.TimesStat.Idle != 1500 {
					t.Errorf("TimesStat.Idle = %f, want 1500", result.TimesStat.Idle)
				}
				if result.LoadStat.Load1 != 2.0 {
					t.Errorf("LoadStat.Load1 = %f, want 2.0", result.LoadStat.Load1)
				}
				if result.LoadStat.Load5 != 3.0 {
					t.Errorf("LoadStat.Load5 = %f, want 3.0", result.LoadStat.Load5)
				}
				if result.LoadStat.Load15 != 2.6 {
					t.Errorf("LoadStat.Load15 = %f, want 2.6", result.LoadStat.Load15)
				}
				if result.CPUCount != 8 {
					t.Errorf("CPUCount = %d, want 8", result.CPUCount)
				}
			},
		},
		{
			name: "merge nil TimesStat",
			base: &CPUMetrics{
				Nodes:     1,
				TimesStat: nil,
				CPUCount:  2,
			},
			other: &CPUMetrics{
				Nodes: 1,
				TimesStat: &cpu.TimesStat{
					User: 100,
				},
				CPUCount: 2,
			},
			verify: func(t *testing.T, result *CPUMetrics) {
				if result.Nodes != 2 {
					t.Errorf("Nodes = %d, want 2", result.Nodes)
				}
				if result.TimesStat == nil {
					t.Error("TimesStat is nil, should be set")
				}
				if result.TimesStat != nil && result.TimesStat.User != 100 {
					t.Errorf("TimesStat.User = %f, want 100", result.TimesStat.User)
				}
				if result.CPUCount != 4 {
					t.Errorf("CPUCount = %d, want 4", result.CPUCount)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.base.Merge(tt.other)
			tt.verify(t, tt.base)
		})
	}
}

// TestAPIMetricsMerge tests APIMetrics.Merge functionality
func TestAPIMetricsMerge(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour)

	tests := []struct {
		name   string
		base   *APIMetrics
		other  *APIMetrics
		verify func(t *testing.T, result *APIMetrics)
	}{
		{
			name:  "merge nil other",
			base:  &APIMetrics{},
			other: nil,
			verify: func(_ *testing.T, _ *APIMetrics) {
				// Should not panic
			},
		},
		{
			name: "merge basic fields",
			base: &APIMetrics{
				CollectedAt:    now,
				Nodes:          2,
				ActiveRequests: 100,
				QueuedRequests: 50,
			},
			other: &APIMetrics{
				CollectedAt:    later,
				Nodes:          3,
				ActiveRequests: 200,
				QueuedRequests: 75,
			},
			verify: func(t *testing.T, result *APIMetrics) {
				if !result.CollectedAt.Equal(later) {
					t.Errorf("CollectedAt = %v, want %v", result.CollectedAt, later)
				}
				if result.Nodes != 5 {
					t.Errorf("Nodes = %d, want 5", result.Nodes)
				}
				if result.ActiveRequests != 300 {
					t.Errorf("ActiveRequests = %d, want 300", result.ActiveRequests)
				}
				if result.QueuedRequests != 125 {
					t.Errorf("QueuedRequests = %d, want 125", result.QueuedRequests)
				}
			},
		},
		{
			name: "merge LastMinuteAPI maps",
			base: &APIMetrics{
				LastMinuteAPI: map[string]APIStats{
					"GET": {Requests: 100},
					"PUT": {Requests: 50},
				},
			},
			other: &APIMetrics{
				LastMinuteAPI: map[string]APIStats{
					"GET":    {Requests: 200},
					"DELETE": {Requests: 25},
				},
			},
			verify: func(t *testing.T, result *APIMetrics) {
				if result.LastMinuteAPI["GET"].Requests != 300 {
					t.Errorf("LastMinuteAPI[GET].Requests = %d, want 300", result.LastMinuteAPI["GET"].Requests)
				}
				if result.LastMinuteAPI["PUT"].Requests != 50 {
					t.Errorf("LastMinuteAPI[PUT].Requests = %d, want 50", result.LastMinuteAPI["PUT"].Requests)
				}
				if result.LastMinuteAPI["DELETE"].Requests != 25 {
					t.Errorf("LastMinuteAPI[DELETE].Requests = %d, want 25", result.LastMinuteAPI["DELETE"].Requests)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.base.Merge(tt.other)
			tt.verify(t, tt.base)
		})
	}
}

// TestReplicationMetricsMerge tests ReplicationMetrics.Merge functionality
func TestReplicationMetricsMerge(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour)

	tests := []struct {
		name   string
		base   *ReplicationMetrics
		other  *ReplicationMetrics
		verify func(t *testing.T, result *ReplicationMetrics)
	}{
		{
			name:  "merge nil other",
			base:  &ReplicationMetrics{},
			other: nil,
			verify: func(_ *testing.T, _ *ReplicationMetrics) {
				// Should not panic
			},
		},
		{
			name: "merge basic fields",
			base: &ReplicationMetrics{
				CollectedAt: now,
				Nodes:       2,
				Active:      100,
				Queued:      50,
			},
			other: &ReplicationMetrics{
				CollectedAt: later,
				Nodes:       3,
				Active:      150,
				Queued:      75,
			},
			verify: func(t *testing.T, result *ReplicationMetrics) {
				if !result.CollectedAt.Equal(later) {
					t.Errorf("CollectedAt = %v, want %v", result.CollectedAt, later)
				}
				if result.Nodes != 5 {
					t.Errorf("Nodes = %d, want 5", result.Nodes)
				}
				if result.Active != 250 {
					t.Errorf("Active = %d, want 250", result.Active)
				}
				if result.Queued != 125 {
					t.Errorf("Queued = %d, want 125", result.Queued)
				}
			},
		},
		{
			name: "merge Targets map - nil base",
			base: &ReplicationMetrics{
				CollectedAt: now,
				Nodes:       2,
			},
			other: &ReplicationMetrics{
				CollectedAt: later,
				Nodes:       3,
				Targets: map[string]ReplicationTargetStats{
					"target1": {
						Nodes:      2,
						LastHour:   ReplicationStats{Nodes: 2, Events: 100, Bytes: 1000, LatencySecs: 10.5, MaxLatencySecs: 5.2},
						SinceStart: ReplicationStats{Nodes: 2, Events: 500, Bytes: 5000},
					},
				},
			},
			verify: func(t *testing.T, result *ReplicationMetrics) {
				if result.Targets == nil {
					t.Fatal("Targets should not be nil after merge")
				}
				if len(result.Targets) != 1 {
					t.Errorf("Targets length = %d, want 1", len(result.Targets))
				}
				target1 := result.Targets["target1"]
				if target1.Nodes != 2 {
					t.Errorf("Targets[target1].Nodes = %d, want 2", target1.Nodes)
				}
				if target1.LastHour.LatencySecs != 10.5 {
					t.Errorf("Targets[target1].LastHour.LatencySecs = %f, want 10.5", target1.LastHour.LatencySecs)
				}
				if target1.LastHour.MaxLatencySecs != 5.2 {
					t.Errorf("Targets[target1].LastHour.MaxLatencySecs = %f, want 5.2", target1.LastHour.MaxLatencySecs)
				}
				if target1.LastHour.Events != 100 {
					t.Errorf("Targets[target1].LastHour.Events = %d, want 100", target1.LastHour.Events)
				}
				if target1.SinceStart.Events != 500 {
					t.Errorf("Targets[target1].SinceStart.Events = %d, want 500", target1.SinceStart.Events)
				}
			},
		},
		{
			name: "merge Targets map - accumulate existing",
			base: &ReplicationMetrics{
				Targets: map[string]ReplicationTargetStats{
					"target1": {
						Nodes:      2,
						LastHour:   ReplicationStats{Nodes: 2, Events: 100, Bytes: 1000, LatencySecs: 10.0, MaxLatencySecs: 4.0},
						SinceStart: ReplicationStats{Nodes: 2, Events: 200, Bytes: 2000},
					},
					"target2": {
						Nodes:      1,
						LastHour:   ReplicationStats{Nodes: 1, Events: 50, Bytes: 500, LatencySecs: 5.0, MaxLatencySecs: 3.0},
						SinceStart: ReplicationStats{Nodes: 1, Events: 150, Bytes: 1500},
					},
				},
			},
			other: &ReplicationMetrics{
				Targets: map[string]ReplicationTargetStats{
					"target1": {
						Nodes:      3,
						LastHour:   ReplicationStats{Nodes: 3, Events: 200, Bytes: 2000, LatencySecs: 15.0, MaxLatencySecs: 6.0},
						SinceStart: ReplicationStats{Nodes: 3, Events: 300, Bytes: 3000},
					},
					"target3": {
						Nodes:      1,
						LastHour:   ReplicationStats{Nodes: 1, Events: 25, Bytes: 250, LatencySecs: 8.0, MaxLatencySecs: 7.0},
						SinceStart: ReplicationStats{Nodes: 1, Events: 75, Bytes: 750},
					},
				},
			},
			verify: func(t *testing.T, result *ReplicationMetrics) {
				if len(result.Targets) != 3 {
					t.Errorf("Targets length = %d, want 3", len(result.Targets))
				}

				// Check target1 (merged)
				target1 := result.Targets["target1"]
				if target1.Nodes != 5 { // 2 + 3
					t.Errorf("Targets[target1].Nodes = %d, want 5", target1.Nodes)
				}
				if target1.LastHour.LatencySecs != 25.0 { // 10 + 15
					t.Errorf("Targets[target1].LastHour.LatencySecs = %f, want 25.0", target1.LastHour.LatencySecs)
				}
				if target1.LastHour.MaxLatencySecs != 6.0 { // max(4, 6)
					t.Errorf("Targets[target1].LastHour.MaxLatencySecs = %f, want 6.0", target1.LastHour.MaxLatencySecs)
				}
				if target1.LastHour.Events != 300 { // 100 + 200
					t.Errorf("Targets[target1].LastHour.Events = %d, want 300", target1.LastHour.Events)
				}
				if target1.SinceStart.Events != 500 { // 200 + 300
					t.Errorf("Targets[target1].SinceStart.Events = %d, want 500", target1.SinceStart.Events)
				}

				// Check target2 (unchanged)
				target2 := result.Targets["target2"]
				if target2.Nodes != 1 {
					t.Errorf("Targets[target2].Nodes = %d, want 1", target2.Nodes)
				}
				if target2.LastHour.Events != 50 {
					t.Errorf("Targets[target2].LastHour.Events = %d, want 50", target2.LastHour.Events)
				}

				// Check target3 (new)
				target3 := result.Targets["target3"]
				if target3.Nodes != 1 {
					t.Errorf("Targets[target3].Nodes = %d, want 1", target3.Nodes)
				}
				if target3.LastHour.Events != 25 {
					t.Errorf("Targets[target3].LastHour.Events = %d, want 25", target3.LastHour.Events)
				}
			},
		},
		{
			name: "merge with empty Targets in other",
			base: &ReplicationMetrics{
				Targets: map[string]ReplicationTargetStats{
					"target1": {
						Nodes:      1,
						LastHour:   ReplicationStats{Nodes: 1, Events: 100},
						SinceStart: ReplicationStats{Nodes: 1, Events: 500},
					},
				},
			},
			other: &ReplicationMetrics{
				Nodes:  2,
				Active: 50,
			},
			verify: func(t *testing.T, result *ReplicationMetrics) {
				// Targets should remain unchanged
				if len(result.Targets) != 1 {
					t.Errorf("Targets length = %d, want 1", len(result.Targets))
				}
				if result.Targets["target1"].Nodes != 1 {
					t.Errorf("Targets[target1].Nodes = %d, want 1", result.Targets["target1"].Nodes)
				}
				if result.Nodes != 2 {
					t.Errorf("Nodes = %d, want 2", result.Nodes)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.base.Merge(tt.other)
			tt.verify(t, tt.base)
		})
	}
}

// TestReplicationTargetStatsMerge tests ReplicationTargetStats.Merge functionality
func TestReplicationTargetStatsMerge(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name   string
		base   *ReplicationTargetStats
		other  *ReplicationTargetStats
		verify func(t *testing.T, result *ReplicationTargetStats)
	}{
		{
			name:  "merge nil other",
			base:  &ReplicationTargetStats{Nodes: 2},
			other: nil,
			verify: func(t *testing.T, result *ReplicationTargetStats) {
				if result.Nodes != 2 {
					t.Errorf("Nodes = %d, want 2", result.Nodes)
				}
			},
		},
		{
			name: "merge with zero nodes other",
			base: &ReplicationTargetStats{
				Nodes:    2,
				LastHour: ReplicationStats{Events: 100},
			},
			other: &ReplicationTargetStats{
				Nodes:    0,
				LastHour: ReplicationStats{Events: 200},
			},
			verify: func(t *testing.T, result *ReplicationTargetStats) {
				if result.Nodes != 2 {
					t.Errorf("Nodes = %d, want 2", result.Nodes)
				}
				if result.LastHour.Events != 100 {
					t.Errorf("LastHour.Events = %d, want 100", result.LastHour.Events)
				}
			},
		},
		{
			name: "merge basic stats",
			base: &ReplicationTargetStats{
				Nodes: 2,
				LastHour: ReplicationStats{
					Nodes:          2,
					Events:         100,
					Bytes:          1000,
					LatencySecs:    10.0,
					MaxLatencySecs: 5.0,
				},
				SinceStart: ReplicationStats{
					Nodes:  2,
					Events: 500,
					Bytes:  5000,
				},
			},
			other: &ReplicationTargetStats{
				Nodes: 3,
				LastHour: ReplicationStats{
					Nodes:          3,
					Events:         200,
					Bytes:          2000,
					LatencySecs:    15.0,
					MaxLatencySecs: 7.0,
				},
				SinceStart: ReplicationStats{
					Nodes:  3,
					Events: 700,
					Bytes:  7000,
				},
			},
			verify: func(t *testing.T, result *ReplicationTargetStats) {
				if result.Nodes != 5 {
					t.Errorf("Nodes = %d, want 5", result.Nodes)
				}
				if result.LastHour.Events != 300 {
					t.Errorf("LastHour.Events = %d, want 300", result.LastHour.Events)
				}
				if result.LastHour.Bytes != 3000 {
					t.Errorf("LastHour.Bytes = %d, want 3000", result.LastHour.Bytes)
				}
				if result.LastHour.LatencySecs != 25.0 {
					t.Errorf("LastHour.LatencySecs = %f, want 25.0", result.LastHour.LatencySecs)
				}
				if result.LastHour.MaxLatencySecs != 7.0 {
					t.Errorf("LastHour.MaxLatencySecs = %f, want 7.0", result.LastHour.MaxLatencySecs)
				}
				if result.SinceStart.Events != 1200 {
					t.Errorf("SinceStart.Events = %d, want 1200", result.SinceStart.Events)
				}
			},
		},
		{
			name: "merge with LastDay segmented stats",
			base: &ReplicationTargetStats{
				Nodes: 1,
			},
			other: &ReplicationTargetStats{
				Nodes: 1,
				LastDay: &SegmentedReplicationStats{
					Segments: []ReplicationStats{
						{Events: 1000, Bytes: 10000},
					},
				},
			},
			verify: func(t *testing.T, result *ReplicationTargetStats) {
				if result.Nodes != 2 {
					t.Errorf("Nodes = %d, want 2", result.Nodes)
				}
				if result.LastDay == nil {
					t.Fatal("LastDay should not be nil after merge")
				}
				if len(result.LastDay.Segments) != 1 {
					t.Errorf("LastDay.Segments length = %d, want 1", len(result.LastDay.Segments))
				}
				if result.LastDay.Segments[0].Events != 1000 {
					t.Errorf("LastDay.Segments[0].Events = %d, want 1000", result.LastDay.Segments[0].Events)
				}
			},
		},
		{
			name: "merge both with LastDay",
			base: &ReplicationTargetStats{
				Nodes: 1,
				LastDay: &SegmentedReplicationStats{
					Interval:  60,
					FirstTime: now,
					Segments: []ReplicationStats{
						{Nodes: 1, Events: 500, Bytes: 5000},
					},
				},
			},
			other: &ReplicationTargetStats{
				Nodes: 1,
				LastDay: &SegmentedReplicationStats{
					Interval:  60,
					FirstTime: now,
					Segments: []ReplicationStats{
						{Nodes: 1, Events: 700, Bytes: 7000},
					},
				},
			},
			verify: func(t *testing.T, result *ReplicationTargetStats) {
				if result.Nodes != 2 {
					t.Errorf("Nodes = %d, want 2", result.Nodes)
				}
				if result.LastDay == nil {
					t.Fatal("LastDay should not be nil after merge")
				}
				// After merge with same FirstTime and Interval, segments should be merged
				totalEvents := int64(0)
				for _, seg := range result.LastDay.Segments {
					totalEvents += seg.Events
				}
				if totalEvents != 1200 {
					t.Errorf("Total events in LastDay.Segments = %d, want 1200", totalEvents)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.base.Merge(tt.other)
			tt.verify(t, tt.base)
		})
	}
}

// TestReplicationStatsAdd tests ReplicationStats.Add functionality
func TestReplicationStatsAdd(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour)

	tests := []struct {
		name   string
		base   *ReplicationStats
		other  *ReplicationStats
		verify func(t *testing.T, result *ReplicationStats)
	}{
		{
			name:  "add nil other",
			base:  &ReplicationStats{},
			other: nil,
			verify: func(_ *testing.T, _ *ReplicationStats) {
				// Should not panic
			},
		},
		{
			name: "add all fields",
			base: &ReplicationStats{
				Nodes:            2,
				StartTime:        &now,
				EndTime:          &later,
				WallTimeSecs:     100,
				Events:           1000,
				Bytes:            10000,
				EventTimeSecs:    50,
				PutObject:        500,
				UpdateMeta:       50,
				DelObject:        300,
				DelTag:           30,
				LatencySecs:      10.0,
				MaxLatencySecs:   5.0,
				PutErrors:        10,
				UpdateMetaErrors: 5,
				DelErrors:        8,
				DelTagErrors:     2,
				Synced:           800,
				AlreadyOK:        100,
				Rejected:         50,
				ProxyEvents:      20,
				ProxyBytes:       2000,
				ProxyHead:        5,
				ProxyGet:         10,
				ProxyGetTag:      5,
				ProxyGetOK:       8,
				ProxyGetTagOK:    4,
				ProxyHeadOK:      3,
			},
			other: &ReplicationStats{
				Nodes:            3,
				StartTime:        &now,
				EndTime:          &later,
				WallTimeSecs:     150,
				Events:           2000,
				Bytes:            20000,
				EventTimeSecs:    75,
				PutObject:        1000,
				UpdateMeta:       100,
				DelObject:        600,
				DelTag:           60,
				LatencySecs:      15.0,
				MaxLatencySecs:   7.0,
				PutErrors:        20,
				UpdateMetaErrors: 10,
				DelErrors:        15,
				DelTagErrors:     5,
				Synced:           1600,
				AlreadyOK:        200,
				Rejected:         100,
				ProxyEvents:      40,
				ProxyBytes:       4000,
				ProxyHead:        10,
				ProxyGet:         20,
				ProxyGetTag:      10,
				ProxyGetOK:       16,
				ProxyGetTagOK:    8,
				ProxyHeadOK:      7,
			},
			verify: func(t *testing.T, result *ReplicationStats) {
				if result.Nodes != 5 {
					t.Errorf("Nodes = %d, want 5", result.Nodes)
				}
				if result.WallTimeSecs != 250 {
					t.Errorf("WallTimeSecs = %f, want 250", result.WallTimeSecs)
				}
				if result.Events != 3000 {
					t.Errorf("Events = %d, want 3000", result.Events)
				}
				if result.Bytes != 30000 {
					t.Errorf("Bytes = %d, want 30000", result.Bytes)
				}
				if result.EventTimeSecs != 125 {
					t.Errorf("EventTimeSecs = %f, want 125", result.EventTimeSecs)
				}
				if result.LatencySecs != 25.0 {
					t.Errorf("LatencySecs = %f, want 25.0", result.LatencySecs)
				}
				if result.MaxLatencySecs != 7.0 {
					t.Errorf("MaxLatencySecs = %f, want 7.0", result.MaxLatencySecs)
				}
				if result.PutObject != 1500 {
					t.Errorf("PutObject = %d, want 1500", result.PutObject)
				}
				if result.UpdateMeta != 150 {
					t.Errorf("UpdateMeta = %d, want 150", result.UpdateMeta)
				}
				if result.DelObject != 900 {
					t.Errorf("DelObject = %d, want 900", result.DelObject)
				}
				if result.DelTag != 90 {
					t.Errorf("DelTag = %d, want 90", result.DelTag)
				}
				if result.PutErrors != 30 {
					t.Errorf("PutErrors = %d, want 30", result.PutErrors)
				}
				if result.UpdateMetaErrors != 15 {
					t.Errorf("UpdateMetaErrors = %d, want 15", result.UpdateMetaErrors)
				}
				if result.DelErrors != 23 {
					t.Errorf("DelErrors = %d, want 23", result.DelErrors)
				}
				if result.DelTagErrors != 7 {
					t.Errorf("DelTagErrors = %d, want 7", result.DelTagErrors)
				}
				if result.Synced != 2400 {
					t.Errorf("Synced = %d, want 2400", result.Synced)
				}
				if result.AlreadyOK != 300 {
					t.Errorf("AlreadyOK = %d, want 300", result.AlreadyOK)
				}
				if result.Rejected != 150 {
					t.Errorf("Rejected = %d, want 150", result.Rejected)
				}
				if result.ProxyEvents != 60 {
					t.Errorf("ProxyEvents = %d, want 60", result.ProxyEvents)
				}
				if result.ProxyBytes != 6000 {
					t.Errorf("ProxyBytes = %d, want 6000", result.ProxyBytes)
				}
				if result.ProxyHead != 15 {
					t.Errorf("ProxyHead = %d, want 15", result.ProxyHead)
				}
				if result.ProxyGet != 30 {
					t.Errorf("ProxyGet = %d, want 30", result.ProxyGet)
				}
				if result.ProxyGetTag != 15 {
					t.Errorf("ProxyGetTag = %d, want 15", result.ProxyGetTag)
				}
				if result.ProxyGetOK != 24 {
					t.Errorf("ProxyGetOK = %d, want 24", result.ProxyGetOK)
				}
				if result.ProxyGetTagOK != 12 {
					t.Errorf("ProxyGetTagOK = %d, want 12", result.ProxyGetTagOK)
				}
				if result.ProxyHeadOK != 10 {
					t.Errorf("ProxyHeadOK = %d, want 10", result.ProxyHeadOK)
				}
			},
		},
		{
			name: "different timestamps should nullify",
			base: &ReplicationStats{
				Nodes:     1,
				StartTime: &now,
				EndTime:   &now,
				Events:    100,
			},
			other: &ReplicationStats{
				Nodes:     1,
				StartTime: &later,
				EndTime:   &later,
				Events:    200,
			},
			verify: func(t *testing.T, result *ReplicationStats) {
				if result.StartTime != nil {
					t.Error("StartTime should be nil when timestamps differ")
				}
				if result.EndTime != nil {
					t.Error("EndTime should be nil when timestamps differ")
				}
				if result.Events != 300 {
					t.Errorf("Events = %d, want 300", result.Events)
				}
			},
		},
		{
			name: "skip when other has zero nodes",
			base: &ReplicationStats{
				Nodes:  1,
				Events: 100,
			},
			other: &ReplicationStats{
				Nodes:  0,
				Events: 200,
			},
			verify: func(t *testing.T, result *ReplicationStats) {
				if result.Nodes != 1 {
					t.Errorf("Nodes = %d, want 1", result.Nodes)
				}
				if result.Events != 100 {
					t.Errorf("Events = %d, want 100", result.Events)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.base.Add(tt.other)
			tt.verify(t, tt.base)
		})
	}
}

// TestMetricsMerge tests the top-level Metrics.Merge functionality
func TestMetricsMerge(t *testing.T) {
	tests := []struct {
		name   string
		base   *Metrics
		other  *Metrics
		verify func(t *testing.T, result *Metrics)
	}{
		{
			name:  "merge nil other",
			base:  &Metrics{},
			other: nil,
			verify: func(_ *testing.T, _ *Metrics) {
				// Should not panic
			},
		},
		{
			name: "merge all non-nil fields",
			base: &Metrics{
				Scanner:     &ScannerMetrics{OngoingBuckets: 5},
				Disk:        &DiskMetric{NDisks: 10},
				OS:          &OSMetrics{},
				BatchJobs:   &BatchJobMetrics{},
				SiteResync:  &SiteResyncMetrics{NumBuckets: 20},
				Net:         &NetMetrics{},
				Mem:         &MemMetrics{},
				CPU:         &CPUMetrics{CPUCount: 4},
				RPC:         &RPCMetrics{Nodes: 2, ConnectionStats: ConnectionStats{Connected: 5}},
				Go:          &RuntimeMetrics{N: 1},
				API:         &APIMetrics{Nodes: 3},
				Replication: &ReplicationMetrics{Active: 100},
			},
			other: &Metrics{
				Scanner:     &ScannerMetrics{OngoingBuckets: 3},
				Disk:        &DiskMetric{NDisks: 5},
				OS:          &OSMetrics{},
				BatchJobs:   &BatchJobMetrics{},
				SiteResync:  &SiteResyncMetrics{NumBuckets: 10},
				Net:         &NetMetrics{},
				Mem:         &MemMetrics{},
				CPU:         &CPUMetrics{CPUCount: 4},
				RPC:         &RPCMetrics{Nodes: 3, ConnectionStats: ConnectionStats{Connected: 3}},
				Go:          &RuntimeMetrics{N: 1},
				API:         &APIMetrics{Nodes: 2},
				Replication: &ReplicationMetrics{Active: 50},
			},
			verify: func(t *testing.T, result *Metrics) {
				if result.Scanner.OngoingBuckets != 5 {
					t.Errorf("Scanner.OngoingBuckets = %d, want 5", result.Scanner.OngoingBuckets)
				}
				if result.Disk.NDisks != 15 {
					t.Errorf("Disk.NDisks = %d, want 15", result.Disk.NDisks)
				}
				if result.CPU.CPUCount != 8 {
					t.Errorf("CPU.CPUCount = %d, want 8", result.CPU.CPUCount)
				}
				if result.RPC.Nodes != 5 {
					t.Errorf("RPC.Nodes = %d, want 5", result.RPC.Nodes)
				}
				if result.RPC.Connected != 8 {
					t.Errorf("RPC.Connected = %d, want 8", result.RPC.Connected)
				}
				if result.Go.N != 2 {
					t.Errorf("Go.N = %d, want 2", result.Go.N)
				}
				if result.API.Nodes != 5 {
					t.Errorf("API.Nodes = %d, want 5", result.API.Nodes)
				}
				if result.Replication.Active != 150 {
					t.Errorf("Replication.Active = %d, want 150", result.Replication.Active)
				}
			},
		},
		{
			name: "merge with nil base fields",
			base: &Metrics{},
			other: &Metrics{
				Scanner:     &ScannerMetrics{OngoingBuckets: 5},
				Disk:        &DiskMetric{NDisks: 10},
				CPU:         &CPUMetrics{CPUCount: 4},
				Replication: &ReplicationMetrics{Active: 100},
			},
			verify: func(t *testing.T, result *Metrics) {
				if result.Scanner == nil {
					t.Error("Scanner should not be nil")
				}
				if result.Scanner != nil && result.Scanner.OngoingBuckets != 5 {
					t.Errorf("Scanner.OngoingBuckets = %d, want 5", result.Scanner.OngoingBuckets)
				}
				if result.Disk == nil {
					t.Error("Disk should not be nil")
				}
				if result.Disk != nil && result.Disk.NDisks != 10 {
					t.Errorf("Disk.NDisks = %d, want 10", result.Disk.NDisks)
				}
				if result.CPU == nil {
					t.Error("CPU should not be nil")
				}
				if result.CPU != nil && result.CPU.CPUCount != 4 {
					t.Errorf("CPU.CPUCount = %d, want 4", result.CPU.CPUCount)
				}
				if result.Replication == nil {
					t.Error("Replication should not be nil")
				}
				if result.Replication != nil && result.Replication.Active != 100 {
					t.Errorf("Replication.Active = %d, want 100", result.Replication.Active)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.base.Merge(tt.other)
			tt.verify(t, tt.base)
		})
	}
}

// TestRealtimeMetricsMerge tests RealtimeMetrics.Merge functionality
func TestRealtimeMetricsMerge(t *testing.T) {
	tests := []struct {
		name   string
		base   *RealtimeMetrics
		other  *RealtimeMetrics
		verify func(t *testing.T, result *RealtimeMetrics)
	}{
		{
			name:  "merge nil other",
			base:  &RealtimeMetrics{},
			other: nil,
			verify: func(_ *testing.T, _ *RealtimeMetrics) {
				// Should not panic
			},
		},
		{
			name: "merge errors",
			base: &RealtimeMetrics{
				Errors: []string{"error1"},
			},
			other: &RealtimeMetrics{
				Errors: []string{"error2", "error3"},
			},
			verify: func(t *testing.T, result *RealtimeMetrics) {
				if len(result.Errors) != 3 {
					t.Errorf("Errors length = %d, want 3", len(result.Errors))
				}
				expected := []string{"error1", "error2", "error3"}
				if !reflect.DeepEqual(result.Errors, expected) {
					t.Errorf("Errors = %v, want %v", result.Errors, expected)
				}
			},
		},
		{
			name: "merge hosts",
			base: &RealtimeMetrics{
				Hosts: []string{"host1", "host2"},
			},
			other: &RealtimeMetrics{
				Hosts: []string{"host3", "host1"},
			},
			verify: func(t *testing.T, result *RealtimeMetrics) {
				if len(result.Hosts) != 4 {
					t.Errorf("Hosts length = %d, want 4", len(result.Hosts))
				}
				// Should be sorted
				expected := []string{"host1", "host1", "host2", "host3"}
				if !reflect.DeepEqual(result.Hosts, expected) {
					t.Errorf("Hosts = %v, want %v", result.Hosts, expected)
				}
			},
		},
		{
			name: "merge ByHost maps",
			base: &RealtimeMetrics{
				ByHost: map[string]Metrics{
					"host1": {Scanner: &ScannerMetrics{OngoingBuckets: 5}},
				},
			},
			other: &RealtimeMetrics{
				ByHost: map[string]Metrics{
					"host2": {Scanner: &ScannerMetrics{OngoingBuckets: 3}},
				},
			},
			verify: func(t *testing.T, result *RealtimeMetrics) {
				if len(result.ByHost) != 2 {
					t.Errorf("ByHost length = %d, want 2", len(result.ByHost))
				}
				if result.ByHost["host1"].Scanner.OngoingBuckets != 5 {
					t.Errorf("ByHost[host1].Scanner.OngoingBuckets = %d, want 5", result.ByHost["host1"].Scanner.OngoingBuckets)
				}
				if result.ByHost["host2"].Scanner.OngoingBuckets != 3 {
					t.Errorf("ByHost[host2].Scanner.OngoingBuckets = %d, want 3", result.ByHost["host2"].Scanner.OngoingBuckets)
				}
			},
		},
		{
			name: "merge ByDiskSet nested maps",
			base: &RealtimeMetrics{
				ByDiskSet: map[int]map[int]DiskMetric{
					0: {
						0: {NDisks: 4},
						1: {NDisks: 4},
					},
				},
			},
			other: &RealtimeMetrics{
				ByDiskSet: map[int]map[int]DiskMetric{
					0: {
						1: {NDisks: 2},
						2: {NDisks: 4},
					},
					1: {
						0: {NDisks: 4},
					},
				},
			},
			verify: func(t *testing.T, result *RealtimeMetrics) {
				if len(result.ByDiskSet) != 2 {
					t.Errorf("ByDiskSet length = %d, want 2", len(result.ByDiskSet))
				}
				if result.ByDiskSet[0][0].NDisks != 4 {
					t.Errorf("ByDiskSet[0][0].NDisks = %d, want 4", result.ByDiskSet[0][0].NDisks)
				}
				if result.ByDiskSet[0][1].NDisks != 6 { // 4 + 2
					t.Errorf("ByDiskSet[0][1].NDisks = %d, want 6", result.ByDiskSet[0][1].NDisks)
				}
				if result.ByDiskSet[0][2].NDisks != 4 {
					t.Errorf("ByDiskSet[0][2].NDisks = %d, want 4", result.ByDiskSet[0][2].NDisks)
				}
				if result.ByDiskSet[1][0].NDisks != 4 {
					t.Errorf("ByDiskSet[1][0].NDisks = %d, want 4", result.ByDiskSet[1][0].NDisks)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.base.Merge(tt.other)
			tt.verify(t, tt.base)
		})
	}
}

// TestRPCStatsMerge tests RPCStats.Merge functionality
func TestRPCStatsMerge(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour)

	tests := []struct {
		name   string
		base   *RPCStats
		other  *RPCStats
		verify func(t *testing.T, result *RPCStats)
	}{
		{
			name: "merge basic fields",
			base: &RPCStats{
				StartTime:       &now,
				EndTime:         &now,
				WallTimeSecs:    10.0,
				Requests:        100,
				RequestTimeSecs: 10.0,
				IncomingBytes:   1000,
				OutgoingBytes:   2000,
			},
			other: &RPCStats{
				StartTime:       &now,
				EndTime:         &now,
				WallTimeSecs:    15.0,
				Requests:        200,
				RequestTimeSecs: 15.0,
				IncomingBytes:   1500,
				OutgoingBytes:   2500,
			},
			verify: func(t *testing.T, result *RPCStats) {
				if result.WallTimeSecs != 25.0 {
					t.Errorf("WallTimeSecs = %f, want 25.0", result.WallTimeSecs)
				}
				if result.Requests != 300 {
					t.Errorf("Requests = %d, want 300", result.Requests)
				}
				if result.RequestTimeSecs != 25.0 {
					t.Errorf("RequestTimeSecs = %f, want 25.0", result.RequestTimeSecs)
				}
				if result.IncomingBytes != 2500 {
					t.Errorf("IncomingBytes = %d, want 2500", result.IncomingBytes)
				}
				if result.OutgoingBytes != 4500 {
					t.Errorf("OutgoingBytes = %d, want 4500", result.OutgoingBytes)
				}
			},
		},
		{
			name: "merge with same timestamps",
			base: &RPCStats{
				StartTime: &now,
				EndTime:   &later,
				Requests:  100,
			},
			other: &RPCStats{
				StartTime: &now,
				EndTime:   &later,
				Requests:  200,
			},
			verify: func(t *testing.T, result *RPCStats) {
				if result.StartTime == nil || !result.StartTime.Equal(now) {
					t.Error("StartTime should be preserved when times are equal")
				}
				if result.EndTime == nil || !result.EndTime.Equal(later) {
					t.Error("EndTime should be preserved when times are equal")
				}
				if result.Requests != 300 {
					t.Errorf("Requests = %d, want 300", result.Requests)
				}
			},
		},
		{
			name: "merge different start/end times - should nullify",
			base: &RPCStats{
				StartTime: &now,
				EndTime:   &now,
				Requests:  100,
			},
			other: &RPCStats{
				StartTime: &later,
				EndTime:   &later,
				Requests:  200,
			},
			verify: func(t *testing.T, result *RPCStats) {
				if result.StartTime != nil {
					t.Error("StartTime should be nil when merging different times")
				}
				if result.EndTime != nil {
					t.Error("EndTime should be nil when merging different times")
				}
				if result.Requests != 300 {
					t.Errorf("Requests = %d, want 300", result.Requests)
				}
			},
		},
		{
			name: "merge with nil timestamps",
			base: &RPCStats{
				Requests:        100,
				IncomingBytes:   1000,
				OutgoingBytes:   2000,
				RequestTimeSecs: 5.0,
				WallTimeSecs:    10.0,
			},
			other: &RPCStats{
				Requests:        50,
				IncomingBytes:   500,
				OutgoingBytes:   1000,
				RequestTimeSecs: 2.5,
				WallTimeSecs:    5.0,
			},
			verify: func(t *testing.T, result *RPCStats) {
				if result.Requests != 150 {
					t.Errorf("Requests = %d, want 150", result.Requests)
				}
				if result.IncomingBytes != 1500 {
					t.Errorf("IncomingBytes = %d, want 1500", result.IncomingBytes)
				}
				if result.OutgoingBytes != 3000 {
					t.Errorf("OutgoingBytes = %d, want 3000", result.OutgoingBytes)
				}
				if result.RequestTimeSecs != 7.5 {
					t.Errorf("RequestTimeSecs = %f, want 7.5", result.RequestTimeSecs)
				}
				if result.WallTimeSecs != 15.0 {
					t.Errorf("WallTimeSecs = %f, want 15.0", result.WallTimeSecs)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.base.Merge(*tt.other)
			tt.verify(t, tt.base)
		})
	}
}

// TestConnectionStatsMerge tests ConnectionStats.Merge functionality
func TestConnectionStatsMerge(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour)

	tests := []struct {
		name   string
		base   *ConnectionStats
		other  *ConnectionStats
		verify func(t *testing.T, result *ConnectionStats)
	}{
		{
			name:  "merge nil other",
			base:  &ConnectionStats{Connected: 5},
			other: nil,
			verify: func(t *testing.T, result *ConnectionStats) {
				if result.Connected != 5 {
					t.Errorf("Connected = %d, want 5", result.Connected)
				}
			},
		},
		{
			name: "merge all fields",
			base: &ConnectionStats{
				Connected:        5,
				Disconnected:     2,
				ReconnectCount:   3,
				OutgoingStreams:  10,
				IncomingStreams:  8,
				OutgoingMessages: 1000,
				IncomingMessages: 800,
				OutgoingBytes:    50000,
				IncomingBytes:    40000,
				OutQueue:         15,
				LastPongTime:     now,
				LastConnectTime:  now,
				LastPingMS:       5.5,
				MaxPingDurMS:     10.0,
			},
			other: &ConnectionStats{
				Connected:        3,
				Disconnected:     1,
				ReconnectCount:   2,
				OutgoingStreams:  5,
				IncomingStreams:  4,
				OutgoingMessages: 500,
				IncomingMessages: 400,
				OutgoingBytes:    25000,
				IncomingBytes:    20000,
				OutQueue:         10,
				LastPongTime:     later,
				LastConnectTime:  later,
				LastPingMS:       8.0,
				MaxPingDurMS:     7.0,
			},
			verify: func(t *testing.T, result *ConnectionStats) {
				if result.Connected != 8 {
					t.Errorf("Connected = %d, want 8", result.Connected)
				}
				if result.Disconnected != 3 {
					t.Errorf("Disconnected = %d, want 3", result.Disconnected)
				}
				if result.ReconnectCount != 5 {
					t.Errorf("ReconnectCount = %d, want 5", result.ReconnectCount)
				}
				if result.OutgoingStreams != 15 {
					t.Errorf("OutgoingStreams = %d, want 15", result.OutgoingStreams)
				}
				if result.IncomingStreams != 12 {
					t.Errorf("IncomingStreams = %d, want 12", result.IncomingStreams)
				}
				if result.OutgoingMessages != 1500 {
					t.Errorf("OutgoingMessages = %d, want 1500", result.OutgoingMessages)
				}
				if result.IncomingMessages != 1200 {
					t.Errorf("IncomingMessages = %d, want 1200", result.IncomingMessages)
				}
				if result.OutgoingBytes != 75000 {
					t.Errorf("OutgoingBytes = %d, want 75000", result.OutgoingBytes)
				}
				if result.IncomingBytes != 60000 {
					t.Errorf("IncomingBytes = %d, want 60000", result.IncomingBytes)
				}
				if result.OutQueue != 25 {
					t.Errorf("OutQueue = %d, want 25", result.OutQueue)
				}
				// Latest timestamps should win
				if !result.LastPongTime.Equal(later) {
					t.Errorf("LastPongTime = %v, want %v", result.LastPongTime, later)
				}
				if !result.LastConnectTime.Equal(later) {
					t.Errorf("LastConnectTime = %v, want %v", result.LastConnectTime, later)
				}
				// LastPingMS should come from the source with the later LastPongTime
				if result.LastPingMS != 8.0 {
					t.Errorf("LastPingMS = %f, want 8.0", result.LastPingMS)
				}
				// MaxPingDurMS should be the maximum
				if result.MaxPingDurMS != 10.0 {
					t.Errorf("MaxPingDurMS = %f, want 10.0", result.MaxPingDurMS)
				}
			},
		},
		{
			name: "merge with older times - keep existing",
			base: &ConnectionStats{
				LastPongTime:    later,
				LastConnectTime: later,
				LastPingMS:      5.0,
				MaxPingDurMS:    8.0,
			},
			other: &ConnectionStats{
				LastPongTime:    now,
				LastConnectTime: now,
				LastPingMS:      7.0,
				MaxPingDurMS:    6.0,
			},
			verify: func(t *testing.T, result *ConnectionStats) {
				if !result.LastPongTime.Equal(later) {
					t.Errorf("LastPongTime = %v, want %v", result.LastPongTime, later)
				}
				if !result.LastConnectTime.Equal(later) {
					t.Errorf("LastConnectTime = %v, want %v", result.LastConnectTime, later)
				}
				if result.LastPingMS != 5.0 {
					t.Errorf("LastPingMS = %f, want 5.0", result.LastPingMS)
				}
				if result.MaxPingDurMS != 8.0 {
					t.Errorf("MaxPingDurMS = %f, want 8.0", result.MaxPingDurMS)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.base.Merge(tt.other)
			tt.verify(t, tt.base)
		})
	}
}

// TestRPCMetricsMerge tests RPCMetrics.Merge functionality with embedded ConnectionStats
func TestRPCMetricsMerge(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour)

	tests := []struct {
		name   string
		base   *RPCMetrics
		other  *RPCMetrics
		verify func(t *testing.T, result *RPCMetrics)
	}{
		{
			name:  "merge nil other",
			base:  &RPCMetrics{Nodes: 3},
			other: nil,
			verify: func(t *testing.T, result *RPCMetrics) {
				if result.Nodes != 3 {
					t.Errorf("Nodes = %d, want 3", result.Nodes)
				}
			},
		},
		{
			name: "merge basic fields and embedded ConnectionStats",
			base: &RPCMetrics{
				Nodes:       2,
				CollectedAt: now,
				ConnectionStats: ConnectionStats{
					Connected:        5,
					Disconnected:     1,
					ReconnectCount:   2,
					OutgoingStreams:  10,
					IncomingStreams:  8,
					OutgoingMessages: 1000,
					IncomingMessages: 800,
					OutgoingBytes:    50000,
					IncomingBytes:    40000,
					OutQueue:         15,
					LastPongTime:     now,
					LastConnectTime:  now,
					LastPingMS:       5.5,
					MaxPingDurMS:     10.0,
				},
			},
			other: &RPCMetrics{
				Nodes:       3,
				CollectedAt: later,
				ConnectionStats: ConnectionStats{
					Connected:        3,
					Disconnected:     2,
					ReconnectCount:   1,
					OutgoingStreams:  5,
					IncomingStreams:  4,
					OutgoingMessages: 500,
					IncomingMessages: 400,
					OutgoingBytes:    25000,
					IncomingBytes:    20000,
					OutQueue:         10,
					LastPongTime:     later,
					LastConnectTime:  later,
					LastPingMS:       8.0,
					MaxPingDurMS:     7.0,
				},
			},
			verify: func(t *testing.T, result *RPCMetrics) {
				// Check Nodes
				if result.Nodes != 5 {
					t.Errorf("Nodes = %d, want 5", result.Nodes)
				}
				// Check CollectedAt
				if !result.CollectedAt.Equal(later) {
					t.Errorf("CollectedAt = %v, want %v", result.CollectedAt, later)
				}
				// Check embedded ConnectionStats fields
				if result.Connected != 8 {
					t.Errorf("Connected = %d, want 8", result.Connected)
				}
				if result.Disconnected != 3 {
					t.Errorf("Disconnected = %d, want 3", result.Disconnected)
				}
				if result.ReconnectCount != 3 {
					t.Errorf("ReconnectCount = %d, want 3", result.ReconnectCount)
				}
				if result.OutgoingStreams != 15 {
					t.Errorf("OutgoingStreams = %d, want 15", result.OutgoingStreams)
				}
				if result.IncomingStreams != 12 {
					t.Errorf("IncomingStreams = %d, want 12", result.IncomingStreams)
				}
				if result.OutgoingMessages != 1500 {
					t.Errorf("OutgoingMessages = %d, want 1500", result.OutgoingMessages)
				}
				if result.IncomingMessages != 1200 {
					t.Errorf("IncomingMessages = %d, want 1200", result.IncomingMessages)
				}
				if result.OutgoingBytes != 75000 {
					t.Errorf("OutgoingBytes = %d, want 75000", result.OutgoingBytes)
				}
				if result.IncomingBytes != 60000 {
					t.Errorf("IncomingBytes = %d, want 60000", result.IncomingBytes)
				}
				if result.OutQueue != 25 {
					t.Errorf("OutQueue = %d, want 25", result.OutQueue)
				}
				if !result.LastPongTime.Equal(later) {
					t.Errorf("LastPongTime = %v, want %v", result.LastPongTime, later)
				}
				if !result.LastConnectTime.Equal(later) {
					t.Errorf("LastConnectTime = %v, want %v", result.LastConnectTime, later)
				}
				if result.LastPingMS != 8.0 {
					t.Errorf("LastPingMS = %f, want 8.0", result.LastPingMS)
				}
				if result.MaxPingDurMS != 10.0 {
					t.Errorf("MaxPingDurMS = %f, want 10.0", result.MaxPingDurMS)
				}
			},
		},
		{
			name: "merge LastMinute map",
			base: &RPCMetrics{
				Nodes: 1,
				LastMinute: map[string]RPCStats{
					"handler1": {Requests: 100, IncomingBytes: 1000},
					"handler2": {Requests: 50, IncomingBytes: 500},
				},
			},
			other: &RPCMetrics{
				Nodes: 1,
				LastMinute: map[string]RPCStats{
					"handler1": {Requests: 200, IncomingBytes: 2000},
					"handler3": {Requests: 75, IncomingBytes: 750},
				},
			},
			verify: func(t *testing.T, result *RPCMetrics) {
				if len(result.LastMinute) != 3 {
					t.Errorf("LastMinute length = %d, want 3", len(result.LastMinute))
				}
				// handler1 should be merged
				if result.LastMinute["handler1"].Requests != 300 {
					t.Errorf("LastMinute[handler1].Requests = %d, want 300", result.LastMinute["handler1"].Requests)
				}
				if result.LastMinute["handler1"].IncomingBytes != 3000 {
					t.Errorf("LastMinute[handler1].IncomingBytes = %d, want 3000", result.LastMinute["handler1"].IncomingBytes)
				}
				// handler2 should remain unchanged
				if result.LastMinute["handler2"].Requests != 50 {
					t.Errorf("LastMinute[handler2].Requests = %d, want 50", result.LastMinute["handler2"].Requests)
				}
				// handler3 should be added
				if result.LastMinute["handler3"].Requests != 75 {
					t.Errorf("LastMinute[handler3].Requests = %d, want 75", result.LastMinute["handler3"].Requests)
				}
			},
		},
		{
			name: "merge LastDay map with SegmentedRPCMetrics",
			base: &RPCMetrics{
				Nodes: 1,
				LastDay: map[string]SegmentedRPCMetrics{
					"handler1": {
						Interval:  60,
						FirstTime: now,
						Segments:  []RPCStats{{Requests: 100}},
					},
				},
			},
			other: &RPCMetrics{
				Nodes: 1,
				LastDay: map[string]SegmentedRPCMetrics{
					"handler1": {
						Interval:  60,
						FirstTime: now,
						Segments:  []RPCStats{{Requests: 200}},
					},
					"handler2": {
						Interval:  60,
						FirstTime: now,
						Segments:  []RPCStats{{Requests: 150}},
					},
				},
			},
			verify: func(t *testing.T, result *RPCMetrics) {
				if len(result.LastDay) != 2 {
					t.Errorf("LastDay length = %d, want 2", len(result.LastDay))
				}
				// handler1 should be merged (same interval and start time, so segments are merged in place)
				if len(result.LastDay["handler1"].Segments) != 1 {
					t.Errorf("LastDay[handler1].Segments length = %d, want 1", len(result.LastDay["handler1"].Segments))
				}
				if result.LastDay["handler1"].Segments[0].Requests != 300 {
					t.Errorf("LastDay[handler1].Segments[0].Requests = %d, want 300", result.LastDay["handler1"].Segments[0].Requests)
				}
				// handler2 should be added
				if len(result.LastDay["handler2"].Segments) != 1 {
					t.Errorf("LastDay[handler2].Segments length = %d, want 1", len(result.LastDay["handler2"].Segments))
				}
				if result.LastDay["handler2"].Segments[0].Requests != 150 {
					t.Errorf("LastDay[handler2].Segments[0].Requests = %d, want 150", result.LastDay["handler2"].Segments[0].Requests)
				}
			},
		},
		{
			name: "merge ByDestination map with ConnectionStats",
			base: &RPCMetrics{
				Nodes: 1,
				ByDestination: map[string]ConnectionStats{
					"server1": {
						Connected:        3,
						OutgoingMessages: 500,
						IncomingMessages: 400,
						MaxPingDurMS:     5.0,
					},
					"server2": {
						Connected:        2,
						OutgoingMessages: 300,
						IncomingMessages: 250,
						MaxPingDurMS:     3.0,
					},
				},
			},
			other: &RPCMetrics{
				Nodes: 1,
				ByDestination: map[string]ConnectionStats{
					"server1": {
						Connected:        2,
						OutgoingMessages: 300,
						IncomingMessages: 250,
						MaxPingDurMS:     7.0,
					},
					"server3": {
						Connected:        1,
						OutgoingMessages: 100,
						IncomingMessages: 80,
						MaxPingDurMS:     2.0,
					},
				},
			},
			verify: func(t *testing.T, result *RPCMetrics) {
				if len(result.ByDestination) != 3 {
					t.Errorf("ByDestination length = %d, want 3", len(result.ByDestination))
				}
				// server1 should be merged
				if result.ByDestination["server1"].Connected != 5 {
					t.Errorf("ByDestination[server1].Connected = %d, want 5", result.ByDestination["server1"].Connected)
				}
				if result.ByDestination["server1"].OutgoingMessages != 800 {
					t.Errorf("ByDestination[server1].OutgoingMessages = %d, want 800", result.ByDestination["server1"].OutgoingMessages)
				}
				if result.ByDestination["server1"].MaxPingDurMS != 7.0 {
					t.Errorf("ByDestination[server1].MaxPingDurMS = %f, want 7.0", result.ByDestination["server1"].MaxPingDurMS)
				}
				// server2 should remain unchanged
				if result.ByDestination["server2"].Connected != 2 {
					t.Errorf("ByDestination[server2].Connected = %d, want 2", result.ByDestination["server2"].Connected)
				}
				// server3 should be added
				if result.ByDestination["server3"].Connected != 1 {
					t.Errorf("ByDestination[server3].Connected = %d, want 1", result.ByDestination["server3"].Connected)
				}
			},
		},
		{
			name: "merge ByCaller map with ConnectionStats",
			base: &RPCMetrics{
				Nodes: 1,
				ByCaller: map[string]ConnectionStats{
					"client1": {
						Connected:        2,
						IncomingMessages: 300,
						OutgoingMessages: 250,
						LastPingMS:       4.0,
					},
				},
			},
			other: &RPCMetrics{
				Nodes: 1,
				ByCaller: map[string]ConnectionStats{
					"client1": {
						Connected:        3,
						IncomingMessages: 400,
						OutgoingMessages: 350,
						LastPingMS:       6.0,
					},
					"client2": {
						Connected:        1,
						IncomingMessages: 100,
						OutgoingMessages: 80,
						LastPingMS:       2.0,
					},
				},
			},
			verify: func(t *testing.T, result *RPCMetrics) {
				if len(result.ByCaller) != 2 {
					t.Errorf("ByCaller length = %d, want 2", len(result.ByCaller))
				}
				// client1 should be merged
				if result.ByCaller["client1"].Connected != 5 {
					t.Errorf("ByCaller[client1].Connected = %d, want 5", result.ByCaller["client1"].Connected)
				}
				if result.ByCaller["client1"].IncomingMessages != 700 {
					t.Errorf("ByCaller[client1].IncomingMessages = %d, want 700", result.ByCaller["client1"].IncomingMessages)
				}
				// client2 should be added
				if result.ByCaller["client2"].Connected != 1 {
					t.Errorf("ByCaller[client2].Connected = %d, want 1", result.ByCaller["client2"].Connected)
				}
			},
		},
		{
			name: "merge all fields together",
			base: &RPCMetrics{
				Nodes:       2,
				CollectedAt: now,
				ConnectionStats: ConnectionStats{
					Connected:        5,
					OutgoingMessages: 1000,
					MaxPingDurMS:     10.0,
				},
				LastMinute: map[string]RPCStats{
					"handler1": {Requests: 100},
				},
				LastDay: map[string]SegmentedRPCMetrics{
					"handler1": {Segments: []RPCStats{{Requests: 1000}}},
				},
				ByDestination: map[string]ConnectionStats{
					"server1": {Connected: 3},
				},
				ByCaller: map[string]ConnectionStats{
					"client1": {Connected: 2},
				},
			},
			other: &RPCMetrics{
				Nodes:       3,
				CollectedAt: later,
				ConnectionStats: ConnectionStats{
					Connected:        3,
					OutgoingMessages: 500,
					MaxPingDurMS:     7.0,
				},
				LastMinute: map[string]RPCStats{
					"handler2": {Requests: 50},
				},
				LastDay: map[string]SegmentedRPCMetrics{
					"handler2": {Segments: []RPCStats{{Requests: 500}}},
				},
				ByDestination: map[string]ConnectionStats{
					"server2": {Connected: 1},
				},
				ByCaller: map[string]ConnectionStats{
					"client2": {Connected: 1},
				},
			},
			verify: func(t *testing.T, result *RPCMetrics) {
				// Check basic fields
				if result.Nodes != 5 {
					t.Errorf("Nodes = %d, want 5", result.Nodes)
				}
				if !result.CollectedAt.Equal(later) {
					t.Errorf("CollectedAt = %v, want %v", result.CollectedAt, later)
				}
				// Check ConnectionStats
				if result.Connected != 8 {
					t.Errorf("Connected = %d, want 8", result.Connected)
				}
				if result.OutgoingMessages != 1500 {
					t.Errorf("OutgoingMessages = %d, want 1500", result.OutgoingMessages)
				}
				if result.MaxPingDurMS != 10.0 {
					t.Errorf("MaxPingDurMS = %f, want 10.0", result.MaxPingDurMS)
				}
				// Check maps
				if len(result.LastMinute) != 2 {
					t.Errorf("LastMinute length = %d, want 2", len(result.LastMinute))
				}
				if len(result.LastDay) != 2 {
					t.Errorf("LastDay length = %d, want 2", len(result.LastDay))
				}
				if len(result.ByDestination) != 2 {
					t.Errorf("ByDestination length = %d, want 2", len(result.ByDestination))
				}
				if len(result.ByCaller) != 2 {
					t.Errorf("ByCaller length = %d, want 2", len(result.ByCaller))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.base.Merge(tt.other)
			tt.verify(t, tt.base)
		})
	}
}

// TestRPCMetricsLastMinuteTotal tests RPCMetrics.LastMinuteTotal functionality
func TestRPCMetricsLastMinuteTotal(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name   string
		input  *RPCMetrics
		verify func(t *testing.T, result RPCStats)
	}{
		{
			name: "empty LastMinute map",
			input: &RPCMetrics{
				LastMinute: map[string]RPCStats{},
			},
			verify: func(t *testing.T, result RPCStats) {
				if result.Requests != 0 {
					t.Errorf("Requests = %d, want 0", result.Requests)
				}
				if result.IncomingBytes != 0 {
					t.Errorf("IncomingBytes = %d, want 0", result.IncomingBytes)
				}
				if result.OutgoingBytes != 0 {
					t.Errorf("OutgoingBytes = %d, want 0", result.OutgoingBytes)
				}
			},
		},
		{
			name: "nil LastMinute map",
			input: &RPCMetrics{
				LastMinute: nil,
			},
			verify: func(t *testing.T, result RPCStats) {
				if result.Requests != 0 {
					t.Errorf("Requests = %d, want 0", result.Requests)
				}
				if result.IncomingBytes != 0 {
					t.Errorf("IncomingBytes = %d, want 0", result.IncomingBytes)
				}
				if result.OutgoingBytes != 0 {
					t.Errorf("OutgoingBytes = %d, want 0", result.OutgoingBytes)
				}
			},
		},
		{
			name: "single handler stats",
			input: &RPCMetrics{
				LastMinute: map[string]RPCStats{
					"handler1": {
						StartTime:       &now,
						EndTime:         &now,
						WallTimeSecs:    10.0,
						Requests:        100,
						RequestTimeSecs: 5.0,
						IncomingBytes:   1000,
						OutgoingBytes:   2000,
					},
				},
			},
			verify: func(t *testing.T, result RPCStats) {
				if result.Requests != 100 {
					t.Errorf("Requests = %d, want 100", result.Requests)
				}
				if result.RequestTimeSecs != 5.0 {
					t.Errorf("RequestTimeSecs = %f, want 5.0", result.RequestTimeSecs)
				}
				if result.IncomingBytes != 1000 {
					t.Errorf("IncomingBytes = %d, want 1000", result.IncomingBytes)
				}
				if result.OutgoingBytes != 2000 {
					t.Errorf("OutgoingBytes = %d, want 2000", result.OutgoingBytes)
				}
				if result.WallTimeSecs != 10.0 {
					t.Errorf("WallTimeSecs = %f, want 10.0", result.WallTimeSecs)
				}
			},
		},
		{
			name: "multiple handler stats",
			input: &RPCMetrics{
				LastMinute: map[string]RPCStats{
					"handler1": {
						StartTime:       &now,
						EndTime:         &now,
						WallTimeSecs:    10.0,
						Requests:        100,
						RequestTimeSecs: 5.0,
						IncomingBytes:   1000,
						OutgoingBytes:   2000,
					},
					"handler2": {
						StartTime:       &now,
						EndTime:         &now,
						WallTimeSecs:    15.0,
						Requests:        200,
						RequestTimeSecs: 10.0,
						IncomingBytes:   1500,
						OutgoingBytes:   2500,
					},
					"handler3": {
						StartTime:       &now,
						EndTime:         &now,
						WallTimeSecs:    5.0,
						Requests:        50,
						RequestTimeSecs: 2.5,
						IncomingBytes:   500,
						OutgoingBytes:   1000,
					},
				},
			},
			verify: func(t *testing.T, result RPCStats) {
				if result.Requests != 350 { // 100 + 200 + 50
					t.Errorf("Requests = %d, want 350", result.Requests)
				}
				if result.RequestTimeSecs != 17.5 { // 5.0 + 10.0 + 2.5
					t.Errorf("RequestTimeSecs = %f, want 17.5", result.RequestTimeSecs)
				}
				if result.IncomingBytes != 3000 { // 1000 + 1500 + 500
					t.Errorf("IncomingBytes = %d, want 3000", result.IncomingBytes)
				}
				if result.OutgoingBytes != 5500 { // 2000 + 2500 + 1000
					t.Errorf("OutgoingBytes = %d, want 5500", result.OutgoingBytes)
				}
				if result.WallTimeSecs != 30.0 { // 10.0 + 15.0 + 5.0
					t.Errorf("WallTimeSecs = %f, want 30.0", result.WallTimeSecs)
				}
			},
		},
		{
			name: "different timestamps should nullify in merge",
			input: &RPCMetrics{
				LastMinute: map[string]RPCStats{
					"handler1": {
						StartTime:     &now,
						EndTime:       &now,
						Requests:      100,
						IncomingBytes: 1000,
						OutgoingBytes: 2000,
					},
					"handler2": {
						StartTime:     nil,
						EndTime:       nil,
						Requests:      200,
						IncomingBytes: 1500,
						OutgoingBytes: 2500,
					},
				},
			},
			verify: func(t *testing.T, result RPCStats) {
				// When merging different timestamps, they should be nullified
				if result.StartTime != nil {
					t.Error("StartTime should be nil when merging different timestamps")
				}
				if result.EndTime != nil {
					t.Error("EndTime should be nil when merging different timestamps")
				}
				if result.Requests != 300 {
					t.Errorf("Requests = %d, want 300", result.Requests)
				}
				if result.IncomingBytes != 2500 {
					t.Errorf("IncomingBytes = %d, want 2500", result.IncomingBytes)
				}
				if result.OutgoingBytes != 4500 {
					t.Errorf("OutgoingBytes = %d, want 4500", result.OutgoingBytes)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.LastMinuteTotal()
			tt.verify(t, result)
		})
	}
}

// TestRPCMetricsLastDayTotalSegmented tests RPCMetrics.LastDayTotalSegmented functionality
func TestRPCMetricsLastDayTotalSegmented(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name   string
		input  *RPCMetrics
		verify func(t *testing.T, result SegmentedRPCMetrics)
	}{
		{
			name: "empty LastDay map",
			input: &RPCMetrics{
				LastDay: map[string]SegmentedRPCMetrics{},
			},
			verify: func(t *testing.T, result SegmentedRPCMetrics) {
				if len(result.Segments) != 0 {
					t.Errorf("Segments length = %d, want 0", len(result.Segments))
				}
			},
		},
		{
			name: "nil LastDay map",
			input: &RPCMetrics{
				LastDay: nil,
			},
			verify: func(t *testing.T, result SegmentedRPCMetrics) {
				if len(result.Segments) != 0 {
					t.Errorf("Segments length = %d, want 0", len(result.Segments))
				}
			},
		},
		{
			name: "single handler with segments",
			input: &RPCMetrics{
				LastDay: map[string]SegmentedRPCMetrics{
					"handler1": {
						Interval:  60,
						FirstTime: now,
						Segments: []RPCStats{
							{Requests: 100, IncomingBytes: 1000, OutgoingBytes: 2000},
							{Requests: 200, IncomingBytes: 1500, OutgoingBytes: 2500},
							{Requests: 150, IncomingBytes: 1200, OutgoingBytes: 2200},
						},
					},
				},
			},
			verify: func(t *testing.T, result SegmentedRPCMetrics) {
				if result.Interval != 60 {
					t.Errorf("Interval = %d, want 60", result.Interval)
				}
				if !result.FirstTime.Equal(now) {
					t.Errorf("FirstTime = %v, want %v", result.FirstTime, now)
				}
				if len(result.Segments) != 3 {
					t.Errorf("Segments length = %d, want 3", len(result.Segments))
				}
				// Check individual segments
				if result.Segments[0].Requests != 100 {
					t.Errorf("Segments[0].Requests = %d, want 100", result.Segments[0].Requests)
				}
				if result.Segments[1].Requests != 200 {
					t.Errorf("Segments[1].Requests = %d, want 200", result.Segments[1].Requests)
				}
				if result.Segments[2].Requests != 150 {
					t.Errorf("Segments[2].Requests = %d, want 150", result.Segments[2].Requests)
				}
			},
		},
		{
			name: "multiple handlers with same interval and time - should merge",
			input: &RPCMetrics{
				LastDay: map[string]SegmentedRPCMetrics{
					"handler1": {
						Interval:  60,
						FirstTime: now,
						Segments: []RPCStats{
							{Requests: 100, IncomingBytes: 1000},
							{Requests: 200, IncomingBytes: 1500},
						},
					},
					"handler2": {
						Interval:  60,
						FirstTime: now,
						Segments: []RPCStats{
							{Requests: 50, IncomingBytes: 500},
							{Requests: 75, IncomingBytes: 750},
						},
					},
				},
			},
			verify: func(t *testing.T, result SegmentedRPCMetrics) {
				if result.Interval != 60 {
					t.Errorf("Interval = %d, want 60", result.Interval)
				}
				if !result.FirstTime.Equal(now) {
					t.Errorf("FirstTime = %v, want %v", result.FirstTime, now)
				}
				if len(result.Segments) != 2 {
					t.Errorf("Segments length = %d, want 2", len(result.Segments))
				}
				// Check merged segments
				if result.Segments[0].Requests != 150 { // 100 + 50
					t.Errorf("Segments[0].Requests = %d, want 150", result.Segments[0].Requests)
				}
				if result.Segments[0].IncomingBytes != 1500 { // 1000 + 500
					t.Errorf("Segments[0].IncomingBytes = %d, want 1500", result.Segments[0].IncomingBytes)
				}
				if result.Segments[1].Requests != 275 { // 200 + 75
					t.Errorf("Segments[1].Requests = %d, want 275", result.Segments[1].Requests)
				}
				if result.Segments[1].IncomingBytes != 2250 { // 1500 + 750
					t.Errorf("Segments[1].IncomingBytes = %d, want 2250", result.Segments[1].IncomingBytes)
				}
			},
		},
		{
			name: "multiple handlers with different intervals - should not merge",
			input: &RPCMetrics{
				LastDay: map[string]SegmentedRPCMetrics{
					"handler1": {
						Interval:  60,
						FirstTime: now,
						Segments: []RPCStats{
							{Requests: 100},
							{Requests: 200},
						},
					},
					"handler2": {
						Interval:  120, // Different interval
						FirstTime: now,
						Segments: []RPCStats{
							{Requests: 50},
						},
					},
				},
			},
			verify: func(t *testing.T, result SegmentedRPCMetrics) {
				// When intervals differ, the second one is silently ignored
				if result.Interval != 60 {
					t.Errorf("Interval = %d, want 60", result.Interval)
				}
				if len(result.Segments) != 2 { // Only handler1's segments
					t.Errorf("Segments length = %d, want 2", len(result.Segments))
				}
				// Only handler1's data should be present
				if result.Segments[0].Requests != 100 {
					t.Errorf("Segments[0].Requests = %d, want 100", result.Segments[0].Requests)
				}
				if result.Segments[1].Requests != 200 {
					t.Errorf("Segments[1].Requests = %d, want 200", result.Segments[1].Requests)
				}
			},
		},
		{
			name: "multiple handlers with different first times - should create timeline",
			input: &RPCMetrics{
				LastDay: map[string]SegmentedRPCMetrics{
					"handler1": {
						Interval:  60,
						FirstTime: now,
						Segments: []RPCStats{
							{Requests: 100},
						},
					},
					"handler2": {
						Interval:  60,
						FirstTime: now.Add(time.Hour), // 1 hour later = 60 segments later
						Segments: []RPCStats{
							{Requests: 50},
						},
					},
				},
			},
			verify: func(t *testing.T, result SegmentedRPCMetrics) {
				if result.Interval != 60 {
					t.Errorf("Interval = %d, want 60", result.Interval)
				}
				// Should create a timeline from earliest to latest (61 segments total)
				if len(result.Segments) != 61 { // 60 minute gap + 1 segment on each end
					t.Errorf("Segments length = %d, want 61", len(result.Segments))
				}
				// First segment should have handler1's data
				if result.Segments[0].Requests != 100 {
					t.Errorf("Segments[0].Requests = %d, want 100", result.Segments[0].Requests)
				}
				// Last segment should have handler2's data
				if result.Segments[60].Requests != 50 {
					t.Errorf("Segments[60].Requests = %d, want 50", result.Segments[60].Requests)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.LastDayTotalSegmented()
			tt.verify(t, result)
		})
	}
}

// TestRPCMetricsLastDayTotal tests RPCMetrics.LastDayTotal functionality
func TestRPCMetricsLastDayTotal(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name   string
		input  *RPCMetrics
		verify func(t *testing.T, result RPCStats)
	}{
		{
			name: "empty LastDay map",
			input: &RPCMetrics{
				LastDay: map[string]SegmentedRPCMetrics{},
			},
			verify: func(t *testing.T, result RPCStats) {
				if result.Requests != 0 {
					t.Errorf("Requests = %d, want 0", result.Requests)
				}
				if result.IncomingBytes != 0 {
					t.Errorf("IncomingBytes = %d, want 0", result.IncomingBytes)
				}
				if result.OutgoingBytes != 0 {
					t.Errorf("OutgoingBytes = %d, want 0", result.OutgoingBytes)
				}
			},
		},
		{
			name: "nil LastDay map",
			input: &RPCMetrics{
				LastDay: nil,
			},
			verify: func(t *testing.T, result RPCStats) {
				if result.Requests != 0 {
					t.Errorf("Requests = %d, want 0", result.Requests)
				}
				if result.IncomingBytes != 0 {
					t.Errorf("IncomingBytes = %d, want 0", result.IncomingBytes)
				}
				if result.OutgoingBytes != 0 {
					t.Errorf("OutgoingBytes = %d, want 0", result.OutgoingBytes)
				}
			},
		},
		{
			name: "single handler with single segment",
			input: &RPCMetrics{
				LastDay: map[string]SegmentedRPCMetrics{
					"handler1": {
						Interval:  60,
						FirstTime: now,
						Segments: []RPCStats{
							{
								StartTime:       &now,
								EndTime:         &now,
								WallTimeSecs:    10.0,
								Requests:        100,
								RequestTimeSecs: 5.0,
								IncomingBytes:   1000,
								OutgoingBytes:   2000,
							},
						},
					},
				},
			},
			verify: func(t *testing.T, result RPCStats) {
				if result.Requests != 100 {
					t.Errorf("Requests = %d, want 100", result.Requests)
				}
				if result.RequestTimeSecs != 5.0 {
					t.Errorf("RequestTimeSecs = %f, want 5.0", result.RequestTimeSecs)
				}
				if result.IncomingBytes != 1000 {
					t.Errorf("IncomingBytes = %d, want 1000", result.IncomingBytes)
				}
				if result.OutgoingBytes != 2000 {
					t.Errorf("OutgoingBytes = %d, want 2000", result.OutgoingBytes)
				}
				if result.WallTimeSecs != 10.0 {
					t.Errorf("WallTimeSecs = %f, want 10.0", result.WallTimeSecs)
				}
			},
		},
		{
			name: "single handler with multiple segments",
			input: &RPCMetrics{
				LastDay: map[string]SegmentedRPCMetrics{
					"handler1": {
						Interval:  60,
						FirstTime: now,
						Segments: []RPCStats{
							{
								StartTime:       &now,
								EndTime:         &now,
								WallTimeSecs:    10.0,
								Requests:        100,
								RequestTimeSecs: 5.0,
								IncomingBytes:   1000,
								OutgoingBytes:   2000,
							},
							{
								StartTime:       &now,
								EndTime:         &now,
								WallTimeSecs:    15.0,
								Requests:        200,
								RequestTimeSecs: 10.0,
								IncomingBytes:   1500,
								OutgoingBytes:   2500,
							},
							{
								StartTime:       &now,
								EndTime:         &now,
								WallTimeSecs:    5.0,
								Requests:        50,
								RequestTimeSecs: 2.5,
								IncomingBytes:   500,
								OutgoingBytes:   1000,
							},
						},
					},
				},
			},
			verify: func(t *testing.T, result RPCStats) {
				if result.Requests != 350 { // 100 + 200 + 50
					t.Errorf("Requests = %d, want 350", result.Requests)
				}
				if result.RequestTimeSecs != 17.5 { // 5.0 + 10.0 + 2.5
					t.Errorf("RequestTimeSecs = %f, want 17.5", result.RequestTimeSecs)
				}
				if result.IncomingBytes != 3000 { // 1000 + 1500 + 500
					t.Errorf("IncomingBytes = %d, want 3000", result.IncomingBytes)
				}
				if result.OutgoingBytes != 5500 { // 2000 + 2500 + 1000
					t.Errorf("OutgoingBytes = %d, want 5500", result.OutgoingBytes)
				}
				if result.WallTimeSecs != 30.0 { // 10.0 + 15.0 + 5.0
					t.Errorf("WallTimeSecs = %f, want 30.0", result.WallTimeSecs)
				}
			},
		},
		{
			name: "multiple handlers with segments",
			input: &RPCMetrics{
				LastDay: map[string]SegmentedRPCMetrics{
					"handler1": {
						Interval:  60,
						FirstTime: now,
						Segments: []RPCStats{
							{Requests: 100, IncomingBytes: 1000, OutgoingBytes: 2000},
							{Requests: 200, IncomingBytes: 1500, OutgoingBytes: 2500},
						},
					},
					"handler2": {
						Interval:  60,
						FirstTime: now,
						Segments: []RPCStats{
							{Requests: 50, IncomingBytes: 500, OutgoingBytes: 1000},
							{Requests: 75, IncomingBytes: 750, OutgoingBytes: 1250},
						},
					},
					"handler3": {
						Interval:  60,
						FirstTime: now,
						Segments: []RPCStats{
							{Requests: 25, IncomingBytes: 250, OutgoingBytes: 500},
						},
					},
				},
			},
			verify: func(t *testing.T, result RPCStats) {
				// Total across all handlers and all segments
				if result.Requests != 450 { // 100+200+50+75+25
					t.Errorf("Requests = %d, want 450", result.Requests)
				}
				if result.IncomingBytes != 4000 { // 1000+1500+500+750+250
					t.Errorf("IncomingBytes = %d, want 4000", result.IncomingBytes)
				}
				if result.OutgoingBytes != 7250 { // 2000+2500+1000+1250+500
					t.Errorf("OutgoingBytes = %d, want 7250", result.OutgoingBytes)
				}
			},
		},
		{
			name: "handlers with empty segments",
			input: &RPCMetrics{
				LastDay: map[string]SegmentedRPCMetrics{
					"handler1": {
						Interval:  60,
						FirstTime: now,
						Segments:  []RPCStats{}, // Empty segments
					},
					"handler2": {
						Interval:  60,
						FirstTime: now,
						Segments: []RPCStats{
							{Requests: 100, IncomingBytes: 1000, OutgoingBytes: 2000},
						},
					},
				},
			},
			verify: func(t *testing.T, result RPCStats) {
				// Only handler2's data should be counted
				if result.Requests != 100 {
					t.Errorf("Requests = %d, want 100", result.Requests)
				}
				if result.IncomingBytes != 1000 {
					t.Errorf("IncomingBytes = %d, want 1000", result.IncomingBytes)
				}
				if result.OutgoingBytes != 2000 {
					t.Errorf("OutgoingBytes = %d, want 2000", result.OutgoingBytes)
				}
			},
		},
		{
			name: "merging with different timestamps across segments",
			input: &RPCMetrics{
				LastDay: map[string]SegmentedRPCMetrics{
					"handler1": {
						Interval:  60,
						FirstTime: now,
						Segments: []RPCStats{
							{
								StartTime:     &now,
								EndTime:       &now,
								Requests:      100,
								IncomingBytes: 1000,
							},
							{
								StartTime:     nil, // Different timestamps
								EndTime:       nil,
								Requests:      200,
								IncomingBytes: 2000,
							},
						},
					},
				},
			},
			verify: func(t *testing.T, result RPCStats) {
				// When first segment has timestamps and requests, they stay
				// But when merging with nil timestamps, they should stay as is
				// because the first merge copies the timestamp
				if result.StartTime == nil || !result.StartTime.Equal(now) {
					t.Error("StartTime should be preserved from first segment")
				}
				if result.EndTime == nil || !result.EndTime.Equal(now) {
					t.Error("EndTime should be preserved from first segment")
				}
				if result.Requests != 300 {
					t.Errorf("Requests = %d, want 300", result.Requests)
				}
				if result.IncomingBytes != 3000 {
					t.Errorf("IncomingBytes = %d, want 3000", result.IncomingBytes)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.LastDayTotal()
			tt.verify(t, result)
		})
	}
}

// TestNetMetricsMerge tests NetMetrics.Merge functionality with Interfaces field
func TestNetMetricsMerge(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-time.Hour)

	tests := []struct {
		name   string
		base   *NetMetrics
		other  *NetMetrics
		verify func(t *testing.T, result *NetMetrics)
	}{
		{
			name:  "merge nil other",
			base:  &NetMetrics{CollectedAt: now},
			other: nil,
			verify: func(t *testing.T, result *NetMetrics) {
				if !result.CollectedAt.Equal(now) {
					t.Error("CollectedAt should not change when merging nil")
				}
			},
		},
		{
			name: "merge timestamps - use latest",
			base: &NetMetrics{
				CollectedAt: earlier,
			},
			other: &NetMetrics{
				CollectedAt: now,
			},
			verify: func(t *testing.T, result *NetMetrics) {
				if !result.CollectedAt.Equal(now) {
					t.Error("CollectedAt should use the latest timestamp")
				}
			},
		},
		{
			name: "merge NetStats",
			base: &NetMetrics{
				CollectedAt: now,
				NetStats: procfs.NetDevLine{
					RxBytes:   1000,
					TxBytes:   2000,
					RxPackets: 100,
					TxPackets: 200,
				},
			},
			other: &NetMetrics{
				CollectedAt: earlier,
				NetStats: procfs.NetDevLine{
					RxBytes:   500,
					TxBytes:   1500,
					RxPackets: 50,
					TxPackets: 150,
				},
			},
			verify: func(t *testing.T, result *NetMetrics) {
				if result.NetStats.RxBytes != 1500 {
					t.Errorf("NetStats.RxBytes = %d, want 1500", result.NetStats.RxBytes)
				}
				if result.NetStats.TxBytes != 3500 {
					t.Errorf("NetStats.TxBytes = %d, want 3500", result.NetStats.TxBytes)
				}
				if result.NetStats.RxPackets != 150 {
					t.Errorf("NetStats.RxPackets = %d, want 150", result.NetStats.RxPackets)
				}
				if result.NetStats.TxPackets != 350 {
					t.Errorf("NetStats.TxPackets = %d, want 350", result.NetStats.TxPackets)
				}
			},
		},
		{
			name: "merge Interfaces - nil base map",
			base: &NetMetrics{
				CollectedAt: now,
			},
			other: &NetMetrics{
				CollectedAt: earlier,
				Interfaces: map[string]InterfaceStats{
					"eth0": {
						N: 1,
						NetDevLine: procfs.NetDevLine{
							RxBytes: 1000,
							TxBytes: 2000,
						},
					},
					"eth1": {
						N: 1,
						NetDevLine: procfs.NetDevLine{
							RxBytes: 3000,
							TxBytes: 4000,
						},
					},
				},
			},
			verify: func(t *testing.T, result *NetMetrics) {
				if result.Interfaces == nil {
					t.Fatal("Interfaces should not be nil after merge")
				}
				if len(result.Interfaces) != 2 {
					t.Errorf("Interfaces length = %d, want 2", len(result.Interfaces))
				}
				if eth0, ok := result.Interfaces["eth0"]; !ok {
					t.Error("Interfaces should contain eth0")
				} else {
					if eth0.RxBytes != 1000 {
						t.Errorf("Interfaces[eth0].RxBytes = %d, want 1000", eth0.RxBytes)
					}
					if eth0.TxBytes != 2000 {
						t.Errorf("Interfaces[eth0].TxBytes = %d, want 2000", eth0.TxBytes)
					}
				}
				if eth1, ok := result.Interfaces["eth1"]; !ok {
					t.Error("Interfaces should contain eth1")
				} else {
					if eth1.RxBytes != 3000 {
						t.Errorf("Interfaces[eth1].RxBytes = %d, want 3000", eth1.RxBytes)
					}
					if eth1.TxBytes != 4000 {
						t.Errorf("Interfaces[eth1].TxBytes = %d, want 4000", eth1.TxBytes)
					}
				}
			},
		},
		{
			name: "merge Interfaces - accumulate existing entries",
			base: &NetMetrics{
				CollectedAt: now,
				Interfaces: map[string]InterfaceStats{
					"eth0": {
						N: 1,
						NetDevLine: procfs.NetDevLine{
							RxBytes:   1000,
							TxBytes:   2000,
							RxPackets: 100,
							TxPackets: 200,
						},
					},
					"eth1": {
						N: 1,
						NetDevLine: procfs.NetDevLine{
							RxBytes: 5000,
							TxBytes: 6000,
						},
					},
				},
			},
			other: &NetMetrics{
				CollectedAt: earlier,
				Interfaces: map[string]InterfaceStats{
					"eth0": {
						N: 1,
						NetDevLine: procfs.NetDevLine{
							RxBytes:   500,
							TxBytes:   1500,
							RxPackets: 50,
							TxPackets: 150,
						},
					},
					"eth2": {
						N: 1,
						NetDevLine: procfs.NetDevLine{
							RxBytes: 7000,
							TxBytes: 8000,
						},
					},
				},
			},
			verify: func(t *testing.T, result *NetMetrics) {
				if len(result.Interfaces) != 3 {
					t.Errorf("Interfaces length = %d, want 3", len(result.Interfaces))
				}
				// Check eth0 (should be accumulated)
				if eth0, ok := result.Interfaces["eth0"]; !ok {
					t.Error("Interfaces should contain eth0")
				} else {
					if eth0.RxBytes != 1500 {
						t.Errorf("Interfaces[eth0].RxBytes = %d, want 1500", eth0.RxBytes)
					}
					if eth0.TxBytes != 3500 {
						t.Errorf("Interfaces[eth0].TxBytes = %d, want 3500", eth0.TxBytes)
					}
					if eth0.RxPackets != 150 {
						t.Errorf("Interfaces[eth0].RxPackets = %d, want 150", eth0.RxPackets)
					}
					if eth0.TxPackets != 350 {
						t.Errorf("Interfaces[eth0].TxPackets = %d, want 350", eth0.TxPackets)
					}
				}
				// Check eth1 (should remain unchanged)
				if eth1, ok := result.Interfaces["eth1"]; !ok {
					t.Error("Interfaces should contain eth1")
				} else {
					if eth1.RxBytes != 5000 {
						t.Errorf("Interfaces[eth1].RxBytes = %d, want 5000", eth1.RxBytes)
					}
					if eth1.TxBytes != 6000 {
						t.Errorf("Interfaces[eth1].TxBytes = %d, want 6000", eth1.TxBytes)
					}
				}
				// Check eth2 (should be added)
				if eth2, ok := result.Interfaces["eth2"]; !ok {
					t.Error("Interfaces should contain eth2")
				} else {
					if eth2.RxBytes != 7000 {
						t.Errorf("Interfaces[eth2].RxBytes = %d, want 7000", eth2.RxBytes)
					}
					if eth2.TxBytes != 8000 {
						t.Errorf("Interfaces[eth2].TxBytes = %d, want 8000", eth2.TxBytes)
					}
				}
			},
		},
		{
			name: "merge all fields together",
			base: &NetMetrics{
				CollectedAt: earlier,
				Interfaces: map[string]InterfaceStats{
					"lo": {
						N: 1,
						NetDevLine: procfs.NetDevLine{
							RxBytes: 100,
							TxBytes: 100,
						},
					},
				},
				NetStats: procfs.NetDevLine{
					RxBytes: 10000,
					TxBytes: 20000,
				},
			},
			other: &NetMetrics{
				CollectedAt: now,
				Interfaces: map[string]InterfaceStats{
					"lo": {
						N: 1,
						NetDevLine: procfs.NetDevLine{
							RxBytes: 200,
							TxBytes: 200,
						},
					},
					"docker0": {
						N: 1,
						NetDevLine: procfs.NetDevLine{
							RxBytes: 500,
							TxBytes: 600,
						},
					},
				},
				NetStats: procfs.NetDevLine{
					RxBytes: 5000,
					TxBytes: 6000,
				},
			},
			verify: func(t *testing.T, result *NetMetrics) {
				// Check timestamp
				if !result.CollectedAt.Equal(now) {
					t.Error("CollectedAt should use the latest timestamp")
				}
				// Check NetStats
				if result.NetStats.RxBytes != 15000 {
					t.Errorf("NetStats.RxBytes = %d, want 15000", result.NetStats.RxBytes)
				}
				if result.NetStats.TxBytes != 26000 {
					t.Errorf("NetStats.TxBytes = %d, want 26000", result.NetStats.TxBytes)
				}
				// Check Interfaces
				if len(result.Interfaces) != 2 {
					t.Errorf("Interfaces length = %d, want 2", len(result.Interfaces))
				}
				if lo, ok := result.Interfaces["lo"]; !ok {
					t.Error("Interfaces should contain lo")
				} else {
					if lo.RxBytes != 300 {
						t.Errorf("Interfaces[lo].RxBytes = %d, want 300", lo.RxBytes)
					}
					if lo.TxBytes != 300 {
						t.Errorf("Interfaces[lo].TxBytes = %d, want 300", lo.TxBytes)
					}
				}
				if docker0, ok := result.Interfaces["docker0"]; !ok {
					t.Error("Interfaces should contain docker0")
				} else {
					if docker0.RxBytes != 500 {
						t.Errorf("Interfaces[docker0].RxBytes = %d, want 500", docker0.RxBytes)
					}
					if docker0.TxBytes != 600 {
						t.Errorf("Interfaces[docker0].TxBytes = %d, want 600", docker0.TxBytes)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.base.Merge(tt.other)
			tt.verify(t, tt.base)
		})
	}
}
