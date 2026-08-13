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

// MetricType.String() is a hand-maintained list, so a new bit is silently
// nameless if the addIf line is forgotten.
func TestTierMetricTypeString(t *testing.T) {
	if got := MetricsTier.String(); got != "Tier" {
		t.Errorf("MetricsTier.String() = %q, want %q", got, "Tier")
	}
}

func TestWarmTierStatAdd(t *testing.T) {
	t0 := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)

	a := &WarmTierStat{
		N: 1, Type: "s3",
		Ops: map[string]TimedAction{
			"put": {Count: 10, AccTime: 1000, Bytes: 5000, MinTime: 50, MaxTime: 200},
			"get": {Count: 4, AccTime: 400, Bytes: 800, MinTime: 80, MaxTime: 120},
		},
		GetTTFB:       TimedAction{Count: 5, AccTime: 100, MinTime: 10, MaxTime: 30},
		Errors:        map[string]uint64{"not_found": 2},
		InflightPut:   1,
		LastSuccess:   t0,
		LastError:     "older",
		LastErrorTime: t0,
	}
	b := &WarmTierStat{
		N: 1, Type: "s3",
		Ops: map[string]TimedAction{
			"put":    {Count: 5, AccTime: 5000, Bytes: 2000, MinTime: 500, MaxTime: 900},
			"delete": {Count: 2, AccTime: 20, MinTime: 5, MaxTime: 15},
		},
		GetTTFB:        TimedAction{Count: 2, AccTime: 60, MinTime: 25, MaxTime: 40},
		Errors:         map[string]uint64{"not_found": 1, "unreachable": 7},
		InflightDelete: 3,
		LastSuccess:    t0.Add(time.Minute),
		LastError:      "newer",
		LastErrorTime:  t0.Add(time.Minute),
	}

	got := cloneWarmTierStat(a)
	got.Add(b)

	if got.N != 2 || got.Type != "s3" {
		t.Errorf("N/Type = %d/%q, want 2/s3", got.N, got.Type)
	}
	// TimedAction keeps the extremes, so the slow node stays visible instead of
	// being averaged away.
	put := got.Ops["put"]
	if put.Count != 15 || put.Bytes != 7000 || put.MinTime != 50 || put.MaxTime != 900 {
		t.Errorf("put = %+v", put)
	}
	// An op only one node performed must survive the merge.
	if got.Ops["delete"].Count != 2 || got.Ops["get"].Count != 4 {
		t.Errorf("ops lost: %+v", got.Ops)
	}
	if got.GetTTFB.Count != 7 || got.GetTTFB.MinTime != 10 || got.GetTTFB.MaxTime != 40 {
		t.Errorf("GetTTFB = %+v", got.GetTTFB)
	}
	if !maps.Equal(got.Errors, map[string]uint64{"not_found": 3, "unreachable": 7}) {
		t.Errorf("Errors = %v", got.Errors)
	}
	if got.InflightPut != 1 || got.InflightDelete != 3 {
		t.Errorf("inflight = %d/%d, want 1/3", got.InflightPut, got.InflightDelete)
	}
	if !got.LastSuccess.Equal(t0.Add(time.Minute)) {
		t.Errorf("LastSuccess = %v, want the most recent", got.LastSuccess)
	}
	if got.LastError != "newer" {
		t.Errorf("LastError = %q, want the most recent failure", got.LastError)
	}

	// collectRealtimeMetrics merges remote reports first and the local one last,
	// so a merge that is not order-independent lets the local node win.
	rev := cloneWarmTierStat(b)
	rev.Add(a)
	if !reflect.DeepEqual(rev, got) {
		t.Errorf("order dependent:\n%+v\n%+v", rev, got)
	}
}

func cloneWarmTierStat(s *WarmTierStat) WarmTierStat {
	out := *s
	out.Ops = maps.Clone(s.Ops)
	out.Errors = maps.Clone(s.Errors)
	return out
}

// Add must not write through into its argument: the argument is another node's
// report, and mergeMap relies on the guarantee.
func TestWarmTierStatAddDoesNotMutateArgument(t *testing.T) {
	src := &WarmTierStat{
		N:      1,
		Ops:    map[string]TimedAction{"put": {Count: 1, AccTime: 10}},
		Errors: map[string]uint64{"other": 1},
	}
	wantOps, wantErrs := maps.Clone(src.Ops), maps.Clone(src.Errors)

	var dst WarmTierStat
	dst.Add(src)
	dst.Add(src)

	if !maps.Equal(src.Ops, wantOps) || !maps.Equal(src.Errors, wantErrs) {
		t.Errorf("argument mutated: %v / %v", src.Ops, src.Errors)
	}
	if dst.Ops["put"].Count != 2 || dst.Errors["other"] != 2 {
		t.Errorf("dst = %+v after two merges", dst)
	}
}

// A configured tier that has never been used must survive a merge as a present,
// all-zero entry. This is the only way to tell "configured but idle" from "not
// configured", so a merge that dropped it would erase the distinction.
func TestWarmTierMetricsKeepsIdleTier(t *testing.T) {
	busy := &WarmTierMetrics{
		Nodes: 1,
		Tiers: map[string]WarmTierStat{
			"WARM": {N: 1, Type: "s3", Ops: map[string]TimedAction{"put": {Count: 3}}},
			"COLD": {N: 1, Type: "azure"},
		},
	}
	idle := &WarmTierMetrics{
		Nodes: 1,
		Tiers: map[string]WarmTierStat{
			"WARM": {N: 1, Type: "s3"},
			"COLD": {N: 1, Type: "azure"},
		},
	}

	var got WarmTierMetrics
	got.Merge(busy)
	got.Merge(idle)

	cold, ok := got.Tiers["COLD"]
	if !ok {
		t.Fatal("an idle tier was dropped; it is indistinguishable from unconfigured")
	}
	if cold.N != 2 || cold.Type != "azure" {
		t.Errorf("COLD = %+v, want N=2 type=azure", cold)
	}
	if len(cold.Ops) != 0 || !cold.LastSuccess.IsZero() {
		t.Errorf("an idle tier gained activity: %+v", cold)
	}
	if got.Tiers["WARM"].Ops["put"].Count != 3 {
		t.Errorf("WARM = %+v", got.Tiers["WARM"])
	}
}

func TestWarmTierMetricsMergeNil(t *testing.T) {
	full := &WarmTierMetrics{
		Nodes: 1,
		Tiers: map[string]WarmTierStat{"WARM": {N: 1, Ops: map[string]TimedAction{"get": {Count: 9}}}},
	}

	var got WarmTierMetrics
	got.Merge(full)
	got.Merge(&WarmTierMetrics{})
	got.Merge(nil)

	if got.Tiers["WARM"].Ops["get"].Count != 9 {
		t.Errorf("a node reporting nothing blanked a real report: %+v", got.Tiers)
	}
}

// Reading a window rolls it forward by adding a fresh factory element, so every
// segment field must have the zero value as its additive identity -- otherwise
// the act of reading corrupts it.
func TestWarmTierSegmentAddCoversAllFields(t *testing.T) {
	src := WarmTierSegment{
		PutCount: 1, PutBytes: 2, PutNanos: 3,
		GetCount: 4, GetBytes: 5, GetNanos: 6,
		DeleteCount: 7, DeleteNanos: 8,
		ErrorsNotFound: 9, ErrorsUnreachable: 10, ErrorsOther: 11,
		N: 12,
	}

	var got WarmTierSegment
	got.Add(&src)
	if got != src {
		t.Errorf("Add lost a field:\n got %+v\nwant %+v", got, src)
	}

	// Adding the zero value must change nothing -- that is what makes a window
	// read non-destructive.
	got.Add(&WarmTierSegment{})
	got.Add(nil)
	if got != src {
		t.Errorf("zero is not the additive identity: %+v", got)
	}

	got.Add(&src)
	v, s := reflect.ValueOf(got), reflect.ValueOf(src)
	for i := range v.NumField() {
		name := v.Type().Field(i).Name
		switch v.Field(i).Kind() {
		case reflect.Uint64:
			if v.Field(i).Uint() != 2*s.Field(i).Uint() {
				t.Errorf("%s did not sum", name)
			}
		case reflect.Int:
			if v.Field(i).Int() != 2*s.Field(i).Int() {
				t.Errorf("%s did not sum", name)
			}
		default:
			t.Errorf("%s has kind %v; every segment field must be a plain "+
				"summable counter", name, v.Field(i).Kind())
		}
	}
}
