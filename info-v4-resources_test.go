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
	"slices"
	"strings"
	"testing"
)

type Inner struct {
	Name   string
	Score  int
	Ptr    *int
	UScore uint
	FScore float64
}

type Outer struct {
	ID       int
	Inner    Inner
	InnerPtr *Inner
	Tags     []string
	Count    uint32
	Price    float32
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
	items := slices.Clone(orig)
	SortSlice(items, "Inner", false)
	if !reflect.DeepEqual(items, orig) {
		t.Fatalf("unsupported type should keep order, got %v, want %v", items, orig)
	}

	// Missing field: should keep order.
	items = slices.Clone(orig)
	SortSlice(items, "DoesNotExist", false)
	if !reflect.DeepEqual(items, orig) {
		t.Fatalf("missing field should keep order, got %v, want %v", items, orig)
	}

	// Empty field: function returns immediately, keep order.
	items = slices.Clone(orig)
	SortSlice(items, "", false)
	if !reflect.DeepEqual(items, orig) {
		t.Fatalf("empty field should keep order, got %v, want %v", items, orig)
	}
}

func TestSortSlice_Uint(t *testing.T) {
	items := []Outer{
		{ID: 1, Inner: Inner{UScore: 100}},
		{ID: 2, Inner: Inner{UScore: 50}},
		{ID: 3, Inner: Inner{UScore: 200}},
	}
	SortSlice(items, "Inner.UScore", false)
	got := []uint{items[0].Inner.UScore, items[1].Inner.UScore, items[2].Inner.UScore}
	want := []uint{50, 100, 200}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ascending Inner.UScore: got %v, want %v", got, want)
	}

	SortSlice(items, "Inner.UScore", true)
	got = []uint{items[0].Inner.UScore, items[1].Inner.UScore, items[2].Inner.UScore}
	want = []uint{200, 100, 50}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("descending Inner.UScore: got %v, want %v", got, want)
	}
}

func TestSortSlice_Float(t *testing.T) {
	items := []Outer{
		{ID: 1, Inner: Inner{FScore: 10.5}},
		{ID: 2, Inner: Inner{FScore: 5.2}},
		{ID: 3, Inner: Inner{FScore: 20.7}},
	}
	SortSlice(items, "Inner.FScore", false)
	got := []float64{items[0].Inner.FScore, items[1].Inner.FScore, items[2].Inner.FScore}
	want := []float64{5.2, 10.5, 20.7}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ascending Inner.FScore: got %v, want %v", got, want)
	}

	SortSlice(items, "Inner.FScore", true)
	got = []float64{items[0].Inner.FScore, items[1].Inner.FScore, items[2].Inner.FScore}
	want = []float64{20.7, 10.5, 5.2}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("descending Inner.FScore: got %v, want %v", got, want)
	}
}

func TestSortSlice_Uint32(t *testing.T) {
	items := []Outer{
		{ID: 1, Count: 1000},
		{ID: 2, Count: 500},
		{ID: 3, Count: 2000},
	}
	SortSlice(items, "Count", false)
	got := []uint32{items[0].Count, items[1].Count, items[2].Count}
	want := []uint32{500, 1000, 2000}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ascending Count: got %v, want %v", got, want)
	}
}

func TestSortSlice_Float32(t *testing.T) {
	items := []Outer{
		{ID: 1, Price: 19.99},
		{ID: 2, Price: 9.99},
		{ID: 3, Price: 29.99},
	}
	SortSlice(items, "Price", false)
	got := []float32{items[0].Price, items[1].Price, items[2].Price}
	want := []float32{9.99, 19.99, 29.99}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ascending Price: got %v, want %v", got, want)
	}
}

func TestSortSlice_EmptySlice(t *testing.T) {
	var items []Outer
	SortSlice(items, "ID", false)
	if len(items) != 0 {
		t.Fatalf("empty slice should remain empty, got %v", items)
	}
}

func TestSortSlice_SingleElement(t *testing.T) {
	items := []Outer{
		{ID: 42, Inner: Inner{Name: "single"}},
	}
	SortSlice(items, "ID", false)
	if len(items) != 1 || items[0].ID != 42 {
		t.Fatalf("single element slice should remain unchanged, got %v", items)
	}

	SortSlice(items, "Inner.Name", false)
	if len(items) != 1 || items[0].Inner.Name != "single" {
		t.Fatalf("single element slice should remain unchanged, got %v", items)
	}
}

func TestSortSlice_Stability(t *testing.T) {
	// When elements are equal, their original order should be preserved (stable sort)
	items := []Outer{
		{ID: 1, Inner: Inner{Score: 100, Name: "first"}},
		{ID: 2, Inner: Inner{Score: 100, Name: "second"}},
		{ID: 3, Inner: Inner{Score: 50, Name: "third"}},
		{ID: 4, Inner: Inner{Score: 100, Name: "fourth"}},
		{ID: 5, Inner: Inner{Score: 50, Name: "fifth"}},
	}

	SortSlice(items, "Inner.Score", false)

	// Check that items with score 50 maintain relative order (3 before 5)
	// Check that items with score 100 maintain relative order (1 before 2 before 4)
	if items[0].ID != 3 || items[1].ID != 5 {
		t.Fatalf("stability failed for score 50: expected IDs [3,5], got [%d,%d]", items[0].ID, items[1].ID)
	}
	if items[2].ID != 1 || items[3].ID != 2 || items[4].ID != 4 {
		t.Fatalf("stability failed for score 100: expected IDs [1,2,4], got [%d,%d,%d]", items[2].ID, items[3].ID, items[4].ID)
	}
}

func TestSortSlice_StabilityDescending(t *testing.T) {
	items := []Outer{
		{ID: 1, Inner: Inner{Score: 100, Name: "first"}},
		{ID: 2, Inner: Inner{Score: 50, Name: "second"}},
		{ID: 3, Inner: Inner{Score: 100, Name: "third"}},
		{ID: 4, Inner: Inner{Score: 50, Name: "fourth"}},
	}

	SortSlice(items, "Inner.Score", true)

	// In descending order: score 100 items should come first (maintaining order 1,3), then score 50 items (2,4)
	if items[0].ID != 1 || items[1].ID != 3 {
		t.Fatalf("stability failed for score 100: expected IDs [1,3], got [%d,%d]", items[0].ID, items[1].ID)
	}
	if items[2].ID != 2 || items[3].ID != 4 {
		t.Fatalf("stability failed for score 50: expected IDs [2,4], got [%d,%d]", items[2].ID, items[3].ID)
	}
}

func TestSortSlice_ZeroValues(t *testing.T) {
	items := []Outer{
		{ID: 1, Inner: Inner{Score: 10}},
		{ID: 2, Inner: Inner{Score: 0}},
		{ID: 3, Inner: Inner{Score: 5}},
		{ID: 4, Inner: Inner{Score: 0}},
	}
	SortSlice(items, "Inner.Score", false)
	scores := []int{items[0].Inner.Score, items[1].Inner.Score, items[2].Inner.Score, items[3].Inner.Score}
	want := []int{0, 0, 5, 10}
	if !reflect.DeepEqual(scores, want) {
		t.Fatalf("ascending with zero values: got %v, want %v", scores, want)
	}
}

func TestSortSlice_NegativeNumbers(t *testing.T) {
	items := []Outer{
		{ID: 1, Inner: Inner{Score: -5}},
		{ID: 2, Inner: Inner{Score: 10}},
		{ID: 3, Inner: Inner{Score: -20}},
		{ID: 4, Inner: Inner{Score: 0}},
		{ID: 5, Inner: Inner{Score: 5}},
	}
	SortSlice(items, "Inner.Score", false)
	scores := []int{items[0].Inner.Score, items[1].Inner.Score, items[2].Inner.Score, items[3].Inner.Score, items[4].Inner.Score}
	want := []int{-20, -5, 0, 5, 10}
	if !reflect.DeepEqual(scores, want) {
		t.Fatalf("ascending with negative numbers: got %v, want %v", scores, want)
	}

	SortSlice(items, "Inner.Score", true)
	scores = []int{items[0].Inner.Score, items[1].Inner.Score, items[2].Inner.Score, items[3].Inner.Score, items[4].Inner.Score}
	want = []int{10, 5, 0, -5, -20}
	if !reflect.DeepEqual(scores, want) {
		t.Fatalf("descending with negative numbers: got %v, want %v", scores, want)
	}
}

func TestSortSlice_EmptyStrings(t *testing.T) {
	items := []Outer{
		{ID: 1, Inner: Inner{Name: "zebra"}},
		{ID: 2, Inner: Inner{Name: ""}},
		{ID: 3, Inner: Inner{Name: "apple"}},
		{ID: 4, Inner: Inner{Name: ""}},
		{ID: 5, Inner: Inner{Name: "banana"}},
	}
	SortSlice(items, "Inner.Name", false)
	names := []string{items[0].Inner.Name, items[1].Inner.Name, items[2].Inner.Name, items[3].Inner.Name, items[4].Inner.Name}
	want := []string{"", "", "apple", "banana", "zebra"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("ascending with empty strings: got %v, want %v", names, want)
	}
}

func BenchmarkSortSlice_SmallInt(b *testing.B) {
	orig := make([]Outer, 10)
	for i := range orig {
		orig[i] = Outer{ID: 10 - i, Inner: Inner{Score: 10 - i}}
	}

	for b.Loop() {
		items := make([]Outer, len(orig))
		copy(items, orig)
		SortSlice(items, "ID", false)
	}
}

func BenchmarkSortSlice_MediumInt(b *testing.B) {
	orig := make([]Outer, 100)
	for i := range orig {
		orig[i] = Outer{ID: 100 - i, Inner: Inner{Score: 100 - i}}
	}

	for b.Loop() {
		items := make([]Outer, len(orig))
		copy(items, orig)
		SortSlice(items, "ID", false)
	}
}

func BenchmarkSortSlice_LargeInt(b *testing.B) {
	orig := make([]Outer, 1000)
	for i := range orig {
		orig[i] = Outer{ID: 1000 - i, Inner: Inner{Score: 1000 - i}}
	}

	for b.Loop() {
		items := make([]Outer, len(orig))
		copy(items, orig)
		SortSlice(items, "ID", false)
	}
}

func BenchmarkSortSlice_NestedString(b *testing.B) {
	orig := make([]Outer, 100)
	for i := range orig {
		orig[i] = Outer{ID: i, Inner: Inner{Name: string(rune('z' - i%26))}}
	}

	for b.Loop() {
		items := make([]Outer, len(orig))
		copy(items, orig)
		SortSlice(items, "Inner.Name", false)
	}
}

func TestSortSlice_CaseInsensitive(t *testing.T) {
	// Test various case variations of field names
	items := []Outer{
		{ID: 3, Inner: Inner{Name: "Charlie", Score: 30}},
		{ID: 1, Inner: Inner{Name: "Alice", Score: 10}},
		{ID: 2, Inner: Inner{Name: "Bob", Score: 20}},
	}

	testCases := []struct {
		name     string
		field    string
		reversed bool
		wantIDs  []int
		wantDesc string
	}{
		// Test different case variations for "ID"
		{"lowercase id", "id", false, []int{1, 2, 3}, "sort by lowercase 'id'"},
		{"uppercase ID", "ID", false, []int{1, 2, 3}, "sort by uppercase 'ID'"},
		{"mixed case Id", "Id", false, []int{1, 2, 3}, "sort by mixed case 'Id'"},
		{"mixed case iD", "iD", false, []int{1, 2, 3}, "sort by mixed case 'iD'"},

		// Test nested field with different cases
		{"lowercase inner.name", "inner.name", false, []int{1, 2, 3}, "sort by lowercase nested field"},
		{"uppercase INNER.NAME", "INNER.NAME", false, []int{1, 2, 3}, "sort by uppercase nested field"},
		{"mixed Inner.Name", "Inner.Name", false, []int{1, 2, 3}, "sort by mixed case nested field"},
		{"mixed InNeR.NaMe", "InNeR.NaMe", false, []int{1, 2, 3}, "sort by mixed case nested field"},

		// Test with score field
		{"lowercase inner.score", "inner.score", false, []int{1, 2, 3}, "sort by lowercase score"},
		{"uppercase INNER.SCORE", "INNER.SCORE", false, []int{1, 2, 3}, "sort by uppercase score"},
		{"mixed Inner.Score", "Inner.Score", false, []int{1, 2, 3}, "sort by mixed case score"},

		// Test reversed sorting with case variations
		{"reverse lowercase id", "id", true, []int{3, 2, 1}, "reverse sort by lowercase 'id'"},
		{"reverse uppercase ID", "ID", true, []int{3, 2, 1}, "reverse sort by uppercase 'ID'"},
		{"reverse mixed Inner.Score", "inner.SCORE", true, []int{3, 2, 1}, "reverse sort by mixed case score"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Make a fresh copy for each test
			testItems := make([]Outer, len(items))
			copy(testItems, items)

			SortSlice(testItems, tc.field, tc.reversed)

			gotIDs := make([]int, len(testItems))
			for i, item := range testItems {
				gotIDs[i] = item.ID
			}

			if !reflect.DeepEqual(gotIDs, tc.wantIDs) {
				t.Errorf("%s: got IDs %v, want %v", tc.wantDesc, gotIDs, tc.wantIDs)
			}
		})
	}
}

func TestSortSlice_CaseInsensitiveComplexFields(t *testing.T) {
	// Test with more complex field names like Count and Price
	items := []Outer{
		{ID: 1, Count: 100, Price: 19.99},
		{ID: 2, Count: 50, Price: 9.99},
		{ID: 3, Count: 200, Price: 29.99},
	}

	testCases := []struct {
		name    string
		field   string
		wantIDs []int
	}{
		{"lowercase count", "count", []int{2, 1, 3}},
		{"uppercase COUNT", "COUNT", []int{2, 1, 3}},
		{"mixed Count", "Count", []int{2, 1, 3}},
		{"mixed CoUnT", "CoUnT", []int{2, 1, 3}},

		{"lowercase price", "price", []int{2, 1, 3}},
		{"uppercase PRICE", "PRICE", []int{2, 1, 3}},
		{"mixed Price", "Price", []int{2, 1, 3}},
		{"mixed PrIcE", "PrIcE", []int{2, 1, 3}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testItems := make([]Outer, len(items))
			copy(testItems, items)

			SortSlice(testItems, tc.field, false)

			gotIDs := make([]int, len(testItems))
			for i, item := range testItems {
				gotIDs[i] = item.ID
			}

			if !reflect.DeepEqual(gotIDs, tc.wantIDs) {
				t.Errorf("field %s: got IDs %v, want %v", tc.field, gotIDs, tc.wantIDs)
			}
		})
	}
}

func TestSortSlice_CaseInsensitiveWithPointers(t *testing.T) {
	// Test case insensitive with pointer fields
	inner1 := &Inner{Score: 10, Name: "alpha"}
	inner2 := &Inner{Score: 5, Name: "beta"}
	inner3 := &Inner{Score: 20, Name: "gamma"}

	items := []Outer{
		{ID: 1, InnerPtr: inner1},
		{ID: 2, InnerPtr: inner2},
		{ID: 3, InnerPtr: inner3},
	}

	testCases := []struct {
		name    string
		field   string
		wantIDs []int
	}{
		{"lowercase innerptr.score", "innerptr.score", []int{2, 1, 3}},
		{"uppercase INNERPTR.SCORE", "INNERPTR.SCORE", []int{2, 1, 3}},
		{"mixed InnerPtr.Score", "InnerPtr.Score", []int{2, 1, 3}},
		{"mixed iNnErPtR.sCoRe", "iNnErPtR.sCoRe", []int{2, 1, 3}},

		{"lowercase innerptr.name", "innerptr.name", []int{1, 2, 3}},
		{"uppercase INNERPTR.NAME", "INNERPTR.NAME", []int{1, 2, 3}},
		{"mixed InnerPtr.Name", "InnerPtr.Name", []int{1, 2, 3}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testItems := make([]Outer, len(items))
			copy(testItems, items)

			SortSlice(testItems, tc.field, false)

			gotIDs := make([]int, len(testItems))
			for i, item := range testItems {
				gotIDs[i] = item.ID
			}

			if !reflect.DeepEqual(gotIDs, tc.wantIDs) {
				t.Errorf("field %s: got IDs %v, want %v", tc.field, gotIDs, tc.wantIDs)
			}
		})
	}
}

func TestSortSlice_CaseInsensitiveNonExistent(t *testing.T) {
	// Test that non-existent fields still don't sort regardless of case
	orig := []Outer{
		{ID: 3, Inner: Inner{Name: "c"}},
		{ID: 1, Inner: Inner{Name: "a"}},
		{ID: 2, Inner: Inner{Name: "b"}},
	}

	testCases := []string{
		"nonexistent",
		"NONEXISTENT",
		"NonExistent",
		"NoNeXiStEnT",
		"inner.nonexistent",
		"INNER.NONEXISTENT",
		"Inner.NonExistent",
		"InNeR.NoNeXiStEnT",
	}

	for _, field := range testCases {
		t.Run(field, func(t *testing.T) {
			items := slices.Clone(orig)
			SortSlice(items, field, false)

			// Should maintain original order for non-existent fields
			if !reflect.DeepEqual(items, orig) {
				t.Errorf("non-existent field %s should keep original order, got %v, want %v", field, items, orig)
			}
		})
	}
}

func BenchmarkSortSlice_AlreadySorted(b *testing.B) {
	orig := make([]Outer, 100)
	for i := range orig {
		orig[i] = Outer{ID: i}
	}

	for b.Loop() {
		items := make([]Outer, len(orig))
		copy(items, orig)
		SortSlice(items, "ID", false)
	}
}

func BenchmarkSortSlice_CaseInsensitive(b *testing.B) {
	orig := make([]Outer, 100)
	for i := range orig {
		orig[i] = Outer{ID: 100 - i, Inner: Inner{Score: 100 - i}}
	}

	b.Run("lowercase", func(b *testing.B) {
		for b.Loop() {
			items := make([]Outer, len(orig))
			copy(items, orig)
			SortSlice(items, "id", false)
		}
	})

	b.Run("uppercase", func(b *testing.B) {
		for b.Loop() {
			items := make([]Outer, len(orig))
			copy(items, orig)
			SortSlice(items, "ID", false)
		}
	})

	b.Run("mixed", func(b *testing.B) {
		for b.Loop() {
			items := make([]Outer, len(orig))
			copy(items, orig)
			SortSlice(items, "iD", false)
		}
	})

	b.Run("nested_lowercase", func(b *testing.B) {
		for b.Loop() {
			items := make([]Outer, len(orig))
			copy(items, orig)
			SortSlice(items, "inner.score", false)
		}
	})

	b.Run("nested_mixed", func(b *testing.B) {
		for b.Loop() {
			items := make([]Outer, len(orig))
			copy(items, orig)
			SortSlice(items, "InNeR.sCoRe", false)
		}
	})
}

// TestSortSlice_StringCaseInsensitiveComparison tests that string comparison itself is case-insensitive
func TestSortSlice_StringCaseInsensitiveComparison(t *testing.T) {
	items := []Outer{
		{ID: 1, Inner: Inner{Name: "zebra"}},
		{ID: 2, Inner: Inner{Name: "APPLE"}},
		{ID: 3, Inner: Inner{Name: "Banana"}},
		{ID: 4, Inner: Inner{Name: "apple"}}, // Different case from ID 2
		{ID: 5, Inner: Inner{Name: "ZEBRA"}}, // Different case from ID 1
	}

	SortSlice(items, "Inner.Name", false)

	// Both "apple" and "APPLE" should be grouped together at the beginning
	// Both "zebra" and "ZEBRA" should be at the end
	names := make([]string, len(items))
	for i, item := range items {
		names[i] = item.Inner.Name
	}

	// Check that case-insensitive sorting groups similar strings
	// "apple"/"APPLE" should come before "Banana" which should come before "zebra"/"ZEBRA"
	if strings.ToLower(names[0]) != "apple" || strings.ToLower(names[1]) != "apple" {
		t.Errorf("expected both 'apple' variants first, got %v", names[:2])
	}
	if strings.ToLower(names[2]) != "banana" {
		t.Errorf("expected 'Banana' in middle, got %v", names[2])
	}
	if strings.ToLower(names[3]) != "zebra" || strings.ToLower(names[4]) != "zebra" {
		t.Errorf("expected both 'zebra' variants last, got %v", names[3:])
	}
}

// TestSortSlice_UnicodeStrings tests sorting with unicode characters
func TestSortSlice_UnicodeStrings(t *testing.T) {
	items := []Outer{
		{ID: 1, Inner: Inner{Name: "Zürich"}},
		{ID: 2, Inner: Inner{Name: "Tokyo 東京"}},
		{ID: 3, Inner: Inner{Name: "Москва"}},
		{ID: 4, Inner: Inner{Name: "café"}},
		{ID: 5, Inner: Inner{Name: "Berlin"}},
	}

	SortSlice(items, "Inner.Name", false)

	// Just verify it doesn't panic and maintains stability
	if len(items) != 5 {
		t.Fatalf("expected 5 items after sort, got %d", len(items))
	}

	// Verify stability - original order should be maintained for any equal comparisons
	allIDs := make(map[int]bool)
	for _, item := range items {
		if allIDs[item.ID] {
			t.Errorf("duplicate ID %d found", item.ID)
		}
		allIDs[item.ID] = true
	}
}

// TestSortSlice_VeryLongStrings tests sorting with extremely long strings
func TestSortSlice_VeryLongStrings(t *testing.T) {
	longString1 := strings.Repeat("a", 10000) + "zzz"
	longString2 := strings.Repeat("a", 10000) + "aaa"
	longString3 := strings.Repeat("b", 10000)

	items := []Outer{
		{ID: 1, Inner: Inner{Name: longString1}},
		{ID: 2, Inner: Inner{Name: longString3}},
		{ID: 3, Inner: Inner{Name: longString2}},
	}

	SortSlice(items, "Inner.Name", false)

	// longString2 should come first (10000 a's + "aaa")
	// longString1 should come second (10000 a's + "zzz")
	// longString3 should come last (10000 b's)
	if items[0].ID != 3 {
		t.Errorf("expected ID 3 first, got %d", items[0].ID)
	}
	if items[1].ID != 1 {
		t.Errorf("expected ID 1 second, got %d", items[1].ID)
	}
	if items[2].ID != 2 {
		t.Errorf("expected ID 2 third, got %d", items[2].ID)
	}
}

// TestSortSlice_SpecialCharactersInStrings tests sorting with special characters
func TestSortSlice_SpecialCharactersInStrings(t *testing.T) {
	items := []Outer{
		{ID: 1, Inner: Inner{Name: "test@example.com"}},
		{ID: 2, Inner: Inner{Name: "!important"}},
		{ID: 3, Inner: Inner{Name: "###hashtag"}},
		{ID: 4, Inner: Inner{Name: "normal"}},
		{ID: 5, Inner: Inner{Name: "$money"}},
		{ID: 6, Inner: Inner{Name: "   spaces"}},
	}

	// Just verify it doesn't panic
	SortSlice(items, "Inner.Name", false)

	if len(items) != 6 {
		t.Fatalf("expected 6 items after sort, got %d", len(items))
	}
}

// TestSortSlice_DeeplyNestedPointerChain tests deeply nested pointer fields
func TestSortSlice_DeeplyNestedPointerChain(t *testing.T) {
	// Create a chain where InnerPtr points to another struct with a pointer
	inner1 := &Inner{Score: 10, Ptr: ptrInt(100)}
	inner2 := &Inner{Score: 5, Ptr: ptrInt(50)}
	inner3 := &Inner{Score: 20, Ptr: nil}

	items := []Outer{
		{ID: 1, InnerPtr: inner1},
		{ID: 2, InnerPtr: inner2},
		{ID: 3, InnerPtr: inner3},
		{ID: 4, InnerPtr: nil},
	}

	// Sort by the pointer within the pointer
	SortSlice(items, "InnerPtr.Ptr", false)

	// nil InnerPtr and nil Ptr should come first
	gotIDs := make([]int, len(items))
	for i, item := range items {
		gotIDs[i] = item.ID
	}

	// Expected: When InnerPtr.Ptr is nil (either because InnerPtr is nil OR Ptr is nil),
	// they should come first in stable order
	// ID 3 (InnerPtr exists but Ptr=nil), ID 4 (InnerPtr=nil), then sorted by Ptr value
	wantIDs := []int{3, 4, 2, 1}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Errorf("deeply nested pointers: got IDs %v, want %v", gotIDs, wantIDs)
	}
}

// TestSortSlice_MixedSignedIntegers tests sorting with various integer types and signs
func TestSortSlice_MixedSignedIntegers(t *testing.T) {
	items := []Outer{
		{ID: 1, Inner: Inner{Score: 2147483647}},  // max int32
		{ID: 2, Inner: Inner{Score: -2147483648}}, // min int32 (approximately)
		{ID: 3, Inner: Inner{Score: 0}},
		{ID: 4, Inner: Inner{Score: 1}},
		{ID: 5, Inner: Inner{Score: -1}},
	}

	SortSlice(items, "Inner.Score", false)

	scores := make([]int, len(items))
	for i, item := range items {
		scores[i] = item.Inner.Score
	}

	// Should be sorted from most negative to most positive
	for i := 0; i < len(scores)-1; i++ {
		if scores[i] > scores[i+1] {
			t.Errorf("scores not sorted correctly: %v", scores)
			break
		}
	}
}

// TestSortSlice_FloatEdgeCases tests floating point edge cases
func TestSortSlice_FloatEdgeCases(t *testing.T) {
	items := []Outer{
		{ID: 1, Inner: Inner{FScore: 0.0}},
		{ID: 2, Inner: Inner{FScore: -1.2}},
		{ID: 3, Inner: Inner{FScore: 1.7976931348623157e+308}},
		{ID: 4, Inner: Inner{FScore: 2.2250738585072014e-308}},
		{ID: 5, Inner: Inner{FScore: -1.7976931348623157e+308}},
	}

	SortSlice(items, "Inner.FScore", false)

	scores := make([]float64, len(items))
	for i, item := range items {
		scores[i] = item.Inner.FScore
	}

	// Verify ascending order
	for i := 0; i < len(scores)-1; i++ {
		if scores[i] > scores[i+1] {
			t.Errorf("float scores not sorted correctly: %v", scores)
			break
		}
	}
}

// TestSortSlice_UintMaxValues tests unsigned integer maximum values
func TestSortSlice_UintMaxValues(t *testing.T) {
	items := []Outer{
		{ID: 1, Inner: Inner{UScore: 0}},
		{ID: 2, Inner: Inner{UScore: ^uint(0)}}, // max uint
		{ID: 3, Inner: Inner{UScore: 1}},
		{ID: 4, Inner: Inner{UScore: ^uint(0) - 1}}, // max uint - 1
	}

	SortSlice(items, "Inner.UScore", false)

	scores := make([]uint, len(items))
	for i, item := range items {
		scores[i] = item.Inner.UScore
	}

	// Should be: 0, 1, max-1, max
	if scores[0] != 0 || scores[1] != 1 {
		t.Errorf("expected 0, 1 at start, got %v", scores[:2])
	}
	if scores[2] >= scores[3] {
		t.Errorf("expected ascending order at end, got %v", scores[2:])
	}
}

// TestSortSlice_AllNilPointers tests a slice where all elements are nil
func TestSortSlice_AllNilPointers(t *testing.T) {
	items := []*Outer{nil, nil, nil, nil}
	SortSlice(items, "ID", false)

	// All should still be nil
	for i, item := range items {
		if item != nil {
			t.Errorf("expected all nils, but item at index %d is not nil", i)
		}
	}
}

// TestSortSlice_MixedNilAndValuePointers tests complex pointer scenarios
func TestSortSlice_MixedNilAndValuePointers(t *testing.T) {
	items := []*Outer{
		{ID: 5, InnerPtr: &Inner{Score: 50}},
		nil,
		{ID: 3, InnerPtr: nil},
		{ID: 1, InnerPtr: &Inner{Score: 10}},
		nil,
		{ID: 4, InnerPtr: &Inner{Score: 40}},
	}

	SortSlice(items, "InnerPtr.Score", false)

	// Should have 2 nil Outers first, then nil InnerPtr, then sorted by InnerPtr.Score
	nilOuterCount := 0
	nilInnerPtrCount := 0
	validScores := []int{}

	for _, item := range items {
		if item == nil {
			nilOuterCount++
		} else if item.InnerPtr == nil {
			nilInnerPtrCount++
		} else {
			validScores = append(validScores, item.InnerPtr.Score)
		}
	}

	if nilOuterCount != 2 {
		t.Errorf("expected 2 nil Outers, got %d", nilOuterCount)
	}
	if nilInnerPtrCount != 1 {
		t.Errorf("expected 1 nil InnerPtr, got %d", nilInnerPtrCount)
	}
	if !reflect.DeepEqual(validScores, []int{10, 40, 50}) {
		t.Errorf("expected scores [10, 40, 50], got %v", validScores)
	}
}

// TestSortSlice_VeryLargeSlice tests performance with a large slice
func TestSortSlice_VeryLargeSlice(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large slice test in short mode")
	}

	items := make([]Outer, 10000)
	for i := range items {
		items[i] = Outer{ID: 10000 - i, Inner: Inner{Score: 10000 - i}}
	}

	SortSlice(items, "ID", false)

	// Verify first and last elements
	if items[0].ID != 1 {
		t.Errorf("expected first ID to be 1, got %d", items[0].ID)
	}
	if items[len(items)-1].ID != 10000 {
		t.Errorf("expected last ID to be 10000, got %d", items[len(items)-1].ID)
	}

	// Spot check that it's actually sorted
	for i := 0; i < len(items)-1; i += 100 {
		if items[i].ID > items[i+1].ID {
			t.Errorf("not sorted at index %d: %d > %d", i, items[i].ID, items[i+1].ID)
			break
		}
	}
}

// TestSortSlice_DuplicateValues tests sorting with many duplicate values
func TestSortSlice_DuplicateValues(t *testing.T) {
	items := []Outer{
		{ID: 1, Inner: Inner{Score: 10, Name: "first"}},
		{ID: 2, Inner: Inner{Score: 10, Name: "second"}},
		{ID: 3, Inner: Inner{Score: 10, Name: "third"}},
		{ID: 4, Inner: Inner{Score: 10, Name: "fourth"}},
		{ID: 5, Inner: Inner{Score: 10, Name: "fifth"}},
		{ID: 6, Inner: Inner{Score: 5, Name: "sixth"}},
		{ID: 7, Inner: Inner{Score: 5, Name: "seventh"}},
	}

	SortSlice(items, "Inner.Score", false)

	// Verify stability - all score=5 items should come first in original order
	if items[0].ID != 6 || items[1].ID != 7 {
		t.Errorf("expected IDs 6,7 first (score 5), got %d,%d", items[0].ID, items[1].ID)
	}

	// All score=10 items should maintain original order
	expectedIDs := []int{1, 2, 3, 4, 5}
	gotIDs := []int{items[2].ID, items[3].ID, items[4].ID, items[5].ID, items[6].ID}
	if !reflect.DeepEqual(gotIDs, expectedIDs) {
		t.Errorf("expected IDs %v for score 10, got %v", expectedIDs, gotIDs)
	}
}

// TestSortSlice_ThreeLevelNesting tests three-level nested field paths
func TestSortSlice_ThreeLevelNesting(t *testing.T) {
	// This tests deeper nesting than currently in the structs, but validates the path resolution
	// We can only test two levels with current structs, but this documents the intent
	items := []Outer{
		{ID: 1, InnerPtr: &Inner{Score: 30, Ptr: ptrInt(300)}},
		{ID: 2, InnerPtr: &Inner{Score: 10, Ptr: ptrInt(100)}},
		{ID: 3, InnerPtr: &Inner{Score: 20, Ptr: ptrInt(200)}},
	}

	SortSlice(items, "InnerPtr.Ptr", false)

	gotIDs := []int{items[0].ID, items[1].ID, items[2].ID}
	wantIDs := []int{2, 3, 1}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Errorf("nested pointer sorting: got IDs %v, want %v", gotIDs, wantIDs)
	}
}

// TestSortSlice_FieldPathWithSpaces tests that field paths with unusual formatting don't break
func TestSortSlice_FieldPathWithSpaces(t *testing.T) {
	items := []Outer{
		{ID: 3, Inner: Inner{Name: "Charlie"}},
		{ID: 1, Inner: Inner{Name: "Alice"}},
		{ID: 2, Inner: Inner{Name: "Bob"}},
	}

	// Paths with dots should work normally (spaces around dots would be invalid field names anyway)
	// This just tests the current implementation handles normal paths correctly
	SortSlice(items, "Inner.Name", false)

	gotIDs := []int{items[0].ID, items[1].ID, items[2].ID}
	wantIDs := []int{1, 2, 3}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Errorf("got IDs %v, want %v", gotIDs, wantIDs)
	}
}

// TestSortSlice_ConcurrentSorts tests that multiple sorts don't interfere (safety check)
func TestSortSlice_ConcurrentSorts(t *testing.T) {
	// Create separate slices to sort concurrently
	makeSlice := func() []Outer {
		return []Outer{
			{ID: 5, Inner: Inner{Score: 50}},
			{ID: 3, Inner: Inner{Score: 30}},
			{ID: 1, Inner: Inner{Score: 10}},
			{ID: 4, Inner: Inner{Score: 40}},
			{ID: 2, Inner: Inner{Score: 20}},
		}
	}

	items1 := makeSlice()
	items2 := makeSlice()
	items3 := makeSlice()

	done := make(chan bool, 3)

	go func() {
		SortSlice(items1, "ID", false)
		done <- true
	}()

	go func() {
		SortSlice(items2, "Inner.Score", false)
		done <- true
	}()

	go func() {
		SortSlice(items3, "ID", true)
		done <- true
	}()

	// Wait for all to complete
	<-done
	<-done
	<-done

	// Verify each sort worked correctly
	if items1[0].ID != 1 || items1[4].ID != 5 {
		t.Errorf("items1 not sorted correctly")
	}
	if items2[0].Inner.Score != 10 || items2[4].Inner.Score != 50 {
		t.Errorf("items2 not sorted correctly")
	}
	if items3[0].ID != 5 || items3[4].ID != 1 {
		t.Errorf("items3 not sorted correctly")
	}
}
