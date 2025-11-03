//go:build linux

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

// addMemoryMaps aggregates Linux-specific memory map information
func addMemoryMaps(memMaps []process.MemoryMapsStat, metrics *ProcessMemoryMaps) {
	for _, memMap := range memMaps {
		metrics.TotalSize += memMap.Size
		metrics.TotalRSS += memMap.Rss
		metrics.TotalPSS += memMap.Pss
		metrics.TotalSharedClean += memMap.SharedClean
		metrics.TotalSharedDirty += memMap.SharedDirty
		metrics.TotalPrivateClean += memMap.PrivateClean
		metrics.TotalPrivateDirty += memMap.PrivateDirty
		metrics.TotalReferenced += memMap.Referenced
		metrics.TotalAnonymous += memMap.Anonymous
		metrics.TotalSwap += memMap.Swap
		metrics.Count++
	}
}
