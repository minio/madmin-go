//go:build linux
// +build linux

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
	"os"
	"path/filepath"
	"strconv"
)

func getCPUFreqStats() (stats []CPUFreqStats, err error) {
	// Attempt to read CPU stats for governor on CPU0
	// which is enough indicating atleast the system
	// has one CPU.
	cpuName := "cpu" + strconv.Itoa(0)

	governorPath := filepath.Join(
		"/sys/devices/system/cpu",
		cpuName,
		"cpufreq",
		"scaling_governor",
	)

	content, err1 := os.ReadFile(governorPath)
	if err1 != nil {
		err = err1
		return
	}

	stats = append(stats, CPUFreqStats{
		Name:     cpuName,
		Governor: string(bytes.TrimSpace(content)),
	})

	return stats, nil
}
