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
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/minio/madmin-go/v4"
	"github.com/minio/minio-go/v7/pkg/set"
)

// TargetMetricsNode is the navigation node for asynchronous delivery targets:
// bucket-event notification, audit logs and system logs.
type TargetMetricsNode struct {
	targets *madmin.DeliveryTargetMetrics
	parent  MetricNode
	path    string
}

// NewTargetMetricsNode constructs a new TargetMetricsNode.
func NewTargetMetricsNode(targets *madmin.DeliveryTargetMetrics, parent MetricNode, path string) *TargetMetricsNode {
	return &TargetMetricsNode{targets: targets, parent: parent, path: path}
}

func (node *TargetMetricsNode) GetOpts() madmin.MetricsOptions   { return getNodeOpts(node) }
func (node *TargetMetricsNode) GetMetricType() madmin.MetricType { return madmin.MetricsTargets }

// GetMetricFlags requests no historic window: this node refreshes continuously,
// and asking for them here would pull 60+96 segments per target on every tick to
// render a few summary lines. They are fetched on navigating into one.
func (node *TargetMetricsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *TargetMetricsNode) GetParent() MetricNode              { return node.parent }
func (node *TargetMetricsNode) GetPath() string                    { return node.path }
func (node *TargetMetricsNode) ShouldPauseRefresh() bool           { return false }

// GetChildren lists the segmented windows, then one child per configured target.
// The target child name is the wire key, since a target name is only unique within
// its config subsystem. The windows are listed unconditionally: their nodes explain
// a window that is missing or empty, so they are always constructible.
func (node *TargetMetricsNode) GetChildren() []MetricChild {
	if node.targets == nil {
		return []MetricChild{}
	}
	t := node.targets
	children := make([]MetricChild, 0, 4+len(t.Notification)+len(t.Audit)+len(t.Logs))
	children = append(children,
		MetricChild{Name: "last_hour", Description: "Delivery flow per target over the last hour, by time segment"},
		MetricChild{Name: "last_day", Description: "Delivery flow per target over the last day, by time segment"},
		MetricChild{Name: "log_volume_last_hour", Description: "Log lines emitted by severity over the last hour, by time segment"},
		MetricChild{Name: "log_volume_last_day", Description: "Log lines emitted by severity over the last day, by time segment"},
	)
	listed := set.NewStringSet()
	for _, class := range []struct {
		kind string
		m    map[string]madmin.TargetMetrics
	}{
		{"notification", t.Notification},
		{"audit log", t.Audit},
		{"system log", t.Logs},
	} {
		for _, key := range sortedKeys(class.m) {
			// The server's key is unique across the three classes, but the types do
			// not enforce it, so first wins, in the order GetChild already resolves.
			// Listing a key twice would give the reader two entries that both open
			// the same node.
			if listed.Contains(key) {
				continue
			}
			listed.Add(key)
			tm := class.m[key]
			children = append(children, MetricChild{
				Name:        key,
				Description: fmt.Sprintf("%s %s target on %d node(s)", tm.Type, class.kind, tm.N),
			})
		}
	}
	return children
}

func (node *TargetMetricsNode) GetChild(name string) (MetricNode, error) {
	if node.targets == nil {
		return nil, fmt.Errorf("no delivery target data available")
	}
	t := node.targets
	// A target key is "<subsystem>:<name>", so it can never collide with these.
	switch name {
	case "last_hour", "last_day":
		hour := name == "last_hour"
		ops, class := targetWindowMap(t, hour)
		window := "day"
		flag := madmin.MetricsDayStats
		if hour {
			window, flag = "hour", madmin.MetricsHourStats
		}
		return &targetWindowsNode{
			ops: ops, class: class, flag: flag, window: window,
			configured: len(t.Notification) + len(t.Audit) + len(t.Logs),
			parent:     node, path: fmt.Sprintf("%s/%s", node.path, name),
		}, nil
	case "log_volume_last_hour":
		return &logVolumeWindowNode{
			seg: t.LogVolumeLastHour, flags: madmin.MetricsHourStats, window: "hour",
			parent: node, path: fmt.Sprintf("%s/%s", node.path, name),
		}, nil
	case "log_volume_last_day":
		return &logVolumeWindowNode{
			seg: t.LogVolumeLastDay, flags: madmin.MetricsDayStats, window: "day",
			parent: node, path: fmt.Sprintf("%s/%s", node.path, name),
		}, nil
	}
	for _, m := range []map[string]madmin.TargetMetrics{t.Notification, t.Audit, t.Logs} {
		if tm, ok := m[name]; ok {
			return NewTargetNode(tm, node, fmt.Sprintf("%s/%s", node.path, name)), nil
		}
	}
	return nil, fmt.Errorf("child not found: %s", name)
}

func (node *TargetMetricsNode) GetLeafData() map[string]string {
	if node.targets == nil {
		return map[string]string{"Status": "Delivery target metrics not available"}
	}
	t := node.targets
	data := map[string]string{
		"Collected At":         t.CollectedAt.Format("2006-01-02 15:04:05"),
		"Nodes":                strconv.Itoa(t.Nodes),
		"Notification Targets": strconv.Itoa(len(t.Notification)),
		"Audit Log Targets":    strconv.Itoa(len(t.Audit)),
		"System Log Targets":   strconv.Itoa(len(t.Logs)),
	}

	// Log volume works even with no target configured. Summarized when the history
	// happens to be loaded -- it is persisted to disk and restored on startup, and a
	// child node may have fetched it -- but never requested from here, and never
	// rendered as a zero when it is absent.
	if w := t.LogVolumeLastHour; w != nil && len(w.Segments) > 0 {
		data["Logged (last hour)"] = formatLogVolumeWindow(w)
	}
	if w := t.LogVolumeLastDay; w != nil && len(w.Segments) > 0 {
		data["Logged (last day)"] = formatLogVolumeWindow(w)
	}

	if t.Spill != nil && (t.Spill.Bytes > 0 || t.Spill.Files > 0) {
		data["Spilled To Disk"] = fmt.Sprintf("%s in %d file(s)",
			humanize.IBytes(uint64(t.Spill.Bytes)), t.Spill.Files)
	}

	// The failure states an operator scans for. Drops only exist in the windows, and
	// each window is only populated when its flag was requested, so both are totaled
	// and reported under their own label rather than added together.
	var hourDrops, dayDrops, queued, capacity uint64
	var degraded []string
	for _, m := range []map[string]madmin.TargetMetrics{t.Notification, t.Audit, t.Logs} {
		for key, tm := range m {
			hourDrops += droppedIn(tm.LastHour)
			dayDrops += droppedIn(tm.LastDay)
			queued += uint64(max(tm.QueueLength, 0))
			capacity += uint64(max(tm.QueueCapacity, 0))
			if tm.NodesOnline < tm.N {
				degraded = append(degraded, key)
			}
		}
	}
	if capacity > 0 {
		data["Queue"] = fmt.Sprintf("%d of %d (%s)", queued, capacity,
			calculatePercentage(queued, capacity))
	}
	if hourDrops > 0 {
		data["Dropped Events (last hour)"] = strconv.FormatUint(hourDrops, 10)
	}
	if dayDrops > 0 {
		data["Dropped Events (last day)"] = strconv.FormatUint(dayDrops, 10)
	}
	if len(degraded) > 0 {
		sort.Strings(degraded)
		data["Not Online Everywhere"] = strings.Join(degraded, ", ")
	}
	return data
}

// formatLogVolumeWindow totals a segmented log-volume window into one line.
// Errors and warnings are named even at zero, because "0 errors in the last hour"
// is the reassurance an operator is after; the quieter severities appear only when
// something was logged at them.
func formatLogVolumeWindow(w *madmin.SegmentedLogVolume) string {
	if state := windowSummaryState(w); state != "" {
		return state
	}
	t := w.Total()
	out := fmt.Sprintf("%d errors, %d warnings", t.ErrorLines, t.WarningLines)
	for _, rest := range []struct {
		label string
		n     uint64
	}{
		{"fatal", t.FatalLines},
		{"info", t.InfoLines},
		{"events", t.EventLines},
	} {
		if rest.n > 0 {
			out += fmt.Sprintf(", %d %s", rest.n, rest.label)
		}
	}
	return out
}

// droppedIn totals the events a window discarded, for any reason. A window that
// was not requested is nil and totals to zero.
func droppedIn(w *madmin.SegmentedTargetMetrics) uint64 {
	return droppedInSegment(w.Total())
}

// droppedInSegment totals the events one segment discarded. The four reasons sum
// to it, so the wire carries no total.
func droppedInSegment(s madmin.TargetSegment) uint64 {
	return s.DroppedQueueFull + s.DroppedRetriesExhausted + s.DroppedShutdown + s.DroppedOther
}

// TargetNode is one delivery target.
type TargetNode struct {
	target madmin.TargetMetrics
	parent MetricNode
	path   string
}

// NewTargetNode constructs a new TargetNode.
func NewTargetNode(target madmin.TargetMetrics, parent MetricNode, path string) *TargetNode {
	return &TargetNode{target: target, parent: parent, path: path}
}

func (node *TargetNode) GetOpts() madmin.MetricsOptions   { return getNodeOpts(node) }
func (node *TargetNode) GetMetricType() madmin.MetricType { return madmin.MetricsTargets }

// GetMetricFlags requests no historic window, for the reason on
// TargetMetricsNode.GetMetricFlags: this node refreshes continuously, so its
// windows are fetched on entering one of them.
func (node *TargetNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *TargetNode) GetParent() MetricNode              { return node.parent }
func (node *TargetNode) GetPath() string                    { return node.path }
func (node *TargetNode) ShouldPauseRefresh() bool           { return false }

// GetChildren lists this target's own windows, which is where an operator sees
// whether a day's drops were one burst or a steady trickle.
func (node *TargetNode) GetChildren() []MetricChild {
	return []MetricChild{
		{Name: "last_hour", Description: "This target's delivery flow over the last hour, by time segment"},
		{Name: "last_day", Description: "This target's delivery flow over the last day, by time segment"},
	}
}

func (node *TargetNode) GetChild(name string) (MetricNode, error) {
	key := node.target.Name
	if node.target.Subsystem != "" {
		key = node.target.Subsystem + ":" + key
	}
	switch name {
	case "last_hour":
		return &targetWindowNode{
			seg: node.target.LastHour, key: key, flags: madmin.MetricsHourStats, window: "hour",
			parent: node, path: fmt.Sprintf("%s/last_hour", node.path),
		}, nil
	case "last_day":
		return &targetWindowNode{
			seg: node.target.LastDay, key: key, flags: madmin.MetricsDayStats, window: "day",
			parent: node, path: fmt.Sprintf("%s/last_day", node.path),
		}, nil
	}
	return nil, fmt.Errorf("child not found: %s", name)
}

func (node *TargetNode) GetLeafData() map[string]string {
	t := node.target
	data := map[string]string{
		"Subsystem": t.Subsystem,
		"Name":      t.Name,
		"Type":      t.Type,
	}
	if t.Endpoint != "" {
		data["Endpoint"] = t.Endpoint
	}
	// Readiness as node counts, which shows a partial outage.
	data["Online"] = fmt.Sprintf("%d of %d node(s)", t.NodesOnline, t.N)
	if t.NodesChecking > 0 {
		data["Checking"] = fmt.Sprintf("%d node(s)", t.NodesChecking)
	}

	// Flow only exists in the windows, so they are where events, requests, failures
	// and drops are reported. Summarized here when the history happens to be loaded,
	// never requested from here, and never rendered as a zero when it is absent.
	if w := t.LastHour; w != nil && len(w.Segments) > 0 {
		data["Last Hour"] = formatTargetWindow(w)
	}
	if w := t.LastDay; w != nil && len(w.Segments) > 0 {
		data["Last Day"] = formatTargetWindow(w)
	}

	if t.QueueCapacity > 0 {
		data["Queue"] = fmt.Sprintf("%d of %d (%s)", t.QueueLength, t.QueueCapacity,
			calculatePercentage(uint64(max(t.QueueLength, 0)), uint64(t.QueueCapacity)))
	}
	if t.Inflight > 0 {
		data["In Flight"] = strconv.FormatInt(t.Inflight, 10)
	}
	if t.Workers > 0 {
		data["Workers"] = strconv.FormatInt(t.Workers, 10)
	}

	// Every completed attempt is timed. The mean is derived here.
	if lm := t.LastMinute; lm.Count > 0 {
		data["Latency (last min)"] = fmt.Sprintf("avg %s, min %s, max %s over %d request(s)",
			durationOf(lm.AccTime, lm.Count),
			time.Duration(lm.MinTime).Round(time.Microsecond),
			time.Duration(lm.MaxTime).Round(time.Microsecond),
			lm.Count)
		if lm.Bytes > 0 {
			data["Sent (last min)"] = humanize.IBytes(lm.Bytes)
		}
	}

	if t.LastError != "" {
		data["Last Error"] = t.LastError
		if !t.LastErrorTime.IsZero() {
			data["Last Error At"] = t.LastErrorTime.Format("2006-01-02 15:04:05")
		}
	}
	return data
}

// formatTargetWindow totals a segmented window into one line. Failures and drop
// reasons are named only when they happened, so a healthy target reads as healthy
// rather than as a wall of zeros.
func formatTargetWindow(w *madmin.SegmentedTargetMetrics) string {
	if state := windowSummaryState(w); state != "" {
		return state
	}
	t := w.Total()
	out := strconv.FormatUint(t.Events, 10) + " events"
	// Count is requests, not events; the ratio is the batch factor.
	if t.Requests > 0 {
		out += fmt.Sprintf(", %d reqs (%.1f ea, avg %s)", t.Requests,
			float64(t.Events)/float64(t.Requests),
			durationOf(t.RequestNanos, t.Requests))
	}
	for _, rare := range []struct {
		label string
		n     uint64
	}{
		{"failed", t.WriterErrors},
		{"dropped (queue full)", t.DroppedQueueFull},
		{"dropped (retries exhausted)", t.DroppedRetriesExhausted},
		{"dropped (shutdown)", t.DroppedShutdown},
		{"dropped (other)", t.DroppedOther},
	} {
		if rare.n > 0 {
			out += fmt.Sprintf(", %d %s", rare.n, rare.label)
		}
	}
	return out
}

// targetWindowMap flattens every target's window of one resolution into the keyed
// shape the time-first navigation wants, plus the class each key came from.
//
// One resolution per map: Segmented.Add bails out silently when intervals differ,
// so an hour window dropped into a day map would contribute nothing and its target
// would disappear from every segment list.
func targetWindowMap(t *madmin.DeliveryTargetMetrics, hour bool) (
	map[string]madmin.SegmentedTargetMetrics, map[string]string,
) {
	if t == nil {
		return nil, nil
	}
	size := len(t.Notification) + len(t.Audit) + len(t.Logs)
	ops := make(map[string]madmin.SegmentedTargetMetrics, size)
	class := make(map[string]string, size)
	for _, c := range []struct {
		kind string
		m    map[string]madmin.TargetMetrics
	}{
		{"notification", t.Notification},
		{"audit log", t.Audit},
		{"system log", t.Logs},
	} {
		for _, key := range sortedKeys(c.m) {
			tm := c.m[key]
			w := tm.LastDay
			if hour {
				w = tm.LastHour
			}
			// nil is a window whose flag was not requested, so the target has nothing
			// to say at this resolution and is left out. A window that was requested
			// is kept even with no completed segment, so it reads as still filling
			// rather than as never asked for.
			if w == nil {
				continue
			}
			// The server's key is unique across the three classes, but the types do
			// not enforce it, so first wins, in the order GetChild already resolves.
			// Blending two unrelated targets into one series would be worse than
			// hiding one.
			if _, dup := ops[key]; dup {
				continue
			}
			ops[key] = *w
			class[key] = c.kind
		}
	}
	return ops, class
}

// targetView adapts one resolution of the per-target windows to the generic
// _by_time navigation: pick a time segment, then see every target active in it.
func targetView(ops map[string]madmin.SegmentedTargetMetrics, class map[string]string,
	flag madmin.MetricFlags,
) segView[madmin.TargetSegment, *madmin.TargetSegment] {
	return segView[madmin.TargetSegment, *madmin.TargetSegment]{
		ops:         ops,
		metricType:  madmin.MetricsTargets,
		metricFlags: flag,
		empty:       targetSegmentEmpty,
		segDesc: func(total madmin.TargetSegment, interval int, segTime, _ time.Time) string {
			// Only a day window can straddle midnight, so only it needs the date.
			return segmentDescTime(segTime, flag == madmin.MetricsDayStats) + ", " +
				describeTargetSegment(total, interval)
		},
		opDesc: func(key string, s madmin.TargetSegment, interval int) string {
			return class[key] + ": " + describeTargetSegment(s, interval)
		},
		opLeaf: func(key string, s madmin.TargetSegment, segTime time.Time, interval int, parent MetricNode, path string) MetricNode {
			return &targetSegmentLeafNode{
				key: key, kind: class[key], seg: s, segTime: segTime,
				interval: interval, merged: 1, flags: flag, parent: parent, path: path,
			}
		},
		sumLeaf: func(s madmin.TargetSegment, segTime time.Time, interval, merged int, parent MetricNode, path string) MetricNode {
			return &targetSegmentLeafNode{
				key: "_ALL", seg: s, segTime: segTime,
				interval: interval, merged: merged, flags: flag, parent: parent, path: path,
			}
		},
	}
}

// targetSegmentEmpty reports a segment with nothing to show. N is not part of the
// test: an idle target still reports itself in every slot, so testing it would
// filter nothing and return a wall of zeros.
func targetSegmentEmpty(s *madmin.TargetSegment) bool {
	return s.Events == 0 && s.Requests == 0 && s.WriterErrors == 0 && droppedInSegment(*s) == 0
}

// targetWindowsNode groups one resolution of the per-target windows: browse by
// time, by every target combined, or by one target's own series.
type targetWindowsNode struct {
	ops        map[string]madmin.SegmentedTargetMetrics
	class      map[string]string
	flag       madmin.MetricFlags
	window     string
	configured int
	parent     MetricNode
	path       string
}

func (node *targetWindowsNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *targetWindowsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsTargets }
func (node *targetWindowsNode) GetMetricFlags() madmin.MetricFlags { return node.flag }
func (node *targetWindowsNode) GetParent() MetricNode              { return node.parent }
func (node *targetWindowsNode) GetPath() string                    { return node.path }
func (node *targetWindowsNode) ShouldPauseRefresh() bool           { return true }

func (node *targetWindowsNode) GetChildren() []MetricChild {
	if len(node.ops) == 0 {
		return []MetricChild{}
	}
	children := make([]MetricChild, 0, len(node.ops)+2)
	children = append(children,
		MetricChild{Name: byTimeName, Description: "Browse by time segment (all targets)"},
		MetricChild{Name: "_ALL", Description: fmt.Sprintf("All %d target(s) combined", len(node.ops))},
	)
	for _, key := range sortedKeys(node.ops) {
		w := node.ops[key]
		children = append(children, MetricChild{
			Name:        key,
			Description: node.class[key] + ": " + formatTargetWindow(&w),
		})
	}
	return children
}

// merged is every target's window on one timeline. It starts from a zero value so
// the wire segments are never mutated in place.
func (node *targetWindowsNode) merged() madmin.SegmentedTargetMetrics {
	var merged madmin.SegmentedTargetMetrics
	for _, key := range sortedKeys(node.ops) {
		w := node.ops[key]
		merged.Add(&w)
	}
	return merged
}

func (node *targetWindowsNode) GetChild(name string) (MetricNode, error) {
	if name == byTimeName {
		return newByTimeNode(targetView(node.ops, node.class, node.flag), node,
			node.path+"/"+byTimeName), nil
	}
	if name == "_ALL" {
		merged := node.merged()
		return &targetWindowNode{
			seg: &merged, key: "_ALL", flags: node.flag, window: node.window,
			parent: node, path: node.path + "/_ALL",
		}, nil
	}
	w, ok := node.ops[name]
	if !ok {
		return nil, fmt.Errorf("target not found: %s", name)
	}
	return &targetWindowNode{
		seg: &w, key: name, kind: node.class[name], flags: node.flag, window: node.window,
		parent: node, path: node.path + "/" + name,
	}, nil
}

func (node *targetWindowsNode) GetLeafData() map[string]string {
	if node.configured == 0 {
		return map[string]string{"Status": "no delivery targets configured"}
	}
	if len(node.ops) == 0 {
		return map[string]string{"Status": "not collected: last-" + node.window + " stats were not requested"}
	}
	data := map[string]string{}
	for i, key := range sortedKeys(node.ops) {
		w := node.ops[key]
		data[fmt.Sprintf("%02d:%s", i, key)] = node.class[key] + ": " + formatTargetWindow(&w)
	}
	return data
}

// targetWindowNode is one target's persisted window -- the hour or the day -- as a
// row per time segment plus one navigable child per segment, so a burst can be told
// apart from a steady trickle.
type targetWindowNode struct {
	seg    *madmin.SegmentedTargetMetrics
	key    string
	kind   string
	flags  madmin.MetricFlags
	window string
	parent MetricNode
	path   string
}

func (node *targetWindowNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *targetWindowNode) GetMetricType() madmin.MetricType   { return madmin.MetricsTargets }
func (node *targetWindowNode) GetMetricFlags() madmin.MetricFlags { return node.flags }
func (node *targetWindowNode) GetParent() MetricNode              { return node.parent }
func (node *targetWindowNode) GetPath() string                    { return node.path }
func (node *targetWindowNode) ShouldPauseRefresh() bool           { return true }

// hasSegments reports whether the window can be placed on a timeline at all. An
// interval of zero would stamp every segment with the same time.
func (node *targetWindowNode) hasSegments() bool {
	return node.seg != nil && node.seg.Interval > 0 && len(node.seg.Segments) > 0
}

// wholeSecs is the span the whole window covers: one segment as wide as every
// slot it holds, so an _ALL rate reads as the window average.
func (node *targetWindowNode) wholeSecs() int {
	return node.seg.Interval * len(node.seg.Segments)
}

func (node *targetWindowNode) GetChildren() []MetricChild {
	if !node.hasSegments() {
		return []MetricChild{}
	}
	children := []MetricChild{{
		Name: "_ALL",
		Description: "Every segment in the window combined. " +
			windowCoverage(node.seg.FirstTime, node.wholeSecs()),
	}}
	owners := segmentSecOwners(node.seg.FirstTime, node.seg.Interval, len(node.seg.Segments))
	withDate := windowCrossesDay(node.seg.FirstTime, node.seg.Interval, len(node.seg.Segments))
	for i := range node.seg.Segments {
		s := node.seg.Segments[i]
		if targetSegmentEmpty(&s) {
			continue
		}
		start := segmentStart(node.seg.FirstTime, node.seg.Interval, i)
		name := segmentKey(start)
		if owners[name] != i {
			continue
		}
		children = append(children, MetricChild{
			Name:        name,
			Description: segmentDescTime(start, withDate) + ", " + describeTargetSegment(s, node.seg.Interval),
		})
	}
	return children
}

func (node *targetWindowNode) GetChild(name string) (MetricNode, error) {
	if !node.hasSegments() {
		return nil, fmt.Errorf("no last-%s segments available", node.window)
	}
	leaf := func(s madmin.TargetSegment, segTime time.Time, interval, merged int) MetricNode {
		return &targetSegmentLeafNode{
			key: node.key, kind: node.kind, seg: s, segTime: segTime, interval: interval,
			merged: merged, flags: node.flags, parent: node, path: node.path + "/" + name,
		}
	}
	if name == "_ALL" {
		return leaf(node.seg.Total(), node.seg.FirstTime, node.wholeSecs(), len(node.seg.Segments)), nil
	}
	owners := segmentSecOwners(node.seg.FirstTime, node.seg.Interval, len(node.seg.Segments))
	if i, ok := owners[name]; ok {
		return leaf(node.seg.Segments[i], segmentStart(node.seg.FirstTime, node.seg.Interval, i), node.seg.Interval, 1), nil
	}
	return nil, fmt.Errorf("time segment not found: %s", name)
}

func (node *targetWindowNode) GetLeafData() map[string]string {
	if status := windowStatus(node.seg, node.window); status != "" {
		return map[string]string{"Status": status}
	}
	data := map[string]string{
		"00:Total (last " + node.window + ")": formatTargetWindow(node.seg),
		"01:Coverage":                         windowCoverage(node.seg.FirstTime, node.wholeSecs()),
	}
	idx := 2
	for i := range node.seg.Segments {
		s := node.seg.Segments[i]
		if targetSegmentEmpty(&s) {
			continue
		}
		start := segmentStart(node.seg.FirstTime, node.seg.Interval, i)
		end := start.Add(time.Duration(node.seg.Interval) * time.Second)
		data[segmentRowKey(idx, start, end)] = describeTargetSegment(s, node.seg.Interval)
		idx++
	}
	// The totals above are measured zeros, so they stay; only the reason there is
	// no segment row is added.
	if idx == 2 {
		data["02:Segments"] = fmt.Sprintf("no delivery recorded in any of the %d segment(s) measured",
			len(node.seg.Segments))
	}
	return data
}

// describeTargetSegment renders one delivery segment on a single line. The batch
// factor and the mean latency are derived here; the wire carries only sums.
func describeTargetSegment(s madmin.TargetSegment, interval int) string {
	out := strconv.FormatUint(s.Events, 10) + " events"
	if interval > 0 {
		out += fmt.Sprintf(" (%.1f/s)", float64(s.Events)/float64(interval))
	}
	if s.Requests > 0 {
		out += fmt.Sprintf(", %d reqs (%.1f ea, avg %s)", s.Requests,
			float64(s.Events)/float64(s.Requests), durationOf(s.RequestNanos, s.Requests))
	}
	if s.WriterErrors > 0 {
		out += fmt.Sprintf(", %d failed", s.WriterErrors)
	}
	if dropped := droppedInSegment(s); dropped > 0 {
		out += fmt.Sprintf(", %d dropped", dropped)
	}
	return out
}

// targetSegmentLeafNode is one target within one time segment, or a cross-target
// summary (key == "_ALL").
type targetSegmentLeafNode struct {
	key      string
	kind     string
	seg      madmin.TargetSegment
	segTime  time.Time
	interval int
	// merged is how many segment values were summed into seg, so N can be divided
	// back into a node count: one per segment for a whole window, one per target for
	// a cross-target summary.
	merged int
	flags  madmin.MetricFlags
	parent MetricNode
	path   string
}

func (node *targetSegmentLeafNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *targetSegmentLeafNode) GetMetricType() madmin.MetricType   { return madmin.MetricsTargets }
func (node *targetSegmentLeafNode) GetMetricFlags() madmin.MetricFlags { return node.flags }
func (node *targetSegmentLeafNode) GetParent() MetricNode              { return node.parent }
func (node *targetSegmentLeafNode) GetPath() string                    { return node.path }
func (node *targetSegmentLeafNode) ShouldPauseRefresh() bool           { return true }
func (node *targetSegmentLeafNode) GetChildren() []MetricChild         { return []MetricChild{} }

func (node *targetSegmentLeafNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children")
}

func (node *targetSegmentLeafNode) GetLeafData() map[string]string {
	s := node.seg
	if targetSegmentEmpty(&s) {
		return map[string]string{"Status": "no delivery activity in this time segment"}
	}
	data := map[string]string{}
	idx := 0
	add := func(k, v string) {
		data[fmt.Sprintf("%02d:%s", idx, k)] = v
		idx++
	}
	if node.key != "" && node.key != "_ALL" {
		add("Target", node.key)
	}
	if node.kind != "" {
		add("Class", node.kind)
	}
	if node.interval > 0 {
		add("Time Segment", windowCoverage(node.segTime, node.interval))
	}
	add("Events", strconv.FormatUint(s.Events, 10))
	if node.interval > 0 {
		add("Event Rate", fmt.Sprintf("%.2f/s", float64(s.Events)/float64(node.interval)))
	}
	// Requests are delivery attempts, not events: the ratio is the batch factor.
	if s.Requests > 0 {
		add("Requests", strconv.FormatUint(s.Requests, 10))
		add("Batch Factor", fmt.Sprintf("%.1f events per request", float64(s.Events)/float64(s.Requests)))
		add("Mean Latency", durationOf(s.RequestNanos, s.Requests))
	}
	// No share of requests beside it: attempts are retried and a batching target
	// increments only this, so the ratio can exceed 100%.
	if s.WriterErrors > 0 {
		add("Writer Errors", strconv.FormatUint(s.WriterErrors, 10))
	}
	reasons := 0
	for _, drop := range []struct {
		label string
		n     uint64
	}{
		{"Dropped (queue full)", s.DroppedQueueFull},
		{"Dropped (retries exhausted)", s.DroppedRetriesExhausted},
		{"Dropped (shutdown)", s.DroppedShutdown},
		{"Dropped (other)", s.DroppedOther},
	} {
		if drop.n > 0 {
			add(drop.label, strconv.FormatUint(drop.n, 10))
			reasons++
		}
	}
	// Only when it is more than one reason restated: with a single reason the total
	// is the same number under a second label.
	if reasons > 1 {
		add("Dropped Total", strconv.FormatUint(droppedInSegment(s), 10))
	}
	// Context only: every field above is a cross-node sum, and dividing one by the
	// node count would invent a per-node figure that moves when a node joins or
	// leaves. N itself is per merged value, so it is divided back down.
	if s.N > 0 {
		add("Nodes", formatNodeCount(s.N, node.merged))
	}
	return data
}

// logVolumeLines totals the lines a segment emitted. The wire carries no total.
func logVolumeLines(s madmin.LogVolumeSegment) uint64 {
	return s.ErrorLines + s.WarningLines + s.FatalLines + s.InfoLines + s.EventLines
}

// logVolumeSegmentEmpty reports a segment where nothing was logged. N is not part
// of the test: a quiet node still reports itself in every slot.
func logVolumeSegmentEmpty(s *madmin.LogVolumeSegment) bool {
	return logVolumeLines(*s) == 0
}

// logVolumeWindowNode is the node-level log-line window -- the hour or the day --
// as a row per time segment plus one navigable child per segment.
type logVolumeWindowNode struct {
	seg    *madmin.SegmentedLogVolume
	flags  madmin.MetricFlags
	window string
	parent MetricNode
	path   string
}

func (node *logVolumeWindowNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *logVolumeWindowNode) GetMetricType() madmin.MetricType   { return madmin.MetricsTargets }
func (node *logVolumeWindowNode) GetMetricFlags() madmin.MetricFlags { return node.flags }
func (node *logVolumeWindowNode) GetParent() MetricNode              { return node.parent }
func (node *logVolumeWindowNode) GetPath() string                    { return node.path }
func (node *logVolumeWindowNode) ShouldPauseRefresh() bool           { return true }

func (node *logVolumeWindowNode) hasSegments() bool {
	return node.seg != nil && node.seg.Interval > 0 && len(node.seg.Segments) > 0
}

// wholeSecs is the span the whole window covers: one segment as wide as every
// slot it holds, so an _ALL rate reads as the window average.
func (node *logVolumeWindowNode) wholeSecs() int {
	return node.seg.Interval * len(node.seg.Segments)
}

func (node *logVolumeWindowNode) GetChildren() []MetricChild {
	if !node.hasSegments() {
		return []MetricChild{}
	}
	children := []MetricChild{{
		Name: "_ALL",
		Description: "Every segment in the window combined. " +
			windowCoverage(node.seg.FirstTime, node.wholeSecs()),
	}}
	owners := segmentSecOwners(node.seg.FirstTime, node.seg.Interval, len(node.seg.Segments))
	withDate := windowCrossesDay(node.seg.FirstTime, node.seg.Interval, len(node.seg.Segments))
	for i := range node.seg.Segments {
		s := node.seg.Segments[i]
		if logVolumeSegmentEmpty(&s) {
			continue
		}
		start := segmentStart(node.seg.FirstTime, node.seg.Interval, i)
		name := segmentKey(start)
		if owners[name] != i {
			continue
		}
		children = append(children, MetricChild{
			Name:        name,
			Description: segmentDescTime(start, withDate) + ", " + describeLogVolumeSegment(s, node.seg.Interval),
		})
	}
	return children
}

func (node *logVolumeWindowNode) GetChild(name string) (MetricNode, error) {
	if !node.hasSegments() {
		return nil, fmt.Errorf("no last-%s log volume segments available", node.window)
	}
	leaf := func(s madmin.LogVolumeSegment, segTime time.Time, interval, merged int) MetricNode {
		return &logVolumeSegmentLeafNode{
			seg: s, segTime: segTime, interval: interval,
			merged: merged, flags: node.flags, parent: node, path: node.path + "/" + name,
		}
	}
	if name == "_ALL" {
		return leaf(node.seg.Total(), node.seg.FirstTime, node.wholeSecs(), len(node.seg.Segments)), nil
	}
	owners := segmentSecOwners(node.seg.FirstTime, node.seg.Interval, len(node.seg.Segments))
	if i, ok := owners[name]; ok {
		return leaf(node.seg.Segments[i], segmentStart(node.seg.FirstTime, node.seg.Interval, i), node.seg.Interval, 1), nil
	}
	return nil, fmt.Errorf("time segment not found: %s", name)
}

func (node *logVolumeWindowNode) GetLeafData() map[string]string {
	if status := windowStatus(node.seg, node.window); status != "" {
		return map[string]string{"Status": status}
	}
	data := map[string]string{
		"00:Total (last " + node.window + ")": formatLogVolumeWindow(node.seg),
		"01:Coverage":                         windowCoverage(node.seg.FirstTime, node.wholeSecs()),
	}
	idx := 2
	for i := range node.seg.Segments {
		s := node.seg.Segments[i]
		if logVolumeSegmentEmpty(&s) {
			continue
		}
		start := segmentStart(node.seg.FirstTime, node.seg.Interval, i)
		end := start.Add(time.Duration(node.seg.Interval) * time.Second)
		data[segmentRowKey(idx, start, end)] = describeLogVolumeSegment(s, node.seg.Interval)
		idx++
	}
	// The totals above are measured zeros, so they stay; only the reason there is
	// no segment row is added.
	if idx == 2 {
		data["02:Segments"] = fmt.Sprintf("nothing logged in any of the %d segment(s) measured",
			len(node.seg.Segments))
	}
	return data
}

// describeLogVolumeSegment renders one segment on a single line. Errors and
// warnings are named even at zero, matching the summary line this hangs under.
func describeLogVolumeSegment(s madmin.LogVolumeSegment, interval int) string {
	out := fmt.Sprintf("%d errors, %d warnings", s.ErrorLines, s.WarningLines)
	for _, rest := range []struct {
		label string
		n     uint64
	}{
		{"fatal", s.FatalLines},
		{"info", s.InfoLines},
		{"events", s.EventLines},
	} {
		if rest.n > 0 {
			out += fmt.Sprintf(", %d %s", rest.n, rest.label)
		}
	}
	// Per minute rather than per second: a quarter-hour segment holding a handful of
	// lines rounds to 0.0/s and says nothing.
	if lines := logVolumeLines(s); lines > 0 && interval > 0 {
		out += fmt.Sprintf(", %.1f lines/min", float64(lines)*60/float64(interval))
	}
	return out
}

// logVolumeSegmentLeafNode is the lines emitted within one time segment, or the
// whole window combined.
type logVolumeSegmentLeafNode struct {
	seg      madmin.LogVolumeSegment
	segTime  time.Time
	interval int
	// merged is how many segment values were summed into seg, so N can be divided
	// back into a node count.
	merged int
	flags  madmin.MetricFlags
	parent MetricNode
	path   string
}

func (node *logVolumeSegmentLeafNode) GetOpts() madmin.MetricsOptions { return getNodeOpts(node) }
func (node *logVolumeSegmentLeafNode) GetMetricType() madmin.MetricType {
	return madmin.MetricsTargets
}

func (node *logVolumeSegmentLeafNode) GetMetricFlags() madmin.MetricFlags { return node.flags }
func (node *logVolumeSegmentLeafNode) GetParent() MetricNode              { return node.parent }
func (node *logVolumeSegmentLeafNode) GetPath() string                    { return node.path }
func (node *logVolumeSegmentLeafNode) ShouldPauseRefresh() bool           { return true }
func (node *logVolumeSegmentLeafNode) GetChildren() []MetricChild         { return []MetricChild{} }

func (node *logVolumeSegmentLeafNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children")
}

func (node *logVolumeSegmentLeafNode) GetLeafData() map[string]string {
	s := node.seg
	data := map[string]string{}
	idx := 0
	add := func(k, v string) {
		data[fmt.Sprintf("%02d:%s", idx, k)] = v
		idx++
	}
	if node.interval > 0 {
		add("Time Segment", windowCoverage(node.segTime, node.interval))
	}
	// Errors and warnings are rendered at zero: "0 errors at 14:05" is the
	// reassurance an operator is after.
	add("Errors", strconv.FormatUint(s.ErrorLines, 10))
	add("Warnings", strconv.FormatUint(s.WarningLines, 10))
	for _, rest := range []struct {
		label string
		n     uint64
	}{
		{"Fatal", s.FatalLines},
		{"Info", s.InfoLines},
		{"Events", s.EventLines},
	} {
		if rest.n > 0 {
			add(rest.label, strconv.FormatUint(rest.n, 10))
		}
	}
	// Lines emitted, so suppression already happened upstream: this is not a count
	// of errors that occurred.
	lines := logVolumeLines(s)
	add("Total Lines", strconv.FormatUint(lines, 10))
	if node.interval > 0 {
		add("Line Rate", fmt.Sprintf("%.2f/min", float64(lines)*60/float64(node.interval)))
	}
	// Context only: the severities are cross-node sums, so dividing one by the node
	// count would invent a per-node rate that moves when a node restarts. N itself is
	// per merged value, so it is divided back down.
	if s.N > 0 {
		add("Nodes", formatNodeCount(s.N, node.merged))
	}
	return data
}
