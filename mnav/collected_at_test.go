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

// Ages are measured against the collection time, so metrics restored from a
// capture read as they did when taken rather than as decades stale. A capture
// timestamp far in the past makes a wall-clock regression obvious.
var captureAt = time.Date(2001, 5, 14, 8, 15, 0, 0, time.UTC)

func TestCollectedAtFromRoot(t *testing.T) {
	nav := NewRealtimeMetricsNavigator(&madmin.RealtimeMetrics{
		CollectedAt: captureAt,
		Aggregated: madmin.Metrics{Locks: &madmin.LockMetrics{
			Purge: &madmin.LockPurgeStats{
				SampledAt:    captureAt.Add(-90 * time.Second),
				OldestHeldAt: captureAt.Add(-5 * time.Minute),
				Readers:      3,
				Writers:      1,
			},
		}},
	})
	leaf, err := nav.Navigate("locks/purge")
	if err != nil {
		t.Fatalf("navigate locks/purge: %v", err)
	}
	d := leaf.GetLeafData()
	if got := leafValue(d, "Sampled At"); !strings.Contains(got, "1m30s ago") {
		t.Errorf("Sampled At = %q, want an age measured against collection time", got)
	}
	if got := leafValue(d, "Oldest Lock"); !strings.Contains(got, "held 5m0s") {
		t.Errorf("Oldest Lock = %q, want an age measured against collection time", got)
	}
}

// A metric family carries its own collection time. The nearest ancestor that
// knows one wins, so a family scraped at a different moment than the envelope
// reports against its own timestamp.
func TestCollectedAtFamilyBeatsRoot(t *testing.T) {
	diskAt := captureAt.Add(30 * time.Minute)
	nav := NewRealtimeMetricsNavigator(&madmin.RealtimeMetrics{
		CollectedAt: captureAt,
		Aggregated: madmin.Metrics{Disk: &madmin.DiskMetric{
			CollectedAt: diskAt,
			NDisks:      1,
			Reclaim: madmin.DriveReclaimStats{
				CleanupCycles: 1,
				LastCleanupAt: diskAt.Add(-45 * time.Second),
			},
		}},
	})
	leaf, err := nav.Navigate("drive/reclaim")
	if err != nil {
		t.Fatalf("navigate drive/reclaim: %v", err)
	}
	if got := leafValue(leaf.GetLeafData(), "Last Cleanup"); !strings.Contains(got, "45s ago") {
		t.Errorf("Last Cleanup = %q, want the age against the disk collection time", got)
	}

	// Same rule one family over, reached through an intermediate node: the purge
	// leaf resolves its parent's timestamp, not the envelope's.
	lockAt := captureAt.Add(20 * time.Minute)
	nav = NewRealtimeMetricsNavigator(&madmin.RealtimeMetrics{
		CollectedAt: captureAt,
		Aggregated: madmin.Metrics{Locks: &madmin.LockMetrics{
			CollectedAt: lockAt,
			Purge: &madmin.LockPurgeStats{
				SampledAt: lockAt.Add(-2 * time.Minute),
				Readers:   1,
			},
		}},
	})
	if leaf, err = nav.Navigate("locks/purge"); err != nil {
		t.Fatalf("navigate locks/purge: %v", err)
	}
	if got := leafValue(leaf.GetLeafData(), "Sampled At"); !strings.Contains(got, "2m0s ago") {
		t.Errorf("Sampled At = %q, want the age against the lock collection time", got)
	}
}

// With no collection time anywhere on the chain there is no reference point, so
// the absolute timestamp must be shown rather than an age. Substituting the wall
// clock here would reintroduce the bug on exactly the captures that lack one.
func TestCollectedAtUnknownShowsAbsolute(t *testing.T) {
	stamp := time.Date(2003, 9, 2, 4, 5, 6, 0, time.UTC)
	nav := NewRealtimeMetricsNavigator(&madmin.RealtimeMetrics{
		Aggregated: madmin.Metrics{Disk: &madmin.DiskMetric{
			NDisks:  1,
			Reclaim: madmin.DriveReclaimStats{CleanupCycles: 1, LastCleanupAt: stamp},
		}},
	})
	leaf, err := nav.Navigate("drive/reclaim")
	if err != nil {
		t.Fatalf("navigate drive/reclaim: %v", err)
	}
	got := leafValue(leaf.GetLeafData(), "Last Cleanup")
	if want := "2003-09-02 04:05:06"; got != want {
		t.Errorf("Last Cleanup = %q, want the bare stamp %q with no derived age", got, want)
	}
}

// An unset timestamp has no stamp and no age to show, so its row is omitted
// rather than rendered as the zero time -- which would read as year one, or as an
// age of two millennia once measured against collection.
func TestUnsetTimestampOmitsRow(t *testing.T) {
	nav := NewRealtimeMetricsNavigator(&madmin.RealtimeMetrics{
		CollectedAt: captureAt,
		Aggregated: madmin.Metrics{
			Locks: &madmin.LockMetrics{
				CollectedAt: captureAt,
				// Counts recorded, no timestamp on either field.
				Purge: &madmin.LockPurgeStats{Readers: 4, Writers: 2, Expired: 1},
			},
			IAM: &madmin.IAMMetrics{
				CollectedAt: captureAt,
				Cache:       &madmin.IAMCacheStats{Policies: 3},
			},
		},
	})

	for _, tc := range []struct{ path, key string }{
		{"locks/purge", "Sampled At"},
		{"locks", "Cleanup Sampled"},
		{"locks", "Oldest Lock Held"},
		{"iam", "Inventory Sampled"},
	} {
		leaf, err := nav.Navigate(tc.path)
		if err != nil {
			t.Fatalf("navigate %s: %v", tc.path, err)
		}
		if got := leafValue(leaf.GetLeafData(), tc.key); got != "" {
			t.Errorf("%s[%s] = %q, want the row omitted for an unset timestamp", tc.path, tc.key, got)
		}
	}

	// The rows that do not depend on a timestamp must still be there.
	leaf, err := nav.Navigate("locks/purge")
	if err != nil {
		t.Fatalf("navigate locks/purge: %v", err)
	}
	if got, want := leafValue(leaf.GetLeafData(), "Read Locks"), "4"; got != want {
		t.Errorf("Read Locks = %q, want %q", got, want)
	}
}

// The same rule on an age-only row: it degrades to the absolute stamp, since
// there is no stamp elsewhere on the row to fall back to.
func TestCollectedAtUnknownAgeOnlyRow(t *testing.T) {
	nav := NewRealtimeMetricsNavigator(&madmin.RealtimeMetrics{
		Aggregated: madmin.Metrics{Locks: &madmin.LockMetrics{
			Purge: &madmin.LockPurgeStats{
				SampledAt: time.Date(2003, 9, 2, 4, 5, 6, 0, time.UTC),
				Readers:   1,
			},
		}},
	})
	leaf, err := nav.Navigate("locks")
	if err != nil {
		t.Fatalf("navigate locks: %v", err)
	}
	if got, want := leafValue(leaf.GetLeafData(), "Cleanup Sampled"), "04:05:06"; got != want {
		t.Errorf("Cleanup Sampled = %q, want the bare stamp %q", got, want)
	}
}
