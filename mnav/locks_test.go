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
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package mnav

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/minio/madmin-go/v4"
)

var lockFirstTime = time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)

func lockWindow(interval int, segments ...madmin.LockSegment) *madmin.SegmentedLockMetrics {
	return &madmin.SegmentedLockMetrics{
		Interval:  interval,
		FirstTime: lockFirstTime,
		Segments:  segments,
	}
}

// A busy segment: 100 grants waiting 2s in total, so the mean wait is 20ms.
func busyLockSegment() madmin.LockSegment {
	return madmin.LockSegment{
		N: 2, AcquireCount: 100, AcquireNanos: 2_000_000_000,
		HeldCount: 90, HeldNanos: 900_000_000,
		ReleaseCount: 90, ReleaseNanos: 90_000_000,
	}
}

func lockNav(t *testing.T, locks *madmin.LockMetrics) MetricNavigator {
	t.Helper()
	return NewRealtimeMetricsNavigator(&madmin.RealtimeMetrics{
		Aggregated: madmin.Metrics{Locks: locks},
	})
}

// The cleanup pass has not sampled on a fresh cluster, but the windows must still
// be listed -- and everything listed must be constructible.
func TestLockChildrenWithoutPurge(t *testing.T) {
	nav := lockNav(t, &madmin.LockMetrics{LastHour: lockWindow(60, busyLockSegment())})
	node, err := nav.Navigate("locks")
	if err != nil {
		t.Fatalf("navigate locks: %v", err)
	}
	got := childNames(node.GetChildren())
	if want := []string{"last_hour", "last_day"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("children = %v, want %v", got, want)
	}
	for _, name := range got {
		if _, err := node.GetChild(name); err != nil {
			t.Errorf("GetChild(%q) = %v, want a node", name, err)
		}
	}

	// Once the pass has sampled, purge joins the list and still resolves.
	nav = lockNav(t, &madmin.LockMetrics{Purge: &madmin.LockPurgeStats{Readers: 3}})
	node, err = nav.Navigate("locks")
	if err != nil {
		t.Fatalf("navigate locks: %v", err)
	}
	got = childNames(node.GetChildren())
	if want := []string{"purge", "last_hour", "last_day"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("children with purge = %v, want %v", got, want)
	}
	for _, name := range got {
		if _, err := node.GetChild(name); err != nil {
			t.Errorf("GetChild(%q) = %v, want a node", name, err)
		}
	}
}

// Rows carry the real span they cover, oldest first, and every listed child
// resolves to that segment's leaf.
func TestLockWindowSegments(t *testing.T) {
	rare := madmin.LockSegment{
		N: 2, AcquireCount: 10, AcquireNanos: 100_000_000,
		AcquireFailed: 5, AcquireFailedNanos: 500_000_000,
		Rejected: 2, Expired: 1, QuorumLost: 3,
	}
	nav := lockNav(t, &madmin.LockMetrics{
		// The first slot is empty, so it is neither a row nor a child.
		LastHour: lockWindow(60, madmin.LockSegment{N: 2}, busyLockSegment(), rare),
	})
	node, err := nav.Navigate("locks/last_hour")
	if err != nil {
		t.Fatalf("navigate locks/last_hour: %v", err)
	}

	got := childNames(node.GetChildren())
	if want := []string{"_ALL", "10:01Z", "10:02Z"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("children = %v, want %v", got, want)
	}
	for _, name := range got {
		if _, err := node.GetChild(name); err != nil {
			t.Errorf("GetChild(%q) = %v, want a node", name, err)
		}
	}

	data := node.GetLeafData()
	for _, key := range []string{
		"00:Total (last hour)", "01:Coverage",
		"02: 10:01Z", "03: 10:02Z",
	} {
		if _, ok := data[key]; !ok {
			t.Errorf("missing row %q in %v", key, data)
		}
	}
	if len(data) != 4 {
		t.Errorf("rows = %v, want exactly 4 (total + coverage + two non-empty segments)", data)
	}
	// The busy segment is the older of the two, so it is the first segment row.
	if got, want := data["02: 10:01Z"], "100 acqs (1.7/s), wait 20ms, held 10ms, rel 1ms"; got != want {
		t.Errorf("first segment row = %q, want %q", got, want)
	}

	// The mean wait is derived from AcquireNanos/AcquireCount, never sent.
	busy, err := nav.Navigate("locks/last_hour/10:01Z")
	if err != nil {
		t.Fatalf("navigate segment: %v", err)
	}
	bd := busy.GetLeafData()
	if got := leafValue(bd, "Mean Acquire Wait"); got != "20ms" {
		t.Errorf("Mean Acquire Wait = %q, want 20ms", got)
	}
	if got := leafValue(bd, "Grant Rate"); got != "1.67/s" {
		t.Errorf("Grant Rate = %q, want 1.67/s", got)
	}
	if got := leafValue(bd, "Mean Hold"); got != "10ms" {
		t.Errorf("Mean Hold = %q, want 10ms", got)
	}
	if got := leafValue(bd, "Nodes"); got != "2 node(s)" {
		t.Errorf("Nodes = %q, want 2 node(s)", got)
	}
	// A quiet segment names no rare event at all.
	for _, key := range []string{"Rejected", "Expired", "Quorum Lost", "Failed"} {
		if got := leafValue(bd, key); got != "" {
			t.Errorf("%s = %q on a clean segment, want no row", key, got)
		}
	}

	// The segment that had them names all three, plus the wasted waiting.
	bad, err := nav.Navigate("locks/last_hour/10:02Z")
	if err != nil {
		t.Fatalf("navigate segment: %v", err)
	}
	xd := bad.GetLeafData()
	for key, want := range map[string]string{
		"Rejected": "2", "Expired": "1", "Quorum Lost": "3",
		"Failed": "5", "Mean Failed Wait": "100ms", "Failure Share": "33.3%",
	} {
		if got := leafValue(xd, key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}

	// _ALL is the whole window as one segment: 110 grants over three minutes.
	all, err := nav.Navigate("locks/last_hour/_ALL")
	if err != nil {
		t.Fatalf("navigate _ALL: %v", err)
	}
	if got := leafValue(all.GetLeafData(), "Acquires"); got != "110" {
		t.Errorf("_ALL Acquires = %q, want 110", got)
	}
}

// A window the caller did not request must read as unavailable, an empty one as no
// data -- never as a row of real zeros.
func TestLockWindowUnavailable(t *testing.T) {
	nav := lockNav(t, &madmin.LockMetrics{
		LastDay: &madmin.SegmentedLockMetrics{Interval: 900, FirstTime: lockFirstTime},
	})

	hour, err := nav.Navigate("locks/last_hour")
	if err != nil {
		t.Fatalf("navigate locks/last_hour: %v", err)
	}
	if got := hour.GetLeafData()["Status"]; !strings.Contains(got, "not requested") {
		t.Errorf("nil window Status = %q, want it to say the stats were not requested", got)
	}
	if got := hour.GetChildren(); len(got) != 0 {
		t.Errorf("nil window children = %v, want none", childNames(got))
	}
	if _, err := hour.GetChild("_ALL"); err == nil {
		t.Error("GetChild on a nil window returned a node, want an error")
	}

	day, err := nav.Navigate("locks/last_day")
	if err != nil {
		t.Fatalf("navigate locks/last_day: %v", err)
	}
	if got, want := day.GetLeafData()["Status"], "no data yet: the last-day window holds no completed segment"; got != want {
		t.Errorf("empty window Status = %q, want %q", got, want)
	}

	// Segments that exist but recorded nothing are measured zeros: the totals are
	// real and stay, and only the missing segment rows are explained.
	nav = lockNav(t, &madmin.LockMetrics{LastDay: lockWindow(900, madmin.LockSegment{N: 4})})
	day, err = nav.Navigate("locks/last_day")
	if err != nil {
		t.Fatalf("navigate locks/last_day: %v", err)
	}
	idle := day.GetLeafData()
	if got := idle["Status"]; got != "" {
		t.Errorf("idle window Status = %q, want the measured totals instead", got)
	}
	if got, want := idle["00:Total (last day)"], "0 acqs, avg n/a"; got != want {
		t.Errorf("idle window total = %q, want the measured %q", got, want)
	}
	if got, want := idle["02:Segments"], "no locking recorded in any of the 1 segment(s) measured"; got != want {
		t.Errorf("idle window segment note = %q, want %q", got, want)
	}
	if got := childNames(day.GetChildren()); !reflect.DeepEqual(got, []string{"_ALL"}) {
		t.Errorf("idle window children = %v, want just _ALL", got)
	}
}

// The one-line summaries must keep the three window states apart: an absent
// window is not a measured zero, and neither is one that is still filling.
func TestFormatLockWindowStates(t *testing.T) {
	if got, want := formatLockWindow(nil), "not collected"; got != want {
		t.Errorf("nil window = %q, want %q", got, want)
	}
	if got, want := formatLockWindow(lockWindow(900)), "no data yet"; got != want {
		t.Errorf("empty window = %q, want %q", got, want)
	}
	if got, want := formatLockWindow(lockWindow(900, madmin.LockSegment{N: 4})), "0 acqs, avg n/a"; got != want {
		t.Errorf("idle window = %q, want the measured %q", got, want)
	}
}

// A whole-window label states the span it covers and the instant it ends: an
// interval as wide as the window would otherwise print one time twice.
func TestLockWindowCoverageLabel(t *testing.T) {
	// 95 quarter-hour slots, the shape a cross-node merge can produce.
	segments := make([]madmin.LockSegment, 95)
	segments[0] = busyLockSegment()
	nav := lockNav(t, &madmin.LockMetrics{LastDay: lockWindow(900, segments...)})
	node, err := nav.Navigate("locks/last_day")
	if err != nil {
		t.Fatalf("navigate locks/last_day: %v", err)
	}
	wantUTC := "Covering 23h45m, until Aug 13, 09:45Z"
	wantCoverage(t, "window coverage", node.GetLeafData()["01:Coverage"], wantUTC)
	for _, child := range node.GetChildren() {
		if child.Name != "_ALL" {
			continue
		}
		if !strings.Contains(child.Description, wantUTC) {
			t.Errorf("_ALL described as %q, want it to contain %q", child.Description, wantUTC)
		}
	}
	all, err := nav.Navigate("locks/last_day/_ALL")
	if err != nil {
		t.Fatalf("navigate _ALL: %v", err)
	}
	wantCoverage(t, "_ALL time segment", leafValue(all.GetLeafData(), "Time Segment"), wantUTC)
}

// A key is a bare wall-clock time, so a window that reaches 24 hours -- what
// Segmented.Add yields when two nodes' FirstTime differ by one interval -- has two
// slots wanting one name. The newest keeps it, and the list agrees with lookup.
func TestLockWindowDuplicateSegmentKeys(t *testing.T) {
	// 97 quarter-hour slots span 24h15m, so slot 0 and slot 96 are both 10:00Z.
	segments := make([]madmin.LockSegment, 97)
	segments[0] = madmin.LockSegment{N: 1, AcquireCount: 7, AcquireNanos: 7_000_000}
	segments[96] = madmin.LockSegment{N: 1, AcquireCount: 11, AcquireNanos: 11_000_000}
	nav := lockNav(t, &madmin.LockMetrics{LastDay: lockWindow(900, segments...)})
	node, err := nav.Navigate("locks/last_day")
	if err != nil {
		t.Fatalf("navigate locks/last_day: %v", err)
	}

	names := childNames(node.GetChildren())
	if want := []string{"_ALL", "10:00Z"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("children = %v, want %v (the older 10:00Z slot dropped)", names, want)
	}
	for _, name := range names {
		if _, err := node.GetChild(name); err != nil {
			t.Errorf("GetChild(%q) = %v, want a node", name, err)
		}
	}

	// The surviving segment is the newest: 11 acquires, not the older slot's 7.
	dup, err := node.GetChild("10:00Z")
	if err != nil {
		t.Fatalf("GetChild(10:00Z): %v", err)
	}
	if got := leafValue(dup.GetLeafData(), "Acquires"); got != "11" {
		t.Errorf("Acquires = %q, want 11 from the newest slot", got)
	}
	// And it is the newest by its label too: one interval short of a full extra day.
	wantCoverage(t, "duplicate-key Time Segment",
		leafValue(dup.GetLeafData(), "Time Segment"), "Covering 15m, until Aug 13, 10:15Z")
}

// The section refreshes continuously, so it must not pull the windows: it renders
// their summaries only when the history is already loaded.
func TestLockSectionRequestsNoHistory(t *testing.T) {
	nav := lockNav(t, &madmin.LockMetrics{
		LastHour: lockWindow(60, busyLockSegment()),
		LastDay:  &madmin.SegmentedLockMetrics{Interval: 900, FirstTime: lockFirstTime},
	})
	node, err := nav.Navigate("locks")
	if err != nil {
		t.Fatalf("navigate locks: %v", err)
	}
	if got := node.GetMetricFlags(); got != 0 {
		t.Errorf("section flags = %v, want none: the windows are fetched on entering one", got)
	}
	if opts := node.GetOpts(); opts.Flags != 0 {
		t.Errorf("section opts flags = %v, want none", opts.Flags)
	}
	data := node.GetLeafData()
	// Loaded history is summarized...
	if got, want := data["Last Hour"], "100 acqs, avg 20ms, held 10ms"; got != want {
		t.Errorf("Last Hour = %q, want %q", got, want)
	}
	// ...but a window with no segment is not, so it can never read as a zero.
	if got, ok := data["Last Day"]; ok {
		t.Errorf("Last Day = %q, want no row for a window holding no segment", got)
	}
}

// Every window node must request the window it renders: parent inheritance hides a
// wrong constant, so the node's own flags are what is asserted.
func TestLockWindowFlags(t *testing.T) {
	nav := lockNav(t, &madmin.LockMetrics{
		LastHour: lockWindow(60, busyLockSegment()),
		LastDay:  lockWindow(900, busyLockSegment()),
	})
	for _, tt := range []struct {
		path string
		want madmin.MetricFlags
	}{
		{"locks/last_hour", madmin.MetricsHourStats},
		{"locks/last_hour/10:00Z", madmin.MetricsHourStats},
		{"locks/last_hour/_ALL", madmin.MetricsHourStats},
		{"locks/last_day", madmin.MetricsDayStats},
		{"locks/last_day/10:00Z", madmin.MetricsDayStats},
		{"locks/last_day/_ALL", madmin.MetricsDayStats},
	} {
		node, err := nav.Navigate(tt.path)
		if err != nil {
			t.Fatalf("navigate %s: %v", tt.path, err)
		}
		if got := node.GetMetricFlags(); got != tt.want {
			t.Errorf("%s flags = %v, want exactly %v", tt.path, got, tt.want)
		}
		if opts := node.GetOpts(); !opts.Flags.Contains(tt.want) || opts.Type&madmin.MetricsLocks == 0 {
			t.Errorf("%s opts = %v/%v, want lock metrics with %v", tt.path, opts.Type, opts.Flags, tt.want)
		}
		if !node.ShouldPauseRefresh() {
			t.Errorf("%s does not pause refresh, want a paused time series", tt.path)
		}
	}
}

// wantCoverage asserts a coverage label's UTC part exactly and only that a local
// time follows it. The local rendering depends on the machine's zone, so pinning
// it would pass here and fail in CI, which runs in UTC.
func wantCoverage(t *testing.T, where, got, wantUTC string) {
	t.Helper()
	if !strings.HasPrefix(got, wantUTC+" (") || !strings.HasSuffix(got, ").") {
		t.Errorf("%s = %q, want %q followed by a parenthesised local time", where, got, wantUTC)
	}
}
