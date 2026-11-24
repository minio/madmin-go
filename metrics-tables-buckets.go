//
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
//

package madmin

import "maps"

// A separate file keeps msgp from inlining the map into
// TableMaintenanceRuns, where it mis-generates code for a pointer to it.
//go:generate go tool msgp -unexported -d clearomitted -d "tag json" -d "timezone utc" -d "maps binkeys" -file $GOFILE

// TableMaintenanceDurationBuckets maps an upper bound in seconds, or "+Inf",
// to a run count. It is an AIStor extension.
type TableMaintenanceDurationBuckets map[string]uint64

// IsZero reports whether b holds no buckets, for omitzero.
func (b *TableMaintenanceDurationBuckets) IsZero() bool {
	return b == nil || len(*b) == 0
}

func (b *TableMaintenanceDurationBuckets) clone() *TableMaintenanceDurationBuckets {
	c := maps.Clone(*b)
	return &c
}
