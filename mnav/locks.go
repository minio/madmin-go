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
	"strconv"
	"time"

	"github.com/minio/madmin-go/v4"
)

// LockMetricsNode is the navigation node for distributed-locking metrics.
type LockMetricsNode struct {
	locks  *madmin.LockMetrics
	parent MetricNode
	path   string
}

func (node *LockMetricsNode) collectionTime() time.Time {
	if node.locks == nil {
		return time.Time{}
	}
	return node.locks.CollectedAt
}

// NewLockMetricsNode constructs a new LockMetricsNode.
func NewLockMetricsNode(locks *madmin.LockMetrics, parent MetricNode, path string) *LockMetricsNode {
	return &LockMetricsNode{locks: locks, parent: parent, path: path}
}

func (node *LockMetricsNode) GetOpts() madmin.MetricsOptions   { return getNodeOpts(node) }
func (node *LockMetricsNode) GetMetricType() madmin.MetricType { return madmin.MetricsLocks }

// GetMetricFlags requests no historic window: this node refreshes continuously,
// and asking for them here would pull 60+96 segments on every tick to render two
// summary lines. They are fetched when the reader navigates into one.
func (node *LockMetricsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *LockMetricsNode) GetParent() MetricNode              { return node.parent }
func (node *LockMetricsNode) GetPath() string                    { return node.path }
func (node *LockMetricsNode) ShouldPauseRefresh() bool           { return false }

// GetChildren lists the cleanup pass only once it has sampled, but the windows
// unconditionally: their nodes explain a window that is missing or empty, so they
// are always constructible.
func (node *LockMetricsNode) GetChildren() []MetricChild {
	if node.locks == nil {
		return []MetricChild{}
	}
	children := make([]MetricChild, 0, 3)
	if node.locks.Purge != nil {
		children = append(children,
			MetricChild{Name: "purge", Description: "Values sampled by the periodic lock-cleanup pass"})
	}
	return append(children,
		MetricChild{Name: "last_hour", Description: "Locking activity over the last hour, by time segment"},
		MetricChild{Name: "last_day", Description: "Locking activity over the last day, by time segment"},
	)
}

func (node *LockMetricsNode) GetChild(name string) (MetricNode, error) {
	if node.locks == nil {
		return nil, fmt.Errorf("no lock data available")
	}
	switch name {
	case "purge":
		return NewLockPurgeNode(node.locks.Purge, node, fmt.Sprintf("%s/purge", node.path)), nil
	case "last_hour":
		return &lockWindowNode{
			seg: node.locks.LastHour, flags: madmin.MetricsHourStats, window: "hour",
			parent: node, path: fmt.Sprintf("%s/last_hour", node.path),
		}, nil
	case "last_day":
		return &lockWindowNode{
			seg: node.locks.LastDay, flags: madmin.MetricsDayStats, window: "day",
			parent: node, path: fmt.Sprintf("%s/last_day", node.path),
		}, nil
	}
	return nil, fmt.Errorf("child not found: %s", name)
}

func (node *LockMetricsNode) GetLeafData() map[string]string {
	if node.locks == nil {
		return map[string]string{"Status": "Lock metrics not available"}
	}
	l := node.locks
	data := map[string]string{
		"Collected At": l.CollectedAt.Format("2006-01-02 15:04:05"),
		"Nodes":        strconv.Itoa(l.Nodes),
		// Resources, not locks: one resource can carry many readers.
		"Locked Resources": strconv.FormatInt(l.Resources, 10),
	}
	// Always rendered, including at zero: "0 waiting, 0 failed" is the reassurance
	// an operator is looking for, and hiding it makes a healthy subsystem look
	// unreported.
	data["Waiting"] = strconv.FormatInt(l.Waiting, 10)

	// The caller's view over the last minute. Read and write are separate lines
	// because they queue for different reasons.
	data["Acquire (read)"] = formatTimedAction(l.Acquire.Read)
	data["Acquire (write)"] = formatTimedAction(l.Acquire.Write)
	data["Held (read)"] = formatTimedAction(l.Held.Read)
	data["Held (write)"] = formatTimedAction(l.Held.Write)
	data["Release (read)"] = formatTimedAction(l.Release.Read)
	data["Release (write)"] = formatTimedAction(l.Release.Write)

	// Wasted waiting is the point of showing AccTime here, so it is rendered even
	// when the count is zero.
	data["Failed"] = fmt.Sprintf("%d (%d timed out, %d conflicts, %d canceled)",
		l.AcquireFailed.Count(), l.TimedOut, l.Conflicts, l.Canceled)
	if wasted := l.AcquireFailed.Read.AccTime + l.AcquireFailed.Write.AccTime; wasted > 0 {
		data["Failed Waiting"] = time.Duration(wasted).Round(time.Millisecond).String()
	}

	if s := l.ServerLatency; s.N > 0 {
		data["Lock Server RPC"] = fmt.Sprintf("avg %s, max %s across %d servers",
			durationOf(s.AvgSumNanos, uint64(s.N)),
			time.Duration(s.MaxNanos).Round(time.Microsecond), s.N)
	}

	// Summarized when the history happens to be loaded -- it is persisted to disk
	// and restored on startup, and a child node may have fetched it -- but never
	// requested from here, and never rendered as a zero when it is absent.
	if sum := l.LastHour; sum != nil && len(sum.Segments) > 0 {
		data["Last Hour"] = formatLockWindow(sum)
	}
	if sum := l.LastDay; sum != nil && len(sum.Segments) > 0 {
		data["Last Day"] = formatLockWindow(sum)
	}

	// The purge block is also a child node, but the read/write split and the
	// oldest held lock are what you want at a glance, so summarize them here.
	if p := l.Purge; p != nil {
		data["Read / Write Locks"] = fmt.Sprintf("%d / %d", p.Readers, p.Writers)
		if !p.OldestHeldAt.IsZero() {
			data["Oldest Lock Held"] = ageStamp(node, p.OldestHeldAt, "15:04:05", time.Second)
		}
		if !p.SampledAt.IsZero() {
			data["Cleanup Sampled"] = ageStamp(node, p.SampledAt, "15:04:05", time.Second)
		}
	}
	return data
}

// formatLockWindow totals a segmented window into one line. The rare events are
// named only when they happened, so a quiet cluster reads as quiet rather than as
// a wall of zeros.
func formatLockWindow(w *madmin.SegmentedLockMetrics) string {
	if state := windowSummaryState(w); state != "" {
		return state
	}
	t := w.Total()
	out := fmt.Sprintf("%d acqs, avg %s", t.AcquireCount,
		durationOf(t.AcquireNanos, t.AcquireCount))
	if t.HeldCount > 0 {
		out += ", held " + durationOf(t.HeldNanos, t.HeldCount)
	}
	if t.AcquireFailed > 0 {
		out += fmt.Sprintf(", %d failed (%s)", t.AcquireFailed,
			time.Duration(t.AcquireFailedNanos).Round(time.Millisecond))
	}
	for _, rare := range []struct {
		label string
		n     uint64
	}{
		{"rejected", t.Rejected},
		{"expired", t.Expired},
		{"quorum", t.QuorumLost},
	} {
		if rare.n > 0 {
			out += fmt.Sprintf(", %d %s", rare.n, rare.label)
		}
	}
	return out
}

// lockSegmentEmpty reports a segment with nothing to show. N is not part of the
// test: a node that recorded no locking still reports itself in every slot, so an
// idle cluster would come back as a wall of zeros.
func lockSegmentEmpty(s *madmin.LockSegment) bool {
	return s.AcquireCount == 0 && s.AcquireFailed == 0 && s.HeldCount == 0 &&
		s.ReleaseCount == 0 && s.Rejected == 0 && s.Expired == 0 && s.QuorumLost == 0
}

// lockWindowNode is one persisted locking window -- the hour or the day -- as a
// row per time segment plus one navigable child per segment, so a burst can be
// told apart from a steady trickle.
type lockWindowNode struct {
	seg    *madmin.SegmentedLockMetrics
	flags  madmin.MetricFlags
	window string
	parent MetricNode
	path   string
}

func (node *lockWindowNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *lockWindowNode) GetMetricType() madmin.MetricType   { return madmin.MetricsLocks }
func (node *lockWindowNode) GetMetricFlags() madmin.MetricFlags { return node.flags }
func (node *lockWindowNode) GetParent() MetricNode              { return node.parent }
func (node *lockWindowNode) GetPath() string                    { return node.path }
func (node *lockWindowNode) ShouldPauseRefresh() bool           { return true }

// hasSegments reports whether the window can be placed on a timeline at all. An
// interval of zero would stamp every segment with the same time.
func (node *lockWindowNode) hasSegments() bool {
	return node.seg != nil && node.seg.Interval > 0 && len(node.seg.Segments) > 0
}

// wholeSecs is the span the whole window covers: one segment as wide as every
// slot it holds, so an _ALL rate reads as the window average.
func (node *lockWindowNode) wholeSecs() int {
	return node.seg.Interval * len(node.seg.Segments)
}

func (node *lockWindowNode) GetChildren() []MetricChild {
	if !node.hasSegments() {
		return []MetricChild{}
	}
	children := []MetricChild{{
		Name: "_ALL",
		Description: "Every segment in the window combined. " +
			windowCoverage(node.seg.FirstTime, node.wholeSecs()),
	}}
	owners := segmentSecOwners(node.seg.FirstTime, node.seg.Interval, len(node.seg.Segments))
	withDate := windowCrossesDay(node.seg.FirstTime, node.seg.Interval, len(node.seg.Segments))
	for i := range node.seg.Segments {
		s := node.seg.Segments[i]
		if lockSegmentEmpty(&s) {
			continue
		}
		start := segmentStart(node.seg.FirstTime, node.seg.Interval, i)
		name := segmentKey(start)
		if owners[name] != i {
			continue
		}
		children = append(children, MetricChild{
			Name:        name,
			Description: segmentDescTime(start, withDate) + ", " + describeLockSegment(s, node.seg.Interval),
		})
	}
	return children
}

func (node *lockWindowNode) GetChild(name string) (MetricNode, error) {
	if !node.hasSegments() {
		return nil, fmt.Errorf("no last-%s lock segments available", node.window)
	}
	leaf := func(s madmin.LockSegment, segTime time.Time, interval, segments int) MetricNode {
		return &lockSegmentLeafNode{
			seg: s, segTime: segTime, interval: interval, segments: segments,
			flags: node.flags, parent: node, path: node.path + "/" + name,
		}
	}
	if name == "_ALL" {
		return leaf(node.seg.Total(), node.seg.FirstTime, node.wholeSecs(), len(node.seg.Segments)), nil
	}
	owners := segmentSecOwners(node.seg.FirstTime, node.seg.Interval, len(node.seg.Segments))
	if i, ok := owners[name]; ok {
		return leaf(node.seg.Segments[i], segmentStart(node.seg.FirstTime, node.seg.Interval, i), node.seg.Interval, 1), nil
	}
	return nil, fmt.Errorf("time segment not found: %s", name)
}

func (node *lockWindowNode) GetLeafData() map[string]string {
	if status := windowStatus(node.seg, node.window); status != "" {
		return map[string]string{"Status": status}
	}
	data := map[string]string{
		"00:Total (last " + node.window + ")": formatLockWindow(node.seg),
		"01:Coverage":                         windowCoverage(node.seg.FirstTime, node.wholeSecs()),
	}
	idx := 2
	for i := range node.seg.Segments {
		s := node.seg.Segments[i]
		if lockSegmentEmpty(&s) {
			continue
		}
		start := segmentStart(node.seg.FirstTime, node.seg.Interval, i)
		end := start.Add(time.Duration(node.seg.Interval) * time.Second)
		data[segmentRowKey(idx, start, end)] = describeLockSegment(s, node.seg.Interval)
		idx++
	}
	// The totals above are measured zeros, so they stay; only the reason there is
	// no segment row is added.
	if idx == 2 {
		data["02:Segments"] = fmt.Sprintf("no locking recorded in any of the %d segment(s) measured",
			len(node.seg.Segments))
	}
	return data
}

// describeLockSegment renders one segment on a single line. Every mean is derived
// here; the wire carries only counts and summed durations.
func describeLockSegment(s madmin.LockSegment, interval int) string {
	out := fmt.Sprintf("%d acqs", s.AcquireCount)
	if interval > 0 {
		out += fmt.Sprintf(" (%.1f/s)", float64(s.AcquireCount)/float64(interval))
	}
	out += ", wait " + durationOf(s.AcquireNanos, s.AcquireCount)
	if s.HeldCount > 0 {
		out += ", held " + durationOf(s.HeldNanos, s.HeldCount)
	}
	if s.ReleaseCount > 0 {
		out += ", rel " + durationOf(s.ReleaseNanos, s.ReleaseCount)
	}
	if s.AcquireFailed > 0 {
		out += fmt.Sprintf(", %d failed (avg %s)", s.AcquireFailed,
			durationOf(s.AcquireFailedNanos, s.AcquireFailed))
	}
	for _, rare := range []struct {
		label string
		n     uint64
	}{
		{"rejected", s.Rejected},
		{"expired", s.Expired},
		{"quorum", s.QuorumLost},
	} {
		if rare.n > 0 {
			out += fmt.Sprintf(", %d %s", rare.n, rare.label)
		}
	}
	return out
}

// lockSegmentLeafNode is one lock segment, or the whole window combined.
type lockSegmentLeafNode struct {
	seg      madmin.LockSegment
	segTime  time.Time
	interval int
	// segments is how many segments were merged into seg, so N can be divided back
	// into a node count. One for a single segment, the whole window for _ALL.
	segments int
	flags    madmin.MetricFlags
	parent   MetricNode
	path     string
}

func (node *lockSegmentLeafNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *lockSegmentLeafNode) GetMetricType() madmin.MetricType   { return madmin.MetricsLocks }
func (node *lockSegmentLeafNode) GetMetricFlags() madmin.MetricFlags { return node.flags }
func (node *lockSegmentLeafNode) GetParent() MetricNode              { return node.parent }
func (node *lockSegmentLeafNode) GetPath() string                    { return node.path }
func (node *lockSegmentLeafNode) ShouldPauseRefresh() bool           { return true }
func (node *lockSegmentLeafNode) GetChildren() []MetricChild         { return []MetricChild{} }

func (node *lockSegmentLeafNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children")
}

func (node *lockSegmentLeafNode) GetLeafData() map[string]string {
	s := node.seg
	if lockSegmentEmpty(&s) {
		return map[string]string{"Status": "no locking activity in this time segment"}
	}
	data := map[string]string{}
	idx := 0
	add := func(k, v string) {
		data[fmt.Sprintf("%02d:%s", idx, k)] = v
		idx++
	}
	if node.interval > 0 {
		add("Time Segment", windowCoverage(node.segTime, node.interval))
	}
	add("Acquires", strconv.FormatUint(s.AcquireCount, 10))
	if node.interval > 0 {
		add("Grant Rate", fmt.Sprintf("%.2f/s", float64(s.AcquireCount)/float64(node.interval)))
	}
	add("Mean Acquire Wait", durationOf(s.AcquireNanos, s.AcquireCount))
	if s.HeldCount > 0 {
		add("Held", strconv.FormatUint(s.HeldCount, 10))
		add("Mean Hold", durationOf(s.HeldNanos, s.HeldCount))
	}
	// Near zero for a plain lock and real for the coalesced paths, which is why it
	// is worth its own row.
	if s.ReleaseCount > 0 {
		add("Released", strconv.FormatUint(s.ReleaseCount, 10))
		add("Mean Release", durationOf(s.ReleaseNanos, s.ReleaseCount))
	}
	if s.AcquireFailed > 0 {
		add("Failed", strconv.FormatUint(s.AcquireFailed, 10))
		add("Mean Failed Wait", durationOf(s.AcquireFailedNanos, s.AcquireFailed))
		// Attempts is the denominator: a grant and a failure are both attempts.
		add("Failure Share", calculatePercentage(s.AcquireFailed, s.AcquireCount+s.AcquireFailed))
	}
	for _, rare := range []struct {
		label string
		n     uint64
	}{
		{"Rejected", s.Rejected},
		{"Expired", s.Expired},
		{"Quorum Lost", s.QuorumLost},
	} {
		if rare.n > 0 {
			add(rare.label, strconv.FormatUint(rare.n, 10))
		}
	}
	// Context only: every field above is a cross-node sum, and dividing one by the
	// node count would invent a per-node figure that moves when a node joins or
	// leaves. N itself is per segment, so it is divided by the merge count.
	if s.N > 0 {
		add("Nodes", formatNodeCount(s.N, node.segments))
	}
	return data
}

// LockPurgeNode is what the periodic lock-cleanup pass observed.
type LockPurgeNode struct {
	purge  *madmin.LockPurgeStats
	parent MetricNode
	path   string
}

// NewLockPurgeNode constructs a new LockPurgeNode.
func NewLockPurgeNode(purge *madmin.LockPurgeStats, parent MetricNode, path string) *LockPurgeNode {
	return &LockPurgeNode{purge: purge, parent: parent, path: path}
}

func (node *LockPurgeNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *LockPurgeNode) GetMetricType() madmin.MetricType   { return madmin.MetricsLocks }
func (node *LockPurgeNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *LockPurgeNode) GetParent() MetricNode              { return node.parent }
func (node *LockPurgeNode) GetPath() string                    { return node.path }
func (node *LockPurgeNode) ShouldPauseRefresh() bool           { return false }
func (node *LockPurgeNode) GetChildren() []MetricChild         { return []MetricChild{} }

func (node *LockPurgeNode) GetChild(name string) (MetricNode, error) {
	return nil, fmt.Errorf("child not found: %s", name)
}

func (node *LockPurgeNode) GetLeafData() map[string]string {
	if node.purge == nil {
		return map[string]string{"Status": "no cleanup pass has run yet"}
	}
	p := node.purge
	data := map[string]string{
		// Up to one cleanup interval old; not comparable with the live counters.
		"Sampled At":  stampAge(node, p.SampledAt, "15:04:05", time.Second),
		"Read Locks":  strconv.FormatInt(p.Readers, 10),
		"Write Locks": strconv.FormatInt(p.Writers, 10),
	}
	if p.Expired > 0 {
		data["Expired (last pass)"] = strconv.FormatInt(p.Expired, 10)
	}
	// Age derived here; the wire carries only the timestamp. Without a collection
	// time to measure against, the acquisition time is all that can be shown.
	if !p.OldestHeldAt.IsZero() {
		stamp := p.OldestHeldAt.Format("15:04:05")
		if held, future, ok := since(node, p.OldestHeldAt); !ok {
			data["Oldest Lock"] = fmt.Sprintf("since %s", stamp)
		} else if future {
			data["Oldest Lock"] = fmt.Sprintf("acquired in %s (at %s)", held.Round(time.Second), stamp)
		} else {
			data["Oldest Lock"] = fmt.Sprintf("held %s (since %s)", held.Round(time.Second), stamp)
		}
	}
	return data
}
