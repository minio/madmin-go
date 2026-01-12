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
	children := []MetricChild{
		{Name: "cpu", Description: "Process CPU usage and timing statistics"},
		{Name: "memory", Description: "Process memory usage information"},
		{Name: "io", Description: "Process I/O statistics"},
		{Name: "context_switches", Description: "Process context switch statistics"},
		{Name: "page_faults", Description: "Process page fault statistics"},
		{Name: "mem_maps", Description: "Process memory mapping details"},
	}
	children = append(children, MetricChild{Name: "last_day", Description: "Last 24h process statistics"})
	return children
}

func (node *ProcessMetricsNode) GetLeafData() map[string]string {
	if node.process == nil {
		return map[string]string{"Status": "No process metrics available"}
	}

	data := make(map[string]string)

	// Overview
	data["00:Process Overview"] = fmt.Sprintf("Collected at %s",
		node.process.CollectedAt.Format("2006-01-02 15:04:05"))

	// Cluster information
	if node.process.Nodes > 0 {
		data["Cluster Status"] = fmt.Sprintf("%s nodes reporting", humanize.Comma(int64(node.process.Nodes)))
		if node.process.Count > 0 {
			data["Total Processes"] = fmt.Sprintf("%s MinIO processes", humanize.Comma(int64(node.process.Count)))
		}
	}

	// Process status
	if node.process.RunningProcesses > 0 || node.process.BackgroundProcesses > 0 {
		data["Running Processes"] = humanize.Comma(int64(node.process.RunningProcesses))
		data["Background Processes"] = humanize.Comma(int64(node.process.BackgroundProcesses))
	}

	// Key performance metrics
	if node.process.TotalCPUPercent > 0 {
		data["Total CPU Usage"] = fmt.Sprintf("%.2f%% across cluster", node.process.TotalCPUPercent)
	}

	if node.process.TotalRunningSecs > 0 {
		uptime := time.Duration(node.process.TotalRunningSecs) * time.Second
		data["Cumulative Uptime"] = formatDuration(uptime)
	}

	// Resource utilization
	if node.process.TotalNumConnections > 0 {
		data["Network Connections"] = humanize.Comma(int64(node.process.TotalNumConnections))
	}

	if node.process.TotalNumFDs > 0 {
		data["File Descriptors"] = humanize.Comma(node.process.TotalNumFDs)
	}

	if node.process.TotalNumThreads > 0 {
		data["Total Threads"] = humanize.Comma(node.process.TotalNumThreads)
	}

	// Memory summary
	if node.process.MemInfo.RSS > 0 {
		data["Resident Memory"] = humanize.Bytes(node.process.MemInfo.RSS)
		if node.process.MemInfo.VMS > 0 {
			data["Virtual Memory"] = humanize.Bytes(node.process.MemInfo.VMS)
		}
	}

	// I/O summary
	if node.process.IOCounters.ReadBytes > 0 || node.process.IOCounters.WriteBytes > 0 {
		data["Total Read I/O"] = humanize.Bytes(node.process.IOCounters.ReadBytes)
		data["Total Write I/O"] = humanize.Bytes(node.process.IOCounters.WriteBytes)
	}

	return data
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

	data := make(map[string]string)

	if node.cpuTimes.Count > 0 {
		data["Data Sources"] = fmt.Sprintf("%d processes reporting", node.cpuTimes.Count)
	}

	// Calculate total time for percentages
	totalTime := node.cpuTimes.User + node.cpuTimes.System + node.cpuTimes.Idle +
		node.cpuTimes.Nice + node.cpuTimes.Iowait + node.cpuTimes.Irq +
		node.cpuTimes.Softirq + node.cpuTimes.Steal + node.cpuTimes.Guest +
		node.cpuTimes.GuestNice

	if totalTime > 0 {
		data["00:CPU"] = "Cumulative CPU time across all processes"

		data["User Time"] = fmt.Sprintf("%.2f seconds (%.1f%%)",
			node.cpuTimes.User, (node.cpuTimes.User/totalTime)*100)
		data["System Time"] = fmt.Sprintf("%.2f seconds (%.1f%%)",
			node.cpuTimes.System, (node.cpuTimes.System/totalTime)*100)
		data["Idle Time"] = fmt.Sprintf("%.2f seconds (%.1f%%)",
			node.cpuTimes.Idle, (node.cpuTimes.Idle/totalTime)*100)

		// Only show non-zero times
		if node.cpuTimes.Nice > 0 {
			data["Nice Time"] = fmt.Sprintf("%.2f seconds (%.1f%%)",
				node.cpuTimes.Nice, (node.cpuTimes.Nice/totalTime)*100)
		}
		if node.cpuTimes.Iowait > 0 {
			data["IO Wait Time"] = fmt.Sprintf("%.2f seconds (%.1f%%)",
				node.cpuTimes.Iowait, (node.cpuTimes.Iowait/totalTime)*100)
		}
		if node.cpuTimes.Irq > 0 {
			data["IRQ Time"] = fmt.Sprintf("%.2f seconds (%.1f%%)",
				node.cpuTimes.Irq, (node.cpuTimes.Irq/totalTime)*100)
		}
		if node.cpuTimes.Softirq > 0 {
			data["Soft IRQ Time"] = fmt.Sprintf("%.2f seconds (%.1f%%)",
				node.cpuTimes.Softirq, (node.cpuTimes.Softirq/totalTime)*100)
		}
		if node.cpuTimes.Steal > 0 {
			data["Steal Time"] = fmt.Sprintf("%.2f seconds (%.1f%%)",
				node.cpuTimes.Steal, (node.cpuTimes.Steal/totalTime)*100)
		}
		if node.cpuTimes.Guest > 0 {
			data["Guest Time"] = fmt.Sprintf("%.2f seconds (%.1f%%)",
				node.cpuTimes.Guest, (node.cpuTimes.Guest/totalTime)*100)
		}
		if node.cpuTimes.GuestNice > 0 {
			data["Guest Nice Time"] = fmt.Sprintf("%.2f seconds (%.1f%%)",
				node.cpuTimes.GuestNice, (node.cpuTimes.GuestNice/totalTime)*100)
		}
	}

	return data
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

	data := make(map[string]string)

	if node.memInfo.Count > 0 {
		data["Data Sources"] = fmt.Sprintf("%d processes reporting", node.memInfo.Count)
	}

	data["00:Memory usage"] = "Cumulative memory usage across all processes"

	// Primary memory metrics
	if node.memInfo.RSS > 0 {
		data["Resident Set Size"] = humanize.Bytes(node.memInfo.RSS)
	}
	if node.memInfo.VMS > 0 {
		data["Virtual Memory Size"] = humanize.Bytes(node.memInfo.VMS)
	}
	if node.memInfo.HWM > 0 {
		data["High Water Mark"] = humanize.Bytes(node.memInfo.HWM)
	}

	// Detailed memory breakdown
	if node.memInfo.Data > 0 {
		data["Data Segment"] = humanize.Bytes(node.memInfo.Data)
	}
	if node.memInfo.Stack > 0 {
		data["Stack Memory"] = humanize.Bytes(node.memInfo.Stack)
	}
	if node.memInfo.Shared > 0 {
		data["Shared Memory"] = humanize.Bytes(node.memInfo.Shared)
	}
	if node.memInfo.Locked > 0 {
		data["Locked Memory"] = humanize.Bytes(node.memInfo.Locked)
	}
	if node.memInfo.Swap > 0 {
		data["Swap Memory"] = humanize.Bytes(node.memInfo.Swap)
	}

	return data
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

	data := make(map[string]string)

	if node.ioCounters.Count > 0 {
		data["Data Sources"] = fmt.Sprintf("%d processes reporting", node.ioCounters.Count)
	}

	data["00:I/O"] = "Cumulative I/O operations across all processes"

	// Operation counts
	if node.ioCounters.ReadCount > 0 {
		data["Read Operations"] = humanize.Comma(int64(node.ioCounters.ReadCount))
	}
	if node.ioCounters.WriteCount > 0 {
		data["Write Operations"] = humanize.Comma(int64(node.ioCounters.WriteCount))
	}

	// Data transferred
	if node.ioCounters.ReadBytes > 0 {
		data["Bytes Read"] = humanize.Bytes(node.ioCounters.ReadBytes)
	}
	if node.ioCounters.WriteBytes > 0 {
		data["Bytes Written"] = humanize.Bytes(node.ioCounters.WriteBytes)
	}

	// Calculate averages if we have both counts and bytes
	if node.ioCounters.ReadCount > 0 && node.ioCounters.ReadBytes > 0 {
		avgReadSize := float64(node.ioCounters.ReadBytes) / float64(node.ioCounters.ReadCount)
		data["Average Read Size"] = humanize.Bytes(uint64(avgReadSize))
	}
	if node.ioCounters.WriteCount > 0 && node.ioCounters.WriteBytes > 0 {
		avgWriteSize := float64(node.ioCounters.WriteBytes) / float64(node.ioCounters.WriteCount)
		data["Average Write Size"] = humanize.Bytes(uint64(avgWriteSize))
	}

	return data
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

	data := make(map[string]string)

	if node.ctxSwitches.Count > 0 {
		data["Data Sources"] = fmt.Sprintf("%d processes reporting", node.ctxSwitches.Count)
	}

	data["00:Context Switches"] = "Cumulative context switches across all processes"

	totalSwitches := node.ctxSwitches.Voluntary + node.ctxSwitches.Involuntary

	if totalSwitches > 0 {
		data["Total Context Switches"] = humanize.Comma(totalSwitches)

		if node.ctxSwitches.Voluntary > 0 {
			voluntaryPercent := float64(node.ctxSwitches.Voluntary) / float64(totalSwitches) * 100
			data["Voluntary Switches"] = fmt.Sprintf("%s (%.1f%%)",
				humanize.Comma(node.ctxSwitches.Voluntary), voluntaryPercent)
		}

		if node.ctxSwitches.Involuntary > 0 {
			involuntaryPercent := float64(node.ctxSwitches.Involuntary) / float64(totalSwitches) * 100
			data["Involuntary Switches"] = fmt.Sprintf("%s (%.1f%%)",
				humanize.Comma(node.ctxSwitches.Involuntary), involuntaryPercent)
		}
	}

	return data
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

	data := make(map[string]string)

	if node.pageFaults.Count > 0 {
		data["Data Sources"] = fmt.Sprintf("%d processes reporting", node.pageFaults.Count)
	}

	data["00:Page Faults"] = "Cumulative page faults across all processes"

	totalFaults := node.pageFaults.MinorFaults + node.pageFaults.MajorFaults
	totalChildFaults := node.pageFaults.ChildMinorFaults + node.pageFaults.ChildMajorFaults

	if totalFaults > 0 {
		data["Total Page Faults"] = humanize.Comma(int64(totalFaults))

		if node.pageFaults.MinorFaults > 0 {
			minorPercent := float64(node.pageFaults.MinorFaults) / float64(totalFaults) * 100
			data["Minor Faults"] = fmt.Sprintf("%s (%.1f%%)",
				humanize.Comma(int64(node.pageFaults.MinorFaults)), minorPercent)
		}

		if node.pageFaults.MajorFaults > 0 {
			majorPercent := float64(node.pageFaults.MajorFaults) / float64(totalFaults) * 100
			data["Major Faults"] = fmt.Sprintf("%s (%.1f%%)",
				humanize.Comma(int64(node.pageFaults.MajorFaults)), majorPercent)
		}
	}

	if totalChildFaults > 0 {
		data["Child Process Faults"] = humanize.Comma(int64(totalChildFaults))

		if node.pageFaults.ChildMinorFaults > 0 {
			data["Child Minor Faults"] = humanize.Comma(int64(node.pageFaults.ChildMinorFaults))
		}

		if node.pageFaults.ChildMajorFaults > 0 {
			data["Child Major Faults"] = humanize.Comma(int64(node.pageFaults.ChildMajorFaults))
		}
	}

	return data
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

	data := make(map[string]string)
	data["Data Sources"] = fmt.Sprintf("%d processes reporting", node.memMaps.Count)
	data["00:Memory Maps"] = "Memory mapping details (platform-specific)"

	// Total mapping sizes
	if node.memMaps.TotalSize > 0 {
		data["Total Map Size"] = humanize.Bytes(node.memMaps.TotalSize)
	}
	if node.memMaps.TotalRSS > 0 {
		data["Total RSS"] = humanize.Bytes(node.memMaps.TotalRSS)
	}
	if node.memMaps.TotalPSS > 0 {
		data["Total PSS"] = humanize.Bytes(node.memMaps.TotalPSS)
	}

	// Shared memory
	if node.memMaps.TotalSharedClean > 0 {
		data["Shared Clean"] = humanize.Bytes(node.memMaps.TotalSharedClean)
	}
	if node.memMaps.TotalSharedDirty > 0 {
		data["Shared Dirty"] = humanize.Bytes(node.memMaps.TotalSharedDirty)
	}

	// Private memory
	if node.memMaps.TotalPrivateClean > 0 {
		data["Private Clean"] = humanize.Bytes(node.memMaps.TotalPrivateClean)
	}
	if node.memMaps.TotalPrivateDirty > 0 {
		data["Private Dirty"] = humanize.Bytes(node.memMaps.TotalPrivateDirty)
	}

	// Other memory details
	if node.memMaps.TotalReferenced > 0 {
		data["Referenced Memory"] = humanize.Bytes(node.memMaps.TotalReferenced)
	}
	if node.memMaps.TotalAnonymous > 0 {
		data["Anonymous Memory"] = humanize.Bytes(node.memMaps.TotalAnonymous)
	}
	if node.memMaps.TotalSwap > 0 {
		data["Swap Memory"] = humanize.Bytes(node.memMaps.TotalSwap)
	}

	return data
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

// ProcessLastDayNode shows last 24h process statistics with time segment navigation
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
func (node *ProcessLastDayNode) GetLeafData() map[string]string     { return nil }

func (node *ProcessLastDayNode) GetChildren() []MetricChild {
	if node.segmented == nil || len(node.segmented.Segments) == 0 {
		return nil
	}

	var children []MetricChild
	children = append(children, MetricChild{
		Name:        "Total",
		Description: "Aggregated process statistics across all time segments",
	})

	for i := len(node.segmented.Segments) - 1; i >= 0; i-- {
		seg := node.segmented.Segments[i]
		if seg.N == 0 {
			continue
		}

		segmentTime := node.segmented.FirstTime.Add(time.Duration(i*node.segmented.Interval) * time.Second)
		endTime := segmentTime.Add(time.Duration(node.segmented.Interval) * time.Second)
		segmentName := endTime.UTC().Format("15:04Z")

		avgCPU := seg.CPUPercent / float64(seg.N)
		avgRSS := seg.RSS / uint64(seg.N)
		throughput := seg.ReadBytes + seg.WriteBytes

		day := "Today "
		if segmentTime.Local().Day() != time.Now().Day() {
			day = "Yesterday "
		}

		children = append(children, MetricChild{
			Name: segmentName,
			Description: fmt.Sprintf("%s%s -> %s: CPU %.1f%%, RSS %s, I/O %s",
				day,
				segmentTime.Local().Format("15:04"),
				endTime.Local().Format("15:04"),
				avgCPU,
				humanize.Bytes(avgRSS),
				humanize.Bytes(throughput)),
		})
	}
	return children
}

func (node *ProcessLastDayNode) GetChild(name string) (MetricNode, error) {
	if node.segmented == nil {
		return nil, fmt.Errorf("no segmented data")
	}

	if name == "Total" {
		return &ProcessSegmentTotalNode{
			segmented: node.segmented,
			parent:    node,
			path:      node.path + "/Total",
		}, nil
	}

	for i := len(node.segmented.Segments) - 1; i >= 0; i-- {
		segmentTime := node.segmented.FirstTime.Add(time.Duration(i*node.segmented.Interval) * time.Second)
		endTime := segmentTime.Add(time.Duration(node.segmented.Interval) * time.Second)
		if endTime.UTC().Format("15:04Z") == name {
			return &ProcessTimeSegmentNode{
				segment:     node.segmented.Segments[i],
				segmentTime: segmentTime,
				interval:    node.segmented.Interval,
				parent:      node,
				path:        node.path + "/" + name,
			}, nil
		}
	}

	return nil, fmt.Errorf("time segment not found: %s", name)
}

// ProcessSegmentTotalNode shows aggregated process statistics across all time segments
type ProcessSegmentTotalNode struct {
	segmented *madmin.SegmentedProcessMetrics
	parent    MetricNode
	path      string
}

func (node *ProcessSegmentTotalNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *ProcessSegmentTotalNode) GetPath() string                    { return node.path }
func (node *ProcessSegmentTotalNode) GetParent() MetricNode              { return node.parent }
func (node *ProcessSegmentTotalNode) GetMetricType() madmin.MetricType   { return madmin.MetricsProcess }
func (node *ProcessSegmentTotalNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *ProcessSegmentTotalNode) ShouldPauseRefresh() bool           { return false }
func (node *ProcessSegmentTotalNode) GetChildren() []MetricChild         { return nil }

func (node *ProcessSegmentTotalNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("total is a leaf node")
}

func (node *ProcessSegmentTotalNode) GetLeafData() map[string]string {
	if node.segmented == nil || len(node.segmented.Segments) == 0 {
		return nil
	}

	total := node.segmented.Total()
	if total.N == 0 {
		return map[string]string{"Status": "No data collected"}
	}

	data := make(map[string]string)
	n := float64(total.N)

	// Time range
	firstTime := node.segmented.FirstTime
	lastSegmentTime := node.segmented.FirstTime.Add(time.Duration((len(node.segmented.Segments)-1)*node.segmented.Interval) * time.Second)
	endTime := lastSegmentTime.Add(time.Duration(node.segmented.Interval) * time.Second)
	data["00:Time Range"] = fmt.Sprintf("%s -> %s", firstTime.Local().Format("15:04"), endTime.Local().Format("15:04"))

	// CPU
	wallTime := float64(len(node.segmented.Segments) * node.segmented.Interval)
	data["01:CPU Usage"] = fmt.Sprintf("%.2f%% average", total.CPUPercent/n)
	if total.CPUUser > 0 {
		data["01a:CPU User"] = fmt.Sprintf("%.1fs (%.1f%% wall)", total.CPUUser, (total.CPUUser/wallTime)*100)
	}
	if total.CPUSystem > 0 {
		data["01b:CPU System"] = fmt.Sprintf("%.1fs (%.1f%% wall)", total.CPUSystem, (total.CPUSystem/wallTime)*100)
	}
	if total.CPUIdle > 0 {
		data["01c:CPU Idle"] = fmt.Sprintf("%.1fs (%.1f%% wall)", total.CPUIdle, (total.CPUIdle/wallTime)*100)
	}
	if total.CPUNice > 0 {
		data["01d:CPU Nice"] = fmt.Sprintf("%.1fs (%.1f%% wall)", total.CPUNice, (total.CPUNice/wallTime)*100)
	}
	if total.CPUIowait > 0 {
		data["01e:CPU IOwait"] = fmt.Sprintf("%.1fs (%.1f%% wall)", total.CPUIowait, (total.CPUIowait/wallTime)*100)
	}
	if total.CPUIrq > 0 {
		data["01f:CPU IRQ"] = fmt.Sprintf("%.1fs (%.1f%% wall)", total.CPUIrq, (total.CPUIrq/wallTime)*100)
	}
	if total.CPUSoftirq > 0 {
		data["01g:CPU SoftIRQ"] = fmt.Sprintf("%.1fs (%.1f%% wall)", total.CPUSoftirq, (total.CPUSoftirq/wallTime)*100)
	}
	if total.CPUSteal > 0 {
		data["01h:CPU Steal"] = fmt.Sprintf("%.1fs (%.1f%% wall)", total.CPUSteal, (total.CPUSteal/wallTime)*100)
	}
	if total.CPUGuest > 0 {
		data["01i:CPU Guest"] = fmt.Sprintf("%.1fs (%.1f%% wall)", total.CPUGuest, (total.CPUGuest/wallTime)*100)
	}
	if total.CPUGuestNice > 0 {
		data["01j:CPU GuestNice"] = fmt.Sprintf("%.1fs (%.1f%% wall)", total.CPUGuestNice, (total.CPUGuestNice/wallTime)*100)
	}

	// Memory
	data["02:RSS"] = humanize.Bytes(total.RSS/uint64(total.N)) + " average"
	data["03:VMS"] = humanize.Bytes(total.VMS/uint64(total.N)) + " average"

	// Threads/FDs/Connections
	data["04:Threads"] = humanize.Comma(total.NumThreads/int64(total.N)) + " average"
	data["05:FDs"] = humanize.Comma(total.NumFDs/int64(total.N)) + " average"
	data["06:Connections"] = humanize.Comma(int64(total.NumConnections/total.N)) + " average"

	// I/O totals
	data["07:Read Ops"] = humanize.Comma(int64(total.ReadCount))
	data["08:Write Ops"] = humanize.Comma(int64(total.WriteCount))
	data["09:Bytes Read"] = humanize.Bytes(total.ReadBytes)
	data["10:Bytes Written"] = humanize.Bytes(total.WriteBytes)

	// Context switches
	data["11:Voln Ctx Sw"] = humanize.Comma(total.CtxSwitchesVoluntary)
	data["12:Involn Ctx Sw"] = humanize.Comma(total.CtxSwitchesInvoluntary)

	// Page faults
	data["13:Minor Faults"] = humanize.Comma(int64(total.MinorFaults))
	data["14:Major Faults"] = humanize.Comma(int64(total.MajorFaults))

	// CPU time breakdown (show as percentages of total CPU time)
	totalCPUTime := total.CPUUser + total.CPUSystem + total.CPUIdle + total.CPUNice +
		total.CPUIowait + total.CPUIrq + total.CPUSoftirq + total.CPUSteal +
		total.CPUGuest + total.CPUGuestNice
	if totalCPUTime > 0 {
		data["15a:CPU User %"] = fmt.Sprintf("%.1f%% of cpu", (total.CPUUser/totalCPUTime)*100)
		data["15b:CPU System %"] = fmt.Sprintf("%.1f%% of cpu", (total.CPUSystem/totalCPUTime)*100)
		if total.CPUIdle > 0 {
			data["15c:CPU Idle %"] = fmt.Sprintf("%.1f%% of cpu", (total.CPUIdle/totalCPUTime)*100)
		}
		if total.CPUNice > 0 {
			data["15d:CPU Nice %"] = fmt.Sprintf("%.1f%% of cpu", (total.CPUNice/totalCPUTime)*100)
		}
		if total.CPUIowait > 0 {
			data["15e:CPU IOwait %"] = fmt.Sprintf("%.1f%% of cpu", (total.CPUIowait/totalCPUTime)*100)
		}
		if total.CPUIrq > 0 {
			data["15f:CPU IRQ %"] = fmt.Sprintf("%.1f%% of cpu", (total.CPUIrq/totalCPUTime)*100)
		}
		if total.CPUSoftirq > 0 {
			data["15g:CPU SoftIRQ %"] = fmt.Sprintf("%.1f%% of cpu", (total.CPUSoftirq/totalCPUTime)*100)
		}
		if total.CPUSteal > 0 {
			data["15h:CPU Steal %"] = fmt.Sprintf("%.1f%% of cpu", (total.CPUSteal/totalCPUTime)*100)
		}
		if total.CPUGuest > 0 {
			data["15i:CPU Guest %"] = fmt.Sprintf("%.1f%% of cpu", (total.CPUGuest/totalCPUTime)*100)
		}
		if total.CPUGuestNice > 0 {
			data["15j:CPU GuestNice %"] = fmt.Sprintf("%.1f%% of cpu", (total.CPUGuestNice/totalCPUTime)*100)
		}
	}

	return data
}

// ProcessTimeSegmentNode shows process statistics for a specific time segment
type ProcessTimeSegmentNode struct {
	segment     madmin.ProcessSegment
	segmentTime time.Time
	interval    int
	parent      MetricNode
	path        string
}

func (node *ProcessTimeSegmentNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *ProcessTimeSegmentNode) GetPath() string                    { return node.path }
func (node *ProcessTimeSegmentNode) GetParent() MetricNode              { return node.parent }
func (node *ProcessTimeSegmentNode) GetMetricType() madmin.MetricType   { return madmin.MetricsProcess }
func (node *ProcessTimeSegmentNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *ProcessTimeSegmentNode) ShouldPauseRefresh() bool           { return false }
func (node *ProcessTimeSegmentNode) GetChildren() []MetricChild         { return nil }

func (node *ProcessTimeSegmentNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("time segment is a leaf node")
}

func (node *ProcessTimeSegmentNode) GetLeafData() map[string]string {
	seg := node.segment
	if seg.N == 0 {
		return map[string]string{"Status": "No data for this segment"}
	}

	data := make(map[string]string)
	n := float64(seg.N)

	// Time range
	endTime := node.segmentTime.Add(time.Duration(node.interval) * time.Second)
	data["00:Time Range"] = fmt.Sprintf("%s -> %s", node.segmentTime.Local().Format("15:04"), endTime.Local().Format("15:04"))

	// CPU
	wallTime := float64(node.interval)
	data["01:CPU Usage"] = fmt.Sprintf("%.2f%% average", seg.CPUPercent/n)
	if seg.CPUUser > 0 {
		data["01a:CPU User"] = fmt.Sprintf("%.1fs (%.1f%% wall)", seg.CPUUser, (seg.CPUUser/wallTime)*100)
	}
	if seg.CPUSystem > 0 {
		data["01b:CPU System"] = fmt.Sprintf("%.1fs (%.1f%% wall)", seg.CPUSystem, (seg.CPUSystem/wallTime)*100)
	}
	if seg.CPUIdle > 0 {
		data["01c:CPU Idle"] = fmt.Sprintf("%.1fs (%.1f%% wall)", seg.CPUIdle, (seg.CPUIdle/wallTime)*100)
	}
	if seg.CPUNice > 0 {
		data["01d:CPU Nice"] = fmt.Sprintf("%.1fs (%.1f%% wall)", seg.CPUNice, (seg.CPUNice/wallTime)*100)
	}
	if seg.CPUIowait > 0 {
		data["01e:CPU IOwait"] = fmt.Sprintf("%.1fs (%.1f%% wall)", seg.CPUIowait, (seg.CPUIowait/wallTime)*100)
	}
	if seg.CPUIrq > 0 {
		data["01f:CPU IRQ"] = fmt.Sprintf("%.1fs (%.1f%% wall)", seg.CPUIrq, (seg.CPUIrq/wallTime)*100)
	}
	if seg.CPUSoftirq > 0 {
		data["01g:CPU SoftIRQ"] = fmt.Sprintf("%.1fs (%.1f%% wall)", seg.CPUSoftirq, (seg.CPUSoftirq/wallTime)*100)
	}
	if seg.CPUSteal > 0 {
		data["01h:CPU Steal"] = fmt.Sprintf("%.1fs (%.1f%% wall)", seg.CPUSteal, (seg.CPUSteal/wallTime)*100)
	}
	if seg.CPUGuest > 0 {
		data["01i:CPU Guest"] = fmt.Sprintf("%.1fs (%.1f%% wall)", seg.CPUGuest, (seg.CPUGuest/wallTime)*100)
	}
	if seg.CPUGuestNice > 0 {
		data["01j:CPU GuestNice"] = fmt.Sprintf("%.1fs (%.1f%% wall)", seg.CPUGuestNice, (seg.CPUGuestNice/wallTime)*100)
	}

	// Memory
	data["02:RSS"] = humanize.Bytes(seg.RSS/uint64(seg.N)) + " average"
	data["03:VMS"] = humanize.Bytes(seg.VMS/uint64(seg.N)) + " average"

	// Threads/FDs/Connections
	data["04:Threads"] = humanize.Comma(seg.NumThreads/int64(seg.N)) + " average"
	data["05:FDs"] = humanize.Comma(seg.NumFDs/int64(seg.N)) + " average"
	data["06:Connections"] = humanize.Comma(int64(seg.NumConnections/seg.N)) + " average"

	// I/O
	data["07:Read Ops"] = humanize.Comma(int64(seg.ReadCount))
	data["08:Write Ops"] = humanize.Comma(int64(seg.WriteCount))
	data["09:Bytes Read"] = humanize.Bytes(seg.ReadBytes)
	data["10:Bytes Written"] = humanize.Bytes(seg.WriteBytes)

	// Context switches
	data["11:Voln Ctx Sw"] = humanize.Comma(seg.CtxSwitchesVoluntary)
	data["12:Involn Ctx Sw"] = humanize.Comma(seg.CtxSwitchesInvoluntary)

	// Page faults
	data["13:Minor Faults"] = humanize.Comma(int64(seg.MinorFaults))
	data["14:Major Faults"] = humanize.Comma(int64(seg.MajorFaults))

	// CPU time breakdown
	totalCPUTime := seg.CPUUser + seg.CPUSystem + seg.CPUIdle + seg.CPUNice +
		seg.CPUIowait + seg.CPUIrq + seg.CPUSoftirq + seg.CPUSteal +
		seg.CPUGuest + seg.CPUGuestNice
	if totalCPUTime > 0 {
		data["15a:CPU User %"] = fmt.Sprintf("%.1f%% of cpu", (seg.CPUUser/totalCPUTime)*100)
		data["15b:CPU System %"] = fmt.Sprintf("%.1f%% of cpu", (seg.CPUSystem/totalCPUTime)*100)
		if seg.CPUIdle > 0 {
			data["15c:CPU Idle %"] = fmt.Sprintf("%.1f%% of cpu", (seg.CPUIdle/totalCPUTime)*100)
		}
		if seg.CPUNice > 0 {
			data["15d:CPU Nice %"] = fmt.Sprintf("%.1f%% of cpu", (seg.CPUNice/totalCPUTime)*100)
		}
		if seg.CPUIowait > 0 {
			data["15e:CPU IOwait %"] = fmt.Sprintf("%.1f%% of cpu", (seg.CPUIowait/totalCPUTime)*100)
		}
		if seg.CPUIrq > 0 {
			data["15f:CPU IRQ %"] = fmt.Sprintf("%.1f%% of cpu", (seg.CPUIrq/totalCPUTime)*100)
		}
		if seg.CPUSoftirq > 0 {
			data["15g:CPU SoftIRQ %"] = fmt.Sprintf("%.1f%% of cpu", (seg.CPUSoftirq/totalCPUTime)*100)
		}
		if seg.CPUSteal > 0 {
			data["15h:CPU Steal %"] = fmt.Sprintf("%.1f%% of cpu", (seg.CPUSteal/totalCPUTime)*100)
		}
		if seg.CPUGuest > 0 {
			data["15i:CPU Guest %"] = fmt.Sprintf("%.1f%% of cpu", (seg.CPUGuest/totalCPUTime)*100)
		}
		if seg.CPUGuestNice > 0 {
			data["15j:CPU GuestNice %"] = fmt.Sprintf("%.1f%% of cpu", (seg.CPUGuestNice/totalCPUTime)*100)
		}
	}

	return data
}
