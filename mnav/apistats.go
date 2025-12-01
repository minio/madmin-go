package mnav

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/minio/madmin-go/v4"
)

// === ENHANCED API METRICS FORMATTING ===

type APIMetricsNode struct {
	api    *madmin.APIMetrics
	parent MetricNode
	path   string
}

func (a *APIMetricsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(a)
}

func (n *APIMetricsNode) ShouldPauseRefresh() bool {
	return false
}

func (node *APIMetricsNode) GetChildren() []MetricChild {
	return []MetricChild{
		{Name: "last_minute", Description: "Last minute API statistics by endpoint"},
		{Name: "last_day", Description: "Last day API statistics segmented"},
		{Name: "since_start", Description: "API statistics since server start"},
	}
}

func (node *APIMetricsNode) GetLeafData() map[string]string {
	// Create comprehensive executive-level API performance dashboard
	return node.generateAPIOverviewDashboard()
}
func (node *APIMetricsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsAPI }
func (node *APIMetricsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *APIMetricsNode) GetParent() MetricNode              { return node.parent }
func (node *APIMetricsNode) GetPath() string                    { return node.path }
func (node *APIMetricsNode) GetChild(name string) (MetricNode, error) {
	if node.api == nil {
		return nil, fmt.Errorf("no API data available")
	}

	switch name {
	case "last_minute":
		return &APILastMinuteNode{
			api:    node.api,
			parent: node,
			path:   node.path + "/last_minute",
		}, nil
	case "last_day":
		return &APILastDayNode{
			api:    node.api,
			parent: node,
			path:   node.path + "/last_day",
		}, nil
	case "since_start":
		return &APISinceStartNode{
			api:    node.api,
			parent: node,
			path:   node.path + "/since_start",
		}, nil
	default:
		return nil, fmt.Errorf("unknown API metric child: %s", name)
	}
}

// === API CHILD NODES ===

// APILastMinuteNode shows last minute API statistics by endpoint
type APILastMinuteNode struct {
	api    *madmin.APIMetrics
	parent MetricNode
	path   string
}

func (node *APILastMinuteNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *APILastMinuteNode) ShouldPauseRefresh() bool {
	return false
}

func (node *APILastMinuteNode) GetChildren() []MetricChild {
	if node.api.LastMinuteAPI == nil || len(node.api.LastMinuteAPI) == 0 {
		return []MetricChild{}
	}

	// Get sorted endpoint names to ensure consistent ordering
	var endpoints []string
	for endpoint := range node.api.LastMinuteAPI {
		endpoints = append(endpoints, endpoint)
	}
	sort.Strings(endpoints)

	var children []MetricChild
	for _, endpoint := range endpoints {
		if node.api.LastMinuteAPI[endpoint].Requests == 0 {
			continue
		}
		lastMinute := node.api.LastMinuteAPI[endpoint]
		avgLatency := float64(0)
		var sz string
		if lastMinute.Requests > 0 {
			avgLatency = (lastMinute.RequestTimeSecs / float64(lastMinute.Requests)) * 1000
			inout := []string{""}
			if lastMinute.IncomingBytes > 0 {
				inout = append(inout, "in: "+humanize.Bytes(uint64(lastMinute.IncomingBytes)))
			}
			if lastMinute.OutgoingBytes > 0 {
				inout = append(inout, "out: "+humanize.Bytes(uint64(lastMinute.OutgoingBytes)))
			}
			if len(inout) > 1 {
				sz = strings.Join(inout, ", ")
			}
		}

		children = append(children, MetricChild{
			Name: endpoint,
			Description: fmt.Sprintf("%s req (%.1fms avg%s)",
				humanize.Comma(lastMinute.Requests), avgLatency, sz),
		})
	}
	return children
}

// generateAPIStatsDisplay creates a consistent API statistics display
func generateAPIStatsDisplay(stats madmin.APIStats, endpointsCount int, showTopEndpoints bool, endpoints map[string]madmin.APIStats) map[string]string {
	if stats.Requests == 0 {
		data := make(map[string]string)
		data["Status"] = "No API requests recorded"
		return data
	}

	// Use ordered slice to maintain consistent display order
	var entries []struct{ key, value string }

	// === BASIC METRICS ===
	entries = append(entries, struct{ key, value string }{"Total Requests", humanize.Comma(stats.Requests)})
	if endpointsCount > 0 {
		entries = append(entries, struct{ key, value string }{"Active Endpoints", fmt.Sprintf("%d endpoints", endpointsCount)})
	}
	entries = append(entries, struct{ key, value string }{"Responding Nodes", fmt.Sprintf("%d nodes", stats.Nodes)})

	// Calculate RPS if we have wall time
	if stats.WallTimeSecs > 0 {
		rps := float64(stats.Requests) / stats.WallTimeSecs
		entries = append(entries, struct{ key, value string }{"Avg RPS", fmt.Sprintf("%.1f req/sec", rps)})
	}

	// === TIMING METRICS ===
	avgLatency := (stats.RequestTimeSecs / float64(stats.Requests)) * 1000
	entries = append(entries, struct{ key, value string }{"Avg Latency", fmt.Sprintf("%.1f ms", avgLatency)})

	if stats.RespTTFBSecs > 0 {
		avgTTFB := (stats.RespTTFBSecs / float64(stats.Requests)) * 1000
		entries = append(entries, struct{ key, value string }{"Avg TTFB", fmt.Sprintf("%.1f ms", avgTTFB)})
	}

	// === TIMING RANGES ===
	if stats.RequestTimeSecsMax > 0 {
		entries = append(entries, struct{ key, value string }{"Latency Range", fmt.Sprintf("%.1f - %.1f ms",
			stats.RequestTimeSecsMin*1000, stats.RequestTimeSecsMax*1000)})
	}
	if stats.RespTTFBSecsMax > 0 {
		entries = append(entries, struct{ key, value string }{"TTFB Range", fmt.Sprintf("%.1f - %.1f ms",
			stats.RespTTFBSecsMin*1000, stats.RespTTFBSecsMax*1000)})
	}
	if stats.ReqReadSecsMax > 0 {
		entries = append(entries, struct{ key, value string }{"Req Read Range", fmt.Sprintf("%.1f - %.1f ms",
			stats.ReqReadSecsMin*1000, stats.ReqReadSecsMax*1000)})
	}
	if stats.RespSecsMax > 0 {
		entries = append(entries, struct{ key, value string }{"Resp Wr Range", fmt.Sprintf("%.1f - %.1f ms",
			stats.RespSecsMin*1000, stats.RespSecsMax*1000)})
	}

	// === THROUGHPUT ===
	totalBytes := stats.IncomingBytes + stats.OutgoingBytes
	if totalBytes > 0 {
		entries = append(entries, struct{ key, value string }{"Total Throughput", humanize.Bytes(uint64(totalBytes))})
		entries = append(entries, struct{ key, value string }{"-> Incoming", humanize.Bytes(uint64(stats.IncomingBytes))})
		entries = append(entries, struct{ key, value string }{"<- Outgoing", humanize.Bytes(uint64(stats.OutgoingBytes))})

		avgBytesPerReq := totalBytes / stats.Requests
		entries = append(entries, struct{ key, value string }{"Avg Bytes", humanize.Bytes(uint64(avgBytesPerReq)) + "/req"})
	}

	// === ERROR ANALYSIS ===
	totalErrors := stats.Errors4xx + stats.Errors5xx
	if stats.Requests > 0 {
		if stats.Requests > 0 {
			errorRate := float64(totalErrors) / float64(stats.Requests) * 100
			entries = append(entries, struct{ key, value string }{"Error Rate", fmt.Sprintf("%.2f%% (%d errors)", errorRate, totalErrors)})
		}
		if totalErrors > 0 {
			if stats.Errors4xx > 0 {
				clientErrorRate := float64(stats.Errors4xx) / float64(stats.Requests) * 100
				entries = append(entries, struct{ key, value string }{"↳ 4xx Client Errors", fmt.Sprintf("%d (%.2f%%)",
					stats.Errors4xx, clientErrorRate)})
			}
			if stats.Errors5xx > 0 {
				serverErrorRate := float64(stats.Errors5xx) / float64(stats.Requests) * 100
				entries = append(entries, struct{ key, value string }{"↳ 5xx Server Errors", fmt.Sprintf("%d (%.2f%%)",
					stats.Errors5xx, serverErrorRate)})
			}
			if stats.Canceled > 0 {
				cancelRate := float64(stats.Canceled) / float64(stats.Requests) * 100
				entries = append(entries, struct{ key, value string }{"↳ Canceled Requests", fmt.Sprintf("%d (%.2f%%)", stats.Canceled, cancelRate)})
			}
		}
	}

	// === REJECTIONS ===
	totalRejected := stats.Rejected.Auth + stats.Rejected.Header +
		stats.Rejected.Invalid + stats.Rejected.NotImplemented +
		stats.Rejected.RequestsTime
	if totalRejected > 0 {
		rejectionRate := float64(totalRejected) / float64(stats.Requests) * 100
		entries = append(entries, struct{ key, value string }{"Rejected Requests", fmt.Sprintf("%d rejections (%.2f%%)", totalRejected, rejectionRate)})

		if stats.Rejected.Auth > 0 {
			authRate := float64(stats.Rejected.Auth) / float64(stats.Requests) * 100
			entries = append(entries, struct{ key, value string }{"↳ Authentication", fmt.Sprintf("%d (%.2f%%)", stats.Rejected.Auth, authRate)})
		}
		if stats.Rejected.Header > 0 {
			headerRate := float64(stats.Rejected.Header) / float64(stats.Requests) * 100
			entries = append(entries, struct{ key, value string }{"↳ Header Issues", fmt.Sprintf("%d (%.2f%%)", stats.Rejected.Header, headerRate)})
		}
		if stats.Rejected.Invalid > 0 {
			invalidRate := float64(stats.Rejected.Invalid) / float64(stats.Requests) * 100
			entries = append(entries, struct{ key, value string }{"↳ Invalid Requests", fmt.Sprintf("%d (%.2f%%)", stats.Rejected.Invalid, invalidRate)})
		}
		if stats.Rejected.NotImplemented > 0 {
			notImplRate := float64(stats.Rejected.NotImplemented) / float64(stats.Requests) * 100
			entries = append(entries, struct{ key, value string }{"↳ Not Implemented", fmt.Sprintf("%d (%.2f%%)", stats.Rejected.NotImplemented, notImplRate)})
		}
		if stats.Rejected.RequestsTime > 0 {
			timeRate := float64(stats.Rejected.RequestsTime) / float64(stats.Requests) * 100
			entries = append(entries, struct{ key, value string }{"↳ Outdated Signatures", fmt.Sprintf("%d (%.2f%%)", stats.Rejected.RequestsTime, timeRate)})
		}
	}

	// === BLOCKING ANALYSIS ===
	if stats.Requests > 0 && (stats.ReadBlockedSecs > 0 || stats.WriteBlockedSecs > 0) {
		if stats.ReadBlockedSecs > 0 {
			avgReadBlocked := (stats.ReadBlockedSecs / float64(stats.Requests)) * 1000
			entries = append(entries, struct{ key, value string }{"Avg Read Blocking", fmt.Sprintf("%.1f ms/req", avgReadBlocked)})
		}
		if stats.WriteBlockedSecs > 0 {
			avgWriteBlocked := (stats.WriteBlockedSecs / float64(stats.Requests)) * 1000
			entries = append(entries, struct{ key, value string }{"Avg Write Blocking", fmt.Sprintf("%.1f ms/req", avgWriteBlocked)})
		}
	}

	// === TOP ENDPOINTS (only for last_minute) ===
	if showTopEndpoints && endpoints != nil {
		type endpointStat struct {
			name  string
			stats madmin.APIStats
		}

		var endpointList []endpointStat
		for name, stat := range endpoints {
			endpointList = append(endpointList, endpointStat{name, stat})
		}

		// Sort by request count, name second.
		slices.SortFunc(endpointList, func(a, b endpointStat) int {
			if a.stats.Requests > b.stats.Requests {
				return -1
			}
			if a.stats.Requests < b.stats.Requests {
				return 1
			}
			if a.name < b.name {
				return -1
			}
			return 1
		})

		maxShow := 5
		if len(endpointList) < maxShow {
			maxShow = len(endpointList)
		}

		if maxShow > 0 {
			entries = append(entries, struct{ key, value string }{"Top Endpoints", fmt.Sprintf("Showing %d busiest endpoints", maxShow)})
			for i := 0; i < maxShow; i++ {
				ep := endpointList[i]
				if ep.stats.Requests > 0 {
					avgLatency := (ep.stats.RequestTimeSecs / float64(ep.stats.Requests)) * 1000
					errors := ep.stats.Errors4xx + ep.stats.Errors5xx
					epName := ep.name[:min(len(ep.name), 15)]
					entries = append(entries, struct{ key, value string }{fmt.Sprintf("* %s", epName), fmt.Sprintf("%s req, %.1fms avg, %d err",
						humanize.Comma(ep.stats.Requests), avgLatency, errors)})
				}
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

func (node *APILastMinuteNode) GetLeafData() map[string]string {
	if node.api == nil {
		return map[string]string{"Status": "No API metrics available"}
	}

	total := node.api.LastMinuteTotal()
	return generateAPIStatsDisplay(total, len(node.api.LastMinuteAPI), true, node.api.LastMinuteAPI)
}

func (node *APILastMinuteNode) GetMetricType() madmin.MetricType   { return madmin.MetricsAPI }
func (node *APILastMinuteNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *APILastMinuteNode) GetParent() MetricNode              { return node.parent }
func (node *APILastMinuteNode) GetPath() string                    { return node.path }
func (node *APILastMinuteNode) GetChild(name string) (MetricNode, error) {
	if node.api.LastMinuteAPI == nil {
		return nil, fmt.Errorf("no last minute API data available")
	}

	if stats, exists := node.api.LastMinuteAPI[name]; exists {
		return &APIEndpointNode{
			endpoint: name,
			stats:    stats,
			parent:   node,
			path:     node.path + "/" + name,
		}, nil
	}

	return nil, fmt.Errorf("endpoint not found: %s", name)
}

// APILastDayNode shows last day API statistics segmented
type APILastDayNode struct {
	api    *madmin.APIMetrics
	parent MetricNode
	path   string
}

func (node *APILastDayNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *APILastDayNode) ShouldPauseRefresh() bool {
	return true
}

func (node *APILastDayNode) GetChildren() []MetricChild {
	if len(node.api.LastDayAPI) == 0 {
		return []MetricChild{}
	}

	var children []MetricChild

	// Add "All" entry first - shows aggregated time segments
	children = append(children, MetricChild{
		Name:        "All",
		Description: "Aggregated 24h statistics for all API endpoints",
	})

	// Add individual API endpoints, sorted alphabetically
	var apiNames []string
	for apiName := range node.api.LastDayAPI {
		apiNames = append(apiNames, apiName)
	}
	sort.Strings(apiNames)

	for _, apiName := range apiNames {
		segmented := node.api.LastDayAPI[apiName]
		totalRequests := int64(0)
		totalTimeSecs := float64(0)
		for _, segment := range segmented.Segments {
			totalRequests += segment.Requests
			totalTimeSecs += segment.RequestTimeSecs
		}
		avg := ""
		if totalRequests > 0 {
			avg = fmt.Sprintf(" %.1fms avg.", (totalTimeSecs/float64(totalRequests))*1000)
		}
		children = append(children, MetricChild{
			Name:        apiName,
			Description: fmt.Sprintf("Time segmented - %d total requests.%s", totalRequests, avg),
		})
	}

	return children
}

func (node *APILastDayNode) GetLeafData() map[string]string {
	if node.api == nil {
		return map[string]string{"Status": "No API metrics available"}
	}

	total := node.api.LastDayTotal()
	return generateAPIStatsDisplay(total, len(node.api.LastDayAPI), false, nil)
}

func (node *APILastDayNode) GetMetricType() madmin.MetricType   { return madmin.MetricsAPI }
func (node *APILastDayNode) GetMetricFlags() madmin.MetricFlags { return madmin.MetricsDayStats }
func (node *APILastDayNode) GetParent() MetricNode              { return node.parent }
func (node *APILastDayNode) GetPath() string                    { return node.path }

func (node *APILastDayNode) GetChild(name string) (MetricNode, error) {
	// Handle "All" entry - shows aggregated time segments
	if name == "All" {
		return &APILastDayAllNode{
			api:    node.api,
			parent: node,
			path:   node.path + "/" + name,
		}, nil
	}

	// Handle individual API endpoints
	if segmented, exists := node.api.LastDayAPI[name]; exists {
		return &APILastDayEndpointNode{
			api:       node.api,
			apiName:   name,
			segmented: segmented,
			parent:    node,
			path:      node.path + "/" + name,
		}, nil
	}

	return nil, fmt.Errorf("API endpoint not found: %s", name)
}

// APILastDayAllNode shows aggregated time segments for all API endpoints
type APILastDayAllNode struct {
	api    *madmin.APIMetrics
	parent MetricNode
	path   string
}

func (node *APILastDayAllNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *APILastDayAllNode) ShouldPauseRefresh() bool {
	return true
}

func (node *APILastDayAllNode) GetChildren() []MetricChild {
	segmented := node.api.LastDayTotalSegmented()
	if len(segmented.Segments) == 0 {
		return []MetricChild{}
	}

	var children []MetricChild

	// Add "Total" entry first
	children = append(children, MetricChild{
		Name:        "Total",
		Description: "Last day total statistics across all time segments",
	})

	// Add time segments, most recent first (filter out empty segments)
	for i := len(segmented.Segments) - 1; i >= 0; i-- {
		segmentTime := segmented.FirstTime.Add(time.Duration(i*segmented.Interval) * time.Second)
		endTime := segmentTime.Add(time.Duration(segmented.Interval) * time.Second)
		segmentName := segmentTime.UTC().Format("15:04Z")

		// Get request count for this segment
		requests := int64(0)
		if i < len(segmented.Segments) {
			requests = segmented.Segments[i].Requests
		}

		// Filter out time segments with no requests
		if requests == 0 {
			continue
		}

		avg := ""
		if requests > 0 {
			avg = fmt.Sprintf(", %.1fms avg", (segmented.Segments[i].RequestTimeSecs/float64(requests))*1000)
		}
		day := "Today "
		if segmentTime.Local().Day() != time.Now().Day() {
			day = "Yesterday "
		}
		children = append(children, MetricChild{
			Name: segmentName,
			Description: fmt.Sprintf("API %s%s -> %s (%d requests%s)",
				day,
				segmentTime.Local().Format("15:04"),
				endTime.Local().Format("15:04"),
				requests, avg),
		})
	}

	return children
}

func (node *APILastDayAllNode) GetChild(name string) (MetricNode, error) {
	segmented := node.api.LastDayTotalSegmented()
	if len(segmented.Segments) == 0 {
		return nil, fmt.Errorf("no last day segmented data available")
	}

	// Handle "Total" entry
	if name == "Total" {
		return &APILastDayTotalNode{
			api:    node.api,
			parent: node,
			path:   node.path + "/" + name,
		}, nil
	}

	// Handle time segments - find by time format (with UTC indicator)
	for i := len(segmented.Segments) - 1; i >= 0; i-- {
		segmentTime := segmented.FirstTime.Add(time.Duration(i*segmented.Interval) * time.Second)
		if segmentTime.UTC().Format("15:04Z") == name {
			return &APITimeSegmentAllNode{
				segment:     segmented.Segments[i],
				segmentTime: segmentTime,
				parent:      node,
				path:        node.path + "/" + name,
			}, nil
		}
	}

	return nil, fmt.Errorf("time segment not found: %s", name)
}

func (node *APILastDayAllNode) GetLeafData() map[string]string {
	if node.api == nil {
		return map[string]string{"Status": "No API metrics available"}
	}

	total := node.api.LastDayTotal()
	return generateAPIStatsDisplay(total, len(node.api.LastDayAPI), false, nil)
}

func (node *APILastDayAllNode) GetMetricType() madmin.MetricType   { return madmin.MetricsAPI }
func (node *APILastDayAllNode) GetMetricFlags() madmin.MetricFlags { return madmin.MetricsDayStats }
func (node *APILastDayAllNode) GetParent() MetricNode              { return node.parent }
func (node *APILastDayAllNode) GetPath() string                    { return node.path }

// APILastDayEndpointNode shows time segments for a specific API endpoint
type APILastDayEndpointNode struct {
	api       *madmin.APIMetrics
	apiName   string
	segmented madmin.SegmentedAPIMetrics
	parent    MetricNode
	path      string
}

func (node *APILastDayEndpointNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *APILastDayEndpointNode) GetChildren() []MetricChild {
	if len(node.segmented.Segments) == 0 {
		return []MetricChild{}
	}

	var children []MetricChild

	// Add "Total" entry first
	children = append(children, MetricChild{
		Name:        "Total",
		Description: fmt.Sprintf("Total statistics for %s across all time segments", node.apiName),
	})

	// Add time segments, most recent first (filter out empty segments)
	for i := len(node.segmented.Segments) - 1; i >= 0; i-- {
		segmentTime := node.segmented.FirstTime.Add(time.Duration(i*node.segmented.Interval) * time.Second)
		endTime := segmentTime.Add(time.Duration(node.segmented.Interval) * time.Second)
		segmentName := segmentTime.UTC().Format("15:04Z")

		// Get request count for this segment
		requests := int64(0)
		if i < len(node.segmented.Segments) {
			requests = node.segmented.Segments[i].Requests
		}

		// Filter out time segments with no requests
		if requests == 0 {
			continue
		}
		avg := ""
		if requests > 0 {
			avg = fmt.Sprintf(", %.1fms avg", (node.segmented.Segments[i].RequestTimeSecs/float64(requests))*1000)
		}
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
				requests, avg),
		})
	}

	return children
}

func (node *APILastDayEndpointNode) GetChild(name string) (MetricNode, error) {
	if len(node.segmented.Segments) == 0 {
		return nil, fmt.Errorf("no segmented data available for API %s", node.apiName)
	}

	// Handle "Total" entry
	if name == "Total" {
		// Calculate total stats for this endpoint
		total := madmin.APIStats{}
		for _, segment := range node.segmented.Segments {
			total.Merge(segment)
		}

		return &APIEndpointNode{
			endpoint: node.apiName,
			stats:    total,
			parent:   node,
			path:     node.path + "/" + name,
		}, nil
	}

	// Handle time segments - find by time format (with UTC indicator)
	for i := len(node.segmented.Segments) - 1; i >= 0; i-- {
		segmentTime := node.segmented.FirstTime.Add(time.Duration(i*node.segmented.Interval) * time.Second)
		if segmentTime.UTC().Format("15:04Z") == name {
			return &APIEndpointNode{
				endpoint: node.apiName,
				stats:    node.segmented.Segments[i],
				parent:   node,
				path:     node.path + "/" + name,
			}, nil
		}
	}

	return nil, fmt.Errorf("time segment not found: %s", name)
}

func (node *APILastDayEndpointNode) GetLeafData() map[string]string {
	if len(node.segmented.Segments) == 0 {
		return map[string]string{"Status": "No endpoint data available"}
	}

	// Calculate total stats for this endpoint
	total := madmin.APIStats{}
	for _, segment := range node.segmented.Segments {
		total.Merge(segment)
	}
	return generateAPIStatsDisplay(total, 1, false, nil)
}

func (node *APILastDayEndpointNode) GetMetricType() madmin.MetricType { return madmin.MetricsAPI }
func (node *APILastDayEndpointNode) GetMetricFlags() madmin.MetricFlags {
	return madmin.MetricsDayStats
}
func (node *APILastDayEndpointNode) GetParent() MetricNode { return node.parent }
func (node *APILastDayEndpointNode) GetPath() string       { return node.path }
func (node *APILastDayEndpointNode) ShouldPauseRefresh() bool {
	return true
}

// APISinceStartNode shows API statistics since server start
type APISinceStartNode struct {
	api    *madmin.APIMetrics
	parent MetricNode
	path   string
}

func (node *APISinceStartNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *APISinceStartNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *APISinceStartNode) GetLeafData() map[string]string {
	if node.api == nil {
		return map[string]string{"Status": "No API metrics available"}
	}

	return generateAPIStatsDisplay(node.api.SinceStart, 0, false, nil)
}

func (node *APISinceStartNode) GetMetricType() madmin.MetricType   { return madmin.MetricsAPI }
func (node *APISinceStartNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *APISinceStartNode) GetParent() MetricNode              { return node.parent }
func (node *APISinceStartNode) GetPath() string                    { return node.path }

func (node *APISinceStartNode) ShouldPauseRefresh() bool {
	return false
}

func (node *APISinceStartNode) GetChild(name string) (MetricNode, error) {
	return nil, fmt.Errorf("no children available for since_start node")
}

// APILastDayTotalNode shows the total last day statistics
type APILastDayTotalNode struct {
	api    *madmin.APIMetrics
	parent MetricNode
	path   string
}

func (node *APILastDayTotalNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *APILastDayTotalNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *APILastDayTotalNode) GetLeafData() map[string]string {
	if node.api == nil {
		return map[string]string{"Status": "No API metrics available"}
	}

	total := node.api.LastDayTotal()
	return generateAPIStatsDisplay(total, len(node.api.LastDayAPI), false, nil)
}

func (node *APILastDayTotalNode) GetMetricType() madmin.MetricType   { return madmin.MetricsAPI }
func (node *APILastDayTotalNode) GetMetricFlags() madmin.MetricFlags { return madmin.MetricsDayStats }
func (node *APILastDayTotalNode) GetParent() MetricNode              { return node.parent }
func (node *APILastDayTotalNode) GetPath() string                    { return node.path }

func (node *APILastDayTotalNode) ShouldPauseRefresh() bool {
	return true
}

func (node *APILastDayTotalNode) GetChild(name string) (MetricNode, error) {
	return nil, fmt.Errorf("no children available for last day total node")
}

// APITimeSegmentNode shows statistics for a specific time segment
// APITimeSegmentAllNode shows aggregated API statistics for a specific time segment
type APITimeSegmentAllNode struct {
	segment     madmin.APIStats
	segmentTime time.Time
	parent      MetricNode
	path        string
}

func (node *APITimeSegmentAllNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *APITimeSegmentAllNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *APITimeSegmentAllNode) GetChild(name string) (MetricNode, error) {
	return nil, fmt.Errorf("no children available for time segment")
}

func (node *APITimeSegmentAllNode) GetLeafData() map[string]string {
	return generateAPIStatsDisplay(node.segment, 1, false, nil)
}

func (node *APITimeSegmentAllNode) GetMetricType() madmin.MetricType   { return madmin.MetricsAPI }
func (node *APITimeSegmentAllNode) GetMetricFlags() madmin.MetricFlags { return madmin.MetricsDayStats }
func (node *APITimeSegmentAllNode) GetParent() MetricNode              { return node.parent }
func (node *APITimeSegmentAllNode) GetPath() string                    { return node.path }

func (node *APITimeSegmentAllNode) ShouldPauseRefresh() bool {
	return true
}

type APITimeSegmentNode struct {
	segment     madmin.APIStats
	segmentTime time.Time
	parent      MetricNode
	path        string
}

func (node *APITimeSegmentNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *APITimeSegmentNode) ShouldPauseRefresh() bool {
	return true
}

func (node *APITimeSegmentNode) GetChildren() []MetricChild {
	// Check if we have individual API data for this time segment
	// For now, we'll show "All" as the only option since we have aggregated data
	return []MetricChild{
		{
			Name:        "All",
			Description: fmt.Sprintf("All API endpoints combined for %s time segment", node.segmentTime.Local().Format("15:04")),
		},
	}
}

func (node *APITimeSegmentNode) GetLeafData() map[string]string {
	// This node now has children, so it should just show navigation info
	data := make(map[string]string)
	endTime := node.segmentTime.Add(time.Duration(15) * time.Minute) // Assume 15-minute intervals for now
	data["Time Range"] = fmt.Sprintf("%s -> %s",
		node.segmentTime.Local().Format("15:04"),
		endTime.Local().Format("15:04"))
	data["Total Requests"] = humanize.Comma(node.segment.Requests)
	data["Available APIs"] = "Select 'All' to view combined statistics"
	return data
}

func (node *APITimeSegmentNode) GetMetricType() madmin.MetricType   { return madmin.MetricsAPI }
func (node *APITimeSegmentNode) GetMetricFlags() madmin.MetricFlags { return madmin.MetricsDayStats }
func (node *APITimeSegmentNode) GetParent() MetricNode              { return node.parent }
func (node *APITimeSegmentNode) GetPath() string                    { return node.path }
func (node *APITimeSegmentNode) GetChild(name string) (MetricNode, error) {
	if name == "All" {
		return &APITimeSegmentAllNode{
			segment:     node.segment,
			segmentTime: node.segmentTime,
			parent:      node,
			path:        node.path + "/" + name,
		}, nil
	}
	return nil, fmt.Errorf("API selection not found: %s", name)
}

// APIEndpointNode shows detailed statistics for a specific endpoint
type APIEndpointNode struct {
	endpoint string
	stats    madmin.APIStats
	parent   MetricNode
	path     string
}

func (node *APIEndpointNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *APIEndpointNode) ShouldPauseRefresh() bool {
	return false
}

func (node *APIEndpointNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *APIEndpointNode) GetLeafData() map[string]string {
	if node.stats.Requests == 0 {
		data := make(map[string]string)
		data["Status"] = "No requests recorded for this endpoint"
		return data
	}

	// Use ordered slice to maintain consistent display order
	var entries []struct{ key, value string }

	// === ENDPOINT HEADER ===
	entries = append(entries, struct{ key, value string }{"Endpoint", node.endpoint})
	entries = append(entries, struct{ key, value string }{"Total Requests", humanize.Comma(node.stats.Requests)})

	// === BASIC METRICS ===
	entries = append(entries, struct{ key, value string }{"Responding Nodes", fmt.Sprintf("%d nodes", node.stats.Nodes)})

	// Calculate RPS if we have wall time
	if node.stats.WallTimeSecs > 0 {
		rps := float64(node.stats.Requests) / node.stats.WallTimeSecs
		entries = append(entries, struct{ key, value string }{"Avg RPS", fmt.Sprintf("%.1f req/sec", rps)})
	}

	// === TIMING METRICS ===
	avgLatency := (node.stats.RequestTimeSecs / float64(node.stats.Requests)) * 1000
	entries = append(entries, struct{ key, value string }{"Avg Latency", fmt.Sprintf("%.1f ms", avgLatency)})

	if node.stats.RespTTFBSecs > 0 {
		avgTTFB := (node.stats.RespTTFBSecs / float64(node.stats.Requests)) * 1000
		entries = append(entries, struct{ key, value string }{"Avg TTFB", fmt.Sprintf("%.1f ms", avgTTFB)})
	}

	// === TIMING RANGES (all 4 like main page) ===
	if node.stats.RequestTimeSecsMax > 0 {
		entries = append(entries, struct{ key, value string }{"Latency Range", fmt.Sprintf("%.1f - %.1f ms",
			node.stats.RequestTimeSecsMin*1000, node.stats.RequestTimeSecsMax*1000)})
	}
	if node.stats.RespTTFBSecsMax > 0 {
		entries = append(entries, struct{ key, value string }{"TTFB Range", fmt.Sprintf("%.1f - %.1f ms",
			node.stats.RespTTFBSecsMin*1000, node.stats.RespTTFBSecsMax*1000)})
	}
	if node.stats.ReqReadSecsMax > 0 {
		entries = append(entries, struct{ key, value string }{"Req Read Range", fmt.Sprintf("%.1f - %.1f ms",
			node.stats.ReqReadSecsMin*1000, node.stats.ReqReadSecsMax*1000)})
	}
	if node.stats.RespSecsMax > 0 {
		entries = append(entries, struct{ key, value string }{"Resp Wr Range", fmt.Sprintf("%.1f - %.1f ms",
			node.stats.RespSecsMin*1000, node.stats.RespSecsMax*1000)})
	}

	// === THROUGHPUT ===
	totalBytes := node.stats.IncomingBytes + node.stats.OutgoingBytes
	if totalBytes > 0 && node.stats.Requests > 0 {
		entries = append(entries, struct{ key, value string }{"Total Throughput", humanize.Bytes(uint64(totalBytes))})
		entries = append(entries, struct{ key, value string }{"-> Incoming", humanize.Bytes(uint64(node.stats.IncomingBytes))})
		entries = append(entries, struct{ key, value string }{"<- Outgoing", humanize.Bytes(uint64(node.stats.OutgoingBytes))})

		avgBytesPerReq := totalBytes / node.stats.Requests
		entries = append(entries, struct{ key, value string }{"Avg Bytes", humanize.Bytes(uint64(avgBytesPerReq)) + "/req"})
	}

	// === ERROR ANALYSIS ===
	stats := node.stats
	if stats.Requests > 0 {
		totalErrors := stats.Errors5xx + stats.Errors5xx
		if stats.Requests > 0 {
			errorRate := float64(totalErrors) / float64(stats.Requests) * 100
			entries = append(entries, struct{ key, value string }{"Error Rate", fmt.Sprintf("%.2f%% (%d errors)", errorRate, totalErrors)})
		}
		if totalErrors > 0 {
			if stats.Errors4xx > 0 {
				clientErrorRate := float64(stats.Errors4xx) / float64(stats.Requests) * 100
				entries = append(entries, struct{ key, value string }{"↳ 4xx Client Errors", fmt.Sprintf("%d (%.2f%%)",
					stats.Errors4xx, clientErrorRate)})
			}
			if stats.Errors5xx > 0 {
				serverErrorRate := float64(stats.Errors5xx) / float64(stats.Requests) * 100
				entries = append(entries, struct{ key, value string }{"↳ 5xx Server Errors", fmt.Sprintf("%d (%.2f%%)",
					stats.Errors5xx, serverErrorRate)})
			}
			if stats.Canceled > 0 {
				cancelRate := float64(stats.Canceled) / float64(stats.Requests) * 100
				entries = append(entries, struct{ key, value string }{"↳ Canceled Requests", fmt.Sprintf("%d (%.2f%%)", stats.Canceled, cancelRate)})
			}
		}
	}

	// === REJECTIONS ===
	totalRejected := node.stats.Rejected.Auth + node.stats.Rejected.Header +
		node.stats.Rejected.Invalid + node.stats.Rejected.NotImplemented +
		node.stats.Rejected.RequestsTime
	if totalRejected > 0 {
		rejectionRate := float64(totalRejected) / float64(node.stats.Requests) * 100
		entries = append(entries, struct{ key, value string }{"Rejected Requests", fmt.Sprintf("%d rejections (%.2f%%)", totalRejected, rejectionRate)})

		if node.stats.Rejected.Auth > 0 {
			authRate := float64(node.stats.Rejected.Auth) / float64(node.stats.Requests) * 100
			entries = append(entries, struct{ key, value string }{"↳ Authentication", fmt.Sprintf("%d (%.2f%%)", node.stats.Rejected.Auth, authRate)})
		}
		if node.stats.Rejected.Header > 0 {
			headerRate := float64(node.stats.Rejected.Header) / float64(node.stats.Requests) * 100
			entries = append(entries, struct{ key, value string }{"↳ Header Issues", fmt.Sprintf("%d (%.2f%%)", node.stats.Rejected.Header, headerRate)})
		}
		if node.stats.Rejected.Invalid > 0 {
			invalidRate := float64(node.stats.Rejected.Invalid) / float64(node.stats.Requests) * 100
			entries = append(entries, struct{ key, value string }{"↳ Invalid Requests", fmt.Sprintf("%d (%.2f%%)", node.stats.Rejected.Invalid, invalidRate)})
		}
		if node.stats.Rejected.NotImplemented > 0 {
			notImplRate := float64(node.stats.Rejected.NotImplemented) / float64(node.stats.Requests) * 100
			entries = append(entries, struct{ key, value string }{"↳ Not Implemented", fmt.Sprintf("%d (%.2f%%)", node.stats.Rejected.NotImplemented, notImplRate)})
		}
		if node.stats.Rejected.RequestsTime > 0 {
			timeRate := float64(node.stats.Rejected.RequestsTime) / float64(node.stats.Requests) * 100
			entries = append(entries, struct{ key, value string }{"↳ Outdated Signatures", fmt.Sprintf("%d (%.2f%%)", node.stats.Rejected.RequestsTime, timeRate)})
		}
	}

	// === BLOCKING ANALYSIS ===
	if node.stats.Requests > 0 && (node.stats.ReadBlockedSecs > 0 || node.stats.WriteBlockedSecs > 0) {
		if node.stats.ReadBlockedSecs > 0 {
			avgReadBlocked := (node.stats.ReadBlockedSecs / float64(node.stats.Requests)) * 1000
			entries = append(entries, struct{ key, value string }{"Avg Read Blocking", fmt.Sprintf("%.1f ms/req", avgReadBlocked)})
		}
		if node.stats.WriteBlockedSecs > 0 {
			avgWriteBlocked := (node.stats.WriteBlockedSecs / float64(node.stats.Requests)) * 1000
			entries = append(entries, struct{ key, value string }{"Avg Write Blocking", fmt.Sprintf("%.1f ms/req", avgWriteBlocked)})
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

func (node *APIEndpointNode) GetMetricType() madmin.MetricType   { return madmin.MetricsAPI }
func (node *APIEndpointNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *APIEndpointNode) GetParent() MetricNode              { return node.parent }
func (node *APIEndpointNode) GetPath() string                    { return node.path }
func (node *APIEndpointNode) GetChild(name string) (MetricNode, error) {
	return nil, fmt.Errorf("no children available for endpoint node")
}

// APISegmentedNode shows segmented statistics for a specific endpoint over the last day
type APISegmentedNode struct {
	endpoint  string
	segmented madmin.SegmentedAPIMetrics
	parent    MetricNode
	path      string
}

func (node *APISegmentedNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *APISegmentedNode) GetChildren() []MetricChild {
	var children []MetricChild
	for i := range node.segmented.Segments {
		children = append(children, MetricChild{
			Name:        fmt.Sprintf("segment_%d", i),
			Description: fmt.Sprintf("Time segment %d statistics", i),
		})
	}
	return children
}

func (node *APISegmentedNode) GetLeafData() map[string]string {
	data := make(map[string]string)

	data["Segment Count"] = fmt.Sprintf("%d segments", len(node.segmented.Segments))
	data["Interval"] = fmt.Sprintf("%d seconds", node.segmented.Interval)

	// Aggregate statistics
	var totalRequests int64
	var totalErrors int
	var totalBytes int64
	var totalLatency float64

	for _, segment := range node.segmented.Segments {
		totalRequests += segment.Requests
		totalErrors += segment.Errors4xx + segment.Errors5xx
		totalBytes += segment.IncomingBytes + segment.OutgoingBytes
		totalLatency += segment.RequestTimeSecs
	}

	if totalRequests > 0 {
		avgLatency := (totalLatency / float64(totalRequests)) * 1000
		data["Total Requests"] = humanize.Comma(totalRequests)
		data["Average Latency"] = fmt.Sprintf("%.1f ms", avgLatency)

		if totalErrors > 0 {
			errorRate := float64(totalErrors) / float64(totalRequests) * 100
			data["Error Rate"] = fmt.Sprintf("%.2f%% (%d errors)", errorRate, totalErrors)
		}

		if totalBytes > 0 {
			data["Total Throughput"] = humanize.Bytes(uint64(totalBytes))
		}

		// Calculate request rate per segment
		avgReqPerSegment := float64(totalRequests) / float64(len(node.segmented.Segments))
		data["Avg Requests/Segment"] = fmt.Sprintf("%.1f", avgReqPerSegment)
	}

	return data
}

func (node *APISegmentedNode) GetMetricType() madmin.MetricType   { return madmin.MetricsAPI }
func (node *APISegmentedNode) GetMetricFlags() madmin.MetricFlags { return madmin.MetricsDayStats }
func (node *APISegmentedNode) GetParent() MetricNode              { return node.parent }
func (node *APISegmentedNode) GetPath() string                    { return node.path }

func (node *APISegmentedNode) ShouldPauseRefresh() bool {
	return true
}

func (node *APISegmentedNode) GetChild(name string) (MetricNode, error) {
	return nil, fmt.Errorf("segmented endpoint children not yet implemented: %s", name)
}

// generateAPIOverviewDashboard creates a clean API performance dashboard
func (node *APIMetricsNode) generateAPIOverviewDashboard() map[string]string {
	if node.api == nil {
		return map[string]string{"Status": "No API metrics available"}
	}

	lastMinute := node.api.LastMinuteTotal()

	// Use ordered slice to maintain consistent display order
	var entries []struct{ key, value string }

	// === SYSTEM INFO ===
	entries = append(entries, struct{ key, value string }{"Active Nodes", fmt.Sprintf("%d nodes responding", node.api.Nodes)})
	entries = append(entries, struct{ key, value string }{"Collection Time", node.api.CollectedAt.Format("15:04:05")})

	// === QUEUE STATUS ===
	entries = append(entries, struct{ key, value string }{"Active Requests", humanize.Comma(node.api.ActiveRequests)})
	entries = append(entries, struct{ key, value string }{"Queued Requests", humanize.Comma(node.api.QueuedRequests)})
	totalQueue := node.api.ActiveRequests + node.api.QueuedRequests
	entries = append(entries, struct{ key, value string }{"Total Queue Depth", humanize.Comma(totalQueue)})

	// === REQUEST METRICS ===
	if lastMinute.Requests > 0 {
		entries = append(entries, struct{ key, value string }{"Request Rate", fmt.Sprintf("%s req/min", humanize.Comma(lastMinute.Requests))})
		rps := float64(lastMinute.Requests) / 60.0
		entries = append(entries, struct{ key, value string }{"Avg RPS", fmt.Sprintf("%.1f req/sec", rps)})

		avgLatency := (lastMinute.RequestTimeSecs / float64(lastMinute.Requests)) * 1000
		entries = append(entries, struct{ key, value string }{"Avg Latency", fmt.Sprintf("%.1f ms", avgLatency)})

		if lastMinute.RespTTFBSecs > 0 {
			avgTTFB := (lastMinute.RespTTFBSecs / float64(lastMinute.Requests)) * 1000
			entries = append(entries, struct{ key, value string }{"Avg TTFB", fmt.Sprintf("%.1f ms", avgTTFB)})
		}
	} else {
		entries = append(entries, struct{ key, value string }{"Request Rate", "No requests"})
		entries = append(entries, struct{ key, value string }{"Avg RPS", "0.0 req/sec"})
	}

	// === TIMING RANGES ===
	if lastMinute.RequestTimeSecsMax > 0 {
		entries = append(entries, struct{ key, value string }{"Latency Range", fmt.Sprintf("%.1f - %.1f ms",
			lastMinute.RequestTimeSecsMin*1000, lastMinute.RequestTimeSecsMax*1000)})
	}
	if lastMinute.RespTTFBSecsMax > 0 {
		entries = append(entries, struct{ key, value string }{"TTFB Range", fmt.Sprintf("%.1f - %.1f ms",
			lastMinute.RespTTFBSecsMin*1000, lastMinute.RespTTFBSecsMax*1000)})
	}
	if lastMinute.ReqReadSecsMax > 0 {
		entries = append(entries, struct{ key, value string }{"Req Read Range", fmt.Sprintf("%.1f - %.1f ms",
			lastMinute.ReqReadSecsMin*1000, lastMinute.ReqReadSecsMax*1000)})
	}
	if lastMinute.RespSecsMax > 0 {
		entries = append(entries, struct{ key, value string }{"Resp Wr Range", fmt.Sprintf("%.1f - %.1f ms",
			lastMinute.RespSecsMin*1000, lastMinute.RespSecsMax*1000)})
	}

	// === THROUGHPUT ===
	totalBytes := lastMinute.IncomingBytes + lastMinute.OutgoingBytes
	if totalBytes > 0 {
		entries = append(entries, struct{ key, value string }{"Throughput", fmt.Sprintf("%s/min", humanize.Bytes(uint64(totalBytes)))})
		entries = append(entries, struct{ key, value string }{"↳ Incoming", fmt.Sprintf("%s/min", humanize.Bytes(uint64(lastMinute.IncomingBytes)))})
		entries = append(entries, struct{ key, value string }{"↳ Outgoing", fmt.Sprintf("%s/min", humanize.Bytes(uint64(lastMinute.OutgoingBytes)))})
	}

	// === ERROR ANALYSIS ===
	totalErrors := lastMinute.Errors4xx + lastMinute.Errors5xx
	if totalErrors > 0 || lastMinute.Requests > 0 {
		errorRate := float64(totalErrors) / float64(lastMinute.Requests) * 100
		if lastMinute.Requests == 0 {
			errorRate = 0
		}
		entries = append(entries, struct{ key, value string }{"Error Rate", fmt.Sprintf("%.2f%% (%d errors)", errorRate, totalErrors)})

		if lastMinute.Errors4xx > 0 {
			clientErrorRate := float64(lastMinute.Errors4xx) / float64(lastMinute.Requests) * 100
			entries = append(entries, struct{ key, value string }{"↳ 4xx Client Errors", fmt.Sprintf("%d (%.2f%%)",
				lastMinute.Errors4xx, clientErrorRate)})
		}
		if lastMinute.Errors5xx > 0 {
			serverErrorRate := float64(lastMinute.Errors5xx) / float64(lastMinute.Requests) * 100
			entries = append(entries, struct{ key, value string }{"↳ 5xx Server Errors", fmt.Sprintf("%d (%.2f%%)",
				lastMinute.Errors5xx, serverErrorRate)})
		}
		if lastMinute.Canceled > 0 {
			cancelRate := float64(lastMinute.Canceled) / float64(lastMinute.Requests) * 100
			entries = append(entries, struct{ key, value string }{"↳ Canceled Requests", fmt.Sprintf("%d (%.2f%%)", lastMinute.Canceled, cancelRate)})
		}
	} else {
		entries = append(entries, struct{ key, value string }{"Error Rate", "No errors detected"})
	}

	// === REJECTIONS ===
	rejections := lastMinute.Rejected
	totalRejected := rejections.Auth + rejections.Header + rejections.Invalid + rejections.NotImplemented + rejections.RequestsTime
	if totalRejected > 0 {
		rejectionRate := float64(totalRejected) / float64(lastMinute.Requests) * 100
		entries = append(entries, struct{ key, value string }{"Rejected Requests", fmt.Sprintf("%d rejections (%.2f%%)", totalRejected, rejectionRate)})

		if rejections.Auth > 0 {
			authRate := float64(rejections.Auth) / float64(lastMinute.Requests) * 100
			entries = append(entries, struct{ key, value string }{"↳ Authentication", fmt.Sprintf("%d (%.2f%%)", rejections.Auth, authRate)})
		}
		if rejections.Header > 0 {
			headerRate := float64(rejections.Header) / float64(lastMinute.Requests) * 100
			entries = append(entries, struct{ key, value string }{"↳ Header Issues", fmt.Sprintf("%d (%.2f%%)", rejections.Header, headerRate)})
		}
		if rejections.Invalid > 0 {
			invalidRate := float64(rejections.Invalid) / float64(lastMinute.Requests) * 100
			entries = append(entries, struct{ key, value string }{"↳ Invalid Requests", fmt.Sprintf("%d (%.2f%%)", rejections.Invalid, invalidRate)})
		}
		if rejections.NotImplemented > 0 {
			notImplRate := float64(rejections.NotImplemented) / float64(lastMinute.Requests) * 100
			entries = append(entries, struct{ key, value string }{"↳ Not Implemented", fmt.Sprintf("%d (%.2f%%)", rejections.NotImplemented, notImplRate)})
		}
	}

	// === BLOCKING ANALYSIS ===
	if lastMinute.Requests > 0 && (lastMinute.ReadBlockedSecs > 0 || lastMinute.WriteBlockedSecs > 0) {
		if lastMinute.ReadBlockedSecs > 0 {
			avgReadBlocked := (lastMinute.ReadBlockedSecs / float64(lastMinute.Requests)) * 1000
			entries = append(entries, struct{ key, value string }{"Avg Read Blocking", fmt.Sprintf("%.1f ms/req", avgReadBlocked)})
		}
		if lastMinute.WriteBlockedSecs > 0 {
			avgWriteBlocked := (lastMinute.WriteBlockedSecs / float64(lastMinute.Requests)) * 1000
			entries = append(entries, struct{ key, value string }{"Avg Write Blocking", fmt.Sprintf("%.1f ms/req", avgWriteBlocked)})
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

// calculateAPIHealthScore computes health score based on error rates, latency, and queue status
func (node *APIMetricsNode) calculateAPIHealthScore(lastMinute *madmin.APIStats) float64 {
	score := 10.0

	if lastMinute.Requests == 0 {
		return 8.0 // Neutral score for no activity
	}

	// Error rate penalty
	errorRate := float64(lastMinute.Errors4xx+lastMinute.Errors5xx) / float64(lastMinute.Requests) * 100
	if errorRate > 5.0 {
		score -= 3.0
	} else if errorRate > 1.0 {
		score -= 1.0
	}

	// Latency penalty
	avgLatency := (lastMinute.RequestTimeSecs / float64(lastMinute.Requests)) * 1000
	if avgLatency > 5000 {
		score -= 2.0
	} else if avgLatency > 1000 {
		score -= 1.0
	}

	// Queue buildup penalty
	if node.api.QueuedRequests > 100 {
		score -= 2.0
	} else if node.api.QueuedRequests > 10 {
		score -= 1.0
	}

	// High rejection penalty
	totalRejected := lastMinute.Rejected.Auth + lastMinute.Rejected.Header +
		lastMinute.Rejected.Invalid + lastMinute.Rejected.NotImplemented + lastMinute.Rejected.RequestsTime
	if totalRejected > 0 {
		rejectionRate := float64(totalRejected) / float64(lastMinute.Requests) * 100
		if rejectionRate > 10.0 {
			score -= 2.0
		} else if rejectionRate > 5.0 {
			score -= 1.0
		}
	}

	if score < 0.0 {
		return 0.0
	}
	return score
}

// getHealthStatus returns health status string based on score
func (node *APIMetricsNode) getHealthStatus(score float64) string {
	switch {
	case score >= 9.0:
		return "🟢 EXCELLENT"
	case score >= 8.0:
		return "🟢 GOOD"
	case score >= 6.0:
		return "🟡 FAIR"
	case score >= 4.0:
		return "🟠 POOR"
	default:
		return "🔴 CRITICAL"
	}
}

// generateAPIRecommendations provides actionable insights
func (node *APIMetricsNode) generateAPIRecommendations(lastMinute, sinceStart *madmin.APIStats) []string {
	var recommendations []string

	if lastMinute.Requests == 0 {
		return []string{"No recent API activity to analyze"}
	}

	// Error rate recommendations
	errorRate := float64(lastMinute.Errors4xx+lastMinute.Errors5xx) / float64(lastMinute.Requests) * 100
	if errorRate > 5.0 {
		recommendations = append(recommendations, "High error rate detected - investigate failing endpoints")
	}

	if lastMinute.Errors5xx > lastMinute.Errors4xx && lastMinute.Errors5xx > 0 {
		recommendations = append(recommendations, "Server errors exceed client errors - check system health")
	}

	// Latency recommendations
	avgLatency := (lastMinute.RequestTimeSecs / float64(lastMinute.Requests)) * 1000
	if avgLatency > 2000 {
		recommendations = append(recommendations, "High average latency - consider performance optimization")
	}

	if lastMinute.RequestTimeSecsMax > 0 && lastMinute.RequestTimeSecsMax*1000 > avgLatency*5 {
		recommendations = append(recommendations, "High latency variance detected - investigate slow endpoints")
	}

	// Queue recommendations
	if node.api.QueuedRequests > 50 {
		recommendations = append(recommendations, "Request queue buildup - consider scaling or load balancing")
	}

	// TTFB recommendations
	if lastMinute.RespTTFBSecs > 0 && lastMinute.Requests > 0 {
		avgTTFB := (lastMinute.RespTTFBSecs / float64(lastMinute.Requests)) * 1000
		if avgTTFB > 500 {
			recommendations = append(recommendations, "Slow time-to-first-byte - optimize request processing")
		}
	}

	// Rejection recommendations
	rejections := lastMinute.Rejected
	if rejections.Auth > 0 {
		recommendations = append(recommendations, "Authentication failures detected - verify client credentials")
	}

	if rejections.Invalid > 0 {
		recommendations = append(recommendations, "Invalid request signatures - check client request formatting")
	}

	// Throughput recommendations
	if len(node.api.LastMinuteAPI) > 10 {
		recommendations = append(recommendations, "High endpoint diversity - monitor for unused or deprecated APIs")
	}

	return recommendations
}
