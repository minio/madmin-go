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
	"sort"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/minio/madmin-go/v4"
)

// ProcessMetricsNode provides navigation for process metrics
type ProcessMetricsNode struct {
	process *madmin.ProcessMetrics
	parent  MetricNode `msg:"-"`
	path    string
}

func (node *ProcessMetricsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewProcessMetricsNode(process *madmin.ProcessMetrics, parent MetricNode, path string) *ProcessMetricsNode {
	return &ProcessMetricsNode{process: process, parent: parent, path: path}
}

func (node *ProcessMetricsNode) ShouldPauseRefresh() bool {
	return false
}

func (node *ProcessMetricsNode) GetChildren() []MetricChild {
	children := make([]MetricChild, 0, 7)
	children = append(children,
		MetricChild{Name: "cpu", Description: "Process CPU usage and timing statistics"},
		MetricChild{Name: "memory", Description: "Process memory usage information"},
		MetricChild{Name: "io", Description: "Process I/O statistics"},
		MetricChild{Name: "context_switches", Description: "Process context switch statistics"},
		MetricChild{Name: "page_faults", Description: "Process page fault statistics"},
		MetricChild{Name: "mem_maps", Description: "Process memory mapping details"},
		MetricChild{Name: "last_day", Description: "Last 24h process statistics"},
	)
	return children
}

// procUptime is the mean per-process uptime: TotalRunningSecs is now minus each
// process's start, summed, so Count is what turns it back into one process's.
//
// It is the window every cumulative counter in this family is measured over --
// I/O, faults and switches are all since process start -- and being a mean, it
// hides a node that restarted while the others stayed up.
func procUptime(p *madmin.ProcessMetrics) (time.Duration, bool) {
	if p == nil || p.Count <= 0 || p.TotalRunningSecs <= 0 {
		return 0, false
	}
	return time.Duration(p.TotalRunningSecs / float64(p.Count) * float64(time.Second)), true
}

// procAncestor walks up to the node holding the whole sample, so a subsection can
// reach the uptime and process count that its own struct does not carry.
func procAncestor(n MetricNode) *madmin.ProcessMetrics {
	for ; n != nil; n = n.GetParent() {
		if p, ok := n.(*ProcessMetricsNode); ok {
			return p.process
		}
	}
	return nil
}

// procRows builds leaf data in display order, dividing every summed value back
// down to one process.
//
// Everything on this wire is a sum over the processes that reported: a 17-node
// cluster's file-descriptor count is the fleet total, and a bare total answers no
// question an operator has. count is what makes it per-process, and each
// subsection carries its own because a node can report memory but not I/O.
type procRows struct {
	data  map[string]string
	n     int
	count int
	up    time.Duration
}

func newProcRows(count int, up time.Duration) *procRows {
	return &procRows{data: make(map[string]string), count: max(count, 1), up: up}
}

func (r *procRows) add(label, value string) {
	r.data[fmt.Sprintf("%02d:%s", r.n, label)] = value
	r.n++
}

// total states a summed value as the cluster figure with the per-process mean
// behind it, and skips a value nobody reported.
func (r *procRows) total(label string, v float64, render func(float64) string) {
	if v == 0 {
		return
	}
	r.add(label, render(v)+qualify(r.count, "process", v, 0, render))
}

// share is total with the value's percentage of a whole, for the breakdowns whose
// point is the split.
func (r *procRows) share(label string, v, whole float64, render func(float64) string) {
	if v == 0 {
		return
	}
	r.add(label, render(v)+qualify(r.count, "process", v, whole, render))
}

// counter is total with the per-process rate over the uptime, which is the window
// a value cumulative since process start was accrued over.
func (r *procRows) counter(label string, v float64, render func(float64) string, unit string) {
	if v == 0 {
		return
	}
	value := render(v) + qualify(r.count, "process", v, 0, render)
	if r.up > 0 {
		value += ", " + fmtRate(v/float64(r.count)/r.up.Seconds(), render, unit)
	}
	r.add(label, value)
}

func (node *ProcessMetricsNode) GetLeafData() map[string]string {
	if node.process == nil {
		return map[string]string{"Status": "No process metrics available"}
	}
	p := node.process
	up, hasUp := procUptime(p)
	r := newProcRows(p.Count, up)

	r.add("Collected At", p.CollectedAt.Format("2006-01-02 15:04:05"))
	if p.Nodes > 0 {
		r.add("Nodes", formatNodeCount(p.Nodes, 1))
	}
	if p.Count > 0 && p.Count != p.Nodes {
		r.add("Processes", humanize.Comma(int64(p.Count)))
	}
	if hasUp {
		r.add("Uptime", coverDuration(up)+" (mean per process)")
	}
	if p.RunningProcesses > 0 || p.BackgroundProcesses > 0 {
		r.add("Running / Background", fmt.Sprintf("%d / %d", p.RunningProcesses, p.BackgroundProcesses))
	}

	r.total("CPU", p.TotalCPUPercent, func(v float64) string { return fmt.Sprintf("%.1f%%", v) })
	r.total("Threads", float64(p.TotalNumThreads), fmtCount)
	r.total("File Descriptors", float64(p.TotalNumFDs), fmtCount)
	r.total("Connections", float64(p.TotalNumConnections), fmtCount)
	r.total("Resident", float64(p.MemInfo.RSS), fmtBytes)
	r.total("Virtual", float64(p.MemInfo.VMS), fmtBytes)
	r.counter("Read", float64(p.IOCounters.ReadBytes), fmtBytes, "")
	r.counter("Written", float64(p.IOCounters.WriteBytes), fmtBytes, "")

	// Kernel thread states, busiest first.
	if len(p.ThreadStates) > 0 {
		r.add("Thread States", formatCountMap(p.ThreadStates, 8))
	}

	// PSI: mean stall across the reporting nodes, with the worst node, so a
	// single stalled host is distinguishable from a stalled cluster.
	// Known lines first in their fixed order, then anything the kernel has added
	// since -- Pressure is an open map precisely so a new resource needs no wire
	// change, and dropping unknown keys here would defeat that.
	known := make(map[string]bool, len(psiLineOrder))
	for _, line := range psiLineOrder {
		known[line] = true
	}
	extra := make([]string, 0, len(p.Pressure))
	for line := range p.Pressure {
		if !known[line] {
			extra = append(extra, line)
		}
	}
	sort.Strings(extra)

	for _, line := range append(append([]string{}, psiLineOrder...), extra...) {
		stall, ok := p.Pressure[line]
		if !ok || stall.N == 0 {
			continue
		}
		label, ok := psiLineLabels[line]
		if !ok {
			label = line
		}
		r.add("Pressure "+label, fmt.Sprintf("%.2f%% avg10 (max %.2f%%), %s stalled",
			stall.Avg10Sum/float64(stall.N), stall.Avg10Max,
			fmtSecs(float64(stall.StallUS)/1e6)))
	}

	if d := p.DState; d != nil {
		if len(d.DwellBuckets) > 0 {
			r.add("Uninterruptible Dwell", formatDwellBuckets(d.DwellBuckets, d.WindowSecs))
		}
		if len(d.ByWchan) > 0 {
			r.add("Uninterruptible By Wchan", formatCountMap(d.ByWchan, 5))
		}
	}

	return r.data
}

// psiLineOrder fixes the display order of PSI lines; psiLineLabels gives each
// the operator-facing name.
var (
	psiLineOrder  = []string{"cpu_some", "cpu_full", "io_some", "io_full", "mem_some", "mem_full"}
	psiLineLabels = map[string]string{
		"cpu_some": "CPU (some)",
		"cpu_full": "CPU (all)",
		"io_some":  "I/O (some)",
		"io_full":  "I/O (all)",
		"mem_some": "Memory (some)",
		"mem_full": "Memory (all)",
	}
)

// formatDwellBuckets renders the cumulative at-least-N-seconds ladder shortest
// rung first, so "3 threads blocked, 1 of them for 30s or more" reads directly.
func formatDwellBuckets(buckets map[int]int, windowSecs int) string {
	rungs := make([]int, 0, len(buckets))
	for k := range buckets {
		rungs = append(rungs, k)
	}
	sort.Ints(rungs)

	parts := make([]string, 0, len(rungs)+1)
	for _, r := range rungs {
		if buckets[r] == 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%d at %ds+", buckets[r], r))
	}
	if len(parts) == 0 {
		return "none"
	}
	out := strings.Join(parts, ", ")
	if windowSecs > 0 {
		out += fmt.Sprintf(" (%ds window)", windowSecs)
	}
	return out
}

// formatCountMap renders a bounded value-to-count map largest first, keeping at
// most topN entries and summarising the rest.
func formatCountMap[K comparable, V int | int64 | uint64](m map[K]V, topN int) string {
	type kv struct {
		k K
		v V
	}
	entries := make([]kv, 0, len(m))
	for k, v := range m {
		entries = append(entries, kv{k, v})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].v != entries[j].v {
			return entries[i].v > entries[j].v
		}
		return fmt.Sprint(entries[i].k) < fmt.Sprint(entries[j].k)
	})

	parts := make([]string, 0, topN+1)
	for i, e := range entries {
		if i >= topN {
			parts = append(parts, fmt.Sprintf("+%d more", len(entries)-topN))
			break
		}
		parts = append(parts, fmt.Sprintf("%v=%v", e.k, e.v))
	}
	return strings.Join(parts, " ")
}

func (node *ProcessMetricsNode) GetChild(name string) (MetricNode, error) {
	if node.process == nil {
		return nil, fmt.Errorf("no process data available")
	}

	switch name {
	case "cpu":
		return NewProcessCPUTimesNode(&node.process.CPUTimes, node, node.path+"/"+name), nil
	case "memory":
		return NewProcessMemoryInfoNode(&node.process.MemInfo, node, node.path+"/"+name), nil
	case "io":
		return NewProcessIOCountersNode(&node.process.IOCounters, node, node.path+"/"+name), nil
	case "context_switches":
		return NewProcessCtxSwitchesNode(&node.process.NumCtxSwitches, node, node.path+"/"+name), nil
	case "page_faults":
		return NewProcessPageFaultsNode(&node.process.PageFaults, node, node.path+"/"+name), nil
	case "mem_maps":
		return NewProcessMemoryMapsNode(&node.process.MemMaps, node, node.path+"/"+name), nil
	case "last_day":
		return NewProcessLastDayNode(node.process.LastDay, node, node.path+"/last_day"), nil
	default:
		return nil, fmt.Errorf("unknown process metric section: %s", name)
	}
}

func (node *ProcessMetricsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsProcess }
func (node *ProcessMetricsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *ProcessMetricsNode) GetParent() MetricNode              { return node.parent }
func (node *ProcessMetricsNode) GetPath() string                    { return node.path }

// ProcessCPUTimesNode displays CPU timing statistics
type ProcessCPUTimesNode struct {
	cpuTimes *madmin.ProcessCPUTimes
	parent   MetricNode `msg:"-"`
	path     string
}

func (node *ProcessCPUTimesNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewProcessCPUTimesNode(cpuTimes *madmin.ProcessCPUTimes, parent MetricNode, path string) *ProcessCPUTimesNode {
	return &ProcessCPUTimesNode{cpuTimes: cpuTimes, parent: parent, path: path}
}

func (node *ProcessCPUTimesNode) ShouldPauseRefresh() bool {
	return false
}

func (node *ProcessCPUTimesNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *ProcessCPUTimesNode) GetLeafData() map[string]string {
	if node.cpuTimes == nil {
		return map[string]string{"Status": "No CPU timing data available"}
	}
	c := node.cpuTimes
	up, _ := procUptime(procAncestor(node))
	r := newProcRows(c.Count, up)

	total := c.User + c.System + c.Idle + c.Nice + c.Iowait + c.Irq +
		c.Softirq + c.Steal + c.Guest + c.GuestNice
	if total == 0 {
		return map[string]string{"Status": "No CPU time recorded"}
	}
	r.add("Processes", humanize.Comma(int64(max(c.Count, 1))))
	// Cumulative since process start, so the share of the uptime is what says
	// how busy a process was; the seconds alone only say how long it has run.
	if up > 0 {
		r.add("Busy", fmtPct(total-c.Idle, up.Seconds()*float64(max(c.Count, 1)))+" of one core, per process")
	}
	r.share("Total", total, 0, fmtSecs)
	for _, row := range []struct {
		label string
		v     float64
	}{
		{"User", c.User},
		{"System", c.System},
		{"Idle", c.Idle},
		{"Nice", c.Nice},
		{"IO Wait", c.Iowait},
		{"IRQ", c.Irq},
		{"Soft IRQ", c.Softirq},
		{"Steal", c.Steal},
		{"Guest", c.Guest},
		{"Guest Nice", c.GuestNice},
	} {
		r.share(row.label, row.v, total, fmtSecs)
	}
	return r.data
}

func (node *ProcessCPUTimesNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("CPU times node has no children")
}

func (node *ProcessCPUTimesNode) GetMetricType() madmin.MetricType   { return madmin.MetricsProcess }
func (node *ProcessCPUTimesNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *ProcessCPUTimesNode) GetParent() MetricNode              { return node.parent }
func (node *ProcessCPUTimesNode) GetPath() string                    { return node.path }

// ProcessMemoryInfoNode displays memory usage information
type ProcessMemoryInfoNode struct {
	memInfo *madmin.ProcessMemoryInfo
	parent  MetricNode `msg:"-"`
	path    string
}

func (node *ProcessMemoryInfoNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewProcessMemoryInfoNode(memInfo *madmin.ProcessMemoryInfo, parent MetricNode, path string) *ProcessMemoryInfoNode {
	return &ProcessMemoryInfoNode{memInfo: memInfo, parent: parent, path: path}
}

func (node *ProcessMemoryInfoNode) ShouldPauseRefresh() bool {
	return false
}

func (node *ProcessMemoryInfoNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *ProcessMemoryInfoNode) GetLeafData() map[string]string {
	if node.memInfo == nil {
		return map[string]string{"Status": "No memory information available"}
	}
	m := node.memInfo
	r := newProcRows(m.Count, 0)
	r.add("Processes", humanize.Comma(int64(max(m.Count, 1))))
	// Resident first: it is the number that decides whether a host is about to
	// run out. Virtual and the segment breakdown explain its shape.
	for _, row := range []struct {
		label string
		v     uint64
	}{
		{"Resident", m.RSS},
		{"Peak Resident", m.HWM},
		{"Virtual", m.VMS},
		{"Data", m.Data},
		{"Stack", m.Stack},
		{"Shared", m.Shared},
		{"Locked", m.Locked},
		{"Swap", m.Swap},
	} {
		r.total(row.label, float64(row.v), fmtBytes)
	}
	if r.n == 1 {
		return map[string]string{"Status": "No memory usage recorded"}
	}
	return r.data
}

func (node *ProcessMemoryInfoNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("memory info node has no children")
}

func (node *ProcessMemoryInfoNode) GetMetricType() madmin.MetricType   { return madmin.MetricsProcess }
func (node *ProcessMemoryInfoNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *ProcessMemoryInfoNode) GetParent() MetricNode              { return node.parent }
func (node *ProcessMemoryInfoNode) GetPath() string                    { return node.path }

// ProcessIOCountersNode displays I/O statistics
type ProcessIOCountersNode struct {
	ioCounters *madmin.ProcessIOCounters
	parent     MetricNode `msg:"-"`
	path       string
}

func (node *ProcessIOCountersNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewProcessIOCountersNode(ioCounters *madmin.ProcessIOCounters, parent MetricNode, path string) *ProcessIOCountersNode {
	return &ProcessIOCountersNode{ioCounters: ioCounters, parent: parent, path: path}
}

func (node *ProcessIOCountersNode) ShouldPauseRefresh() bool {
	return false
}

func (node *ProcessIOCountersNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *ProcessIOCountersNode) GetLeafData() map[string]string {
	if node.ioCounters == nil {
		return map[string]string{"Status": "No I/O statistics available"}
	}
	io := node.ioCounters
	up, _ := procUptime(procAncestor(node))
	r := newProcRows(io.Count, up)
	r.add("Processes", humanize.Comma(int64(max(io.Count, 1))))
	r.counter("Read", float64(io.ReadBytes), fmtBytes, "")
	r.counter("Written", float64(io.WriteBytes), fmtBytes, "")
	r.counter("Reads", float64(io.ReadCount), fmtCount, "ops")
	r.counter("Writes", float64(io.WriteCount), fmtCount, "ops")
	// Mean request size says whether the load is streaming or metadata-shaped,
	// which the byte and operation counts only imply separately.
	if io.ReadCount > 0 && io.ReadBytes > 0 {
		r.add("Mean Read", fmtBytes(float64(io.ReadBytes)/float64(io.ReadCount)))
	}
	if io.WriteCount > 0 && io.WriteBytes > 0 {
		r.add("Mean Write", fmtBytes(float64(io.WriteBytes)/float64(io.WriteCount)))
	}
	if r.n == 1 {
		return map[string]string{"Status": "No I/O recorded"}
	}
	return r.data
}

func (node *ProcessIOCountersNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("I/O counters node has no children")
}

func (node *ProcessIOCountersNode) GetMetricType() madmin.MetricType   { return madmin.MetricsProcess }
func (node *ProcessIOCountersNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *ProcessIOCountersNode) GetParent() MetricNode              { return node.parent }
func (node *ProcessIOCountersNode) GetPath() string                    { return node.path }

// ProcessCtxSwitchesNode displays context switch statistics
type ProcessCtxSwitchesNode struct {
	ctxSwitches *madmin.ProcessCtxSwitches
	parent      MetricNode `msg:"-"`
	path        string
}

func (node *ProcessCtxSwitchesNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewProcessCtxSwitchesNode(ctxSwitches *madmin.ProcessCtxSwitches, parent MetricNode, path string) *ProcessCtxSwitchesNode {
	return &ProcessCtxSwitchesNode{ctxSwitches: ctxSwitches, parent: parent, path: path}
}

func (node *ProcessCtxSwitchesNode) ShouldPauseRefresh() bool {
	return false
}

func (node *ProcessCtxSwitchesNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *ProcessCtxSwitchesNode) GetLeafData() map[string]string {
	if node.ctxSwitches == nil {
		return map[string]string{"Status": "No context switch data available"}
	}
	c := node.ctxSwitches
	total := c.Voluntary + c.Involuntary
	if total == 0 {
		return map[string]string{"Status": "No context switches recorded"}
	}
	up, _ := procUptime(procAncestor(node))
	r := newProcRows(c.Count, up)
	r.add("Processes", humanize.Comma(int64(max(c.Count, 1))))
	r.counter("Total", float64(total), fmtCount, "")
	// Voluntary means the thread blocked and gave the CPU up; involuntary means
	// the scheduler took it away, which is the one that says the box is
	// oversubscribed.
	r.share("Voluntary", float64(c.Voluntary), float64(total), fmtCount)
	r.share("Involuntary", float64(c.Involuntary), float64(total), fmtCount)
	return r.data
}

func (node *ProcessCtxSwitchesNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("context switches node has no children")
}

func (node *ProcessCtxSwitchesNode) GetMetricType() madmin.MetricType   { return madmin.MetricsProcess }
func (node *ProcessCtxSwitchesNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *ProcessCtxSwitchesNode) GetParent() MetricNode              { return node.parent }
func (node *ProcessCtxSwitchesNode) GetPath() string                    { return node.path }

// ProcessPageFaultsNode displays page fault statistics
type ProcessPageFaultsNode struct {
	pageFaults *madmin.ProcessPageFaults
	parent     MetricNode `msg:"-"`
	path       string
}

func (node *ProcessPageFaultsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewProcessPageFaultsNode(pageFaults *madmin.ProcessPageFaults, parent MetricNode, path string) *ProcessPageFaultsNode {
	return &ProcessPageFaultsNode{pageFaults: pageFaults, parent: parent, path: path}
}

func (node *ProcessPageFaultsNode) ShouldPauseRefresh() bool {
	return false
}

func (node *ProcessPageFaultsNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *ProcessPageFaultsNode) GetLeafData() map[string]string {
	if node.pageFaults == nil {
		return map[string]string{"Status": "No page fault data available"}
	}
	f := node.pageFaults
	total := f.MinorFaults + f.MajorFaults
	if total == 0 {
		return map[string]string{"Status": "No page faults recorded"}
	}
	up, _ := procUptime(procAncestor(node))
	r := newProcRows(f.Count, up)
	r.add("Processes", humanize.Comma(int64(max(f.Count, 1))))
	r.counter("Total", float64(total), fmtCount, "")
	// A minor fault found the page already in memory; a major one went to disk,
	// and its rate is what says the host is thrashing.
	r.share("Minor", float64(f.MinorFaults), float64(total), fmtCount)
	r.counter("Major", float64(f.MajorFaults), fmtCount, "")
	if child := f.ChildMinorFaults + f.ChildMajorFaults; child > 0 {
		r.total("Child Processes", float64(child), fmtCount)
	}
	return r.data
}

func (node *ProcessPageFaultsNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("page faults node has no children")
}

func (node *ProcessPageFaultsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsProcess }
func (node *ProcessPageFaultsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *ProcessPageFaultsNode) GetParent() MetricNode              { return node.parent }
func (node *ProcessPageFaultsNode) GetPath() string                    { return node.path }

// ProcessMemoryMapsNode displays memory mapping details
type ProcessMemoryMapsNode struct {
	memMaps *madmin.ProcessMemoryMaps
	parent  MetricNode `msg:"-"`
	path    string
}

func (node *ProcessMemoryMapsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewProcessMemoryMapsNode(memMaps *madmin.ProcessMemoryMaps, parent MetricNode, path string) *ProcessMemoryMapsNode {
	return &ProcessMemoryMapsNode{memMaps: memMaps, parent: parent, path: path}
}

func (node *ProcessMemoryMapsNode) ShouldPauseRefresh() bool {
	return false
}

func (node *ProcessMemoryMapsNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *ProcessMemoryMapsNode) GetLeafData() map[string]string {
	if node.memMaps == nil || node.memMaps.Count == 0 {
		return map[string]string{
			"Status": "No memory mapping data available",
			"Note":   "Memory mapping details are platform-specific",
		}
	}
	m := node.memMaps
	r := newProcRows(m.Count, 0)
	r.add("Processes", humanize.Comma(int64(max(m.Count, 1))))
	r.total("Mapped", float64(m.TotalSize), fmtBytes)
	// Against the mapped total, so a row says what share of the address space is
	// actually backed rather than only how large it is.
	for _, row := range []struct {
		label string
		v     uint64
	}{
		{"Resident", m.TotalRSS},
		{"Proportional", m.TotalPSS},
		{"Shared Clean", m.TotalSharedClean},
		{"Shared Dirty", m.TotalSharedDirty},
		{"Private Clean", m.TotalPrivateClean},
		{"Private Dirty", m.TotalPrivateDirty},
		{"Referenced", m.TotalReferenced},
		{"Anonymous", m.TotalAnonymous},
		{"Swapped", m.TotalSwap},
	} {
		r.share(row.label, float64(row.v), float64(m.TotalSize), fmtBytes)
	}
	return r.data
}

func (node *ProcessMemoryMapsNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("memory maps node has no children")
}

func (node *ProcessMemoryMapsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsProcess }
func (node *ProcessMemoryMapsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *ProcessMemoryMapsNode) GetParent() MetricNode              { return node.parent }
func (node *ProcessMemoryMapsNode) GetPath() string                    { return node.path }

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1f seconds", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1f minutes", d.Minutes())
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%.1f hours", d.Hours())
	}
	days := int(d.Hours() / 24)
	hours := d.Hours() - float64(days*24)
	return fmt.Sprintf("%d days, %.1f hours", days, hours)
}

// ProcessLastDayNode shows the last-day process window: a navigable child per
// time segment, and an _ALL entry over the whole span.
type ProcessLastDayNode struct {
	segmented *madmin.SegmentedProcessMetrics
	parent    MetricNode
	path      string
}

func NewProcessLastDayNode(segmented *madmin.SegmentedProcessMetrics, parent MetricNode, path string) *ProcessLastDayNode {
	return &ProcessLastDayNode{segmented: segmented, parent: parent, path: path}
}

func (node *ProcessLastDayNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *ProcessLastDayNode) GetPath() string                    { return node.path }
func (node *ProcessLastDayNode) GetParent() MetricNode              { return node.parent }
func (node *ProcessLastDayNode) GetMetricType() madmin.MetricType   { return madmin.MetricsProcess }
func (node *ProcessLastDayNode) GetMetricFlags() madmin.MetricFlags { return madmin.MetricsDayStats }
func (node *ProcessLastDayNode) ShouldPauseRefresh() bool           { return true }

// hasSegments reports whether the window can be placed on a timeline at all. An
// interval of zero would stamp every segment with the same time.
func (node *ProcessLastDayNode) hasSegments() bool {
	return node.segmented != nil && node.segmented.Interval > 0 && len(node.segmented.Segments) > 0
}

// wholeSecs is the span the whole window covers.
func (node *ProcessLastDayNode) wholeSecs() int {
	return node.segmented.Interval * len(node.segmented.Segments)
}

func (node *ProcessLastDayNode) GetLeafData() map[string]string {
	if status := windowStatus(node.segmented, "day"); status != "" {
		return map[string]string{"Status": status}
	}
	// The aggregate only: each segment states its own figures in the
	// description of the child that opens it.
	return processSegmentRows(node.segmented.Total(), node.segmented.Interval,
		len(node.segmented.Segments), windowCoverage(node.segmented.FirstTime, node.wholeSecs()))
}

func (node *ProcessLastDayNode) GetChildren() []MetricChild {
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
	// Newest first, as every other family lists a window: the reader is here to
	// see what just happened, not to read a day from the beginning.
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
			Description: segmentDescTime(start, withDate) + ", " + describeProcessSegment(seg, node.segmented.Interval),
		})
	}
	return children
}

func (node *ProcessLastDayNode) GetChild(name string) (MetricNode, error) {
	if !node.hasSegments() {
		return nil, fmt.Errorf("no last-day process segments available")
	}
	if name == "_ALL" {
		return &ProcessSegmentTotalNode{
			segmented: node.segmented,
			parent:    node,
			path:      node.path + "/" + name,
		}, nil
	}
	owners := segmentSecOwners(node.segmented.FirstTime, node.segmented.Interval, len(node.segmented.Segments))
	if i, ok := owners[name]; ok {
		return &ProcessTimeSegmentNode{
			segment:     node.segmented.Segments[i],
			segmentTime: segmentStart(node.segmented.FirstTime, node.segmented.Interval, i),
			interval:    node.segmented.Interval,
			parent:      node,
			path:        node.path + "/" + name,
		}, nil
	}
	return nil, fmt.Errorf("time segment not found: %s", name)
}

// describeProcessSegment renders one segment on a single line, per process.
func describeProcessSegment(seg madmin.ProcessSegment, interval int) string {
	n := float64(seg.N)
	parts := []string{fmt.Sprintf("CPU %.1f%%", seg.CPUPercent/n)}
	if seg.RSS > 0 {
		parts = append(parts, "RSS "+fmtBytes(float64(seg.RSS)/n))
	}
	if seg.NumThreads > 0 {
		parts = append(parts, "threads "+fmtCount(float64(seg.NumThreads)/n))
	}
	// I/O is a within-segment sum, not a level, so it earns a rate.
	if io := seg.ReadBytes + seg.WriteBytes; io > 0 && interval > 0 {
		parts = append(parts, "I/O "+fmtRate(float64(io)/n/float64(interval), fmtBytes, ""))
	}
	return strings.Join(parts, ", ") + " per process"
}

// processSegmentRows renders one segment, or a whole window, as leaf data.
//
// Every field is summed over the samples folded in -- N of them, one per process
// per sample -- so nothing here is read without dividing by N first.
//
// That quotient is a mean over samples, and one sample covers one interval, so
// interval is the divisor for every rate and utilisation here even when the whole
// window is being rendered. Handing it the window span instead understated a
// day's CPU and I/O by the number of segments in it: a 14-segment window read
// 136% of a core where the segments it was built from each read ~1900%.
//
// segments is how many were folded in, so N can be turned back into a count of
// the processes reporting rather than a count of samples.
func processSegmentRows(seg madmin.ProcessSegment, interval, segments int, coverage string) map[string]string {
	if seg.N == 0 {
		return map[string]string{"Status": "no process reported this time segment"}
	}
	n := float64(seg.N)
	r := newProcRows(1, 0)
	r.add("Coverage", coverage)
	r.add("Processes", formatNodeCount(seg.N, segments)+" reporting")
	r.add("CPU", fmt.Sprintf("%.2f%% per process", seg.CPUPercent/n))

	// CPU seconds, per process per interval, as a share of that interval: one
	// full core is 100%. The old form divided the cluster-wide sum by the
	// window, scaling every figure by the number of processes reporting.
	cpuRow := func(label string, v float64) {
		if v <= 0 {
			return
		}
		perProc := v / n
		if interval > 0 {
			r.add(label, fmt.Sprintf("%s (%s of one core)", fmtSecs(perProc), fmtPct(perProc, float64(interval))))
			return
		}
		r.add(label, fmtSecs(perProc))
	}
	for _, row := range []struct {
		label string
		v     float64
	}{
		{"CPU User", seg.CPUUser},
		{"CPU System", seg.CPUSystem},
		{"CPU IO Wait", seg.CPUIowait},
		{"CPU Nice", seg.CPUNice},
		{"CPU IRQ", seg.CPUIrq},
		{"CPU Soft IRQ", seg.CPUSoftirq},
		{"CPU Steal", seg.CPUSteal},
		{"CPU Guest", seg.CPUGuest},
		{"CPU Guest Nice", seg.CPUGuestNice},
		{"CPU Idle", seg.CPUIdle},
	} {
		cpuRow(row.label, row.v)
	}

	r.add("Resident", fmtBytes(float64(seg.RSS)/n)+" per process")
	if seg.VMS > 0 {
		r.add("Virtual", fmtBytes(float64(seg.VMS)/n)+" per process")
	}
	r.add("Threads", fmtCount(float64(seg.NumThreads)/n)+" per process")
	r.add("File Descriptors", fmtCount(float64(seg.NumFDs)/n)+" per process")
	r.add("Connections", fmtCount(float64(seg.NumConnections)/n)+" per process")
	if seg.ThreadsD > 0 {
		r.add("Uninterruptible", fmtCount(float64(seg.ThreadsD)/n)+" threads per process")
	}

	// I/O and the fault and switch counters accrue inside the window, so they
	// carry a rate rather than a level.
	ioRow := func(label string, v float64, render func(float64) string, unit string) {
		if v <= 0 {
			return
		}
		perProc := v / n
		value := render(perProc) + " per process"
		if interval > 0 {
			value += ", " + fmtRate(perProc/float64(interval), render, unit)
		}
		r.add(label, value)
	}
	ioRow("Read", float64(seg.ReadBytes), fmtBytes, "")
	ioRow("Written", float64(seg.WriteBytes), fmtBytes, "")
	ioRow("Reads", float64(seg.ReadCount), fmtCount, "ops")
	ioRow("Writes", float64(seg.WriteCount), fmtCount, "ops")
	ioRow("Ctx Switches (vol)", float64(seg.CtxSwitchesVoluntary), fmtCount, "")
	ioRow("Ctx Switches (invol)", float64(seg.CtxSwitchesInvoluntary), fmtCount, "")
	ioRow("Minor Faults", float64(seg.MinorFaults), fmtCount, "")
	ioRow("Major Faults", float64(seg.MajorFaults), fmtCount, "")

	// PSI is Linux-only and can be missing on a host whose process sample is
	// present, so it has its own count; dividing by N would under-report by the
	// share of hosts without it.
	if seg.PSIN > 0 {
		psi := float64(seg.PSIN)
		for _, row := range []struct {
			label string
			v     float64
		}{
			{"Pressure CPU (some)", seg.PSICPUSome10},
			{"Pressure I/O (some)", seg.PSIIOSome10},
			{"Pressure I/O (all)", seg.PSIIOFull10},
			{"Pressure Memory (some)", seg.PSIMemSome10},
			{"Pressure Memory (all)", seg.PSIMemFull10},
		} {
			if row.v > 0 {
				r.add(row.label, fmt.Sprintf("%.2f%% avg10", row.v/psi))
			}
		}
	}
	return r.data
}

// ProcessSegmentTotalNode is the _ALL entry: the whole window rather than one
// slot of it.
type ProcessSegmentTotalNode struct {
	segmented *madmin.SegmentedProcessMetrics
	parent    MetricNode
	path      string
}

func (node *ProcessSegmentTotalNode) GetOpts() madmin.MetricsOptions   { return getNodeOpts(node) }
func (node *ProcessSegmentTotalNode) GetPath() string                  { return node.path }
func (node *ProcessSegmentTotalNode) GetParent() MetricNode            { return node.parent }
func (node *ProcessSegmentTotalNode) GetMetricType() madmin.MetricType { return madmin.MetricsProcess }

// GetMetricFlags keeps the day window on the refresh request: without it a tick
// would drop the very segments this node renders.
func (node *ProcessSegmentTotalNode) GetMetricFlags() madmin.MetricFlags {
	return madmin.MetricsDayStats
}
func (node *ProcessSegmentTotalNode) ShouldPauseRefresh() bool   { return true }
func (node *ProcessSegmentTotalNode) GetChildren() []MetricChild { return []MetricChild{} }

func (node *ProcessSegmentTotalNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children")
}

func (node *ProcessSegmentTotalNode) GetLeafData() map[string]string {
	if node.segmented == nil || len(node.segmented.Segments) == 0 {
		return map[string]string{"Status": "no last-day process segments available"}
	}
	secs := node.segmented.Interval * len(node.segmented.Segments)
	return processSegmentRows(node.segmented.Total(), node.segmented.Interval,
		len(node.segmented.Segments), windowCoverage(node.segmented.FirstTime, secs))
}

// ProcessTimeSegmentNode is one time segment of the window.
type ProcessTimeSegmentNode struct {
	segment     madmin.ProcessSegment
	segmentTime time.Time
	interval    int
	parent      MetricNode
	path        string
}

func (node *ProcessTimeSegmentNode) GetOpts() madmin.MetricsOptions   { return getNodeOpts(node) }
func (node *ProcessTimeSegmentNode) GetPath() string                  { return node.path }
func (node *ProcessTimeSegmentNode) GetParent() MetricNode            { return node.parent }
func (node *ProcessTimeSegmentNode) GetMetricType() madmin.MetricType { return madmin.MetricsProcess }

// GetMetricFlags keeps the day window on the refresh request, as for the _ALL
// node above.
func (node *ProcessTimeSegmentNode) GetMetricFlags() madmin.MetricFlags {
	return madmin.MetricsDayStats
}
func (node *ProcessTimeSegmentNode) ShouldPauseRefresh() bool   { return true }
func (node *ProcessTimeSegmentNode) GetChildren() []MetricChild { return []MetricChild{} }

func (node *ProcessTimeSegmentNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children")
}

func (node *ProcessTimeSegmentNode) GetLeafData() map[string]string {
	return processSegmentRows(node.segment, node.interval, 1,
		windowCoverage(node.segmentTime, node.interval))
}
