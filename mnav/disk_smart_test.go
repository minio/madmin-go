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
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package mnav

import (
	"testing"
	"time"

	"github.com/minio/madmin-go/v4"
)

// The node formats on its own, without a navigator above it, and the row keeps
// its "01:" prefix so it sorts directly after the drive count.
func TestDiskSMARTNodeHealthStatusReason(t *testing.T) {
	node := NewDiskSMARTNode(&madmin.SMARTInfo{
		N:      2,
		Status: map[string]int{"warning": 2},
		LastStatusReason: map[string]madmin.StatusReason{
			"warning": {Reason: "NVMe percentage used above threshold", CreatedAt: time.Now()},
		},
	}, nil, "drive/smart")

	want := "2 warning (last: NVMe percentage used above threshold)"
	if got := leafValue(node.GetLeafData(), "01:Health Status"); got != want {
		t.Errorf("01:Health Status = %q, want %q", got, want)
	}
}
