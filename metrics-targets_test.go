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
	"slices"
	"testing"
	"time"
)

func TestTargetsWireValuesStable(t *testing.T) {
	if MetricsTargets != 1<<18 {
		t.Errorf("MetricsTargets = %d, want %d", MetricsTargets, 1<<18)
	}
	if MetricsDistJobs != 1<<17 || MetricsTablesAPI != 1<<16 {
		t.Error("an existing MetricType bit moved")
	}
}

func TestTargetMetricsAdd(t *testing.T) {
	t0 := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)

	a := &TargetMetrics{
		N: 1, Subsystem: "notify_webhook", Name: "primary", Type: "webhook",
		NodesOnline:   1,
		QueueLength:   10,
		QueueCapacity: 100_000,
		LastMinute:    TimedAction{Count: 5, AccTime: 500, MinTime: 50, MaxTime: 200},
		LastError:     "older", LastErrorTime: t0,
		LastHour: targetWindow(t0, TargetSegment{
			N: 1, Events: 100, Requests: 20, RequestNanos: 2000,
			WriterErrors: 1, DroppedQueueFull: 2,
		}),
	}
	b := &TargetMetrics{
		N: 1, Subsystem: "notify_webhook", Name: "primary", Type: "webhook",
		NodesOnline: 0, NodesChecking: 1,
		QueueLength:   90_000,
		QueueCapacity: 100_000,
		LastMinute:    TimedAction{Count: 3, AccTime: 900, MinTime: 300, MaxTime: 400},
		LastError:     "newer", LastErrorTime: t0.Add(time.Minute),
		LastHour: targetWindow(t0, TargetSegment{
			N: 1, Events: 50, Requests: 10, RequestNanos: 3000,
			WriterErrors:     2,
			DroppedQueueFull: 1, DroppedRetriesExhausted: 3,
		}),
	}

	// Add mutates the receiver's windows in place by design, so a shallow copy of
	// the input would alias them. mergeMap never does that in production: it
	// starts from the zero value, and merging into an empty window copies the
	// source's segments -- see TestTargetMetricsAddDoesNotMutateArgument.
	got := cloneTargetMetrics(a)
	got.Add(b)

	// Readiness as node counts: "online on 1 of 2 nodes".
	if got.N != 2 || got.NodesOnline != 1 || got.NodesChecking != 1 {
		t.Errorf("readiness = %+v, want N=2 online=1 checking=1", got)
	}
	// Flow lives only in the windows, and every segment field is a plain sum.
	if tot := got.LastHour.Total(); tot != (TargetSegment{
		N: 2, Events: 150, Requests: 30, RequestNanos: 5000,
		WriterErrors:     3,
		DroppedQueueFull: 3, DroppedRetriesExhausted: 3,
	}) {
		t.Errorf("LastHour total = %+v", tot)
	}
	// Both sum, so saturation is correct at either scope.
	if got.QueueLength != 90_010 || got.QueueCapacity != 200_000 {
		t.Errorf("queue = %d/%d", got.QueueLength, got.QueueCapacity)
	}
	// TimedAction carries the extremes, so the slow node stays visible.
	if got.LastMinute.Count != 8 || got.LastMinute.MinTime != 50 || got.LastMinute.MaxTime != 400 {
		t.Errorf("LastMinute = %+v", got.LastMinute)
	}
	if got.LastError != "newer" {
		t.Errorf("LastError = %q, want the most recent failure", got.LastError)
	}

	// Order independence: collectRealtimeMetrics merges remote then local.
	rev := cloneTargetMetrics(b)
	rev.Add(a)
	if rev.LastError != got.LastError || rev.LastMinute != got.LastMinute ||
		rev.N != got.N || rev.LastHour.Total() != got.LastHour.Total() {
		t.Errorf("order dependent:\n%+v\n%+v", rev, got)
	}
}

// targetWindow is one node's report: a single segment on the minute grid.
func targetWindow(first time.Time, seg TargetSegment) *SegmentedTargetMetrics {
	return &SegmentedTargetMetrics{
		Interval:  60,
		FirstTime: first,
		Segments:  []TargetSegment{seg},
	}
}

// logVolumeWindow is one node's log-volume report: a single segment on the minute
// grid.
func logVolumeWindow(first time.Time, seg LogVolumeSegment) *SegmentedLogVolume {
	return &SegmentedLogVolume{
		Interval:  60,
		FirstTime: first,
		Segments:  []LogVolumeSegment{seg},
	}
}

func cloneTargetMetrics(t *TargetMetrics) TargetMetrics {
	out := *t
	out.LastHour = cloneTargetWindow(t.LastHour)
	out.LastDay = cloneTargetWindow(t.LastDay)
	return out
}

func cloneTargetWindow(w *SegmentedTargetMetrics) *SegmentedTargetMetrics {
	if w == nil {
		return nil
	}
	out := *w
	out.Segments = slices.Clone(w.Segments)
	return &out
}

// Merge must never write through into the value it is given: the caller's copy is
// another node's report, and mergeMap relies on Add being side-effect free on its
// argument.
func TestTargetMetricsAddDoesNotMutateArgument(t *testing.T) {
	t0 := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	src := &TargetMetrics{
		N:        1,
		LastHour: targetWindow(t0, TargetSegment{N: 1, Events: 5, DroppedQueueFull: 2}),
	}
	want := src.LastHour.Total()

	dst := TargetMetrics{}
	dst.Add(src)
	dst.Add(src)

	if got := src.LastHour.Total(); got != want {
		t.Errorf("argument mutated: %+v, want %+v", got, want)
	}
	if got := dst.LastHour.Total(); got.Events != 10 || got.DroppedQueueFull != 4 {
		t.Errorf("dst = %+v, want 10 events and 4 queue_full drops after two merges", got)
	}
}

// The same guard one level up: merging a map of targets must not write into the
// source maps.
func TestDeliveryTargetMetricsMergeDoesNotMutateArgument(t *testing.T) {
	t0 := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	src := &DeliveryTargetMetrics{
		Nodes: 1,
		Notification: map[string]TargetMetrics{
			"notify_webhook:p": {
				N:        1,
				LastHour: targetWindow(t0, TargetSegment{N: 1, Events: 3, DroppedShutdown: 1}),
			},
		},
		LogVolumeLastHour: logVolumeWindow(t0, LogVolumeSegment{N: 1, ErrorLines: 2}),
	}
	wantWindow := src.Notification["notify_webhook:p"].LastHour.Total()
	wantVolume := src.LogVolumeLastHour.Total()

	var dst DeliveryTargetMetrics
	dst.Merge(src)
	dst.Merge(src)

	if got := src.Notification["notify_webhook:p"].LastHour.Total(); got != wantWindow {
		t.Errorf("source target window mutated: %+v, want %+v", got, wantWindow)
	}
	if got := src.LogVolumeLastHour.Total(); got != wantVolume {
		t.Errorf("source log volume mutated: %+v, want %+v", got, wantVolume)
	}
	if got := dst.LogVolumeLastHour.Total(); got != (LogVolumeSegment{N: 2, ErrorLines: 4}) {
		t.Errorf("dst log volume = %+v, want 4 error lines from 2 nodes", got)
	}
	if got := dst.Notification["notify_webhook:p"].LastHour.Total(); got.Events != 6 || got.DroppedShutdown != 2 {
		t.Errorf("dst = %+v, want events 6 and 2 shutdown drops", got)
	}
}

// Identity strings are config echoes: the first reporting node supplies them and
// a later one must not blank them.
func TestTargetMetricsAddKeepsIdentity(t *testing.T) {
	got := TargetMetrics{}
	got.Add(&TargetMetrics{N: 1, Subsystem: "audit_kafka", Name: "1", Type: "kafka", Endpoint: "k:9092"})
	got.Add(&TargetMetrics{N: 1})

	if got.Subsystem != "audit_kafka" || got.Name != "1" || got.Type != "kafka" || got.Endpoint != "k:9092" {
		t.Errorf("identity = %+v, want preserved", got)
	}
}

// The three classes stay in separate maps because a target name is only unique
// within its config subsystem.
func TestDeliveryTargetMetricsMerge(t *testing.T) {
	t0 := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	target := func(events uint64) TargetMetrics {
		return TargetMetrics{N: 1, LastHour: targetWindow(t0, TargetSegment{N: 1, Events: events})}
	}

	a := &DeliveryTargetMetrics{
		Nodes:        1,
		Notification: map[string]TargetMetrics{"notify_webhook:p": target(10)},
		Audit:        map[string]TargetMetrics{"audit_kafka:1": target(5)},
		Spill:        &TargetSpillStats{Bytes: 100, Files: 1},

		LogVolumeLastHour: logVolumeWindow(t0, LogVolumeSegment{N: 1, ErrorLines: 3}),
	}
	b := &DeliveryTargetMetrics{
		Nodes:        1,
		Notification: map[string]TargetMetrics{"notify_webhook:p": target(7)},
		Logs:         map[string]TargetMetrics{"logger_webhook:main": target(2)},
		Spill:        &TargetSpillStats{Bytes: 50, Files: 2},

		LogVolumeLastHour: logVolumeWindow(t0, LogVolumeSegment{N: 1, ErrorLines: 1, WarningLines: 4}),
		LogVolumeLastDay:  logVolumeWindow(t0, LogVolumeSegment{N: 1, FatalLines: 1}),
	}

	var got DeliveryTargetMetrics
	got.Merge(a)
	got.Merge(b)

	if got.Nodes != 2 {
		t.Errorf("Nodes = %d, want 2", got.Nodes)
	}
	if n := got.Notification["notify_webhook:p"]; n.N != 2 || n.LastHour.Total().Events != 17 {
		t.Errorf("notification = %+v, want N=2 events=17", n)
	}
	// A class only one node reported must survive.
	if got.Audit["audit_kafka:1"].LastHour.Total().Events != 5 ||
		got.Logs["logger_webhook:main"].LastHour.Total().Events != 2 {
		t.Errorf("audit/logs lost: %+v / %+v", got.Audit, got.Logs)
	}
	if got.Spill == nil || got.Spill.Bytes != 150 || got.Spill.Files != 3 {
		t.Errorf("Spill = %+v, want 150/3", got.Spill)
	}
	// Log volume is node-level, so it sums across reports like everything else --
	// and a window only one node reported must survive.
	if tot := got.LogVolumeLastHour.Total(); tot != (LogVolumeSegment{N: 2, ErrorLines: 4, WarningLines: 4}) {
		t.Errorf("LogVolumeLastHour total = %+v", tot)
	}
	if tot := got.LogVolumeLastDay.Total(); tot != (LogVolumeSegment{N: 1, FatalLines: 1}) {
		t.Errorf("LogVolumeLastDay total = %+v", tot)
	}
}

func TestDeliveryTargetMetricsMergeNil(t *testing.T) {
	t0 := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	full := &DeliveryTargetMetrics{
		Nodes: 1,
		Notification: map[string]TargetMetrics{
			"notify_webhook:p": {N: 1, LastHour: targetWindow(t0, TargetSegment{N: 1, Events: 10})},
		},
		Spill:             &TargetSpillStats{Bytes: 100},
		LogVolumeLastHour: logVolumeWindow(t0, LogVolumeSegment{N: 1, ErrorLines: 7}),
	}

	var got DeliveryTargetMetrics
	got.Merge(full)
	got.Merge(&DeliveryTargetMetrics{})
	got.Merge(nil)

	if got.Notification["notify_webhook:p"].LastHour.Total().Events != 10 || got.Spill == nil ||
		got.LogVolumeLastHour.Total().ErrorLines != 7 {
		t.Errorf("a node reporting nothing blanked a real report: %+v", got)
	}
}
