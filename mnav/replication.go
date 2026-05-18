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

// shortARN trims "arn:minio:replication::338f8fdd-16da-41da-82fb-c36acd2fef8a:bucket"
// to "c36acd2fef8a:bucket" by dropping everything through the last UUID dash.
func shortARN(arn string) string {
	idx := strings.Index(arn, "::")
	if idx < 0 {
		return arn
	}
	rest := arn[idx+2:]
	colon := strings.Index(rest, ":")
	if colon < 0 {
		return arn
	}
	dash := strings.LastIndex(rest[:colon], "-")
	if dash < 0 {
		return arn
	}
	return rest[dash+1:]
}

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

// prependEntry inserts a key before all existing numbered entries.
// It shifts existing "nn:" keys up by 1 and inserts at "00:".
func prependEntry(data map[string]string, key, value string) {
	// Find max index
	maxIdx := -1
	for k := range data {
		if len(k) > 2 && k[2] == ':' {
			var idx int
			if _, err := fmt.Sscanf(k[:2], "%d", &idx); err == nil && idx > maxIdx {
				maxIdx = idx
			}
		}
	}
	// Shift all entries up by 1
	for i := maxIdx; i >= 0; i-- {
		old := fmt.Sprintf("%02d:", i)
		for k, v := range data {
			if strings.HasPrefix(k, old) {
				newKey := fmt.Sprintf("%02d:%s", i+1, k[3:])
				data[newKey] = v
				delete(data, k)
			}
		}
	}
	data[fmt.Sprintf("00:%s", key)] = value
}

// generateReplicationStatsDisplay formats ReplicationStats for display
func generateReplicationStatsDisplay(stats madmin.ReplicationStats, includeTimeInfo bool) map[string]string {
	data := make(map[string]string)

	if stats.Nodes == 0 && stats.Events == 0 {
		data["Status"] = "No replication data available"
		return data
	}

	idx := 0
	add := func(key, value string) {
		data[fmt.Sprintf("%02d:%s", idx, key)] = value
		idx++
	}

	// Time information
	if includeTimeInfo && stats.StartTime != nil && stats.EndTime != nil {
		add("Time Range", fmt.Sprintf("%s → %s",
			stats.StartTime.Local().Format("15:04:05"),
			stats.EndTime.Local().Format("15:04:05")))
		if stats.WallTimeSecs > 0 {
			add("Duration", fmt.Sprintf("%.1f seconds", stats.WallTimeSecs))
		}
	}

	// Activity Summary
	if stats.Nodes > 0 {
		add("Nodes Reporting", humanize.Comma(int64(stats.Nodes)))
	}
	add("Total Events", humanize.Comma(stats.Events))
	add("Data Transferred", humanize.Bytes(uint64(stats.Bytes)))
	if stats.EventTimeSecs > 0 {
		add("Event Rate", fmt.Sprintf("%.1f events/s", float64(stats.Events)/stats.EventTimeSecs))
	}
	if stats.WallTimeSecs > 0 {
		add("Throughput", fmt.Sprintf("%s/s", humanize.Bytes(uint64(float64(stats.Bytes)/stats.WallTimeSecs))))
	}

	// Latency metrics
	if stats.Events > 0 && stats.LatencySecs > 0 {
		add("Average Latency", formatReplicationLatency(stats.LatencySecs/float64(stats.Events)))
	}
	if stats.MaxLatencySecs > 0 {
		add("Maximum Latency", formatReplicationLatency(stats.MaxLatencySecs))
	}

	// Operation breakdown
	totalOps := stats.PutObject + stats.UpdateMeta + stats.DelObject + stats.DelTag
	opPct := func(n int64) string {
		if totalOps == 0 {
			return humanize.Comma(n)
		}
		return fmt.Sprintf("%s (%.1f%%)", humanize.Comma(n), float64(n)/float64(totalOps)*100)
	}
	add("PUT Operations", opPct(stats.PutObject))
	add("Update Metadata", opPct(stats.UpdateMeta))
	add("DELETE Operations", opPct(stats.DelObject))
	add("DELETE Tag Operations", opPct(stats.DelTag))

	// Error analysis
	totalErrors := stats.PutErrors + stats.UpdateMetaErrors + stats.DelErrors + stats.DelTagErrors
	if stats.Events > 0 {
		add("Error Rate", fmt.Sprintf("%.2f%% (%s errors)",
			float64(totalErrors)/float64(stats.Events)*100, humanize.Comma(totalErrors)))
	}
	if totalErrors > 0 {
		errPct := func(n int64) string {
			return fmt.Sprintf("%s (%.1f%%)", humanize.Comma(n), float64(n)/float64(totalErrors)*100)
		}
		add("PUT Errors", errPct(stats.PutErrors))
		add("Metadata Errors", errPct(stats.UpdateMetaErrors))
		add("DELETE Errors", errPct(stats.DelErrors))
		add("DELETE Tag Errors", errPct(stats.DelTagErrors))
	}

	// Outcome analysis
	totalOutcomes := stats.Synced + stats.AlreadyOK + stats.Rejected
	outPct := func(n int64) string {
		if totalOutcomes == 0 {
			return humanize.Comma(n)
		}
		return fmt.Sprintf("%s (%.1f%%)", humanize.Comma(n), float64(n)/float64(totalOutcomes)*100)
	}
	add("Synced", outPct(stats.Synced))
	add("Already Synchronized", outPct(stats.AlreadyOK))
	add("Rejected", outPct(stats.Rejected))

	// Proxy operations
	totalProxy := stats.ProxyEvents + stats.ProxyHead + stats.ProxyGet + stats.ProxyGetTag
	add("Proxy Operations", humanize.Comma(totalProxy))
	if totalProxy > 0 {
		add("Proxy Data Transfer", humanize.Bytes(uint64(stats.ProxyBytes)))
		if stats.ProxyHead > 0 {
			add("HEAD Proxy Success", fmt.Sprintf("%.1f%% (%s/%s)",
				float64(stats.ProxyHeadOK)/float64(stats.ProxyHead)*100,
				humanize.Comma(stats.ProxyHeadOK), humanize.Comma(stats.ProxyHead)))
		}
		if stats.ProxyGet > 0 {
			add("GET Proxy Success", fmt.Sprintf("%.1f%% (%s/%s)",
				float64(stats.ProxyGetOK)/float64(stats.ProxyGet)*100,
				humanize.Comma(stats.ProxyGetOK), humanize.Comma(stats.ProxyGet)))
		}
		if stats.ProxyGetTag > 0 {
			add("GET Tag Proxy Success", fmt.Sprintf("%.1f%% (%s/%s)",
				float64(stats.ProxyGetTagOK)/float64(stats.ProxyGetTag)*100,
				humanize.Comma(stats.ProxyGetTagOK), humanize.Comma(stats.ProxyGetTag)))
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
		type targetEntry struct {
			name   string
			events int64
		}
		entries := make([]targetEntry, 0, len(node.replication.Targets))
		for targetName, ts := range node.replication.Targets {
			entries = append(entries, targetEntry{name: targetName, events: ts.LastHour.Events + ts.SinceStart.Events})
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].events != entries[j].events {
				return entries[i].events > entries[j].events
			}
			return entries[i].name < entries[j].name
		})

		for _, e := range entries {
			ts := node.replication.Targets[e.name]
			children = append(children, MetricChild{
				Name:        shortARN(e.name),
				Description: targetChildDesc(ts),
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
			targets = append(targets, shortARN(targetName))
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

	// Handle individual targets by short ARN
	for fullARN, targetStats := range node.replication.Targets {
		if shortARN(fullARN) == name {
			return NewReplicationTargetNode(fullARN, &targetStats, node, node.path+"/"+name), nil
		}
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
	targetCount := len(node.replication.Targets)
	if targetCount > 0 {
		prependEntry(data, "Aggregation", fmt.Sprintf("Combined from %d targets", targetCount))
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
	children := make([]MetricChild, 0, 3)
	children = append(children, MetricChild{
		Name:        "last_hour",
		Description: fmt.Sprintf("Last hour statistics for target %s", shortARN(node.targetName)),
	})

	children = append(children, MetricChild{
		Name:        "last_day",
		Description: fmt.Sprintf("Last day time-segmented statistics for target %s", shortARN(node.targetName)),
	})

	children = append(children, MetricChild{
		Name:        "since_start",
		Description: fmt.Sprintf("Cumulative statistics since startup for target %s", shortARN(node.targetName)),
	})

	return children
}

func (node *ReplicationTargetNode) GetLeafData() map[string]string {
	if node.target == nil {
		return map[string]string{"Status": "No target data available"}
	}

	data := make(map[string]string)

	data["Target Name"] = shortARN(node.targetName)
	data["Nodes Reporting"] = humanize.Comma(int64(node.target.Nodes))

	addStatsSummary(data, "Last Hour", node.target.LastHour)
	if node.target.LastDay != nil && len(node.target.LastDay.Segments) > 0 {
		day := node.target.LastDay.Total()
		addStatsSummary(data, "Last Day", day)
	}
	addStatsSummary(data, "Since Start", node.target.SinceStart)

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
	prependEntry(data, "Target", shortARN(node.targetName))
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
	prependEntry(data, "Target", shortARN(node.targetName))
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

// addStatsSummary adds a compact summary of ReplicationStats to data,
// prefixed with the given label (e.g. "Last Hour", "Last Day").
func addStatsSummary(data map[string]string, prefix string, s madmin.ReplicationStats) {
	if s.Events == 0 {
		return
	}
	data[prefix+" Events"] = humanize.Comma(s.Events)
	data[prefix+" Data"] = humanize.Bytes(uint64(s.Bytes))
	if s.EventTimeSecs > 0 {
		data[prefix+" Rate"] = fmt.Sprintf("%.1f events/s", float64(s.Events)/s.EventTimeSecs)
	}
	if s.LatencySecs > 0 {
		avg := s.LatencySecs / float64(s.Events)
		data[prefix+" Avg Latency"] = formatReplicationLatency(avg)
	}
	totalErrors := s.PutErrors + s.UpdateMetaErrors + s.DelErrors + s.DelTagErrors
	if totalErrors > 0 {
		pct := float64(totalErrors) / float64(s.Events) * 100
		data[prefix+" Errors"] = fmt.Sprintf("%.2f%% (%s)", pct, humanize.Comma(totalErrors))
	}
	synced := s.Synced + s.AlreadyOK
	if synced > 0 {
		data[prefix+" Synced"] = fmt.Sprintf("%s (%.1f%%)",
			humanize.Comma(synced), float64(synced)/float64(s.Events)*100)
	}
}

func targetChildDesc(ts madmin.ReplicationTargetStats) string {
	s := ts.LastHour
	label := "1h"
	if s.Events == 0 {
		// Fall back to since-start if last hour is empty.
		s = ts.SinceStart
		label = "total"
	}
	if s.Events == 0 {
		return "No events"
	}
	totalErrs := s.PutErrors + s.UpdateMetaErrors + s.DelErrors + s.DelTagErrors
	errPct := float64(totalErrs) / float64(s.Events) * 100
	return fmt.Sprintf("%s: %s events, %s, %.1f%% errors",
		label, humanize.Comma(s.Events), humanize.Bytes(uint64(s.Bytes)), errPct)
}

func replicationSegmentDesc(start, end time.Time, s madmin.ReplicationStats) string {
	totalErrs := s.PutErrors + s.UpdateMetaErrors + s.DelErrors + s.DelTagErrors
	errPct := float64(0)
	if s.Events > 0 {
		errPct = float64(totalErrs) / float64(s.Events) * 100
	}
	return fmt.Sprintf("%s → %s  %s events, %s, %.1f%% errors",
		start.Local().Format("15:04"),
		end.Local().Format("15:04"),
		humanize.Comma(s.Events),
		humanize.Bytes(uint64(s.Bytes)),
		errPct)
}

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
		Description: fmt.Sprintf("Last day total statistics for target %s", shortARN(node.targetName)),
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
			Name:        segmentName,
			Description: replicationSegmentDesc(segmentTime, endTime, node.segmented.Segments[i]),
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

	data["Target Name"] = shortARN(node.targetName)
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
	prependEntry(data, "Target", shortARN(node.targetName))
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
	endTime := node.segmentTime.Add(time.Duration(node.interval) * time.Second)
	prependEntry(data, "Time Range", fmt.Sprintf("%s → %s",
		node.segmentTime.Local().Format("15:04:05"),
		endTime.Local().Format("15:04:05")))
	prependEntry(data, "Target", shortARN(node.targetName))

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

// aggregated merges LastDay segments across all targets.
func (node *ReplicationLastDayAggregatedNode) aggregated() *madmin.SegmentedReplicationStats {
	if node.replication == nil {
		return nil
	}
	var merged madmin.SegmentedReplicationStats
	var found bool
	for _, t := range node.replication.Targets {
		if t.LastDay != nil && len(t.LastDay.Segments) > 0 {
			merged.Add(t.LastDay)
			found = true
		}
	}
	if !found {
		return nil
	}
	return &merged
}

func (node *ReplicationLastDayAggregatedNode) ShouldPauseRefresh() bool {
	return true
}

func (node *ReplicationLastDayAggregatedNode) GetChildren() []MetricChild {
	seg := node.aggregated()
	if seg == nil || len(seg.Segments) == 0 {
		return []MetricChild{}
	}

	children := []MetricChild{{
		Name:        "Total",
		Description: "Aggregated total across all targets",
	}}

	for i := len(seg.Segments) - 1; i >= 0; i-- {
		if seg.Segments[i].Events == 0 {
			continue
		}
		segTime := seg.FirstTime.Add(time.Duration(i*seg.Interval) * time.Second)
		endTime := segTime.Add(time.Duration(seg.Interval) * time.Second)
		children = append(children, MetricChild{
			Name:        segTime.UTC().Format("15:04Z"),
			Description: replicationSegmentDesc(segTime, endTime, seg.Segments[i]),
		})
	}
	return children
}

func (node *ReplicationLastDayAggregatedNode) GetLeafData() map[string]string {
	seg := node.aggregated()
	if seg == nil {
		return map[string]string{
			"Status": "No last day replication data available",
			"Note":   "Last day aggregated data requires MetricsDayStats flag",
		}
	}

	data := make(map[string]string)
	targetCount := 0
	for _, t := range node.replication.Targets {
		if t.LastDay != nil && len(t.LastDay.Segments) > 0 {
			targetCount++
		}
	}
	data["Aggregated Targets"] = fmt.Sprintf("%d targets with last day data", targetCount)
	data["Segments Available"] = fmt.Sprintf("%d", len(seg.Segments))
	if seg.Interval > 0 {
		data["Segment Interval"] = fmt.Sprintf("%d minutes", seg.Interval/60)
	}
	if !seg.FirstTime.IsZero() {
		lastTime := seg.FirstTime.Add(time.Duration(len(seg.Segments)*seg.Interval) * time.Second)
		data["Time Range"] = fmt.Sprintf("%s → %s",
			seg.FirstTime.Local().Format("15:04 MST"),
			lastTime.Local().Format("15:04 MST"))
	}
	return data
}

func (node *ReplicationLastDayAggregatedNode) GetChild(name string) (MetricNode, error) {
	seg := node.aggregated()
	if seg == nil {
		return nil, fmt.Errorf("no last day data available")
	}

	if name == "Total" {
		total := seg.Total()
		return &ReplicationLastDayTotalNode{
			targetName: "all targets",
			total:      total,
			parent:     node,
			path:       node.path + "/Total",
		}, nil
	}

	for i := len(seg.Segments) - 1; i >= 0; i-- {
		segTime := seg.FirstTime.Add(time.Duration(i*seg.Interval) * time.Second)
		if segTime.UTC().Format("15:04Z") == name {
			return &ReplicationTimeSegmentNode{
				targetName:  "all targets",
				segment:     seg.Segments[i],
				segmentTime: segTime,
				interval:    seg.Interval,
				parent:      node,
				path:        fmt.Sprintf("%s/%s", node.path, name),
			}, nil
		}
	}
	return nil, fmt.Errorf("time segment not found: %s", name)
}

func (node *ReplicationLastDayAggregatedNode) GetMetricType() madmin.MetricType {
	return madmin.MetricsReplication
}

func (node *ReplicationLastDayAggregatedNode) GetMetricFlags() madmin.MetricFlags {
	return madmin.MetricsDayStats
}
func (node *ReplicationLastDayAggregatedNode) GetParent() MetricNode { return node.parent }
func (node *ReplicationLastDayAggregatedNode) GetPath() string       { return node.path }
