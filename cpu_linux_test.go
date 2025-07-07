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
	"encoding/json"
	"reflect"
	"testing"

	"github.com/prometheus/procfs/sysfs"
)

func Test_cpuFreqSpeeds(t *testing.T) {
	toPtr := func(i uint64) *uint64 {
		return &i
	}
	tests := []struct {
		name string
		args []sysfs.SystemCPUCpufreqStats
		want []CPUFreqStats
	}{
		{
			name: "empty-dedupe",
			args: []sysfs.SystemCPUCpufreqStats{{Name: "a"}, {}, {}},
			want: []CPUFreqStats{{Name: "a", Count: 3}},
		},
		{
			name: "deduplicate",
			want: []CPUFreqStats{{Name: "0", Count: 8, CpuinfoCurrentFrequency: (*uint64)(nil), CpuinfoMinimumFrequency: toPtr(1200000), CpuinfoMaximumFrequency: toPtr(3600000), CpuinfoTransitionLatency: toPtr(20000), ScalingCurrentFrequency: toPtr(2786080), ScalingMinimumFrequency: toPtr(3600000), ScalingMaximumFrequency: toPtr(3600000), AvailableGovernors: "conservative ondemand userspace powersave performance schedutil", Driver: "intel_cpufreq", Governor: "performance", RelatedCpus: "0", SetSpeed: "<unsupported>"}},
		},
	}
	var in []sysfs.SystemCPUCpufreqStats
	testdata := `[{"AvailableGovernors":"conservative ondemand userspace powersave performance schedutil","CpuinfoCurrentFrequency":null,"CpuinfoMaximumFrequency":3600000,"CpuinfoMinimumFrequency":1200000,"CpuinfoTransitionLatency":20000,"Driver":"intel_cpufreq","Governor":"performance","Name":"0","RelatedCpus":"0","ScalingCurrentFrequency":2786080,"ScalingMaximumFrequency":3600000,"ScalingMinimumFrequency":3600000,"SetSpeed":"<unsupported>"},{"AvailableGovernors":"conservative ondemand userspace powersave performance schedutil","CpuinfoCurrentFrequency":null,"CpuinfoMaximumFrequency":3600000,"CpuinfoMinimumFrequency":1200000,"CpuinfoTransitionLatency":20000,"Driver":"intel_cpufreq","Governor":"performance","Name":"1","RelatedCpus":"1","ScalingCurrentFrequency":2780224,"ScalingMaximumFrequency":3600000,"ScalingMinimumFrequency":3600000,"SetSpeed":"<unsupported>"},{"AvailableGovernors":"conservative ondemand userspace powersave performance schedutil","CpuinfoCurrentFrequency":null,"CpuinfoMaximumFrequency":3600000,"CpuinfoMinimumFrequency":1200000,"CpuinfoTransitionLatency":20000,"Driver":"intel_cpufreq","Governor":"performance","Name":"10","RelatedCpus":"10","ScalingCurrentFrequency":2733664,"ScalingMaximumFrequency":3600000,"ScalingMinimumFrequency":3600000,"SetSpeed":"<unsupported>"},{"AvailableGovernors":"conservative ondemand userspace powersave performance schedutil","CpuinfoCurrentFrequency":null,"CpuinfoMaximumFrequency":3600000,"CpuinfoMinimumFrequency":1200000,"CpuinfoTransitionLatency":20000,"Driver":"intel_cpufreq","Governor":"performance","Name":"11","RelatedCpus":"11","ScalingCurrentFrequency":2793146,"ScalingMaximumFrequency":3600000,"ScalingMinimumFrequency":3600000,"SetSpeed":"<unsupported>"},{"AvailableGovernors":"conservative ondemand userspace powersave performance schedutil","CpuinfoCurrentFrequency":null,"CpuinfoMaximumFrequency":3600000,"CpuinfoMinimumFrequency":1200000,"CpuinfoTransitionLatency":20000,"Driver":"intel_cpufreq","Governor":"performance","Name":"12","RelatedCpus":"12","ScalingCurrentFrequency":2793145,"ScalingMaximumFrequency":3600000,"ScalingMinimumFrequency":3600000,"SetSpeed":"<unsupported>"},{"AvailableGovernors":"conservative ondemand userspace powersave performance schedutil","CpuinfoCurrentFrequency":null,"CpuinfoMaximumFrequency":3600000,"CpuinfoMinimumFrequency":1200000,"CpuinfoTransitionLatency":20000,"Driver":"intel_cpufreq","Governor":"performance","Name":"13","RelatedCpus":"13","ScalingCurrentFrequency":2793145,"ScalingMaximumFrequency":3600000,"ScalingMinimumFrequency":3600000,"SetSpeed":"<unsupported>"},{"AvailableGovernors":"conservative ondemand userspace powersave performance schedutil","CpuinfoCurrentFrequency":null,"CpuinfoMaximumFrequency":3600000,"CpuinfoMinimumFrequency":1200000,"CpuinfoTransitionLatency":20000,"Driver":"intel_cpufreq","Governor":"performance","Name":"14","RelatedCpus":"14","ScalingCurrentFrequency":2793145,"ScalingMaximumFrequency":3600000,"ScalingMinimumFrequency":3600000,"SetSpeed":"<unsupported>"},{"AvailableGovernors":"conservative ondemand userspace powersave performance schedutil","CpuinfoCurrentFrequency":null,"CpuinfoMaximumFrequency":3600000,"CpuinfoMinimumFrequency":1200000,"CpuinfoTransitionLatency":20000,"Driver":"intel_cpufreq","Governor":"performance","Name":"15","RelatedCpus":"15","ScalingCurrentFrequency":2793143,"ScalingMaximumFrequency":3600000,"ScalingMinimumFrequency":3600000,"SetSpeed":"<unsupported>"}]`
	err := json.Unmarshal([]byte(testdata), &in)
	if err != nil {
		t.Fatalf("Failed to unmarshal test data: %v", err)
	}
	tests[1].args = in

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cpuFreqSpeeds(tt.args); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("cpuFreqSpeeds() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
