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
	"context"
	"encoding/json"
	"runtime"
	"testing"
)

// TestCPUFreqStatsJSONMarshal tests that only Name and Governor are included in JSON output when set
func TestCPUFreqStatsJSONMarshal(t *testing.T) {
	tests := []struct {
		name     string
		input    CPUFreqStats
		expected string
	}{
		{
			name: "Only Name and Governor set",
			input: CPUFreqStats{
				Name:     "cpu0",
				Governor: "performance",
			},
			expected: `{"Name":"cpu0","Governor":"performance"}`,
		},
		{
			name: "Only Name set",
			input: CPUFreqStats{
				Name: "cpu0",
			},
			expected: `{"Name":"cpu0"}`,
		},
		{
			name: "Only Governor set",
			input: CPUFreqStats{
				Governor: "powersave",
			},
			expected: `{"Governor":"powersave"}`,
		},
		{
			name:     "Empty struct",
			input:    CPUFreqStats{},
			expected: `{}`,
		},
		{
			name: "All fields set",
			input: CPUFreqStats{
				Name:                     "cpu0",
				Governor:                 "performance",
				CpuinfoCurrentFrequency:  ptr(uint64(2000000)),
				CpuinfoMinimumFrequency:  ptr(uint64(800000)),
				CpuinfoMaximumFrequency:  ptr(uint64(3000000)),
				CpuinfoTransitionLatency: ptr(uint64(1000)),
				ScalingCurrentFrequency:  ptr(uint64(2000000)),
				ScalingMinimumFrequency:  ptr(uint64(800000)),
				ScalingMaximumFrequency:  ptr(uint64(3000000)),
				AvailableGovernors:       "performance powersave",
				Driver:                   "intel_pstate",
				RelatedCpus:              "0-3",
				SetSpeed:                 "2000000",
			},
			expected: `{"Name":"cpu0","Governor":"performance","CpuinfoCurrentFrequency":2000000,"CpuinfoMinimumFrequency":800000,"CpuinfoMaximumFrequency":3000000,"CpuinfoTransitionLatency":1000,"ScalingCurrentFrequency":2000000,"ScalingMinimumFrequency":800000,"ScalingMaximumFrequency":3000000,"AvailableGovernors":"performance powersave","Driver":"intel_pstate","RelatedCpus":"0-3","SetSpeed":"2000000"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Failed to marshal JSON: %v", err)
			}

			if string(output) != tt.expected {
				t.Errorf("Expected JSON: %s, got: %s", tt.expected, string(output))
			}

			// Unmarshal back to verify structure
			var result CPUFreqStats
			if err := json.Unmarshal(output, &result); err != nil {
				t.Fatalf("Failed to unmarshal JSON: %v", err)
			}

			// Compare non-omitted fields
			if tt.input.Name != result.Name {
				t.Errorf("Expected Name: %s, got: %s", tt.input.Name, result.Name)
			}
			if tt.input.Governor != result.Governor {
				t.Errorf("Expected Governor: %s, got: %s", tt.input.Governor, result.Governor)
			}

			// Verify that unset fields remain unset after unmarshaling
			if tt.input.CpuinfoCurrentFrequency == nil && result.CpuinfoCurrentFrequency != nil {
				t.Errorf("Expected CpuinfoCurrentFrequency to be nil, got: %v", result.CpuinfoCurrentFrequency)
			}
			if tt.input.CpuinfoMinimumFrequency == nil && result.CpuinfoMinimumFrequency != nil {
				t.Errorf("Expected CpuinfoMinimumFrequency to be nil, got: %v", result.CpuinfoMinimumFrequency)
			}
			if tt.input.CpuinfoMaximumFrequency == nil && result.CpuinfoMaximumFrequency != nil {
				t.Errorf("Expected CpuinfoMaximumFrequency to be nil, got: %v", result.CpuinfoMaximumFrequency)
			}
			if tt.input.CpuinfoTransitionLatency == nil && result.CpuinfoTransitionLatency != nil {
				t.Errorf("Expected CpuinfoTransitionLatency to be nil, got: %v", result.CpuinfoTransitionLatency)
			}
			if tt.input.ScalingCurrentFrequency == nil && result.ScalingCurrentFrequency != nil {
				t.Errorf("Expected ScalingCurrentFrequency to be nil, got: %v", result.ScalingCurrentFrequency)
			}
			if tt.input.ScalingMinimumFrequency == nil && result.ScalingMinimumFrequency != nil {
				t.Errorf("Expected ScalingMinimumFrequency to be nil, got: %v", result.ScalingMinimumFrequency)
			}
			if tt.input.ScalingMaximumFrequency == nil && result.ScalingMaximumFrequency != nil {
				t.Errorf("Expected ScalingMaximumFrequency to be nil, got: %v", result.ScalingMaximumFrequency)
			}
			if tt.input.AvailableGovernors == "" && result.AvailableGovernors != "" {
				t.Errorf("Expected AvailableGovernors to be empty, got: %s", result.AvailableGovernors)
			}
			if tt.input.Driver == "" && result.Driver != "" {
				t.Errorf("Expected Driver to be empty, got: %s", result.Driver)
			}
			if tt.input.RelatedCpus == "" && result.RelatedCpus != "" {
				t.Errorf("Expected RelatedCpus to be empty, got: %s", result.RelatedCpus)
			}
			if tt.input.SetSpeed == "" && result.SetSpeed != "" {
				t.Errorf("Expected SetSpeed to be empty, got: %s", result.SetSpeed)
			}
		})
	}
}

// ptr is a helper function to create a pointer to a uint64
func ptr(val uint64) *uint64 {
	return &val
}

// TestCPUMultithreadingDetection tests actual CPU detection on the running system
func TestCPUMultithreadingDetection(t *testing.T) {
	cpusInfo := GetCPUs(context.TODO(), "test-addr")

	if len(cpusInfo.CPUs) == 0 {
		t.Fatal("Expected at least one CPU entry")
	}

	for i, cpu := range cpusInfo.CPUs {
		t.Logf("CPU %d:", i)
		t.Logf("  ModelName: %s", cpu.ModelName)
		t.Logf("  Cores: %d", cpu.Cores)
		if cpu.MultithreadCapable != nil {
			t.Logf("  MultithreadCapable: %v", *cpu.MultithreadCapable)
		} else {
			t.Logf("  MultithreadCapable: <not available>")
		}
		if cpu.MultithreadEnabled != nil {
			t.Logf("  MultithreadEnabled: %v", *cpu.MultithreadEnabled)
		} else {
			t.Logf("  MultithreadEnabled: <not available>")
		}

		if cpu.Cores == 0 {
			t.Errorf("CPU %d: Expected Cores to be greater than 0", i)
		}

		if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
			if cpu.MultithreadCapable == nil {
				t.Errorf("CPU %d: MultithreadCapable should be set on Linux/Darwin platforms", i)
			}
			if cpu.MultithreadEnabled == nil {
				t.Errorf("CPU %d: MultithreadEnabled should be set on Linux/Darwin platforms", i)
			}
			if cpu.MultithreadEnabled != nil && cpu.MultithreadCapable != nil {
				if *cpu.MultithreadEnabled && !*cpu.MultithreadCapable {
					t.Errorf("CPU %d: MultithreadEnabled is true but MultithreadCapable is false (impossible)", i)
				}
			}
		} else {
			if cpu.MultithreadCapable != nil {
				t.Errorf("CPU %d: MultithreadCapable should be nil on unsupported platforms", i)
			}
			if cpu.MultithreadEnabled != nil {
				t.Errorf("CPU %d: MultithreadEnabled should be nil on unsupported platforms", i)
			}
		}
	}

	if len(cpusInfo.CPUs) > 1 {
		first := cpusInfo.CPUs[0]
		for i := 1; i < len(cpusInfo.CPUs); i++ {
			if (cpusInfo.CPUs[i].MultithreadCapable == nil) != (first.MultithreadCapable == nil) {
				t.Errorf("CPU %d has different MultithreadCapable presence than CPU 0", i)
			}
			if cpusInfo.CPUs[i].MultithreadCapable != nil && first.MultithreadCapable != nil {
				if *cpusInfo.CPUs[i].MultithreadCapable != *first.MultithreadCapable {
					t.Errorf("CPU %d has different MultithreadCapable value than CPU 0", i)
				}
			}
			if (cpusInfo.CPUs[i].MultithreadEnabled == nil) != (first.MultithreadEnabled == nil) {
				t.Errorf("CPU %d has different MultithreadEnabled presence than CPU 0", i)
			}
			if cpusInfo.CPUs[i].MultithreadEnabled != nil && first.MultithreadEnabled != nil {
				if *cpusInfo.CPUs[i].MultithreadEnabled != *first.MultithreadEnabled {
					t.Errorf("CPU %d has different MultithreadEnabled value than CPU 0", i)
				}
			}
		}
	}
}
