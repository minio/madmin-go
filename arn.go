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
	"strings"
)

// ARN is a struct to define arn.
type ARN struct {
	Type     ServiceType
	ID       string
	Region   string
	Resource string
	Bucket   string
}

// Empty returns true if arn struct is empty
func (a ARN) Empty() bool {
	return a.Type == ""
}

func (a ARN) String() string {
	if a.Bucket != "" {
		return fmt.Sprintf("arn:minio:%s:%s:%s:%s", a.Type, a.Region, a.ID, a.Bucket)
	}
	return fmt.Sprintf("arn:minio:%s:%s:%s:%s", a.Type, a.Region, a.ID, a.Resource)
}

// ParseARN return ARN struct from string in arn format.
func ParseARN(s string) (*ARN, error) {
	// ARN must be in the format of arn:minio:<Type>:<REGION>:<ID>:<remote-bucket/remote-resource>
	if !strings.HasPrefix(s, "arn:minio:") {
		return nil, fmt.Errorf("invalid ARN %s", s)
	}

	tokens := strings.Split(s, ":")
	if len(tokens) != 6 {
		return nil, fmt.Errorf("invalid ARN %s", s)
	}

	if tokens[4] == "" || tokens[5] == "" {
		return nil, fmt.Errorf("invalid ARN %s", s)
	}

	return &ARN{
		Type:     ServiceType(tokens[2]),
		Region:   tokens[3],
		ID:       tokens[4],
		Resource: tokens[5],
	}, nil
}

// ServiceType represents service type
type ServiceType string

const (
	// ReplicationService specifies replication service
	ReplicationService ServiceType = "replication"

	// NotificationService specifies notification/lambda service
	NotificationService ServiceType = "sqs"
)

// IsValid returns true if ARN type is set.
func (t1 ServiceType) IsValid(t2 ServiceType) bool {
	return t1 == t2
}
