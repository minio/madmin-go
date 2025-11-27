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
	"math"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/load"
)

// Helper function for comparing floats with tolerance
func almostEqual(a, b float64) bool {
	const epsilon = 1e-9
	return math.Abs(a-b) < epsilon
}

func TestAddCPUs(t *testing.T) {
	tests := []struct {
		name     string
		cpus     CPUs
		initial  CPUMetrics
		expected CPUMetrics
	}{
		{
			name: "Add CPUs to empty metrics",
			cpus: CPUs{
				CPUs: []CPU{
					{
						ModelName: "Intel Core i7",
						Mhz:       2600.0,
						Cores:     4,
						CacheSize: 8192,
					},
					{
						ModelName: "Intel Core i7",
						Mhz:       2800.0,
						Cores:     4,
						CacheSize: 8192,
					},
				},
				CPUFreqStats: []CPUFreqStats{
					{
						Governor:                "performance",
						CpuinfoCurrentFrequency: uint64Ptr(2600000),
						CpuinfoMinimumFrequency: uint64Ptr(800000),
						CpuinfoMaximumFrequency: uint64Ptr(3400000),
						ScalingCurrentFrequency: uint64Ptr(2500000),
						ScalingMinimumFrequency: uint64Ptr(800000),
						ScalingMaximumFrequency: uint64Ptr(3400000),
					},
				},
			},
			initial: CPUMetrics{},
			expected: CPUMetrics{
				CPUByModel:              map[string]int{"Intel Core i7": 2},
				TotalMhz:                5400.0,
				TotalCores:              8,
				TotalCacheSize:          16777216, // 8192 * 1024 * 2
				CPUCount:                2,
				FreqStatsCount:          1,
				GovernorFreq:            map[string]int{"performance": 1},
				TotalCurrentFreq:        2600000,
				TotalScalingCurrentFreq: 2500000,
				MinCPUInfoFreq:          800000,
				MaxCPUInfoFreq:          3400000,
				MinScalingFreq:          800000,
				MaxScalingFreq:          3400000,
			},
		},
		{
			name: "Add CPUs with different models",
			cpus: CPUs{
				CPUs: []CPU{
					{
						ModelName: "AMD Ryzen 9",
						Mhz:       3800.0,
						Cores:     8,
						CacheSize: 32768,
					},
					{
						ModelName: "Intel Xeon",
						Mhz:       2100.0,
						Cores:     16,
						CacheSize: 25600,
					},
				},
			},
			initial: CPUMetrics{
				CPUByModel:     map[string]int{"Intel Core i7": 2},
				TotalMhz:       5400.0,
				TotalCores:     8,
				TotalCacheSize: 16777216,
				CPUCount:       2,
			},
			expected: CPUMetrics{
				CPUByModel:     map[string]int{"Intel Core i7": 2, "AMD Ryzen 9": 1, "Intel Xeon": 1},
				TotalMhz:       11300.0,  // 5400 + 3800 + 2100
				TotalCores:     32,       // 8 + 8 + 16
				TotalCacheSize: 76546048, // 16777216 + 32768*1024 + 25600*1024
				CPUCount:       4,
			},
		},
		{
			name: "Add frequency stats with multiple governors",
			cpus: CPUs{
				CPUFreqStats: []CPUFreqStats{
					{
						Governor:                "powersave",
						CpuinfoCurrentFrequency: uint64Ptr(1200000),
						CpuinfoMinimumFrequency: uint64Ptr(600000),
						CpuinfoMaximumFrequency: uint64Ptr(3000000),
					},
					{
						Governor:                "performance",
						CpuinfoCurrentFrequency: uint64Ptr(2800000),
						CpuinfoMinimumFrequency: uint64Ptr(800000),
						CpuinfoMaximumFrequency: uint64Ptr(3600000),
					},
				},
			},
			initial: CPUMetrics{
				FreqStatsCount:   1,
				GovernorFreq:     map[string]int{"performance": 1},
				TotalCurrentFreq: 2600000,
				MinCPUInfoFreq:   800000,
				MaxCPUInfoFreq:   3400000,
			},
			expected: CPUMetrics{
				FreqStatsCount:   3,
				GovernorFreq:     map[string]int{"performance": 2, "powersave": 1},
				TotalCurrentFreq: 6600000, // 2600000 + 1200000 + 2800000
				MinCPUInfoFreq:   600000,  // Min of 800000 and 600000
				MaxCPUInfoFreq:   3600000, // Max of 3400000 and 3600000
			},
		},
		{
			name: "Handle nil frequency values",
			cpus: CPUs{
				CPUFreqStats: []CPUFreqStats{
					{
						Governor:                "ondemand",
						CpuinfoCurrentFrequency: nil,
						CpuinfoMinimumFrequency: uint64Ptr(1000000),
						CpuinfoMaximumFrequency: nil,
					},
				},
			},
			initial: CPUMetrics{},
			expected: CPUMetrics{
				FreqStatsCount: 1,
				GovernorFreq:   map[string]int{"ondemand": 1},
				MinCPUInfoFreq: 1000000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.initial
			tt.cpus.AddCPUs(&m)

			// Check CPU model counts
			if len(m.CPUByModel) != len(tt.expected.CPUByModel) {
				t.Errorf("CPUByModel length mismatch: got %d, want %d", len(m.CPUByModel), len(tt.expected.CPUByModel))
			}
			for model, count := range tt.expected.CPUByModel {
				if m.CPUByModel[model] != count {
					t.Errorf("CPUByModel[%s]: got %d, want %d", model, m.CPUByModel[model], count)
				}
			}

			// Check accumulated values
			if m.TotalMhz != tt.expected.TotalMhz {
				t.Errorf("TotalMhz: got %f, want %f", m.TotalMhz, tt.expected.TotalMhz)
			}
			if m.TotalCores != tt.expected.TotalCores {
				t.Errorf("TotalCores: got %d, want %d", m.TotalCores, tt.expected.TotalCores)
			}
			if m.TotalCacheSize != tt.expected.TotalCacheSize {
				t.Errorf("TotalCacheSize: got %d, want %d", m.TotalCacheSize, tt.expected.TotalCacheSize)
			}
			if m.CPUCount != tt.expected.CPUCount {
				t.Errorf("CPUCount: got %d, want %d", m.CPUCount, tt.expected.CPUCount)
			}

			// Check frequency stats
			if m.FreqStatsCount != tt.expected.FreqStatsCount {
				t.Errorf("FreqStatsCount: got %d, want %d", m.FreqStatsCount, tt.expected.FreqStatsCount)
			}
			if len(m.GovernorFreq) != len(tt.expected.GovernorFreq) {
				t.Errorf("GovernorFreq length mismatch: got %d, want %d", len(m.GovernorFreq), len(tt.expected.GovernorFreq))
			}
			for gov, count := range tt.expected.GovernorFreq {
				if m.GovernorFreq[gov] != count {
					t.Errorf("GovernorFreq[%s]: got %d, want %d", gov, m.GovernorFreq[gov], count)
				}
			}
			if m.TotalCurrentFreq != tt.expected.TotalCurrentFreq {
				t.Errorf("TotalCurrentFreq: got %d, want %d", m.TotalCurrentFreq, tt.expected.TotalCurrentFreq)
			}
			if m.TotalScalingCurrentFreq != tt.expected.TotalScalingCurrentFreq {
				t.Errorf("TotalScalingCurrentFreq: got %d, want %d", m.TotalScalingCurrentFreq, tt.expected.TotalScalingCurrentFreq)
			}
			if m.MinCPUInfoFreq != tt.expected.MinCPUInfoFreq {
				t.Errorf("MinCPUInfoFreq: got %d, want %d", m.MinCPUInfoFreq, tt.expected.MinCPUInfoFreq)
			}
			if m.MaxCPUInfoFreq != tt.expected.MaxCPUInfoFreq {
				t.Errorf("MaxCPUInfoFreq: got %d, want %d", m.MaxCPUInfoFreq, tt.expected.MaxCPUInfoFreq)
			}
			if m.MinScalingFreq != tt.expected.MinScalingFreq {
				t.Errorf("MinScalingFreq: got %d, want %d", m.MinScalingFreq, tt.expected.MinScalingFreq)
			}
			if m.MaxScalingFreq != tt.expected.MaxScalingFreq {
				t.Errorf("MaxScalingFreq: got %d, want %d", m.MaxScalingFreq, tt.expected.MaxScalingFreq)
			}
		})
	}
}

func TestCPUMetricsMergeAggregated(t *testing.T) {
	tests := []struct {
		name     string
		m        CPUMetrics
		other    CPUMetrics
		expected CPUMetrics
	}{
		{
			name: "Merge into empty metrics",
			m:    CPUMetrics{},
			other: CPUMetrics{
				CollectedAt:             time.Now(),
				Nodes:                   2,
				TimesStat:               cpu.TimesStat{User: 100, System: 50},
				LoadStat:                load.AvgStat{Load1: 1.5, Load5: 2.0, Load15: 1.8},
				CPUCount:                4,
				CPUByModel:              map[string]int{"Intel Core i7": 4},
				TotalMhz:                10400.0,
				TotalCores:              16,
				TotalCacheSize:          33554432,
				FreqStatsCount:          2,
				GovernorFreq:            map[string]int{"performance": 2},
				TotalCurrentFreq:        5200000,
				TotalScalingCurrentFreq: 5000000,
				MinCPUInfoFreq:          800000,
				MaxCPUInfoFreq:          3400000,
				MinScalingFreq:          800000,
				MaxScalingFreq:          3400000,
			},
			expected: CPUMetrics{
				Nodes:                   2,
				TimesStat:               cpu.TimesStat{User: 100, System: 50},
				LoadStat:                load.AvgStat{Load1: 1.5, Load5: 2.0, Load15: 1.8},
				CPUCount:                4,
				CPUByModel:              map[string]int{"Intel Core i7": 4},
				TotalMhz:                10400.0,
				TotalCores:              16,
				TotalCacheSize:          33554432,
				FreqStatsCount:          2,
				GovernorFreq:            map[string]int{"performance": 2},
				TotalCurrentFreq:        5200000,
				TotalScalingCurrentFreq: 5000000,
				MinCPUInfoFreq:          800000,
				MaxCPUInfoFreq:          3400000,
				MinScalingFreq:          800000,
				MaxScalingFreq:          3400000,
			},
		},
		{
			name: "Merge two non-empty metrics",
			m: CPUMetrics{
				Nodes:                   1,
				TimesStat:               cpu.TimesStat{User: 100, System: 50},
				LoadStat:                load.AvgStat{Load1: 1.0, Load5: 1.5, Load15: 1.2},
				CPUCount:                2,
				CPUByModel:              map[string]int{"Intel Core i7": 2},
				TotalMhz:                5200.0,
				TotalCores:              8,
				TotalCacheSize:          16777216,
				FreqStatsCount:          1,
				GovernorFreq:            map[string]int{"performance": 1},
				TotalCurrentFreq:        2600000,
				TotalScalingCurrentFreq: 2500000,
				MinCPUInfoFreq:          800000,
				MaxCPUInfoFreq:          3400000,
				MinScalingFreq:          900000,
				MaxScalingFreq:          3300000,
			},
			other: CPUMetrics{
				Nodes:                   2,
				TimesStat:               cpu.TimesStat{User: 150, System: 75},
				LoadStat:                load.AvgStat{Load1: 2.0, Load5: 2.5, Load15: 2.2},
				CPUCount:                4,
				CPUByModel:              map[string]int{"Intel Core i7": 2, "AMD Ryzen 9": 2},
				TotalMhz:                7600.0,
				TotalCores:              16,
				TotalCacheSize:          33554432,
				FreqStatsCount:          2,
				GovernorFreq:            map[string]int{"performance": 1, "powersave": 1},
				TotalCurrentFreq:        5200000,
				TotalScalingCurrentFreq: 5000000,
				MinCPUInfoFreq:          700000,
				MaxCPUInfoFreq:          3600000,
				MinScalingFreq:          800000,
				MaxScalingFreq:          3500000,
			},
			expected: CPUMetrics{
				Nodes:                   3,
				TimesStat:               cpu.TimesStat{User: 250, System: 125},
				LoadStat:                load.AvgStat{Load1: 3.0, Load5: 4.0, Load15: 3.4},
				CPUCount:                6,
				CPUByModel:              map[string]int{"Intel Core i7": 4, "AMD Ryzen 9": 2},
				TotalMhz:                12800.0,
				TotalCores:              24,
				TotalCacheSize:          50331648,
				FreqStatsCount:          3,
				GovernorFreq:            map[string]int{"performance": 2, "powersave": 1},
				TotalCurrentFreq:        7800000,
				TotalScalingCurrentFreq: 7500000,
				MinCPUInfoFreq:          700000,
				MaxCPUInfoFreq:          3600000,
				MinScalingFreq:          800000,
				MaxScalingFreq:          3500000,
			},
		},
		{
			name: "Merge with nil TimesStat and LoadStat",
			m: CPUMetrics{
				Nodes:    1,
				CPUCount: 2,
			},
			other: CPUMetrics{
				Nodes:     2,
				TimesStat: cpu.TimesStat{User: 100, System: 50},
				LoadStat:  load.AvgStat{Load1: 1.5, Load5: 2.0, Load15: 1.8},
				CPUCount:  4,
			},
			expected: CPUMetrics{
				Nodes:     3,
				TimesStat: cpu.TimesStat{User: 100, System: 50},
				LoadStat:  load.AvgStat{Load1: 1.5, Load5: 2.0, Load15: 1.8},
				CPUCount:  6,
			},
		},
		{
			name: "Merge handles min/max correctly",
			m: CPUMetrics{
				FreqStatsCount: 1,
				MinCPUInfoFreq: 1000000,
				MaxCPUInfoFreq: 3000000,
				MinScalingFreq: 1000000,
				MaxScalingFreq: 3000000,
			},
			other: CPUMetrics{
				FreqStatsCount: 1,
				MinCPUInfoFreq: 800000,  // Lower min
				MaxCPUInfoFreq: 3400000, // Higher max
				MinScalingFreq: 1200000, // Higher min (should not update)
				MaxScalingFreq: 2800000, // Lower max (should not update)
			},
			expected: CPUMetrics{
				FreqStatsCount: 2,
				MinCPUInfoFreq: 800000,  // Takes lower
				MaxCPUInfoFreq: 3400000, // Takes higher
				MinScalingFreq: 1000000, // Keeps lower
				MaxScalingFreq: 3000000, // Keeps higher
			},
		},
		{
			name: "Merge with zero MinFreq values",
			m: CPUMetrics{
				FreqStatsCount: 0,
				MinCPUInfoFreq: 0,
				MaxCPUInfoFreq: 0,
			},
			other: CPUMetrics{
				FreqStatsCount: 1,
				MinCPUInfoFreq: 800000,
				MaxCPUInfoFreq: 3400000,
			},
			expected: CPUMetrics{
				FreqStatsCount: 1,
				MinCPUInfoFreq: 800000,
				MaxCPUInfoFreq: 3400000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.m
			m.Merge(&tt.other)

			// Check basic fields
			if m.Nodes != tt.expected.Nodes {
				t.Errorf("Nodes: got %d, want %d", m.Nodes, tt.expected.Nodes)
			}
			if m.CPUCount != tt.expected.CPUCount {
				t.Errorf("CPUCount: got %d, want %d", m.CPUCount, tt.expected.CPUCount)
			}

			// Check TimesStat
			if m.TimesStat.User != tt.expected.TimesStat.User {
				t.Errorf("TimesStat.User: got %f, want %f", m.TimesStat.User, tt.expected.TimesStat.User)
			}
			if m.TimesStat.System != tt.expected.TimesStat.System {
				t.Errorf("TimesStat.System: got %f, want %f", m.TimesStat.System, tt.expected.TimesStat.System)
			}

			// Check LoadStat
			if !almostEqual(m.LoadStat.Load1, tt.expected.LoadStat.Load1) {
				t.Errorf("LoadStat.Load1: got %f, want %f", m.LoadStat.Load1, tt.expected.LoadStat.Load1)
			}
			if !almostEqual(m.LoadStat.Load5, tt.expected.LoadStat.Load5) {
				t.Errorf("LoadStat.Load5: got %f, want %f", m.LoadStat.Load5, tt.expected.LoadStat.Load5)
			}
			if !almostEqual(m.LoadStat.Load15, tt.expected.LoadStat.Load15) {
				t.Errorf("LoadStat.Load15: got %f, want %f", m.LoadStat.Load15, tt.expected.LoadStat.Load15)
			}

			// Check CPU model counts
			if len(m.CPUByModel) != len(tt.expected.CPUByModel) {
				t.Errorf("CPUByModel length mismatch: got %d, want %d", len(m.CPUByModel), len(tt.expected.CPUByModel))
			}
			for model, count := range tt.expected.CPUByModel {
				if m.CPUByModel[model] != count {
					t.Errorf("CPUByModel[%s]: got %d, want %d", model, m.CPUByModel[model], count)
				}
			}

			// Check accumulated values
			if m.TotalMhz != tt.expected.TotalMhz {
				t.Errorf("TotalMhz: got %f, want %f", m.TotalMhz, tt.expected.TotalMhz)
			}
			if m.TotalCores != tt.expected.TotalCores {
				t.Errorf("TotalCores: got %d, want %d", m.TotalCores, tt.expected.TotalCores)
			}
			if m.TotalCacheSize != tt.expected.TotalCacheSize {
				t.Errorf("TotalCacheSize: got %d, want %d", m.TotalCacheSize, tt.expected.TotalCacheSize)
			}

			// Check frequency stats
			if m.FreqStatsCount != tt.expected.FreqStatsCount {
				t.Errorf("FreqStatsCount: got %d, want %d", m.FreqStatsCount, tt.expected.FreqStatsCount)
			}
			for gov, count := range tt.expected.GovernorFreq {
				if m.GovernorFreq[gov] != count {
					t.Errorf("GovernorFreq[%s]: got %d, want %d", gov, m.GovernorFreq[gov], count)
				}
			}
			if m.TotalCurrentFreq != tt.expected.TotalCurrentFreq {
				t.Errorf("TotalCurrentFreq: got %d, want %d", m.TotalCurrentFreq, tt.expected.TotalCurrentFreq)
			}
			if m.TotalScalingCurrentFreq != tt.expected.TotalScalingCurrentFreq {
				t.Errorf("TotalScalingCurrentFreq: got %d, want %d", m.TotalScalingCurrentFreq, tt.expected.TotalScalingCurrentFreq)
			}
			if m.MinCPUInfoFreq != tt.expected.MinCPUInfoFreq {
				t.Errorf("MinCPUInfoFreq: got %d, want %d", m.MinCPUInfoFreq, tt.expected.MinCPUInfoFreq)
			}
			if m.MaxCPUInfoFreq != tt.expected.MaxCPUInfoFreq {
				t.Errorf("MaxCPUInfoFreq: got %d, want %d", m.MaxCPUInfoFreq, tt.expected.MaxCPUInfoFreq)
			}
			if m.MinScalingFreq != tt.expected.MinScalingFreq {
				t.Errorf("MinScalingFreq: got %d, want %d", m.MinScalingFreq, tt.expected.MinScalingFreq)
			}
			if m.MaxScalingFreq != tt.expected.MaxScalingFreq {
				t.Errorf("MaxScalingFreq: got %d, want %d", m.MaxScalingFreq, tt.expected.MaxScalingFreq)
			}
		})
	}
}

func TestCPUMetricsMergeNil(t *testing.T) {
	m := CPUMetrics{
		Nodes:    1,
		CPUCount: 2,
	}

	// Test merging with nil
	m.Merge(nil)

	if m.Nodes != 1 {
		t.Errorf("Nodes should remain unchanged after merging with nil: got %d, want 1", m.Nodes)
	}
	if m.CPUCount != 2 {
		t.Errorf("CPUCount should remain unchanged after merging with nil: got %d, want 2", m.CPUCount)
	}
}

func TestAddCPUsNil(t *testing.T) {
	// Test nil CPUs
	var c *CPUs
	m := CPUMetrics{CPUCount: 1}
	c.AddCPUs(&m)
	if m.CPUCount != 1 {
		t.Errorf("CPUCount should remain unchanged when adding nil CPUs: got %d, want 1", m.CPUCount)
	}

	// Test nil metrics
	c2 := CPUs{
		CPUs: []CPU{{ModelName: "Test", Cores: 4}},
	}
	c2.AddCPUs(nil)
	// Should not panic
}

// Helper function to create a pointer to uint64
func uint64Ptr(v uint64) *uint64 {
	return &v
}
