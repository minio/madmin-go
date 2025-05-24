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

package madmin

import (
	"fmt"
	"sort"
	"testing"
)

// TestListNotificationARNs tests the ListNotificationARNs method of the Services struct.
func TestListNotificationARNs(t *testing.T) {
	tests := []struct {
		name     string
		services Services
		expected []ARN
	}{
		{
			name:     "Empty Services",
			services: Services{},
			expected: []ARN{},
		},
		{
			name: "Nil Notifications",
			services: Services{
				Notifications: nil,
			},
			expected: []ARN{},
		},
		{
			name: "Single Notification",
			services: Services{
				Notifications: []map[string][]TargetIDStatus{
					{
						"queue1": []TargetIDStatus{
							{"target1": Status{Status: "active"}},
						},
					},
				},
			},
			expected: []ARN{
				{
					Type:     ServiceType("sqs"),
					ID:       "target1",
					Resource: "queue1",
					Region:   "",
					Bucket:   "",
				},
			},
		},
		{
			name: "Multiple Notifications",
			services: Services{
				Notifications: []map[string][]TargetIDStatus{
					{
						"queue1": []TargetIDStatus{
							{"target1": Status{Status: "active"}},
							{"target2": Status{Status: "inactive"}},
						},
						"queue2": []TargetIDStatus{
							{"target3": Status{Status: "active"}},
						},
					},
					{
						"queue3": []TargetIDStatus{
							{"target4": Status{Status: "inactive"}},
						},
					},
				},
			},
			expected: []ARN{
				{
					Type:     ServiceType("sqs"),
					ID:       "target1",
					Resource: "queue1",
					Region:   "",
					Bucket:   "",
				},
				{
					Type:     ServiceType("sqs"),
					ID:       "target2",
					Resource: "queue1",
					Region:   "",
					Bucket:   "",
				},
				{
					Type:     ServiceType("sqs"),
					ID:       "target3",
					Resource: "queue2",
					Region:   "",
					Bucket:   "",
				},
				{
					Type:     ServiceType("sqs"),
					ID:       "target4",
					Resource: "queue3",
					Region:   "",
					Bucket:   "",
				},
			},
		},
		{
			name: "Empty Target Types",
			services: Services{
				Notifications: []map[string][]TargetIDStatus{
					{},
				},
			},
			expected: []ARN{},
		},
		{
			name: "Empty Target Statuses",
			services: Services{
				Notifications: []map[string][]TargetIDStatus{
					{
						"queue1": []TargetIDStatus{},
					},
				},
			},
			expected: []ARN{},
		},
		{
			name: "Empty Target ID",
			services: Services{
				Notifications: []map[string][]TargetIDStatus{
					{
						"queue1": []TargetIDStatus{
							{"": Status{Status: "active"}},
						},
					},
				},
			},
			expected: []ARN{
				{
					Type:     ServiceType("sqs"),
					ID:       "",
					Resource: "queue1",
					Region:   "",
					Bucket:   "",
				},
			},
		},
		{
			name: "Multiple Target IDs in Single Status",
			services: Services{
				Notifications: []map[string][]TargetIDStatus{
					{
						"queue1": []TargetIDStatus{
							{
								"target1": Status{Status: "active"},
								"target2": Status{Status: "inactive"},
							},
						},
					},
				},
			},
			expected: []ARN{
				{
					Type:     ServiceType("sqs"),
					ID:       "target1",
					Resource: "queue1",
					Region:   "",
					Bucket:   "",
				},
				{
					Type:     ServiceType("sqs"),
					ID:       "target2",
					Resource: "queue1",
					Region:   "",
					Bucket:   "",
				},
			},
		},
		{
			name: "Invalid TargetIDStatus",
			services: Services{
				Notifications: []map[string][]TargetIDStatus{
					{
						"queue1": []TargetIDStatus{
							{}, // Empty TargetIDStatus map
						},
					},
				},
			},
			expected: []ARN{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.services.ListNotificationARNs()
			// Sort ARNs by ID for consistent comparison
			sort.Slice(got, func(i, j int) bool {
				return got[i].ID < got[j].ID
			})
			sort.Slice(tt.expected, func(i, j int) bool {
				return tt.expected[i].ID < tt.expected[j].ID
			})

			if len(got) != len(tt.expected) {
				t.Errorf("ListNotificationARNs() length = %d, want %d; got = %v, want = %v", len(got), len(tt.expected), got, tt.expected)
				return
			}

			for i := range got {
				if ok, diff := compareARNs(got[i], tt.expected[i]); !ok {
					t.Errorf("ListNotificationARNs() ARN[%d] mismatch: %s; got = %+v, want = %+v", i, diff, got[i], tt.expected[i])
				}
			}
		})
	}
}

// compareARNs compares two ARN structs and returns true if they are equal, along with a diff string if unequal.
func compareARNs(a, b ARN) (bool, string) {
	if a.Type != b.Type {
		return false, fmt.Sprintf("Type mismatch: %q != %q", a.Type, b.Type)
	}
	if a.ID != b.ID {
		return false, fmt.Sprintf("ID mismatch: %q != %q", a.ID, b.ID)
	}
	if a.Region != b.Region {
		return false, fmt.Sprintf("Region mismatch: %q != %q", a.Region, b.Region)
	}
	if a.Resource != b.Resource {
		return false, fmt.Sprintf("Resource mismatch: %q != %q", a.Resource, b.Resource)
	}
	if a.Bucket != b.Bucket {
		return false, fmt.Sprintf("Bucket mismatch: %q != %q", a.Bucket, b.Bucket)
	}
	return true, ""
}
