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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/minio/madmin-go/v4"
)

var iamFirstTime = time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)

func iamSegWindow(interval int, segments ...madmin.IAMSegment) *madmin.SegmentedIAMMetrics {
	return &madmin.SegmentedIAMMetrics{
		Interval:  interval,
		FirstTime: iamFirstTime,
		Segments:  segments,
	}
}

// A busy segment: 600 regular-user authorizations costing 12ms in total, so the
// mean is 20µs.
func busyIAMSegment() madmin.IAMSegment {
	return madmin.IAMSegment{
		N: 2, UserCount: 600, UserNanos: 12_000_000, MaxNanos: 3_000_000,
	}
}

func iamNav(t *testing.T, iam *madmin.IAMMetrics) MetricNavigator {
	t.Helper()
	return NewRealtimeMetricsNavigator(&madmin.RealtimeMetrics{
		Aggregated: madmin.Metrics{IAM: iam},
	})
}

// Both windows are listed whatever state they are in, and everything listed
// resolves: their nodes are what explain a window that is missing or empty.
func TestIAMChildren(t *testing.T) {
	nav := iamNav(t, &madmin.IAMMetrics{})
	node, err := nav.Navigate("iam")
	if err != nil {
		t.Fatalf("navigate iam: %v", err)
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
}

// The section itself must not request the windows: it refreshes continuously, and
// asking here would pull 156 segments a tick to render two summary lines. The
// window nodes ask, and they pause the refresh while the reader is inside.
func TestIAMSectionDoesNotRequestWindows(t *testing.T) {
	nav := iamNav(t, &madmin.IAMMetrics{})
	node, err := nav.Navigate("iam")
	if err != nil {
		t.Fatalf("navigate iam: %v", err)
	}
	if flags := node.GetMetricFlags(); flags != 0 {
		t.Errorf("section flags = %v, want none", flags)
	}
	if node.ShouldPauseRefresh() {
		t.Error("the live section must keep refreshing")
	}

	for name, want := range map[string]madmin.MetricFlags{
		"last_hour": madmin.MetricsHourStats,
		"last_day":  madmin.MetricsDayStats,
	} {
		child, err := nav.Navigate("iam/" + name)
		if err != nil {
			t.Fatalf("navigate iam/%s: %v", name, err)
		}
		if got := child.GetOpts().Flags; !got.Contains(want) {
			t.Errorf("iam/%s requests %v, want it to include %v", name, got, want)
		}
		if !child.ShouldPauseRefresh() {
			t.Errorf("iam/%s must pause the refresh while it is being read", name)
		}
	}
}

// History that happens to be loaded is summarized on the live section -- it is
// persisted and restored on startup -- but a window that was never collected must
// never be rendered as a measured zero.
func TestIAMSectionSummarizesLoadedHistory(t *testing.T) {
	nav := iamNav(t, &madmin.IAMMetrics{
		LastHour: iamSegWindow(60, busyIAMSegment()),
	})
	node, err := nav.Navigate("iam")
	if err != nil {
		t.Fatalf("navigate iam: %v", err)
	}
	data := node.GetLeafData()
	if got, want := data["Last Hour"], "600 authz, avg 20µs, user, max 3ms"; got != want {
		t.Errorf("Last Hour = %q, want %q", got, want)
	}
	if _, ok := data["Last Day"]; ok {
		t.Errorf("Last Day rendered although it was never collected: %q", data["Last Day"])
	}
}

// The live section splits authorization by path, because the paths differ in cost
// by orders of magnitude. A path nobody took is absent, not zero.
func TestIAMSectionAuthByPath(t *testing.T) {
	nav := iamNav(t, &madmin.IAMMetrics{
		Auth: &madmin.IAMAuthStats{
			ByPath: map[string]madmin.TimedAction{
				madmin.IAMPathUser: {Count: 10, AccTime: 200_000, MaxTime: 50_000},
				madmin.IAMPathSTS:  {Count: 2, AccTime: 400_000, MaxTime: 300_000},
			},
			Denied: 3, Errors: 1, CacheMiss: 4,
		},
	})
	node, err := nav.Navigate("iam")
	if err != nil {
		t.Fatalf("navigate iam: %v", err)
	}
	data := node.GetLeafData()
	if got, want := data["Authz user"], "10, avg 20µs, max 50µs"; got != want {
		t.Errorf("Authz user = %q, want %q", got, want)
	}
	if got, want := data["Authz sts"], "2, avg 200µs, max 300µs"; got != want {
		t.Errorf("Authz sts = %q, want %q", got, want)
	}
	if _, ok := data["Authz plugin"]; ok {
		t.Error("a path nobody took was rendered")
	}
	for key, want := range map[string]string{
		"Denied": "3", "Unresolved": "1", "Policy Cache Miss": "4",
	} {
		if got := data[key]; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

// Rows carry the segment's own start, oldest first, and every listed child
// resolves to that segment's leaf.
func TestIAMWindowSegments(t *testing.T) {
	mixed := madmin.IAMSegment{
		N: 2,
		// Two paths, so the busiest is named with its share.
		UserCount: 30, UserNanos: 600_000,
		STSCount: 10, STSNanos: 1_000_000,
		MaxNanos: 400_000,
		Denied:   2, Errors: 1, CacheMiss: 5,
		LoadCount: 1, LoadNanos: 5_000_000, StoreErrors: 1,
	}
	nav := iamNav(t, &madmin.IAMMetrics{
		// The first slot is empty, so it is neither a row nor a child.
		LastHour: iamSegWindow(60, madmin.IAMSegment{N: 2}, busyIAMSegment(), mixed),
	})
	node, err := nav.Navigate("iam/last_hour")
	if err != nil {
		t.Fatalf("navigate iam/last_hour: %v", err)
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
		"00:Total (last hour)", "01:Coverage", "02: 10:01Z", "03: 10:02Z",
	} {
		if _, ok := data[key]; !ok {
			t.Errorf("missing row %q in %v", key, data)
		}
	}
	if len(data) != 4 {
		t.Errorf("rows = %v, want exactly 4 (total + coverage + two non-empty segments)", data)
	}
	if got, want := data["02: 10:01Z"], "600 authz (10.0/s), avg 20µs, user, max 3ms"; got != want {
		t.Errorf("busy segment row = %q, want %q", got, want)
	}
	// The mixed segment names the busiest path with its share, and every unusual
	// count it carries.
	mixedRow := data["03: 10:02Z"]
	for _, want := range []string{
		"40 authz (0.7/s)", "avg 40µs", "user 75.0%", "max 400µs",
		"2 denied", "1 unresolved", "1 store (avg 5ms)", "1 store err",
	} {
		if !strings.Contains(mixedRow, want) {
			t.Errorf("mixed segment row %q is missing %q", mixedRow, want)
		}
	}
}

// The leaf is where the whole path split lives, with each path's share of the
// segment.
func TestIAMSegmentLeaf(t *testing.T) {
	nav := iamNav(t, &madmin.IAMMetrics{
		LastHour: iamSegWindow(60, madmin.IAMSegment{
			N:         3,
			UserCount: 30, UserNanos: 600_000,
			OwnerCount: 10, OwnerNanos: 10_000,
			MaxNanos: 400_000,
			Denied:   2, Errors: 4, CacheMiss: 6,
			SaveCount: 2, SaveNanos: 8_000_000, StoreErrors: 1,
		}),
	})
	leaf, err := nav.Navigate("iam/last_hour/10:00Z")
	if err != nil {
		t.Fatalf("navigate segment: %v", err)
	}
	d := leaf.GetLeafData()

	// Every mean and every share is derived here; the wire carries counts and sums.
	for key, want := range map[string]string{
		"Authorizations":    "40",
		"Rate":              "0.67/s",
		"Mean Latency":      "15µs",
		"Slowest":           "400µs",
		"Path user":         "30 (75.0%), avg 20µs",
		"Path owner":        "10 (25.0%), avg 1µs",
		"Denied":            "2 (5.0%)",
		"Unresolved":        "4",
		"Policy Cache Miss": "6 of 30 user authz",
		"Store save":        "2, avg 4ms",
		"Store Errors":      "1",
		"Nodes":             "3 node(s)",
	} {
		if got := leafValue(d, key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	// A path nobody took gets no row.
	for _, key := range []string{"Path plugin", "Path sts", "Path svcacct", "Store load"} {
		if got := leafValue(d, key); got != "" {
			t.Errorf("%s = %q, want no row", key, got)
		}
	}
}

// A window nobody asked for and a window that is still filling are opposite
// answers, and neither may read as a measured zero.
func TestIAMWindowStates(t *testing.T) {
	notRequested, err := iamNav(t, &madmin.IAMMetrics{}).Navigate("iam/last_day")
	if err != nil {
		t.Fatalf("navigate iam/last_day: %v", err)
	}
	if got := notRequested.GetLeafData()["Status"]; !strings.Contains(got, "not requested") {
		t.Errorf("Status = %q, want it to say the window was not requested", got)
	}
	if len(notRequested.GetChildren()) != 0 {
		t.Error("a window with no data listed children")
	}
	if _, err := notRequested.GetChild("_ALL"); err == nil {
		t.Error("GetChild succeeded on a window with no segments")
	}

	requestedEmpty, err := iamNav(t, &madmin.IAMMetrics{
		LastDay: &madmin.SegmentedIAMMetrics{Interval: 900},
	}).Navigate("iam/last_day")
	if err != nil {
		t.Fatalf("navigate iam/last_day: %v", err)
	}
	if got := requestedEmpty.GetLeafData()["Status"]; !strings.Contains(got, "no data yet") {
		t.Errorf("Status = %q, want it to say the window is still filling", got)
	}
}

// A window whose every segment is empty still reports its measured totals, and
// says why there is no segment row rather than showing none.
func TestIAMWindowAllSegmentsEmpty(t *testing.T) {
	nav := iamNav(t, &madmin.IAMMetrics{
		LastDay: iamSegWindow(900, madmin.IAMSegment{N: 2}, madmin.IAMSegment{N: 2}),
	})
	node, err := nav.Navigate("iam/last_day")
	if err != nil {
		t.Fatalf("navigate iam/last_day: %v", err)
	}
	data := node.GetLeafData()
	if got, want := data["00:Total (last day)"], "0 authz"; got != want {
		t.Errorf("total = %q, want %q", got, want)
	}
	if got := data["02:Segments"]; !strings.Contains(got, "2 segment(s)") {
		t.Errorf("Segments = %q, want it to name how many segments were measured", got)
	}
	if names := childNames(node.GetChildren()); !reflect.DeepEqual(names, []string{"_ALL"}) {
		t.Errorf("children = %v, want only _ALL", names)
	}
}

// Keys collide once a merged window runs past 24 hours. The newest slot wins, so
// every key GetChildren lists resolves to the segment it described.
func TestIAMWindowDuplicateKeys(t *testing.T) {
	full := make([]madmin.IAMSegment, 97)
	for i := range full {
		full[i] = madmin.IAMSegment{N: 1, OwnerCount: uint64(i)}
	}
	nav := iamNav(t, &madmin.IAMMetrics{LastDay: iamSegWindow(900, full...)})
	node, err := nav.Navigate("iam/last_day")
	if err != nil {
		t.Fatalf("navigate iam/last_day: %v", err)
	}
	for _, name := range childNames(node.GetChildren()) {
		if _, err := node.GetChild(name); err != nil {
			t.Errorf("GetChild(%q) = %v: a listed key must resolve", name, err)
		}
	}
	// The first and last slot share a wall-clock time; the newest owns it.
	dup, err := node.GetChild("10:00Z")
	if err != nil {
		t.Fatalf("GetChild(10:00Z): %v", err)
	}
	if got := leafValue(dup.GetLeafData(), "Path owner"); !strings.HasPrefix(got, "96 ") {
		t.Errorf("Path owner = %q, want the newest slot's 96", got)
	}
}
