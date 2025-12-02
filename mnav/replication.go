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

// formatReplicationLatency formats latency values with appropriate units
func formatReplicationLatency(latencySecs float64) string {
	if latencySecs == 0 {
		return "0ms"
	}
	if latencySecs < 0.001 {
		return fmt.Sprintf("%.2fμs", latencySecs*1000000)
	}
	if latencySecs < 1 {
		return fmt.Sprintf("%.2fms", latencySecs*1000)
	}
	return fmt.Sprintf("%.2fs", latencySecs)
}

// formatReplicationThroughput calculates and formats throughput rates
func formatReplicationThroughput(bytes int64, timeSecs float64, label string) string {
	if timeSecs <= 0 || bytes <= 0 {
		return fmt.Sprintf("%s: 0B/s", label)
	}
	bytesPerSec := float64(bytes) / timeSecs
	return fmt.Sprintf("%s: %s/s", label, humanize.Bytes(uint64(bytesPerSec)))
}

// generateReplicationStatsDisplay formats ReplicationStats for display
func generateReplicationStatsDisplay(stats madmin.ReplicationStats, includeTimeInfo bool) map[string]string {
	data := make(map[string]string)

	if stats.Nodes == 0 {
		data["Status"] = "No replication data available"
		return data
	}

	// Time information
	if includeTimeInfo && stats.StartTime != nil && stats.EndTime != nil {
		data["Time Range"] = fmt.Sprintf("%s → %s",
			stats.StartTime.Local().Format("15:04:05"),
			stats.EndTime.Local().Format("15:04:05"))
		if stats.WallTimeSecs > 0 {
			data["Duration"] = fmt.Sprintf("%.1f seconds", stats.WallTimeSecs)
		}
	}

	// Activity Summary
	data["Nodes Reporting"] = humanize.Comma(int64(stats.Nodes))
	if stats.Events > 0 {
		data["Total Events"] = humanize.Comma(stats.Events)
		data["Data Transferred"] = humanize.Bytes(uint64(stats.Bytes))

		// Calculate rates
		if stats.EventTimeSecs > 0 {
			eventsPerSec := float64(stats.Events) / stats.EventTimeSecs
			data["Event Rate"] = fmt.Sprintf("%.1f events/s", eventsPerSec)
		}
		if stats.WallTimeSecs > 0 {
			data[formatReplicationThroughput(stats.Bytes, stats.WallTimeSecs, "Throughput")] = ""
		}
	}

	// Latency metrics
	if stats.Events > 0 && stats.LatencySecs > 0 {
		avgLatency := stats.LatencySecs / float64(stats.Events)
		data["Average Latency"] = formatReplicationLatency(avgLatency)
		if stats.MaxLatencySecs > 0 {
			data["Maximum Latency"] = formatReplicationLatency(stats.MaxLatencySecs)
		}
	}

	// Operation breakdown
	totalOps := stats.PutObject + stats.UpdateMeta + stats.DelObject + stats.DelTag
	if totalOps > 0 {
		data["PUT Operations"] = fmt.Sprintf("%s (%.1f%%)",
			humanize.Comma(stats.PutObject),
			float64(stats.PutObject)/float64(totalOps)*100)
		if stats.UpdateMeta > 0 {
			data["Update Metadata"] = fmt.Sprintf("%s (%.1f%%)",
				humanize.Comma(stats.UpdateMeta),
				float64(stats.UpdateMeta)/float64(totalOps)*100)
		}
		if stats.DelObject > 0 {
			data["DELETE Operations"] = fmt.Sprintf("%s (%.1f%%)",
				humanize.Comma(stats.DelObject),
				float64(stats.DelObject)/float64(totalOps)*100)
		}
		if stats.DelTag > 0 {
			data["DELETE Tag Operations"] = fmt.Sprintf("%s (%.1f%%)",
				humanize.Comma(stats.DelTag),
				float64(stats.DelTag)/float64(totalOps)*100)
		}
	}

	// Error analysis
	totalErrors := stats.PutErrors + stats.UpdateMetaErrors + stats.DelErrors + stats.DelTagErrors
	if totalErrors > 0 {
		errorRate := float64(totalErrors) / float64(stats.Events) * 100
		data["Error Rate"] = fmt.Sprintf("%.2f%% (%s errors)", errorRate, humanize.Comma(totalErrors))

		if stats.PutErrors > 0 {
			data["PUT Errors"] = humanize.Comma(stats.PutErrors)
		}
		if stats.UpdateMetaErrors > 0 {
			data["Metadata Errors"] = humanize.Comma(stats.UpdateMetaErrors)
		}
		if stats.DelErrors > 0 {
			data["DELETE Errors"] = humanize.Comma(stats.DelErrors)
		}
		if stats.DelTagErrors > 0 {
			data["DELETE Tag Errors"] = humanize.Comma(stats.DelTagErrors)
		}
	} else if stats.Events > 0 {
		data["Error Rate"] = "0.00%"
	}

	// Outcome analysis
	totalOutcomes := stats.Synced + stats.AlreadyOK + stats.Rejected
	if totalOutcomes > 0 {
		data["Synced"] = fmt.Sprintf("%s (%.1f%%)",
			humanize.Comma(stats.Synced),
			float64(stats.Synced)/float64(totalOutcomes)*100)
		if stats.AlreadyOK > 0 {
			data["Already Synchronized"] = fmt.Sprintf("%s (%.1f%%)",
				humanize.Comma(stats.AlreadyOK),
				float64(stats.AlreadyOK)/float64(totalOutcomes)*100)
		}
		if stats.Rejected > 0 {
			data["Rejected"] = fmt.Sprintf("%s (%.1f%%)",
				humanize.Comma(stats.Rejected),
				float64(stats.Rejected)/float64(totalOutcomes)*100)
		}
	}

	// Proxy operations
	totalProxy := stats.ProxyEvents + stats.ProxyHead + stats.ProxyGet + stats.ProxyGetTag
	if totalProxy > 0 {
		data["Total Proxy Operations"] = humanize.Comma(totalProxy)
		if stats.ProxyBytes > 0 {
			data["Proxy Data Transfer"] = humanize.Bytes(uint64(stats.ProxyBytes))
		}

		// Proxy success rates
		if stats.ProxyHead > 0 {
			successRate := float64(stats.ProxyHeadOK) / float64(stats.ProxyHead) * 100
			data["HEAD Proxy Success"] = fmt.Sprintf("%.1f%% (%s/%s)",
				successRate, humanize.Comma(stats.ProxyHeadOK), humanize.Comma(stats.ProxyHead))
		}
		if stats.ProxyGet > 0 {
			successRate := float64(stats.ProxyGetOK) / float64(stats.ProxyGet) * 100
			data["GET Proxy Success"] = fmt.Sprintf("%.1f%% (%s/%s)",
				successRate, humanize.Comma(stats.ProxyGetOK), humanize.Comma(stats.ProxyGet))
		}
		if stats.ProxyGetTag > 0 {
			successRate := float64(stats.ProxyGetTagOK) / float64(stats.ProxyGetTag) * 100
			data["GET Tag Proxy Success"] = fmt.Sprintf("%.1f%% (%s/%s)",
				successRate, humanize.Comma(stats.ProxyGetTagOK), humanize.Comma(stats.ProxyGetTag))
		}
	}

	return data
}

// ReplicationMetricsNode is the root node for replication metrics
type ReplicationMetricsNode struct {
	replication *madmin.ReplicationMetrics
	parent      MetricNode
	path        string
}

func (node *ReplicationMetricsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewReplicationMetricsNode(replication *madmin.ReplicationMetrics, parent MetricNode, path string) *ReplicationMetricsNode {
	return &ReplicationMetricsNode{replication: replication, parent: parent, path: path}
}

func (node *ReplicationMetricsNode) ShouldPauseRefresh() bool {
	return false
}

func (node *ReplicationMetricsNode) GetChildren() []MetricChild {
	if node.replication == nil {
		return []MetricChild{}
	}

	var children []MetricChild

	// Add last hour summary first
	children = append(children, MetricChild{
		Name:        "last_hour",
		Description: "Aggregated last hour replication totals across all targets",
	})
	// Add last day summary
	children = append(children, MetricChild{
		Name:        "last_day",
		Description: "Aggregated last day replication totals across all targets",
	})

	// Add individual targets
	if len(node.replication.Targets) > 0 {
		var targets []string
		for targetName := range node.replication.Targets {
			targets = append(targets, targetName)
		}
		sort.Strings(targets)

		for _, target := range targets {
			children = append(children, MetricChild{
				Name:        target,
				Description: fmt.Sprintf("Replication statistics for target %s", target),
			})
		}
	}

	return children
}

func (node *ReplicationMetricsNode) GetLeafData() map[string]string {
	if node.replication == nil {
		return map[string]string{"Status": "No replication metrics available"}
	}

	data := make(map[string]string)

	// Overall cluster information
	data["Nodes Reporting"] = humanize.Comma(int64(node.replication.Nodes))
	data["Collection Time"] = node.replication.CollectedAt.Local().Format("15:04:05 MST")

	// Current activity
	if node.replication.Active > 0 {
		data["Active Replications"] = humanize.Comma(node.replication.Active)
	}
	if node.replication.Queued > 0 {
		data["Queued Replications"] = humanize.Comma(node.replication.Queued)
	}

	// Target summary
	targetCount := len(node.replication.Targets)
	if targetCount > 0 {
		data["Configured Targets"] = fmt.Sprintf("%d", targetCount)

		// Show target names
		var targets []string
		for targetName := range node.replication.Targets {
			targets = append(targets, targetName)
		}
		sort.Strings(targets)

		if len(targets) <= 5 {
			data["Target Names"] = strings.Join(targets, ", ")
		} else {
			data["Target Names"] = strings.Join(targets[:5], ", ") + fmt.Sprintf(" (and %d more)", len(targets)-5)
		}

		// Aggregate last hour stats across all targets
		allTargets := node.replication.AllTargets()
		if allTargets.LastHour.Events > 0 {
			data["Last Hour Events"] = humanize.Comma(allTargets.LastHour.Events)
			data["Last Hour Data"] = humanize.Bytes(uint64(allTargets.LastHour.Bytes))
			if allTargets.LastHour.EventTimeSecs > 0 {
				rate := float64(allTargets.LastHour.Events) / allTargets.LastHour.EventTimeSecs
				data["Last Hour Rate"] = fmt.Sprintf("%.1f events/s", rate)
			}
		}
	} else {
		data["Status"] = "No replication targets configured"
	}

	return data
}

func (node *ReplicationMetricsNode) GetChild(name string) (MetricNode, error) {
	if node.replication == nil {
		return nil, fmt.Errorf("no replication data available")
	}

	// Handle last hour summary
	if name == "last_hour" {
		return NewReplicationLastHourNode(node.replication, node, node.path+"/"+name), nil
	}
	// Handle last day summary
	if name == "last_day" {
		return NewReplicationLastDayAggregatedNode(node.replication, node, node.path+"/"+name), nil
	}

	// Handle individual targets
	if targetStats, exists := node.replication.Targets[name]; exists {
		return NewReplicationTargetNode(name, &targetStats, node, node.path+"/"+name), nil
	}

	return nil, fmt.Errorf("replication target or section not found: %s", name)
}

func (node *ReplicationMetricsNode) GetMetricType() madmin.MetricType {
	return madmin.MetricsReplication
}
func (node *ReplicationMetricsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *ReplicationMetricsNode) GetParent() MetricNode              { return node.parent }
func (node *ReplicationMetricsNode) GetPath() string                    { return node.path }

// ReplicationLastHourNode shows aggregated last hour statistics
type ReplicationLastHourNode struct {
	replication *madmin.ReplicationMetrics
	parent      MetricNode
	path        string
}

func (node *ReplicationLastHourNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewReplicationLastHourNode(replication *madmin.ReplicationMetrics, parent MetricNode, path string) *ReplicationLastHourNode {
	return &ReplicationLastHourNode{replication: replication, parent: parent, path: path}
}

func (node *ReplicationLastHourNode) ShouldPauseRefresh() bool {
	return false
}

func (node *ReplicationLastHourNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *ReplicationLastHourNode) GetLeafData() map[string]string {
	if node.replication == nil {
		return map[string]string{"Status": "No replication data available"}
	}

	// Aggregate last hour stats across all targets
	allTargets := node.replication.AllTargets()

	data := generateReplicationStatsDisplay(allTargets.LastHour, true)

	// Add summary header
	targetCount := len(node.replication.Targets)
	if targetCount > 0 {
		data["Aggregation Source"] = fmt.Sprintf("Combined statistics from %d replication targets", targetCount)
	}

	return data
}

func (node *ReplicationLastHourNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("last hour node has no children")
}

func (node *ReplicationLastHourNode) GetMetricType() madmin.MetricType {
	return madmin.MetricsReplication
}
func (node *ReplicationLastHourNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *ReplicationLastHourNode) GetParent() MetricNode              { return node.parent }
func (node *ReplicationLastHourNode) GetPath() string                    { return node.path }

// ReplicationTargetNode handles navigation for individual replication targets
type ReplicationTargetNode struct {
	targetName string
	target     *madmin.ReplicationTargetStats
	parent     MetricNode
	path       string
}

func (node *ReplicationTargetNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewReplicationTargetNode(targetName string, target *madmin.ReplicationTargetStats, parent MetricNode, path string) *ReplicationTargetNode {
	return &ReplicationTargetNode{targetName: targetName, target: target, parent: parent, path: path}
}

func (node *ReplicationTargetNode) ShouldPauseRefresh() bool {
	return false
}

func (node *ReplicationTargetNode) GetChildren() []MetricChild {
	var children []MetricChild

	children = append(children, MetricChild{
		Name:        "last_hour",
		Description: fmt.Sprintf("Last hour statistics for target %s", node.targetName),
	})

	children = append(children, MetricChild{
		Name:        "last_day",
		Description: fmt.Sprintf("Last day time-segmented statistics for target %s", node.targetName),
	})

	children = append(children, MetricChild{
		Name:        "since_start",
		Description: fmt.Sprintf("Cumulative statistics since startup for target %s", node.targetName),
	})

	return children
}

func (node *ReplicationTargetNode) GetLeafData() map[string]string {
	if node.target == nil {
		return map[string]string{"Status": "No target data available"}
	}

	data := make(map[string]string)

	data["Target Name"] = node.targetName
	data["Nodes Reporting"] = humanize.Comma(int64(node.target.Nodes))

	// Quick overview from last hour
	if node.target.LastHour.Events > 0 {
		data["Last Hour Events"] = humanize.Comma(node.target.LastHour.Events)
		data["Last Hour Data"] = humanize.Bytes(uint64(node.target.LastHour.Bytes))
		if node.target.LastHour.EventTimeSecs > 0 {
			rate := float64(node.target.LastHour.Events) / node.target.LastHour.EventTimeSecs
			data["Last Hour Rate"] = fmt.Sprintf("%.1f events/s", rate)
		}

		// Error rate
		totalErrors := node.target.LastHour.PutErrors + node.target.LastHour.UpdateMetaErrors +
			node.target.LastHour.DelErrors + node.target.LastHour.DelTagErrors
		if totalErrors > 0 {
			errorRate := float64(totalErrors) / float64(node.target.LastHour.Events) * 100
			data["Last Hour Error Rate"] = fmt.Sprintf("%.2f%%", errorRate)
		}
	}

	// Since start summary
	if node.target.SinceStart.Events > 0 {
		data["Total Events"] = humanize.Comma(node.target.SinceStart.Events)
		data["Total Data"] = humanize.Bytes(uint64(node.target.SinceStart.Bytes))
	}

	return data
}

func (node *ReplicationTargetNode) GetChild(name string) (MetricNode, error) {
	if node.target == nil {
		return nil, fmt.Errorf("no target data available")
	}

	switch name {
	case "last_hour":
		return NewReplicationTargetLastHourNode(node.targetName, node.target, node, node.path+"/"+name), nil
	case "last_day":
		return NewReplicationLastDayNode(node.targetName, node.target.LastDay, node, node.path+"/"+name), nil
	case "since_start":
		return NewReplicationSinceStartNode(node.targetName, node.target, node, node.path+"/"+name), nil
	default:
		return nil, fmt.Errorf("target section not found: %s", name)
	}
}

func (node *ReplicationTargetNode) GetMetricType() madmin.MetricType {
	return madmin.MetricsReplication
}
func (node *ReplicationTargetNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *ReplicationTargetNode) GetParent() MetricNode              { return node.parent }
func (node *ReplicationTargetNode) GetPath() string                    { return node.path }

// ReplicationTargetLastHourNode shows last hour statistics for a specific target
type ReplicationTargetLastHourNode struct {
	targetName string
	target     *madmin.ReplicationTargetStats
	parent     MetricNode
	path       string
}

func (node *ReplicationTargetLastHourNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewReplicationTargetLastHourNode(targetName string, target *madmin.ReplicationTargetStats, parent MetricNode, path string) *ReplicationTargetLastHourNode {
	return &ReplicationTargetLastHourNode{targetName: targetName, target: target, parent: parent, path: path}
}

func (node *ReplicationTargetLastHourNode) ShouldPauseRefresh() bool {
	return false
}

func (node *ReplicationTargetLastHourNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *ReplicationTargetLastHourNode) GetLeafData() map[string]string {
	if node.target == nil {
		return map[string]string{"Status": "No target data available"}
	}

	data := generateReplicationStatsDisplay(node.target.LastHour, true)
	data["Target Name"] = node.targetName

	return data
}

func (node *ReplicationTargetLastHourNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("target last hour node has no children")
}

func (node *ReplicationTargetLastHourNode) GetMetricType() madmin.MetricType {
	return madmin.MetricsReplication
}
func (node *ReplicationTargetLastHourNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *ReplicationTargetLastHourNode) GetParent() MetricNode              { return node.parent }
func (node *ReplicationTargetLastHourNode) GetPath() string                    { return node.path }

// ReplicationSinceStartNode shows cumulative statistics since startup for a target
type ReplicationSinceStartNode struct {
	targetName string
	target     *madmin.ReplicationTargetStats
	parent     MetricNode
	path       string
}

func (node *ReplicationSinceStartNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewReplicationSinceStartNode(targetName string, target *madmin.ReplicationTargetStats, parent MetricNode, path string) *ReplicationSinceStartNode {
	return &ReplicationSinceStartNode{targetName: targetName, target: target, parent: parent, path: path}
}

func (node *ReplicationSinceStartNode) ShouldPauseRefresh() bool {
	return false
}

func (node *ReplicationSinceStartNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *ReplicationSinceStartNode) GetLeafData() map[string]string {
	if node.target == nil {
		return map[string]string{"Status": "No target data available"}
	}

	data := generateReplicationStatsDisplay(node.target.SinceStart, true)
	data["Target Name"] = node.targetName

	return data
}

func (node *ReplicationSinceStartNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("since start node has no children")
}

func (node *ReplicationSinceStartNode) GetMetricType() madmin.MetricType {
	return madmin.MetricsReplication
}
func (node *ReplicationSinceStartNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *ReplicationSinceStartNode) GetParent() MetricNode              { return node.parent }
func (node *ReplicationSinceStartNode) GetPath() string                    { return node.path }

// ReplicationLastDayNode handles time segmentation navigation for LastDay data
type ReplicationLastDayNode struct {
	targetName string
	segmented  *madmin.SegmentedReplicationStats
	parent     MetricNode
	path       string
}

func (node *ReplicationLastDayNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewReplicationLastDayNode(targetName string, segmented *madmin.SegmentedReplicationStats, parent MetricNode, path string) *ReplicationLastDayNode {
	return &ReplicationLastDayNode{targetName: targetName, segmented: segmented, parent: parent, path: path}
}

func (node *ReplicationLastDayNode) ShouldPauseRefresh() bool {
	return true
}

func (node *ReplicationLastDayNode) GetChildren() []MetricChild {
	if node.segmented == nil || len(node.segmented.Segments) == 0 {
		return []MetricChild{}
	}

	var children []MetricChild

	// Add "Total" entry first for aggregated stats
	children = append(children, MetricChild{
		Name:        "Total",
		Description: fmt.Sprintf("Last day total statistics for target %s", node.targetName),
	})

	// Add time segments, most recent first (filter out empty segments)
	for i := len(node.segmented.Segments) - 1; i >= 0; i-- {
		segmentTime := node.segmented.FirstTime.Add(time.Duration(i*node.segmented.Interval) * time.Second)
		segmentName := segmentTime.UTC().Format("15:04Z")

		// Get event count for this segment
		var events int64
		if i < len(node.segmented.Segments) {
			events = node.segmented.Segments[i].Events
		}

		// Filter out time segments with no events
		if events == 0 {
			continue
		}

		endTime := segmentTime.Add(time.Duration(node.segmented.Interval) * time.Second)
		children = append(children, MetricChild{
			Name: segmentName,
			Description: fmt.Sprintf("%s → %s (%s events)",
				segmentTime.Local().Format("15:04"),
				endTime.Local().Format("15:04"),
				humanize.Comma(events)),
		})
	}

	return children
}

func (node *ReplicationLastDayNode) GetLeafData() map[string]string {
	if node.segmented == nil || len(node.segmented.Segments) == 0 {
		return map[string]string{
			"Status": "No last day replication data available",
			"Note":   "Last day segmented data requires MetricsDayStats flag",
		}
	}

	data := make(map[string]string)

	data["Target Name"] = node.targetName
	data["Segments Available"] = fmt.Sprintf("%d", len(node.segmented.Segments))
	if node.segmented.Interval > 0 {
		data["Segment Interval"] = fmt.Sprintf("%d minutes", node.segmented.Interval/60)
	}

	// Show time range
	if !node.segmented.FirstTime.IsZero() {
		lastTime := node.segmented.FirstTime.Add(time.Duration(len(node.segmented.Segments)*node.segmented.Interval) * time.Second)
		data["Time Range"] = fmt.Sprintf("%s → %s",
			node.segmented.FirstTime.Local().Format("15:04 MST"),
			lastTime.Local().Format("15:04 MST"))
	}

	return data
}

func (node *ReplicationLastDayNode) GetChild(name string) (MetricNode, error) {
	// Handle "Total" entry - shows aggregated stats
	if name == "Total" {
		var total madmin.ReplicationStats
		if node.segmented != nil {
			total = node.segmented.Total()
		}
		return &ReplicationLastDayTotalNode{
			targetName: node.targetName,
			total:      total,
			parent:     node,
			path:       node.path + "/" + name,
		}, nil
	}

	// Handle time segments - find by time format (with UTC indicator)
	if node.segmented != nil {
		for i := len(node.segmented.Segments) - 1; i >= 0; i-- {
			segmentTime := node.segmented.FirstTime.Add(time.Duration(i*node.segmented.Interval) * time.Second)
			if segmentTime.UTC().Format("15:04Z") == name {
				return &ReplicationTimeSegmentNode{
					targetName:  node.targetName,
					segment:     node.segmented.Segments[i],
					segmentTime: segmentTime,
					interval:    node.segmented.Interval,
					parent:      node,
					path:        node.path + "/" + name,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("time segment not found: %s", name)
}

func (node *ReplicationLastDayNode) GetMetricType() madmin.MetricType {
	return madmin.MetricsReplication
}

func (node *ReplicationLastDayNode) GetMetricFlags() madmin.MetricFlags {
	return madmin.MetricsDayStats
}
func (node *ReplicationLastDayNode) GetParent() MetricNode { return node.parent }
func (node *ReplicationLastDayNode) GetPath() string       { return node.path }

// ReplicationLastDayTotalNode shows aggregated last day statistics
type ReplicationLastDayTotalNode struct {
	targetName string
	total      madmin.ReplicationStats
	parent     MetricNode
	path       string
}

func (node *ReplicationLastDayTotalNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *ReplicationLastDayTotalNode) ShouldPauseRefresh() bool {
	return true
}

func (node *ReplicationLastDayTotalNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *ReplicationLastDayTotalNode) GetLeafData() map[string]string {
	data := generateReplicationStatsDisplay(node.total, true)
	data["Target Name"] = node.targetName
	data["Data Source"] = "Aggregated last day statistics"

	return data
}

func (node *ReplicationLastDayTotalNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("total node has no children")
}

func (node *ReplicationLastDayTotalNode) GetMetricType() madmin.MetricType {
	return madmin.MetricsReplication
}

func (node *ReplicationLastDayTotalNode) GetMetricFlags() madmin.MetricFlags {
	return madmin.MetricsDayStats
}
func (node *ReplicationLastDayTotalNode) GetParent() MetricNode { return node.parent }
func (node *ReplicationLastDayTotalNode) GetPath() string       { return node.path }

// ReplicationTimeSegmentNode shows statistics for a specific time segment
type ReplicationTimeSegmentNode struct {
	targetName  string
	segment     madmin.ReplicationStats
	segmentTime time.Time
	interval    int
	parent      MetricNode
	path        string
}

func (node *ReplicationTimeSegmentNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *ReplicationTimeSegmentNode) ShouldPauseRefresh() bool {
	return true
}

func (node *ReplicationTimeSegmentNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *ReplicationTimeSegmentNode) GetLeafData() map[string]string {
	data := generateReplicationStatsDisplay(node.segment, false)

	// Add time segment specific information
	data["Target Name"] = node.targetName
	endTime := node.segmentTime.Add(time.Duration(node.interval) * time.Second)
	data["Time Range"] = fmt.Sprintf("%s → %s",
		node.segmentTime.Local().Format("15:04:05"),
		endTime.Local().Format("15:04:05"))

	return data
}

func (node *ReplicationTimeSegmentNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("time segment node has no children")
}

func (node *ReplicationTimeSegmentNode) GetMetricType() madmin.MetricType {
	return madmin.MetricsReplication
}

func (node *ReplicationTimeSegmentNode) GetMetricFlags() madmin.MetricFlags {
	return madmin.MetricsDayStats
}
func (node *ReplicationTimeSegmentNode) GetParent() MetricNode { return node.parent }
func (node *ReplicationTimeSegmentNode) GetPath() string       { return node.path }

// ReplicationLastDayAggregatedNode displays aggregated last day replication statistics across all targets
type ReplicationLastDayAggregatedNode struct {
	replication *madmin.ReplicationMetrics
	parent      MetricNode
	path        string
}

func (node *ReplicationLastDayAggregatedNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewReplicationLastDayAggregatedNode(replication *madmin.ReplicationMetrics, parent MetricNode, path string) *ReplicationLastDayAggregatedNode {
	return &ReplicationLastDayAggregatedNode{replication: replication, parent: parent, path: path}
}

func (node *ReplicationLastDayAggregatedNode) ShouldPauseRefresh() bool {
	return true
}

func (node *ReplicationLastDayAggregatedNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *ReplicationLastDayAggregatedNode) GetLeafData() map[string]string {
	if node.replication == nil {
		return map[string]string{
			"Status": "No last day replication data available",
			"Note":   "Last day aggregated data requires MetricsDayStats flag",
		}
	}

	// Aggregate all targets' last day stats
	var aggregated madmin.ReplicationStats
	var targetCount int

	for _, targetStats := range node.replication.Targets {
		if targetStats.LastDay != nil && len(targetStats.LastDay.Segments) > 0 {
			total := targetStats.LastDay.Total()
			aggregated.Events += total.Events
			aggregated.Bytes += total.Bytes
			aggregated.EventTimeSecs += total.EventTimeSecs
			aggregated.WallTimeSecs += total.WallTimeSecs
			aggregated.LatencySecs += total.LatencySecs
			aggregated.PutObject += total.PutObject
			aggregated.UpdateMeta += total.UpdateMeta
			aggregated.DelObject += total.DelObject
			aggregated.DelTag += total.DelTag
			aggregated.PutErrors += total.PutErrors
			aggregated.UpdateMetaErrors += total.UpdateMetaErrors
			aggregated.DelErrors += total.DelErrors
			aggregated.DelTagErrors += total.DelTagErrors
			if aggregated.MaxLatencySecs < total.MaxLatencySecs {
				aggregated.MaxLatencySecs = total.MaxLatencySecs
			}
			targetCount++
		}
	}

	if targetCount == 0 {
		return map[string]string{
			"Status": "No last day replication data available",
			"Note":   "Last day aggregated data requires MetricsDayStats flag",
		}
	}

	aggregated.Nodes = targetCount
	data := generateReplicationStatsDisplay(aggregated, true)
	data["Aggregated Targets"] = fmt.Sprintf("%d targets with last day data", targetCount)

	return data
}

func (node *ReplicationLastDayAggregatedNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children available - all aggregated data shown in main display")
}

func (node *ReplicationLastDayAggregatedNode) GetMetricType() madmin.MetricType {
	return madmin.MetricsReplication
}

func (node *ReplicationLastDayAggregatedNode) GetMetricFlags() madmin.MetricFlags {
	return madmin.MetricsDayStats
}
func (node *ReplicationLastDayAggregatedNode) GetParent() MetricNode { return node.parent }
func (node *ReplicationLastDayAggregatedNode) GetPath() string       { return node.path }
