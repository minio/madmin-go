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
	"strconv"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/minio/madmin-go/v4"
)

// ILMMetricsNode is the navigation node for lifecycle worker-pool metrics.
type ILMMetricsNode struct {
	ilm    *madmin.ILMMetrics
	parent MetricNode
	path   string
}

// NewILMMetricsNode constructs a new ILMMetricsNode.
func NewILMMetricsNode(ilm *madmin.ILMMetrics, parent MetricNode, path string) *ILMMetricsNode {
	return &ILMMetricsNode{ilm: ilm, parent: parent, path: path}
}

func (node *ILMMetricsNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *ILMMetricsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsILM }
func (node *ILMMetricsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *ILMMetricsNode) GetParent() MetricNode              { return node.parent }
func (node *ILMMetricsNode) GetPath() string                    { return node.path }
func (node *ILMMetricsNode) ShouldPauseRefresh() bool           { return false }

func (node *ILMMetricsNode) GetChildren() []MetricChild {
	if node.ilm == nil {
		return []MetricChild{}
	}
	children := make([]MetricChild, 0, 3)
	if node.ilm.Transition != nil {
		children = append(children,
			MetricChild{Name: "transition", Description: "Transition worker pool: queue, throughput, failures"})
	}
	if node.ilm.Expiry != nil {
		children = append(children,
			MetricChild{Name: "expiry", Description: "Expiry worker pool: queue, throughput, failures"})
	}
	if node.ilm.Restore != nil {
		children = append(children,
			MetricChild{Name: "restore", Description: "Object restoration from a remote tier"})
	}
	return children
}

func (node *ILMMetricsNode) GetChild(name string) (MetricNode, error) {
	if node.ilm == nil {
		return nil, fmt.Errorf("no ILM data available")
	}
	switch name {
	case "transition":
		return NewILMQueueNode(node.ilm.Transition, "transition", node,
			fmt.Sprintf("%s/transition", node.path)), nil
	case "expiry":
		return NewILMQueueNode(node.ilm.Expiry, "expiry", node,
			fmt.Sprintf("%s/expiry", node.path)), nil
	case "restore":
		return NewILMRestoreNode(node.ilm.Restore, node, fmt.Sprintf("%s/restore", node.path)), nil
	default:
		return nil, fmt.Errorf("child not found: %s", name)
	}
}

func (node *ILMMetricsNode) GetLeafData() map[string]string {
	if node.ilm == nil {
		return map[string]string{"Status": "ILM metrics not available"}
	}
	data := map[string]string{
		"Collected At": node.ilm.CollectedAt.Format("2006-01-02 15:04:05"),
		"Nodes":        strconv.Itoa(node.ilm.Nodes),
	}
	// A nil pool means the object layer has not wired it up yet.
	for name, q := range map[string]*madmin.ILMQueueStats{
		"Transition": node.ilm.Transition, "Expiry": node.ilm.Expiry,
	} {
		if q == nil {
			data[name] = "not initialised"
			continue
		}
		data[name] = fmt.Sprintf("%d queued of %d, %d worker(s)", q.Queued, q.Capacity, q.Workers)
	}
	return data
}

// ILMQueueNode is one lifecycle worker pool.
type ILMQueueNode struct {
	queue  *madmin.ILMQueueStats
	name   string
	parent MetricNode
	path   string
}

// NewILMQueueNode constructs a new ILMQueueNode.
func NewILMQueueNode(queue *madmin.ILMQueueStats, name string, parent MetricNode, path string) *ILMQueueNode {
	return &ILMQueueNode{queue: queue, name: name, parent: parent, path: path}
}

func (node *ILMQueueNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *ILMQueueNode) GetMetricType() madmin.MetricType   { return madmin.MetricsILM }
func (node *ILMQueueNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *ILMQueueNode) GetParent() MetricNode              { return node.parent }
func (node *ILMQueueNode) GetPath() string                    { return node.path }
func (node *ILMQueueNode) ShouldPauseRefresh() bool           { return false }
func (node *ILMQueueNode) GetChildren() []MetricChild         { return []MetricChild{} }

func (node *ILMQueueNode) GetChild(name string) (MetricNode, error) {
	return nil, fmt.Errorf("child not found: %s", name)
}

func (node *ILMQueueNode) GetLeafData() map[string]string {
	if node.queue == nil {
		return map[string]string{"Status": "pool not initialised"}
	}
	q := node.queue
	data := map[string]string{"Pool": node.name}

	if q.Capacity > 0 {
		data["Queue"] = fmt.Sprintf("%d of %d (%s)", q.Queued, q.Capacity,
			calculatePercentage(uint64(max(q.Queued, 0)), uint64(q.Capacity)))
	} else {
		data["Queue"] = strconv.FormatInt(q.Queued, 10)
	}
	if q.Workers > 0 {
		data["Workers"] = fmt.Sprintf("%d busy of %d", q.Active, q.Workers)
	}
	// A missed task is discarded work, not backpressure.
	if q.Missed > 0 {
		data["Missed (dropped)"] = strconv.FormatUint(q.Missed, 10)
	}

	if q.Tasks.Count > 0 {
		data["Tasks"] = fmt.Sprintf("%d, avg %s, max %s", q.Tasks.Count,
			durationOf(q.Tasks.AccTime, q.Tasks.Count),
			time.Duration(q.Tasks.MaxTime).Round(time.Millisecond))
	}

	var errs uint64
	for _, class := range sortedKeys(q.Errors) {
		data["Errors ("+class+")"] = strconv.FormatUint(q.Errors[class], 10)
		errs += q.Errors[class]
	}
	if errs > 0 {
		data["Errors"] = strconv.FormatUint(errs, 10)
	}

	// A recent head means throughput-bound, an old one means wedged. Derived here.
	if !q.HeadQueuedAt.IsZero() {
		data["Head Lag"] = fmt.Sprintf("<= %s", time.Since(q.HeadQueuedAt).Round(time.Second))
	}
	return data
}

// ILMRestoreNode is object restoration from a remote tier.
type ILMRestoreNode struct {
	restore *madmin.ILMRestoreStats
	parent  MetricNode
	path    string
}

// NewILMRestoreNode constructs a new ILMRestoreNode.
func NewILMRestoreNode(restore *madmin.ILMRestoreStats, parent MetricNode, path string) *ILMRestoreNode {
	return &ILMRestoreNode{restore: restore, parent: parent, path: path}
}

func (node *ILMRestoreNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *ILMRestoreNode) GetMetricType() madmin.MetricType   { return madmin.MetricsILM }
func (node *ILMRestoreNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *ILMRestoreNode) GetParent() MetricNode              { return node.parent }
func (node *ILMRestoreNode) GetPath() string                    { return node.path }
func (node *ILMRestoreNode) ShouldPauseRefresh() bool           { return false }
func (node *ILMRestoreNode) GetChildren() []MetricChild         { return []MetricChild{} }

func (node *ILMRestoreNode) GetChild(name string) (MetricNode, error) {
	return nil, fmt.Errorf("child not found: %s", name)
}

func (node *ILMRestoreNode) GetLeafData() map[string]string {
	if node.restore == nil {
		return map[string]string{"Status": "no restores on this cluster"}
	}
	r := node.restore
	data := map[string]string{}
	if r.Restores.Count > 0 {
		data["Restores"] = fmt.Sprintf("%d, avg %s, max %s", r.Restores.Count,
			durationOf(r.Restores.AccTime, r.Restores.Count),
			time.Duration(r.Restores.MaxTime).Round(time.Millisecond))
		if r.Restores.Bytes > 0 {
			data["Restored"] = humanize.IBytes(r.Restores.Bytes)
		}
	}
	if r.Active > 0 {
		data["In Flight"] = strconv.FormatInt(r.Active, 10)
	}
	var errs uint64
	for _, class := range sortedKeys(r.Errors) {
		data["Errors ("+class+")"] = strconv.FormatUint(r.Errors[class], 10)
		errs += r.Errors[class]
	}
	if errs > 0 {
		data["Errors"] = strconv.FormatUint(errs, 10)
	}
	return data
}
