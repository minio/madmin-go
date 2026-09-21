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
	for _, want := range []string{"last 09:30:00", "12 cycles", "2.5s/cycle", "4 warehouses", "30 tables"} {
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
