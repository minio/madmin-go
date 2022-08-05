//
// MinIO Object Storage (c) 2022 MinIO, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package madmin

import (
	"sort"
	"testing"
)

func TestTimings(t *testing.T) {
	durations := TimeDurations{
		4000000,
		4000000,
		9000000,
		9000000,
		12000000,
		12000000,
		14000000,
		14000000,
		17000000,
		17000000,
		21000000,
		21000000,
		36000000,
		36000000,
		37000000,
		37000000,
		42000000,
		42000000,
		54000000,
		54000000,
		67000000,
		67000000,
		77000000,
		77000000,
		88000000,
		88000000,
		89000000,
		89000000,
		93000000,
		93000000,
	}

	sort.Slice(durations, func(i, j int) bool {
		return int64(durations[i]) < int64(durations[j])
	})

	timings := durations.Measure()
	if timings.Avg != 44000000 {
		t.Errorf("Expected 44000000, got %d\n", timings.Avg)
	}

	if timings.P50 != 37000000 {
		t.Errorf("Expected 37000000, got %d\n", timings.P50)
	}

	if timings.P75 != 77000000 {
		t.Errorf("Expected 77000000, got %d\n", timings.P75)
	}

	if timings.P95 != 93000000 {
		t.Errorf("Expected 93000000, got %d\n", timings.P95)
	}

	if timings.P99 != 93000000 {
		t.Errorf("Expected 93000000, got %d\n", timings.P99)
	}

	if timings.P999 != 93000000 {
		t.Errorf("Expected 93000000, got %d\n", timings.P999)
	}

	if timings.Long5p != 93000000 {
		t.Errorf("Expected 93000000, got %d\n", timings.Long5p)
	}

	if timings.Short5p != 4000000 {
		t.Errorf("Expected 4000000, got %d\n", timings.Short5p)
	}

	if timings.Max != 93000000 {
		t.Errorf("Expected 93000000, got %d\n", timings.Max)
	}

	if timings.Min != 4000000 {
		t.Errorf("Expected 4000000, got %d\n", timings.Min)
	}

	if timings.Range != 89000000 {
		t.Errorf("Expected 89000000, got %d\n", timings.Range)
	}

	if timings.StdDev != 30772281 {
		t.Errorf("Expected abc, got %d\n", timings.StdDev)
	}
}
