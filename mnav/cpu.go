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
	"time"

	"github.com/dustin/go-humanize"
	"github.com/minio/madmin-go/v4"
)

// formatFrequency formats frequency values
func formatFrequency(freq uint64) string {
	return humanize.SI(float64(freq), "Hz")
}

// CPUMetricsNavigator provides navigation for CPU metrics
type CPUMetricsNavigator struct {
	cpu    *madmin.CPUMetrics
	parent MetricNode
	path   string
}

func (node *CPUMetricsNavigator) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *CPUMetricsNavigator) ShouldPauseRefresh() bool {
	return false
}

// NewCPUMetricsNavigator creates a new CPU metrics navigator
func NewCPUMetricsNavigator(cpu *madmin.CPUMetrics, parent MetricNode, path string) *CPUMetricsNavigator {
	return &CPUMetricsNavigator{cpu: cpu, parent: parent, path: path}
}

func (node *CPUMetricsNavigator) GetChildren() []MetricChild {
	if node.cpu == nil {
		return nil
	}
	return []MetricChild{
		{Name: "last_hour", Description: "Last hour CPU usage (1-min segments)"},
		{Name: "last_day", Description: "Last 24h CPU usage (15-min segments)"},
		{Name: "power_last_hour", Description: "Last hour power draw (1-min segments)"},
		{Name: "power_last_day", Description: "Last 24h power draw (15-min segments)"},
	}
}

func (node *CPUMetricsNavigator) GetLeafData() map[string]string {
	if node.cpu == nil {
		return map[string]string{"Error": "CPU metrics not available"}
	}

	// Use ordered slice to maintain consistent display order
	var entries []struct{ key, value string }
	addEntry := func(key, value string) {
		entries = append(entries, struct{ key, value string }{key, value})
	}

	// CPU Overview
	addEntry("Overview", fmt.Sprintf("Collected at %s",
		node.cpu.CollectedAt.Format("2006-01-02 15:04:05")))

	// Cluster Architecture
	if node.cpu.Nodes > 0 {
		addEntry("Cluster Architecture", fmt.Sprintf("%s nodes, %s total CPUs (%s CPUs/node avg)",
			humanize.Comma(int64(node.cpu.Nodes)),
			humanize.Comma(int64(node.cpu.CPUCount)),
			fmt.Sprintf("%.1f", float64(node.cpu.CPUCount)/float64(node.cpu.Nodes))))

		if node.cpu.TotalCores > 0 {
			addEntry("Processing Cores", fmt.Sprintf("%s total cores (%s cores/node avg, %.1f cores/CPU avg)",
				humanize.Comma(int64(node.cpu.TotalCores)),
				fmt.Sprintf("%.1f", float64(node.cpu.TotalCores)/float64(node.cpu.Nodes)),
				float64(node.cpu.TotalCores)/float64(node.cpu.CPUCount)))
		}
	}

	// Performance Summary
	if node.cpu.TotalMhz > 0 {
		totalGhz := node.cpu.TotalMhz / 1000
		addEntry("Processing Power", fmt.Sprintf("%.2f GHz total cluster capacity",
			totalGhz))
		if node.cpu.Nodes > 0 {
			avgGhzPerNode := totalGhz / float64(node.cpu.Nodes)
			addEntry("CPU Speed", fmt.Sprintf("%.2f GHz average per node",
				avgGhzPerNode))
		}
	}

	// Frequency Analysis
	if node.cpu.FreqStatsCount > 0 {
		currentFreq := node.cpu.TotalCurrentFreq / uint64(node.cpu.FreqStatsCount)
		maxFreq := node.cpu.MaxCPUInfoFreq

		addEntry("FREQUENCY ANALYSIS", fmt.Sprintf("%d CPUs monitored for frequency",
			node.cpu.FreqStatsCount))

		addEntry("Current Performance", fmt.Sprintf("%s average frequency",
			formatFrequency(currentFreq)))

		if maxFreq > 0 {
			utilizationPercent := float64(currentFreq) / float64(maxFreq) * 100
			addEntry("Frequency Utilization", fmt.Sprintf("%.1f%% of maximum capability (%s max)",
				utilizationPercent, formatFrequency(maxFreq)))
		}

		if node.cpu.MinCPUInfoFreq > 0 && node.cpu.MaxCPUInfoFreq > 0 {
			addEntry("Frequency Range", fmt.Sprintf("%s - %s available range",
				formatFrequency(node.cpu.MinCPUInfoFreq),
				formatFrequency(node.cpu.MaxCPUInfoFreq)))
		}
	}

	// Cache Architecture
	if node.cpu.TotalCacheSize > 0 {
		totalCacheGB := float64(node.cpu.TotalCacheSize) / (1024 * 1024 * 1024)
		addEntry("Cache Architecture", fmt.Sprintf("%.2f GB total cache across cluster",
			totalCacheGB))
		if node.cpu.Nodes > 0 {
			avgCacheMB := float64(node.cpu.TotalCacheSize) / (1024 * 1024 * float64(node.cpu.Nodes))
			addEntry("Cache per Node", fmt.Sprintf("%.1f MB average per node",
				avgCacheMB))
		}
	}

	// Hardware Diversity
	if len(node.cpu.CPUByModel) > 0 {
		addEntry("CPU Models", fmt.Sprintf("%d distinct CPU models deployed",
			len(node.cpu.CPUByModel)))

		// Find most common CPU model
		var mostCommonModel string
		var maxCount int
		for model, count := range node.cpu.CPUByModel {
			if count > maxCount {
				maxCount = count
				mostCommonModel = model
			}
		}
		if mostCommonModel != "" {
			percentage := float64(maxCount) / float64(node.cpu.CPUCount) * 100
			modelDisplay := mostCommonModel
			if len(modelDisplay) > 50 {
				modelDisplay = modelDisplay[:47] + "..."
			}
			addEntry("Primary CPU Model", fmt.Sprintf("%s (%d CPUs, %.1f%%)",
				modelDisplay, maxCount, percentage))
		}
	}

	// Governor Configuration
	if len(node.cpu.GovernorFreq) > 0 {
		addEntry("Power Management", fmt.Sprintf("%d frequency governors active",
			len(node.cpu.GovernorFreq)))

		// Find most common governor
		var primaryGovernor string
		var maxCount int
		for governor, count := range node.cpu.GovernorFreq {
			if count > maxCount {
				maxCount = count
				primaryGovernor = governor
			}
		}
		if primaryGovernor != "" {
			addEntry("Primary Governor", fmt.Sprintf("%s (%d CPUs)",
				primaryGovernor, maxCount))
		}
	}

	// CPU Times Breakdown - divide by number of nodes to get averages
	if node.cpu.LoadStatCount > 0 {
		times := node.cpu.TimesStat
		nodeCount := float64(node.cpu.LoadStatCount)

		// Average the times across all nodes
		avgUser := times.User / nodeCount
		avgSystem := times.System / nodeCount
		avgIdle := times.Idle / nodeCount
		avgNice := times.Nice / nodeCount
		avgIowait := times.Iowait / nodeCount
		avgIrq := times.Irq / nodeCount
		avgSoftirq := times.Softirq / nodeCount
		avgSteal := times.Steal / nodeCount
		avgGuest := times.Guest / nodeCount
		avgGuestNice := times.GuestNice / nodeCount

		// Calculate total time for percentages (using averaged values)
		totalTime := avgUser + avgSystem + avgIdle + avgNice +
			avgIowait + avgIrq + avgSoftirq + avgSteal +
			avgGuest + avgGuestNice

		if totalTime > 0 {
			addEntry("User Time", fmt.Sprintf("%.1f%% (%.2fs avg)", (avgUser/totalTime)*100, avgUser))
			addEntry("System Time", fmt.Sprintf("%.1f%% (%.2fs avg)", (avgSystem/totalTime)*100, avgSystem))
			addEntry("Idle Time", fmt.Sprintf("%.1f%% (%.2fs avg)", (avgIdle/totalTime)*100, avgIdle))

			// Only show non-zero times to keep display clean
			addEntry("Nice Time", fmt.Sprintf("%.1f%% (%.2fs avg)", (avgNice/totalTime)*100, avgNice))
			addEntry("IO Wait Time", fmt.Sprintf("%.1f%% (%.2fs avg)", (avgIowait/totalTime)*100, avgIowait))
			addEntry("IRQ Time", fmt.Sprintf("%.1f%% (%.2fs avg)", (avgIrq/totalTime)*100, avgIrq))
			addEntry("Soft IRQ Time", fmt.Sprintf("%.1f%% (%.2fs avg)", (avgSoftirq/totalTime)*100, avgSoftirq))
			addEntry("Steal Time", fmt.Sprintf("%.1f%% (%.2fs avg)", (avgSteal/totalTime)*100, avgSteal))
			addEntry("Guest Time", fmt.Sprintf("%.1f%% (%.2fs avg)", (avgGuest/totalTime)*100, avgGuest))
			addEntry("Guest Nice Time", fmt.Sprintf("%.1f%% (%.2fs avg)", (avgGuestNice/totalTime)*100, avgGuestNice))
		}

		// Load Averages - divide by number of nodes to get averages
		if node.cpu.LoadStatCount > 0 {
			load := node.cpu.LoadStat
			addEntry("Load 1min", fmt.Sprintf("%.2f avg", load.Load1/float64(node.cpu.LoadStatCount)))
			addEntry("Load 5min", fmt.Sprintf("%.2f avg", load.Load5/float64(node.cpu.LoadStatCount)))
			addEntry("Load 15min", fmt.Sprintf("%.2f avg", load.Load15/float64(node.cpu.LoadStatCount)))
		}
	}

	// Frequency Information
	if node.cpu.FreqStatsCount > 0 {
		currentFreq := node.cpu.TotalCurrentFreq / uint64(node.cpu.FreqStatsCount)
		addEntry("Current Frequency", formatFrequency(currentFreq))

		if node.cpu.MaxCPUInfoFreq > 0 {
			utilization := float64(currentFreq) / float64(node.cpu.MaxCPUInfoFreq) * 100
			addEntry("Frequency Utilization", fmt.Sprintf("%.1f%%", utilization))
		}

		if node.cpu.TotalScalingCurrentFreq > 0 {
			scalingFreq := node.cpu.TotalScalingCurrentFreq / uint64(node.cpu.FreqStatsCount)
			addEntry("Scaling Frequency", formatFrequency(scalingFreq))
		}
	}

	// CPU Models Distribution
	if len(node.cpu.CPUByModel) > 0 {
		totalCPUs := 0
		for _, count := range node.cpu.CPUByModel {
			totalCPUs += count
		}

		// Sort models by count (descending)
		type modelStat struct {
			name  string
			count int
		}
		var models []modelStat
		for name, count := range node.cpu.CPUByModel {
			models = append(models, modelStat{name, count})
		}
		sort.Slice(models, func(i, j int) bool {
			return models[i].count > models[j].count
		})

		// Show top 3 models
		for i, model := range models {
			if i >= 3 {
				break
			}
			percentage := float64(model.count) / float64(totalCPUs) * 100
			key := fmt.Sprintf("CPU Model %d", i+1)
			// Truncate long model names
			name := model.name
			if len(name) > 40 {
				name = name[:37] + "..."
			}
			addEntry(key, fmt.Sprintf("%s (%d CPUs, %.1f%%)", name, model.count, percentage))
		}

		if len(models) > 3 {
			addEntry("Other Models", fmt.Sprintf("%d additional models", len(models)-3))
		}
	}

	// Governor Distribution
	if len(node.cpu.GovernorFreq) > 0 {
		totalCPUs := 0
		for _, count := range node.cpu.GovernorFreq {
			totalCPUs += count
		}

		// Sort governors by count (descending)
		type govStat struct {
			name  string
			count int
		}
		var governors []govStat
		for name, count := range node.cpu.GovernorFreq {
			governors = append(governors, govStat{name, count})
		}
		sort.Slice(governors, func(i, j int) bool {
			return governors[i].count > governors[j].count
		})

		// Show all governors since there are usually only a few
		for i, gov := range governors {
			percentage := float64(gov.count) / float64(totalCPUs) * 100
			key := fmt.Sprintf("Governor %s", gov.name)
			addEntry(key, fmt.Sprintf("%d CPUs (%.1f%%)", gov.count, percentage))

			if i >= 3 { // Limit to avoid clutter
				break
			}
		}
	}

	// Power Draw
	if node.cpu.PowerNodes > 0 {
		avgWatts := node.cpu.TotalWatts / float64(node.cpu.PowerNodes)
		addEntry("POWER DRAW", fmt.Sprintf("%d of %d nodes reporting",
			node.cpu.PowerNodes, node.cpu.Nodes))
		addEntry("Fleet Total", fmt.Sprintf("%.0f W", node.cpu.TotalWatts))
		addEntry("Per Node", fmt.Sprintf("%.1f W avg, %.0f W min, %.0f W max",
			avgWatts, node.cpu.MinNodeWatts, node.cpu.MaxNodeWatts))

		if len(node.cpu.PowerSourceCounts) > 0 {
			sources := make([]string, 0, len(node.cpu.PowerSourceCounts))
			for src := range node.cpu.PowerSourceCounts {
				sources = append(sources, src)
			}
			sort.Strings(sources)
			for _, src := range sources {
				addEntry(fmt.Sprintf("Source %s", src), fmt.Sprintf("%d nodes", node.cpu.PowerSourceCounts[src]))
			}
		}
	}

	// Convert ordered entries to map with numbered prefixes to preserve order
	data := make(map[string]string)
	for i, entry := range entries {
		key := fmt.Sprintf("%02d:%s", i, entry.key)
		data[key] = entry.value
	}
	return data
}

func (node *CPUMetricsNavigator) GetMetricType() madmin.MetricType {
	return madmin.MetricsCPU
}

func (node *CPUMetricsNavigator) GetMetricFlags() madmin.MetricFlags {
	return 0
}

func (node *CPUMetricsNavigator) GetParent() MetricNode {
	return node.parent
}

func (node *CPUMetricsNavigator) GetPath() string {
	return node.path
}

// CPUSegmentedNode shows time-segmented CPU usage data.
type CPUSegmentedNode struct {
	segmented *madmin.SegmentedCPUMetrics
	parent    MetricNode
	path      string
	flags     madmin.MetricFlags
}

func (node *CPUSegmentedNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *CPUSegmentedNode) GetPath() string                    { return node.path }
func (node *CPUSegmentedNode) GetParent() MetricNode              { return node.parent }
func (node *CPUSegmentedNode) GetMetricType() madmin.MetricType   { return madmin.MetricsCPU }
func (node *CPUSegmentedNode) GetMetricFlags() madmin.MetricFlags { return node.flags }
func (node *CPUSegmentedNode) ShouldPauseRefresh() bool           { return true }
func (node *CPUSegmentedNode) GetChildren() []MetricChild         { return nil }

func (node *CPUSegmentedNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children")
}

func (node *CPUSegmentedNode) GetLeafData() map[string]string {
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

		n := float64(seg.N)
		user := seg.User / n
		system := seg.System / n
		idle := seg.Idle / n
		iowait := seg.Iowait / n
		total := user + system + idle + seg.Nice/n + iowait +
			seg.Irq/n + seg.Softirq/n + seg.Steal/n +
			seg.Guest/n + seg.GuestNice/n
		if total == 0 {
			continue
		}
		data[name] = fmt.Sprintf("User: %.1f%%, Sys: %.1f%%, Idle: %.1f%%, IOWait: %.1f%% (%d nodes)",
			user/total*100, system/total*100, idle/total*100, iowait/total*100, seg.N)
	}
	return data
}

// PowerSegmentedNode shows time-segmented power draw data.
type PowerSegmentedNode struct {
	segmented *madmin.SegmentedPowerMetrics
	parent    MetricNode
	path      string
	flags     madmin.MetricFlags
}

func (node *PowerSegmentedNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *PowerSegmentedNode) GetPath() string                    { return node.path }
func (node *PowerSegmentedNode) GetParent() MetricNode              { return node.parent }
func (node *PowerSegmentedNode) GetMetricType() madmin.MetricType   { return madmin.MetricsCPU }
func (node *PowerSegmentedNode) GetMetricFlags() madmin.MetricFlags { return node.flags }
func (node *PowerSegmentedNode) ShouldPauseRefresh() bool           { return true }
func (node *PowerSegmentedNode) GetChildren() []MetricChild         { return nil }

func (node *PowerSegmentedNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children")
}

func (node *PowerSegmentedNode) GetLeafData() map[string]string {
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
		avg := seg.SumWatts / float64(seg.N)
		data[name] = fmt.Sprintf("Avg: %s, Min: %s, Max: %s (%s nodes)",
			humanize.SI(avg, "W"),
			humanize.SI(seg.MinWatts, "W"),
			humanize.SI(seg.MaxWatts, "W"),
			humanize.Comma(int64(seg.N)))
	}
	return data
}

func (node *CPUMetricsNavigator) GetChild(name string) (MetricNode, error) {
	if node.cpu == nil {
		return nil, fmt.Errorf("no CPU data available")
	}
	switch name {
	case "last_hour":
		return &CPUSegmentedNode{
			segmented: node.cpu.LastHour,
			parent:    node,
			path:      node.path + "/last_hour",
			flags:     madmin.MetricsHourStats,
		}, nil
	case "last_day":
		return &CPUSegmentedNode{
			segmented: node.cpu.LastDay,
			parent:    node,
			path:      node.path + "/last_day",
			flags:     madmin.MetricsDayStats,
		}, nil
	case "power_last_hour":
		return &PowerSegmentedNode{
			segmented: node.cpu.PowerLastHour,
			parent:    node,
			path:      node.path + "/power_last_hour",
			flags:     madmin.MetricsHourStats,
		}, nil
	case "power_last_day":
		return &PowerSegmentedNode{
			segmented: node.cpu.PowerLastDay,
			parent:    node,
			path:      node.path + "/power_last_day",
			flags:     madmin.MetricsDayStats,
		}, nil
	default:
		return nil, fmt.Errorf("child not found: %s", name)
	}
}
