//go:build linux
// +build linux

//
// Copyright (c) 2015-2024 MinIO, Inc.
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
	"hash/crc32"

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
	return cpuFreqSpeeds(stats), nil
}

// cpuFreqSpeeds deduplicates CPU frequency stats and converts them.
func cpuFreqSpeeds(stats []sysfs.SystemCPUCpufreqStats) []CPUFreqStats {
	found := map[uint32]*CPUFreqStats{}
	for _, stat := range stats {
		c := CPUFreqStats{
			Count:                    1,
			CpuinfoMinimumFrequency:  stat.CpuinfoMinimumFrequency,
			CpuinfoMaximumFrequency:  stat.CpuinfoMaximumFrequency,
			CpuinfoTransitionLatency: stat.CpuinfoTransitionLatency,
			ScalingMinimumFrequency:  stat.ScalingMinimumFrequency,
			ScalingMaximumFrequency:  stat.ScalingMaximumFrequency,
			AvailableGovernors:       stat.AvailableGovernors,
			Driver:                   stat.Driver,
			Governor:                 stat.Governor,
			SetSpeed:                 stat.SetSpeed,
		}
		b, _ := json.Marshal(c)
		h := crc32.ChecksumIEEE(b)
		// Set variable fields, excluded from dedupe...
		c.Name = stat.Name
		c.RelatedCpus = stat.RelatedCpus
		c.CpuinfoCurrentFrequency = stat.CpuinfoCurrentFrequency
		c.ScalingCurrentFrequency = stat.ScalingCurrentFrequency

		if f := found[h]; f != nil {
			f.Count++
			continue
		}
		found[h] = &c
	}

	out := make([]CPUFreqStats, 0, len(found))
	for _, v := range found {
		out = append(out, *v)
	}
	return out
}
