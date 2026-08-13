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

func (node *TargetMetricsNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *TargetMetricsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsTargets }
func (node *TargetMetricsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *TargetMetricsNode) GetParent() MetricNode              { return node.parent }
func (node *TargetMetricsNode) GetPath() string                    { return node.path }
func (node *TargetMetricsNode) ShouldPauseRefresh() bool           { return false }

// GetChildren lists one child per configured target. The child name is the wire
// key, since a target name is only unique within its config subsystem.
func (node *TargetMetricsNode) GetChildren() []MetricChild {
	if node.targets == nil {
		return []MetricChild{}
	}
	t := node.targets
	children := make([]MetricChild, 0, len(t.Notification)+len(t.Audit)+len(t.Logs))
	for _, class := range []struct {
		kind string
		m    map[string]madmin.TargetMetrics
	}{
		{"notification", t.Notification},
		{"audit log", t.Audit},
		{"system log", t.Logs},
	} {
		for _, key := range sortedKeys(class.m) {
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

	// Log volume works even with no target configured.
	for _, level := range sortedKeys(t.LogVolume) {
		data["Logged "+level] = strconv.FormatUint(t.LogVolume[level], 10)
	}

	if t.Spill != nil && (t.Spill.Bytes > 0 || t.Spill.Files > 0) {
		data["Spilled To Disk"] = fmt.Sprintf("%s in %d file(s)",
			humanize.IBytes(uint64(t.Spill.Bytes)), t.Spill.Files)
	}

	// The failure states an operator scans for.
	var drops, queued, capacity uint64
	var degraded []string
	for _, m := range []map[string]madmin.TargetMetrics{t.Notification, t.Audit, t.Logs} {
		for key, tm := range m {
			for _, n := range tm.Drops {
				drops += n
			}
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
	if drops > 0 {
		data["Dropped Events"] = strconv.FormatUint(drops, 10)
	}
	if len(degraded) > 0 {
		sort.Strings(degraded)
		data["Not Online Everywhere"] = strings.Join(degraded, ", ")
	}
	return data
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

func (node *TargetNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *TargetNode) GetMetricType() madmin.MetricType   { return madmin.MetricsTargets }
func (node *TargetNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *TargetNode) GetParent() MetricNode              { return node.parent }
func (node *TargetNode) GetPath() string                    { return node.path }
func (node *TargetNode) ShouldPauseRefresh() bool           { return false }
func (node *TargetNode) GetChildren() []MetricChild         { return []MetricChild{} }

func (node *TargetNode) GetChild(name string) (MetricNode, error) {
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

	data["Events"] = strconv.FormatUint(t.TotalEvents, 10)
	// Count is requests, not events; the ratio is the batch factor.
	if t.TotalRequests > 0 {
		data["Requests"] = fmt.Sprintf("%d (%.1f events each)", t.TotalRequests,
			float64(t.TotalEvents)/float64(t.TotalRequests))
	}
	if t.FailedRequests > 0 {
		data["Failed Requests"] = strconv.FormatUint(t.FailedRequests, 10)
	}
	if t.WriterErrors > 0 {
		data["Writer Errors"] = strconv.FormatUint(t.WriterErrors, 10)
	}

	// The reasons sum to the total, so both can be shown.
	var drops uint64
	for _, reason := range sortedKeys(t.Drops) {
		data["Dropped ("+reason+")"] = strconv.FormatUint(t.Drops[reason], 10)
		drops += t.Drops[reason]
	}
	if drops > 0 {
		data["Dropped"] = strconv.FormatUint(drops, 10)
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
