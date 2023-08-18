//
// Copyright (c) 2015-2023 MinIO, Inc.
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
	"strings"
	"testing"

	"github.com/prometheus/prom2json"
)

func TestParsePrometheusResultsReturnsPrometheusObjectsFromStringReader(t *testing.T) {
	prometheusResults := `# HELP go_gc_duration_seconds A summary of the pause duration of garbage collection cycles.
		# TYPE go_gc_duration_seconds summary
		go_gc_duration_seconds_sum 0.248349766
		go_gc_duration_seconds_count 397
	`
	myReader := strings.NewReader(prometheusResults)
	results, err := parsePrometheusResults(myReader)
	if err != nil {
		t.Errorf("error not expected, got: %v", err)
	}

	expectedResults := []*prom2json.Family{
		{
			Name: "go_gc_duration_seconds",
			Type: "SUMMARY",
			Help: "A summary of the pause duration of garbage collection cycles.",
			Metrics: []interface{}{
				prom2json.Summary{}, // We just verify length, not content
			},
		},
	}

	if len(results) != len(expectedResults) {
		t.Errorf("len(results): %d  not equal to len(expectedResults): %d", len(results), len(expectedResults))
	}

	for i, result := range results {
		if result.Name != expectedResults[i].Name {
			t.Errorf("result.Name: %v  not equal to expectedResults[i].Name: %v", result.Name, expectedResults[i].Name)
		}
		if len(result.Metrics) != len(expectedResults[i].Metrics) {
			t.Errorf("len(result.Metrics): %d  not equal to len(expectedResults[i].Metrics): %d", len(result.Metrics), len(expectedResults[i].Metrics))
		}
	}
}
