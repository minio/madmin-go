// Copyright (c) 2015-2026 MinIO, Inc.
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

package mnav

import (
	"strings"
	"testing"
	"time"

	"github.com/minio/madmin-go/v4"
)

// collectedAt is fixed so the derived cleanup age does not depend on the wall
// clock: the reader measures against collection time, not time.Now.
var reclaimCollectedAt = time.Date(2026, 2, 3, 10, 30, 0, 0, time.UTC)

func reclaimNav(t *testing.T, r madmin.DriveReclaimStats) MetricNavigator {
	t.Helper()
	return NewRealtimeMetricsNavigator(&madmin.RealtimeMetrics{
		Aggregated: madmin.Metrics{Disk: &madmin.DiskMetric{
			CollectedAt: reclaimCollectedAt, NDisks: 4, Reclaim: r,
		}},
	})
}

// Reclaim is a value, not a pointer, so a drive on which no pass has completed
// carries an all-zero block. That must not be listed as a child: it would read as
// "reclamation measured, nothing reclaimed" on a server that has simply not swept
// yet.
func TestDiskReclaimHiddenUntilItHasRun(t *testing.T) {
	node, err := reclaimNav(t, madmin.DriveReclaimStats{}).Navigate("drive")
	if err != nil {
		t.Fatalf("navigate drive: %v", err)
	}
	for _, name := range childNames(node.GetChildren()) {
		if name == "reclaim" {
			t.Error("reclaim listed although no cleanup pass has completed")
		}
	}

	// A single completed cycle with nothing to reclaim is still a measurement.
	node, err = reclaimNav(t, madmin.DriveReclaimStats{CleanupCycles: 1}).Navigate("drive")
	if err != nil {
		t.Fatalf("navigate drive: %v", err)
	}
	var found bool
	for _, name := range childNames(node.GetChildren()) {
		if name == "reclaim" {
			found = true
		}
	}
	if !found {
		t.Errorf("reclaim not listed after a cycle completed: %v", childNames(node.GetChildren()))
	}
}

// The counters are reported as-is. Notably no backlog is derived from them: the
// trash sweeper also removes entries staged by paths that do not increment the
// counters above it, so a difference would not be a backlog.
func TestDiskReclaimLeaf(t *testing.T) {
	last := reclaimCollectedAt.Add(-90 * time.Second)
	nav := reclaimNav(t, madmin.DriveReclaimStats{
		StaleMultipartPurged: 1200,
		TmpWriteDirPurged:    25,
		TrashPurged:          1000,
		TrashPurgedBytes:     5 * 1024 * 1024 * 1024,
		CleanupCycles:        7,
		LastCleanupAt:        last,
	})
	leaf, err := nav.Navigate("drive/reclaim")
	if err != nil {
		t.Fatalf("navigate drive/reclaim: %v", err)
	}
	d := leaf.GetLeafData()

	for key, want := range map[string]string{
		"Stale Multipart Purged": "1,200",
		"Tmp Write Dirs Purged":  "25",
		"Trash Purged":           "1,000 object(s), 5.0 GiB",
		"Cleanup Cycles":         "7",
	} {
		if got := leafValue(d, key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	// The age is derived here; the wire carries only the timestamp.
	if got := leafValue(d, "Last Cleanup"); !strings.Contains(got, "1m30s ago") {
		t.Errorf("Last Cleanup = %q, want it to carry the derived age", got)
	}
}

// The counters cover different populations, so no backlog may be inferred from
// the difference between them -- staged exceeding purged is not a backlog row.
func TestDiskReclaimNoDerivedBacklog(t *testing.T) {
	nav := reclaimNav(t, madmin.DriveReclaimStats{
		StaleMultipartPurged: 1200, TmpWriteDirPurged: 25, TrashPurged: 1000, CleanupCycles: 3,
	})
	leaf, err := nav.Navigate("drive/reclaim")
	if err != nil {
		t.Fatalf("navigate drive/reclaim: %v", err)
	}
	for key, value := range leaf.GetLeafData() {
		if strings.Contains(strings.ToLower(key), "awaiting") {
			t.Errorf("derived backlog row %q = %q, want none", key, value)
		}
	}
}

// Metrics loaded from a capture must report the age at collection time, not an
// age inflated by however long ago the capture was taken.
func TestDiskReclaimAgeUsesCollectionTime(t *testing.T) {
	nav := reclaimNav(t, madmin.DriveReclaimStats{
		CleanupCycles: 2,
		LastCleanupAt: reclaimCollectedAt.Add(-2 * time.Minute),
	})
	leaf, err := nav.Navigate("drive/reclaim")
	if err != nil {
		t.Fatalf("navigate drive/reclaim: %v", err)
	}
	if got := leafValue(leaf.GetLeafData(), "Last Cleanup"); !strings.Contains(got, "2m0s ago") {
		t.Errorf("Last Cleanup = %q, want the age measured against collection time", got)
	}
}

// Clock skew across nodes can leave a cleanup stamped after collection. That
// must not render as a negative age.
func TestDiskReclaimFutureCleanupNotNegative(t *testing.T) {
	nav := reclaimNav(t, madmin.DriveReclaimStats{
		CleanupCycles: 2,
		LastCleanupAt: reclaimCollectedAt.Add(5 * time.Second),
	})
	leaf, err := nav.Navigate("drive/reclaim")
	if err != nil {
		t.Fatalf("navigate drive/reclaim: %v", err)
	}
	got := leafValue(leaf.GetLeafData(), "Last Cleanup")
	if !strings.Contains(got, "(in 5s)") {
		t.Errorf("Last Cleanup = %q, want a forward-looking age, not a negative one", got)
	}
}
