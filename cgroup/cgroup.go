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

// Package cgroup reads the cgroup (control group) membership and limits of a
// process.
//
// Both the original v1 hierarchies and the unified v2 hierarchy are supported.
// Cgroups are a Linux concept; on other platforms the exported functions
// return ErrNotSupported.
package cgroup

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// ErrNotSupported is returned by the exported functions on platforms that do
// not implement cgroups.
var ErrNotSupported = errors.New("cgroup: not supported on this platform")

// procRoot is where the kernel exposes process information.
const procRoot = "/proc"

// CGEntries maps a cgroup controller to the path of the process's cgroup in
// that controller's hierarchy, as reported by /proc/<pid>/cgroup.
//
// Under cgroup v2 the map holds one entry under the empty controller name,
// because the unified hierarchy is shared by every controller.
type CGEntries map[string]string

// GetEntries reads the cgroup membership of pid from /proc/<pid>/cgroup.
func GetEntries(pid int) (CGEntries, error) {
	if runtime.GOOS != "linux" {
		return nil, ErrNotSupported
	}
	return getEntries(procRoot, pid)
}

// getEntries reads the cgroup membership of pid from procRoot, which is
// normally /proc.
func getEntries(procRoot string, pid int) (CGEntries, error) {
	r, err := os.Open(filepath.Join(procRoot, strconv.Itoa(pid), "cgroup"))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return parseProcCGroup(r)
}

// GetMemoryLimit returns the memory limit, in bytes, of pid's cgroup.
//
// The cgroup v2 unified hierarchy is preferred, with a fallback to v1 for
// hybrid setups where memory is still accounted by v1. A limit of
// math.MaxUint64 means the cgroup is unlimited, so callers should compare the
// result against the machine's memory size and use whichever is smaller.
func GetMemoryLimit(pid int) (uint64, error) {
	if runtime.GOOS != "linux" {
		return 0, ErrNotSupported
	}
	return getMemoryLimit(procRoot, pid)
}

// getMemoryLimit resolves the memory limit of pid's cgroup below procRoot.
//
// The mount table of the target process is used rather than the current
// process's, because a process in another mount namespace may see a different
// cgroup hierarchy at the same paths.
func getMemoryLimit(procRoot string, pid int) (uint64, error) {
	entries, err := getEntries(procRoot, pid)
	if err != nil {
		return 0, err
	}

	mounts, err := readMounts(filepath.Join(procRoot, strconv.Itoa(pid), "mountinfo"))
	if err != nil {
		return 0, err
	}

	// Prefer cgroup v2 when the unified hierarchy accounts for memory. In a
	// hybrid setup a cgroup2 mount exists but may not carry the controller,
	// in which case memory.max is absent and v1 is authoritative.
	if path, ok := entries[unifiedController]; ok {
		limit, found, err := memoryLimitV2(mounts, path)
		if err != nil {
			return 0, err
		}
		if found {
			return limit, nil
		}
	}

	path, ok := entries[memoryController]
	if !ok {
		return 0, fmt.Errorf("cgroup: no memory controller for pid %d", pid)
	}

	return memoryLimitV1(mounts, path)
}

// readMounts parses the mount table stored at path.
func readMounts(path string) ([]mountInfo, error) {
	r, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return parseMountInfo(r)
}

// parseProcCGroup parses the contents of /proc/<pid>/cgroup.
//
// Each line has the form "hierarchy-ID:controller[,controller]:path". The
// controller list is empty for the cgroup v2 unified hierarchy, in which case
// the path is stored under the empty controller name.
func parseProcCGroup(r io.Reader) (CGEntries, error) {
	entries := CGEntries{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.SplitN(line, ":", 3)
		if len(fields) != 3 {
			return nil, fmt.Errorf("cgroup: malformed /proc/<pid>/cgroup line %q", line)
		}

		path := fields[2]
		if fields[1] == "" {
			entries[unifiedController] = path
			continue
		}
		for _, controller := range strings.Split(fields[1], ",") {
			controller = strings.TrimPrefix(controller, "name=")
			if controller != "" {
				entries[controller] = path
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

// mountInfo is the subset of a /proc/<pid>/mountinfo line needed to locate the
// files backing a cgroup hierarchy.
type mountInfo struct {
	root         string
	mountPoint   string
	fsType       string
	superOptions string
}

// resolve maps a cgroup path from /proc/<pid>/cgroup onto this mount.
//
// The two are expressed in the same cgroup namespace, but a mount can be
// rooted below the hierarchy root: inside a container /proc/<pid>/cgroup may
// report the host path (for example /docker/<id>) while the controller is
// bind-mounted from exactly that directory. Stripping the mount root before
// joining makes both layouts resolve to the files that belong to the process.
func (m mountInfo) resolve(cgPath string) string {
	rel := cgPath
	if m.root != "" && m.root != "/" {
		switch {
		case rel == m.root:
			rel = "/"
		case strings.HasPrefix(rel, m.root+"/"):
			rel = strings.TrimPrefix(rel, m.root)
		default:
			// The path lies outside the mounted subtree, for example a
			// private cgroup namespace that reports "/" for the container's
			// own cgroup. The mount point already is the cgroup root.
			rel = "/"
		}
	}
	return filepath.Join(m.mountPoint, rel)
}

// parseMountInfo parses a /proc/<pid>/mountinfo file.
//
// A line consists of at least six space-separated fields, an optional set of
// per-mount fields, a "-" separator and three per-filesystem fields:
//
//	36 35 98:0 /root /mount rw,nosuid shared:1 - cgroup2 cgroup2 rw,nsdelegate
//
// Spaces, tabs, newlines and backslashes in the root and mount point are
// octal-escaped by the kernel.
func parseMountInfo(r io.Reader) ([]mountInfo, error) {
	var mounts []mountInfo
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 10 {
			return nil, fmt.Errorf("cgroup: malformed mountinfo line %q", line)
		}

		// Optional fields sit between the mount options and the separator.
		sep := -1
		for i := 6; i < len(fields); i++ {
			if fields[i] == "-" {
				sep = i
				break
			}
		}
		if sep < 0 || len(fields) < sep+4 {
			return nil, fmt.Errorf("cgroup: malformed mountinfo line %q", line)
		}

		mounts = append(mounts, mountInfo{
			root:         unescapeMountPath(fields[3]),
			mountPoint:   unescapeMountPath(fields[4]),
			fsType:       fields[sep+1],
			superOptions: fields[sep+3],
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return mounts, nil
}

// hasOption reports whether the comma-separated option list contains name.
func hasOption(options, name string) bool {
	for _, option := range strings.Split(options, ",") {
		if option == name {
			return true
		}
	}
	return false
}

// unescapeMountPath decodes the octal escapes the kernel writes for space
// (\040), tab (\011), newline (\012) and backslash (\134) in mountinfo.
func unescapeMountPath(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) &&
			isOctal(s[i+1]) && isOctal(s[i+2]) && isOctal(s[i+3]) {
			b.WriteByte((s[i+1]-'0')<<6 | (s[i+2]-'0')<<3 | (s[i+3] - '0'))
			i += 3
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// isOctal reports whether c is an octal digit.
func isOctal(c byte) bool {
	return c >= '0' && c <= '7'
}

// parseMemoryLimit parses a memory limit file, whose contents are a byte count.
func parseMemoryLimit(path, value string) (uint64, error) {
	limit, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("cgroup: parse %s: %w", path, err)
	}
	return limit, nil
}
