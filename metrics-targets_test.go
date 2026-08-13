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
		NodesOnline: 1,
		TotalEvents: 100, TotalRequests: 20, FailedRequests: 1, WriterErrors: 1,
		Drops:         map[string]uint64{"queue_full": 2},
		QueueLength:   10,
		QueueCapacity: 100_000,
		LastMinute:    TimedAction{Count: 5, AccTime: 500, MinTime: 50, MaxTime: 200},
		LastError:     "older", LastErrorTime: t0,
	}
	b := &TargetMetrics{
		N: 1, Subsystem: "notify_webhook", Name: "primary", Type: "webhook",
		NodesOnline: 0, NodesChecking: 1,
		TotalEvents: 50, TotalRequests: 10, FailedRequests: 4, WriterErrors: 2,
		Drops:         map[string]uint64{"queue_full": 1, "retries_exhausted": 3},
		QueueLength:   90_000,
		QueueCapacity: 100_000,
		LastMinute:    TimedAction{Count: 3, AccTime: 900, MinTime: 300, MaxTime: 400},
		LastError:     "newer", LastErrorTime: t0.Add(time.Minute),
	}

	// Add mutates the receiver's maps in place by design, so a shallow copy of
	// the input would alias them. mergeMap never does that in production: it
	// starts from the zero value and addMap allocates rather than aliasing a
	// source -- see TestTargetMetricsAddDoesNotMutateArgument.
	got := cloneTargetMetrics(a)
	got.Add(b)

	// Readiness as node counts: "online on 1 of 2 nodes".
	if got.N != 2 || got.NodesOnline != 1 || got.NodesChecking != 1 {
		t.Errorf("readiness = %+v, want N=2 online=1 checking=1", got)
	}
	if got.TotalEvents != 150 || got.TotalRequests != 30 || got.WriterErrors != 3 {
		t.Errorf("counters = %+v", got)
	}
	if !maps.Equal(got.Drops, map[string]uint64{"queue_full": 3, "retries_exhausted": 3}) {
		t.Errorf("Drops = %v", got.Drops)
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
		rev.N != got.N || !maps.Equal(rev.Drops, got.Drops) {
		t.Errorf("order dependent:\n%+v\n%+v", rev, got)
	}
}

func cloneTargetMetrics(t *TargetMetrics) TargetMetrics {
	out := *t
	out.Drops = maps.Clone(t.Drops)
	return out
}

// Merge must never write through into the value it is given: the caller's copy is
// another node's report, and mergeMap relies on Add being side-effect free on its
// argument.
func TestTargetMetricsAddDoesNotMutateArgument(t *testing.T) {
	src := &TargetMetrics{N: 1, TotalEvents: 5, Drops: map[string]uint64{"queue_full": 2}}
	want := maps.Clone(src.Drops)

	dst := TargetMetrics{}
	dst.Add(src)
	dst.Add(src)

	if !maps.Equal(src.Drops, want) {
		t.Errorf("argument mutated: %v, want %v", src.Drops, want)
	}
	if dst.Drops["queue_full"] != 4 {
		t.Errorf("dst = %v, want queue_full 4 after two merges", dst.Drops)
	}
}

// The same guard one level up: merging a map of targets must not write into the
// source maps.
func TestDeliveryTargetMetricsMergeDoesNotMutateArgument(t *testing.T) {
	src := &DeliveryTargetMetrics{
		Nodes: 1,
		Notification: map[string]TargetMetrics{
			"notify_webhook:p": {N: 1, TotalEvents: 3, Drops: map[string]uint64{"shutdown": 1}},
		},
		LogVolume: map[string]uint64{"error": 2},
	}
	wantDrops := maps.Clone(src.Notification["notify_webhook:p"].Drops)
	wantVolume := maps.Clone(src.LogVolume)

	var dst DeliveryTargetMetrics
	dst.Merge(src)
	dst.Merge(src)

	if !maps.Equal(src.Notification["notify_webhook:p"].Drops, wantDrops) {
		t.Errorf("source target Drops mutated: %v", src.Notification["notify_webhook:p"].Drops)
	}
	if !maps.Equal(src.LogVolume, wantVolume) {
		t.Errorf("source LogVolume mutated: %v", src.LogVolume)
	}
	if got := dst.Notification["notify_webhook:p"]; got.TotalEvents != 6 || got.Drops["shutdown"] != 2 {
		t.Errorf("dst = %+v, want events 6 and shutdown 2", got)
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
	a := &DeliveryTargetMetrics{
		Nodes:        1,
		Notification: map[string]TargetMetrics{"notify_webhook:p": {N: 1, TotalEvents: 10}},
		Audit:        map[string]TargetMetrics{"audit_kafka:1": {N: 1, TotalEvents: 5}},
		Spill:        &TargetSpillStats{Bytes: 100, Files: 1},
		LogVolume:    map[string]uint64{"error": 3},
	}
	b := &DeliveryTargetMetrics{
		Nodes:        1,
		Notification: map[string]TargetMetrics{"notify_webhook:p": {N: 1, TotalEvents: 7}},
		Logs:         map[string]TargetMetrics{"logger_webhook:main": {N: 1, TotalEvents: 2}},
		Spill:        &TargetSpillStats{Bytes: 50, Files: 2},
		LogVolume:    map[string]uint64{"error": 1, "warning": 4},
	}

	var got DeliveryTargetMetrics
	got.Merge(a)
	got.Merge(b)

	if got.Nodes != 2 {
		t.Errorf("Nodes = %d, want 2", got.Nodes)
	}
	if n := got.Notification["notify_webhook:p"]; n.N != 2 || n.TotalEvents != 17 {
		t.Errorf("notification = %+v, want N=2 events=17", n)
	}
	// A class only one node reported must survive.
	if got.Audit["audit_kafka:1"].TotalEvents != 5 || got.Logs["logger_webhook:main"].TotalEvents != 2 {
		t.Errorf("audit/logs lost: %+v / %+v", got.Audit, got.Logs)
	}
	if got.Spill == nil || got.Spill.Bytes != 150 || got.Spill.Files != 3 {
		t.Errorf("Spill = %+v, want 150/3", got.Spill)
	}
	if !maps.Equal(got.LogVolume, map[string]uint64{"error": 4, "warning": 4}) {
		t.Errorf("LogVolume = %v", got.LogVolume)
	}
}

func TestDeliveryTargetMetricsMergeNil(t *testing.T) {
	full := &DeliveryTargetMetrics{
		Nodes:        1,
		Notification: map[string]TargetMetrics{"notify_webhook:p": {N: 1, TotalEvents: 10}},
		Spill:        &TargetSpillStats{Bytes: 100},
	}

	var got DeliveryTargetMetrics
	got.Merge(full)
	got.Merge(&DeliveryTargetMetrics{})
	got.Merge(nil)

	if got.Notification["notify_webhook:p"].TotalEvents != 10 || got.Spill == nil {
		t.Errorf("a node reporting nothing blanked a real report: %+v", got)
	}
}
