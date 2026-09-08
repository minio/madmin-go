// Copyright (c) 2015-2025 MinIO, Inc.
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
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/minio/madmin-go/v4"
)

// formatMemoryBytes formats bytes in human readable format for memory
func formatMemoryBytes(bytes uint64) string {
	return humanize.Bytes(bytes)
}

// calculatePercentage calculates percentage with 1 decimal place
func calculatePercentage(used, total uint64) string {
	if total == 0 {
		return "0.0%"
	}
	percentage := float64(used) / float64(total) * 100
	return fmt.Sprintf("%.1f%%", percentage)
}

// MemMetricsNavigator provides navigation for Memory metrics
type MemMetricsNavigator struct {
	mem    *madmin.MemMetrics
	parent MetricNode
	path   string
}

func (node *MemMetricsNavigator) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

// NewMemMetricsNavigator creates a new memory metrics navigator
func NewMemMetricsNavigator(mem *madmin.MemMetrics, parent MetricNode, path string) *MemMetricsNavigator {
	return &MemMetricsNavigator{mem: mem, parent: parent, path: path}
}

func (node *MemMetricsNavigator) GetChildren() []MetricChild {
	if node.mem == nil {
		return []MetricChild{}
	}
	children := make([]MetricChild, 0, 9)
	children = append(children,
		MetricChild{Name: "usage", Description: "Core memory usage statistics and utilization"},
		MetricChild{Name: "system", Description: "System memory details (cache, buffers, shared)"},
		MetricChild{Name: "swap", Description: "Swap space information and utilization"},
		MetricChild{Name: "limits", Description: "Memory limits and cgroup configuration"},
		MetricChild{Name: "last_hour", Description: "Memory levels and kernel reclaim over the last hour, by time segment"},
		MetricChild{Name: "last_day", Description: "Memory levels and kernel reclaim over the last day, by time segment"},
	)
	if node.mem.VMStat != nil {
		children = append(children,
			MetricChild{Name: "vmstat", Description: "Kernel memory-management counters since boot"})
	}
	if node.mem.Cgroup != nil {
		children = append(children,
			MetricChild{Name: "cgroup", Description: "cgroup-v2 memory accounting and OOM events"})
	}
	if node.mem.ECC != nil {
		children = append(children,
			MetricChild{Name: "ecc", Description: "ECC correctable/uncorrectable error rollup"})
	}
	if node.mem.Fragmentation != nil {
		children = append(children,
			MetricChild{Name: "fragmentation", Description: "Free memory reachable as contiguous runs"})
	}
	return children
}

// memRows builds leaf data in display order, dividing every summed value back
// down to one node.
//
// MemInfo.Merge sums every field, so a 17-node cluster reports 2.2 TB of RAM and
// nothing an operator can compare against a machine. nodes is what makes a row
// per-node; the share of the cluster total is the same number either way, so both
// are stated.
type memRows struct {
	data  map[string]string
	n     int
	nodes int
}

func newMemRows(nodes int) *memRows {
	return &memRows{data: make(map[string]string), nodes: max(nodes, 1)}
}

func (r *memRows) add(label, value string) {
	r.data[fmt.Sprintf("%02d:%s", r.n, label)] = value
	r.n++
}

// total states a summed value as the cluster figure with the per-node mean, and
// its share of whole where there is one. A value nobody reported is skipped
// rather than rendered as a measured zero.
func (r *memRows) total(label string, v, whole uint64) {
	if v == 0 {
		return
	}
	r.add(label, fmtBytes(float64(v))+qualify(r.nodes, "node", float64(v), float64(whole), fmtBytes))
}

func (node *MemMetricsNavigator) GetLeafData() map[string]string {
	if node.mem == nil {
		return map[string]string{"Status": "Memory metrics not available"}
	}
	info := node.mem.Info
	r := newMemRows(node.mem.Nodes)
	r.add("Collected At", node.mem.CollectedAt.Format("2006-01-02 15:04:05"))
	if node.mem.Nodes > 0 {
		r.add("Nodes", formatNodeCount(node.mem.Nodes, 1))
	}
	r.total("Total", info.Total, 0)
	// Available, not Free, is the number that says whether a host is about to
	// run out: Linux lends everything it is not using to the page cache, so Free
	// reads alarmingly low on a perfectly healthy machine.
	r.total("Used", info.Used, info.Total)
	r.total("Available", info.Available, info.Total)
	r.total("Free", info.Free, info.Total)
	r.total("Cache", info.Cache, info.Total)
	if info.SwapSpaceTotal > 0 {
		r.total("Swap Used", info.SwapSpaceTotal-info.SwapSpaceFree, info.SwapSpaceTotal)
	}
	return r.data
}

func (node *MemMetricsNavigator) GetMetricType() madmin.MetricType {
	return madmin.MetricsMem
}

func (node *MemMetricsNavigator) GetMetricFlags() madmin.MetricFlags {
	return 0
}

func (node *MemMetricsNavigator) GetParent() MetricNode {
	return node.parent
}

func (node *MemMetricsNavigator) GetPath() string {
	return node.path
}

func (node *MemMetricsNavigator) ShouldPauseRefresh() bool {
	return false
}

func (node *MemMetricsNavigator) GetChild(name string) (MetricNode, error) {
	if node.mem == nil {
		return nil, fmt.Errorf("no memory data available")
	}

	switch name {
	case "usage":
		return NewMemUsageNode(node.mem, node, fmt.Sprintf("%s/usage", node.path)), nil
	case "system":
		return NewMemSystemNode(node.mem, node, fmt.Sprintf("%s/system", node.path)), nil
	case "swap":
		return NewMemSwapNode(node.mem, node, fmt.Sprintf("%s/swap", node.path)), nil
	case "limits":
		return NewMemLimitsNode(node.mem, node, fmt.Sprintf("%s/limits", node.path)), nil
	case "last_hour":
		return &MemLastDayNode{
			segmented: node.mem.LastHour, flags: madmin.MetricsHourStats, window: "hour",
			parent: node, path: fmt.Sprintf("%s/last_hour", node.path),
		}, nil
	case "last_day":
		return NewMemLastDayNode(node.mem.LastDay, node, fmt.Sprintf("%s/last_day", node.path)), nil
	case "vmstat":
		return NewMemVMStatNode(node.mem.VMStat, node, fmt.Sprintf("%s/vmstat", node.path)), nil
	case "cgroup":
		return NewMemCgroupNode(node.mem.Cgroup, node, fmt.Sprintf("%s/cgroup", node.path)), nil
	case "ecc":
		return NewMemECCNode(node.mem.ECC, node, fmt.Sprintf("%s/ecc", node.path)), nil
	case "fragmentation":
		return NewMemFragNode(node.mem.Fragmentation, node, fmt.Sprintf("%s/fragmentation", node.path)), nil
	default:
		return nil, fmt.Errorf("child not found: %s", name)
	}
}

// MemUsageNode handles core memory usage statistics
type MemUsageNode struct {
	mem    *madmin.MemMetrics
	parent MetricNode
	path   string
}

func (node *MemUsageNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewMemUsageNode(mem *madmin.MemMetrics, parent MetricNode, path string) *MemUsageNode {
	return &MemUsageNode{mem: mem, parent: parent, path: path}
}

func (node *MemUsageNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *MemUsageNode) GetLeafData() map[string]string {
	if node.mem == nil {
		return map[string]string{"Status": "Memory usage metrics not available"}
	}
	info := node.mem.Info
	if info.Total == 0 {
		return map[string]string{"Status": "No memory usage reported"}
	}
	r := newMemRows(node.mem.Nodes)
	r.total("Total", info.Total, 0)
	r.total("Used", info.Used, info.Total)
	// Available is what an allocation can actually get: it counts the reclaimable
	// page cache that Free does not.
	r.total("Available", info.Available, info.Total)
	r.total("Free", info.Free, info.Total)
	r.total("Cache", info.Cache, info.Total)
	r.total("Buffers", info.Buffers, info.Total)
	r.total("Shared", info.Shared, info.Total)
	return r.data
}

func (node *MemUsageNode) GetMetricType() madmin.MetricType   { return madmin.MetricsMem }
func (node *MemUsageNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *MemUsageNode) GetParent() MetricNode              { return node.parent }
func (node *MemUsageNode) GetPath() string                    { return node.path }

func (node *MemUsageNode) ShouldPauseRefresh() bool {
	return false
}

func (node *MemUsageNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("memory usage node has no children")
}

// MemSystemNode handles system memory details
type MemSystemNode struct {
	mem    *madmin.MemMetrics
	parent MetricNode
	path   string
}

func (node *MemSystemNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewMemSystemNode(mem *madmin.MemMetrics, parent MetricNode, path string) *MemSystemNode {
	return &MemSystemNode{mem: mem, parent: parent, path: path}
}

func (node *MemSystemNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *MemSystemNode) GetLeafData() map[string]string {
	if node.mem == nil {
		return map[string]string{"Status": "System memory metrics not available"}
	}
	info := node.mem.Info
	if info.Cache == 0 && info.Buffers == 0 && info.Shared == 0 {
		return map[string]string{"Status": "No cache, buffer or shared memory reported"}
	}
	r := newMemRows(node.mem.Nodes)
	r.total("Cache", info.Cache, info.Total)
	r.total("Buffers", info.Buffers, info.Total)
	r.total("Shared", info.Shared, info.Total)
	// Cache plus buffers is the memory the kernel will hand back under pressure,
	// which is why it is worth a line of its own next to the total.
	r.total("Reclaimable", info.Cache+info.Buffers, info.Total)
	return r.data
}

func (node *MemSystemNode) GetMetricType() madmin.MetricType   { return madmin.MetricsMem }
func (node *MemSystemNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *MemSystemNode) GetParent() MetricNode              { return node.parent }
func (node *MemSystemNode) GetPath() string                    { return node.path }

func (node *MemSystemNode) ShouldPauseRefresh() bool {
	return false
}

func (node *MemSystemNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("system memory node has no children")
}

// MemSwapNode handles swap space analysis
type MemSwapNode struct {
	mem    *madmin.MemMetrics
	parent MetricNode
	path   string
}

func (node *MemSwapNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewMemSwapNode(mem *madmin.MemMetrics, parent MetricNode, path string) *MemSwapNode {
	return &MemSwapNode{mem: mem, parent: parent, path: path}
}

func (node *MemSwapNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *MemSwapNode) GetLeafData() map[string]string {
	if node.mem == nil {
		return map[string]string{"Status": "Swap space metrics not available"}
	}
	info := node.mem.Info
	if info.SwapSpaceTotal == 0 {
		return map[string]string{"Status": "no swap configured on any node"}
	}
	r := newMemRows(node.mem.Nodes)
	r.total("Total", info.SwapSpaceTotal, 0)
	// Rendered even at zero: "none in use" is the reassurance being looked for,
	// and hiding it makes a healthy cluster look unreported.
	used := info.SwapSpaceTotal - info.SwapSpaceFree
	r.add("Used", fmtBytes(float64(used))+qualify(r.nodes, "node", float64(used), float64(info.SwapSpaceTotal), fmtBytes))
	r.total("Free", info.SwapSpaceFree, info.SwapSpaceTotal)
	if info.Total > 0 {
		r.add("Swap : RAM", fmt.Sprintf("%.2f : 1", float64(info.SwapSpaceTotal)/float64(info.Total)))
	}
	return r.data
}

func (node *MemSwapNode) GetMetricType() madmin.MetricType   { return madmin.MetricsMem }
func (node *MemSwapNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *MemSwapNode) GetParent() MetricNode              { return node.parent }
func (node *MemSwapNode) GetPath() string                    { return node.path }

func (node *MemSwapNode) ShouldPauseRefresh() bool {
	return false
}

func (node *MemSwapNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("swap memory node has no children")
}

// MemLimitsNode handles memory limits and cgroup configuration
type MemLimitsNode struct {
	mem    *madmin.MemMetrics
	parent MetricNode
	path   string
}

func (node *MemLimitsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewMemLimitsNode(mem *madmin.MemMetrics, parent MetricNode, path string) *MemLimitsNode {
	return &MemLimitsNode{mem: mem, parent: parent, path: path}
}

func (node *MemLimitsNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *MemLimitsNode) GetLeafData() map[string]string {
	if node.mem == nil {
		return map[string]string{"Status": "Memory limits metrics not available"}
	}
	info := node.mem.Info
	// Limit is set to Total where no cgroup limit applies, so the two being equal
	// is how "unlimited" arrives on the wire rather than a zero.
	if info.Limit == 0 || info.Limit == info.Total {
		return map[string]string{"Status": "no cgroup memory limit configured"}
	}
	r := newMemRows(node.mem.Nodes)
	r.total("Limit", info.Limit, info.Total)
	r.total("Physical", info.Total, 0)
	if info.Used > 0 {
		r.total("Used", info.Used, info.Limit)
		if info.Limit > info.Used {
			r.total("Headroom", info.Limit-info.Used, info.Limit)
		} else {
			r.add("Headroom", "none: usage is at or above the limit")
		}
	}
	if info.Limit > info.Total {
		r.add("Note", "the limit exceeds physical memory, so it cannot bind")
	}
	return r.data
}

func (node *MemLimitsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsMem }
func (node *MemLimitsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *MemLimitsNode) GetParent() MetricNode              { return node.parent }
func (node *MemLimitsNode) GetPath() string                    { return node.path }

func (node *MemLimitsNode) ShouldPauseRefresh() bool {
	return false
}

func (node *MemLimitsNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("memory limits node has no children")
}

// memWindowRow is a value worth a row in a memory window. A gauge reads as a
// level; a counter is a per-segment delta on the wire and earns a rate.
type memWindowRow struct {
	label   string
	counter bool
	brief   bool
	unit    string
	render  func(float64) string
	value   func(madmin.MemSegment) uint64
}

var memWindowRows = []memWindowRow{
	{"Used", false, true, "", fmtBytes, func(m madmin.MemSegment) uint64 { return m.Used }},
	{"Available", false, true, "", fmtBytes, func(m madmin.MemSegment) uint64 { return m.Available }},
	{"Free", false, false, "", fmtBytes, func(m madmin.MemSegment) uint64 { return m.Free }},
	{"Limit", false, false, "", fmtBytes, func(m madmin.MemSegment) uint64 { return m.Limit }},
	// The kernel's own account of what the pressure cost. Every one of these is
	// a delta over the segment, so a segment is comparable to its neighbours.
	{"Swap In", true, true, "", fmtBytes, func(m madmin.MemSegment) uint64 { return m.SwapInBytes }},
	{"Swap Out", true, true, "", fmtBytes, func(m madmin.MemSegment) uint64 { return m.SwapOutBytes }},
	{"Major Faults", true, true, "", fmtCount, func(m madmin.MemSegment) uint64 { return m.MajorFaults }},
	{"Workingset Refault", true, false, "", fmtCount, func(m madmin.MemSegment) uint64 { return m.WorkingsetRefault }},
	{"Compaction Stalls", true, false, "", fmtCount, func(m madmin.MemSegment) uint64 { return m.CompactStall }},
	{"OOM Kills", true, true, "", fmtCount, func(m madmin.MemSegment) uint64 { return m.OOMKill }},
}

// memSegmentRows renders one segment, or a whole window, as leaf data.
//
// Every field is summed over the samples folded in -- N of them, one per node per
// segment -- so the quotient is a mean per sample and one sample covers one
// interval. That makes interval the divisor for every rate here even when the
// whole window is rendered; segments is what turns N back into a node count and
// scales a counter's total to one node's share of the window.
func memSegmentRows(seg madmin.MemSegment, interval, segments int, coverage string) map[string]string {
	if seg.N == 0 {
		return map[string]string{"Status": "no node reported this time segment"}
	}
	n := float64(seg.N)
	r := newMemRows(1)
	r.add("Coverage", coverage)
	r.add("Nodes", formatNodeCount(seg.N, segments)+" reporting")

	total := float64(seg.Used+seg.Free) / n
	for _, row := range memWindowRows {
		v := float64(row.value(seg))
		if v == 0 {
			continue
		}
		if !row.counter {
			r.add(row.label, fmtBytes(v/n)+" per node"+percentOf(v/n, total))
			continue
		}
		// A counter's window total is one node's share of every segment folded
		// in; its rate is that spread over the segments it accrued across.
		value := row.render(v*float64(max(segments, 1))/n) + " per node"
		if interval > 0 {
			value += ", " + fmtRate(v/n/float64(interval), row.render, row.unit)
		}
		r.add(row.label, value)
	}

	// Fragmentation carries its own divisor: buddyinfo can be unreadable on a
	// host whose meminfo is fine, so dividing by N would under-report by the
	// share of hosts without it.
	if seg.FragN > 0 && seg.FragFreeBytes > 0 {
		f := float64(seg.FragN)
		r.add("Free (Fragmented)", fmtBytes(float64(seg.FragFreeBytes)/f)+" per node, "+
			fmtPct(float64(seg.FragFreeBytes-seg.FragFreeBytesLarge), float64(seg.FragFreeBytes))+" unusable")
	}
	return r.data
}

// percentOf is the parenthesised share a level carries next to its value.
func percentOf(v, whole float64) string {
	if whole <= 0 {
		return ""
	}
	return " (" + fmtPct(v, whole) + ")"
}

// describeMemSegment renders one segment on a single line, per node.
func describeMemSegment(seg madmin.MemSegment) string {
	n := float64(seg.N)
	var parts []string
	for _, row := range memWindowRows {
		v := float64(row.value(seg))
		if !row.brief || v == 0 {
			continue
		}
		if row.counter {
			parts = append(parts, row.label+" +"+row.render(v/n))
			continue
		}
		parts = append(parts, row.label+" "+row.render(v/n))
	}
	if len(parts) == 0 {
		return formatNodeCount(seg.N, 1) + ", nothing recorded"
	}
	return strings.Join(parts, ", ") + " per node"
}

// MemLastDayNode is one persisted memory window -- the hour or the day -- as a
// navigable child per time segment plus an _ALL entry over the whole span.
type MemLastDayNode struct {
	segmented *madmin.SegmentedMemMetrics
	flags     madmin.MetricFlags
	window    string
	parent    MetricNode
	path      string
}

// NewMemLastDayNode creates a navigator for the last-day memory window.
func NewMemLastDayNode(segmented *madmin.SegmentedMemMetrics, parent MetricNode, path string) *MemLastDayNode {
	return &MemLastDayNode{
		segmented: segmented, flags: madmin.MetricsDayStats, window: "day",
		parent: parent, path: path,
	}
}

func (node *MemLastDayNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *MemLastDayNode) GetPath() string                    { return node.path }
func (node *MemLastDayNode) GetParent() MetricNode              { return node.parent }
func (node *MemLastDayNode) GetMetricType() madmin.MetricType   { return madmin.MetricsMem }
func (node *MemLastDayNode) GetMetricFlags() madmin.MetricFlags { return node.flags }
func (node *MemLastDayNode) ShouldPauseRefresh() bool           { return true }

// hasSegments reports whether the window can be placed on a timeline at all. An
// interval of zero would stamp every segment with the same time.
func (node *MemLastDayNode) hasSegments() bool {
	return node.segmented != nil && node.segmented.Interval > 0 && len(node.segmented.Segments) > 0
}

func (node *MemLastDayNode) wholeSecs() int {
	return node.segmented.Interval * len(node.segmented.Segments)
}

func (node *MemLastDayNode) GetChildren() []MetricChild {
	if !node.hasSegments() {
		return []MetricChild{}
	}
	children := []MetricChild{{
		Name: "_ALL",
		Description: "Every segment in the window combined. " +
			windowCoverage(node.segmented.FirstTime, node.wholeSecs()),
	}}
	owners := segmentSecOwners(node.segmented.FirstTime, node.segmented.Interval, len(node.segmented.Segments))
	withDate := windowCrossesDay(node.segmented.FirstTime, node.segmented.Interval, len(node.segmented.Segments))
	// Newest first, as every other family lists a window.
	for i := len(node.segmented.Segments) - 1; i >= 0; i-- {
		seg := node.segmented.Segments[i]
		if seg.N == 0 {
			continue
		}
		start := segmentStart(node.segmented.FirstTime, node.segmented.Interval, i)
		name := segmentKey(start)
		if owners[name] != i {
			continue
		}
		children = append(children, MetricChild{
			Name:        name,
			Description: segmentDescTime(start, withDate) + ", " + describeMemSegment(seg),
		})
	}
	return children
}

func (node *MemLastDayNode) GetChild(name string) (MetricNode, error) {
	if !node.hasSegments() {
		return nil, fmt.Errorf("no last-%s memory segments available", node.window)
	}
	if name == "_ALL" {
		return &MemTimeSegmentNode{
			segment:  node.segmented.Total(),
			interval: node.segmented.Interval,
			segments: len(node.segmented.Segments),
			coverage: windowCoverage(node.segmented.FirstTime, node.wholeSecs()),
			flags:    node.flags, parent: node, path: node.path + "/" + name,
		}, nil
	}
	owners := segmentSecOwners(node.segmented.FirstTime, node.segmented.Interval, len(node.segmented.Segments))
	if i, ok := owners[name]; ok {
		start := segmentStart(node.segmented.FirstTime, node.segmented.Interval, i)
		return &MemTimeSegmentNode{
			segment:  node.segmented.Segments[i],
			interval: node.segmented.Interval,
			segments: 1,
			coverage: windowCoverage(start, node.segmented.Interval),
			flags:    node.flags, parent: node, path: node.path + "/" + name,
		}, nil
	}
	return nil, fmt.Errorf("time segment not found: %s", name)
}

func (node *MemLastDayNode) GetLeafData() map[string]string {
	if status := windowStatus(node.segmented, node.window); status != "" {
		return map[string]string{"Status": status}
	}
	// The aggregate only: each segment states its own figures in the description
	// of the child that opens it.
	return memSegmentRows(node.segmented.Total(), node.segmented.Interval,
		len(node.segmented.Segments), windowCoverage(node.segmented.FirstTime, node.wholeSecs()))
}

// MemTimeSegmentNode is one segment of a memory window, or the whole window.
type MemTimeSegmentNode struct {
	segment  madmin.MemSegment
	interval int
	segments int
	coverage string
	flags    madmin.MetricFlags
	parent   MetricNode
	path     string
}

func (node *MemTimeSegmentNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *MemTimeSegmentNode) GetPath() string                    { return node.path }
func (node *MemTimeSegmentNode) GetParent() MetricNode              { return node.parent }
func (node *MemTimeSegmentNode) GetMetricType() madmin.MetricType   { return madmin.MetricsMem }
func (node *MemTimeSegmentNode) GetMetricFlags() madmin.MetricFlags { return node.flags }
func (node *MemTimeSegmentNode) ShouldPauseRefresh() bool           { return true }
func (node *MemTimeSegmentNode) GetChildren() []MetricChild         { return []MetricChild{} }

func (node *MemTimeSegmentNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children")
}

func (node *MemTimeSegmentNode) GetLeafData() map[string]string {
	return memSegmentRows(node.segment, node.interval, node.segments, node.coverage)
}

// MemVMStatNode shows the kernel memory-management counters.
type MemVMStatNode struct {
	vm     *madmin.MemVMStat
	parent MetricNode
	path   string
}

func (node *MemVMStatNode) GetOpts() madmin.MetricsOptions { return getNodeOpts(node) }

// NewMemVMStatNode creates a navigator for the vmstat counters.
func NewMemVMStatNode(vm *madmin.MemVMStat, parent MetricNode, path string) *MemVMStatNode {
	return &MemVMStatNode{vm: vm, parent: parent, path: path}
}

func (node *MemVMStatNode) GetChildren() []MetricChild { return []MetricChild{} }

func (node *MemVMStatNode) GetLeafData() map[string]string {
	if node.vm == nil {
		return map[string]string{"Status": "vmstat counters not available (Linux only)"}
	}
	vm := node.vm
	data := map[string]string{
		"Swap In":            formatMemoryBytes(vm.SwapInBytes),
		"Swap Out":           formatMemoryBytes(vm.SwapOutBytes),
		"Major Faults":       strconv.FormatUint(vm.MajorFaults, 10),
		"OOM Kills":          strconv.FormatUint(vm.OOMKill, 10),
		"Workingset Refault": strconv.FormatUint(vm.WorkingsetRefault, 10),
		"Direct Reclaim":     fmt.Sprintf("%d scanned, %d reclaimed", vm.PgScanDirect, vm.PgStealDirect),
	}
	if vm.CompactStall > 0 || vm.CompactFail > 0 || vm.THPCollapseAllocFailed > 0 {
		data["Compaction Stalls"] = fmt.Sprintf("%d (%d failed)", vm.CompactStall, vm.CompactFail)
		data["THP Collapse Failed"] = strconv.FormatUint(vm.THPCollapseAllocFailed, 10)
	}
	return data
}

func (node *MemVMStatNode) GetMetricType() madmin.MetricType   { return madmin.MetricsMem }
func (node *MemVMStatNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *MemVMStatNode) GetParent() MetricNode              { return node.parent }
func (node *MemVMStatNode) GetPath() string                    { return node.path }
func (node *MemVMStatNode) ShouldPauseRefresh() bool           { return false }

func (node *MemVMStatNode) GetChild(name string) (MetricNode, error) {
	return nil, fmt.Errorf("child not found: %s", name)
}

// MemCgroupNode shows cgroup-v2 memory accounting.
type MemCgroupNode struct {
	cg     *madmin.MemCgroupStats
	parent MetricNode
	path   string
}

func (node *MemCgroupNode) GetOpts() madmin.MetricsOptions { return getNodeOpts(node) }

// NewMemCgroupNode creates a navigator for cgroup-v2 memory accounting.
func NewMemCgroupNode(cg *madmin.MemCgroupStats, parent MetricNode, path string) *MemCgroupNode {
	return &MemCgroupNode{cg: cg, parent: parent, path: path}
}

func (node *MemCgroupNode) GetChildren() []MetricChild { return []MetricChild{} }

func (node *MemCgroupNode) GetLeafData() map[string]string {
	if node.cg == nil {
		return map[string]string{"Status": "not running under cgroup v2"}
	}
	cg := node.cg
	data := map[string]string{
		"Current": formatMemoryBytes(cg.Current),
		"Peak":    formatMemoryBytes(cg.Peak),
	}
	// Any unlimited contributor makes the aggregate unlimited, whatever the
	// limited nodes sum to.
	switch {
	case cg.UnlimitedMax > 0 && cg.Max > 0:
		data["Limit"] = fmt.Sprintf("unlimited on %d node(s); %s across the rest",
			cg.UnlimitedMax, formatMemoryBytes(cg.Max))
	case cg.UnlimitedMax > 0:
		data["Limit"] = "unlimited"
	case cg.Max > 0:
		data["Limit"] = fmt.Sprintf("%s (%s used)", formatMemoryBytes(cg.Max),
			calculatePercentage(cg.Current, cg.Max))
	default:
		data["Limit"] = "unlimited"
	}
	if cg.High > 0 {
		data["Throttle At"] = formatMemoryBytes(cg.High)
	}
	if cg.SwapCurrent > 0 {
		data["Swap"] = formatMemoryBytes(cg.SwapCurrent)
	}
	// The cgroup OOM killer, not the global one, is what kills the server under
	// Kubernetes.
	for _, k := range sortedKeys(cg.Events) {
		data["Event "+k] = strconv.FormatUint(cg.Events[k], 10)
	}
	return data
}

func (node *MemCgroupNode) GetMetricType() madmin.MetricType   { return madmin.MetricsMem }
func (node *MemCgroupNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *MemCgroupNode) GetParent() MetricNode              { return node.parent }
func (node *MemCgroupNode) GetPath() string                    { return node.path }
func (node *MemCgroupNode) ShouldPauseRefresh() bool           { return false }

func (node *MemCgroupNode) GetChild(name string) (MetricNode, error) {
	return nil, fmt.Errorf("child not found: %s", name)
}

// MemECCNode shows the ECC error rollup.
type MemECCNode struct {
	ecc    *madmin.MemECCStats
	parent MetricNode
	path   string
}

func (node *MemECCNode) GetOpts() madmin.MetricsOptions { return getNodeOpts(node) }

// NewMemECCNode creates a navigator for the ECC rollup.
func NewMemECCNode(ecc *madmin.MemECCStats, parent MetricNode, path string) *MemECCNode {
	return &MemECCNode{ecc: ecc, parent: parent, path: path}
}

func (node *MemECCNode) GetChildren() []MetricChild { return []MetricChild{} }

func (node *MemECCNode) GetLeafData() map[string]string {
	if node.ecc == nil {
		return map[string]string{"Status": "no ECC reporting hardware detected"}
	}
	ecc := node.ecc
	data := map[string]string{
		"Controllers": strconv.Itoa(ecc.Controllers),
		"DIMMs":       strconv.Itoa(ecc.DIMMs),
	}
	// The DIMM counts are the outlier axis a total loses.
	data["Correctable"] = fmt.Sprintf("%d across %d DIMM(s)", ecc.Corrected, ecc.DIMMsWithCorrected)
	data["Uncorrectable"] = fmt.Sprintf("%d across %d DIMM(s)", ecc.Uncorrected, ecc.DIMMsWithUncorrected)
	if ecc.HardwareCorruptedBytes > 0 {
		data["Retired Memory"] = formatMemoryBytes(ecc.HardwareCorruptedBytes)
	}
	return data
}

func (node *MemECCNode) GetMetricType() madmin.MetricType   { return madmin.MetricsMem }
func (node *MemECCNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *MemECCNode) GetParent() MetricNode              { return node.parent }
func (node *MemECCNode) GetPath() string                    { return node.path }
func (node *MemECCNode) ShouldPauseRefresh() bool           { return false }

func (node *MemECCNode) GetChild(name string) (MetricNode, error) {
	return nil, fmt.Errorf("child not found: %s", name)
}

// MemFragNode shows the buddy-allocator view of free memory.
type MemFragNode struct {
	frag   *madmin.MemFragStats
	parent MetricNode
	path   string
}

func (node *MemFragNode) GetOpts() madmin.MetricsOptions { return getNodeOpts(node) }

// NewMemFragNode creates a navigator for memory fragmentation.
func NewMemFragNode(frag *madmin.MemFragStats, parent MetricNode, path string) *MemFragNode {
	return &MemFragNode{frag: frag, parent: parent, path: path}
}

func (node *MemFragNode) GetChildren() []MetricChild { return []MetricChild{} }

func (node *MemFragNode) GetLeafData() map[string]string {
	if node.frag == nil {
		return map[string]string{"Status": "fragmentation metrics not available (Linux only)"}
	}
	f := node.frag
	data := map[string]string{
		"Zones": strconv.Itoa(f.Zones),
		"Free":  formatMemoryBytes(f.FreeBytes),
	}
	if f.PageSize > 0 {
		data["Page Size"] = formatMemoryBytes(f.PageSize)
	} else if f.Zones > 0 {
		data["Page Size"] = "mixed across hosts"
	}
	if f.LargeOrderBytes > 0 {
		data["Large Allocation"] = formatMemoryBytes(f.LargeOrderBytes)
		data["Free (Large Runs)"] = fmt.Sprintf("%s (%s of free)",
			formatMemoryBytes(f.FreeBytesLarge),
			calculatePercentage(f.FreeBytesLarge, f.FreeBytes))
		// The unusable-free-space index, derived rather than carried.
		var unusable uint64
		if f.FreeBytes > f.FreeBytesLarge {
			unusable = f.FreeBytes - f.FreeBytesLarge
		}
		data["Fragmentation"] = calculatePercentage(unusable, f.FreeBytes)
	}
	for _, zone := range sortedKeys(f.ByZone) {
		z := f.ByZone[zone]
		data["Zone "+zone] = fmt.Sprintf("%s free, %s in large runs",
			formatMemoryBytes(z.FreeBytes), formatMemoryBytes(z.FreeBytesLarge))
	}
	return data
}

func (node *MemFragNode) GetMetricType() madmin.MetricType   { return madmin.MetricsMem }
func (node *MemFragNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *MemFragNode) GetParent() MetricNode              { return node.parent }
func (node *MemFragNode) GetPath() string                    { return node.path }
func (node *MemFragNode) ShouldPauseRefresh() bool           { return false }

func (node *MemFragNode) GetChild(name string) (MetricNode, error) {
	return nil, fmt.Errorf("child not found: %s", name)
}
