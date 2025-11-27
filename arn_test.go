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
	"strings"
	"testing"
)

// TestARNEmpty tests the Empty method of the ARN struct.
func TestARNEmpty(t *testing.T) {
	tests := []struct {
		name     string
		arn      ARN
		expected bool
	}{
		{
			name:     "Empty ARN",
			arn:      ARN{},
			expected: true,
		},
		{
			name: "Non-empty ARN with Type",
			arn: ARN{
				Type: ReplicationService,
			},
			expected: false,
		},
		{
			name: "Non-empty ARN with full fields",
			arn: ARN{
				Type:     ReplicationService,
				ID:       "12345",
				Region:   "us-east-1",
				Resource: "resource",
				Bucket:   "mybucket",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.arn.Empty()
			if got != tt.expected {
				t.Errorf("Empty() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestARNString tests the String method of the ARN struct.
func TestARNString(t *testing.T) {
	tests := []struct {
		name     string
		arn      ARN
		expected string
	}{
		{
			name: "ARN with Bucket",
			arn: ARN{
				Type:   ReplicationService,
				Region: "us-east-1",
				ID:     "12345",
				Bucket: "mybucket",
			},
			expected: "arn:minio:replication:us-east-1:12345:mybucket",
		},
		{
			name: "ARN with Resource (no Bucket)",
			arn: ARN{
				Type:     NotificationService,
				Region:   "eu-west-1",
				ID:       "67890",
				Resource: "queue",
			},
			expected: "arn:minio:sqs:eu-west-1:67890:queue",
		},
		{
			name: "ARN with empty fields except Type",
			arn: ARN{
				Type: ReplicationService,
			},
			expected: "arn:minio:replication:::",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.arn.String()
			if got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestParseARN tests the ParseARN function.
func TestParseARN(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expected      *ARN
		expectedError string
	}{
		{
			name:  "Valid ARN with Bucket",
			input: "arn:minio:replication:us-east-1:12345:mybucket",
			expected: &ARN{
				Type:   ReplicationService,
				Region: "us-east-1",
				ID:     "12345",
				Bucket: "mybucket",
			},
			expectedError: "",
		},
		{
			name:  "Valid ARN with Resource",
			input: "arn:minio:sqs:eu-west-1:67890:queue",
			expected: &ARN{
				Type:   NotificationService,
				Region: "eu-west-1",
				ID:     "67890",
				Bucket: "queue",
			},
			expectedError: "",
		},
		{
			name:          "Invalid prefix",
			input:         "arn:aws:replication:us-east-1:12345:mybucket",
			expected:      nil,
			expectedError: "invalid ARN arn:aws:replication:us-east-1:12345:mybucket",
		},
		{
			name:          "Too few tokens",
			input:         "arn:minio:replication:us-east-1:12345",
			expected:      nil,
			expectedError: "invalid ARN arn:minio:replication:us-east-1:12345",
		},
		{
			name:          "Too many tokens",
			input:         "arn:minio:replication:us-east-1:12345:mybucket:extra",
			expected:      nil,
			expectedError: "invalid ARN arn:minio:replication:us-east-1:12345:mybucket:extra",
		},
		{
			name:          "Empty ID",
			input:         "arn:minio:replication:us-east-1::mybucket",
			expected:      nil,
			expectedError: "invalid ARN arn:minio:replication:us-east-1::mybucket",
		},
		{
			name:          "Empty Bucket/Resource",
			input:         "arn:minio:replication:us-east-1:12345:",
			expected:      nil,
			expectedError: "invalid ARN arn:minio:replication:us-east-1:12345:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseARN(tt.input)
			if tt.expectedError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("ParseARN() error = %v, want %q", err, tt.expectedError)
				}
				if got != nil {
					t.Errorf("ParseARN() = %v, want nil", got)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseARN() unexpected error: %v", err)
			}

			if got != nil && tt.expected != nil {
				if got.String() != tt.expected.String() {
					t.Errorf("ParseARN() = %v, want %v", got, tt.expected)
				}
			}
		})
	}
}

// TestServiceTypeIsValid tests the IsValid method of ServiceType.
func TestServiceTypeIsValid(t *testing.T) {
	tests := []struct {
		name     string
		t1       ServiceType
		t2       ServiceType
		expected bool
	}{
		{
			name:     "Matching replication service",
			t1:       ReplicationService,
			t2:       ReplicationService,
			expected: true,
		},
		{
			name:     "Matching notification service",
			t1:       NotificationService,
			t2:       NotificationService,
			expected: true,
		},
		{
			name:     "Non-matching services",
			t1:       ReplicationService,
			t2:       NotificationService,
			expected: false,
		},
		{
			name:     "Empty service type",
			t1:       "",
			t2:       ReplicationService,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.t1.IsValid(tt.t2)
			if got != tt.expected {
				t.Errorf("IsValid(%q, %q) = %v, want %v", tt.t1, tt.t2, got, tt.expected)
			}
		})
	}
}
