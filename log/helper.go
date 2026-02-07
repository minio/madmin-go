// Copyright (c) 2015-2025 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package log

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// stringifyMap sorts and joins a map[string]string as "k1=v1,k2=v2".
func stringifyMap(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	pairs := make([]string, 0, len(m))
	for k, v := range m {
		pairs = append(pairs, fmt.Sprintf("%v=%v", k, v))
	}
	slices.Sort(pairs)
	return strings.Join(pairs, ",")
}

func toString(key, value string) string {
	if value == "" {
		return ""
	}
	return fmt.Sprintf("%s=%s", key, value)
}

func toInt(key string, value int) string {
	if value == 0 {
		return ""
	}
	return fmt.Sprintf("%s=%d", key, value)
}

func toInt64(key string, value int64) string {
	if value == 0 {
		return ""
	}
	return fmt.Sprintf("%s=%d", key, value)
}

func toTime(key string, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return fmt.Sprintf("%s=%s", key, t.UTC().Format(time.RFC3339Nano))
}

func toMap(key string, m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	return fmt.Sprintf("%s={%s}", key, stringifyMap(m))
}

// filterAndSort removes empty entries and sorts.
func filterAndSort(values []string) []string {
	out := values[:0]
	for _, v := range values {
		if v != "" {
			out = append(out, v)
		}
	}
	slices.Sort(out)
	return out
}
