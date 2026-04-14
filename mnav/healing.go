// Copyright (c) 2015-2026 MinIO, Inc.
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

// HealingMetricsNode handles navigation for HealingMetrics.
type HealingMetricsNode struct {
	healing *madmin.HealingMetrics
	parent  MetricNode
	path    string
}

// NewHealingMetricsNode creates a new HealingMetricsNode.
func NewHealingMetricsNode(healing *madmin.HealingMetrics, parent MetricNode, path string) *HealingMetricsNode {
	return &HealingMetricsNode{healing: healing, parent: parent, path: path}
}

func (node *HealingMetricsNode) GetChildren() []MetricChild {
	if node.healing == nil {
		return []MetricChild{}
	}
	return []MetricChild{
		{Name: "last_minute", Description: "Rolling 60-second totals"},
		{Name: "last_hour", Description: "Rolling 60-minute totals"},
		{Name: "last_day", Description: "Last 24h in 15-minute segments"},
		{Name: "since_start", Description: "Accumulated all-time totals"},
		{Name: "buckets_minute", Description: "Per-bucket outcomes (last minute)"},
		{Name: "buckets_hour", Description: "Per-bucket outcomes (last hour)"},
	}
}

func (node *HealingMetricsNode) GetLeafData() map[string]string {
	if node.healing == nil {
		return map[string]string{"Status": "No healing metrics available"}
	}
	h := node.healing
	data := map[string]string{}

	data["00:Nodes reporting"] = fmt.Sprintf("%d", h.Nodes)
	data["01:Last updated"] = h.CollectedAt.Format("15:04:05")

	// Last minute summary
	lm := &h.LastMinute
	if lm.Started > 0 {
		data["03:Last minute"] = formatHealSummary(lm)
	} else {
		data["03:Last minute"] = "No activity"
	}

	// Last hour summary
	lh := &h.LastHour
	if lh.Started > 0 {
		data["04:Last hour"] = formatHealSummary(lh)
	} else {
		data["04:Last hour"] = "No activity"
	}

	// Since start summary
	ss := &h.SinceStart
	if ss.Started > 0 {
		data["05:All time"] = formatHealSummary(ss)
	}

	return data
}

func (node *HealingMetricsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsHealing }
func (node *HealingMetricsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *HealingMetricsNode) GetParent() MetricNode              { return node.parent }
func (node *HealingMetricsNode) GetPath() string                    { return node.path }
func (node *HealingMetricsNode) ShouldPauseRefresh() bool           { return false }
func (node *HealingMetricsNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }

func (node *HealingMetricsNode) GetChild(name string) (MetricNode, error) {
	if node.healing == nil {
		return nil, fmt.Errorf("no healing data available")
	}
	switch name {
	case "last_minute":
		return NewHealingCountsNode(&node.healing.LastMinute, node, node.path+"/last_minute"), nil
	case "last_hour":
		return NewHealingCountsNode(&node.healing.LastHour, node, node.path+"/last_hour"), nil
	case "since_start":
		return NewHealingCountsNode(&node.healing.SinceStart, node, node.path+"/since_start"), nil
	case "last_day":
		return NewHealingLastDayNode(node.healing.LastDay, node, node.path+"/last_day"), nil
	case "buckets_minute":
		return NewHealingBucketsNode(node.healing.BucketsLastMinute, node, node.path+"/buckets_minute"), nil
	case "buckets_hour":
		return NewHealingBucketsNode(node.healing.BucketsLastHour, node, node.path+"/buckets_hour"), nil
	default:
		return nil, fmt.Errorf("child not found: %s", name)
	}
}

// HealingCountsNode displays a single HealingCounts snapshot.
type HealingCountsNode struct {
	counts *madmin.HealingCounts
	parent MetricNode
	path   string
}

// NewHealingCountsNode creates a new HealingCountsNode.
func NewHealingCountsNode(counts *madmin.HealingCounts, parent MetricNode, path string) *HealingCountsNode {
	return &HealingCountsNode{counts: counts, parent: parent, path: path}
}

func (node *HealingCountsNode) GetChildren() []MetricChild { return []MetricChild{} }

func (node *HealingCountsNode) GetLeafData() map[string]string {
	c := node.counts
	if c == nil {
		return map[string]string{"Status": "No data"}
	}
	data := map[string]string{}

	data["00:Started"] = humanize.Comma(c.Started)
	data["01:Completed"] = humanize.Comma(c.Completed)
	data["02:Failed"] = humanize.Comma(c.Failed)
	data["03:Bytes processed"] = humanize.Bytes(uint64(c.Bytes))
	data["04:Bytes healed"] = humanize.Bytes(uint64(c.BytesCompleted))

	if c.Completed > 0 && c.AccTime > 0 {
		avg := c.AccTime / float64(c.Completed)
		data["05:Avg heal time"] = fmt.Sprintf("%.2fms", avg*1000)
		data["06:Total heal time"] = fmt.Sprintf("%.1fs", c.AccTime)
	}

	if c.Dangling > 0 {
		data["07:Dangling cleaned"] = humanize.Comma(c.Dangling)
	}
	if c.WarmTierChecks > 0 {
		data["08:Warm tier checks"] = humanize.Comma(c.WarmTierChecks)
	}

	if len(c.ByOrigin) > 0 {
		for k, v := range c.ByOrigin {
			data["10:Origin/"+k] = humanize.Comma(v)
		}
	}
	if len(c.ByType) > 0 {
		for k, v := range c.ByType {
			data["11:Type/"+k] = humanize.Comma(v)
		}
	}
	if len(c.ByError) > 0 {
		for k, v := range c.ByError {
			data["12:Error/"+k] = humanize.Comma(v)
		}
	}
	return data
}

func (node *HealingCountsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsHealing }
func (node *HealingCountsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *HealingCountsNode) GetParent() MetricNode              { return node.parent }
func (node *HealingCountsNode) GetPath() string                    { return node.path }
func (node *HealingCountsNode) ShouldPauseRefresh() bool           { return false }
func (node *HealingCountsNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }

func (node *HealingCountsNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("healing counts is a leaf node")
}

// HealingLastDayNode navigates the 24h segmented healing data.
type HealingLastDayNode struct {
	lastDay *madmin.SegmentedHealingStats
	parent  MetricNode
	path    string
}

// NewHealingLastDayNode creates a new HealingLastDayNode.
func NewHealingLastDayNode(lastDay *madmin.SegmentedHealingStats, parent MetricNode, path string) *HealingLastDayNode {
	return &HealingLastDayNode{lastDay: lastDay, parent: parent, path: path}
}

func (node *HealingLastDayNode) GetChildren() []MetricChild {
	if node.lastDay == nil || len(node.lastDay.Segments) == 0 {
		return []MetricChild{}
	}
	children := []MetricChild{{Name: "total", Description: "Aggregated totals across all segments"}}
	for i := len(node.lastDay.Segments) - 1; i >= 0; i-- {
		seg := node.lastDay.Segments[i]
		if seg.Started == 0 {
			continue
		}
		segTime := node.lastDay.FirstTime.Add(time.Duration(i*node.lastDay.Interval) * time.Second)
		name := segTime.UTC().Format("15:04Z")
		children = append(children, MetricChild{
			Name:        name,
			Description: fmt.Sprintf("%s started, %s completed", humanize.Comma(seg.Started), humanize.Comma(seg.Completed)),
		})
	}
	return children
}

func (node *HealingLastDayNode) GetLeafData() map[string]string {
	if node.lastDay == nil || len(node.lastDay.Segments) == 0 {
		return map[string]string{"Status": "No last day statistics available"}
	}
	// Show totals across all segments
	var total madmin.HealingCounts
	for i := range node.lastDay.Segments {
		total.Add(&node.lastDay.Segments[i])
	}
	data := map[string]string{
		"00:Segments": fmt.Sprintf("%d (every %ds)", len(node.lastDay.Segments), node.lastDay.Interval),
	}
	if total.Started > 0 {
		data["01:Total"] = formatHealSummary(&total)
	}
	return data
}

func (node *HealingLastDayNode) GetMetricType() madmin.MetricType { return madmin.MetricsHealing }
func (node *HealingLastDayNode) GetMetricFlags() madmin.MetricFlags {
	return madmin.MetricsDayStats
}
func (node *HealingLastDayNode) GetParent() MetricNode          { return node.parent }
func (node *HealingLastDayNode) GetPath() string                { return node.path }
func (node *HealingLastDayNode) ShouldPauseRefresh() bool       { return true }
func (node *HealingLastDayNode) GetOpts() madmin.MetricsOptions { return getNodeOpts(node) }

func (node *HealingLastDayNode) GetChild(name string) (MetricNode, error) {
	if node.lastDay == nil {
		return nil, fmt.Errorf("no last day data available")
	}
	if name == "total" {
		var total madmin.HealingCounts
		for i := range node.lastDay.Segments {
			total.Add(&node.lastDay.Segments[i])
		}
		return NewHealingCountsNode(&total, node, node.path+"/total"), nil
	}
	for i := range node.lastDay.Segments {
		segTime := node.lastDay.FirstTime.Add(time.Duration(i*node.lastDay.Interval) * time.Second)
		if segTime.UTC().Format("15:04Z") == name {
			return NewHealingCountsNode(&node.lastDay.Segments[i], node, node.path+"/"+name), nil
		}
	}
	return nil, fmt.Errorf("segment not found: %s", name)
}

// HealingBucketsNode navigates per-bucket healing outcomes.
type HealingBucketsNode struct {
	buckets map[string]madmin.HealBucketStats
	parent  MetricNode
	path    string
}

// NewHealingBucketsNode creates a new HealingBucketsNode.
func NewHealingBucketsNode(buckets map[string]madmin.HealBucketStats, parent MetricNode, path string) *HealingBucketsNode {
	return &HealingBucketsNode{buckets: buckets, parent: parent, path: path}
}

func (node *HealingBucketsNode) GetChildren() []MetricChild {
	if len(node.buckets) == 0 {
		return []MetricChild{}
	}
	names := make([]string, 0, len(node.buckets))
	for k := range node.buckets {
		names = append(names, k)
	}
	sort.Strings(names)

	children := make([]MetricChild, 0, len(names))
	for _, name := range names {
		bs := node.buckets[name]
		children = append(children, MetricChild{
			Name:        name,
			Description: fmt.Sprintf("%d started, %d ok, %d failed", bs.Started, bs.Completed, bs.Failed),
		})
	}
	return children
}

func (node *HealingBucketsNode) GetLeafData() map[string]string {
	if len(node.buckets) == 0 {
		return map[string]string{"Status": "No per-bucket data"}
	}
	data := map[string]string{
		"Buckets": fmt.Sprintf("%d", len(node.buckets)),
	}
	var totalStarted, totalCompleted, totalFailed int64
	for _, bs := range node.buckets {
		totalStarted += bs.Started
		totalCompleted += bs.Completed
		totalFailed += bs.Failed
	}
	data["Total started"] = humanize.Comma(totalStarted)
	data["Total completed"] = humanize.Comma(totalCompleted)
	if totalFailed > 0 {
		data["Total failed"] = humanize.Comma(totalFailed)
	}
	return data
}

func (node *HealingBucketsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsHealing }
func (node *HealingBucketsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *HealingBucketsNode) GetParent() MetricNode              { return node.parent }
func (node *HealingBucketsNode) GetPath() string                    { return node.path }
func (node *HealingBucketsNode) ShouldPauseRefresh() bool           { return false }
func (node *HealingBucketsNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }

func (node *HealingBucketsNode) GetChild(name string) (MetricNode, error) {
	bs, ok := node.buckets[name]
	if !ok {
		return nil, fmt.Errorf("bucket not found: %s", name)
	}
	return &HealingBucketLeafNode{
		name: name, stats: &bs,
		parent: node, path: node.path + "/" + name,
	}, nil
}

// HealingBucketLeafNode displays stats for a single bucket.
type HealingBucketLeafNode struct {
	name   string
	stats  *madmin.HealBucketStats
	parent MetricNode
	path   string
}

func (node *HealingBucketLeafNode) GetChildren() []MetricChild { return []MetricChild{} }

func (node *HealingBucketLeafNode) GetLeafData() map[string]string {
	return map[string]string{
		"Bucket":    node.name,
		"Started":   humanize.Comma(node.stats.Started),
		"Completed": humanize.Comma(node.stats.Completed),
		"Failed":    humanize.Comma(node.stats.Failed),
	}
}

func (node *HealingBucketLeafNode) GetMetricType() madmin.MetricType   { return madmin.MetricsHealing }
func (node *HealingBucketLeafNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *HealingBucketLeafNode) GetParent() MetricNode              { return node.parent }
func (node *HealingBucketLeafNode) GetPath() string                    { return node.path }
func (node *HealingBucketLeafNode) ShouldPauseRefresh() bool           { return false }
func (node *HealingBucketLeafNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }

func (node *HealingBucketLeafNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("bucket stats is a leaf node")
}

// formatHealSummary returns a one-line summary of a HealingCounts.
func formatHealSummary(c *madmin.HealingCounts) string {
	s := fmt.Sprintf("%s started, %s ok, %s failed, %s",
		humanize.Comma(c.Started),
		humanize.Comma(c.Completed),
		humanize.Comma(c.Failed),
		humanize.Bytes(uint64(c.BytesCompleted)))
	if c.Completed > 0 && c.AccTime > 0 {
		avg := c.AccTime / float64(c.Completed) * 1000
		s += fmt.Sprintf(", %.1fms avg", avg)
	}
	return s
}
