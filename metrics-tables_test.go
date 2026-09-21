//
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
//

package madmin

import (
	"maps"
	"reflect"
	"testing"
	"time"
)

func timePtr(t time.Time) *time.Time {
	return &t
}

func mergeTables(parts ...*TableAPIMetrics) *TableAPIMetrics {
	var out TableAPIMetrics
	for _, p := range parts {
		out.Merge(p)
	}
	return &out
}

// The catalog is replicated to every node, so summing it multiplies every count
// by the node count. The freshest sample wins instead.
func TestTableCatalogMergeNeverSums(t *testing.T) {
	t0 := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)

	older := &TableAPIMetrics{Catalog: &TableCatalog{
		SampledAt: t0, Warehouses: 3, Tables: 100,
	}}
	newer := &TableAPIMetrics{Catalog: &TableCatalog{
		SampledAt: t0.Add(time.Minute), Warehouses: 3, Tables: 101,
	}}

	got := mergeTables(older, newer)
	if got.Catalog == nil {
		t.Fatal("Catalog = nil")
	}
	if got.Catalog.Warehouses != 3 {
		t.Errorf("Warehouses = %d, want 3: the catalog must never be summed",
			got.Catalog.Warehouses)
	}
	if got.Catalog.Tables != 101 {
		t.Errorf("Tables = %d, want 101 (the fresher sample)", got.Catalog.Tables)
	}

	// Order must not matter: collectRealtimeMetrics merges remote then local.
	rev := mergeTables(newer, older)
	if *rev.Catalog != *got.Catalog {
		t.Errorf("order dependent: %+v vs %+v", rev.Catalog, got.Catalog)
	}
}

// A node that has not sampled the catalog reports nothing and must not blank a
// real sample.
func TestTableCatalogMergeSkipsAbsent(t *testing.T) {
	full := &TableAPIMetrics{Catalog: &TableCatalog{
		SampledAt: time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC), Tables: 7,
	}}

	got := mergeTables(full, &TableAPIMetrics{})
	if got.Catalog == nil || got.Catalog.Tables != 7 {
		t.Errorf("Catalog = %+v, want preserved", got.Catalog)
	}
}

// Health is per-node and must sum: this is what makes the console's Health card
// correct on a multi-node cluster for the first time.
func TestTableHealthMergeSums(t *testing.T) {
	a := &TableAPIMetrics{Health: &TableHealth{
		N: 1, ZombieTxns: 2, RecoveryOps: 1, PeerRPCs: 10,
		Rollbacks:   map[string]uint64{"recovery": 2},
		VendedCreds: map[string]uint64{"issued": 5},
	}}
	b := &TableAPIMetrics{Health: &TableHealth{
		N: 1, ZombieTxns: 3, RecoveryOps: 4, PeerRPCs: 7,
		Rollbacks:   map[string]uint64{"recovery": 1, "commit": 6},
		VendedCreds: map[string]uint64{"issued": 2, "denied": 1},
	}}

	got := mergeTables(a, b).Health
	if got == nil {
		t.Fatal("Health = nil")
	}
	if got.N != 2 || got.ZombieTxns != 5 || got.RecoveryOps != 5 || got.PeerRPCs != 17 {
		t.Errorf("Health = %+v, want N=2 zombie=5 recovery=5 peerRPCs=17", got)
	}
	// The rollback split is preserved rather than pre-summed: recovery means a
	// crashed writer was cleaned up, commit means a commit lost a race.
	if !maps.Equal(got.Rollbacks, map[string]uint64{"recovery": 3, "commit": 6}) {
		t.Errorf("Rollbacks = %v", got.Rollbacks)
	}
	if !maps.Equal(got.VendedCreds, map[string]uint64{"issued": 7, "denied": 1}) {
		t.Errorf("VendedCreds = %v", got.VendedCreds)
	}
}

// Maintenance jobs are leader-owned. A demoted leader keeps higher lifetime
// totals in memory, so selecting on "most cycles" would pin the reported state
// to a node that stopped running the job.
func TestTableMaintenanceMergeSelectsLiveLeader(t *testing.T) {
	t0 := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)

	demoted := &TableAPIMetrics{Maintenance: map[string]TableMaintenanceJob{
		"compaction": {Cycles: 500, LastRun: t0, Running: false, TablesProcessed: 5000},
	}}
	// The current leader has run far fewer cycles but is running one now.
	current := &TableAPIMetrics{Maintenance: map[string]TableMaintenanceJob{
		"compaction": {Cycles: 3, LastRun: t0.Add(-time.Hour), Running: true, TablesProcessed: 30},
	}}

	got := mergeTables(demoted, current).Maintenance["compaction"]
	if !got.Running || got.Cycles != 3 {
		t.Errorf("got %+v, want the running leader's report (cycles 3), not the "+
			"demoted node's higher lifetime totals", got)
	}

	rev := mergeTables(current, demoted).Maintenance["compaction"]
	if !reflect.DeepEqual(rev, got) {
		t.Errorf("order dependent: %+v vs %+v", rev, got)
	}
}

// With neither node running a cycle, the most recently completed one wins.
func TestTableMaintenanceMergeSelectsMostRecent(t *testing.T) {
	t0 := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)

	stale := &TableAPIMetrics{Maintenance: map[string]TableMaintenanceJob{
		"snapshot_expiration": {Cycles: 90, LastRun: t0.Add(-time.Hour)},
	}}
	fresh := &TableAPIMetrics{Maintenance: map[string]TableMaintenanceJob{
		"snapshot_expiration": {Cycles: 4, LastRun: t0},
	}}

	got := mergeTables(stale, fresh).Maintenance["snapshot_expiration"]
	if got.Cycles != 4 {
		t.Errorf("Cycles = %d, want 4 (the most recent cycle)", got.Cycles)
	}
	// Never summed: 90+4 would double-count every cycle the old leader ran.
	if got.Cycles == 94 {
		t.Error("maintenance cycles were summed")
	}

	rev := mergeTables(fresh, stale).Maintenance["snapshot_expiration"]
	if !reflect.DeepEqual(rev, got) {
		t.Errorf("order dependent: %+v vs %+v", rev, got)
	}
}

// Distinct job types are independent keys, and Work carries only the units of
// work that job actually has.
func TestTableMaintenanceMergeKeepsJobsSeparate(t *testing.T) {
	a := &TableAPIMetrics{Maintenance: map[string]TableMaintenanceJob{
		"compaction": {Cycles: 2, Work: map[string]uint64{"files_rewritten": 10}},
	}}
	b := &TableAPIMetrics{Maintenance: map[string]TableMaintenanceJob{
		"snapshot_expiration": {Cycles: 1, Work: map[string]uint64{"snapshots_expired": 4}},
	}}

	got := mergeTables(a, b).Maintenance
	if len(got) != 2 {
		t.Fatalf("Maintenance has %d entries, want 2", len(got))
	}
	if !maps.Equal(got["compaction"].Work, map[string]uint64{"files_rewritten": 10}) {
		t.Errorf("compaction Work = %v", got["compaction"].Work)
	}
	if _, ok := got["snapshot_expiration"].Work["files_rewritten"]; ok {
		t.Error("snapshot_expiration carries another job's unit of work")
	}
}

// Selection must be a strict total order, so a tie on every selector still
// merges deterministically.
func TestTableMaintenanceMergeTieIsDeterministic(t *testing.T) {
	t0 := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	a := &TableAPIMetrics{Maintenance: map[string]TableMaintenanceJob{
		"compaction": {Cycles: 5, LastRun: t0, TablesProcessed: 50, Errors: 1},
	}}
	b := &TableAPIMetrics{Maintenance: map[string]TableMaintenanceJob{
		"compaction": {Cycles: 5, LastRun: t0, TablesProcessed: 50, Errors: 9},
	}}

	if !reflect.DeepEqual(mergeTables(a, b).Maintenance["compaction"],
		mergeTables(b, a).Maintenance["compaction"]) {
		t.Error("a full tie merges non-deterministically")
	}
}

// NotOK stays the >= 400 total, so 4xx is derived. 499 is counted as canceled
// and is inside NotOK, which is why it must be subtracted too.
func TestTableAPIStatErrorClassIdentity(t *testing.T) {
	a := TableAPIStat{Reads: 10, NotOK: 5, Errors5xx: 2, Canceled: 1}
	b := TableAPIStat{Reads: 3, NotOK: 4, Errors5xx: 1, Canceled: 2}

	got := a
	got.Add(&b)

	if got.NotOK != 9 || got.Errors5xx != 3 || got.Canceled != 3 {
		t.Fatalf("got %+v, want NotOK=9 Errors5xx=3 Canceled=3", got)
	}
	if fourxx := got.NotOK - got.Errors5xx - got.Canceled; fourxx != 3 {
		t.Errorf("derived 4xx = %d, want 3", fourxx)
	}
}

// A peer on an older build sends shorter or absent slices; the merge must
// right-align rather than panic.
func TestSegmentedTableIOMergeMixedVersions(t *testing.T) {
	// Built by a func, not copied: a struct copy shares the slice backing arrays,
	// so the first Merge would write through into the fixture and the reverse
	// direction below would then be testing against mutated input.
	newCurrent := func() *SegmentedTableIO {
		return &SegmentedTableIO{
			IntervalSecs: 60,
			Reads:        []int64{1, 2},
			NotOK:        []int64{1, 1},
			Errors5xx:    []int64{0, 1},
			Canceled:     []int64{1, 0},
		}
	}
	// No Errors5xx or Canceled at all, and a shorter history.
	newOlder := func() *SegmentedTableIO {
		return &SegmentedTableIO{
			IntervalSecs: 60,
			Reads:        []int64{5},
			NotOK:        []int64{2},
		}
	}

	got := newCurrent()
	got.Merge(newOlder())

	if len(got.Reads) != 2 || got.Reads[1] != 7 {
		t.Errorf("Reads = %v, want the older history right-aligned", got.Reads)
	}
	if len(got.Errors5xx) != 2 || got.Errors5xx[1] != 1 {
		t.Errorf("Errors5xx = %v, want preserved", got.Errors5xx)
	}

	// And the other direction: a current peer merging into an older accumulator.
	rev := newOlder()
	rev.Merge(newCurrent())
	if len(rev.Errors5xx) != 2 || rev.Errors5xx[1] != 1 {
		t.Errorf("Errors5xx = %v after merging into an older accumulator", rev.Errors5xx)
	}
}

func TestSegmentedTableIOAsTableIOStatCarriesErrorClasses(t *testing.T) {
	s := &SegmentedTableIO{
		IntervalSecs: 60,
		NotOK:        []int64{3, 4},
		Errors5xx:    []int64{1, 2},
		Canceled:     []int64{1, 1},
	}
	stats := s.AsTableIOStat()
	if len(stats) != 2 {
		t.Fatalf("got %d stats, want 2", len(stats))
	}
	if stats[1].Errors5xx != 2 || stats[1].Canceled != 1 || stats[1].NotOK != 4 {
		t.Errorf("stats[1] = %+v, want NotOK=4 Errors5xx=2 Canceled=1", stats[1])
	}
}

// The leader-owned select stores the winning report wholesale, so its Work map
// must be cloned or a caller mutating the aggregate reaches into the source.
func TestTableMaintenanceMergeClonesWork(t *testing.T) {
	src := &TableAPIMetrics{Maintenance: map[string]TableMaintenanceJob{
		"compaction": {Cycles: 1, Work: map[string]uint64{"files_rewritten": 5}},
	}}

	var dst TableAPIMetrics
	dst.Merge(src)

	dst.Maintenance["compaction"].Work["files_rewritten"] = 999
	if got := src.Maintenance["compaction"].Work["files_rewritten"]; got != 5 {
		t.Errorf("source Work mutated through the aggregate: %d, want 5", got)
	}
}

func TestCatalogScannerMergeSelectsLiveLeader(t *testing.T) {
	t0 := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)

	demoted := &TableAPIMetrics{CatalogScanner: &CatalogScannerMetrics{
		Cycles: 500, Running: false,
		Previous: &CatalogScannerCycle{FinishedAt: timePtr(t0), Created: 5000},
	}}
	current := &TableAPIMetrics{CatalogScanner: &CatalogScannerMetrics{
		Cycles: 3, Running: true,
		Current: &CatalogScannerCycle{StartedAt: t0.Add(-time.Hour), Created: 30},
	}}

	got := mergeTables(demoted, current).CatalogScanner
	if got == nil || !got.Running || got.Cycles != 3 {
		t.Errorf("got %+v, want the running leader's report (cycles 3), not the "+
			"demoted node's higher lifetime totals", got)
	}

	rev := mergeTables(current, demoted).CatalogScanner
	if !reflect.DeepEqual(rev, got) {
		t.Errorf("order dependent: %+v vs %+v", rev, got)
	}
}

func TestCatalogScannerMergeSelectsMostRecent(t *testing.T) {
	t0 := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)

	stale := &TableAPIMetrics{CatalogScanner: &CatalogScannerMetrics{
		Cycles: 90, Previous: &CatalogScannerCycle{FinishedAt: timePtr(t0.Add(-time.Hour))},
	}}
	fresh := &TableAPIMetrics{CatalogScanner: &CatalogScannerMetrics{
		Cycles: 4, Previous: &CatalogScannerCycle{FinishedAt: timePtr(t0)},
	}}

	got := mergeTables(stale, fresh).CatalogScanner
	if got == nil || got.Cycles != 4 {
		t.Errorf("Cycles = %+v, want 4 (the most recent cycle)", got)
	}

	rev := mergeTables(fresh, stale).CatalogScanner
	if !reflect.DeepEqual(rev, got) {
		t.Errorf("order dependent: %+v vs %+v", rev, got)
	}
}

func TestCatalogScannerMergeSkipsAbsent(t *testing.T) {
	full := &TableAPIMetrics{CatalogScanner: &CatalogScannerMetrics{
		Cycles: 7,
		Previous: &CatalogScannerCycle{
			FinishedAt: timePtr(time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)),
		},
	}}

	got := mergeTables(full, &TableAPIMetrics{}).CatalogScanner
	if got == nil || got.Cycles != 7 {
		t.Errorf("CatalogScanner = %+v, want preserved", got)
	}
}

// Two reports can tie on Running/FinishedAt/Cycles/Errors yet differ in
// every other transmitted field -- Merge must still pick the same one regardless
// of which side it sees first, rather than keeping whichever arrived first.
func TestCatalogScannerMergeTieIsDeterministic(t *testing.T) {
	t0 := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)

	a := &TableAPIMetrics{CatalogScanner: &CatalogScannerMetrics{
		Cycles: 5, Previous: &CatalogScannerCycle{
			FinishedAt: timePtr(t0), DurationSecs: 1, Warehouses: 2, Tables: 3,
			Created: 4, Updated: 5, Tombstoned: 6,
		},
	}}
	b := &TableAPIMetrics{CatalogScanner: &CatalogScannerMetrics{
		Cycles: 5, Previous: &CatalogScannerCycle{
			FinishedAt: timePtr(t0), DurationSecs: 1, Warehouses: 2, Tables: 3,
			Created: 4, Updated: 5, Tombstoned: 9,
		},
	}}

	got := mergeTables(a, b).CatalogScanner
	rev := mergeTables(b, a).CatalogScanner
	if !reflect.DeepEqual(got, rev) {
		t.Errorf("order dependent: %+v vs %+v", got, rev)
	}
	if got.Previous.Tombstoned != 9 {
		t.Errorf("Tombstoned = %d, want 9 (the tie-break winner)", got.Previous.Tombstoned)
	}
}

// A cycle in progress has no FinishedAt yet; freshness for it must fall
// back to StartedAt rather than treating two running cycles as equally
// fresh.
func TestCatalogScannerMergeRunningUsesStartedAt(t *testing.T) {
	t0 := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)

	older := &TableAPIMetrics{CatalogScanner: &CatalogScannerMetrics{
		Running: true, Cycles: 1,
		Current: &CatalogScannerCycle{StartedAt: t0},
	}}
	newer := &TableAPIMetrics{CatalogScanner: &CatalogScannerMetrics{
		Running: true, Cycles: 1,
		Current: &CatalogScannerCycle{StartedAt: t0.Add(time.Minute)},
	}}

	got := mergeTables(older, newer).CatalogScanner
	if !got.Current.StartedAt.Equal(t0.Add(time.Minute)) {
		t.Errorf("StartedAt = %v, want the newer cycle's", got.Current.StartedAt)
	}
}

// A running report with Running true but no Current must not fall back to
// its Previous for freshness purposes -- that would compare the cycle
// before this one as if it were the report's current activity.
func TestCatalogScannerActiveCycleRunningWithoutCurrentIgnoresPrevious(t *testing.T) {
	m := CatalogScannerMetrics{
		Running:  true,
		Previous: &CatalogScannerCycle{Tombstoned: 999},
	}
	if got := m.activeCycle(); got != nil {
		t.Errorf("activeCycle() = %+v, want nil (must not borrow Previous)", got)
	}
}

// Two completed cycles can share a FinishedAt but differ only in StartedAt
// (e.g. one ran longer) -- fresherThan must compare StartedAt as its own
// field rather than relying on DurationSecs happening to differ too.
func TestCatalogScannerMergeTiesOnFinishedAtBreakOnStartedAt(t *testing.T) {
	finishedAt := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)

	a := &TableAPIMetrics{CatalogScanner: &CatalogScannerMetrics{
		Previous: &CatalogScannerCycle{
			FinishedAt: timePtr(finishedAt), StartedAt: finishedAt.Add(-time.Minute), DurationSecs: 60,
		},
	}}
	b := &TableAPIMetrics{CatalogScanner: &CatalogScannerMetrics{
		Previous: &CatalogScannerCycle{
			FinishedAt: timePtr(finishedAt), StartedAt: finishedAt.Add(-2 * time.Minute), DurationSecs: 60,
		},
	}}

	got := mergeTables(a, b).CatalogScanner
	rev := mergeTables(b, a).CatalogScanner
	if !reflect.DeepEqual(got, rev) {
		t.Errorf("order dependent: %+v vs %+v", got, rev)
	}
	if !got.Previous.StartedAt.Equal(finishedAt.Add(-time.Minute)) {
		t.Errorf("StartedAt = %v, want the later-started (fresher) cycle's", got.Previous.StartedAt)
	}
}

// Two running reports can share an identical Current yet carry a different
// Previous (the cycle before the one now running) -- Merge must not treat
// them as equal just because the field it checks first happens to match.
func TestCatalogScannerMergeRunningTiesOnCurrentBreakOnPrevious(t *testing.T) {
	current := &CatalogScannerCycle{StartedAt: time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)}

	a := &TableAPIMetrics{CatalogScanner: &CatalogScannerMetrics{
		Running: true, Current: current,
		Previous: &CatalogScannerCycle{Tombstoned: 1},
	}}
	b := &TableAPIMetrics{CatalogScanner: &CatalogScannerMetrics{
		Running: true, Current: current,
		Previous: &CatalogScannerCycle{Tombstoned: 9},
	}}

	got := mergeTables(a, b).CatalogScanner
	rev := mergeTables(b, a).CatalogScanner
	if !reflect.DeepEqual(got, rev) {
		t.Errorf("order dependent: %+v vs %+v", got, rev)
	}
	if got.Previous.Tombstoned != 9 {
		t.Errorf("Previous.Tombstoned = %d, want 9 (the tie-break winner)", got.Previous.Tombstoned)
	}
}

// A completed cycle and a failed one can otherwise tie (same FinishedAt,
// same everything else a failure leaves zeroed) -- Merge must still prefer
// the completed report deterministically rather than whichever arrived
// first.
func TestCatalogScannerMergeCompletedOutranksFailedOnTie(t *testing.T) {
	finishedAt := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)

	completed := &TableAPIMetrics{CatalogScanner: &CatalogScannerMetrics{
		Previous: &CatalogScannerCycle{FinishedAt: timePtr(finishedAt)},
	}}
	failed := &TableAPIMetrics{CatalogScanner: &CatalogScannerMetrics{
		Previous: &CatalogScannerCycle{FinishedAt: timePtr(finishedAt), Failed: 1},
	}}

	got := mergeTables(completed, failed).CatalogScanner
	rev := mergeTables(failed, completed).CatalogScanner
	if !reflect.DeepEqual(got, rev) {
		t.Errorf("order dependent: %+v vs %+v", got, rev)
	}
	if got.Previous.Failed != 0 {
		t.Errorf("Previous = %+v, want the completed report to win the tie", got.Previous)
	}
}

// A tie on every other field must still resolve by the size of Failed, not
// just its presence -- fewer failures outranks more, so a future per-item
// tally keeps resolving correctly rather than only distinguishing zero from
// nonzero.
func TestCatalogScannerMergeFewerFailuresOutranksMoreOnTie(t *testing.T) {
	finishedAt := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)

	fewer := &TableAPIMetrics{CatalogScanner: &CatalogScannerMetrics{
		Previous: &CatalogScannerCycle{FinishedAt: timePtr(finishedAt), Failed: 1},
	}}
	more := &TableAPIMetrics{CatalogScanner: &CatalogScannerMetrics{
		Previous: &CatalogScannerCycle{FinishedAt: timePtr(finishedAt), Failed: 4},
	}}

	got := mergeTables(fewer, more).CatalogScanner
	rev := mergeTables(more, fewer).CatalogScanner
	if !reflect.DeepEqual(got, rev) {
		t.Errorf("order dependent: %+v vs %+v", got, rev)
	}
	if got.Previous.Failed != 1 {
		t.Errorf("Previous.Failed = %d, want 1 (the tie-break winner)", got.Previous.Failed)
	}
}
