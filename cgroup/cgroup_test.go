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
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestParseProcCGroup(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  CGEntries
	}{
		{
			name: "v1",
			input: `11:memory:/user.slice
10:blkio:/user.slice
3:cpu,cpuacct:/user.slice
1:name=systemd:/user.slice/user-1000.slice/session-1.scope
`,
			want: CGEntries{
				"memory":  "/user.slice",
				"blkio":   "/user.slice",
				"cpu":     "/user.slice",
				"cpuacct": "/user.slice",
				"systemd": "/user.slice/user-1000.slice/session-1.scope",
			},
		},
		{
			name:  "v2",
			input: "0::/kubepods/burstable/pod123/container\n",
			want:  CGEntries{"": "/kubepods/burstable/pod123/container"},
		},
		{
			name:  "v2 root",
			input: "0::/\n",
			want:  CGEntries{"": "/"},
		},
		{
			name: "blank lines",
			input: `
11:memory:/newtest/newtest

1:name=systemd:/session-1.scope
`,
			want: CGEntries{
				"memory":  "/newtest/newtest",
				"systemd": "/session-1.scope",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseProcCGroup(strings.NewReader(test.input))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("expected %v, got %v", test.want, got)
			}
		})
	}
}

func TestParseProcCGroupMalformed(t *testing.T) {
	if _, err := parseProcCGroup(strings.NewReader("not a cgroup line\n")); err == nil {
		t.Fatal("expected an error for a malformed line")
	}
}

func TestCGEntriesIsV2(t *testing.T) {
	if !(CGEntries{"": "/"}).IsV2() {
		t.Fatal("expected unified entries to be v2")
	}
	if (CGEntries{"memory": "/user.slice"}).IsV2() {
		t.Fatal("expected v1 entries not to be v2")
	}
}

func TestParseMountInfo(t *testing.T) {
	input := `25 24 0:22 / /sys rw,nosuid,nodev,noexec,relatime shared:7 - sysfs sysfs rw
57 25 0:39 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime shared:19 - cgroup2 cgroup2 rw,nsdelegate,memory_recursiveprot
220 36 0:24 /docker/abc /sys/fs/cgroup/memory rw,nosuid - cgroup cgroup rw,memory
221 36 0:25 / /mnt/with\040space rw,relatime master:1 - ext4 /dev/sda1 rw,errors=continue
`
	mounts, err := parseMountInfo(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(mounts) != 4 {
		t.Fatalf("expected 4 mounts, got %d", len(mounts))
	}

	if mounts[1].fsType != "cgroup2" || mounts[1].mountPoint != "/sys/fs/cgroup" {
		t.Fatalf("unexpected cgroup2 mount: %+v", mounts[1])
	}
	if !hasOption(mounts[1].superOptions, "memory_recursiveprot") {
		t.Fatalf("unexpected cgroup2 super options: %q", mounts[1].superOptions)
	}

	if mounts[2].root != "/docker/abc" || mounts[2].fsType != "cgroup" {
		t.Fatalf("unexpected v1 mount: %+v", mounts[2])
	}
	if !hasOption(mounts[2].superOptions, memoryController) {
		t.Fatalf("expected the memory controller in %q", mounts[2].superOptions)
	}

	if mounts[3].mountPoint != "/mnt/with space" {
		t.Fatalf("expected escaped space to be decoded, got %q", mounts[3].mountPoint)
	}
}

func TestParseMountInfoMalformed(t *testing.T) {
	for _, input := range []string{
		"too few fields",
		"36 35 98:0 / /mnt rw shared:1 ext4 /dev/sda1 rw",
	} {
		if _, err := parseMountInfo(strings.NewReader(input)); err == nil {
			t.Fatalf("expected an error for %q", input)
		}
	}
}

func TestMountInfoResolve(t *testing.T) {
	tests := []struct {
		name     string
		mount    mountInfo
		cgPath   string
		expected string
	}{
		{
			name:     "hierarchy root",
			mount:    mountInfo{root: "/", mountPoint: "/sys/fs/cgroup"},
			cgPath:   "/user.slice",
			expected: "/sys/fs/cgroup/user.slice",
		},
		{
			name:     "hierarchy root itself",
			mount:    mountInfo{root: "/", mountPoint: "/sys/fs/cgroup"},
			cgPath:   "/",
			expected: "/sys/fs/cgroup",
		},
		{
			name:     "bind mounted container root",
			mount:    mountInfo{root: "/docker/abc", mountPoint: "/sys/fs/cgroup/memory"},
			cgPath:   "/docker/abc",
			expected: "/sys/fs/cgroup/memory",
		},
		{
			name:     "bind mounted container sub cgroup",
			mount:    mountInfo{root: "/docker/abc", mountPoint: "/sys/fs/cgroup/memory"},
			cgPath:   "/docker/abc/sub",
			expected: "/sys/fs/cgroup/memory/sub",
		},
		{
			name:     "private cgroup namespace",
			mount:    mountInfo{root: "/docker/abc", mountPoint: "/sys/fs/cgroup/memory"},
			cgPath:   "/",
			expected: "/sys/fs/cgroup/memory",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// ToSlash keeps the expectations readable on Windows.
			if got := filepath.ToSlash(test.mount.resolve(test.cgPath)); got != test.expected {
				t.Fatalf("expected %q, got %q", test.expected, got)
			}
		})
	}
}

const testPID = 4242

// fakeProc creates a temporary /proc tree with the given /proc/<pid>/cgroup
// contents and mount table, and returns its root.
func fakeProc(t *testing.T, cgroupContents string, mountInfo string) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, fmt.Sprint(testPID), "cgroup"), cgroupContents)
	writeTestFile(t, filepath.Join(root, "self", "mountinfo"), mountInfo)
	return root
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mountInfoLine(root, mountPoint, fsType, superOptions string) string {
	// mountinfo uses Unix paths; ToSlash also keeps Windows temp paths free
	// of backslashes, which the kernel would have escaped as octal.
	return fmt.Sprintf("36 35 0:26 %s %s rw,nosuid - %s cgroup %s\n",
		root, filepath.ToSlash(mountPoint), fsType, superOptions)
}

func TestGetMemoryLimitV2(t *testing.T) {
	root := t.TempDir()
	cgDir := filepath.Join(root, "sys", "fs", "cgroup")
	writeTestFile(t, filepath.Join(cgDir, "kubepods", "pod", "container", memoryMaxFile), "536870912\n")

	proc := fakeProc(t, "0::/kubepods/pod/container\n",
		mountInfoLine("/", cgDir, "cgroup2", "rw,nsdelegate"))

	limit, err := getMemoryLimit(proc, testPID)
	if err != nil {
		t.Fatal(err)
	}
	if limit != 536870912 {
		t.Fatalf("expected 536870912, got %d", limit)
	}
}

func TestGetMemoryLimitV2Unlimited(t *testing.T) {
	root := t.TempDir()
	cgDir := filepath.Join(root, "sys", "fs", "cgroup")
	writeTestFile(t, filepath.Join(cgDir, memoryMaxFile), "max\n")

	proc := fakeProc(t, "0::/\n", mountInfoLine("/", cgDir, "cgroup2", "rw,nsdelegate"))

	limit, err := getMemoryLimit(proc, testPID)
	if err != nil {
		t.Fatal(err)
	}
	if limit != math.MaxUint64 {
		t.Fatalf("expected math.MaxUint64, got %d", limit)
	}
}

// Docker on cgroup v1 reports the host path of the container's cgroup while
// bind-mounting exactly that directory as the memory hierarchy.
func TestGetMemoryLimitV1(t *testing.T) {
	root := t.TempDir()
	cgDir := filepath.Join(root, "sys", "fs", "cgroup", "memory")
	writeTestFile(t, filepath.Join(cgDir, memoryLimitFile), "268435456\n")

	proc := fakeProc(t, "3:memory:/docker/abc\n",
		mountInfoLine("/docker/abc", cgDir, "cgroup", "rw,memory"))

	limit, err := getMemoryLimit(proc, testPID)
	if err != nil {
		t.Fatal(err)
	}
	if limit != 268435456 {
		t.Fatalf("expected 268435456, got %d", limit)
	}
}

func TestGetMemoryLimitV1Unlimited(t *testing.T) {
	root := t.TempDir()
	cgDir := filepath.Join(root, "sys", "fs", "cgroup", "memory")
	writeTestFile(t, filepath.Join(cgDir, "user.slice", memoryLimitFile), "9223372036854771712\n")

	proc := fakeProc(t, "3:memory:/user.slice\n",
		mountInfoLine("/", cgDir, "cgroup", "rw,memory"))

	limit, err := getMemoryLimit(proc, testPID)
	if err != nil {
		t.Fatal(err)
	}
	if limit != math.MaxUint64 {
		t.Fatalf("expected math.MaxUint64, got %d", limit)
	}
}

// A hybrid host mounts both hierarchies, but the unified one may not carry the
// memory controller. The v1 limit is then authoritative.
func TestGetMemoryLimitHybridFallsBackToV1(t *testing.T) {
	root := t.TempDir()
	cgroup2Dir := filepath.Join(root, "sys", "fs", "cgroup", "unified")
	if err := os.MkdirAll(cgroup2Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	memoryDir := filepath.Join(root, "sys", "fs", "cgroup", "memory")
	writeTestFile(t, filepath.Join(memoryDir, "user.slice", memoryLimitFile), "1073741824\n")

	proc := fakeProc(t, "0::/user.slice\n3:memory:/user.slice\n",
		mountInfoLine("/", cgroup2Dir, "cgroup2", "rw,nsdelegate")+
			mountInfoLine("/", memoryDir, "cgroup", "rw,memory"))

	limit, err := getMemoryLimit(proc, testPID)
	if err != nil {
		t.Fatal(err)
	}
	if limit != 1073741824 {
		t.Fatalf("expected 1073741824, got %d", limit)
	}
}

// Some runtimes omit the controller list from the super options of a v1
// mount, so the limit must still be found.
func TestGetMemoryLimitV1WithoutControllerOption(t *testing.T) {
	root := t.TempDir()
	cgDir := filepath.Join(root, "sys", "fs", "cgroup", "memory")
	writeTestFile(t, filepath.Join(cgDir, "user.slice", memoryLimitFile), "67108864\n")

	proc := fakeProc(t, "3:memory:/user.slice\n",
		mountInfoLine("/", cgDir, "cgroup", "rw"))

	limit, err := getMemoryLimit(proc, testPID)
	if err != nil {
		t.Fatal(err)
	}
	if limit != 67108864 {
		t.Fatalf("expected 67108864, got %d", limit)
	}
}

func TestGetMemoryLimitNoMemoryController(t *testing.T) {
	root := t.TempDir()
	cgDir := filepath.Join(root, "sys", "fs", "cgroup", "cpu")
	writeTestFile(t, filepath.Join(cgDir, "user.slice", "cpu.max"), "max 100000\n")

	proc := fakeProc(t, "3:cpu:/user.slice\n",
		mountInfoLine("/", cgDir, "cgroup", "rw,cpu"))

	_, err := getMemoryLimit(proc, testPID)
	if err == nil || !strings.Contains(err.Error(), "no memory controller") {
		t.Fatalf("expected a missing memory controller error, got %v", err)
	}
}

func TestGetEntriesCurrentProcess(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("cgroups are Linux-only")
	}
	entries, err := GetEntries(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one cgroup entry")
	}
}

// Smoke test against the real /proc of the machine running the tests.
func TestGetMemoryLimitCurrentProcess(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("cgroups are Linux-only")
	}
	limit, err := GetMemoryLimit(os.Getpid())
	if err != nil {
		t.Skipf("cgroup memory limit unavailable: %v", err)
	}
	if limit == 0 {
		t.Fatal("unexpected zero memory limit")
	}
	t.Logf("memory limit: %d", limit)
}

func TestErrNotSupported(t *testing.T) {
	_, entriesErr := GetEntries(os.Getpid())
	_, limitErr := GetMemoryLimit(os.Getpid())

	if runtime.GOOS == "linux" {
		if errors.Is(entriesErr, ErrNotSupported) || errors.Is(limitErr, ErrNotSupported) {
			t.Fatalf("unexpected ErrNotSupported on Linux: %v, %v", entriesErr, limitErr)
		}
		return
	}

	if !errors.Is(entriesErr, ErrNotSupported) {
		t.Fatalf("GetEntries: expected ErrNotSupported, got %v", entriesErr)
	}
	if !errors.Is(limitErr, ErrNotSupported) {
		t.Fatalf("GetMemoryLimit: expected ErrNotSupported, got %v", limitErr)
	}
}
