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
	"testing"
	"time"
)

// The path names are the map keys on the wire and the field split in every
// segment, so renaming one silently reattributes a series.
func TestIAMPathNamesStable(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{IAMPathPlugin, "plugin"},
		{IAMPathOwner, "owner"},
		{IAMPathSTS, "sts"},
		{IAMPathSvcAcct, "svcacct"},
		{IAMPathUser, "user"},
	} {
		if tc.got != tc.want {
			t.Errorf("path name = %q, want %q", tc.got, tc.want)
		}
	}
}

// Every counter in a segment sums across nodes. The one exception is the
// maximum, which takes the worst node: summing peaks would invent a latency no
// request ever saw.
func TestIAMSegmentAdd(t *testing.T) {
	a := &IAMSegment{
		UserCount: 10, UserNanos: 1000,
		STSCount: 2, STSNanos: 800,
		MaxNanos: 500,
		Denied:   1, Errors: 2, CacheMiss: 3,
		LoadCount: 1, LoadNanos: 90, StoreErrors: 1,
		N: 1,
	}
	b := &IAMSegment{
		UserCount: 5, UserNanos: 250,
		PluginCount: 4, PluginNanos: 4000,
		MaxNanos:  1500,
		Denied:    2,
		SaveCount: 3, SaveNanos: 300,
		N: 1,
	}

	got := *a
	got.Add(b)

	if got.UserCount != 15 || got.UserNanos != 1250 {
		t.Errorf("user = %d/%d, want 15/1250", got.UserCount, got.UserNanos)
	}
	if got.MaxNanos != 1500 {
		t.Errorf("MaxNanos = %d, want 1500: the worst node's peak, not the sum", got.MaxNanos)
	}
	if got.Denied != 3 || got.Errors != 2 || got.CacheMiss != 3 {
		t.Errorf("counters = %d/%d/%d, want 3/2/3", got.Denied, got.Errors, got.CacheMiss)
	}
	// A path only one node took survives rather than being lost.
	if got.PluginCount != 4 || got.STSCount != 2 {
		t.Errorf("a path only one node reported was lost: %+v", got)
	}
	if got.SaveCount != 3 || got.LoadCount != 1 || got.StoreErrors != 1 {
		t.Errorf("store counters = %+v, want save 3 load 1 errors 1", got)
	}
	if got.N != 2 {
		t.Errorf("N = %d, want 2", got.N)
	}
	// The argument is another node's report and must not be touched.
	if b.UserCount != 5 || b.MaxNanos != 1500 {
		t.Errorf("argument mutated: %+v", b)
	}

	got.Add(nil)
	if got.UserCount != 15 {
		t.Error("Add(nil) changed the receiver")
	}
}

// The derived totals are what a renderer uses instead of adding ten fields by
// hand, so they must cover every path.
func TestIAMSegmentTotals(t *testing.T) {
	s := IAMSegment{
		PluginCount: 1, PluginNanos: 10,
		OwnerCount: 2, OwnerNanos: 20,
		STSCount: 3, STSNanos: 30,
		SvcAcctCount: 4, SvcAcctNanos: 40,
		UserCount: 5, UserNanos: 50,
		SaveCount: 6, SaveNanos: 60,
		LoadCount: 7, LoadNanos: 70,
		DeleteCount: 8, DeleteNanos: 80,
		ListCount: 9, ListNanos: 90,
	}
	if got := s.AuthCount(); got != 15 {
		t.Errorf("AuthCount() = %d, want 15", got)
	}
	if got := s.AuthNanos(); got != 150 {
		t.Errorf("AuthNanos() = %d, want 150", got)
	}
	if got := s.StoreCount(); got != 30 {
		t.Errorf("StoreCount() = %d, want 30", got)
	}
	if got := s.StoreNanos(); got != 300 {
		t.Errorf("StoreNanos() = %d, want 300", got)
	}
}

func TestIAMAuthStatsMerge(t *testing.T) {
	a := &IAMAuthStats{
		ByPath: map[string]TimedAction{
			IAMPathUser: {Count: 2, AccTime: 100, MinTime: 40, MaxTime: 60},
		},
		Denied: 1, CacheMiss: 4,
	}
	b := &IAMAuthStats{
		ByPath: map[string]TimedAction{
			IAMPathUser:   {Count: 1, AccTime: 900, MinTime: 900, MaxTime: 900},
			IAMPathPlugin: {Count: 5, AccTime: 50_000, MinTime: 8000, MaxTime: 20_000},
		},
		Denied: 2, Errors: 3,
	}

	var got IAMAuthStats
	got.Merge(a)
	got.Merge(b)

	user := got.ByPath[IAMPathUser]
	if user.Count != 3 || user.MinTime != 40 || user.MaxTime != 900 {
		t.Errorf("user = %+v, want count 3 with the extremes preserved", user)
	}
	if got.ByPath[IAMPathPlugin].Count != 5 {
		t.Errorf("a path only one node took was lost: %+v", got.ByPath)
	}
	if got.Denied != 3 || got.Errors != 3 || got.CacheMiss != 4 {
		t.Errorf("counters = %d/%d/%d, want 3/3/4", got.Denied, got.Errors, got.CacheMiss)
	}
	// Merge is handed another node's report, so its maps must survive.
	if a.ByPath[IAMPathUser].Count != 2 {
		t.Errorf("argument mutated: %+v", a.ByPath)
	}

	got.Merge(nil)
	if got.Denied != 3 {
		t.Error("Merge(nil) changed the receiver")
	}
}

func TestIAMMetricsMergeWindows(t *testing.T) {
	t0 := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)

	a := &IAMMetrics{
		Nodes:    1,
		LastHour: iamWindow(t0, IAMSegment{N: 1, UserCount: 10, UserNanos: 500, MaxNanos: 90}),
		LastDay:  iamWindow(t0, IAMSegment{N: 1, LoadCount: 1, LoadNanos: 40}),
	}
	b := &IAMMetrics{
		Nodes:    1,
		LastHour: iamWindow(t0, IAMSegment{N: 1, UserCount: 4, UserNanos: 200, MaxNanos: 700}),
		LastDay:  iamWindow(t0, IAMSegment{N: 1, LoadCount: 2, LoadNanos: 80}),
	}

	var got IAMMetrics
	got.Merge(a)
	got.Merge(b)

	if tot := got.LastHour.Total(); tot.UserCount != 14 || tot.UserNanos != 700 {
		t.Errorf("hour total = %+v, want 14 authorizations over 700ns", tot)
	}
	if tot := got.LastHour.Total(); tot.MaxNanos != 700 {
		t.Errorf("hour MaxNanos = %d, want 700", tot.MaxNanos)
	}
	if tot := got.LastDay.Total(); tot.LoadCount != 3 || tot.LoadNanos != 120 {
		t.Errorf("day total = %+v, want 3 loads over 120ns", tot)
	}
	if a.LastHour.Total().UserCount != 10 {
		t.Errorf("argument mutated: %+v", a.LastHour)
	}
}

// A window a node was asked for but has nothing in yet is non-nil with no
// segments. The merge must keep it: dropping it makes the result indistinguishable
// from a window nobody requested, which is a different answer.
func TestIAMMetricsMergeKeepsEmptyRequestedWindow(t *testing.T) {
	requestedButEmpty := &IAMMetrics{
		Nodes:    1,
		LastHour: &SegmentedIAMMetrics{Interval: 60},
	}
	var got IAMMetrics
	got.Merge(requestedButEmpty)

	if got.LastHour == nil {
		t.Fatal("a requested-but-empty window was dropped, so it now reads as not requested")
	}
	if len(got.LastHour.Segments) != 0 {
		t.Errorf("segments = %d, want 0", len(got.LastHour.Segments))
	}

	// And a node that did have data must still merge into it.
	t0 := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	got.Merge(&IAMMetrics{
		Nodes:    1,
		LastHour: iamWindow(t0, IAMSegment{N: 1, OwnerCount: 3}),
	})
	if tot := got.LastHour.Total(); tot.OwnerCount != 3 {
		t.Errorf("hour total = %+v, want the later node's 3 authorizations", tot)
	}
}

func TestIAMMetricsMergeAuthAndNil(t *testing.T) {
	full := &IAMMetrics{
		Nodes: 1,
		Auth: &IAMAuthStats{
			ByPath: map[string]TimedAction{IAMPathOwner: {Count: 7, AccTime: 70}},
			Denied: 1,
		},
	}
	var got IAMMetrics
	got.Merge(full)
	got.Merge(&IAMMetrics{})
	got.Merge(nil)

	if got.Auth == nil || got.Auth.ByPath[IAMPathOwner].Count != 7 || got.Auth.Denied != 1 {
		t.Errorf("a node reporting nothing blanked a real report: %+v", got.Auth)
	}
}

// iamWindow is one node's IAM report: a single segment on the minute grid.
func iamWindow(first time.Time, seg IAMSegment) *SegmentedIAMMetrics {
	return &SegmentedIAMMetrics{
		Interval:  60,
		FirstTime: first,
		Segments:  []IAMSegment{seg},
	}
}
