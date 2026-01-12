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

// RPCMetricsNode represents the root RPC metrics node
type RPCMetricsNode struct {
	rpc    *madmin.RPCMetrics
	parent MetricNode
	path   string
}

func (node *RPCMetricsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *RPCMetricsNode) GetChildren() []MetricChild {
	if node.rpc == nil {
		return []MetricChild{}
	}
	return []MetricChild{
		{Name: "last_minute", Description: "Last minute RPC statistics by handler"},
		{Name: "last_day", Description: "Last day RPC statistics segmented"},
		{Name: "by_destination", Description: "RPC statistics grouped by destination"},
		{Name: "by_caller", Description: "RPC statistics grouped by caller"},
	}
}

func (node *RPCMetricsNode) GetLeafData() map[string]string {
	return node.generateRPCOverviewDashboard()
}

func (node *RPCMetricsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsRPC }
func (node *RPCMetricsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *RPCMetricsNode) GetParent() MetricNode              { return node.parent }
func (node *RPCMetricsNode) GetPath() string                    { return node.path }
func (node *RPCMetricsNode) ShouldPauseRefresh() bool           { return false }

func (node *RPCMetricsNode) GetChild(name string) (MetricNode, error) {
	if node.rpc == nil {
		return nil, fmt.Errorf("no RPC data available")
	}

	switch name {
	case "last_minute":
		return &RPCLastMinuteNode{
			rpc:    node.rpc,
			parent: node,
			path:   node.path + "/last_minute",
		}, nil
	case "last_day":
		return &RPCLastDayNode{
			rpc:    node.rpc,
			parent: node,
			path:   node.path + "/last_day",
		}, nil
	case "by_destination":
		return &RPCByDestinationNode{
			rpc:    node.rpc,
			parent: node,
			path:   node.path + "/by_destination",
		}, nil
	case "by_caller":
		return &RPCByCallerNode{
			rpc:    node.rpc,
			parent: node,
			path:   node.path + "/by_caller",
		}, nil
	default:
		return nil, fmt.Errorf("unknown RPC metric child: %s", name)
	}
}

// RPCLastMinuteNode shows last minute RPC statistics by handler
type RPCLastMinuteNode struct {
	rpc    *madmin.RPCMetrics
	parent MetricNode
	path   string
}

func (node *RPCLastMinuteNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *RPCLastMinuteNode) ShouldPauseRefresh() bool { return false }

func (node *RPCLastMinuteNode) GetChildren() []MetricChild {
	// No children - all data shown as leaf data
	return []MetricChild{}
}

func (node *RPCLastMinuteNode) GetLeafData() map[string]string {
	if node.rpc == nil || len(node.rpc.LastMinute) == 0 {
		return map[string]string{"Status": "No RPC requests recorded"}
	}

	data := make(map[string]string)

	// Get sorted handler names
	handlers := make([]string, 0, len(node.rpc.LastMinute))
	for handler := range node.rpc.LastMinute {
		handlers = append(handlers, handler)
	}
	sort.Strings(handlers)

	// Add individual handler statistics
	for _, handler := range handlers {
		stats := node.rpc.LastMinute[handler]
		if stats.Requests == 0 {
			continue // Skip handlers with no requests
		}

		var parts []string

		// Average time
		if stats.Requests > 0 && stats.RequestTimeSecs > 0 {
			avgLatency := (stats.RequestTimeSecs / float64(stats.Requests)) * 1000
			parts = append(parts, fmt.Sprintf("avg: %.2fms", avgLatency))
		} else {
			parts = append(parts, "avg: 0ms")
		}

		// RPS (requests per second)
		rps := float64(stats.Requests) / 60.0 // over the minute
		parts = append(parts, fmt.Sprintf("rps: %.2f", rps))

		// Incoming bytes per second
		if stats.IncomingBytes > 0 {
			inBps := float64(stats.IncomingBytes) / 60.0
			parts = append(parts, fmt.Sprintf("in: %s/s", humanize.Bytes(uint64(inBps))))
		} else {
			parts = append(parts, "in: 0B/s")
		}

		// Outgoing bytes per second
		if stats.OutgoingBytes > 0 {
			outBps := float64(stats.OutgoingBytes) / 60.0
			parts = append(parts, fmt.Sprintf("out: %s/s", humanize.Bytes(uint64(outBps))))
		} else {
			parts = append(parts, "out: 0B/s")
		}

		// Total number of requests
		parts = append(parts, fmt.Sprintf("n: %s", humanize.Comma(stats.Requests)))

		data[handler] = strings.Join(parts, ", ")
	}

	return data
}

func (node *RPCLastMinuteNode) GetMetricType() madmin.MetricType   { return madmin.MetricsRPC }
func (node *RPCLastMinuteNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *RPCLastMinuteNode) GetParent() MetricNode              { return node.parent }
func (node *RPCLastMinuteNode) GetPath() string                    { return node.path }

func (node *RPCLastMinuteNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children available for last minute RPC stats")
}

// RPCLastDayNode shows last day RPC statistics segmented
type RPCLastDayNode struct {
	rpc    *madmin.RPCMetrics
	parent MetricNode
	path   string
}

func (node *RPCLastDayNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *RPCLastDayNode) ShouldPauseRefresh() bool { return true }

func (node *RPCLastDayNode) GetChildren() []MetricChild {
	if len(node.rpc.LastDay) == 0 {
		return []MetricChild{}
	}

	children := make([]MetricChild, 0, len(node.rpc.LastDay)+1)

	// Add "All" entry first
	children = append(children, MetricChild{
		Name:        "All",
		Description: "Aggregated statistics for all RPC handlers",
	})

	// Add individual handlers, sorted alphabetically
	handlerNames := make([]string, 0, len(node.rpc.LastDay))
	for handlerName := range node.rpc.LastDay {
		handlerNames = append(handlerNames, handlerName)
	}
	sort.Strings(handlerNames)

	for _, handlerName := range handlerNames {
		segmented := node.rpc.LastDay[handlerName]
		totalRequests := int64(0)
		totalTime := float64(0)
		for _, segment := range segmented.Segments {
			totalRequests += segment.Requests
			totalTime += segment.RequestTimeSecs
		}
		avg := ""
		if totalRequests > 0 {
			avg = fmt.Sprintf(", %.1fms avg", (totalTime/float64(totalRequests))*1000)
		}
		children = append(children, MetricChild{
			Name:        handlerName,
			Description: fmt.Sprintf("Time segmented, %d total requests%s.", totalRequests, avg),
		})
	}

	return children
}

func (node *RPCLastDayNode) GetLeafData() map[string]string {
	if node.rpc == nil || len(node.rpc.LastDay) == 0 {
		return map[string]string{"Status": "No last day RPC data available"}
	}

	// Calculate total across all handlers
	var totalStats madmin.RPCStats
	for _, segmented := range node.rpc.LastDay {
		for _, segment := range segmented.Segments {
			totalStats.Merge(segment)
		}
	}

	return generateRPCStatsDisplay(totalStats, len(node.rpc.LastDay), false, nil)
}

func (node *RPCLastDayNode) GetMetricType() madmin.MetricType   { return madmin.MetricsRPC }
func (node *RPCLastDayNode) GetMetricFlags() madmin.MetricFlags { return madmin.MetricsDayStats }
func (node *RPCLastDayNode) GetParent() MetricNode              { return node.parent }
func (node *RPCLastDayNode) GetPath() string                    { return node.path }

func (node *RPCLastDayNode) GetChild(name string) (MetricNode, error) {
	// Handle "All" entry
	if name == "All" {
		return &RPCLastDayAllNode{
			rpc:    node.rpc,
			parent: node,
			path:   node.path + "/" + name,
		}, nil
	}

	// Handle individual handlers
	if segmented, exists := node.rpc.LastDay[name]; exists {
		return &RPCLastDayHandlerNode{
			rpc:         node.rpc,
			handlerName: name,
			segmented:   segmented,
			parent:      node,
			path:        node.path + "/" + name,
		}, nil
	}

	return nil, fmt.Errorf("RPC handler not found: %s", name)
}

// RPCLastDayAllNode shows aggregated time segments for all handlers
type RPCLastDayAllNode struct {
	rpc    *madmin.RPCMetrics
	parent MetricNode
	path   string
}

func (node *RPCLastDayAllNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *RPCLastDayAllNode) ShouldPauseRefresh() bool { return true }

func (node *RPCLastDayAllNode) GetChildren() []MetricChild {
	if len(node.rpc.LastDay) == 0 {
		return []MetricChild{}
	}

	var children []MetricChild

	// Add "Total" entry first
	children = append(children, MetricChild{
		Name:        "Total",
		Description: "Last day total statistics across all time segments",
	})

	// Calculate union of all time segments across all handlers
	timeSegments := node.calculateAllTimeSegments()

	// Add time segments, most recent first (filter out empty segments)
	for i := len(timeSegments) - 1; i >= 0; i-- {
		segment := timeSegments[i]
		segmentTime := segment.Time
		endTime := segmentTime.Add(time.Duration(segment.Interval) * time.Second)
		segmentName := segmentTime.UTC().Format("15:04Z")

		// Filter out time segments with no requests
		if segment.TotalRequests == 0 {
			continue
		}

		avg := ""
		if segment.TotalRequests > 0 {
			avg = fmt.Sprintf(", %.1fms avg", (segment.TotalTime/float64(segment.TotalRequests))*1000)
		}
		day := "Today "
		if segmentTime.Local().Day() != time.Now().Day() {
			day = "Yesterday "
		}

		children = append(children, MetricChild{
			Name: segmentName,
			Description: fmt.Sprintf("RPC %s%s -> %s (%d requests%s)",
				day,
				segmentTime.Local().Format("15:04"),
				endTime.Local().Format("15:04"),
				segment.TotalRequests, avg),
		})
	}

	return children
}

// timeSegmentInfo represents aggregated information for a specific time segment
type timeSegmentInfo struct {
	Time          time.Time
	Interval      int
	TotalRequests int64
	TotalTime     float64
}

// calculateAllTimeSegments calculates the union of all time segments across all handlers
func (node *RPCLastDayAllNode) calculateAllTimeSegments() []timeSegmentInfo {
	segmentMap := make(map[int64]timeSegmentInfo)

	// Collect all unique time segments from all handlers
	for _, segmented := range node.rpc.LastDay {
		for i, segment := range segmented.Segments {
			segmentTime := segmented.FirstTime.Add(time.Duration(i*segmented.Interval) * time.Second)
			// Use Unix timestamp as key to avoid precision loss
			segmentKey := segmentTime.Unix()

			if existing, exists := segmentMap[segmentKey]; exists {
				// Aggregate with existing segment
				existing.TotalRequests += segment.Requests
				existing.TotalTime += segment.RequestTimeSecs
				segmentMap[segmentKey] = existing
			} else {
				// Create new segment
				segmentMap[segmentKey] = timeSegmentInfo{
					Time:          segmentTime,
					Interval:      segmented.Interval,
					TotalRequests: segment.Requests,
					TotalTime:     segment.RequestTimeSecs,
				}
			}
		}
	}

	// Convert map to slice and sort by time
	segments := make([]timeSegmentInfo, 0, len(segmentMap))
	for _, segment := range segmentMap {
		segments = append(segments, segment)
	}

	// Sort by time ascending
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].Time.Before(segments[j].Time)
	})

	return segments
}

func (node *RPCLastDayAllNode) GetLeafData() map[string]string {
	if node.rpc == nil || len(node.rpc.LastDay) == 0 {
		return map[string]string{"Status": "No last day RPC data available"}
	}

	// Calculate total across all handlers and segments
	var totalStats madmin.RPCStats
	for _, segmented := range node.rpc.LastDay {
		for _, segment := range segmented.Segments {
			totalStats.Merge(segment)
		}
	}
	return generateRPCStatsDisplay(totalStats, len(node.rpc.LastDay), false, nil)
}

func (node *RPCLastDayAllNode) GetMetricType() madmin.MetricType   { return madmin.MetricsRPC }
func (node *RPCLastDayAllNode) GetMetricFlags() madmin.MetricFlags { return madmin.MetricsDayStats }
func (node *RPCLastDayAllNode) GetParent() MetricNode              { return node.parent }
func (node *RPCLastDayAllNode) GetPath() string                    { return node.path }

func (node *RPCLastDayAllNode) GetChild(name string) (MetricNode, error) {
	if len(node.rpc.LastDay) == 0 {
		return nil, fmt.Errorf("no last day segmented data available")
	}

	// Handle "Total" entry
	if name == "Total" {
		return &RPCLastDayTotalNode{
			rpc:    node.rpc,
			parent: node,
			path:   node.path + "/" + name,
		}, nil
	}

	// Calculate all time segments to find the requested one
	timeSegments := node.calculateAllTimeSegments()

	// Handle time segments
	for _, segment := range timeSegments {
		segmentTime := segment.Time
		if segmentTime.UTC().Format("15:04Z") == name {
			// Create aggregated stats for this time segment
			var aggregatedStats madmin.RPCStats

			// Aggregate data from all handlers for this specific time
			for _, segmented := range node.rpc.LastDay {
				for i, handlerSegment := range segmented.Segments {
					handlerSegmentTime := segmented.FirstTime.Add(time.Duration(i*segmented.Interval) * time.Second)
					if handlerSegmentTime.Equal(segmentTime) {
						aggregatedStats.Merge(handlerSegment)
						break
					}
				}
			}

			return &RPCTimeSegmentAllNode{
				segment:     aggregatedStats,
				segmentTime: segmentTime,
				parent:      node,
				path:        node.path + "/" + name,
			}, nil
		}
	}

	return nil, fmt.Errorf("time segment not found: %s", name)
}

// RPCLastDayTotalNode shows total last day statistics
type RPCLastDayTotalNode struct {
	rpc    *madmin.RPCMetrics
	parent MetricNode
	path   string
}

func (node *RPCLastDayTotalNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *RPCLastDayTotalNode) ShouldPauseRefresh() bool   { return true }
func (node *RPCLastDayTotalNode) GetChildren() []MetricChild { return []MetricChild{} }

func (node *RPCLastDayTotalNode) GetLeafData() map[string]string {
	if node.rpc == nil || len(node.rpc.LastDay) == 0 {
		return map[string]string{"Status": "No last day RPC data available"}
	}

	var totalStats madmin.RPCStats
	for _, segmented := range node.rpc.LastDay {
		for _, segment := range segmented.Segments {
			totalStats.Merge(segment)
		}
	}
	return generateRPCStatsDisplay(totalStats, len(node.rpc.LastDay), false, nil)
}

func (node *RPCLastDayTotalNode) GetMetricType() madmin.MetricType   { return madmin.MetricsRPC }
func (node *RPCLastDayTotalNode) GetMetricFlags() madmin.MetricFlags { return madmin.MetricsDayStats }
func (node *RPCLastDayTotalNode) GetParent() MetricNode              { return node.parent }
func (node *RPCLastDayTotalNode) GetPath() string                    { return node.path }
func (node *RPCLastDayTotalNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children available for last day total node")
}

// RPCTimeSegmentAllNode shows aggregated RPC statistics for a specific time segment
type RPCTimeSegmentAllNode struct {
	segment     madmin.RPCStats
	segmentTime time.Time
	parent      MetricNode
	path        string
}

func (node *RPCTimeSegmentAllNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *RPCTimeSegmentAllNode) ShouldPauseRefresh() bool   { return true }
func (node *RPCTimeSegmentAllNode) GetChildren() []MetricChild { return []MetricChild{} }

func (node *RPCTimeSegmentAllNode) GetLeafData() map[string]string {
	return generateRPCStatsDisplay(node.segment, 1, false, nil)
}

func (node *RPCTimeSegmentAllNode) GetMetricType() madmin.MetricType   { return madmin.MetricsRPC }
func (node *RPCTimeSegmentAllNode) GetMetricFlags() madmin.MetricFlags { return madmin.MetricsDayStats }
func (node *RPCTimeSegmentAllNode) GetParent() MetricNode              { return node.parent }
func (node *RPCTimeSegmentAllNode) GetPath() string                    { return node.path }
func (node *RPCTimeSegmentAllNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children available for time segment")
}

// RPCLastDayHandlerNode shows segmented statistics for a specific RPC handler
type RPCLastDayHandlerNode struct {
	rpc         *madmin.RPCMetrics
	handlerName string
	segmented   madmin.SegmentedRPCMetrics
	parent      MetricNode
	path        string
}

func (node *RPCLastDayHandlerNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *RPCLastDayHandlerNode) ShouldPauseRefresh() bool { return true }

func (node *RPCLastDayHandlerNode) GetChildren() []MetricChild {
	if len(node.segmented.Segments) == 0 {
		return []MetricChild{}
	}

	var children []MetricChild

	// Add "Total" entry first
	children = append(children, MetricChild{
		Name:        "Total",
		Description: fmt.Sprintf("Total last day statistics for %s", node.handlerName),
	})

	// Add time segments, most recent first (filter out empty segments)
	for i := len(node.segmented.Segments) - 1; i >= 0; i-- {
		segment := node.segmented.Segments[i]

		// Filter out time segments with no requests
		if segment.Requests == 0 {
			continue
		}

		segmentTime := node.segmented.FirstTime.Add(time.Duration(i*node.segmented.Interval) * time.Second)
		endTime := segmentTime.Add(time.Duration(node.segmented.Interval) * time.Second)
		segmentName := segmentTime.UTC().Format("15:04Z")

		avg := fmt.Sprintf(", %.1fms avg", (segment.RequestTimeSecs/float64(segment.Requests))*1000)
		day := "Today "
		if segmentTime.Local().Day() != time.Now().Day() {
			day = "Yesterday "
		}

		children = append(children, MetricChild{
			Name: segmentName,
			Description: fmt.Sprintf("%s%s -> %s (%d requests%s)",
				day,
				segmentTime.Local().Format("15:04"),
				endTime.Local().Format("15:04"),
				segment.Requests, avg),
		})
	}

	return children
}

func (node *RPCLastDayHandlerNode) GetLeafData() map[string]string {
	// Calculate total for this handler
	var totalStats madmin.RPCStats
	for _, segment := range node.segmented.Segments {
		totalStats.Merge(segment)
	}
	return generateRPCStatsDisplay(totalStats, 1, false, nil)
}

func (node *RPCLastDayHandlerNode) GetMetricType() madmin.MetricType   { return madmin.MetricsRPC }
func (node *RPCLastDayHandlerNode) GetMetricFlags() madmin.MetricFlags { return madmin.MetricsDayStats }
func (node *RPCLastDayHandlerNode) GetParent() MetricNode              { return node.parent }
func (node *RPCLastDayHandlerNode) GetPath() string                    { return node.path }

func (node *RPCLastDayHandlerNode) GetChild(name string) (MetricNode, error) {
	// Handle "Total" entry
	if name == "Total" {
		var totalStats madmin.RPCStats
		for _, segment := range node.segmented.Segments {
			totalStats.Merge(segment)
		}

		return &RPCHandlerTotalNode{
			handler:   node.handlerName,
			stats:     totalStats,
			parent:    node,
			path:      node.path + "/" + name,
			timeRange: "last day",
		}, nil
	}

	// Handle time segments
	for i := len(node.segmented.Segments) - 1; i >= 0; i-- {
		segmentTime := node.segmented.FirstTime.Add(time.Duration(i*node.segmented.Interval) * time.Second)
		if segmentTime.UTC().Format("15:04Z") == name {
			return &RPCHandlerSegmentNode{
				handler:     node.handlerName,
				stats:       node.segmented.Segments[i],
				segmentTime: segmentTime,
				parent:      node,
				path:        node.path + "/" + name,
			}, nil
		}
	}

	return nil, fmt.Errorf("time segment not found: %s", name)
}

// RPCConnectionsNode shows RPC connection statistics and health
type RPCConnectionsNode struct {
	rpc    *madmin.RPCMetrics
	parent MetricNode
	path   string
}

func (node *RPCConnectionsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *RPCConnectionsNode) ShouldPauseRefresh() bool { return false }

func (node *RPCConnectionsNode) GetChildren() []MetricChild {
	if node.rpc == nil {
		return []MetricChild{}
	}
	children := make([]MetricChild, 0, 1)
	// Connection summary
	children = append(children, MetricChild{
		Name:        "summary",
		Description: fmt.Sprintf("Connection health overview (Connected: %d, Disconnected: %d)", node.rpc.Connected, node.rpc.Disconnected),
	})

	return children
}

func (node *RPCConnectionsNode) GetLeafData() map[string]string {
	if node.rpc == nil {
		return map[string]string{"Status": "No RPC connection data available"}
	}

	data := make(map[string]string)

	data["Total Nodes"] = fmt.Sprintf("%d", node.rpc.Nodes)
	data["Connected"] = fmt.Sprintf("%d", node.rpc.Connected)
	data["Disconnected"] = fmt.Sprintf("%d", node.rpc.Disconnected)

	if node.rpc.Nodes > 0 {
		connectionRate := float64(node.rpc.Connected) / float64(node.rpc.Nodes) * 100
		data["Connection Rate"] = fmt.Sprintf("%.1f%%", connectionRate)
	}

	data["Last Updated"] = node.rpc.CollectedAt.Format("2006-01-02 15:04:05")

	return data
}

func (node *RPCConnectionsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsRPC }
func (node *RPCConnectionsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *RPCConnectionsNode) GetParent() MetricNode              { return node.parent }
func (node *RPCConnectionsNode) GetPath() string                    { return node.path }

func (node *RPCConnectionsNode) GetChild(name string) (MetricNode, error) {
	switch name {
	case "summary":
		return &RPCConnectionSummaryNode{
			rpc:    node.rpc,
			parent: node,
			path:   node.path + "/" + name,
		}, nil
	default:
		return nil, fmt.Errorf("connection child not found: %s", name)
	}
}

// RPCConnectionSummaryNode shows connection summary details
type RPCConnectionSummaryNode struct {
	rpc    *madmin.RPCMetrics
	parent MetricNode
	path   string
}

func (node *RPCConnectionSummaryNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *RPCConnectionSummaryNode) ShouldPauseRefresh() bool   { return false }
func (node *RPCConnectionSummaryNode) GetChildren() []MetricChild { return []MetricChild{} }

func (node *RPCConnectionSummaryNode) GetLeafData() map[string]string {
	if node.rpc == nil {
		return map[string]string{"Status": "No RPC connection data available"}
	}

	data := make(map[string]string)

	data["Cluster Nodes"] = fmt.Sprintf("%d nodes configured", node.rpc.Nodes)
	data["Connected Nodes"] = fmt.Sprintf("%d online", node.rpc.Connected)
	if node.rpc.Disconnected > 0 {
		data["Disconnected Nodes"] = fmt.Sprintf("%d offline", node.rpc.Disconnected)
	}

	if node.rpc.Nodes > 0 {
		data["Connections"] = fmt.Sprintf("%.1f per node", float64(node.rpc.Connected)/float64(node.rpc.Nodes))
	}

	// Add activity summary
	totalActivity := int64(0)
	for _, stats := range node.rpc.LastMinute {
		totalActivity += stats.Requests
	}
	if totalActivity > 0 {
		data["Recent Activity"] = fmt.Sprintf("%s requests (last minute)", humanize.Comma(totalActivity))
	} else {
		data["Recent Activity"] = "No recent RPC activity"
	}

	return data
}

func (node *RPCConnectionSummaryNode) GetMetricType() madmin.MetricType   { return madmin.MetricsRPC }
func (node *RPCConnectionSummaryNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *RPCConnectionSummaryNode) GetParent() MetricNode              { return node.parent }
func (node *RPCConnectionSummaryNode) GetPath() string                    { return node.path }
func (node *RPCConnectionSummaryNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children available for connection summary")
}

// RPCByDestinationNode groups RPC statistics by destination
type RPCByDestinationNode struct {
	rpc    *madmin.RPCMetrics
	parent MetricNode
	path   string
}

func (node *RPCByDestinationNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *RPCByDestinationNode) ShouldPauseRefresh() bool { return false }

func (node *RPCByDestinationNode) GetChildren() []MetricChild {
	if len(node.rpc.ByDestination) == 0 {
		return []MetricChild{}
	}

	destinations := make([]string, 0, len(node.rpc.ByDestination))
	for dest := range node.rpc.ByDestination {
		destinations = append(destinations, dest)
	}
	sort.Strings(destinations)

	children := make([]MetricChild, 0, len(node.rpc.ByDestination))
	for _, dest := range destinations {
		stats := node.rpc.ByDestination[dest]

		var parts []string

		// Connection status
		if stats.Connected > 0 {
			parts = append(parts, fmt.Sprintf("%d connected", stats.Connected))
		}
		if stats.Disconnected > 0 {
			parts = append(parts, fmt.Sprintf("%d disconnected", stats.Disconnected))
		}

		// Message counts
		if stats.OutgoingMessages > 0 {
			parts = append(parts, fmt.Sprintf("%s out msgs", humanize.Comma(stats.OutgoingMessages)))
		}
		if stats.IncomingMessages > 0 {
			parts = append(parts, fmt.Sprintf("%s in msgs", humanize.Comma(stats.IncomingMessages)))
		}

		// Ping info
		if stats.LastPingMS > 0 {
			parts = append(parts, fmt.Sprintf("%.1fms ping", stats.LastPingMS))
		}

		description := strings.Join(parts, ", ")
		if description == "" {
			description = "No activity"
		}

		children = append(children, MetricChild{
			Name:        dest,
			Description: description,
		})
	}
	return children
}

func (node *RPCByDestinationNode) GetLeafData() map[string]string {
	if len(node.rpc.ByDestination) == 0 {
		return map[string]string{"Status": "No destination data available"}
	}

	data := make(map[string]string)

	var totalConnected, totalDisconnected int
	var totalOutgoing, totalIncoming int64
	var totalOutBytes, totalInBytes int64

	for _, stats := range node.rpc.ByDestination {
		totalConnected += stats.Connected
		totalDisconnected += stats.Disconnected
		totalOutgoing += stats.OutgoingMessages
		totalIncoming += stats.IncomingMessages
		totalOutBytes += stats.OutgoingBytes
		totalInBytes += stats.IncomingBytes
	}

	data["Active Destinations"] = fmt.Sprintf("%d", len(node.rpc.ByDestination))
	data["Total Connected"] = fmt.Sprintf("%d", totalConnected)
	if totalDisconnected > 0 {
		data["Total Disconnected"] = fmt.Sprintf("%d", totalDisconnected)
	}

	data["Total Outgoing"] = humanize.Comma(totalOutgoing) + " Messages"
	data["Total Incoming"] = humanize.Comma(totalIncoming) + " Messages"
	if totalOutBytes > 0 {
		data["Total Outgoing"] += ". " + humanize.Bytes(uint64(totalOutBytes))
	}
	if totalInBytes > 0 {
		data["Total Incoming"] = ". " + humanize.Bytes(uint64(totalInBytes))
	}

	return data
}

func (node *RPCByDestinationNode) GetMetricType() madmin.MetricType   { return madmin.MetricsRPC }
func (node *RPCByDestinationNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *RPCByDestinationNode) GetParent() MetricNode              { return node.parent }
func (node *RPCByDestinationNode) GetPath() string                    { return node.path }

func (node *RPCByDestinationNode) GetChild(name string) (MetricNode, error) {
	if stats, exists := node.rpc.ByDestination[name]; exists {
		return &RPCDestinationNode{
			destination: name,
			stats:       stats,
			parent:      node,
			path:        node.path + "/" + name,
		}, nil
	}
	return nil, fmt.Errorf("destination not found: %s", name)
}

// RPCByCallerNode groups RPC statistics by caller (similar to destination for now)
type RPCByCallerNode struct {
	rpc    *madmin.RPCMetrics
	parent MetricNode
	path   string
}

func (node *RPCByCallerNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *RPCByCallerNode) ShouldPauseRefresh() bool { return false }

func (node *RPCByCallerNode) GetChildren() []MetricChild {
	if len(node.rpc.ByCaller) == 0 {
		return []MetricChild{}
	}

	callers := make([]string, 0, len(node.rpc.ByCaller))
	for caller := range node.rpc.ByCaller {
		callers = append(callers, caller)
	}
	sort.Strings(callers)

	children := make([]MetricChild, 0, len(node.rpc.ByCaller))
	for _, caller := range callers {
		stats := node.rpc.ByCaller[caller]

		var parts []string

		// Connection status
		if stats.Connected > 0 {
			parts = append(parts, fmt.Sprintf("%d connected", stats.Connected))
		}
		if stats.Disconnected > 0 {
			parts = append(parts, fmt.Sprintf("%d disconnected", stats.Disconnected))
		}

		// Message counts
		if stats.IncomingMessages > 0 {
			parts = append(parts, fmt.Sprintf("%s in msgs", humanize.Comma(stats.IncomingMessages)))
		}
		if stats.OutgoingMessages > 0 {
			parts = append(parts, fmt.Sprintf("%s out msgs", humanize.Comma(stats.OutgoingMessages)))
		}

		// Ping info
		if stats.LastPingMS > 0 {
			parts = append(parts, fmt.Sprintf("%.1fms ping", stats.LastPingMS))
		}

		description := strings.Join(parts, ", ")
		if description == "" {
			description = "No activity"
		}

		children = append(children, MetricChild{
			Name:        caller,
			Description: description,
		})
	}
	return children
}

func (node *RPCByCallerNode) GetLeafData() map[string]string {
	if len(node.rpc.ByCaller) == 0 {
		return map[string]string{"Status": "No caller data available"}
	}

	data := make(map[string]string)

	var totalConnected, totalDisconnected int
	var totalOutgoing, totalIncoming int64
	var totalOutBytes, totalInBytes int64

	for _, stats := range node.rpc.ByCaller {
		totalConnected += stats.Connected
		totalDisconnected += stats.Disconnected
		totalOutgoing += stats.OutgoingMessages
		totalIncoming += stats.IncomingMessages
		totalOutBytes += stats.OutgoingBytes
		totalInBytes += stats.IncomingBytes
	}

	data["Active Callers"] = fmt.Sprintf("%d", len(node.rpc.ByCaller))
	data["Total Connected"] = fmt.Sprintf("%d", totalConnected)
	if totalDisconnected > 0 {
		data["Total Disconnected"] = fmt.Sprintf("%d", totalDisconnected)
	}
	data["Total Outgoing Messages"] = humanize.Comma(totalOutgoing)
	data["Total Incoming Messages"] = humanize.Comma(totalIncoming)

	if totalOutBytes > 0 {
		data["Total Outgoing Bytes"] = humanize.Bytes(uint64(totalOutBytes))
	}
	if totalInBytes > 0 {
		data["Total Incoming Bytes"] = humanize.Bytes(uint64(totalInBytes))
	}

	return data
}

func (node *RPCByCallerNode) GetMetricType() madmin.MetricType   { return madmin.MetricsRPC }
func (node *RPCByCallerNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *RPCByCallerNode) GetParent() MetricNode              { return node.parent }
func (node *RPCByCallerNode) GetPath() string                    { return node.path }

func (node *RPCByCallerNode) GetChild(name string) (MetricNode, error) {
	if stats, exists := node.rpc.ByCaller[name]; exists {
		return &RPCCallerNode{
			caller: name,
			stats:  stats,
			parent: node,
			path:   node.path + "/" + name,
		}, nil
	}
	return nil, fmt.Errorf("caller not found: %s", name)
}

// RPCDestinationNode shows detailed connection statistics for a specific destination
type RPCDestinationNode struct {
	destination string
	stats       madmin.ConnectionStats
	parent      MetricNode
	path        string
}

func (node *RPCDestinationNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *RPCDestinationNode) ShouldPauseRefresh() bool   { return true }
func (node *RPCDestinationNode) GetChildren() []MetricChild { return []MetricChild{} }

func (node *RPCDestinationNode) GetLeafData() map[string]string {
	data := make(map[string]string)

	data["Destination"] = node.destination
	data["Connected"] = fmt.Sprintf("%d", node.stats.Connected)
	data["Disconnected"] = fmt.Sprintf("%d", node.stats.Disconnected)
	data["Reconnect Count"] = fmt.Sprintf("%d", node.stats.ReconnectCount)

	// Stream information
	if node.stats.OutgoingStreams > 0 {
		data["Outgoing Streams"] = fmt.Sprintf("%d", node.stats.OutgoingStreams)
	}
	if node.stats.IncomingStreams > 0 {
		data["Incoming Streams"] = fmt.Sprintf("%d", node.stats.IncomingStreams)
	}

	// Message counts
	data["Outgoing Messages"] = humanize.Comma(node.stats.OutgoingMessages)
	data["Incoming Messages"] = humanize.Comma(node.stats.IncomingMessages)

	// Bytes
	if node.stats.OutgoingBytes > 0 {
		data["Outgoing Bytes"] = humanize.Bytes(uint64(node.stats.OutgoingBytes))
	}
	if node.stats.IncomingBytes > 0 {
		data["Incoming Bytes"] = humanize.Bytes(uint64(node.stats.IncomingBytes))
	}

	// Queue and timing
	if node.stats.OutQueue > 0 {
		data["Outgoing Queue"] = fmt.Sprintf("%d", node.stats.OutQueue)
	}
	if node.stats.LastPingMS > 0 {
		data["Last Ping"] = fmt.Sprintf("%.2f ms", node.stats.LastPingMS)
	}
	if node.stats.MaxPingDurMS > 0 {
		data["Max Ping"] = fmt.Sprintf("%.2f ms", node.stats.MaxPingDurMS)
	}

	// Connection timing
	if !node.stats.LastConnectTime.IsZero() {
		data["Last Connect"] = fmt.Sprintf("%s (%v ago)", node.stats.LastConnectTime.Format("2006-01-02 15:04:05"),
			time.Since(node.stats.LastConnectTime).Round(time.Minute).String())
	}
	if !node.stats.LastPongTime.IsZero() {
		data["Last Pong"] = fmt.Sprintf("%s (%v ago)", node.stats.LastPongTime.Format("2006-01-02 15:04:05"),
			time.Since(node.stats.LastPongTime).Round(time.Minute).String())
	}

	return data
}

func (node *RPCDestinationNode) GetMetricType() madmin.MetricType   { return madmin.MetricsRPC }
func (node *RPCDestinationNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *RPCDestinationNode) GetParent() MetricNode              { return node.parent }
func (node *RPCDestinationNode) GetPath() string                    { return node.path }
func (node *RPCDestinationNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children available for destination")
}

// RPCCallerNode shows detailed connection statistics for a specific caller
type RPCCallerNode struct {
	caller string
	stats  madmin.ConnectionStats
	parent MetricNode
	path   string
}

func (node *RPCCallerNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *RPCCallerNode) ShouldPauseRefresh() bool   { return true }
func (node *RPCCallerNode) GetChildren() []MetricChild { return []MetricChild{} }

func (node *RPCCallerNode) GetLeafData() map[string]string {
	data := make(map[string]string)

	data["Caller"] = node.caller
	data["Connected"] = fmt.Sprintf("%d", node.stats.Connected)
	data["Disconnected"] = fmt.Sprintf("%d", node.stats.Disconnected)
	data["Reconnect Count"] = fmt.Sprintf("%d", node.stats.ReconnectCount)

	// Stream information
	if node.stats.IncomingStreams > 0 {
		data["Incoming Streams"] = fmt.Sprintf("%d", node.stats.IncomingStreams)
	}
	if node.stats.OutgoingStreams > 0 {
		data["Outgoing Streams"] = fmt.Sprintf("%d", node.stats.OutgoingStreams)
	}

	// Message counts
	data["Incoming Messages"] = humanize.Comma(node.stats.IncomingMessages)
	data["Outgoing Messages"] = humanize.Comma(node.stats.OutgoingMessages)

	// Bytes
	if node.stats.IncomingBytes > 0 {
		data["Incoming Bytes"] = humanize.Bytes(uint64(node.stats.IncomingBytes))
	}
	if node.stats.OutgoingBytes > 0 {
		data["Outgoing Bytes"] = humanize.Bytes(uint64(node.stats.OutgoingBytes))
	}

	// Queue and timing
	if node.stats.OutQueue > 0 {
		data["Outgoing Queue"] = fmt.Sprintf("%d", node.stats.OutQueue)
	}
	if node.stats.Connected > 0 {
		data["Last Ping"] = fmt.Sprintf("%.2f ms", node.stats.LastPingMS/float64(node.stats.Connected))
		data["Max Ping"] = fmt.Sprintf("%.2f ms", node.stats.MaxPingDurMS/float64(node.stats.Connected))
	}

	// Connection timing
	if !node.stats.LastConnectTime.IsZero() {
		data["Last Connect"] = fmt.Sprintf("%s (%v ago)", node.stats.LastConnectTime.Format("2006-01-02 15:04:05"),
			time.Since(node.stats.LastConnectTime).Round(time.Minute).String())
	}
	if !node.stats.LastPongTime.IsZero() {
		data["Last Pong"] = fmt.Sprintf("%s (%v ago)", node.stats.LastPongTime.Format("2006-01-02 15:04:05"),
			time.Since(node.stats.LastPongTime).Round(time.Minute).String())
	}

	return data
}

func (node *RPCCallerNode) GetMetricType() madmin.MetricType   { return madmin.MetricsRPC }
func (node *RPCCallerNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *RPCCallerNode) GetParent() MetricNode              { return node.parent }
func (node *RPCCallerNode) GetPath() string                    { return node.path }
func (node *RPCCallerNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children available for caller")
}

// RPCHandlerNode shows detailed statistics for a specific RPC handler
type RPCHandlerNode struct {
	stats  madmin.RPCStats
	parent MetricNode
	path   string
}

func (node *RPCHandlerNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *RPCHandlerNode) ShouldPauseRefresh() bool   { return true }
func (node *RPCHandlerNode) GetChildren() []MetricChild { return []MetricChild{} }

func (node *RPCHandlerNode) GetLeafData() map[string]string {
	return generateRPCStatsDisplay(node.stats, 1, false, nil)
}

func (node *RPCHandlerNode) GetMetricType() madmin.MetricType   { return madmin.MetricsRPC }
func (node *RPCHandlerNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *RPCHandlerNode) GetParent() MetricNode              { return node.parent }
func (node *RPCHandlerNode) GetPath() string                    { return node.path }
func (node *RPCHandlerNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children available for RPC handler")
}

// RPCHandlerTotalNode shows total statistics for a handler over a time range
type RPCHandlerTotalNode struct {
	handler   string
	stats     madmin.RPCStats
	parent    MetricNode
	path      string
	timeRange string
}

func (node *RPCHandlerTotalNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *RPCHandlerTotalNode) ShouldPauseRefresh() bool   { return true }
func (node *RPCHandlerTotalNode) GetChildren() []MetricChild { return []MetricChild{} }

func (node *RPCHandlerTotalNode) GetLeafData() map[string]string {
	data := generateRPCStatsDisplay(node.stats, 1, false, nil)
	data["Handler"] = node.handler
	data["Time Range"] = node.timeRange
	return data
}

func (node *RPCHandlerTotalNode) GetMetricType() madmin.MetricType   { return madmin.MetricsRPC }
func (node *RPCHandlerTotalNode) GetMetricFlags() madmin.MetricFlags { return madmin.MetricsDayStats }
func (node *RPCHandlerTotalNode) GetParent() MetricNode              { return node.parent }
func (node *RPCHandlerTotalNode) GetPath() string                    { return node.path }
func (node *RPCHandlerTotalNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children available for handler total")
}

// RPCHandlerSegmentNode shows statistics for a specific handler in a time segment
type RPCHandlerSegmentNode struct {
	handler     string
	stats       madmin.RPCStats
	segmentTime time.Time
	parent      MetricNode
	path        string
}

func (node *RPCHandlerSegmentNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *RPCHandlerSegmentNode) ShouldPauseRefresh() bool   { return true }
func (node *RPCHandlerSegmentNode) GetChildren() []MetricChild { return []MetricChild{} }

func (node *RPCHandlerSegmentNode) GetLeafData() map[string]string {
	data := generateRPCStatsDisplay(node.stats, 1, false, nil)
	data["Handler"] = node.handler
	data["Time Segment"] = node.segmentTime.Format("2006-01-02 15:04:05")
	return data
}

func (node *RPCHandlerSegmentNode) GetMetricType() madmin.MetricType   { return madmin.MetricsRPC }
func (node *RPCHandlerSegmentNode) GetMetricFlags() madmin.MetricFlags { return madmin.MetricsDayStats }
func (node *RPCHandlerSegmentNode) GetParent() MetricNode              { return node.parent }
func (node *RPCHandlerSegmentNode) GetPath() string                    { return node.path }
func (node *RPCHandlerSegmentNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children available for handler segment")
}

// Helper function to generate RPC statistics display
func generateRPCStatsDisplay(stats madmin.RPCStats, _ int, showHandlerBreakdown bool, lastMinute map[string]madmin.RPCStats) map[string]string {
	data := make(map[string]string)

	// Basic request statistics
	data["Total Requests"] = humanize.Comma(stats.Requests)

	// Timing information
	if stats.Requests > 0 {
		avgLatency := (stats.RequestTimeSecs / float64(stats.Requests)) * 1000
		data["Average Latency"] = fmt.Sprintf("%.2f ms", avgLatency)
		data["Total Time"] = fmt.Sprintf("%.2f sec", stats.RequestTimeSecs)
	} else {
		data["Average Latency"] = "0 ms"
		data["Total Time"] = "0 sec"
	}

	// Network throughput
	totalBytes := stats.IncomingBytes + stats.OutgoingBytes
	if totalBytes > 0 {
		data["Total Throughput"] = humanize.Bytes(uint64(totalBytes))
		data["Incoming Bytes"] = humanize.Bytes(uint64(stats.IncomingBytes))
		data["Outgoing Bytes"] = humanize.Bytes(uint64(stats.OutgoingBytes))

		if stats.Requests > 0 {
			avgBytesPerReq := totalBytes / stats.Requests
			data["Avg Bytes/Request"] = humanize.Bytes(uint64(avgBytesPerReq))
		}
	} else {
		data["Total Throughput"] = "0 B"
	}

	// Performance metrics
	if stats.Requests > 0 && stats.RequestTimeSecs > 0 {
		rps := float64(stats.Requests) / stats.RequestTimeSecs
		data["Requests/Second"] = fmt.Sprintf("%.1f", rps)

		if totalBytes > 0 {
			bps := float64(totalBytes) / stats.RequestTimeSecs
			data["Bytes/Second"] = humanize.Bytes(uint64(bps)) + "/s"
		}
	}

	// Handler breakdown for last minute stats
	if showHandlerBreakdown && lastMinute != nil {
		data["Active Handlers"] = fmt.Sprintf("%d", len(lastMinute))

		// Find top 3 handlers by request count
		type handlerStats struct {
			name     string
			requests int64
		}
		var handlers []handlerStats
		for name, hstats := range lastMinute {
			if hstats.Requests > 0 {
				handlers = append(handlers, handlerStats{name, hstats.Requests})
			}
		}

		// Sort by request count descending
		sort.Slice(handlers, func(i, j int) bool {
			return handlers[i].requests > handlers[j].requests
		})

		// Show top 3 handlers
		for i, h := range handlers {
			if i >= 3 {
				break
			}
			key := fmt.Sprintf("Top %d Handler", i+1)
			percentage := float64(h.requests) / float64(stats.Requests) * 100
			data[key] = fmt.Sprintf("%s (%s req, %.1f%%)", h.name, humanize.Comma(h.requests), percentage)
		}
	}

	return data
}

// Helper function to generate RPC overview dashboard
func (node *RPCMetricsNode) generateRPCOverviewDashboard() map[string]string {
	data := make(map[string]string)

	// Check if RPC data is available
	if node.rpc == nil {
		return map[string]string{"Status": "No RPC metrics available"}
	}

	// Collection timestamp
	data["Last Updated"] = node.rpc.CollectedAt.Format("2006-01-02 15:04:05")

	// Connection status
	data["Cluster Nodes"] = fmt.Sprintf("%d nodes configured", node.rpc.Nodes)
	data["Connected Nodes"] = fmt.Sprintf("%d online", node.rpc.Connected)
	if node.rpc.Disconnected > 0 {
		data["Disconnected Nodes"] = fmt.Sprintf("%d offline", node.rpc.Disconnected)
	}

	// Connection health assessment
	if node.rpc.Nodes > 0 {
		data["Connections"] = fmt.Sprintf("%.1f per node", float64(node.rpc.Connected)/float64(node.rpc.Nodes))
	}

	// Activity summary (last minute)
	if len(node.rpc.LastMinute) > 0 {
		var totalRequests int64
		var totalLatency float64
		var totalBytes int64
		handlerCount := 0

		for _, stats := range node.rpc.LastMinute {
			if stats.Requests > 0 {
				totalRequests += stats.Requests
				totalLatency += stats.RequestTimeSecs
				totalBytes += stats.IncomingBytes + stats.OutgoingBytes
				handlerCount++
			}
		}

		data["Recent Activity"] = fmt.Sprintf("%s requests (last minute)", humanize.Comma(totalRequests))
		data["Active Handlers"] = fmt.Sprintf("%d handlers", handlerCount)

		if totalRequests > 0 {
			if totalLatency > 0 {
				avgLatency := (totalLatency / float64(totalRequests)) * 1000
				data["Avg Response Time"] = fmt.Sprintf("%.2f ms", avgLatency)
			}

			rps := float64(totalRequests) / 60.0 // per second over the minute
			data["Request Rate"] = fmt.Sprintf("%.1f req/s", rps)

			if totalBytes > 0 {
				data["Throughput"] = humanize.Bytes(uint64(totalBytes))
				bps := float64(totalBytes) / 60.0 // per second over the minute
				data["Rate"] = humanize.Bytes(uint64(bps)) + "/s"
			}
		}
	} else {
		data["Recent Activity"] = "No recent RPC activity"
	}

	return data
}
