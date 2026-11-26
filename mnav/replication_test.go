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
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package mnav

import (
	"strings"
	"testing"

	"github.com/minio/madmin-go/v4"
)

func hasValue(data map[string]string, key, value string) bool {
	for k, v := range data {
		if strings.HasSuffix(k, key) && v == value {
			return true
		}
	}
	return false
}

func TestReplicationStatsDisplayAnnotations(t *testing.T) {
	data := generateReplicationStatsDisplay(madmin.ReplicationStats{Nodes: 1, Events: 3}, 60, false)
	for k := range data {
		if strings.Contains(k, "Annotation") {
			t.Errorf("annotation line %q shown without annotation activity", k)
		}
	}

	data = generateReplicationStatsDisplay(madmin.ReplicationStats{
		Nodes:              1,
		Annotations:        1200,
		AnnotationsSkipped: 3,
		AnnotationBytes:    2048,
		AnnotationErrors:   1,
	}, 60, false)
	for key, want := range map[string]string{
		"Annotations Replicated": "1,200",
		"Annotations Skipped":    "3",
		"Annotation Data":        "2.0 kB",
		"Annotation Errors":      "1",
	} {
		if !hasValue(data, key, want) {
			t.Errorf("%s: want %q in %v", key, want, data)
		}
	}
}

func TestReplicationAnnotationState(t *testing.T) {
	got := annotationStateDesc(map[string]int{"unknown": 1, "supported": 3, "denied": 1})
	if want := "supported (3 nodes), denied (1 node), unknown (1 node)"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	node := NewReplicationTargetNode("arn:minio:replication::id:bucket", &madmin.ReplicationTargetStats{
		Nodes:           2,
		AnnotationState: map[string]int{"supported": 2},
	}, nil, "")
	if got := node.GetLeafData()["Annotation Replication"]; got != "supported (2 nodes)" {
		t.Errorf("target leaf: got %q", got)
	}
	node = NewReplicationTargetNode("arn:minio:replication::id:bucket", &madmin.ReplicationTargetStats{Nodes: 2}, nil, "")
	if _, ok := node.GetLeafData()["Annotation Replication"]; ok {
		t.Error("target leaf shows annotation replication without any state")
	}
}
