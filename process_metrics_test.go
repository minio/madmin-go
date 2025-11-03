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
	"github.com/shirou/gopsutil/v4/process"
)

func TestAddProcInfo(t *testing.T) {
	tests := []struct {
		name     string
		procInfo ProcInfo
		initial  ProcessMetrics
		expected ProcessMetrics
	}{
		{
			name: "Add single process to empty metrics",
			procInfo: ProcInfo{
				CPUPercent:     15.5,
				NumConnections: 10,
				CreateTime:     1640995200000, // 2022-01-01 00:00:00 in milliseconds
				NumFDs:         50,
				NumThreads:     4,
				Nice:           0,
				IsBackground:   false,
				IsRunning:      true,
				MemInfo: process.MemoryInfoStat{
					RSS: 1024 * 1024, // 1MB
					VMS: 2048 * 1024, // 2MB
				},
				IOCounters: process.IOCountersStat{
					ReadCount:  100,
					WriteCount: 200,
					ReadBytes:  1024 * 100,
					WriteBytes: 1024 * 200,
				},
				NumCtxSwitches: process.NumCtxSwitchesStat{
					Voluntary:   500,
					Involuntary: 100,
				},
				PageFaults: process.PageFaultsStat{
					MinorFaults:      1000,
					MajorFaults:      10,
					ChildMinorFaults: 50,
					ChildMajorFaults: 5,
				},
				Times: cpu.TimesStat{
					User:      10.5,
					System:    5.2,
					Idle:      0.0,
					Nice:      1.0,
					Iowait:    0.5,
					Irq:       0.1,
					Softirq:   0.2,
					Steal:     0.0,
					Guest:     0.0,
					GuestNice: 0.0,
				},
			},
			initial: ProcessMetrics{},
			expected: ProcessMetrics{
				Nodes:               1,
				TotalCPUPercent:     15.5,
				TotalNumConnections: 10,
				TotalNumFDs:         50,
				TotalNumThreads:     4,
				TotalNice:           0,
				Count:               1,
				BackgroundProcesses: 0,
				RunningProcesses:    1,
				MemInfo: ProcessMemoryInfo{
					RSS:   1024 * 1024,
					VMS:   2048 * 1024,
					Count: 1,
				},
				IOCounters: ProcessIOCounters{
					ReadCount:  100,
					WriteCount: 200,
					ReadBytes:  1024 * 100,
					WriteBytes: 1024 * 200,
					Count:      1,
				},
				NumCtxSwitches: ProcessCtxSwitches{
					Voluntary:   500,
					Involuntary: 100,
					Count:       1,
				},
				PageFaults: ProcessPageFaults{
					MinorFaults:      1000,
					MajorFaults:      10,
					ChildMinorFaults: 50,
					ChildMajorFaults: 5,
					Count:            1,
				},
				CPUTimes: ProcessCPUTimes{
					User:      10.5,
					System:    5.2,
					Idle:      0.0,
					Nice:      1.0,
					Iowait:    0.5,
					Irq:       0.1,
					Softirq:   0.2,
					Steal:     0.0,
					Guest:     0.0,
					GuestNice: 0.0,
					Count:     1,
				},
			},
		},
		{
			name: "Add process with background flag set",
			procInfo: ProcInfo{
				CPUPercent:     5.0,
				NumConnections: 2,
				CreateTime:     1640995200000,
				NumFDs:         20,
				NumThreads:     1,
				Nice:           10,
				IsBackground:   true,
				IsRunning:      false,
				MemInfo: process.MemoryInfoStat{
					RSS: 512 * 1024,
					VMS: 1024 * 1024,
				},
			},
			initial: ProcessMetrics{},
			expected: ProcessMetrics{
				Nodes:               1,
				TotalCPUPercent:     5.0,
				TotalNumConnections: 2,
				TotalNumFDs:         20,
				TotalNumThreads:     1,
				TotalNice:           10,
				Count:               1,
				BackgroundProcesses: 1,
				RunningProcesses:    0,
				MemInfo: ProcessMemoryInfo{
					RSS:   512 * 1024,
					VMS:   1024 * 1024,
					Count: 1,
				},
			},
		},
		{
			name: "Add to existing metrics",
			procInfo: ProcInfo{
				CPUPercent:     8.0,
				NumConnections: 5,
				CreateTime:     1640998800000, // 1 hour later
				NumFDs:         30,
				NumThreads:     2,
				Nice:           -5,
				IsBackground:   true,
				IsRunning:      true,
				MemInfo: process.MemoryInfoStat{
					RSS: 2048 * 1024,
					VMS: 4096 * 1024,
				},
				IOCounters: process.IOCountersStat{
					ReadCount:  50,
					WriteCount: 100,
					ReadBytes:  1024 * 50,
					WriteBytes: 1024 * 100,
				},
			},
			initial: ProcessMetrics{
				Nodes:               2,
				TotalCPUPercent:     15.5,
				TotalNumConnections: 10,
				TotalNumFDs:         50,
				TotalNumThreads:     4,
				TotalNice:           0,
				Count:               1,
				BackgroundProcesses: 0,
				RunningProcesses:    1,
				MemInfo: ProcessMemoryInfo{
					RSS:   1024 * 1024,
					VMS:   2048 * 1024,
					Count: 1,
				},
				IOCounters: ProcessIOCounters{
					ReadCount:  100,
					WriteCount: 200,
					ReadBytes:  1024 * 100,
					WriteBytes: 1024 * 200,
					Count:      1,
				},
			},
			expected: ProcessMetrics{
				Nodes:               1, // Gets reset to 1
				TotalCPUPercent:     23.5,
				TotalNumConnections: 15,
				TotalNumFDs:         80,
				TotalNumThreads:     6,
				TotalNice:           -5,
				Count:               2,
				BackgroundProcesses: 1,
				RunningProcesses:    2,
				MemInfo: ProcessMemoryInfo{
					RSS:   3072 * 1024, // 1MB + 2MB
					VMS:   6144 * 1024, // 2MB + 4MB
					Count: 2,
				},
				IOCounters: ProcessIOCounters{
					ReadCount:  150,
					WriteCount: 300,
					ReadBytes:  1024 * 150,
					WriteBytes: 1024 * 300,
					Count:      2,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.initial
			tt.procInfo.AddProcInfo(&m)

			// Check basic fields
			if m.Nodes != tt.expected.Nodes {
				t.Errorf("Nodes: got %d, want %d", m.Nodes, tt.expected.Nodes)
			}
			if !almostEqualFloat64(m.TotalCPUPercent, tt.expected.TotalCPUPercent) {
				t.Errorf("TotalCPUPercent: got %f, want %f", m.TotalCPUPercent, tt.expected.TotalCPUPercent)
			}
			if m.TotalNumConnections != tt.expected.TotalNumConnections {
				t.Errorf("TotalNumConnections: got %d, want %d", m.TotalNumConnections, tt.expected.TotalNumConnections)
			}
			if m.TotalNumFDs != tt.expected.TotalNumFDs {
				t.Errorf("TotalNumFDs: got %d, want %d", m.TotalNumFDs, tt.expected.TotalNumFDs)
			}
			if m.TotalNumThreads != tt.expected.TotalNumThreads {
				t.Errorf("TotalNumThreads: got %d, want %d", m.TotalNumThreads, tt.expected.TotalNumThreads)
			}
			if m.TotalNice != tt.expected.TotalNice {
				t.Errorf("TotalNice: got %d, want %d", m.TotalNice, tt.expected.TotalNice)
			}
			if m.Count != tt.expected.Count {
				t.Errorf("Count: got %d, want %d", m.Count, tt.expected.Count)
			}
			if m.BackgroundProcesses != tt.expected.BackgroundProcesses {
				t.Errorf("BackgroundProcesses: got %d, want %d", m.BackgroundProcesses, tt.expected.BackgroundProcesses)
			}
			if m.RunningProcesses != tt.expected.RunningProcesses {
				t.Errorf("RunningProcesses: got %d, want %d", m.RunningProcesses, tt.expected.RunningProcesses)
			}

			// Check memory info
			if m.MemInfo.RSS != tt.expected.MemInfo.RSS {
				t.Errorf("MemInfo.RSS: got %d, want %d", m.MemInfo.RSS, tt.expected.MemInfo.RSS)
			}
			if m.MemInfo.VMS != tt.expected.MemInfo.VMS {
				t.Errorf("MemInfo.VMS: got %d, want %d", m.MemInfo.VMS, tt.expected.MemInfo.VMS)
			}
			if m.MemInfo.Count != tt.expected.MemInfo.Count {
				t.Errorf("MemInfo.Count: got %d, want %d", m.MemInfo.Count, tt.expected.MemInfo.Count)
			}

			// Check IO counters if expected
			if tt.expected.IOCounters.Count > 0 {
				if m.IOCounters.ReadCount != tt.expected.IOCounters.ReadCount {
					t.Errorf("IOCounters.ReadCount: got %d, want %d", m.IOCounters.ReadCount, tt.expected.IOCounters.ReadCount)
				}
				if m.IOCounters.WriteCount != tt.expected.IOCounters.WriteCount {
					t.Errorf("IOCounters.WriteCount: got %d, want %d", m.IOCounters.WriteCount, tt.expected.IOCounters.WriteCount)
				}
				if m.IOCounters.ReadBytes != tt.expected.IOCounters.ReadBytes {
					t.Errorf("IOCounters.ReadBytes: got %d, want %d", m.IOCounters.ReadBytes, tt.expected.IOCounters.ReadBytes)
				}
				if m.IOCounters.WriteBytes != tt.expected.IOCounters.WriteBytes {
					t.Errorf("IOCounters.WriteBytes: got %d, want %d", m.IOCounters.WriteBytes, tt.expected.IOCounters.WriteBytes)
				}
				if m.IOCounters.Count != tt.expected.IOCounters.Count {
					t.Errorf("IOCounters.Count: got %d, want %d", m.IOCounters.Count, tt.expected.IOCounters.Count)
				}
			}

			// Check context switches if expected
			if tt.expected.NumCtxSwitches.Count > 0 {
				if m.NumCtxSwitches.Voluntary != tt.expected.NumCtxSwitches.Voluntary {
					t.Errorf("NumCtxSwitches.Voluntary: got %d, want %d", m.NumCtxSwitches.Voluntary, tt.expected.NumCtxSwitches.Voluntary)
				}
				if m.NumCtxSwitches.Involuntary != tt.expected.NumCtxSwitches.Involuntary {
					t.Errorf("NumCtxSwitches.Involuntary: got %d, want %d", m.NumCtxSwitches.Involuntary, tt.expected.NumCtxSwitches.Involuntary)
				}
				if m.NumCtxSwitches.Count != tt.expected.NumCtxSwitches.Count {
					t.Errorf("NumCtxSwitches.Count: got %d, want %d", m.NumCtxSwitches.Count, tt.expected.NumCtxSwitches.Count)
				}
			}

			// Check page faults if expected
			if tt.expected.PageFaults.Count > 0 {
				if m.PageFaults.MinorFaults != tt.expected.PageFaults.MinorFaults {
					t.Errorf("PageFaults.MinorFaults: got %d, want %d", m.PageFaults.MinorFaults, tt.expected.PageFaults.MinorFaults)
				}
				if m.PageFaults.MajorFaults != tt.expected.PageFaults.MajorFaults {
					t.Errorf("PageFaults.MajorFaults: got %d, want %d", m.PageFaults.MajorFaults, tt.expected.PageFaults.MajorFaults)
				}
				if m.PageFaults.ChildMinorFaults != tt.expected.PageFaults.ChildMinorFaults {
					t.Errorf("PageFaults.ChildMinorFaults: got %d, want %d", m.PageFaults.ChildMinorFaults, tt.expected.PageFaults.ChildMinorFaults)
				}
				if m.PageFaults.ChildMajorFaults != tt.expected.PageFaults.ChildMajorFaults {
					t.Errorf("PageFaults.ChildMajorFaults: got %d, want %d", m.PageFaults.ChildMajorFaults, tt.expected.PageFaults.ChildMajorFaults)
				}
				if m.PageFaults.Count != tt.expected.PageFaults.Count {
					t.Errorf("PageFaults.Count: got %d, want %d", m.PageFaults.Count, tt.expected.PageFaults.Count)
				}
			}

			// Check CPU times if expected
			if tt.expected.CPUTimes.Count > 0 {
				if !almostEqualFloat64(m.CPUTimes.User, tt.expected.CPUTimes.User) {
					t.Errorf("CPUTimes.User: got %f, want %f", m.CPUTimes.User, tt.expected.CPUTimes.User)
				}
				if !almostEqualFloat64(m.CPUTimes.System, tt.expected.CPUTimes.System) {
					t.Errorf("CPUTimes.System: got %f, want %f", m.CPUTimes.System, tt.expected.CPUTimes.System)
				}
				if m.CPUTimes.Count != tt.expected.CPUTimes.Count {
					t.Errorf("CPUTimes.Count: got %d, want %d", m.CPUTimes.Count, tt.expected.CPUTimes.Count)
				}
			}

			// Verify TotalRunningSecs is calculated (approximately)
			if tt.procInfo.CreateTime > 0 && m.TotalRunningSecs <= 0 {
				t.Errorf("TotalRunningSecs should be > 0 when CreateTime is set, got %f", m.TotalRunningSecs)
			}

			// Verify CollectedAt is set
			if m.CollectedAt.IsZero() {
				t.Errorf("CollectedAt should be set")
			}
		})
	}
}

func TestProcessMetricsMerge(t *testing.T) {
	tests := []struct {
		name     string
		m        ProcessMetrics
		other    ProcessMetrics
		expected ProcessMetrics
	}{
		{
			name: "Merge into empty metrics",
			m:    ProcessMetrics{},
			other: ProcessMetrics{
				CollectedAt:         time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
				Nodes:               2,
				TotalCPUPercent:     20.0,
				TotalNumConnections: 15,
				TotalRunningSecs:    3600.0,
				TotalNumFDs:         100,
				TotalNumThreads:     8,
				TotalNice:           5,
				Count:               3,
				BackgroundProcesses: 1,
				RunningProcesses:    2,
				MemInfo: ProcessMemoryInfo{
					RSS:   2048 * 1024,
					VMS:   4096 * 1024,
					Count: 2,
				},
				IOCounters: ProcessIOCounters{
					ReadCount:  200,
					WriteCount: 300,
					ReadBytes:  1024 * 200,
					WriteBytes: 1024 * 300,
					Count:      2,
				},
				NumCtxSwitches: ProcessCtxSwitches{
					Voluntary:   1000,
					Involuntary: 200,
					Count:       2,
				},
				PageFaults: ProcessPageFaults{
					MinorFaults:      2000,
					MajorFaults:      20,
					ChildMinorFaults: 100,
					ChildMajorFaults: 10,
					Count:            2,
				},
				CPUTimes: ProcessCPUTimes{
					User:      20.5,
					System:    10.2,
					Idle:      5.0,
					Nice:      2.0,
					Count:     2,
				},
				MemMaps: ProcessMemoryMaps{
					TotalSize: 1024 * 1024,
					TotalRSS:  512 * 1024,
					Count:     10,
				},
			},
			expected: ProcessMetrics{
				CollectedAt:         time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
				Nodes:               2,
				TotalCPUPercent:     20.0,
				TotalNumConnections: 15,
				TotalRunningSecs:    3600.0,
				TotalNumFDs:         100,
				TotalNumThreads:     8,
				TotalNice:           5,
				Count:               3,
				BackgroundProcesses: 1,
				RunningProcesses:    2,
				MemInfo: ProcessMemoryInfo{
					RSS:   2048 * 1024,
					VMS:   4096 * 1024,
					Count: 2,
				},
				IOCounters: ProcessIOCounters{
					ReadCount:  200,
					WriteCount: 300,
					ReadBytes:  1024 * 200,
					WriteBytes: 1024 * 300,
					Count:      2,
				},
				NumCtxSwitches: ProcessCtxSwitches{
					Voluntary:   1000,
					Involuntary: 200,
					Count:       2,
				},
				PageFaults: ProcessPageFaults{
					MinorFaults:      2000,
					MajorFaults:      20,
					ChildMinorFaults: 100,
					ChildMajorFaults: 10,
					Count:            2,
				},
				CPUTimes: ProcessCPUTimes{
					User:      20.5,
					System:    10.2,
					Idle:      5.0,
					Nice:      2.0,
					Count:     2,
				},
				MemMaps: ProcessMemoryMaps{
					TotalSize: 1024 * 1024,
					TotalRSS:  512 * 1024,
					Count:     10,
				},
			},
		},
		{
			name: "Merge with existing metrics",
			m: ProcessMetrics{
				CollectedAt:         time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC), // Earlier time
				Nodes:               1,
				TotalCPUPercent:     15.0,
				TotalNumConnections: 10,
				TotalRunningSecs:    1800.0,
				TotalNumFDs:         50,
				TotalNumThreads:     4,
				TotalNice:           0,
				Count:               2,
				BackgroundProcesses: 0,
				RunningProcesses:    2,
				MemInfo: ProcessMemoryInfo{
					RSS:   1024 * 1024,
					VMS:   2048 * 1024,
					Count: 1,
				},
				IOCounters: ProcessIOCounters{
					ReadCount:  100,
					WriteCount: 150,
					ReadBytes:  1024 * 100,
					WriteBytes: 1024 * 150,
					Count:      1,
				},
				CPUTimes: ProcessCPUTimes{
					User:   10.0,
					System: 5.0,
					Count:  1,
				},
			},
			other: ProcessMetrics{
				CollectedAt:         time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC), // Later time
				Nodes:               3,
				TotalCPUPercent:     25.0,
				TotalNumConnections: 20,
				TotalRunningSecs:    5400.0,
				TotalNumFDs:         75,
				TotalNumThreads:     6,
				TotalNice:           -5,
				Count:               4,
				BackgroundProcesses: 2,
				RunningProcesses:    2,
				MemInfo: ProcessMemoryInfo{
					RSS:   1536 * 1024,
					VMS:   3072 * 1024,
					Count: 2,
				},
				IOCounters: ProcessIOCounters{
					ReadCount:  150,
					WriteCount: 200,
					ReadBytes:  1024 * 150,
					WriteBytes: 1024 * 200,
					Count:      2,
				},
				CPUTimes: ProcessCPUTimes{
					User:   15.0,
					System: 8.0,
					Count:  2,
				},
			},
			expected: ProcessMetrics{
				CollectedAt:         time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC), // Later time
				Nodes:               4, // 1 + 3
				TotalCPUPercent:     40.0,
				TotalNumConnections: 30,
				TotalRunningSecs:    7200.0,
				TotalNumFDs:         125,
				TotalNumThreads:     10,
				TotalNice:           -5,
				Count:               6,
				BackgroundProcesses: 2,
				RunningProcesses:    4,
				MemInfo: ProcessMemoryInfo{
					RSS:   2560 * 1024, // 1024 + 1536
					VMS:   5120 * 1024, // 2048 + 3072
					Count: 3,
				},
				IOCounters: ProcessIOCounters{
					ReadCount:  250,
					WriteCount: 350,
					ReadBytes:  1024 * 250,
					WriteBytes: 1024 * 350,
					Count:      3,
				},
				CPUTimes: ProcessCPUTimes{
					User:   25.0,
					System: 13.0,
					Count:  3,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.m
			m.Merge(&tt.other)

			// Check CollectedAt
			if !m.CollectedAt.Equal(tt.expected.CollectedAt) {
				t.Errorf("CollectedAt: got %v, want %v", m.CollectedAt, tt.expected.CollectedAt)
			}

			// Check basic fields
			if m.Nodes != tt.expected.Nodes {
				t.Errorf("Nodes: got %d, want %d", m.Nodes, tt.expected.Nodes)
			}
			if !almostEqualFloat64(m.TotalCPUPercent, tt.expected.TotalCPUPercent) {
				t.Errorf("TotalCPUPercent: got %f, want %f", m.TotalCPUPercent, tt.expected.TotalCPUPercent)
			}
			if m.TotalNumConnections != tt.expected.TotalNumConnections {
				t.Errorf("TotalNumConnections: got %d, want %d", m.TotalNumConnections, tt.expected.TotalNumConnections)
			}
			if !almostEqualFloat64(m.TotalRunningSecs, tt.expected.TotalRunningSecs) {
				t.Errorf("TotalRunningSecs: got %f, want %f", m.TotalRunningSecs, tt.expected.TotalRunningSecs)
			}
			if m.TotalNumFDs != tt.expected.TotalNumFDs {
				t.Errorf("TotalNumFDs: got %d, want %d", m.TotalNumFDs, tt.expected.TotalNumFDs)
			}
			if m.TotalNumThreads != tt.expected.TotalNumThreads {
				t.Errorf("TotalNumThreads: got %d, want %d", m.TotalNumThreads, tt.expected.TotalNumThreads)
			}
			if m.TotalNice != tt.expected.TotalNice {
				t.Errorf("TotalNice: got %d, want %d", m.TotalNice, tt.expected.TotalNice)
			}
			if m.Count != tt.expected.Count {
				t.Errorf("Count: got %d, want %d", m.Count, tt.expected.Count)
			}
			if m.BackgroundProcesses != tt.expected.BackgroundProcesses {
				t.Errorf("BackgroundProcesses: got %d, want %d", m.BackgroundProcesses, tt.expected.BackgroundProcesses)
			}
			if m.RunningProcesses != tt.expected.RunningProcesses {
				t.Errorf("RunningProcesses: got %d, want %d", m.RunningProcesses, tt.expected.RunningProcesses)
			}

			// Check memory info
			if m.MemInfo.RSS != tt.expected.MemInfo.RSS {
				t.Errorf("MemInfo.RSS: got %d, want %d", m.MemInfo.RSS, tt.expected.MemInfo.RSS)
			}
			if m.MemInfo.VMS != tt.expected.MemInfo.VMS {
				t.Errorf("MemInfo.VMS: got %d, want %d", m.MemInfo.VMS, tt.expected.MemInfo.VMS)
			}
			if m.MemInfo.Count != tt.expected.MemInfo.Count {
				t.Errorf("MemInfo.Count: got %d, want %d", m.MemInfo.Count, tt.expected.MemInfo.Count)
			}

			// Check IO counters
			if m.IOCounters.ReadCount != tt.expected.IOCounters.ReadCount {
				t.Errorf("IOCounters.ReadCount: got %d, want %d", m.IOCounters.ReadCount, tt.expected.IOCounters.ReadCount)
			}
			if m.IOCounters.WriteCount != tt.expected.IOCounters.WriteCount {
				t.Errorf("IOCounters.WriteCount: got %d, want %d", m.IOCounters.WriteCount, tt.expected.IOCounters.WriteCount)
			}
			if m.IOCounters.ReadBytes != tt.expected.IOCounters.ReadBytes {
				t.Errorf("IOCounters.ReadBytes: got %d, want %d", m.IOCounters.ReadBytes, tt.expected.IOCounters.ReadBytes)
			}
			if m.IOCounters.WriteBytes != tt.expected.IOCounters.WriteBytes {
				t.Errorf("IOCounters.WriteBytes: got %d, want %d", m.IOCounters.WriteBytes, tt.expected.IOCounters.WriteBytes)
			}
			if m.IOCounters.Count != tt.expected.IOCounters.Count {
				t.Errorf("IOCounters.Count: got %d, want %d", m.IOCounters.Count, tt.expected.IOCounters.Count)
			}

			// Check CPU times
			if !almostEqualFloat64(m.CPUTimes.User, tt.expected.CPUTimes.User) {
				t.Errorf("CPUTimes.User: got %f, want %f", m.CPUTimes.User, tt.expected.CPUTimes.User)
			}
			if !almostEqualFloat64(m.CPUTimes.System, tt.expected.CPUTimes.System) {
				t.Errorf("CPUTimes.System: got %f, want %f", m.CPUTimes.System, tt.expected.CPUTimes.System)
			}
			if m.CPUTimes.Count != tt.expected.CPUTimes.Count {
				t.Errorf("CPUTimes.Count: got %d, want %d", m.CPUTimes.Count, tt.expected.CPUTimes.Count)
			}
		})
	}
}

func TestProcessMetricsMergeNil(t *testing.T) {
	m := ProcessMetrics{
		Nodes:               1,
		TotalCPUPercent:     10.0,
		TotalNumConnections: 5,
		Count:               1,
	}

	// Test merging with nil
	m.Merge(nil)

	if m.Nodes != 1 || m.TotalCPUPercent != 10.0 || m.Count != 1 {
		t.Errorf("ProcessMetrics should remain unchanged after merging with nil")
	}
}

func TestAddProcInfoNil(t *testing.T) {
	// Test nil ProcInfo
	var p *ProcInfo
	m := ProcessMetrics{}
	p.AddProcInfo(&m)
	if m.Count != 0 {
		t.Errorf("ProcessMetrics should remain empty when adding nil ProcInfo")
	}

	// Test nil metrics
	p2 := ProcInfo{
		CPUPercent:     5.0,
		NumConnections: 2,
	}
	p2.AddProcInfo(nil)
	// Should not panic
}

func TestCreateTimeConversion(t *testing.T) {
	tests := []struct {
		name       string
		createTime int64
		expectSecs bool
	}{
		{
			name:       "Valid CreateTime",
			createTime: 1640995200000, // 2022-01-01 00:00:00 in milliseconds
			expectSecs: true,
		},
		{
			name:       "Zero CreateTime",
			createTime: 0,
			expectSecs: false,
		},
		{
			name:       "Negative CreateTime",
			createTime: -1,
			expectSecs: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ProcInfo{
				CreateTime: tt.createTime,
			}
			m := ProcessMetrics{}
			p.AddProcInfo(&m)

			if tt.expectSecs {
				if m.TotalRunningSecs <= 0 {
					t.Errorf("Expected TotalRunningSecs > 0, got %f", m.TotalRunningSecs)
				}
			} else {
				if m.TotalRunningSecs != 0 {
					t.Errorf("Expected TotalRunningSecs = 0, got %f", m.TotalRunningSecs)
				}
			}
		})
	}
}

// Helper function for comparing floats with tolerance
func almostEqualFloat64(a, b float64) bool {
	const epsilon = 1e-9
	return math.Abs(a-b) < epsilon
}