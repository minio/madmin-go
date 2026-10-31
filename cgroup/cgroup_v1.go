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

const (
	// memoryController is the name of the memory controller in cgroup v1.
	memoryController = "memory"

	// memoryLimitFile is the cgroup v1 memory limit.
	memoryLimitFile = "memory.limit_in_bytes"

	// unlimitedMemory is the smallest value treated as "no limit". cgroup v1
	// reports PAGE_COUNTER_MAX (about 2^63) for an unlimited cgroup, and no
	// real system has 4 EiB of memory.
	unlimitedMemory = uint64(1) << 62
)

// memoryLimitV1 reads the limit from the v1 memory hierarchy.
func memoryLimitV1(mounts []mountInfo, cgPath string) (uint64, error) {
	var preferred, others []mountInfo
	for _, m := range mounts {
		if m.fsType != "cgroup" {
			continue
		}
		if hasOption(m.superOptions, memoryController) {
			preferred = append(preferred, m)
		} else {
			others = append(others, m)
		}
	}

	var firstErr error
	try := func(m mountInfo) (uint64, bool) {
		limit, err := readMemoryLimit(filepath.Join(m.resolve(cgPath), memoryLimitFile))
		if err == nil {
			return limit, true
		}
		if firstErr == nil {
			firstErr = err
		}
		return 0, false
	}

	// The mount that advertises the memory controller is authoritative.
	for _, m := range preferred {
		if limit, ok := try(m); ok {
			return limit, nil
		}
	}

	// Some runtimes do not list the controllers in the super options, so try
	// the remaining v1 mounts rather than reporting a limit that may exist.
	for _, m := range others {
		if limit, ok := try(m); ok {
			return limit, nil
		}
	}

	if firstErr != nil {
		return 0, firstErr
	}
	return 0, errors.New("cgroup: memory controller is not mounted")
}

// readMemoryLimit reads a cgroup v1 memory.limit_in_bytes file.
func readMemoryLimit(path string) (uint64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	limit, err := parseMemoryLimit(path, strings.TrimSpace(string(b)))
	if err != nil {
		return 0, err
	}
	if limit >= unlimitedMemory {
		return math.MaxUint64, nil
	}
	return limit, nil
}
