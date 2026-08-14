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
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/minio/madmin-go/v4"
)

// segmentStart is the wall-clock start of segment i of a window: segments are
// oldest-first and aligned to FirstTime + i*Interval.
func segmentStart(firstTime time.Time, interval, i int) time.Time {
	return firstTime.Add(time.Duration(i*interval) * time.Second)
}

// segmentKey is the navigation name of the segment starting at t. It is a bare
// wall-clock time, so it is unique only within 24 hours; segmentOwners resolves
// the collision.
func segmentKey(t time.Time) string {
	return t.UTC().Format("15:04Z")
}

// segmentOwners maps every navigation key of a window to the slot that owns it.
//
// Keys collide once a window reaches 24 hours, and Segmented.Add produces exactly
// that whenever two contributors' FirstTime differ by one interval -- the normal
// outcome of a cross-node collect that straddles a segment boundary. The merged
// timeline then runs one interval past a full day, so its first and last slot
// carry the same wall-clock time. The newest slot wins and the oldest is dropped,
// which is what keeps GetChildren and GetChild in agreement: every listed key
// resolves, and resolves to the segment the reader saw described.
func segmentOwners(firstTime time.Time, step time.Duration, count int) map[string]int {
	owners := make(map[string]int, count)
	for i := range count {
		owners[segmentKey(firstTime.Add(time.Duration(i)*step))] = i
	}
	return owners
}

// segmentSecOwners is segmentOwners for a window whose interval is in seconds.
func segmentSecOwners(firstTime time.Time, interval, count int) map[string]int {
	return segmentOwners(firstTime, time.Duration(interval)*time.Second, count)
}

// segmentTimeOwners is segmentOwners for a timeline given as explicit instants
// rather than a regular grid -- a union of several windows, whose slots need not be
// evenly spaced. The latest instant owns a key two slots share.
func segmentTimeOwners(times []time.Time) map[string]int {
	owners := make(map[string]int, len(times))
	for i, t := range times {
		key := segmentKey(t)
		if j, ok := owners[key]; ok && times[j].After(t) {
			continue
		}
		owners[key] = i
	}
	return owners
}

// segmentAbs is an absolute instant for a label: month, day and UTC wall clock.
// The date is part of it because a window reaches back across midnight, and it is
// UTC rather than local so a label can be matched against the navigation key it
// describes.
func segmentAbs(t time.Time) string {
	return t.UTC().Format("Jan 2, 15:04Z")
}

// segmentLocal is the same instant in the reader's zone, for a description to
// carry alongside the UTC form. No date: it sits next to an absolute UTC label
// that already has one, and its job is only to answer "when was that for me".
func segmentLocal(t time.Time) string {
	return t.Local().Format("15:04 MST")
}

// segmentDescTime is a segment's start for a description: the reader's local clock
// only, because the row's own key already carries the UTC form and repeating it
// wastes the width the statistics need. No end time either -- every segment in a
// window is the same fixed size, stated once by the Coverage row.
//
// The date appears only when the window straddles midnight, which is the only time
// a bare wall clock is ambiguous; an hour view therefore never carries one.
func segmentDescTime(start time.Time, withDate bool) string {
	if withDate {
		return start.Local().Format("Jan 2, 15:04 MST")
	}
	return start.Local().Format("15:04 MST")
}

// windowCrossesDay reports whether a window spans more than one local day, so its
// segment descriptions need a date to stay unambiguous.
func windowCrossesDay(first time.Time, interval, count int) bool {
	if count <= 1 {
		return false
	}
	return first.Local().YearDay() != segmentStart(first, interval, count-1).Local().YearDay()
}

// segmentRowKey is a leaf-data key for one segment row: the numeric prefix carries
// the ordering and uniqueness, the rest is the segment's UTC start.
//
// Deliberately short. This is the narrow column of a two-column table, so a full
// span with a date wraps onto a second line and squeezes the value. The date and
// the span are already stated once by the Coverage row, and the interval is
// constant within a window, so the start alone locates the row -- in UTC, to match
// what an operator greps out of a log.
func segmentRowKey(idx int, start, _ time.Time) string {
	return fmt.Sprintf("%02d: %s", idx, start.UTC().Format("15:04Z"))
}

// windowCoverage states how much time a window or a segment covers and the instant
// it ends. A coverage duration plus one absolute end rather than a start-to-end
// pair: a whole-window entry is as wide as every slot it holds, which two bare
// wall-clock times would render as the zero-length range "13:45Z -> 13:45Z".
func windowCoverage(start time.Time, seconds int) string {
	d := time.Duration(seconds) * time.Second
	end := start.Add(d)
	return fmt.Sprintf("Covering %s, until %s (%s).",
		coverDuration(d), segmentAbs(end), segmentLocal(end))
}

// coverDuration renders a span the way an operator states one: "23h45m", not
// "23h45m0s".
func coverDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h, m := int(d/time.Hour), int(d/time.Minute)%60
	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%dh%dm", h, m)
	case h > 0:
		return strconv.Itoa(h) + "h"
	}
	return strconv.Itoa(m) + "m"
}

// windowStatus explains a window that has nothing to place on a timeline, or ""
// when it has segments and the caller should render them.
//
// The two states are kept apart because they are opposite answers: a nil window
// was never asked for, while an empty one was asked for and is still filling. A
// window that does hold segments is measured data, so its zeros are real and are
// rendered as such.
func windowStatus[T any, PT segPtr[T]](w *madmin.Segmented[T, PT], window string) string {
	switch {
	case w == nil:
		return "not collected: last-" + window + " stats were not requested"
	case w.Interval <= 0 || len(w.Segments) == 0:
		return "no data yet: the last-" + window + " window holds no completed segment"
	}
	return ""
}

// windowSummaryState is windowStatus for a one-line summary, so that a window
// which was never requested is never totaled into a measured zero.
func windowSummaryState[T any, PT segPtr[T]](w *madmin.Segmented[T, PT]) string {
	switch {
	case w == nil:
		return "not collected"
	case len(w.Segments) == 0:
		return "no data yet"
	}
	return ""
}

// formatTimedAction renders one timing on a single line. The mean is derived
// here; the wire carries only the count and the summed duration.
func formatTimedAction(a madmin.TimedAction) string {
	if a.Count == 0 {
		return "none"
	}
	return fmt.Sprintf("%d, avg %s, max %s", a.Count,
		durationOf(a.AccTime, a.Count),
		time.Duration(a.MaxTime).Round(time.Microsecond))
}

// formatNodeCount renders how many nodes contributed to a value.
//
// A segment's N is one sample per node, so it is a node count only for a single
// segment. Every value merged from more than one -- a whole-window _ALL entry,
// most of all -- carries one sample per node per segment, and reading that raw
// reports a five-node cluster as 300. Dividing by the number of segments merged
// is what turns it back into a node count.
//
// A fractional result is real rather than a rounding artifact: a node that joined
// or left mid-window contributed to only some of the segments. It is shown as an
// average instead of being hidden.
func formatNodeCount(n, segments int) string {
	if segments <= 1 {
		return fmt.Sprintf("%d node(s)", n)
	}
	avg := float64(n) / float64(segments)
	if avg == math.Trunc(avg) {
		return fmt.Sprintf("%d node(s)", int(avg))
	}
	return fmt.Sprintf("%.1f node(s) avg", avg)
}
