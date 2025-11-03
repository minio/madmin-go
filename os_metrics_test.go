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

	"github.com/shirou/gopsutil/v4/sensors"
)

func TestAddOSInfo(t *testing.T) {
	tests := []struct {
		name     string
		osInfo   OSInfo
		initial  OSMetrics
		expected OSMetrics
	}{
		{
			name: "Add sensors to empty metrics",
			osInfo: OSInfo{
				Sensors: []sensors.TemperatureStat{
					{
						SensorKey:   "coretemp-isa-0000",
						Temperature: 45.0,
						Critical:    85.0,
					},
					{
						SensorKey:   "acpitz-acpi-0",
						Temperature: 50.0,
						Critical:    90.0,
					},
				},
			},
			initial: OSMetrics{},
			expected: OSMetrics{
				Sensors: map[string]SensorMetrics{
					"coretemp-isa-0000": {
						MinTemp:         45.0,
						MaxTemp:         45.0,
						TotalTemp:       45.0,
						Count:           1,
						ExceedsCritical: 0,
					},
					"acpitz-acpi-0": {
						MinTemp:         50.0,
						MaxTemp:         50.0,
						TotalTemp:       50.0,
						Count:           1,
						ExceedsCritical: 0,
					},
				},
			},
		},
		{
			name: "Add sensors with temperature exceeding critical",
			osInfo: OSInfo{
				Sensors: []sensors.TemperatureStat{
					{
						SensorKey:   "coretemp-isa-0000",
						Temperature: 95.0,
						Critical:    85.0,
					},
					{
						SensorKey:   "acpitz-acpi-0",
						Temperature: 80.0,
						Critical:    90.0,
					},
				},
			},
			initial: OSMetrics{},
			expected: OSMetrics{
				Sensors: map[string]SensorMetrics{
					"coretemp-isa-0000": {
						MinTemp:         95.0,
						MaxTemp:         95.0,
						TotalTemp:       95.0,
						Count:           1,
						ExceedsCritical: 1, // Exceeds 85.0
					},
					"acpitz-acpi-0": {
						MinTemp:         80.0,
						MaxTemp:         80.0,
						TotalTemp:       80.0,
						Count:           1,
						ExceedsCritical: 0, // Does not exceed 90.0
					},
				},
			},
		},
		{
			name: "Update existing sensor metrics with new min/max",
			osInfo: OSInfo{
				Sensors: []sensors.TemperatureStat{
					{
						SensorKey:   "coretemp-isa-0000",
						Temperature: 30.0, // Lower than existing
						Critical:    85.0,
					},
					{
						SensorKey:   "acpitz-acpi-0",
						Temperature: 75.0, // Higher than existing
						Critical:    90.0,
					},
				},
			},
			initial: OSMetrics{
				Sensors: map[string]SensorMetrics{
					"coretemp-isa-0000": {
						MinTemp:   45.0,
						MaxTemp:   60.0,
						TotalTemp: 105.0,
						Count:     2,
					},
					"acpitz-acpi-0": {
						MinTemp:   40.0,
						MaxTemp:   70.0,
						TotalTemp: 110.0,
						Count:     2,
					},
				},
			},
			expected: OSMetrics{
				Sensors: map[string]SensorMetrics{
					"coretemp-isa-0000": {
						MinTemp:         30.0, // Updated to lower value
						MaxTemp:         60.0, // Unchanged
						TotalTemp:       135.0,
						Count:           3,
						ExceedsCritical: 0,
					},
					"acpitz-acpi-0": {
						MinTemp:         40.0, // Unchanged
						MaxTemp:         75.0, // Updated to higher value
						TotalTemp:       185.0,
						Count:           3,
						ExceedsCritical: 0,
					},
				},
			},
		},
		{
			name: "Handle sensors with zero critical threshold",
			osInfo: OSInfo{
				Sensors: []sensors.TemperatureStat{
					{
						SensorKey:   "nvme-pci-0100",
						Temperature: 100.0,
						Critical:    0, // No critical threshold
					},
				},
			},
			initial: OSMetrics{},
			expected: OSMetrics{
				Sensors: map[string]SensorMetrics{
					"nvme-pci-0100": {
						MinTemp:         100.0,
						MaxTemp:         100.0,
						TotalTemp:       100.0,
						Count:           1,
						ExceedsCritical: 0, // Not counted when Critical is 0
					},
				},
			},
		},
		{
			name: "Add multiple readings of same sensor",
			osInfo: OSInfo{
				Sensors: []sensors.TemperatureStat{
					{
						SensorKey:   "cpu-temp",
						Temperature: 45.0,
						Critical:    85.0,
					},
					{
						SensorKey:   "cpu-temp",
						Temperature: 55.0,
						Critical:    85.0,
					},
					{
						SensorKey:   "cpu-temp",
						Temperature: 50.0,
						Critical:    85.0,
					},
				},
			},
			initial: OSMetrics{},
			expected: OSMetrics{
				Sensors: map[string]SensorMetrics{
					"cpu-temp": {
						MinTemp:         45.0,
						MaxTemp:         55.0,
						TotalTemp:       150.0, // 45 + 55 + 50
						Count:           3,
						ExceedsCritical: 0,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.initial
			tt.osInfo.AddOSInfo(&m)

			// Check sensor metrics
			if len(m.Sensors) != len(tt.expected.Sensors) {
				t.Errorf("Sensors length mismatch: got %d, want %d", len(m.Sensors), len(tt.expected.Sensors))
			}

			for key, expected := range tt.expected.Sensors {
				actual, ok := m.Sensors[key]
				if !ok {
					t.Errorf("Sensor key %s not found", key)
					continue
				}

				if !almostEqualFloat(actual.MinTemp, expected.MinTemp) {
					t.Errorf("Sensor %s MinTemp: got %f, want %f", key, actual.MinTemp, expected.MinTemp)
				}
				if !almostEqualFloat(actual.MaxTemp, expected.MaxTemp) {
					t.Errorf("Sensor %s MaxTemp: got %f, want %f", key, actual.MaxTemp, expected.MaxTemp)
				}
				if !almostEqualFloat(actual.TotalTemp, expected.TotalTemp) {
					t.Errorf("Sensor %s TotalTemp: got %f, want %f", key, actual.TotalTemp, expected.TotalTemp)
				}
				if actual.Count != expected.Count {
					t.Errorf("Sensor %s Count: got %d, want %d", key, actual.Count, expected.Count)
				}
				if actual.ExceedsCritical != expected.ExceedsCritical {
					t.Errorf("Sensor %s ExceedsCritical: got %d, want %d", key, actual.ExceedsCritical, expected.ExceedsCritical)
				}
			}
		})
	}
}

func TestOSMetricsMergeSensors(t *testing.T) {
	tests := []struct {
		name     string
		m        OSMetrics
		other    OSMetrics
		expected OSMetrics
	}{
		{
			name: "Merge sensors into empty metrics",
			m:    OSMetrics{},
			other: OSMetrics{
				Sensors: map[string]SensorMetrics{
					"sensor1": {
						MinTemp:         40.0,
						MaxTemp:         60.0,
						TotalTemp:       100.0,
						Count:           2,
						ExceedsCritical: 0,
					},
				},
			},
			expected: OSMetrics{
				Sensors: map[string]SensorMetrics{
					"sensor1": {
						MinTemp:         40.0,
						MaxTemp:         60.0,
						TotalTemp:       100.0,
						Count:           2,
						ExceedsCritical: 0,
					},
				},
			},
		},
		{
			name: "Merge sensors with different keys",
			m: OSMetrics{
				Sensors: map[string]SensorMetrics{
					"sensor1": {
						MinTemp:   40.0,
						MaxTemp:   60.0,
						TotalTemp: 100.0,
						Count:     2,
					},
				},
			},
			other: OSMetrics{
				Sensors: map[string]SensorMetrics{
					"sensor2": {
						MinTemp:   35.0,
						MaxTemp:   55.0,
						TotalTemp: 90.0,
						Count:     2,
					},
				},
			},
			expected: OSMetrics{
				Sensors: map[string]SensorMetrics{
					"sensor1": {
						MinTemp:   40.0,
						MaxTemp:   60.0,
						TotalTemp: 100.0,
						Count:     2,
					},
					"sensor2": {
						MinTemp:   35.0,
						MaxTemp:   55.0,
						TotalTemp: 90.0,
						Count:     2,
					},
				},
			},
		},
		{
			name: "Merge sensors with same keys - update min/max",
			m: OSMetrics{
				Sensors: map[string]SensorMetrics{
					"cpu-temp": {
						MinTemp:         45.0,
						MaxTemp:         65.0,
						TotalTemp:       220.0,
						Count:           4,
						ExceedsCritical: 1,
					},
				},
			},
			other: OSMetrics{
				Sensors: map[string]SensorMetrics{
					"cpu-temp": {
						MinTemp:         40.0, // Lower min
						MaxTemp:         70.0, // Higher max
						TotalTemp:       150.0,
						Count:           3,
						ExceedsCritical: 2,
					},
				},
			},
			expected: OSMetrics{
				Sensors: map[string]SensorMetrics{
					"cpu-temp": {
						MinTemp:         40.0,  // Takes the lower min
						MaxTemp:         70.0,  // Takes the higher max
						TotalTemp:       370.0, // 220 + 150
						Count:           7,     // 4 + 3
						ExceedsCritical: 3,     // 1 + 2
					},
				},
			},
		},
		{
			name: "Merge with CollectedAt update",
			m: OSMetrics{
				CollectedAt: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
				Sensors: map[string]SensorMetrics{
					"sensor1": {
						MinTemp: 50.0,
						MaxTemp: 50.0,
						Count:   1,
					},
				},
			},
			other: OSMetrics{
				CollectedAt: time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC), // Later time
				Sensors: map[string]SensorMetrics{
					"sensor1": {
						MinTemp: 55.0,
						MaxTemp: 55.0,
						Count:   1,
					},
				},
			},
			expected: OSMetrics{
				CollectedAt: time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC), // Updated to later time
				Sensors: map[string]SensorMetrics{
					"sensor1": {
						MinTemp: 50.0, // Keeps lower min
						MaxTemp: 55.0, // Takes higher max
						Count:   2,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.m
			m.Merge(&tt.other)

			// Check CollectedAt if set
			if !tt.expected.CollectedAt.IsZero() {
				if !m.CollectedAt.Equal(tt.expected.CollectedAt) {
					t.Errorf("CollectedAt: got %v, want %v", m.CollectedAt, tt.expected.CollectedAt)
				}
			}

			// Check sensor metrics
			if len(m.Sensors) != len(tt.expected.Sensors) {
				t.Errorf("Sensors length mismatch: got %d, want %d", len(m.Sensors), len(tt.expected.Sensors))
			}

			for key, expected := range tt.expected.Sensors {
				actual, ok := m.Sensors[key]
				if !ok {
					t.Errorf("Sensor key %s not found", key)
					continue
				}

				if !almostEqualFloat(actual.MinTemp, expected.MinTemp) {
					t.Errorf("Sensor %s MinTemp: got %f, want %f", key, actual.MinTemp, expected.MinTemp)
				}
				if !almostEqualFloat(actual.MaxTemp, expected.MaxTemp) {
					t.Errorf("Sensor %s MaxTemp: got %f, want %f", key, actual.MaxTemp, expected.MaxTemp)
				}
				if !almostEqualFloat(actual.TotalTemp, expected.TotalTemp) {
					t.Errorf("Sensor %s TotalTemp: got %f, want %f", key, actual.TotalTemp, expected.TotalTemp)
				}
				if actual.Count != expected.Count {
					t.Errorf("Sensor %s Count: got %d, want %d", key, actual.Count, expected.Count)
				}
				if actual.ExceedsCritical != expected.ExceedsCritical {
					t.Errorf("Sensor %s ExceedsCritical: got %d, want %d", key, actual.ExceedsCritical, expected.ExceedsCritical)
				}
			}
		})
	}
}

func TestOSMetricsMergeNil(t *testing.T) {
	m := OSMetrics{
		Sensors: map[string]SensorMetrics{
			"sensor1": {
				MinTemp: 50.0,
				MaxTemp: 60.0,
				Count:   2,
			},
		},
	}

	// Test merging with nil
	m.Merge(nil)

	if len(m.Sensors) != 1 {
		t.Errorf("Sensors should remain unchanged after merging with nil: got %d sensors", len(m.Sensors))
	}
}

func TestAddOSInfoNil(t *testing.T) {
	// Test nil OSInfo
	var o *OSInfo
	m := OSMetrics{}
	o.AddOSInfo(&m)
	if len(m.Sensors) != 0 {
		t.Errorf("Sensors should remain empty when adding nil OSInfo")
	}

	// Test nil metrics
	o2 := OSInfo{
		Sensors: []sensors.TemperatureStat{{SensorKey: "test", Temperature: 50.0}},
	}
	o2.AddOSInfo(nil)
	// Should not panic
}

// Helper function for comparing floats with tolerance
func almostEqualFloat(a, b float64) bool {
	const epsilon = 1e-9
	return math.Abs(a-b) < epsilon
}
