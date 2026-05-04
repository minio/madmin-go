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
	"strings"
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
		{Name: "last_minute", Description: "Rolling 1 minute total"},
		{Name: "last_hour", Description: "Rolling 1 hour total"},
		{Name: "last_day", Description: "Last 24h in 15-minute segments"},
		{Name: "since_start", Description: "Accumulated all-time totals"},
		{Name: "buckets_minute", Description: "Per-bucket outcomes (last minute)"},
		{Name: "buckets_hour", Description: "Per-bucket outcomes (last hour)"},
		{Name: "active_sessions", Description: fmt.Sprintf("Manual heal sessions (%d recently active)", len(node.healing.ActiveSessions))},
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
	if len(h.ActiveSessions) > 0 {
		data["02:Active sessions"] = fmt.Sprintf("%d", len(h.ActiveSessions))
	}

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
	case "active_sessions":
		return NewHealSessionsNode(node.healing.ActiveSessions, node, node.path+"/active_sessions"), nil
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
	data["02:Healed"] = humanize.Comma(c.Healed)
	data["03:Failed"] = humanize.Comma(c.Failed)
	data["04:Bytes processed"] = humanize.Bytes(uint64(c.Bytes))
	data["05:Bytes completed"] = humanize.Bytes(uint64(c.BytesCompleted))
	data["06:Bytes healed"] = humanize.Bytes(uint64(c.BytesHealed))

	if c.Completed > 0 && c.AccTime > 0 {
		avg := c.AccTime / float64(c.Completed)
		data["07:Avg heal time"] = fmt.Sprintf("%.2fms", avg*1000)
		data["08:Total heal time"] = fmt.Sprintf("%.1fs", c.AccTime)
	}

	if c.Dangling > 0 {
		data["09:Dangling cleaned"] = humanize.Comma(c.Dangling)
	}
	if c.WarmTierChecks > 0 {
		data["10:Warm tier checks"] = humanize.Comma(c.WarmTierChecks)
	}

	if len(c.ByOrigin) > 0 {
		for k, v := range c.ByOrigin {
			data["20:Origin/"+k] = humanize.Comma(v)
		}
	}
	if len(c.ByType) > 0 {
		for k, v := range c.ByType {
			data["21:Type/"+k] = humanize.Comma(v)
		}
	}
	if len(c.ByError) > 0 {
		for k, v := range c.ByError {
			data["22:Error/"+k] = humanize.Comma(v)
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

// HealSessionsNode navigates active heal sessions.
type HealSessionsNode struct {
	sessions map[string]madmin.HealSession
	parent   MetricNode
	path     string
}

// NewHealSessionsNode creates a new HealSessionsNode.
func NewHealSessionsNode(sessions map[string]madmin.HealSession, parent MetricNode, path string) *HealSessionsNode {
	return &HealSessionsNode{sessions: sessions, parent: parent, path: path}
}

func (node *HealSessionsNode) GetChildren() []MetricChild {
	if len(node.sessions) == 0 {
		return []MetricChild{}
	}
	tokens := make([]string, 0, len(node.sessions))
	for k := range node.sessions {
		tokens = append(tokens, k)
	}
	sort.Slice(tokens, func(i, j int) bool {
		return node.sessions[tokens[i]].StartTime.After(node.sessions[tokens[j]].StartTime)
	})

	children := make([]MetricChild, 0, len(tokens))
	for _, token := range tokens {
		s := node.sessions[token]
		desc := s.StartTime.Format("2006-01-02 15:04:05")
		if s.Bucket != "" {
			desc += " " + s.Bucket
			if s.Prefix != "" {
				desc += "/" + s.Prefix
			}
		}
		desc += " [" + s.Status + "]"
		children = append(children, MetricChild{
			Name:        token,
			Description: desc,
		})
	}
	return children
}

func (node *HealSessionsNode) GetLeafData() map[string]string {
	if len(node.sessions) == 0 {
		return map[string]string{"Status": "No active sessions"}
	}
	return map[string]string{"Sessions": fmt.Sprintf("%d", len(node.sessions))}
}

func (node *HealSessionsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsHealing }
func (node *HealSessionsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *HealSessionsNode) GetParent() MetricNode              { return node.parent }
func (node *HealSessionsNode) GetPath() string                    { return node.path }
func (node *HealSessionsNode) ShouldPauseRefresh() bool           { return false }
func (node *HealSessionsNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }

func (node *HealSessionsNode) GetChild(name string) (MetricNode, error) {
	s, ok := node.sessions[name]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", name)
	}
	return &HealSessionLeafNode{
		token: name, session: &s,
		parent: node, path: node.path + "/" + name,
	}, nil
}

// HealSessionLeafNode displays details for a single heal session.
type HealSessionLeafNode struct {
	token   string
	session *madmin.HealSession
	parent  MetricNode
	path    string
}

func (node *HealSessionLeafNode) GetChildren() []MetricChild { return []MetricChild{} }

func (node *HealSessionLeafNode) GetLeafData() map[string]string {
	s := node.session
	status := strings.ToUpper(s.Status[:1]) + s.Status[1:]
	if len(s.ScannedItems) > 0 && !s.StartTime.IsZero() && !s.LastActivity.IsZero() {
		if elapsed := s.LastActivity.Sub(s.StartTime).Minutes(); elapsed > 0 {
			var total int64
			for _, n := range s.ScannedItems {
				total += n
			}
			status += fmt.Sprintf(" (%.1f items/min)", float64(total)/elapsed)
		}
	}
	data := map[string]string{
		"00:Status": status,
		"01:Token":  node.token,
	}
	if s.Bucket != "" {
		scope := s.Bucket
		if s.Prefix != "" {
			scope += "/" + s.Prefix
		}
		data["02:Scope"] = scope
	}
	data["03:Started"] = s.StartTime.Format("2006-01-02 15:04:05")
	if !s.EndTime.IsZero() {
		data["04:Ended"] = s.EndTime.Format("2006-01-02 15:04:05")
		data["05:Duration"] = s.EndTime.Sub(s.StartTime).Round(time.Second).String()
	} else if !s.StartTime.IsZero() {
		data["05:Duration"] = time.Since(s.StartTime).Round(time.Second).String() + " (running)"
	}
	if !s.LastActivity.IsZero() {
		data["06:Last activity"] = s.LastActivity.Format("15:04:05")
	}

	// Settings
	var flags []string
	if s.Settings.Recursive {
		flags = append(flags, "recursive")
	}
	if s.Settings.DryRun {
		flags = append(flags, "dry-run")
	}
	if s.Settings.Remove {
		flags = append(flags, "remove")
	}
	if s.Settings.Recreate {
		flags = append(flags, "recreate")
	}
	if s.Settings.UpdateParity {
		flags = append(flags, "update-parity")
	}
	if s.Settings.NoLock {
		flags = append(flags, "no-lock")
	}
	if s.Settings.CrossPool {
		flags = append(flags, "cross-pool")
	}
	if len(flags) > 0 {
		data["10:Flags"] = strings.Join(flags, ", ")
	}
	switch s.Settings.ScanMode {
	case madmin.HealDeepScan:
		data["11:Scan mode"] = "deep"
	case madmin.HealNormalScan:
		data["11:Scan mode"] = "normal"
	}
	if s.Settings.Pool != nil {
		v := fmt.Sprintf("%d", *s.Settings.Pool)
		if s.Settings.Set != nil {
			v += fmt.Sprintf(", set %d", *s.Settings.Set)
		}
		data["12:Pool"] = v
	}

	// Item counters
	if len(s.ScannedItems) > 0 {
		data["20:Scanned"] = formatItemCounts(s.ScannedItems)
	}
	if len(s.HealedItems) > 0 {
		data["21:Healed"] = formatItemCounts(s.HealedItems)
	} else {
		data["21:Healed"] = "None"
	}
	if len(s.FailedItems) > 0 {
		data["22:Failed"] = formatItemCounts(s.FailedItems)
	} else {
		data["22:Failed"] = "None"
	}
	return data
}

func (node *HealSessionLeafNode) GetMetricType() madmin.MetricType   { return madmin.MetricsHealing }
func (node *HealSessionLeafNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *HealSessionLeafNode) GetParent() MetricNode              { return node.parent }
func (node *HealSessionLeafNode) GetPath() string                    { return node.path }
func (node *HealSessionLeafNode) ShouldPauseRefresh() bool           { return false }
func (node *HealSessionLeafNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }

func (node *HealSessionLeafNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("heal session is a leaf node")
}

// formatItemCounts formats a map of item-type counts into a sorted single line.
func formatItemCounts(items map[madmin.HealItemType]int64) string {
	keys := make([]string, 0, len(items))
	for k := range items {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		name := strings.ToUpper(k[:1]) + k[1:]
		parts = append(parts, fmt.Sprintf("%s: %s", name, humanize.Comma(items[k])))
	}
	return strings.Join(parts, ", ")
}

// formatHealSummary returns a one-line summary of a HealingCounts.
func formatHealSummary(c *madmin.HealingCounts) string {
	s := fmt.Sprintf("%s started, %s ok, %s healed, %s failed, %s",
		humanize.Comma(c.Started),
		humanize.Comma(c.Completed),
		humanize.Comma(c.Healed),
		humanize.Comma(c.Failed),
		humanize.Bytes(uint64(c.BytesCompleted)))
	if c.Completed > 0 && c.AccTime > 0 {
		avg := c.AccTime / float64(c.Completed) * 1000
		s += fmt.Sprintf(", %.1fms avg", avg)
	}
	return s
}
