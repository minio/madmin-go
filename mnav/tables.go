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
	"net/url"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/minio/madmin-go/v4"
)

// TableMetricsNode is the root navigation node for table API metrics.
type TableMetricsNode struct {
	tables *madmin.TableAPIMetrics
	parent MetricNode
	path   string
}

// NewTableMetricsNode constructs a new TableMetricsNode.
func NewTableMetricsNode(tables *madmin.TableAPIMetrics, parent MetricNode, path string) *TableMetricsNode {
	return &TableMetricsNode{tables: tables, parent: parent, path: path}
}

func (node *TableMetricsNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *TableMetricsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsTablesAPI }
func (node *TableMetricsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *TableMetricsNode) GetParent() MetricNode              { return node.parent }
func (node *TableMetricsNode) GetPath() string                    { return node.path }
func (node *TableMetricsNode) ShouldPauseRefresh() bool           { return false }

func (node *TableMetricsNode) GetChildren() []MetricChild {
	if node.tables == nil {
		return []MetricChild{}
	}
	return []MetricChild{
		{Name: "last_minute", Description: "Cluster table API totals over the last minute"},
		{Name: "last_hour", Description: "Per-minute segments over the last hour"},
		{Name: "last_day", Description: "15-minute segments over the last day"},
		{Name: "top_warehouses", Description: "Top warehouses by requests and throughput"},
		{Name: "top_namespaces", Description: "Top namespaces by requests and throughput"},
		{Name: "top_tables", Description: "Top tables by requests and throughput"},
	}
}

func (node *TableMetricsNode) GetLeafData() map[string]string {
	if node.tables == nil {
		return map[string]string{"Status": "No table metrics available"}
	}
	data := map[string]string{}
	idx := 0
	add := func(k, v string) {
		data[fmt.Sprintf("%02d:%s", idx, k)] = v
		idx++
	}
	add("Responding Nodes", fmt.Sprintf("%d", node.tables.Nodes))
	if !node.tables.CollectedAt.IsZero() {
		add("Collection Time", node.tables.CollectedAt.Format("15:04:05"))
	}
	if node.tables.LastMinute != nil {
		add("Last Minute", describeTableAPIStat(node.tables.LastMinute, 60))
	}
	return data
}

func (node *TableMetricsNode) GetChild(name string) (MetricNode, error) {
	if node.tables == nil {
		return nil, fmt.Errorf("no table metrics available")
	}
	switch name {
	case "last_minute":
		return &tableLastMinuteNode{stat: node.tables.LastMinute, parent: node, path: node.path + "/last_minute"}, nil
	case "last_hour":
		return &tableSegmentedNode{
			seg: node.tables.LastHour, windowSecs: 3600,
			flag: madmin.MetricsHourStats, parent: node, path: node.path + "/last_hour",
		}, nil
	case "last_day":
		return &tableSegmentedNode{
			seg: node.tables.LastDay, windowSecs: 86400,
			flag: madmin.MetricsDayStats, parent: node, path: node.path + "/last_day",
		}, nil
	case "top_warehouses":
		return &tableTopGroupNode{
			top: node.tables.TopWarehouses, label: "warehouse",
			flag: madmin.MetricsTopWarehouses, parent: node, path: node.path + "/top_warehouses",
		}, nil
	case "top_namespaces":
		return &tableTopGroupNode{
			top: node.tables.TopNamespaces, label: "namespace",
			flag: madmin.MetricsTopNamespaces, parent: node, path: node.path + "/top_namespaces",
		}, nil
	case "top_tables":
		return &tableTopGroupNode{
			top: node.tables.TopTables, label: "table",
			flag: madmin.MetricsTopTables, parent: node, path: node.path + "/top_tables",
		}, nil
	}
	return nil, fmt.Errorf("unknown table child: %s", name)
}

// tableLastMinuteNode shows cluster-wide totals over the last minute.
type tableLastMinuteNode struct {
	stat   *madmin.TableAPIStat
	parent MetricNode
	path   string
}

func (node *tableLastMinuteNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *tableLastMinuteNode) GetMetricType() madmin.MetricType   { return madmin.MetricsTablesAPI }
func (node *tableLastMinuteNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *tableLastMinuteNode) GetParent() MetricNode              { return node.parent }
func (node *tableLastMinuteNode) GetPath() string                    { return node.path }
func (node *tableLastMinuteNode) ShouldPauseRefresh() bool           { return false }
func (node *tableLastMinuteNode) GetChildren() []MetricChild         { return []MetricChild{} }
func (node *tableLastMinuteNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children")
}

func (node *tableLastMinuteNode) GetLeafData() map[string]string {
	if node.stat == nil || node.stat.IsZero() {
		return map[string]string{"Status": "No table activity in the last minute"}
	}
	return tableStatLeafData(node.stat, 60)
}

// tableSegmentedNode displays a SegmentedTableIO window as a summary leaf plus
// one child per non-empty time slot.
type tableSegmentedNode struct {
	seg        *madmin.SegmentedTableIO
	windowSecs float64
	flag       madmin.MetricFlags
	parent     MetricNode
	path       string
}

func (node *tableSegmentedNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *tableSegmentedNode) GetMetricType() madmin.MetricType   { return madmin.MetricsTablesAPI }
func (node *tableSegmentedNode) GetMetricFlags() madmin.MetricFlags { return node.flag }
func (node *tableSegmentedNode) GetParent() MetricNode              { return node.parent }
func (node *tableSegmentedNode) GetPath() string                    { return node.path }
func (node *tableSegmentedNode) ShouldPauseRefresh() bool           { return true }

func (node *tableSegmentedNode) GetChildren() []MetricChild {
	if node.seg == nil {
		return []MetricChild{}
	}
	stats := node.seg.AsTableIOStat()
	if len(stats) == 0 {
		return []MetricChild{}
	}
	interval := time.Duration(node.seg.IntervalSecs) * time.Second
	var children []MetricChild
	for i := len(stats) - 1; i >= 0; i-- {
		if stats[i].IsZero() {
			continue
		}
		t := node.seg.FirstTime.Add(time.Duration(i) * interval)
		end := t.Add(interval)
		day := ""
		if !sameLocalDay(t, time.Now()) {
			day = "Yesterday "
		}
		children = append(children, MetricChild{
			Name: t.UTC().Format("15:04Z"),
			Description: fmt.Sprintf("%s%s -> %s, %s", day,
				t.Local().Format("15:04"), end.Local().Format("15:04"),
				describeTableAPIStat(&stats[i], float64(node.seg.IntervalSecs))),
		})
	}
	return children
}

func (node *tableSegmentedNode) GetLeafData() map[string]string {
	if node.seg == nil {
		return map[string]string{"Status": "No segmented data"}
	}
	stats := node.seg.AsTableIOStat()
	if len(stats) == 0 {
		return map[string]string{"Status": "No activity"}
	}
	var total madmin.TableAPIStat
	for i := range stats {
		total.Add(&stats[i])
	}
	return tableStatLeafData(&total, node.windowSecs)
}

func (node *tableSegmentedNode) GetChild(name string) (MetricNode, error) {
	if node.seg == nil {
		return nil, fmt.Errorf("no segmented data")
	}
	stats := node.seg.AsTableIOStat()
	interval := time.Duration(node.seg.IntervalSecs) * time.Second
	for i := range stats {
		t := node.seg.FirstTime.Add(time.Duration(i) * interval)
		if t.UTC().Format("15:04Z") == name {
			return &tableSegmentEntryNode{
				stat: stats[i], windowSecs: float64(node.seg.IntervalSecs),
				flag: node.flag, parent: node, path: node.path + "/" + name,
			}, nil
		}
	}
	return nil, fmt.Errorf("segment not found: %s", name)
}

// tableSegmentEntryNode is a leaf for one time slot of a SegmentedTableIO.
type tableSegmentEntryNode struct {
	stat       madmin.TableAPIStat
	windowSecs float64
	flag       madmin.MetricFlags
	parent     MetricNode
	path       string
}

func (node *tableSegmentEntryNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *tableSegmentEntryNode) GetMetricType() madmin.MetricType   { return madmin.MetricsTablesAPI }
func (node *tableSegmentEntryNode) GetMetricFlags() madmin.MetricFlags { return node.flag }
func (node *tableSegmentEntryNode) GetParent() MetricNode              { return node.parent }
func (node *tableSegmentEntryNode) GetPath() string                    { return node.path }
func (node *tableSegmentEntryNode) ShouldPauseRefresh() bool           { return true }
func (node *tableSegmentEntryNode) GetChildren() []MetricChild         { return []MetricChild{} }
func (node *tableSegmentEntryNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children")
}

func (node *tableSegmentEntryNode) GetLeafData() map[string]string {
	return tableStatLeafData(&node.stat, node.windowSecs)
}

// rankedList describes one of the six TopTableIO ranked slices.
type rankedList struct {
	name        string
	description string
	entries     []madmin.TableIOMetrics
	windowSecs  float64
	flag        madmin.MetricFlags // additional flag beyond the parent's MetricsTopXxx
}

// tableTopGroupNode exposes one of the three top categories (warehouses,
// namespaces, tables) and routes to the six ranked sub-lists inside its
// TopTableIO.
type tableTopGroupNode struct {
	top    *madmin.TopTableIO
	label  string
	flag   madmin.MetricFlags
	parent MetricNode
	path   string
}

func (node *tableTopGroupNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *tableTopGroupNode) GetMetricType() madmin.MetricType   { return madmin.MetricsTablesAPI }
func (node *tableTopGroupNode) GetMetricFlags() madmin.MetricFlags { return node.flag }
func (node *tableTopGroupNode) GetParent() MetricNode              { return node.parent }
func (node *tableTopGroupNode) GetPath() string                    { return node.path }
func (node *tableTopGroupNode) ShouldPauseRefresh() bool           { return false }

func (node *tableTopGroupNode) rankedLists() []rankedList {
	if node.top == nil {
		return nil
	}
	lbl := node.label
	return []rankedList{
		{"reqs_minute", "Last-minute " + lbl + "s by requests", node.top.ByRequestsMin, 60, 0},
		{"throughput_minute", "Last-minute " + lbl + "s by throughput", node.top.ByThroughputMin, 60, 0},
		{"reqs_hour", "Last-hour " + lbl + "s by requests", node.top.ByRequestsHour, 3600, madmin.MetricsHourStats},
		{"throughput_hour", "Last-hour " + lbl + "s by throughput", node.top.ByThroughputHour, 3600, madmin.MetricsHourStats},
		{"reqs_day", "Last-day " + lbl + "s by requests", node.top.ByRequestsDay, 86400, madmin.MetricsDayStats},
		{"throughput_day", "Last-day " + lbl + "s by throughput", node.top.ByThroughputDay, 86400, madmin.MetricsDayStats},
	}
}

func (node *tableTopGroupNode) GetChildren() []MetricChild {
	lists := node.rankedLists()
	children := make([]MetricChild, 0, len(lists))
	for _, l := range lists {
		desc := l.description
		if len(l.entries) > 0 {
			desc = fmt.Sprintf("%s (%d entries)", desc, len(l.entries))
		} else {
			desc = desc + " (no data)"
		}
		children = append(children, MetricChild{Name: l.name, Description: desc})
	}
	return children
}

func (node *tableTopGroupNode) GetLeafData() map[string]string {
	if node.top == nil {
		return map[string]string{"Status": "No " + node.label + " data"}
	}
	data := map[string]string{}
	idx := 0
	add := func(k, v string) {
		data[fmt.Sprintf("%02d:%s", idx, k)] = v
		idx++
	}
	for _, l := range node.rankedLists() {
		if len(l.entries) == 0 {
			continue
		}
		add(l.description, fmt.Sprintf("%d entries; top: %s",
			len(l.entries), tableEntryKey(&l.entries[0])))
	}
	if idx == 0 {
		return map[string]string{"Status": "No " + node.label + " data"}
	}
	return data
}

func (node *tableTopGroupNode) GetChild(name string) (MetricNode, error) {
	for _, l := range node.rankedLists() {
		if l.name != name {
			continue
		}
		return &tableRankedListNode{
			entries: l.entries, windowSecs: l.windowSecs,
			flag: l.flag, parent: node, path: node.path + "/" + name,
		}, nil
	}
	return nil, fmt.Errorf("unknown ranked list: %s", name)
}

// tableRankedListNode shows one ranked list (e.g., requests over the last
// minute). Children are the individual entries.
type tableRankedListNode struct {
	entries    []madmin.TableIOMetrics
	windowSecs float64
	flag       madmin.MetricFlags
	parent     MetricNode
	path       string
}

func (node *tableRankedListNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *tableRankedListNode) GetMetricType() madmin.MetricType   { return madmin.MetricsTablesAPI }
func (node *tableRankedListNode) GetMetricFlags() madmin.MetricFlags { return node.flag }
func (node *tableRankedListNode) GetParent() MetricNode              { return node.parent }
func (node *tableRankedListNode) GetPath() string                    { return node.path }
func (node *tableRankedListNode) ShouldPauseRefresh() bool           { return node.windowSecs > 60 }

func (node *tableRankedListNode) GetChildren() []MetricChild {
	children := make([]MetricChild, 0, len(node.entries))
	for i := range node.entries {
		e := &node.entries[i]
		key := tableEntryKey(e)
		children = append(children, MetricChild{
			Name:        url.PathEscape(key),
			DisplayName: key,
			Description: describeTableAPIStat(&e.TableAPIStat, node.windowSecs),
		})
	}
	return children
}

func (node *tableRankedListNode) GetLeafData() map[string]string {
	if len(node.entries) == 0 {
		return map[string]string{"Status": "No entries"}
	}
	data := map[string]string{
		"00:Entries": fmt.Sprintf("%d", len(node.entries)),
	}
	for i := range node.entries {
		key := fmt.Sprintf("%02d:%s", i+1, tableEntryKey(&node.entries[i]))
		data[key] = describeTableAPIStat(&node.entries[i].TableAPIStat, node.windowSecs)
	}
	return data
}

func (node *tableRankedListNode) GetChild(name string) (MetricNode, error) {
	decoded, err := url.PathUnescape(name)
	if err != nil {
		return nil, fmt.Errorf("invalid name: %s", name)
	}
	for i := range node.entries {
		if tableEntryKey(&node.entries[i]) == decoded {
			return &tableEntryNode{
				entry: node.entries[i], windowSecs: node.windowSecs,
				flag: node.flag, parent: node, path: node.path + "/" + name,
			}, nil
		}
	}
	return nil, fmt.Errorf("entry not found: %s", decoded)
}

// tableEntryNode is a leaf for a single TableIOMetrics row from a ranked list.
type tableEntryNode struct {
	entry      madmin.TableIOMetrics
	windowSecs float64
	flag       madmin.MetricFlags
	parent     MetricNode
	path       string
}

func (node *tableEntryNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *tableEntryNode) GetMetricType() madmin.MetricType   { return madmin.MetricsTablesAPI }
func (node *tableEntryNode) GetMetricFlags() madmin.MetricFlags { return node.flag }
func (node *tableEntryNode) GetParent() MetricNode              { return node.parent }
func (node *tableEntryNode) GetPath() string                    { return node.path }
func (node *tableEntryNode) ShouldPauseRefresh() bool           { return node.windowSecs > 60 }
func (node *tableEntryNode) GetChildren() []MetricChild         { return []MetricChild{} }
func (node *tableEntryNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children")
}

func (node *tableEntryNode) GetLeafData() map[string]string {
	data := map[string]string{}
	idx := 0
	add := func(k, v string) {
		data[fmt.Sprintf("%02d:%s", idx, k)] = v
		idx++
	}
	if node.entry.Warehouse != nil {
		add("Warehouse", *node.entry.Warehouse)
	}
	if node.entry.Namespace != nil {
		add("Namespace", *node.entry.Namespace)
	}
	if node.entry.Table != nil {
		add("Table", *node.entry.Table)
	}
	addTableStatData(&node.entry.TableAPIStat, node.windowSecs, add)
	if idx == 0 {
		return map[string]string{"Status": "No data"}
	}
	return data
}

// tableEntryKey returns the path-safe identifier for a TableIOMetrics row.
func tableEntryKey(e *madmin.TableIOMetrics) string {
	parts := make([]string, 0, 3)
	if e.Warehouse != nil {
		parts = append(parts, *e.Warehouse)
	}
	if e.Namespace != nil {
		parts = append(parts, *e.Namespace)
	}
	if e.Table != nil {
		parts = append(parts, *e.Table)
	}
	if len(parts) == 0 {
		return "(unnamed)"
	}
	return strings.Join(parts, ":")
}

// describeTableAPIStat formats a TableAPIStat as a single line for use in
// child descriptions. windowSecs is the window duration for rate display.
func describeTableAPIStat(s *madmin.TableAPIStat, windowSecs float64) string {
	if s == nil || s.IsZero() {
		return "No activity"
	}
	total := s.Reads + s.Writes
	parts := []string{fmt.Sprintf("%s req", humanize.Comma(total))}
	if total > 0 && windowSecs > 0 {
		parts = append(parts, fmt.Sprintf("%.1f req/s", float64(total)/windowSecs))
	}
	if s.Reads > 0 {
		parts = append(parts, "R:"+humanize.Comma(s.Reads))
	}
	if s.Writes > 0 {
		parts = append(parts, "W:"+humanize.Comma(s.Writes))
	}
	if s.NotOK > 0 {
		parts = append(parts, humanize.Comma(s.NotOK)+" err")
	}
	if s.BytesIn > 0 {
		parts = append(parts, "in:"+humanize.Bytes(uint64(s.BytesIn)))
	}
	if s.BytesOut > 0 {
		parts = append(parts, "out:"+humanize.Bytes(uint64(s.BytesOut)))
	}
	if total > 0 && s.RequestTimeSecs > 0 {
		parts = append(parts, fmt.Sprintf("%.1fms avg", (s.RequestTimeSecs/float64(total))*1000))
	}
	if total > 0 && s.RespTTFBSecs > 0 {
		parts = append(parts, fmt.Sprintf("%.1fms ttfb", (s.RespTTFBSecs/float64(total))*1000))
	}
	return strings.Join(parts, ", ")
}

// tableStatLeafData formats a TableAPIStat as the per-key leaf data shown in
// a node's body. windowSecs is the window duration for rate display.
func tableStatLeafData(s *madmin.TableAPIStat, windowSecs float64) map[string]string {
	data := map[string]string{}
	idx := 0
	add := func(k, v string) {
		data[fmt.Sprintf("%02d:%s", idx, k)] = v
		idx++
	}
	addTableStatData(s, windowSecs, add)
	return data
}

// addTableStatData emits leaf entries for a TableAPIStat through add. The
// caller controls ordering and key prefixing via its closure.
func addTableStatData(s *madmin.TableAPIStat, windowSecs float64, add func(k, v string)) {
	total := s.Reads + s.Writes
	add("Requests", humanize.Comma(total))
	if total > 0 && windowSecs > 0 {
		add("Request Rate", fmt.Sprintf("%.1f req/s", float64(total)/windowSecs))
	}
	if s.Reads > 0 {
		add("Reads", humanize.Comma(s.Reads))
	}
	if s.Writes > 0 {
		add("Writes", humanize.Comma(s.Writes))
	}
	if s.NotOK > 0 {
		v := humanize.Comma(s.NotOK)
		if total > 0 {
			v += fmt.Sprintf(" (%.1f%%)", float64(s.NotOK)/float64(total)*100)
		}
		add("Errors", v)
	}
	if s.BytesIn > 0 || s.BytesOut > 0 {
		add("Throughput", humanize.Bytes(uint64(s.BytesIn+s.BytesOut)))
		if s.BytesIn > 0 {
			add("-> Incoming", humanize.Bytes(uint64(s.BytesIn)))
		}
		if s.BytesOut > 0 {
			add("<- Outgoing", humanize.Bytes(uint64(s.BytesOut)))
		}
	}
	if total > 0 && s.RequestTimeSecs > 0 {
		add("Avg Latency", fmt.Sprintf("%.1fms", (s.RequestTimeSecs/float64(total))*1000))
	}
	if total > 0 && s.RespTTFBSecs > 0 {
		add("Avg TTFB", fmt.Sprintf("%.1fms", (s.RespTTFBSecs/float64(total))*1000))
	}
}
