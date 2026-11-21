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

package cgroup

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// unifiedController is the pseudo controller name used by cgroup v2 in
// /proc/<pid>/cgroup, whose single hierarchy is written as "0::/path".
const unifiedController = ""

// memoryMaxFile is the cgroup v2 memory limit. It contains "max" when the
// cgroup has no limit.
const memoryMaxFile = "memory.max"

// IsV2 reports whether the entries describe a cgroup v2 (unified) hierarchy.
func (c CGEntries) IsV2() bool {
	_, ok := c[unifiedController]
	return ok
}

// memoryLimitV2 reads the limit from the unified hierarchy. The boolean
// reports whether a memory.max file was found: a hybrid host can mount
// cgroup2 without the memory controller, in which case v1 is authoritative.
func memoryLimitV2(mounts []mountInfo, cgPath string) (limit uint64, found bool, err error) {
	for _, m := range mounts {
		if m.fsType != "cgroup2" {
			continue
		}
		limit, err := readMemoryMax(filepath.Join(m.resolve(cgPath), memoryMaxFile))
		if err == nil {
			return limit, true, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return 0, true, err
		}
	}
	return 0, false, nil
}

// readMemoryMax reads a cgroup v2 memory.max file, where the literal "max"
// means unlimited.
func readMemoryMax(path string) (uint64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	value := strings.TrimSpace(string(b))
	if value == "max" {
		return math.MaxUint64, nil
	}
	return parseMemoryLimit(path, value)
}
