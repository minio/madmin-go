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
