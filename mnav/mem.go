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
	"time"

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
	children := []MetricChild{
		{Name: "usage", Description: "Core memory usage statistics and utilization"},
		{Name: "system", Description: "System memory details (cache, buffers, shared)"},
		{Name: "swap", Description: "Swap space information and utilization"},
		{Name: "limits", Description: "Memory limits and cgroup configuration"},
	}
	children = append(children, MetricChild{Name: "last_day", Description: "Last 24h memory statistics"})
	return children
}

func (node *MemMetricsNavigator) GetLeafData() map[string]string {
	if node.mem == nil {
		return map[string]string{"Status": "Memory metrics not available"}
	}

	data := map[string]string{
		"Collected At": node.mem.CollectedAt.Format("2006-01-02 15:04:05"),
	}
	if node.mem.Nodes > 0 {
		data["Nodes"] = strconv.Itoa(node.mem.Nodes)
	}
	// Add high-level memory summary with averages
	if node.mem.Info.Total > 0 {
		data["Total Memory"] = fmt.Sprintf("%s across %d nodes",
			formatMemoryBytes(node.mem.Info.Total), node.mem.Nodes)

		if node.mem.Nodes > 0 {
			avgTotal := node.mem.Info.Total / uint64(node.mem.Nodes)
			data["Avg per Node"] = formatMemoryBytes(avgTotal)

			if node.mem.Info.Used > 0 {
				avgUsed := node.mem.Info.Used / uint64(node.mem.Nodes)
				data["Avg Used"] = fmt.Sprintf("%s (%s)",
					formatMemoryBytes(avgUsed),
					calculatePercentage(node.mem.Info.Used, node.mem.Info.Total))
			}

			if node.mem.Info.Available > 0 {
				avgAvailable := node.mem.Info.Available / uint64(node.mem.Nodes)
				data["Avg Available"] = fmt.Sprintf("%s (%s)",
					formatMemoryBytes(avgAvailable),
					calculatePercentage(node.mem.Info.Available, node.mem.Info.Total))
			}
		} else {
			// Fallback if node count is not available
			if node.mem.Info.Used > 0 {
				data["Used"] = fmt.Sprintf("%s (%s)",
					formatMemoryBytes(node.mem.Info.Used),
					calculatePercentage(node.mem.Info.Used, node.mem.Info.Total))
			}

			if node.mem.Info.Available > 0 {
				data["Available"] = fmt.Sprintf("%s (%s)",
					formatMemoryBytes(node.mem.Info.Available),
					calculatePercentage(node.mem.Info.Available, node.mem.Info.Total))
			}
		}
	}

	return data
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
	case "last_day":
		return NewMemLastDayNode(node.mem.LastDay, node, fmt.Sprintf("%s/last_day", node.path)), nil
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

	data := map[string]string{}
	info := node.mem.Info

	if info.Total > 0 {
		data["Total"] = fmt.Sprintf("%s across cluster", formatMemoryBytes(info.Total))

		if node.mem.Nodes > 0 {
			avgPerNode := info.Total / uint64(node.mem.Nodes)
			data["Avg per Node"] = fmt.Sprintf("%s per node", formatMemoryBytes(avgPerNode))
		}
	}

	if info.Used > 0 {
		usedPercent := calculatePercentage(info.Used, info.Total)
		data["Used"] = fmt.Sprintf("%s (%s)", formatMemoryBytes(info.Used), usedPercent)
	}

	if info.Free > 0 {
		freePercent := calculatePercentage(info.Free, info.Total)
		data["Free"] = fmt.Sprintf("%s (%s)", formatMemoryBytes(info.Free), freePercent)
	}

	if info.Available > 0 {
		availPercent := calculatePercentage(info.Available, info.Total)
		data["Available"] = fmt.Sprintf("%s (%s)", formatMemoryBytes(info.Available), availPercent)

		if info.Total > 0 {
			availableRatio := float64(info.Available) / float64(info.Total)
			var pressureStatus string
			if availableRatio > 0.5 {
				pressureStatus = "Low pressure"
			} else if availableRatio > 0.2 {
				pressureStatus = "Moderate pressure"
			} else {
				pressureStatus = "High pressure"
			}
			data["Pressure"] = pressureStatus
		}
	}

	if info.Used > 0 && info.Total > 0 {
		utilizationRatio := float64(info.Used) / float64(info.Total)
		var efficiencyNote string
		if utilizationRatio > 0.8 {
			efficiencyNote = "High utilization"
		} else if utilizationRatio > 0.6 {
			efficiencyNote = "Good utilization"
		} else {
			efficiencyNote = "Low utilization"
		}
		data["Efficiency"] = efficiencyNote
	}

	return data
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

	data := map[string]string{}
	info := node.mem.Info

	// Cache memory
	if info.Cache > 0 {
		cachePercent := calculatePercentage(info.Cache, info.Total)
		data["Cache"] = fmt.Sprintf("%s (%s)",
			formatMemoryBytes(info.Cache), cachePercent)
	}

	// Buffer memory
	if info.Buffers > 0 {
		bufferPercent := calculatePercentage(info.Buffers, info.Total)
		data["Buffers"] = fmt.Sprintf("%s (%s)",
			formatMemoryBytes(info.Buffers), bufferPercent)
	}

	// Shared memory
	if info.Shared > 0 {
		sharedPercent := calculatePercentage(info.Shared, info.Total)
		data["Shared"] = fmt.Sprintf("%s (%s)",
			formatMemoryBytes(info.Shared), sharedPercent)
	}

	// System memory efficiency
	if info.Cache > 0 || info.Buffers > 0 {
		systemMemory := info.Cache + info.Buffers
		if info.Total > 0 {
			systemPercent := calculatePercentage(systemMemory, info.Total)
			data["System Total"] = fmt.Sprintf("%s (%s)",
				formatMemoryBytes(systemMemory), systemPercent)

			if info.Cache > 0 && info.Buffers > 0 {
				cacheRatio := float64(info.Cache) / float64(systemMemory) * 100
				data["Cache/Buffer"] = fmt.Sprintf("%.1f%% / %.1f%%",
					cacheRatio, 100-cacheRatio)
			}
		}
	}

	// Analysis of system memory health
	if info.Total > 0 && (info.Cache > 0 || info.Buffers > 0) {
		systemTotal := info.Cache + info.Buffers
		systemRatio := float64(systemTotal) / float64(info.Total)

		var healthNote string
		if systemRatio > 0.3 {
			healthNote = "High usage - good I/O performance"
		} else if systemRatio > 0.1 {
			healthNote = "Moderate usage - balanced"
		} else {
			healthNote = "Low usage - cache available"
		}
		data["Health"] = healthNote
	}

	return data
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

	data := map[string]string{}
	info := node.mem.Info

	// Total swap space
	if info.SwapSpaceTotal > 0 {
		data["Total Swap"] = fmt.Sprintf("%s across cluster",
			formatMemoryBytes(info.SwapSpaceTotal))

		if node.mem.Nodes > 0 {
			avgSwapPerNode := info.SwapSpaceTotal / uint64(node.mem.Nodes)
			data["Avg per Node"] = fmt.Sprintf("%s per node",
				formatMemoryBytes(avgSwapPerNode))
		}
	}

	// Free swap space
	if info.SwapSpaceFree > 0 {
		freeSwapPercent := calculatePercentage(info.SwapSpaceFree, info.SwapSpaceTotal)
		data["Free Swap"] = fmt.Sprintf("%s (%s)",
			formatMemoryBytes(info.SwapSpaceFree), freeSwapPercent)
	}

	// Used swap calculation and analysis
	if info.SwapSpaceTotal > 0 && info.SwapSpaceFree > 0 {
		swapUsed := info.SwapSpaceTotal - info.SwapSpaceFree
		if swapUsed > 0 {
			usedSwapPercent := calculatePercentage(swapUsed, info.SwapSpaceTotal)
			data["Used Swap"] = fmt.Sprintf("%s (%s)",
				formatMemoryBytes(swapUsed), usedSwapPercent)

			// Swap usage health indicator
			swapUsageRatio := float64(swapUsed) / float64(info.SwapSpaceTotal)
			var swapHealth string
			if swapUsageRatio < 0.1 {
				swapHealth = "Minimal usage - sufficient RAM"
			} else if swapUsageRatio < 0.5 {
				swapHealth = "Moderate usage - monitor pressure"
			} else {
				swapHealth = "High usage - consider more RAM"
			}
			data["Health"] = swapHealth
		} else {
			data["Used Swap"] = "0 bytes - no swap in use"
			data["Health"] = "Optimal - all in RAM"
		}
	} else if info.SwapSpaceTotal == 0 {
		data["Config"] = "No swap configured"
		data["Note"] = "Consider swap for overflow protection"
	}

	// Memory vs Swap ratio analysis
	if info.Total > 0 && info.SwapSpaceTotal > 0 {
		swapRatio := float64(info.SwapSpaceTotal) / float64(info.Total)
		data["Swap:RAM"] = fmt.Sprintf("%.1f:1", swapRatio)

		var ratioAnalysis string
		if swapRatio > 2.0 {
			ratioAnalysis = "Excellent protection"
		} else if swapRatio > 1.0 {
			ratioAnalysis = "Good protection"
		} else if swapRatio > 0.5 {
			ratioAnalysis = "Basic protection"
		} else {
			ratioAnalysis = "Limited protection"
		}
		data["Protection"] = ratioAnalysis
	}

	return data
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

	data := map[string]string{}
	info := node.mem.Info

	// Cgroup memory limit analysis
	if info.Limit > 0 {
		data["Limit"] = formatMemoryBytes(info.Limit)

		// Compare limit to physical memory
		if info.Total > 0 {
			if info.Limit == info.Total {
				data["Type"] = "No cgroup limit - full memory"
			} else if info.Limit < info.Total {
				limitPercent := calculatePercentage(info.Limit, info.Total)
				data["Type"] = fmt.Sprintf("Limited to %s of physical", limitPercent)

				// Headroom analysis
				if info.Used > 0 {
					limitHeadroom := info.Limit - info.Used
					headroomPercent := calculatePercentage(limitHeadroom, info.Limit)
					data["Headroom"] = fmt.Sprintf("%s (%s)",
						formatMemoryBytes(limitHeadroom), headroomPercent)

					// Limit pressure indicator
					usageRatio := float64(info.Used) / float64(info.Limit)
					var pressureStatus string
					if usageRatio > 0.9 {
						pressureStatus = "Critical - very close to limit"
					} else if usageRatio > 0.8 {
						pressureStatus = "High - approaching limit"
					} else if usageRatio > 0.6 {
						pressureStatus = "Moderate - comfortable distance"
					} else {
						pressureStatus = "Low - plenty of headroom"
					}
					data["Pressure"] = pressureStatus
				}
			} else {
				data["Type"] = "Limit exceeds physical memory (misconfigured)"
			}
		}
	} else if info.Total > 0 {
		data["Limit"] = "No cgroup limit configured"
		data["Type"] = "Full memory without restrictions"
		data["Note"] = "Consider limits for resource management"
	}

	// Memory governance analysis
	if info.Limit > 0 && info.Total > 0 && info.Used > 0 {
		effectiveLimit := info.Limit
		if info.Limit > info.Total {
			effectiveLimit = info.Total
		}

		utilizationAgainstLimit := float64(info.Used) / float64(effectiveLimit)
		var governanceNote string
		if utilizationAgainstLimit < 0.5 {
			governanceNote = "Conservative - good margin"
		} else if utilizationAgainstLimit < 0.8 {
			governanceNote = "Healthy - approaching optimal"
		} else {
			governanceNote = "High - monitor for violations"
		}
		data["Status"] = governanceNote
	}

	return data
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

// MemLastDayNode shows last 24h memory statistics
type MemLastDayNode struct {
	segmented *madmin.SegmentedMemMetrics
	parent    MetricNode
	path      string
}

func NewMemLastDayNode(segmented *madmin.SegmentedMemMetrics, parent MetricNode, path string) *MemLastDayNode {
	return &MemLastDayNode{segmented: segmented, parent: parent, path: path}
}

func (node *MemLastDayNode) GetOpts() madmin.MetricsOptions    { return getNodeOpts(node) }
func (node *MemLastDayNode) GetPath() string                   { return node.path }
func (node *MemLastDayNode) GetParent() MetricNode             { return node.parent }
func (node *MemLastDayNode) GetMetricType() madmin.MetricType  { return madmin.MetricsMem }
func (node *MemLastDayNode) GetMetricFlags() madmin.MetricFlags { return madmin.MetricsDayStats }
func (node *MemLastDayNode) ShouldPauseRefresh() bool          { return true }
func (node *MemLastDayNode) GetChildren() []MetricChild        { return nil }

func (node *MemLastDayNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children")
}

func (node *MemLastDayNode) GetLeafData() map[string]string {
	if node.segmented == nil || len(node.segmented.Segments) == 0 {
		return nil
	}
	data := make(map[string]string)
	idx := 0
	for i := len(node.segmented.Segments) - 1; i >= 0; i-- {
		seg := node.segmented.Segments[i]
		if seg.N == 0 {
			continue
		}
		idx++
		startTime := node.segmented.FirstTime.Add(time.Duration(i*node.segmented.Interval) * time.Second)
		endTime := startTime.Add(time.Duration(node.segmented.Interval) * time.Second)
		name := fmt.Sprintf("%02d: %s->%s", idx, startTime.Local().Format("15:04"), endTime.Local().Format("15:04"))

		avgUsed := seg.Used / uint64(seg.N)
		avgFree := seg.Free / uint64(seg.N)
		total := avgUsed + avgFree
		pct := float64(0)
		if total > 0 {
			pct = float64(avgUsed) / float64(total) * 100
		}
		data[name] = fmt.Sprintf("Used: %s (%.1f%%), Free: %s", formatMemoryBytes(avgUsed), pct, formatMemoryBytes(avgFree))
	}
	return data
}
