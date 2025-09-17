//
// Copyright (c) 2015-2025 MinIO, Inc.
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
	"reflect"
	"testing"
)

type Inner struct {
	Name  string
	Score int
	Ptr   *int
}

type Outer struct {
	ID       int
	Inner    Inner
	InnerPtr *Inner
}

func ptrInt(v int) *int { return &v }

func TestSortSlice_StringNested(t *testing.T) {
	items := []Outer{
		{ID: 1, Inner: Inner{Name: "Charlie"}},
		{ID: 2, Inner: Inner{Name: "Alice"}},
		{ID: 3, Inner: Inner{Name: "Bob"}},
	}
	SortSlice(items, "Inner.Name", false)
	got := []string{items[0].Inner.Name, items[1].Inner.Name, items[2].Inner.Name}
	want := []string{"Alice", "Bob", "Charlie"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ascending Inner.Name: got %v, want %v", got, want)
	}

	SortSlice(items, "Inner.Name", true)
	got = []string{items[0].Inner.Name, items[1].Inner.Name, items[2].Inner.Name}
	want = []string{"Charlie", "Bob", "Alice"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("descending Inner.Name: got %v, want %v", got, want)
	}
}

func TestSortSlice_IntNested(t *testing.T) {
	items := []Outer{
		{ID: 1, Inner: Inner{Score: 10}},
		{ID: 2, Inner: Inner{Score: 5}},
		{ID: 3, Inner: Inner{Score: 20}},
	}
	SortSlice(items, "Inner.Score", false)
	got := []int{items[0].Inner.Score, items[1].Inner.Score, items[2].Inner.Score}
	want := []int{5, 10, 20}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ascending Inner.Score: got %v, want %v", got, want)
	}
}

func TestSortSlice_Reversed(t *testing.T) {
	items := []Outer{
		{ID: 1, Inner: Inner{Score: 10}},
		{ID: 2, Inner: Inner{Score: 5}},
		{ID: 3, Inner: Inner{Score: 20}},
	}
	SortSlice(items, "Inner.Score", true)
	got := []int{items[0].Inner.Score, items[1].Inner.Score, items[2].Inner.Score}
	want := []int{20, 10, 5}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("descending Inner.Score: got %v, want %v", got, want)
	}
}

func TestSortSlice_PointerElementsAndNil(t *testing.T) {
	// Slice elements are pointers; include nils. Sorting by "ID".
	items := []*Outer{
		nil,
		{ID: 3, Inner: Inner{Name: "c"}},
		nil,
		{ID: 1, Inner: Inner{Name: "a"}},
		{ID: 2, Inner: Inner{Name: "b"}},
	}
	SortSlice(items, "ID", false)
	// Expect nils first (ascending), then by ID ascending.
	gotIDs := []int{}
	nilCount := 0
	for _, it := range items {
		if it == nil {
			nilCount++
			continue
		}
		gotIDs = append(gotIDs, it.ID)
	}
	if nilCount != 2 {
		t.Fatalf("expected 2 nils first, got %d", nilCount)
	}
	if !reflect.DeepEqual(gotIDs, []int{1, 2, 3}) {
		t.Fatalf("ascending with nils: got IDs %v, want [1 2 3]", gotIDs)
	}

	SortSlice(items, "ID", true)
	// In descending, non-nil should come first (nil last), ordered by ID desc.
	gotIDs = gotIDs[:0]
	nilCount = 0
	for _, it := range items {
		if it == nil {
			nilCount++
			continue
		}
		gotIDs = append(gotIDs, it.ID)
	}
	if !reflect.DeepEqual(gotIDs, []int{3, 2, 1}) {
		t.Fatalf("descending with nils: got IDs %v, want [3 2 1]", gotIDs)
	}
	if nilCount != 2 {
		t.Fatalf("expected 2 nils last in descending, got %d", nilCount)
	}
}

func TestSortSlice_NilIntermediatePointer(t *testing.T) {
	// Sort by nested pointer field "InnerPtr.Score" where some pointers are nil.
	items := []Outer{
		{ID: 1, InnerPtr: &Inner{Score: 10}},
		{ID: 2, InnerPtr: nil}, // nil should be considered "less"
		{ID: 3, InnerPtr: &Inner{Score: 5}},
		{ID: 4, InnerPtr: nil}, // another nil
		{ID: 5, InnerPtr: &Inner{Score: 20}},
	}
	SortSlice(items, "InnerPtr.Score", false)
	// Expect nils first (IDs 2,4 in original relative order), then by score ascending.
	gotIDs := []int{items[0].ID, items[1].ID, items[2].ID, items[3].ID, items[4].ID}
	wantIDs := []int{2, 4, 3, 1, 5}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("ascending with nil intermediate pointers: got IDs %v, want %v", gotIDs, wantIDs)
	}
}

func TestSortSlice_PointerToPrimitive(t *testing.T) {
	items := []Outer{
		{ID: 1, Inner: Inner{Ptr: ptrInt(10)}},
		{ID: 2, Inner: Inner{Ptr: nil}}, // nil first in ascending
		{ID: 3, Inner: Inner{Ptr: ptrInt(5)}},
	}
	SortSlice(items, "Inner.Ptr", false)
	got := []int{items[0].ID, items[1].ID, items[2].ID}
	want := []int{2, 3, 1}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ascending pointer to primitive: got IDs %v, want %v", got, want)
	}

	SortSlice(items, "Inner.Ptr", true)
	got = []int{items[0].ID, items[1].ID, items[2].ID}
	want = []int{1, 3, 2} // non-nil highest first, nil last
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("descending pointer to primitive: got IDs %v, want %v", got, want)
	}
}

func TestSortSlice_UnsupportedOrMissing(t *testing.T) {
	orig := []Outer{
		{ID: 1, Inner: Inner{Name: "z"}},
		{ID: 2, Inner: Inner{Name: "a"}},
		{ID: 3, Inner: Inner{Name: "m"}},
	}

	// Unsupported type: sorting by a struct field itself should keep order (stable, comparator returns equal).
	items := append([]Outer(nil), orig...)
	SortSlice(items, "Inner", false)
	if !reflect.DeepEqual(items, orig) {
		t.Fatalf("unsupported type should keep order, got %v, want %v", items, orig)
	}

	// Missing field: should keep order.
	items = append([]Outer(nil), orig...)
	SortSlice(items, "DoesNotExist", false)
	if !reflect.DeepEqual(items, orig) {
		t.Fatalf("missing field should keep order, got %v, want %v", items, orig)
	}

	// Empty field: function returns immediately, keep order.
	items = append([]Outer(nil), orig...)
	SortSlice(items, "", false)
	if !reflect.DeepEqual(items, orig) {
		t.Fatalf("empty field should keep order, got %v, want %v", items, orig)
	}
}
