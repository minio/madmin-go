//
// Copyright (c) 2015-2024 MinIO, Inc.
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
