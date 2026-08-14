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

// IAMMetricsNode is the navigation node for identity and access management.
type IAMMetricsNode struct {
	iam    *madmin.IAMMetrics
	parent MetricNode
	path   string
}

// NewIAMMetricsNode constructs a new IAMMetricsNode.
func NewIAMMetricsNode(iam *madmin.IAMMetrics, parent MetricNode, path string) *IAMMetricsNode {
	return &IAMMetricsNode{iam: iam, parent: parent, path: path}
}

func (node *IAMMetricsNode) GetOpts() madmin.MetricsOptions   { return getNodeOpts(node) }
func (node *IAMMetricsNode) GetMetricType() madmin.MetricType { return madmin.MetricsIAM }

// GetMetricFlags requests no historic window: this node refreshes continuously,
// and asking for them here would pull 60+96 segments on every tick to render two
// summary lines. They are fetched when the reader navigates into one.
func (node *IAMMetricsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *IAMMetricsNode) GetParent() MetricNode              { return node.parent }
func (node *IAMMetricsNode) GetPath() string                    { return node.path }
func (node *IAMMetricsNode) ShouldPauseRefresh() bool           { return false }

// GetChildren lists the windows unconditionally: their nodes explain a window
// that is missing or empty, so they are always constructible.
func (node *IAMMetricsNode) GetChildren() []MetricChild {
	if node.iam == nil {
		return []MetricChild{}
	}
	return []MetricChild{
		{Name: "last_hour", Description: "Authz and store activity over the last hour, by segment"},
		{Name: "last_day", Description: "Authz and store activity over the last day, by segment"},
	}
}

func (node *IAMMetricsNode) GetChild(name string) (MetricNode, error) {
	if node.iam == nil {
		return nil, fmt.Errorf("no IAM data available")
	}
	switch name {
	case "last_hour":
		return &iamWindowNode{
			seg: node.iam.LastHour, flags: madmin.MetricsHourStats, window: "hour",
			parent: node, path: fmt.Sprintf("%s/last_hour", node.path),
		}, nil
	case "last_day":
		return &iamWindowNode{
			seg: node.iam.LastDay, flags: madmin.MetricsDayStats, window: "day",
			parent: node, path: fmt.Sprintf("%s/last_day", node.path),
		}, nil
	}
	return nil, fmt.Errorf("child not found: %s", name)
}

func (node *IAMMetricsNode) GetLeafData() map[string]string {
	if node.iam == nil {
		return map[string]string{"Status": "IAM metrics not available"}
	}
	m := node.iam
	data := map[string]string{
		"Collected At": m.CollectedAt.Format("2006-01-02 15:04:05"),
		"Nodes":        strconv.Itoa(m.Nodes),
	}

	if c := m.Cache; c != nil {
		// Cluster-wide values, not per node.
		data["Policies"] = strconv.Itoa(c.Policies)
		data["Users"] = strconv.Itoa(c.RegularUsers)
		data["Service Accounts"] = fmt.Sprintf("%d (%d not parented to root)",
			c.ServiceAccounts, c.SvcAccNonRootParent)
		data["Groups"] = strconv.Itoa(c.Groups)
		data["Policy Mappings"] = fmt.Sprintf("%d user, %d group, %d STS",
			c.UserPolicyMappings, c.GroupPolicyMappings, c.STSPolicyMappings)
		// Cached, so it can lag the section's own timestamp.
		data["Inventory Sampled"] = fmt.Sprintf("%s ago",
			time.Since(c.SampledAt).Round(time.Second))
	} else {
		data["Inventory"] = "not available (node still initialising)"
	}

	// Authorization over the last minute, split by how the identity resolved: the
	// paths differ in cost by orders of magnitude, so one combined figure would hide
	// which kind of credential is slow.
	if a := m.Auth; a != nil {
		for _, path := range iamAuthPathOrder {
			if ta, ok := a.ByPath[path]; ok {
				data["Authz "+path] = formatTimedAction(ta)
			}
		}
		if a.Denied > 0 {
			data["Denied"] = strconv.FormatUint(a.Denied, 10)
		}
		if a.Errors > 0 {
			data["Unresolved"] = strconv.FormatUint(a.Errors, 10)
		}
		// Against the user-path count this is the hit rate, which is why both are
		// shown rather than a single derived percentage.
		if a.CacheMiss > 0 {
			data["Policy Cache Miss"] = strconv.FormatUint(a.CacheMiss, 10)
		}
	}

	// Persistence latency: the refresh and admin path, not the request path.
	for _, op := range sortedKeys(m.Store) {
		a := m.Store[op]
		data["Store "+op] = fmt.Sprintf("%d op(s), avg %s, max %s", a.Count,
			durationOf(a.AccTime, a.Count),
			time.Duration(a.MaxTime).Round(time.Millisecond))
	}

	// Summarized when the history happens to be loaded -- it is persisted to disk
	// and restored on startup, and a child node may have fetched it -- but never
	// requested from here, and never rendered as a zero when it is absent.
	if sum := m.LastHour; sum != nil && len(sum.Segments) > 0 {
		data["Last Hour"] = formatIAMWindow(sum)
	}
	if sum := m.LastDay; sum != nil && len(sum.Segments) > 0 {
		data["Last Day"] = formatIAMWindow(sum)
	}
	return data
}

// iamAuthPathOrder fixes the order the paths render in, so a row does not move
// between refreshes as the map iterates differently. It follows the order
// IsAllowed tests them in.
var iamAuthPathOrder = []string{
	madmin.IAMPathPlugin,
	madmin.IAMPathOwner,
	madmin.IAMPathSTS,
	madmin.IAMPathSvcAcct,
	madmin.IAMPathUser,
}

// iamSegmentPaths pairs each path with its accessors, so every renderer walks
// them in one order and a path added later cannot be missed by one of them.
func iamSegmentPaths(s madmin.IAMSegment) []struct {
	label string
	count uint64
	nanos uint64
} {
	return []struct {
		label string
		count uint64
		nanos uint64
	}{
		{madmin.IAMPathPlugin, s.PluginCount, s.PluginNanos},
		{madmin.IAMPathOwner, s.OwnerCount, s.OwnerNanos},
		{madmin.IAMPathSTS, s.STSCount, s.STSNanos},
		{madmin.IAMPathSvcAcct, s.SvcAcctCount, s.SvcAcctNanos},
		{madmin.IAMPathUser, s.UserCount, s.UserNanos},
	}
}

// iamStoreOps pairs each persistence operation with its accessors.
func iamStoreOps(s madmin.IAMSegment) []struct {
	label string
	count uint64
	nanos uint64
} {
	return []struct {
		label string
		count uint64
		nanos uint64
	}{
		{"save", s.SaveCount, s.SaveNanos},
		{"load", s.LoadCount, s.LoadNanos},
		{"delete", s.DeleteCount, s.DeleteNanos},
		{"list", s.ListCount, s.ListNanos},
	}
}

// formatIAMWindow totals a segmented window into one line.
func formatIAMWindow(w *madmin.SegmentedIAMMetrics) string {
	if state := windowSummaryState(w); state != "" {
		return state
	}
	return describeIAMSegment(w.Total(), 0)
}

// iamSegmentEmpty reports a segment with nothing to show. N is not part of the
// test: a node that authorized nothing still reports itself in every slot, so an
// idle cluster would come back as a wall of zeros.
func iamSegmentEmpty(s *madmin.IAMSegment) bool {
	return s.AuthCount() == 0 && s.StoreCount() == 0 && s.Errors == 0 && s.StoreErrors == 0
}

// describeIAMSegment renders one segment on a single line, kept short enough to
// sit in a row label. The dominant path is named rather than every path: with a
// plugin configured it answers everything, and without one the traffic is almost
// always a single credential kind.
func describeIAMSegment(s madmin.IAMSegment, interval int) string {
	n := s.AuthCount()
	out := fmt.Sprintf("%d authz", n)
	if interval > 0 {
		out += fmt.Sprintf(" (%.1f/s)", float64(n)/float64(interval))
	}
	if n > 0 {
		out += ", avg " + durationOf(s.AuthNanos(), n)
		if top := topIAMPath(s); top != "" {
			out += ", " + top
		}
	}
	if s.MaxNanos > 0 {
		out += ", max " + time.Duration(s.MaxNanos).Round(time.Microsecond).String()
	}
	if s.Denied > 0 {
		out += fmt.Sprintf(", %d denied", s.Denied)
	}
	if s.Errors > 0 {
		out += fmt.Sprintf(", %d unresolved", s.Errors)
	}
	if store := s.StoreCount(); store > 0 {
		out += fmt.Sprintf(", %d store (avg %s)", store,
			durationOf(s.StoreNanos(), store))
	}
	if s.StoreErrors > 0 {
		out += fmt.Sprintf(", %d store err", s.StoreErrors)
	}
	return out
}

// topIAMPath names the busiest path and its share, empty when one path took
// everything: "100% user" says nothing a single-path deployment does not already
// know.
func topIAMPath(s madmin.IAMSegment) string {
	total := s.AuthCount()
	if total == 0 {
		return ""
	}
	var best struct {
		label string
		count uint64
	}
	distinct := 0
	for _, p := range iamSegmentPaths(s) {
		if p.count == 0 {
			continue
		}
		distinct++
		if p.count > best.count {
			best.label, best.count = p.label, p.count
		}
	}
	if distinct < 2 {
		return best.label
	}
	return fmt.Sprintf("%s %s", best.label, calculatePercentage(best.count, total))
}

// iamWindowNode is one persisted IAM window -- the hour or the day -- as a row
// per time segment plus one navigable child per segment, so a burst of slow
// authorizations can be told apart from a steady cost.
type iamWindowNode struct {
	seg    *madmin.SegmentedIAMMetrics
	flags  madmin.MetricFlags
	window string
	parent MetricNode
	path   string
}

func (node *iamWindowNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *iamWindowNode) GetMetricType() madmin.MetricType   { return madmin.MetricsIAM }
func (node *iamWindowNode) GetMetricFlags() madmin.MetricFlags { return node.flags }
func (node *iamWindowNode) GetParent() MetricNode              { return node.parent }
func (node *iamWindowNode) GetPath() string                    { return node.path }
func (node *iamWindowNode) ShouldPauseRefresh() bool           { return true }

// hasSegments reports whether the window can be placed on a timeline at all. An
// interval of zero would stamp every segment with the same time.
func (node *iamWindowNode) hasSegments() bool {
	return node.seg != nil && node.seg.Interval > 0 && len(node.seg.Segments) > 0
}

// wholeSecs is the span the whole window covers: one segment as wide as every
// slot it holds, so an _ALL rate reads as the window average.
func (node *iamWindowNode) wholeSecs() int {
	return node.seg.Interval * len(node.seg.Segments)
}

func (node *iamWindowNode) GetChildren() []MetricChild {
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
		if iamSegmentEmpty(&s) {
			continue
		}
		start := segmentStart(node.seg.FirstTime, node.seg.Interval, i)
		name := segmentKey(start)
		if owners[name] != i {
			continue
		}
		children = append(children, MetricChild{
			Name:        name,
			Description: segmentDescTime(start, withDate) + ", " + describeIAMSegment(s, node.seg.Interval),
		})
	}
	return children
}

func (node *iamWindowNode) GetChild(name string) (MetricNode, error) {
	if !node.hasSegments() {
		return nil, fmt.Errorf("no last-%s IAM segments available", node.window)
	}
	leaf := func(s madmin.IAMSegment, segTime time.Time, interval, segments int) MetricNode {
		return &iamSegmentLeafNode{
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

func (node *iamWindowNode) GetLeafData() map[string]string {
	if status := windowStatus(node.seg, node.window); status != "" {
		return map[string]string{"Status": status}
	}
	data := map[string]string{
		"00:Total (last " + node.window + ")": formatIAMWindow(node.seg),
		"01:Coverage":                         windowCoverage(node.seg.FirstTime, node.wholeSecs()),
	}
	idx := 2
	for i := range node.seg.Segments {
		s := node.seg.Segments[i]
		if iamSegmentEmpty(&s) {
			continue
		}
		start := segmentStart(node.seg.FirstTime, node.seg.Interval, i)
		end := start.Add(time.Duration(node.seg.Interval) * time.Second)
		data[segmentRowKey(idx, start, end)] = describeIAMSegment(s, node.seg.Interval)
		idx++
	}
	// The totals above are measured zeros, so they stay; only the reason there is
	// no segment row is added.
	if idx == 2 {
		data["02:Segments"] = fmt.Sprintf("no IAM activity recorded in any of the %d segment(s) measured",
			len(node.seg.Segments))
	}
	return data
}

// iamSegmentLeafNode is one IAM segment, or the whole window combined.
type iamSegmentLeafNode struct {
	seg      madmin.IAMSegment
	segTime  time.Time
	interval int
	// segments is how many segments were merged into seg, so N can be divided back
	// into a node count. One for a single segment, the whole window for _ALL.
	segments int
	flags    madmin.MetricFlags
	parent   MetricNode
	path     string
}

func (node *iamSegmentLeafNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *iamSegmentLeafNode) GetMetricType() madmin.MetricType   { return madmin.MetricsIAM }
func (node *iamSegmentLeafNode) GetMetricFlags() madmin.MetricFlags { return node.flags }
func (node *iamSegmentLeafNode) GetParent() MetricNode              { return node.parent }
func (node *iamSegmentLeafNode) GetPath() string                    { return node.path }
func (node *iamSegmentLeafNode) ShouldPauseRefresh() bool           { return true }
func (node *iamSegmentLeafNode) GetChildren() []MetricChild         { return []MetricChild{} }

func (node *iamSegmentLeafNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children")
}

func (node *iamSegmentLeafNode) GetLeafData() map[string]string {
	s := node.seg
	if iamSegmentEmpty(&s) {
		return map[string]string{"Status": "no IAM activity in this time segment"}
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

	total := s.AuthCount()
	add("Authorizations", strconv.FormatUint(total, 10))
	if node.interval > 0 && total > 0 {
		add("Rate", fmt.Sprintf("%.2f/s", float64(total)/float64(node.interval)))
	}
	if total > 0 {
		add("Mean Latency", durationOf(s.AuthNanos(), total))
	}
	// The peak, not a per-path peak: whichever path is configured answers every
	// request, so there is at most one path in contention for it.
	if s.MaxNanos > 0 {
		add("Slowest", time.Duration(s.MaxNanos).Round(time.Microsecond).String())
	}
	// One row per path that saw traffic, with its share, since that is the whole
	// reason the split exists.
	for _, p := range iamSegmentPaths(s) {
		if p.count == 0 {
			continue
		}
		add("Path "+p.label, fmt.Sprintf("%d (%s), avg %s", p.count,
			calculatePercentage(p.count, total), durationOf(p.nanos, p.count)))
	}
	if s.Denied > 0 {
		add("Denied", fmt.Sprintf("%d (%s)", s.Denied, calculatePercentage(s.Denied, total)))
	}
	// Unresolved lookups never reached a policy decision, so they are not part of
	// the counts above and get no latency of their own.
	if s.Errors > 0 {
		add("Unresolved", strconv.FormatUint(s.Errors, 10))
	}
	if s.CacheMiss > 0 {
		add("Policy Cache Miss", fmt.Sprintf("%d of %d user authz", s.CacheMiss, s.UserCount))
	}

	// Persistence is the refresh and admin path, and infrequent enough that these
	// segments are the only place it can be seen at all.
	for _, op := range iamStoreOps(s) {
		if op.count == 0 {
			continue
		}
		add("Store "+op.label, fmt.Sprintf("%d, avg %s", op.count,
			durationOf(op.nanos, op.count)))
	}
	if s.StoreErrors > 0 {
		add("Store Errors", strconv.FormatUint(s.StoreErrors, 10))
	}

	// Context only: every field above is a cross-node sum, and dividing one by the
	// node count would invent a per-node figure that moves when a node joins or
	// leaves. N itself is per segment, so it is divided by the merge count.
	if s.N > 0 {
		add("Nodes", formatNodeCount(s.N, node.segments))
	}
	return data
}
