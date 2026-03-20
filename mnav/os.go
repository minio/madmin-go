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
	"time"

	"github.com/dustin/go-humanize"
	"github.com/minio/madmin-go/v4"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// formatNumber formats large numbers with thousand separators
func formatNumber(n uint64) string {
	return humanize.Comma(int64(n))
}

// formatBytes formats bytes in human readable format
func formatBytes(bytes uint64) string {
	return humanize.Bytes(bytes)
}

func formatOpsPerSec(ops float64) string {
	if ops >= 1000000 {
		return fmt.Sprintf("%.1fM", ops/1000000)
	}
	if ops >= 1000 {
		return fmt.Sprintf("%.1fK", ops/1000)
	}
	return fmt.Sprintf("%.1f", ops)
}

// getLongestSegmented returns the SegmentedActions with the most segments.
func getLongestSegmented(lastDay map[string]madmin.SegmentedActions) *madmin.SegmentedActions {
	var best *madmin.SegmentedActions
	for _, seg := range lastDay {
		s := seg
		if best == nil || len(s.Segments) > len(best.Segments) {
			best = &s
		}
	}
	return best
}

// OSMetricsNavigator provides navigation for OS metrics
type OSMetricsNavigator struct {
	os     *madmin.OSMetrics
	parent MetricNode
	path   string
}

func (node *OSMetricsNavigator) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

// NewOSMetricsNavigator creates a new OS metrics navigator
func NewOSMetricsNavigator(os *madmin.OSMetrics, parent MetricNode, path string) *OSMetricsNavigator {
	return &OSMetricsNavigator{os: os, parent: parent, path: path}
}

func (node *OSMetricsNavigator) GetChildren() []MetricChild {
	if node.os == nil {
		return []MetricChild{}
	}

	m := []MetricChild{
		{Name: "lifetime_ops", Description: "Accumulated operations since server start"},
		{Name: "last_minute", Description: "Last minute operation statistics"},
		{Name: "last_day", Description: "Last 24h operation statistics"},
	}
	if len(node.os.Sensors) > 0 {
		m = append(m, MetricChild{Name: "sensors", Description: "Temperature sensor metrics"})
	}
	return m
}

func (node *OSMetricsNavigator) GetLeafData() map[string]string {
	if node.os == nil {
		return map[string]string{"Status": "OS metrics not available"}
	}

	data := map[string]string{
		"Collected At":           node.os.CollectedAt.Format("2006-01-02 15:04:05"),
		"Operation Types":        strconv.Itoa(len(node.os.LifeTimeOps)),
		"Last Minute Operations": strconv.Itoa(len(node.os.LastMinute.Operations)),
		"Temperature Sensors":    strconv.Itoa(len(node.os.Sensors)),
	}

	// Add totals for lifetime ops
	var totalLifetimeOps uint64
	for _, count := range node.os.LifeTimeOps {
		totalLifetimeOps += count
	}
	data["Total Lifetime Operations"] = formatNumber(totalLifetimeOps)

	return data
}

func (node *OSMetricsNavigator) GetMetricType() madmin.MetricType {
	return madmin.MetricsOS
}

func (node *OSMetricsNavigator) GetMetricFlags() madmin.MetricFlags {
	return 0
}

func (node *OSMetricsNavigator) GetParent() MetricNode {
	return node.parent
}

func (node *OSMetricsNavigator) GetPath() string {
	return node.path
}

func (node *OSMetricsNavigator) ShouldPauseRefresh() bool {
	return false
}

func (node *OSMetricsNavigator) GetChild(name string) (MetricNode, error) {
	if node.os == nil {
		return nil, fmt.Errorf("os metrics not available")
	}
	switch name {
	case "lifetime_ops":
		return NewOSLifetimeOpsNode(node.os.LifeTimeOps, node, fmt.Sprintf("%s/lifetime_ops", node.path)), nil
	case "last_minute":
		return NewOSLastMinuteNode(node.os.LastMinute.Operations, node, fmt.Sprintf("%s/last_minute", node.path)), nil
	case "last_day":
		return NewOSLastDayNode(node.os.LastDay, node, fmt.Sprintf("%s/last_day", node.path)), nil
	case "sensors":
		return NewOSSensorsNode(node.os.Sensors, node, fmt.Sprintf("%s/sensors", node.path)), nil
	default:
		return nil, fmt.Errorf("child not found: %s", name)
	}
}

type OSMetricsNode struct {
	parent MetricNode
	path   string
}

func (node *OSMetricsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *OSMetricsNode) ShouldPauseUpdates() bool {
	// Legacy method - not used in interface, return false for default behavior
	return false
}

func (node *OSMetricsNode) GetChildren() []MetricChild {
	return []MetricChild{
		{Name: "lifetime_ops", Description: "Accumulated operations since server start"},
		{Name: "last_minute", Description: "Last minute operation statistics"},
		{Name: "sensors", Description: "Temperature sensor metrics"},
	}
}
func (node *OSMetricsNode) GetLeafData() map[string]string     { return nil }
func (node *OSMetricsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsOS }
func (node *OSMetricsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *OSMetricsNode) GetParent() MetricNode              { return node.parent }
func (node *OSMetricsNode) GetPath() string                    { return node.path }

func (node *OSMetricsNode) ShouldPauseRefresh() bool {
	return false
}

func (node *OSMetricsNode) GetChild(name string) (MetricNode, error) {
	return nil, fmt.Errorf("os metric sub-navigation not yet implemented for: %s", name)
}

// OSLifetimeOpsNode handles navigation for OS lifetime operations
type OSLifetimeOpsNode struct {
	ops    map[string]uint64
	parent MetricNode
	path   string
}

func (node *OSLifetimeOpsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewOSLifetimeOpsNode(ops map[string]uint64, parent MetricNode, path string) *OSLifetimeOpsNode {
	return &OSLifetimeOpsNode{ops: ops, parent: parent, path: path}
}

func (node *OSLifetimeOpsNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *OSLifetimeOpsNode) GetLeafData() map[string]string {
	if node.ops == nil {
		return map[string]string{"Operation Types": "0", "Total Operations": "0"}
	}

	data := map[string]string{
		"Operation Types": strconv.Itoa(len(node.ops)),
	}
	var total uint64
	for opType, count := range node.ops {
		// Convert technical operation names to display names
		titleCaser := cases.Title(language.Und)
		displayName := strings.ReplaceAll(titleCaser.String(strings.ReplaceAll(opType, "_", " ")), "Api", "API")
		data[displayName] = formatNumber(count)
		total += count
	}
	data["Total Operations"] = formatNumber(total)
	return data
}

func (node *OSLifetimeOpsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsOS }
func (node *OSLifetimeOpsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *OSLifetimeOpsNode) GetParent() MetricNode              { return node.parent }
func (node *OSLifetimeOpsNode) GetPath() string                    { return node.path }

func (node *OSLifetimeOpsNode) ShouldPauseRefresh() bool {
	return false
}

func (node *OSLifetimeOpsNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("lifetime operations node has no children")
}

// OSLastMinuteNode handles navigation for OS last minute operations
type OSLastMinuteNode struct {
	operations map[string]madmin.TimedAction
	parent     MetricNode
	path       string
}

func (node *OSLastMinuteNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewOSLastMinuteNode(operations map[string]madmin.TimedAction, parent MetricNode, path string) *OSLastMinuteNode {
	return &OSLastMinuteNode{operations: operations, parent: parent, path: path}
}

func (node *OSLastMinuteNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *OSLastMinuteNode) GetLeafData() map[string]string {
	if node.operations == nil {
		return map[string]string{"Operation Types": "0", "Total Operations": "0", "Total Time": "0 ms"}
	}

	data := map[string]string{
		"Operation Types": strconv.Itoa(len(node.operations)),
	}
	var totalCount, totalTime uint64
	for opType, action := range node.operations {
		titleCaser := cases.Title(language.Und)
		displayName := strings.ReplaceAll(titleCaser.String(strings.ReplaceAll(opType, "_", " ")), "Api", "API")

		// Enhanced single-line format with comprehensive timing stats
		var statsParts []string

		// Add operation count
		statsParts = append(statsParts, fmt.Sprintf("%s ops", formatNumber(action.Count)))

		if action.Count > 0 {
			// Calculate average time
			avgTimeMs := float64(action.AccTime) / float64(action.Count) / 1000000 // ns to ms
			statsParts = append(statsParts, fmt.Sprintf("avg: %.2fms", avgTimeMs))

			// Add min/max times
			minTimeMs := float64(action.MinTime) / 1000000
			maxTimeMs := float64(action.MaxTime) / 1000000
			statsParts = append(statsParts, fmt.Sprintf("min: %.2fms", minTimeMs))
			statsParts = append(statsParts, fmt.Sprintf("max: %.2fms", maxTimeMs))

			// Add average bytes if available
			if action.Bytes > 0 {
				avgBytes := action.Bytes / action.Count
				statsParts = append(statsParts, fmt.Sprintf("%s avg", formatBytes(avgBytes)))
			}
		}

		// Combine into single comprehensive line
		data[displayName+" Ops"] = strings.Join(statsParts, ", ")

		totalCount += action.Count
		totalTime += action.AccTime
	}
	data["Total Operations"] = formatNumber(totalCount)
	data["Total Time"] = fmt.Sprintf("%.2f ms", float64(totalTime)/1000000)
	return data
}

func (node *OSLastMinuteNode) GetMetricType() madmin.MetricType   { return madmin.MetricsOS }
func (node *OSLastMinuteNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *OSLastMinuteNode) GetParent() MetricNode              { return node.parent }
func (node *OSLastMinuteNode) GetPath() string                    { return node.path }

func (node *OSLastMinuteNode) ShouldPauseRefresh() bool {
	return false
}

func (node *OSLastMinuteNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("last minute operations node has no children")
}

// OSSensorsNode handles navigation for OS temperature sensors
type OSSensorsNode struct {
	sensors map[string]madmin.SensorMetrics
	parent  MetricNode
	path    string
}

func (node *OSSensorsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewOSSensorsNode(sensors map[string]madmin.SensorMetrics, parent MetricNode, path string) *OSSensorsNode {
	return &OSSensorsNode{sensors: sensors, parent: parent, path: path}
}

func (node *OSSensorsNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *OSSensorsNode) GetLeafData() map[string]string {
	if node.sensors == nil {
		return map[string]string{"Temperature Sensors": "0"}
	}

	data := map[string]string{
		"Temperature Sensors": strconv.Itoa(len(node.sensors)),
	}

	// Calculate aggregate sensor statistics
	var totalReadings, totalExceedsCritical int
	var minTempOverall, maxTempOverall, totalTempOverall float64
	first := true

	for sensorKey, sensor := range node.sensors {
		titleCaser := cases.Title(language.Und)
		displayKey := titleCaser.String(strings.ReplaceAll(sensorKey, "_", " "))

		// Enhanced single-line format with comprehensive sensor stats
		statsParts := make([]string, 0, 2)

		// Calculate average temperature
		avgTemp := float64(0)
		if sensor.Count > 0 {
			avgTemp = sensor.TotalTemp / float64(sensor.Count)
		}

		// Add current/average temperature first
		statsParts = append(statsParts, fmt.Sprintf("%.1f°C", avgTemp))

		// Add detailed breakdown in parentheses
		tempDetails := fmt.Sprintf("avg: %.1f°C, min: %.1f°C, max: %.1f°C, %s readings",
			avgTemp, sensor.MinTemp, sensor.MaxTemp, formatNumber(uint64(sensor.Count)))

		if sensor.ExceedsCritical > 0 {
			tempDetails += fmt.Sprintf(", %s critical", formatNumber(uint64(sensor.ExceedsCritical)))
		}

		statsParts = append(statsParts, fmt.Sprintf("(%s)", tempDetails))

		// Combine into single comprehensive line
		data[displayKey+" Sensor"] = strings.Join(statsParts, " ")

		// Update totals for overall statistics
		totalReadings += sensor.Count
		totalExceedsCritical += sensor.ExceedsCritical
		totalTempOverall += sensor.TotalTemp

		if first {
			minTempOverall = sensor.MinTemp
			maxTempOverall = sensor.MaxTemp
			first = false
		} else {
			if sensor.MinTemp < minTempOverall {
				minTempOverall = sensor.MinTemp
			}
			if sensor.MaxTemp > maxTempOverall {
				maxTempOverall = sensor.MaxTemp
			}
		}
	}

	// Add overall summary
	if totalReadings > 0 {
		data["Total Readings"] = formatNumber(uint64(totalReadings))
		data["Overall Temp Range"] = fmt.Sprintf("%.1f°C to %.1f°C (avg: %.1f°C)",
			minTempOverall, maxTempOverall, totalTempOverall/float64(totalReadings))
		if totalExceedsCritical > 0 {
			data["Total Critical Events"] = formatNumber(uint64(totalExceedsCritical))
		}
	}

	return data
}

func (node *OSSensorsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsOS }
func (node *OSSensorsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *OSSensorsNode) GetParent() MetricNode              { return node.parent }
func (node *OSSensorsNode) GetPath() string                    { return node.path }

func (node *OSSensorsNode) ShouldPauseRefresh() bool {
	return false
}

func (node *OSSensorsNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("sensors node has no children")
}

// OSLastDayNode handles navigation for OS last day statistics
type OSLastDayNode struct {
	lastDay map[string]madmin.SegmentedActions
	parent  MetricNode
	path    string
}

func (node *OSLastDayNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewOSLastDayNode(lastDay map[string]madmin.SegmentedActions, parent MetricNode, path string) *OSLastDayNode {
	return &OSLastDayNode{lastDay: lastDay, parent: parent, path: path}
}

func (node *OSLastDayNode) GetChildren() []MetricChild {
	children := []MetricChild{{Name: "Total", Description: "Aggregated totals across all time segments"}}

	if len(node.lastDay) == 0 {
		return children
	}

	refSeg := getLongestSegmented(node.lastDay)
	if refSeg == nil || len(refSeg.Segments) == 0 {
		return children
	}

	// Iterate segments in reverse (most recent first)
	for i := len(refSeg.Segments) - 1; i >= 0; i-- {
		segmentTime := refSeg.FirstTime.Add(time.Duration(i*refSeg.Interval) * time.Second)
		endTime := segmentTime.Add(time.Duration(refSeg.Interval) * time.Second)

		// Count ops and active op types in this segment
		var totalOps uint64
		activeTypes := 0
		for _, seg := range node.lastDay {
			if i < len(seg.Segments) && seg.Segments[i].Count > 0 {
				totalOps += seg.Segments[i].Count
				activeTypes++
			}
		}
		if totalOps == 0 {
			continue
		}

		day := "Today"
		if segmentTime.Local().Day() != time.Now().Day() {
			day = "Yesterday"
		}

		opsPerSec := float64(totalOps) / float64(refSeg.Interval)
		name := segmentTime.UTC().Format("15:04Z")
		children = append(children, MetricChild{
			Name:        name,
			Description: fmt.Sprintf("%s, ending %s, %d op types, %s ops/s", day, endTime.Local().Format("15:04"), activeTypes, formatOpsPerSec(opsPerSec)),
		})
	}
	return children
}

func (node *OSLastDayNode) GetLeafData() map[string]string {
	if len(node.lastDay) == 0 {
		return map[string]string{"Status": "No last day statistics available"}
	}

	data := map[string]string{}
	var totalOps uint64
	var totalTime float64

	for opType, segmented := range node.lastDay {
		var opOps uint64
		var opTime float64
		for _, seg := range segmented.Segments {
			opOps += seg.Count
			opTime += float64(seg.AccTime)
		}
		if opOps > 0 {
			avgMs := opTime / float64(opOps) / 1e6
			data[opType] = fmt.Sprintf("%s ops, %.2fms avg", formatNumber(opOps), avgMs)
		}
		totalOps += opOps
		totalTime += opTime
	}

	if totalOps > 0 {
		data["01:Total Operations"] = formatNumber(totalOps)
		data["02:Avg Time"] = fmt.Sprintf("%.2fms", totalTime/float64(totalOps)/1e6)
	}

	return data
}

func (node *OSLastDayNode) GetMetricType() madmin.MetricType   { return madmin.MetricsOS }
func (node *OSLastDayNode) GetMetricFlags() madmin.MetricFlags { return madmin.MetricsDayStats }
func (node *OSLastDayNode) GetParent() MetricNode              { return node.parent }
func (node *OSLastDayNode) GetPath() string                    { return node.path }
func (node *OSLastDayNode) ShouldPauseRefresh() bool           { return true }

func (node *OSLastDayNode) GetChild(name string) (MetricNode, error) {
	if name == "Total" {
		return NewOSLastDayTotalNode(node.lastDay, node, fmt.Sprintf("%s/Total", node.path)), nil
	}

	refSeg := getLongestSegmented(node.lastDay)
	if refSeg == nil {
		return nil, fmt.Errorf("segment not found: %s", name)
	}
	for i := range refSeg.Segments {
		segmentTime := refSeg.FirstTime.Add(time.Duration(i*refSeg.Interval) * time.Second)
		if segmentTime.UTC().Format("15:04Z") == name {
			return NewOSLastDaySegmentNode(node.lastDay, i, segmentTime, node, fmt.Sprintf("%s/%s", node.path, name)), nil
		}
	}
	return nil, fmt.Errorf("segment not found: %s", name)
}

// OSLastDayTotalNode shows aggregated totals across all time segments
type OSLastDayTotalNode struct {
	lastDay map[string]madmin.SegmentedActions
	parent  MetricNode
	path    string
}

func (node *OSLastDayTotalNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewOSLastDayTotalNode(lastDay map[string]madmin.SegmentedActions, parent MetricNode, path string) *OSLastDayTotalNode {
	return &OSLastDayTotalNode{lastDay: lastDay, parent: parent, path: path}
}

func (node *OSLastDayTotalNode) GetChildren() []MetricChild { return []MetricChild{} }

func (node *OSLastDayTotalNode) GetLeafData() map[string]string {
	data := map[string]string{}
	var grandTotalOps, grandTotalBytes uint64
	var grandTotalTime float64

	for opType, segmented := range node.lastDay {
		var opOps, opBytes uint64
		var opTime float64
		for _, seg := range segmented.Segments {
			opOps += seg.Count
			opBytes += seg.Bytes
			opTime += float64(seg.AccTime)
		}
		if opOps > 0 {
			avgMs := opTime / float64(opOps) / 1e6
			if opBytes > 0 {
				data[opType] = fmt.Sprintf("%s ops, %.2fms avg, %s", formatNumber(opOps), avgMs, formatBytes(opBytes))
			} else {
				data[opType] = fmt.Sprintf("%s ops, %.2fms avg", formatNumber(opOps), avgMs)
			}
		}
		grandTotalOps += opOps
		grandTotalBytes += opBytes
		grandTotalTime += opTime
	}

	if grandTotalOps > 0 {
		data["00:Total"] = fmt.Sprintf("%s operations in last 24h", formatNumber(grandTotalOps))
		if grandTotalBytes > 0 {
			data["01:Bytes"] = formatBytes(grandTotalBytes)
		}
		data["02:Time"] = fmt.Sprintf("%.2f ms cumulative", grandTotalTime/1e6)
	}

	return data
}

func (node *OSLastDayTotalNode) GetMetricType() madmin.MetricType   { return madmin.MetricsOS }
func (node *OSLastDayTotalNode) GetMetricFlags() madmin.MetricFlags { return madmin.MetricsDayStats }
func (node *OSLastDayTotalNode) GetParent() MetricNode              { return node.parent }
func (node *OSLastDayTotalNode) GetPath() string                    { return node.path }
func (node *OSLastDayTotalNode) ShouldPauseRefresh() bool           { return true }

func (node *OSLastDayTotalNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("total node has no children")
}

// OSLastDaySegmentNode shows data for a single time segment
type OSLastDaySegmentNode struct {
	lastDay     map[string]madmin.SegmentedActions
	segmentIdx  int
	segmentTime time.Time
	parent      MetricNode
	path        string
}

func (node *OSLastDaySegmentNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewOSLastDaySegmentNode(lastDay map[string]madmin.SegmentedActions, segmentIdx int, segmentTime time.Time, parent MetricNode, path string) *OSLastDaySegmentNode {
	return &OSLastDaySegmentNode{lastDay: lastDay, segmentIdx: segmentIdx, segmentTime: segmentTime, parent: parent, path: path}
}

func (node *OSLastDaySegmentNode) GetChildren() []MetricChild { return []MetricChild{} }

func (node *OSLastDaySegmentNode) GetLeafData() map[string]string {
	data := map[string]string{
		"00:Local Time": node.segmentTime.Local().Format("2006-01-02 15:04:05"),
	}

	refSeg := getLongestSegmented(node.lastDay)
	var interval int
	if refSeg != nil {
		interval = refSeg.Interval
	}

	var totalOps, totalBytes uint64
	var totalTime float64

	for opType, segmented := range node.lastDay {
		if node.segmentIdx >= len(segmented.Segments) {
			continue
		}
		seg := segmented.Segments[node.segmentIdx]
		if seg.Count > 0 {
			avgMs := float64(seg.AccTime) / float64(seg.Count) / 1e6
			minMs := float64(seg.MinTime) / 1e6
			maxMs := float64(seg.MaxTime) / 1e6
			if seg.Bytes > 0 {
				data[opType] = fmt.Sprintf("%s ops, %.2f/%.2f/%.2fms (min/avg/max), %s", formatNumber(seg.Count), minMs, avgMs, maxMs, formatBytes(seg.Bytes))
			} else {
				data[opType] = fmt.Sprintf("%s ops, %.2f/%.2f/%.2fms (min/avg/max)", formatNumber(seg.Count), minMs, avgMs, maxMs)
			}
			totalOps += seg.Count
			totalBytes += seg.Bytes
			totalTime += float64(seg.AccTime)
		}
	}

	if totalOps > 0 {
		data["01:Total Ops"] = formatNumber(totalOps)
		if interval > 0 {
			data["02:Ops/s"] = formatOpsPerSec(float64(totalOps) / float64(interval))
		}
		data["03:Avg Op Time"] = fmt.Sprintf("%.2fms", totalTime/float64(totalOps)/1e6)
		if totalBytes > 0 {
			data["04:Total Bytes"] = formatBytes(totalBytes)
		}
	}

	return data
}

func (node *OSLastDaySegmentNode) GetMetricType() madmin.MetricType   { return madmin.MetricsOS }
func (node *OSLastDaySegmentNode) GetMetricFlags() madmin.MetricFlags { return madmin.MetricsDayStats }
func (node *OSLastDaySegmentNode) GetParent() MetricNode              { return node.parent }
func (node *OSLastDaySegmentNode) GetPath() string                    { return node.path }
func (node *OSLastDaySegmentNode) ShouldPauseRefresh() bool           { return true }

func (node *OSLastDaySegmentNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("segment node has no children")
}
