//go:build !linux

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
	"github.com/shirou/gopsutil/v4/process"
)

// addMemoryMaps aggregates memory map information for non-Linux platforms
// Non-Linux implementation with limited memory map support
func addMemoryMaps(memMaps []process.MemoryMapsStat, metrics *ProcessMemoryMaps) {
	// For non-Linux platforms, we can't get detailed memory map information
	// Just count the number of memory maps if available
	if len(memMaps) > 0 {
		metrics.Count += len(memMaps)
		// Note: memory map fields like Size and RSS may not be available on non-Linux platforms
		// Skip detailed aggregation for cross-platform compatibility
	}
}
