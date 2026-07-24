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
	"strconv"
	"time"

	"github.com/minio/madmin-go/v4"
)

// KMSMetricsNode represents the root KMS metrics node.
type KMSMetricsNode struct {
	kms    *madmin.KMSRtMetrics
	parent MetricNode
	path   string
}

// NewKMSMetricsNode creates a new KMSMetricsNode.
func NewKMSMetricsNode(kms *madmin.KMSRtMetrics, parent MetricNode, path string) *KMSMetricsNode {
	return &KMSMetricsNode{kms: kms, parent: parent, path: path}
}

func (node *KMSMetricsNode) GetOpts() madmin.MetricsOptions { return getNodeOpts(node) }
func (node *KMSMetricsNode) GetMetricType() madmin.MetricType {
	return madmin.MetricsKMS
}

func (node *KMSMetricsNode) GetMetricFlags() madmin.MetricFlags {
	return 0
}

func (node *KMSMetricsNode) GetParent() MetricNode    { return node.parent }
func (node *KMSMetricsNode) GetPath() string          { return node.path }
func (node *KMSMetricsNode) ShouldPauseRefresh() bool { return false }

func (node *KMSMetricsNode) GetChildren() []MetricChild {
	if node.kms == nil {
		return []MetricChild{}
	}
	return []MetricChild{
		{Name: "last_minute", Description: "Per-operation stats for the last minute"},
		{Name: "last_hour", Description: "Per-operation summary for the last hour"},
		{Name: "last_day", Description: "Per-operation stats segmented by 15 minutes"},
	}
}

func (node *KMSMetricsNode) GetLeafData() map[string]string {
	if node.kms == nil {
		return map[string]string{"Status": "KMS not configured"}
	}
	data := map[string]string{}
	data["00:Cluster"] = fmt.Sprintf("%d/%d nodes online", node.kms.NodesOnline, node.kms.Nodes)
	if node.kms.OnlineSecs > 0 {
		data["01:Online Duration"] = (time.Duration(node.kms.OnlineSecs) * time.Second).String()
	}
	if node.kms.LastSuccess != nil {
		data["02:Last Success"] = node.kms.LastSuccess.Format(time.RFC3339)
	}
	if node.kms.ActiveOps > 0 {
		data["03:Active Operations"] = strconv.FormatInt(node.kms.ActiveOps, 10)
	}

	// Last minute summary
	if len(node.kms.LastMinute) > 0 {
		var totalCount uint64
		var totalErrs uint64
		for _, a := range node.kms.LastMinute {
			totalCount += a.Count
			totalErrs += a.ConnFails + a.RemoteErrs
		}
		summary := strconv.FormatUint(totalCount, 10) + " calls"
		if totalErrs > 0 {
			summary += ", " + strconv.FormatUint(totalErrs, 10) + " errors"
		}
		data["04:Last Minute"] = summary
	}
	return data
}

func (node *KMSMetricsNode) GetChild(name string) (MetricNode, error) {
	if node.kms == nil {
		return nil, fmt.Errorf("no KMS data available")
	}
	switch name {
	case "last_minute":
		return &kmsLastMinuteNode{kms: node.kms, parent: node, path: node.path + "/last_minute"}, nil
	case "last_hour":
		return &kmsSummaryNode{
			data:   node.kms.LastHour,
			label:  "hour",
			flags:  madmin.MetricsHourStats,
			parent: node,
			path:   node.path + "/last_hour",
		}, nil
	case "last_day":
		return &kmsSegmentedNode{
			data:   node.kms.LastDay,
			parent: node,
			path:   node.path + "/last_day",
		}, nil
	default:
		return nil, fmt.Errorf("unknown KMS child: %s", name)
	}
}

// kmsLastMinuteNode shows per-operation stats for the last minute.
type kmsLastMinuteNode struct {
	kms    *madmin.KMSRtMetrics
	parent MetricNode
	path   string
}

func (node *kmsLastMinuteNode) GetOpts() madmin.MetricsOptions { return getNodeOpts(node) }
func (node *kmsLastMinuteNode) GetMetricType() madmin.MetricType {
	return madmin.MetricsKMS
}
func (node *kmsLastMinuteNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *kmsLastMinuteNode) GetParent() MetricNode              { return node.parent }
func (node *kmsLastMinuteNode) GetPath() string                    { return node.path }
func (node *kmsLastMinuteNode) ShouldPauseRefresh() bool           { return false }
func (node *kmsLastMinuteNode) GetChildren() []MetricChild         { return []MetricChild{} }

func (node *kmsLastMinuteNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children")
}

func (node *kmsLastMinuteNode) GetLeafData() map[string]string {
	if node.kms == nil || len(node.kms.LastMinute) == 0 {
		return map[string]string{"Status": "No KMS requests recorded"}
	}

	data := map[string]string{}
	ops := make([]string, 0, len(node.kms.LastMinute))
	for op := range node.kms.LastMinute {
		ops = append(ops, op)
	}
	sort.Strings(ops)

	for i, op := range ops {
		a := node.kms.LastMinute[op]
		prefix := fmt.Sprintf("%02d:%s", i, op)
		avg := a.Avg()
		rps := float64(a.Count) / 60
		line := fmt.Sprintf("%.1f req/s, %d calls, avg %v, min %v, max %v",
			rps, a.Count,
			avg.Round(time.Microsecond),
			time.Duration(a.MinTime*float64(time.Second)).Round(time.Microsecond),
			time.Duration(a.MaxTime*float64(time.Second)).Round(time.Microsecond),
		)
		if a.ConnFails > 0 {
			line += fmt.Sprintf(", %d conn fails", a.ConnFails)
		}
		if a.RemoteErrs > 0 {
			line += fmt.Sprintf(", %d remote errs", a.RemoteErrs)
		}
		data[prefix] = line
	}
	return data
}

// kmsSummaryNode shows per-operation totals as leaf data only (no children).
type kmsSummaryNode struct {
	data   map[string]madmin.SegmentedKMSActions
	label  string
	flags  madmin.MetricFlags
	parent MetricNode
	path   string
}

func (node *kmsSummaryNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *kmsSummaryNode) GetMetricType() madmin.MetricType   { return madmin.MetricsKMS }
func (node *kmsSummaryNode) GetMetricFlags() madmin.MetricFlags { return node.flags }
func (node *kmsSummaryNode) GetParent() MetricNode              { return node.parent }
func (node *kmsSummaryNode) GetPath() string                    { return node.path }
func (node *kmsSummaryNode) ShouldPauseRefresh() bool           { return false }
func (node *kmsSummaryNode) GetChildren() []MetricChild         { return []MetricChild{} }

func (node *kmsSummaryNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children")
}

func (node *kmsSummaryNode) GetLeafData() map[string]string {
	return kmsSegmentedLeafData(node.data, node.label, 3600)
}

// kmsSegmentedNode shows per-operation totals as leaf data with
// navigable children for each operation's time segments.
type kmsSegmentedNode struct {
	data   map[string]madmin.SegmentedKMSActions
	parent MetricNode
	path   string
}

func (node *kmsSegmentedNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *kmsSegmentedNode) GetMetricType() madmin.MetricType   { return madmin.MetricsKMS }
func (node *kmsSegmentedNode) GetMetricFlags() madmin.MetricFlags { return madmin.MetricsDayStats }
func (node *kmsSegmentedNode) GetParent() MetricNode              { return node.parent }
func (node *kmsSegmentedNode) GetPath() string                    { return node.path }
func (node *kmsSegmentedNode) ShouldPauseRefresh() bool           { return false }

func (node *kmsSegmentedNode) GetChildren() []MetricChild {
	if len(node.data) == 0 {
		return []MetricChild{}
	}
	// _ALL first, then individual ops.
	var totalCalls uint64
	for _, s := range node.data {
		totalCalls += s.Total().Count
	}
	children := []MetricChild{
		{Name: byTimeName, Description: "Browse by time segment (all operations)"},
		{Name: "_ALL", Description: fmt.Sprintf("All operations combined (%d calls)", totalCalls)},
	}
	ops := sortedKeys(node.data)
	for _, op := range ops {
		s := node.data[op]
		total := s.Total()
		desc := fmt.Sprintf("%d calls", total.Count)
		if total.ConnFails+total.RemoteErrs > 0 {
			desc += fmt.Sprintf(", %d errors", total.ConnFails+total.RemoteErrs)
		}
		children = append(children, MetricChild{Name: op, Description: desc})
	}
	return children
}

func (node *kmsSegmentedNode) GetLeafData() map[string]string {
	return kmsSegmentedLeafData(node.data, "day", 86400)
}

func (node *kmsSegmentedNode) GetChild(name string) (MetricNode, error) {
	if name == byTimeName {
		return newByTimeNode(kmsView(node.data), node, node.path+"/"+byTimeName), nil
	}
	if name == "_ALL" {
		var merged madmin.SegmentedKMSActions
		for _, s := range node.data {
			merged.Add(&s)
		}
		return &kmsOpSegmentedNode{
			op:     "_ALL",
			seg:    &merged,
			parent: node,
			path:   node.path + "/_ALL",
		}, nil
	}
	seg, ok := node.data[name]
	if !ok {
		return nil, fmt.Errorf("operation not found: %s", name)
	}
	return &kmsOpSegmentedNode{
		op:     name,
		seg:    &seg,
		parent: node,
		path:   node.path + "/" + name,
	}, nil
}

// kmsOpSegmentedNode shows the time segments for a single operation,
// filtering out segments with no operations.
type kmsOpSegmentedNode struct {
	op     string
	seg    *madmin.SegmentedKMSActions
	parent MetricNode
	path   string
}

func (node *kmsOpSegmentedNode) GetOpts() madmin.MetricsOptions { return getNodeOpts(node) }
func (node *kmsOpSegmentedNode) GetMetricType() madmin.MetricType {
	return madmin.MetricsKMS
}
func (node *kmsOpSegmentedNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *kmsOpSegmentedNode) GetParent() MetricNode              { return node.parent }
func (node *kmsOpSegmentedNode) GetPath() string                    { return node.path }
func (node *kmsOpSegmentedNode) ShouldPauseRefresh() bool           { return false }
func (node *kmsOpSegmentedNode) GetChildren() []MetricChild         { return []MetricChild{} }

func (node *kmsOpSegmentedNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children")
}

func (node *kmsOpSegmentedNode) GetLeafData() map[string]string {
	if node.seg == nil || len(node.seg.Segments) == 0 {
		return map[string]string{"Status": "No segments"}
	}

	data := map[string]string{}
	interval := time.Duration(node.seg.Interval) * time.Second
	idx := 0
	for i, s := range node.seg.Segments {
		if s.Count == 0 {
			continue
		}
		t := node.seg.FirstTime.Add(time.Duration(i) * interval)
		end := t.Add(interval)
		key := fmt.Sprintf("%02d:%s->%sZ", idx, t.UTC().Format("15:04"), end.UTC().Format("15:04"))
		avg := s.Avg()
		rps := float64(s.Count) / interval.Seconds()
		line := fmt.Sprintf("%s->%s - %.1f req/s, %d calls, avg %v, min %v, max %v",
			t.Local().Format("15:04"), end.Local().Format("15:04"),
			rps, s.Count, avg.Round(time.Microsecond),
			time.Duration(s.MinTime*float64(time.Second)).Round(time.Microsecond),
			time.Duration(s.MaxTime*float64(time.Second)).Round(time.Microsecond))
		if s.ConnFails > 0 {
			line += fmt.Sprintf(", %d cf", s.ConnFails)
		}
		if s.RemoteErrs > 0 {
			line += fmt.Sprintf(", %d re", s.RemoteErrs)
		}
		data[key] = line
		idx++
	}
	if len(data) == 0 {
		return map[string]string{"Status": "No activity"}
	}
	return data
}

// kmsView adapts KMS last-day data to the generic _by_time navigation.
func kmsView(ops map[string]madmin.SegmentedKMSActions) segView[madmin.KMSAction, *madmin.KMSAction] {
	return segView[madmin.KMSAction, *madmin.KMSAction]{
		ops:         ops,
		metricType:  madmin.MetricsKMS,
		metricFlags: madmin.MetricsDayStats,
		empty:       func(a *madmin.KMSAction) bool { return a.Count == 0 && a.ConnFails == 0 && a.RemoteErrs == 0 },
		segDesc:     kmsSegmentDesc,
		opDesc:      kmsOpDesc,
		opLeaf: func(op string, a madmin.KMSAction, segTime time.Time, interval int, parent MetricNode, path string) MetricNode {
			return &kmsActionLeafNode{op: op, action: a, segTime: segTime, interval: interval, parent: parent, path: path}
		},
		sumLeaf: func(a madmin.KMSAction, segTime time.Time, interval int, parent MetricNode, path string) MetricNode {
			return &kmsActionLeafNode{op: "_ALL", action: a, segTime: segTime, interval: interval, parent: parent, path: path}
		},
	}
}

func kmsSegmentDesc(total madmin.KMSAction, interval int, segTime, end time.Time) string {
	day := ""
	if !sameLocalDay(segTime, time.Now()) {
		day = "Yesterday "
	}
	var rps float64
	if interval > 0 {
		rps = float64(total.Count) / float64(interval)
	}
	desc := fmt.Sprintf("%s%s -> %s, %.1f req/s, %d calls", day, segTime.Local().Format("15:04"), end.Local().Format("15:04"), rps, total.Count)
	if total.ConnFails+total.RemoteErrs > 0 {
		desc += fmt.Sprintf(", %d errors", total.ConnFails+total.RemoteErrs)
	}
	return desc
}

func kmsOpDesc(_ string, a madmin.KMSAction, interval int) string {
	var rps float64
	if interval > 0 {
		rps = float64(a.Count) / float64(interval)
	}
	desc := fmt.Sprintf("%.1f req/s, %d calls", rps, a.Count)
	if a.ConnFails+a.RemoteErrs > 0 {
		desc += fmt.Sprintf(", %d errors", a.ConnFails+a.RemoteErrs)
	}
	return desc
}

// kmsActionLeafNode renders a single KMSAction: one operation within one time
// segment, or the cross-operation summary (op == "_ALL").
type kmsActionLeafNode struct {
	op       string
	action   madmin.KMSAction
	segTime  time.Time
	interval int
	parent   MetricNode
	path     string
}

func (node *kmsActionLeafNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *kmsActionLeafNode) GetMetricType() madmin.MetricType   { return madmin.MetricsKMS }
func (node *kmsActionLeafNode) GetMetricFlags() madmin.MetricFlags { return madmin.MetricsDayStats }
func (node *kmsActionLeafNode) GetParent() MetricNode              { return node.parent }
func (node *kmsActionLeafNode) GetPath() string                    { return node.path }
func (node *kmsActionLeafNode) ShouldPauseRefresh() bool           { return true }
func (node *kmsActionLeafNode) GetChildren() []MetricChild         { return []MetricChild{} }

func (node *kmsActionLeafNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children")
}

func (node *kmsActionLeafNode) GetLeafData() map[string]string {
	a := node.action
	if a.Count == 0 && a.ConnFails == 0 && a.RemoteErrs == 0 {
		return map[string]string{"Status": "No activity"}
	}
	data := map[string]string{}
	idx := 0
	add := func(k, v string) {
		data[fmt.Sprintf("%02d:%s", idx, k)] = v
		idx++
	}
	if node.op != "" && node.op != "_ALL" {
		add("Operation", node.op)
	}
	if !node.segTime.IsZero() && node.interval > 0 {
		end := node.segTime.Add(time.Duration(node.interval) * time.Second)
		add("Time Segment", fmt.Sprintf("%s -> %s", node.segTime.Local().Format("15:04"), end.Local().Format("15:04")))
	}
	add("Calls", strconv.FormatUint(a.Count, 10))
	if node.interval > 0 {
		add("Rate", fmt.Sprintf("%.2f req/s", float64(a.Count)/float64(node.interval)))
	}
	add("Avg", a.Avg().Round(time.Microsecond).String())
	add("Min", time.Duration(a.MinTime*float64(time.Second)).Round(time.Microsecond).String())
	add("Max", time.Duration(a.MaxTime*float64(time.Second)).Round(time.Microsecond).String())
	if a.ConnFails > 0 {
		add("Conn Fails", strconv.FormatUint(a.ConnFails, 10))
	}
	if a.RemoteErrs > 0 {
		add("Remote Errors", strconv.FormatUint(a.RemoteErrs, 10))
	}
	return data
}

// kmsSegmentedLeafData builds per-operation summary leaf data from segmented stats.
// windowSecs is the total window duration for computing req/s.
func kmsSegmentedLeafData(data map[string]madmin.SegmentedKMSActions, label string, windowSecs float64) map[string]string {
	if len(data) == 0 {
		return map[string]string{"Status": "No data for last " + label}
	}

	result := map[string]string{}
	ops := sortedKeys(data)
	for i, op := range ops {
		s := data[op]
		total := s.Total()
		prefix := fmt.Sprintf("%02d:%s", i, op)
		avg := total.Avg()
		rps := float64(total.Count) / windowSecs
		line := fmt.Sprintf("%.1f req/s, %d calls, avg %v, min %v, max %v",
			rps, total.Count, avg.Round(time.Microsecond),
			time.Duration(total.MinTime*float64(time.Second)).Round(time.Microsecond),
			time.Duration(total.MaxTime*float64(time.Second)).Round(time.Microsecond))
		if total.ConnFails > 0 {
			line += fmt.Sprintf(", %d conn fails", total.ConnFails)
		}
		if total.RemoteErrs > 0 {
			line += fmt.Sprintf(", %d remote errs", total.RemoteErrs)
		}
		result[prefix] = line
	}
	return result
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
