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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"testing"
	"time"
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

// TestProductInfoJSONMarshal tests that empty DMI fields are dropped from
// the JSON output via the omitempty tags.
func TestProductInfoJSONMarshal(t *testing.T) {
	tests := []struct {
		name     string
		input    ProductInfo
		expected string
	}{
		{
			name: "Only addr set (no DMI — macOS / non-root / container)",
			input: ProductInfo{
				NodeCommon: NodeCommon{Addr: "node-1:9000"},
			},
			expected: `{"addr":"node-1:9000"}`,
		},
		{
			name: "Fully populated (Linux + root)",
			input: ProductInfo{
				NodeCommon:   NodeCommon{Addr: "node-1:9000"},
				Family:       "PowerEdge",
				Name:         "PowerEdge R750",
				Vendor:       "Dell Inc.",
				SerialNumber: "5XYZ123",
				UUID:         "4c4c4544-0058-5910-8050-cac04f445232",
				SKU:          "SKU=PE-R750",
				Version:      "2.10.0",
			},
			expected: `{"addr":"node-1:9000","family":"PowerEdge","name":"PowerEdge R750","vendor":"Dell Inc.","serial_number":"5XYZ123","uuid":"4c4c4544-0058-5910-8050-cac04f445232","sku":"SKU=PE-R750","version":"2.10.0"}`,
		},
		{
			name: "Partial (non-root Linux: serial/uuid restricted)",
			input: ProductInfo{
				NodeCommon: NodeCommon{Addr: "node-1:9000"},
				Family:     "PowerEdge",
				Name:       "PowerEdge R750",
				Vendor:     "Dell Inc.",
				SKU:        "SKU=PE-R750",
				Version:    "2.10.0",
			},
			expected: `{"addr":"node-1:9000","family":"PowerEdge","name":"PowerEdge R750","vendor":"Dell Inc.","sku":"SKU=PE-R750","version":"2.10.0"}`,
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
		})
	}
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

// TestServerHealthInfoPreservesBufferedFrames verifies the version probe does
// not swallow the frames behind it. The probe's decoder reads ahead, so when
// the server emits several frames into one flush the bytes after the version
// frame land in that decoder's buffer. Callers decode the rest of the stream
// from resp.Body with a decoder of their own, and would otherwise pick up a
// frame whose prefix is already gone.
func TestServerHealthInfoPreservesBufferedFrames(t *testing.T) {
	frame := func(ts time.Time) []byte {
		b, err := json.Marshal(HealthInfo{Version: HealthInfoVersion, TimeStamp: ts})
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		return b
	}
	first, second := time.Unix(500, 0).UTC(), time.Unix(1000, 0).UTC()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Both frames in a single write, so they reach the client in one read.
		_, _ = w.Write(append(frame(first), frame(second)...))
	}))
	defer server.Close()

	client, err := New(mustParseHost(t, server.URL), "ak", "sk", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, version, err := client.ServerHealthInfo(context.Background(),
		[]HealthDataType{HealthDataTypeMinioInfo}, time.Second, "")
	if err != nil {
		t.Fatalf("ServerHealthInfo: %v", err)
	}
	defer resp.Body.Close()

	if version != HealthInfoVersion {
		t.Errorf("version = %q, want %q", version, HealthInfoVersion)
	}

	var got HealthInfo
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decoding the frame after the version probe: %v", err)
	}
	if !got.TimeStamp.Equal(second) {
		t.Errorf("TimeStamp = %v, want the frame after the probe (%v)", got.TimeStamp, second)
	}
}

func TestIAMPolicyDiagJSON(t *testing.T) {
	testCases := []struct {
		name string
		diag IAMPolicyDiag
		want string
	}{
		{name: "conflict", diag: IAMPolicyDiag{SystemNameConflict: true}, want: `{"system_name_conflict":true}`},
		{name: "no-findings", diag: IAMPolicyDiag{}, want: `{}`},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf, err := json.Marshal(tc.diag)
			if err != nil {
				t.Fatal(err)
			}
			if string(buf) != tc.want {
				t.Fatalf("want %s, got %s", tc.want, buf)
			}
			var got IAMPolicyDiag
			if err := json.Unmarshal(buf, &got); err != nil {
				t.Fatal(err)
			}
			if got.SystemNameConflict != tc.diag.SystemNameConflict {
				t.Fatalf("round trip: want %v, got %v", tc.diag.SystemNameConflict, got.SystemNameConflict)
			}
		})
	}
}

func TestTablesHealthInfoJSON(t *testing.T) {
	want := HealthInfo{Version: HealthInfoVersion, Minio: MinioHealthInfo{Tables: &TablesHealthInfo{
		Warehouses: 2,
		Namespaces: 3,
		Tables:     5,
		Maintenance: map[string]TableMaintenanceNode{
			"compaction": {
				Runs:   TableMaintenanceRuns{InFlight: 1, Done: TimedAction{Count: 4, AccTime: 9, MinTime: 1, MaxTime: 5}},
				Groups: TableMaintenanceGroups{InFlight: 2},
				Memory: TableMaintenanceMemory{PeakArrowBytes: 1 << 20, NodePeakArrowBytes: 2 << 20},
				IO:     TableMaintenanceIO{BytesRead: 10, BytesWritten: 20, Ops: TableMaintenanceIOOps{Get: 3}, ThrottleSecs: 1.5},
			},
		},
		Work: map[string]map[string]uint64{"compaction": {"files_rewritten": 8, "commit_races_lost": 1}},
		Compaction: &TableCompactionClusterBacklog{
			Complete:             true,
			Components:           map[string]TableCompactionClusterBacklogComponent{"small_files": {Files: 12}, "delete_files": {Files: 4}},
			TablesOverdue:        2,
			OldestOverdueSecs:    7200,
			TablesOverdueBuckets: map[string]int64{"3600": 1, "+Inf": 2},
		},
		Config: &TablesMaintenanceConfig{
			Jobs:                 map[string]TablesMaintenanceJobConfig{"compaction": {Enabled: true, IntervalSecs: 3600}, "snapshot_expiration": {}},
			PollIntervalSecs:     300,
			ThrottleFactor:       2,
			ThrottleMaxSleepSecs: 1,
		},
	}}}
	buf, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var got HealthInfo
	if err := json.Unmarshal(buf, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Minio.Tables, want.Minio.Tables) {
		t.Errorf("round trip:\n got %+v\nwant %+v", got.Minio.Tables, want.Minio.Tables)
	}

	var old HealthInfo
	if err := json.Unmarshal([]byte(`{"version":"3","minio":{"iam_info":{"total_policies":1}}}`), &old); err != nil {
		t.Fatalf("payload without tables: %v", err)
	}
	if old.Minio.Tables != nil || old.Minio.IAMInfo == nil || old.Minio.IAMInfo.TotalPolicies != 1 {
		t.Errorf("payload without tables decoded to Tables=%+v IAMInfo=%+v", old.Minio.Tables, old.Minio.IAMInfo)
	}
}

func TestPartitionSMARTJSON(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		buf, err := json.Marshal(Partition{Device: "/dev/nvme0n1", Mountpoint: "/mnt/drive1"})
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(buf, []byte(`"smart"`)) {
			t.Fatalf("partition without SMART must not carry the key, got %s", buf)
		}
	})

	t.Run("older payload", func(t *testing.T) {
		var got Partition
		if err := json.Unmarshal([]byte(`{"device":"/dev/sdb","major":8,"minor":16,"mountpoint":"/mnt/drive1"}`), &got); err != nil {
			t.Fatal(err)
		}
		if got.SMART != nil {
			t.Fatalf("want nil SMART, got %+v", got.SMART)
		}
	})

	t.Run("round trip", func(t *testing.T) {
		want := Partition{
			Device:     "/dev/nvme5n1",
			Major:      259,
			Minor:      7,
			Mountpoint: "/mnt/drive6",
			SMART: &SMARTInfo{
				N:            1,
				Status:       map[string]int{"healthy": 1},
				StatsN:       1,
				Temperature:  86,
				PowerOnHours: 26280,
				DeviceType:   "nvme",
				NVMe: &SMARTNVMe{
					N:                     1,
					WarningTempTime:       31540,
					CritCompTime:          95,
					ThermalManagementTime: 120,
					WarningTempThreshold:  75,
					CriticalTempThreshold: 85,
				},
			},
		}
		buf, err := json.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		var got Partition
		if err := json.Unmarshal(buf, &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("round trip:\nwant %+v\ngot  %+v", want.SMART.NVMe, got.SMART.NVMe)
		}
	})
}
