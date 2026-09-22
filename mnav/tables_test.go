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

func TestDescribeCatalogScannerNeverRun(t *testing.T) {
	got := describeCatalogScanner(&madmin.CatalogScannerMetrics{})
	if !strings.Contains(got, "never run") {
		t.Errorf("describeCatalogScanner() = %q, want it to contain %q", got, "never run")
	}
}

func TestDescribeCatalogScannerRunning(t *testing.T) {
	got := describeCatalogScanner(&madmin.CatalogScannerMetrics{Running: true, Cycles: 3})
	if !strings.HasPrefix(got, "running") {
		t.Errorf("describeCatalogScanner() = %q, want it to start with %q", got, "running")
	}
}

func TestDescribeCatalogScannerCompleted(t *testing.T) {
	finishedAt := time.Date(2026, 3, 4, 9, 30, 0, 0, time.UTC)
	got := describeCatalogScanner(&madmin.CatalogScannerMetrics{
		Cycles: 12,
		Previous: &madmin.CatalogScannerCycle{
			FinishedAt: &finishedAt, DurationSecs: 2.5, Warehouses: 4, Tables: 30,
		},
	})
	// Rendered in the reader's local timezone, not UTC -- compute the
	// expectation the same way rather than hardcoding a UTC-only string,
	// which would only pass by accident on a UTC test runner.
	wantLast := "last " + finishedAt.Local().Format("15:04 MST")
	for _, want := range []string{wantLast, "12 cycles", "2.5s/cycle", "4 warehouses", "30 tables"} {
		if !strings.Contains(got, want) {
			t.Errorf("describeCatalogScanner() = %q, want it to contain %q", got, want)
		}
	}
	if strings.Contains(got, "failed") {
		t.Errorf("describeCatalogScanner() = %q, must not mention a failure for a completed cycle", got)
	}
}

// A failed cycle carries no Warehouses/Tables/DurationSecs of its own (see
// CatalogScannerCycle.Failed's doc comment) -- the description must say so
// rather than rendering the zeros as if they were real counts.
func TestDescribeCatalogScannerFailedCycle(t *testing.T) {
	finishedAt := time.Date(2026, 3, 4, 9, 30, 0, 0, time.UTC)
	got := describeCatalogScanner(&madmin.CatalogScannerMetrics{
		Cycles: 12, Errors: 1,
		Previous: &madmin.CatalogScannerCycle{FinishedAt: &finishedAt, Failed: 1},
	})
	if !strings.Contains(got, "last cycle failed (1)") {
		t.Errorf("describeCatalogScanner() = %q, want it to contain %q", got, "last cycle failed (1)")
	}
	for _, notWant := range []string{"warehouses", "tables", "s/cycle"} {
		if strings.Contains(got, notWant) {
			t.Errorf("describeCatalogScanner() = %q, must not report %q for a failed cycle with no counts", got, notWant)
		}
	}
}

func TestTableMetricsNodeLeafDataIncludesCatalogScanner(t *testing.T) {
	node := NewTableMetricsNode(&madmin.TableAPIMetrics{
		CatalogScanner: &madmin.CatalogScannerMetrics{Cycles: 5},
	}, nil, "tables")

	data := node.GetLeafData()
	var found bool
	for k, v := range data {
		if strings.HasSuffix(k, ":Catalog Scanner") {
			found = true
			if !strings.Contains(v, "5 cycles") {
				t.Errorf("Catalog Scanner leaf = %q, want it to contain %q", v, "5 cycles")
			}
		}
	}
	if !found {
		t.Errorf("GetLeafData() = %v, want a Catalog Scanner entry", data)
	}
}

// A nil CatalogScanner (a peer that hasn't reported one, or an older node)
// must not show a "never run" line -- that would misreport "not yet
// started" for data that was simply never collected.
func TestTableMetricsNodeLeafDataOmitsCatalogScannerWhenAbsent(t *testing.T) {
	node := NewTableMetricsNode(&madmin.TableAPIMetrics{}, nil, "tables")

	data := node.GetLeafData()
	for k := range data {
		if strings.HasSuffix(k, ":Catalog Scanner") {
			t.Errorf("GetLeafData() = %v, want no Catalog Scanner entry when absent", data)
		}
	}
}

// The one-line summary in GetLeafData is not a substitute for the full
// subsection -- TableMetricsNode must offer catalog_scanner as a real,
// navigable child regardless of whether CatalogScanner is populated (the
// same convention top_warehouses etc. already follow: always listed, the
// child itself reports "no data" when there is none).
func TestTableMetricsNodeChildrenIncludesCatalogScanner(t *testing.T) {
	node := NewTableMetricsNode(&madmin.TableAPIMetrics{
		CatalogScanner: &madmin.CatalogScannerMetrics{Cycles: 5},
	}, nil, "tables")

	var found bool
	for _, c := range node.GetChildren() {
		if c.Name == "catalog_scanner" {
			found = true
			if c.Description == "" {
				t.Error("catalog_scanner child has no description")
			}
		}
	}
	if !found {
		t.Errorf("GetChildren() = %v, want a catalog_scanner entry", node.GetChildren())
	}

	child, err := node.GetChild("catalog_scanner")
	if err != nil {
		t.Fatalf("GetChild(catalog_scanner): %v", err)
	}
	if _, ok := child.(*catalogScannerNode); !ok {
		t.Errorf("GetChild(catalog_scanner) = %T, want *catalogScannerNode", child)
	}
}

func TestCatalogScannerNodeNoData(t *testing.T) {
	node := &catalogScannerNode{}
	data := node.GetLeafData()
	if data["Status"] != "No catalog scanner data available" {
		t.Errorf("GetLeafData() = %v, want a no-data status", data)
	}
	if len(node.GetChildren()) != 0 {
		t.Errorf("GetChildren() = %v, want none -- this is a leaf subsection", node.GetChildren())
	}
}

func TestCatalogScannerNodeRunning(t *testing.T) {
	startedAt := time.Now().Add(-90 * time.Second).Truncate(time.Second)
	node := &catalogScannerNode{scanner: &madmin.CatalogScannerMetrics{
		Running: true, Cycles: 5,
		Current: &madmin.CatalogScannerCycle{StartedAt: startedAt},
	}}
	data := node.GetLeafData()

	if !valueContains(data, "Status", "Running") {
		t.Errorf("GetLeafData() = %v, want Status = Running", data)
	}
	if !valueContains(data, "Current Cycle Running For", "") {
		t.Errorf("GetLeafData() = %v, want a Current Cycle Running For entry", data)
	}
}

// Current is a pointer, so a zero StartedAt is a real state -- the field is
// omitted rather than defaulted elsewhere in this payload, so a Current with
// nothing set is possible. time.Since(zero) would otherwise render as a
// saturated multi-million-hour duration off a bogus start time.
func TestCatalogScannerNodeRunningWithZeroStartedAt(t *testing.T) {
	node := &catalogScannerNode{scanner: &madmin.CatalogScannerMetrics{
		Running: true, Cycles: 5,
		Current: &madmin.CatalogScannerCycle{},
	}}
	data := node.GetLeafData()

	if valueContains(data, "Current Cycle Started", "") {
		t.Errorf("GetLeafData() = %v, must not report Current Cycle Started for a zero StartedAt", data)
	}
	if valueContains(data, "Current Cycle Running For", "") {
		t.Errorf("GetLeafData() = %v, must not report Current Cycle Running For a zero StartedAt", data)
	}
}

// The completed-cycle case is where "derive rates for the cycle
// information" actually matters -- a raw count and a raw duration answer
// neither "is this fast" nor "is this slow" without dividing one by the
// other, and this is the only place in mnav that does.
func TestCatalogScannerNodeCompletedCycleDerivesRates(t *testing.T) {
	finishedAt := time.Now()
	node := &catalogScannerNode{scanner: &madmin.CatalogScannerMetrics{
		Cycles: 10,
		Previous: &madmin.CatalogScannerCycle{
			FinishedAt: &finishedAt, DurationSecs: 10,
			Warehouses: 4, Tables: 100, Created: 20, Updated: 30, Tombstoned: 0,
		},
	}}
	data := node.GetLeafData()

	if !valueContains(data, "Warehouses/sec", "0.4") {
		t.Errorf("GetLeafData() = %v, want Warehouses/sec = 0.4 (4 warehouses / 10s)", data)
	}
	if !valueContains(data, "Tables/sec", "10.0") {
		t.Errorf("GetLeafData() = %v, want Tables/sec = 10.0 (100 tables / 10s)", data)
	}
	// 20 created + 30 updated + 0 tombstoned = 50 mutations / 10s.
	if !valueContains(data, "Mutations/sec", "5.0") {
		t.Errorf("GetLeafData() = %v, want Mutations/sec = 5.0", data)
	}
}

// A failed cycle carries no counts of its own (see CatalogScannerCycle.Failed's
// doc comment) -- there is nothing to derive a rate from, and the node must
// not divide zero by the duration and print a misleadingly precise "0.0".
func TestCatalogScannerNodeFailedCycleReportsNoRates(t *testing.T) {
	finishedAt := time.Now()
	node := &catalogScannerNode{scanner: &madmin.CatalogScannerMetrics{
		Errors: 1,
		Previous: &madmin.CatalogScannerCycle{
			FinishedAt: &finishedAt, DurationSecs: 5, Failed: 1,
		},
	}}
	data := node.GetLeafData()

	if !valueContains(data, "Last Cycle", "Failed (1)") {
		t.Errorf("GetLeafData() = %v, want Last Cycle = Failed (1)", data)
	}
	if !valueContains(data, "Lifetime Error Rate", "100.0%") {
		t.Errorf("GetLeafData() = %v, want Lifetime Error Rate = 100.0%%", data)
	}
	for _, key := range []string{"Warehouses/sec", "Tables/sec", "Mutations/sec", "Last Cycle Warehouses", "Last Cycle Tables"} {
		if valueContains(data, key, "") {
			t.Errorf("GetLeafData() = %v, must not report %q for a failed cycle with no counts", data, key)
		}
	}
}

// valueContains reports whether data has a key ending in ":suffix" whose
// value contains want ("" matches any value, i.e. "does the key exist").
func valueContains(data map[string]string, suffix, want string) bool {
	for k, v := range data {
		if strings.HasSuffix(k, ":"+suffix) {
			return want == "" || strings.Contains(v, want)
		}
	}
	return false
}
