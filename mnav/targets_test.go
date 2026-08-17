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

func targetWindow(segments ...madmin.TargetSegment) *madmin.SegmentedTargetMetrics {
	return &madmin.SegmentedTargetMetrics{
		Interval:  60,
		FirstTime: time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC),
		Segments:  segments,
	}
}

// The batch factor and the mean latency are derived from the segment totals; the
// wire carries only sums.
func TestFormatTargetWindow(t *testing.T) {
	got := formatTargetWindow(targetWindow(
		madmin.TargetSegment{N: 1, Events: 60, Requests: 20, RequestNanos: 20_000_000},
		madmin.TargetSegment{N: 1, Events: 40, Requests: 10, RequestNanos: 10_000_000, DroppedShutdown: 3},
	))
	want := "100 events, 30 reqs (3.3 ea, avg 1ms), 3 dropped (shutdown)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// A healthy target names no failure at all rather than a wall of zeros.
	got = formatTargetWindow(targetWindow(
		madmin.TargetSegment{N: 1, Events: 5, Requests: 5, RequestNanos: 5_000},
	))
	if want = "5 events, 5 reqs (1.0 ea, avg 1µs)"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Errors and warnings are named at zero so a quiet cluster reads as quiet rather
// than as unreported; the other severities only appear when they happened.
func TestFormatLogVolumeWindow(t *testing.T) {
	window := func(segments ...madmin.LogVolumeSegment) *madmin.SegmentedLogVolume {
		return &madmin.SegmentedLogVolume{
			Interval:  60,
			FirstTime: time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC),
			Segments:  segments,
		}
	}

	got := formatLogVolumeWindow(window(
		madmin.LogVolumeSegment{N: 1, ErrorLines: 3, WarningLines: 10, InfoLines: 4},
		madmin.LogVolumeSegment{N: 1, ErrorLines: 1, EventLines: 2, FatalLines: 1},
	))
	want := "4 errors, 10 warnings, 1 fatal, 4 info, 2 events"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// Measured zeros are named; the two states that measured nothing are not.
	if got, want = formatLogVolumeWindow(window(madmin.LogVolumeSegment{N: 1})), "0 errors, 0 warnings"; got != want {
		t.Errorf("idle window = %q, want %q", got, want)
	}
	if got, want = formatLogVolumeWindow(window()), "no data yet"; got != want {
		t.Errorf("empty window = %q, want %q", got, want)
	}
	if got, want = formatLogVolumeWindow(nil), "not collected"; got != want {
		t.Errorf("nil window = %q, want %q", got, want)
	}
}

// The three window states must stay apart in the one-line summary too: an absent
// window is not a measured zero, and neither is one that is still filling.
func TestFormatTargetWindowStates(t *testing.T) {
	if got, want := formatTargetWindow(nil), "not collected"; got != want {
		t.Errorf("nil window = %q, want %q", got, want)
	}
	if got, want := formatTargetWindow(targetWindow()), "no data yet"; got != want {
		t.Errorf("empty window = %q, want %q", got, want)
	}
	if got, want := formatTargetWindow(targetWindow(madmin.TargetSegment{N: 1})), "0 events"; got != want {
		t.Errorf("idle window = %q, want the measured %q", got, want)
	}
}

// Drops are only in the windows, so the summary must total every reason -- and a
// window the caller did not request must contribute nothing.
func TestDroppedIn(t *testing.T) {
	got := droppedIn(targetWindow(madmin.TargetSegment{
		N: 1, DroppedQueueFull: 1, DroppedRetriesExhausted: 2,
		DroppedShutdown: 4, DroppedOther: 8,
	}))
	if got != 15 {
		t.Errorf("droppedIn = %d, want 15", got)
	}
	if got = droppedIn(nil); got != 0 {
		t.Errorf("droppedIn(nil) = %d, want 0", got)
	}
}

var targetFirstTime = time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)

// The day window is a coarser resolution, and the two must never share a map:
// Segmented.Add drops everything whose interval differs.
func targetDayWindow(segments ...madmin.TargetSegment) *madmin.SegmentedTargetMetrics {
	return &madmin.SegmentedTargetMetrics{
		Interval:  900,
		FirstTime: targetFirstTime,
		Segments:  segments,
	}
}

func logVolumeWindow(interval int, segments ...madmin.LogVolumeSegment) *madmin.SegmentedLogVolume {
	return &madmin.SegmentedLogVolume{
		Interval:  interval,
		FirstTime: targetFirstTime,
		Segments:  segments,
	}
}

// busyTargetSegment delivers 60 events in 20 requests taking 20ms in total, so the
// batch factor is 3.0 and the mean latency 1ms.
func busyTargetSegment() madmin.TargetSegment {
	return madmin.TargetSegment{N: 2, Events: 60, Requests: 20, RequestNanos: 20_000_000}
}

func targetNav(t *testing.T, targets *madmin.DeliveryTargetMetrics) MetricNavigator {
	t.Helper()
	return NewRealtimeMetricsNavigator(&madmin.RealtimeMetrics{
		Aggregated: madmin.Metrics{Targets: targets},
	})
}

// twoTargets is one notification and one audit target, each with both windows.
func twoTargets() *madmin.DeliveryTargetMetrics {
	return &madmin.DeliveryTargetMetrics{
		Nodes: 2,
		Notification: map[string]madmin.TargetMetrics{
			"notify_webhook:primary": {
				N: 2, Subsystem: "notify_webhook", Name: "primary", Type: "webhook",
				LastHour: targetWindow(busyTargetSegment()),
				LastDay:  targetDayWindow(busyTargetSegment()),
			},
		},
		Audit: map[string]madmin.TargetMetrics{
			"audit_kafka:1": {
				N: 2, Subsystem: "audit_kafka", Name: "1", Type: "kafka",
				LastHour: targetWindow(madmin.TargetSegment{N: 2, Events: 10, Requests: 1, RequestNanos: 5_000_000, DroppedQueueFull: 4, WriterErrors: 2}),
				LastDay:  targetDayWindow(madmin.TargetSegment{N: 2, Events: 10, Requests: 1, RequestNanos: 5_000_000}),
			},
		},
		LogVolumeLastHour: logVolumeWindow(60, madmin.LogVolumeSegment{N: 2, WarningLines: 4, InfoLines: 7}),
		LogVolumeLastDay:  logVolumeWindow(900, madmin.LogVolumeSegment{N: 2, ErrorLines: 3}),
	}
}

// The windows are listed before the targets, and everything listed resolves.
func TestTargetMetricsWindowChildren(t *testing.T) {
	nav := targetNav(t, twoTargets())
	node, err := nav.Navigate("targets")
	if err != nil {
		t.Fatalf("navigate targets: %v", err)
	}
	got := childNames(node.GetChildren())
	want := []string{
		"last_hour", "last_day", "log_volume_last_hour", "log_volume_last_day",
		"notify_webhook:primary", "audit_kafka:1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("children = %v, want %v", got, want)
	}
	for _, name := range got {
		if _, err := node.GetChild(name); err != nil {
			t.Errorf("GetChild(%q) = %v, want a node", name, err)
		}
	}

	// A target carries its own two windows, and both resolve.
	tgt, err := nav.Navigate("targets/notify_webhook:primary")
	if err != nil {
		t.Fatalf("navigate target: %v", err)
	}
	if got, want := childNames(tgt.GetChildren()), []string{"last_hour", "last_day"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("target children = %v, want %v", got, want)
	}
	for _, name := range []string{"last_hour", "last_day"} {
		if _, err := tgt.GetChild(name); err != nil {
			t.Errorf("target GetChild(%q) = %v, want a node", name, err)
		}
	}
}

// One resolution per view: the keyed child list is exactly the targets that have a
// window at that resolution, time-first entry included.
func TestTargetWindowsChildren(t *testing.T) {
	targets := twoTargets()
	// This target reported only the day window, so it must not appear in the hour
	// view -- mixing resolutions loses every series but the first.
	targets.Logs = map[string]madmin.TargetMetrics{
		"logger_webhook:main": {
			N: 1, Subsystem: "logger_webhook", Name: "main", Type: "webhook",
			LastDay: targetDayWindow(busyTargetSegment()),
		},
	}
	nav := targetNav(t, targets)

	for path, want := range map[string][]string{
		"targets/last_hour": {byTimeName, "_ALL", "audit_kafka:1", "notify_webhook:primary"},
		"targets/last_day":  {byTimeName, "_ALL", "audit_kafka:1", "logger_webhook:main", "notify_webhook:primary"},
	} {
		node, err := nav.Navigate(path)
		if err != nil {
			t.Fatalf("navigate %s: %v", path, err)
		}
		got := childNames(node.GetChildren())
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s children = %v, want %v", path, got, want)
		}
		for _, name := range got {
			if _, err := node.GetChild(name); err != nil {
				t.Errorf("%s GetChild(%q) = %v, want a node", path, name, err)
			}
		}
		// The class is labeled, so a reader never has to parse a key.
		for _, child := range node.GetChildren() {
			switch child.Name {
			case "audit_kafka:1":
				if !strings.HasPrefix(child.Description, "audit log: ") {
					t.Errorf("%s: audit child described as %q", path, child.Description)
				}
			case "logger_webhook:main":
				if !strings.HasPrefix(child.Description, "system log: ") {
					t.Errorf("%s: log child described as %q", path, child.Description)
				}
			case "notify_webhook:primary":
				if !strings.HasPrefix(child.Description, "notification: ") {
					t.Errorf("%s: notification child described as %q", path, child.Description)
				}
			}
		}
	}
}

// Rows carry the real span they cover, oldest first, and the derived figures are
// computed here -- the wire carries only sums.
func TestTargetWindowSegments(t *testing.T) {
	nav := targetNav(t, &madmin.DeliveryTargetMetrics{
		Nodes: 2,
		Notification: map[string]madmin.TargetMetrics{
			"notify_webhook:primary": {
				N: 2, Subsystem: "notify_webhook", Name: "primary", Type: "webhook",
				// The middle slot is idle, so it is neither a row nor a child.
				LastHour: targetWindow(
					busyTargetSegment(),
					madmin.TargetSegment{N: 2},
					madmin.TargetSegment{N: 2, Events: 5, Requests: 5, RequestNanos: 5_000_000, DroppedShutdown: 3, DroppedOther: 1},
				),
			},
		},
	})
	node, err := nav.Navigate("targets/notify_webhook:primary/last_hour")
	if err != nil {
		t.Fatalf("navigate window: %v", err)
	}

	got := childNames(node.GetChildren())
	if want := []string{"_ALL", "10:00Z", "10:02Z"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("children = %v, want %v", got, want)
	}
	for _, name := range got {
		if _, err := node.GetChild(name); err != nil {
			t.Errorf("GetChild(%q) = %v, want a node", name, err)
		}
	}

	data := node.GetLeafData()
	if len(data) != 4 {
		t.Errorf("rows = %v, want exactly 4 (total + coverage + two active segments)", data)
	}
	wantCoverage(t, "01:Coverage", data["01:Coverage"], "Covering 3m, until Aug 12, 10:03Z")
	if got, want := data["02: 10:00Z"], "60 events (1.0/s), 20 reqs (3.0 ea, avg 1ms)"; got != want {
		t.Errorf("first segment row = %q, want %q", got, want)
	}
	if got, want := data["03: 10:02Z"], "5 events (0.1/s), 5 reqs (1.0 ea, avg 1ms), 4 dropped"; got != want {
		t.Errorf("second segment row = %q, want %q", got, want)
	}

	// Batch factor and mean latency are derived per segment.
	seg, err := nav.Navigate("targets/notify_webhook:primary/last_hour/10:00Z")
	if err != nil {
		t.Fatalf("navigate segment: %v", err)
	}
	sd := seg.GetLeafData()
	for key, want := range map[string]string{
		"Events": "60", "Requests": "20", "Event Rate": "1.00/s",
		"Batch Factor": "3.0 events per request", "Mean Latency": "1ms",
		"Nodes": "2 node(s)", "Target": "notify_webhook:primary",
	} {
		if got := leafValue(sd, key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	// A clean segment names no failure at all.
	for _, key := range []string{"Writer Errors", "Dropped Total", "Dropped (shutdown)"} {
		if got := leafValue(sd, key); got != "" {
			t.Errorf("%s = %q on a clean segment, want no row", key, got)
		}
	}

	// The segment that dropped names each reason and their total.
	bad, err := nav.Navigate("targets/notify_webhook:primary/last_hour/10:02Z")
	if err != nil {
		t.Fatalf("navigate segment: %v", err)
	}
	bd := bad.GetLeafData()
	for key, want := range map[string]string{
		"Dropped (shutdown)": "3", "Dropped (other)": "1", "Dropped Total": "4",
	} {
		if got := leafValue(bd, key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}

	// _ALL is the whole window as one segment: 65 events across three minutes.
	all, err := nav.Navigate("targets/notify_webhook:primary/last_hour/_ALL")
	if err != nil {
		t.Fatalf("navigate _ALL: %v", err)
	}
	if got := leafValue(all.GetLeafData(), "Events"); got != "65" {
		t.Errorf("_ALL Events = %q, want 65", got)
	}
}

// A window the caller did not request must read as unavailable, an empty one as no
// data -- never as a row of real zeros.
func TestTargetWindowUnavailable(t *testing.T) {
	nav := targetNav(t, &madmin.DeliveryTargetMetrics{
		Nodes: 1,
		Notification: map[string]madmin.TargetMetrics{
			"notify_webhook:primary": {
				N: 1, Subsystem: "notify_webhook", Name: "primary", Type: "webhook",
				LastDay: &madmin.SegmentedTargetMetrics{Interval: 900, FirstTime: targetFirstTime},
			},
		},
	})

	// The target's own hour window was never populated.
	hour, err := nav.Navigate("targets/notify_webhook:primary/last_hour")
	if err != nil {
		t.Fatalf("navigate window: %v", err)
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

	day, err := nav.Navigate("targets/notify_webhook:primary/last_day")
	if err != nil {
		t.Fatalf("navigate window: %v", err)
	}
	if got, want := day.GetLeafData()["Status"], "no data yet: the last-day window holds no completed segment"; got != want {
		t.Errorf("empty window Status = %q, want %q", got, want)
	}

	// The grouping node has a configured target but no window to show for it.
	group, err := nav.Navigate("targets/last_hour")
	if err != nil {
		t.Fatalf("navigate targets/last_hour: %v", err)
	}
	if got := group.GetLeafData()["Status"]; !strings.Contains(got, "not requested") {
		t.Errorf("empty group Status = %q, want it to say the stats were not requested", got)
	}
	if got := group.GetChildren(); len(got) != 0 {
		t.Errorf("empty group children = %v, want none", childNames(got))
	}

	// The day window was requested for that target and came back empty, so the
	// target is still listed -- as still filling, never as a measured zero.
	dayGroup, err := nav.Navigate("targets/last_day")
	if err != nil {
		t.Fatalf("navigate targets/last_day: %v", err)
	}
	found := false
	for _, child := range dayGroup.GetChildren() {
		if child.Name != "notify_webhook:primary" {
			continue
		}
		found = true
		if want := "notification: no data yet"; child.Description != want {
			t.Errorf("empty day window described as %q, want %q", child.Description, want)
		}
	}
	if !found {
		t.Errorf("day group children = %v, want the target with the requested window",
			childNames(dayGroup.GetChildren()))
	}

	// With nothing configured at all, say so rather than blaming the flags.
	nav = targetNav(t, &madmin.DeliveryTargetMetrics{Nodes: 1})
	group, err = nav.Navigate("targets/last_day")
	if err != nil {
		t.Fatalf("navigate targets/last_day: %v", err)
	}
	if got, want := group.GetLeafData()["Status"], "no delivery targets configured"; got != want {
		t.Errorf("unconfigured group Status = %q, want %q", got, want)
	}

	// Log volume works with no target configured, so its own window is what decides.
	lv, err := nav.Navigate("targets/log_volume_last_hour")
	if err != nil {
		t.Fatalf("navigate log volume: %v", err)
	}
	if got := lv.GetLeafData()["Status"]; !strings.Contains(got, "not requested") {
		t.Errorf("nil log volume Status = %q, want it to say the stats were not requested", got)
	}
}

// Errors and warnings render at zero: "0 errors at 10:00" is the reassurance.
func TestLogVolumeWindowSegments(t *testing.T) {
	nav := targetNav(t, &madmin.DeliveryTargetMetrics{
		Nodes: 2,
		LogVolumeLastHour: logVolumeWindow(60,
			madmin.LogVolumeSegment{N: 2, InfoLines: 30},
			madmin.LogVolumeSegment{N: 2},
			madmin.LogVolumeSegment{N: 2, ErrorLines: 2, WarningLines: 5, FatalLines: 1, EventLines: 3},
		),
	})
	node, err := nav.Navigate("targets/log_volume_last_hour")
	if err != nil {
		t.Fatalf("navigate log volume: %v", err)
	}
	got := childNames(node.GetChildren())
	if want := []string{"_ALL", "10:00Z", "10:02Z"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("children = %v, want %v", got, want)
	}
	for _, name := range got {
		if _, err := node.GetChild(name); err != nil {
			t.Errorf("GetChild(%q) = %v, want a node", name, err)
		}
	}

	data := node.GetLeafData()
	if got, want := data["02: 10:00Z"], "0 errors, 0 warnings, 30 info, 30.0 lines/min"; got != want {
		t.Errorf("first segment row = %q, want %q", got, want)
	}
	if got, want := data["03: 10:02Z"], "2 errors, 5 warnings, 1 fatal, 3 events, 11.0 lines/min"; got != want {
		t.Errorf("second segment row = %q, want %q", got, want)
	}

	// The quiet segment still names both severities, at zero.
	quiet, err := nav.Navigate("targets/log_volume_last_hour/10:00Z")
	if err != nil {
		t.Fatalf("navigate segment: %v", err)
	}
	qd := quiet.GetLeafData()
	for key, want := range map[string]string{
		"Errors": "0", "Warnings": "0", "Info": "30", "Total Lines": "30",
		"Line Rate": "30.00/min", "Nodes": "2 node(s)",
	} {
		if got := leafValue(qd, key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	for _, key := range []string{"Fatal", "Events"} {
		if got := leafValue(qd, key); got != "" {
			t.Errorf("%s = %q on a segment that logged none, want no row", key, got)
		}
	}

	// _ALL totals the window: 30 + 11 lines.
	all, err := nav.Navigate("targets/log_volume_last_hour/_ALL")
	if err != nil {
		t.Fatalf("navigate _ALL: %v", err)
	}
	if got := leafValue(all.GetLeafData(), "Total Lines"); got != "41" {
		t.Errorf("_ALL Total Lines = %q, want 41", got)
	}
}

// The section and each target refresh continuously, so neither may pull the
// windows: they render summaries only when the history is already loaded.
func TestTargetSectionRequestsNoHistory(t *testing.T) {
	targets := twoTargets()
	// Requested and returned empty: present, but nothing to summarize.
	tm := targets.Notification["notify_webhook:primary"]
	tm.LastDay = &madmin.SegmentedTargetMetrics{Interval: 900, FirstTime: targetFirstTime}
	targets.Notification["notify_webhook:primary"] = tm
	nav := targetNav(t, targets)

	for _, path := range []string{"targets", "targets/notify_webhook:primary"} {
		node, err := nav.Navigate(path)
		if err != nil {
			t.Fatalf("navigate %s: %v", path, err)
		}
		if got := node.GetMetricFlags(); got != 0 {
			t.Errorf("%s flags = %v, want none: the windows are fetched on entering one", path, got)
		}
		if opts := node.GetOpts(); opts.Flags != 0 {
			t.Errorf("%s opts flags = %v, want none", path, opts.Flags)
		}
	}

	tgt, err := nav.Navigate("targets/notify_webhook:primary")
	if err != nil {
		t.Fatalf("navigate target: %v", err)
	}
	data := tgt.GetLeafData()
	if got, want := data["Last Hour"], "60 events, 20 reqs (3.0 ea, avg 1ms)"; got != want {
		t.Errorf("Last Hour = %q, want %q", got, want)
	}
	if got, ok := data["Last Day"]; ok {
		t.Errorf("Last Day = %q, want no row for a window holding no segment", got)
	}

	section, err := nav.Navigate("targets")
	if err != nil {
		t.Fatalf("navigate targets: %v", err)
	}
	sd := section.GetLeafData()
	if got, want := sd["Logged (last hour)"], "0 errors, 4 warnings, 7 info"; got != want {
		t.Errorf("Logged (last hour) = %q, want %q", got, want)
	}
	// Drops live only in the windows, and the hour window here has 4 of them.
	if got, want := sd["Dropped Events (last hour)"], "4"; got != want {
		t.Errorf("Dropped Events (last hour) = %q, want %q", got, want)
	}
}

// A key is only unique within its class map, so the section listing must resolve a
// collision the same way GetChild does: first class wins, listed once.
func TestTargetSectionDuplicateKeys(t *testing.T) {
	targets := twoTargets()
	shared := "notify_webhook:primary"
	targets.Audit[shared] = madmin.TargetMetrics{
		N: 1, Subsystem: "notify_webhook", Name: "primary", Type: "kafka",
	}
	node, err := targetNav(t, targets).Navigate("targets")
	if err != nil {
		t.Fatalf("navigate targets: %v", err)
	}
	seen := 0
	for _, child := range node.GetChildren() {
		if child.Name != shared {
			continue
		}
		seen++
		if want := "webhook notification target on 2 node(s)"; child.Description != want {
			t.Errorf("shared key described as %q, want the notification class %q", child.Description, want)
		}
	}
	if seen != 1 {
		t.Errorf("shared key listed %d times, want exactly once", seen)
	}
	// And that one entry is what GetChild resolves.
	child, err := node.GetChild(shared)
	if err != nil {
		t.Fatalf("GetChild(%q): %v", shared, err)
	}
	if got := leafValue(child.GetLeafData(), "Type"); got != "webhook" {
		t.Errorf("resolved type = %q, want the notification target's webhook", got)
	}
}

// A key is a bare wall-clock time, so a window that reaches 24 hours -- what
// Segmented.Add yields when two nodes' FirstTime differ by one interval -- has two
// slots wanting one name. The newest keeps it, and the list agrees with lookup.
func TestTargetWindowDuplicateSegmentKeys(t *testing.T) {
	// 97 quarter-hour slots span 24h15m, so slot 0 and slot 96 are both 10:00Z.
	segments := make([]madmin.TargetSegment, 97)
	segments[0] = madmin.TargetSegment{N: 1, Events: 7}
	segments[96] = madmin.TargetSegment{N: 1, Events: 11}
	nav := targetNav(t, &madmin.DeliveryTargetMetrics{
		Nodes: 1,
		Notification: map[string]madmin.TargetMetrics{
			"notify_webhook:primary": {
				N: 1, Subsystem: "notify_webhook", Name: "primary", Type: "webhook",
				LastDay: targetDayWindow(segments...),
			},
		},
	})

	for _, path := range []string{
		"targets/notify_webhook:primary/last_day",
		"targets/last_day/notify_webhook:primary",
	} {
		node, err := nav.Navigate(path)
		if err != nil {
			t.Fatalf("navigate %s: %v", path, err)
		}
		names := childNames(node.GetChildren())
		if want := []string{"_ALL", "10:00Z"}; !reflect.DeepEqual(names, want) {
			t.Fatalf("%s children = %v, want %v (the older 10:00Z slot dropped)", path, names, want)
		}
		for _, name := range names {
			if _, err := node.GetChild(name); err != nil {
				t.Errorf("%s GetChild(%q) = %v, want a node", path, name, err)
			}
		}
		dup, err := node.GetChild("10:00Z")
		if err != nil {
			t.Fatalf("%s GetChild(10:00Z): %v", path, err)
		}
		if got := leafValue(dup.GetLeafData(), "Events"); got != "11" {
			t.Errorf("%s Events = %q, want 11 from the newest slot", path, got)
		}
		wantCoverage(t, "duplicate-key Time Segment",
			leafValue(dup.GetLeafData(), "Time Segment"), "Covering 15m, until Aug 13, 10:15Z")
	}

	// The time-first navigation shares the key format, so it resolves the same way.
	byTime, err := nav.Navigate("targets/last_day/" + byTimeName)
	if err != nil {
		t.Fatalf("navigate by time: %v", err)
	}
	names := childNames(byTime.GetChildren())
	if want := []string{"10:00Z"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("by-time children = %v, want %v", names, want)
	}
	seg, err := byTime.GetChild("10:00Z")
	if err != nil {
		t.Fatalf("by-time GetChild(10:00Z): %v", err)
	}
	if got := leafValue(seg.GetLeafData(), "Events"); got != "11" {
		t.Errorf("by-time Events = %q, want 11 from the newest slot", got)
	}
}

// A total that only restates one reason is the same number under two labels, which
// the API forbids; with two reasons it is new information.
func TestTargetSegmentDropTotal(t *testing.T) {
	segLeaf := func(s madmin.TargetSegment) map[string]string {
		return (&targetSegmentLeafNode{seg: s, segTime: targetFirstTime, interval: 60}).GetLeafData()
	}
	one := segLeaf(madmin.TargetSegment{N: 1, Events: 5, DroppedQueueFull: 3})
	if got, want := leafValue(one, "Dropped (queue full)"), "3"; got != want {
		t.Errorf("Dropped (queue full) = %q, want %q", got, want)
	}
	if got := leafValue(one, "Dropped Total"); got != "" {
		t.Errorf("Dropped Total = %q, want no row when it restates one reason", got)
	}
	two := segLeaf(madmin.TargetSegment{N: 1, Events: 5, DroppedQueueFull: 3, DroppedShutdown: 1})
	if got, want := leafValue(two, "Dropped Total"), "4"; got != want {
		t.Errorf("Dropped Total = %q, want %q", got, want)
	}
}

// Every node must request the window it renders: parent inheritance hides a wrong
// constant, so the node's own flags are what is asserted.
func TestTargetWindowFlags(t *testing.T) {
	nav := targetNav(t, twoTargets())
	key := "notify_webhook:primary"
	for _, tt := range []struct {
		path string
		want madmin.MetricFlags
	}{
		{"targets/last_hour", madmin.MetricsHourStats},
		{"targets/last_hour/" + byTimeName, madmin.MetricsHourStats},
		{"targets/last_hour/" + byTimeName + "/10:00Z", madmin.MetricsHourStats},
		{"targets/last_hour/" + byTimeName + "/10:00Z/" + key, madmin.MetricsHourStats},
		{"targets/last_hour/" + byTimeName + "/10:00Z/_ALL", madmin.MetricsHourStats},
		{"targets/last_hour/_ALL", madmin.MetricsHourStats},
		{"targets/last_hour/" + key, madmin.MetricsHourStats},
		{"targets/last_hour/" + key + "/10:00Z", madmin.MetricsHourStats},
		{"targets/" + key + "/last_hour", madmin.MetricsHourStats},
		{"targets/log_volume_last_hour", madmin.MetricsHourStats},
		{"targets/log_volume_last_hour/10:00Z", madmin.MetricsHourStats},
		{"targets/last_day", madmin.MetricsDayStats},
		{"targets/last_day/" + byTimeName, madmin.MetricsDayStats},
		{"targets/last_day/" + byTimeName + "/10:00Z", madmin.MetricsDayStats},
		{"targets/last_day/" + byTimeName + "/10:00Z/" + key, madmin.MetricsDayStats},
		{"targets/last_day/_ALL", madmin.MetricsDayStats},
		{"targets/last_day/" + key, madmin.MetricsDayStats},
		{"targets/last_day/" + key + "/10:00Z", madmin.MetricsDayStats},
		{"targets/" + key + "/last_day", madmin.MetricsDayStats},
		{"targets/log_volume_last_day", madmin.MetricsDayStats},
		{"targets/log_volume_last_day/10:00Z", madmin.MetricsDayStats},
	} {
		node, err := nav.Navigate(tt.path)
		if err != nil {
			t.Fatalf("navigate %s: %v", tt.path, err)
		}
		if got := node.GetMetricFlags(); got != tt.want {
			t.Errorf("%s flags = %v, want exactly %v", tt.path, got, tt.want)
		}
		if opts := node.GetOpts(); !opts.Flags.Contains(tt.want) || opts.Type&madmin.MetricsTargets == 0 {
			t.Errorf("%s opts = %v/%v, want target metrics with %v", tt.path, opts.Type, opts.Flags, tt.want)
		}
		if !node.ShouldPauseRefresh() {
			t.Errorf("%s does not pause refresh, want a paused time series", tt.path)
		}
	}
}
