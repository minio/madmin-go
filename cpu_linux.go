//go:build linux
// +build linux

// Copyright (c) 2015-2022 MinIO, Inc.
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
	"github.com/prometheus/procfs/sysfs"
)

func getCPUFreqStats() ([]CPUFreqStats, error) {
	fs, err := sysfs.NewFS("/sys")
	if err != nil {
		return nil, err
	}

	stats, err := fs.SystemCpufreq()
	if err != nil {
		return nil, err
	}

	out := make([]CPUFreqStats, 0, len(stats))
	for _, stat := range stats {
		out = append(out, CPUFreqStats{
			Name:                     stat.Name,
			CpuinfoCurrentFrequency:  stat.CpuinfoCurrentFrequency,
			CpuinfoMinimumFrequency:  stat.CpuinfoMinimumFrequency,
			CpuinfoMaximumFrequency:  stat.CpuinfoMaximumFrequency,
			CpuinfoTransitionLatency: stat.CpuinfoTransitionLatency,
			ScalingCurrentFrequency:  stat.ScalingCurrentFrequency,
			ScalingMinimumFrequency:  stat.ScalingMinimumFrequency,
			ScalingMaximumFrequency:  stat.ScalingMaximumFrequency,
			AvailableGovernors:       stat.AvailableGovernors,
			Driver:                   stat.Driver,
			Governor:                 stat.Governor,
			RelatedCpus:              stat.RelatedCpus,
			SetSpeed:                 stat.SetSpeed,
		})
	}
	return out, nil
}
