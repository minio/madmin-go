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
	"time"

	"github.com/minio/madmin-go/v4"
)

// byTimeName is the navigation key for the time-first ("_by_time") entry added
// under a last_day node. Leading underscore sorts/reads first, like _ALL.
const byTimeName = "_by_time"

// segPtr mirrors madmin.Segmented's own pointer constraint: a *T that can
// accumulate another *T. Every last_day metric family is a map[op]Segmented[T,PT].
type segPtr[T any] interface {
	*T
	madmin.Segmenter[T]
}

// segView adapts one metric family to the generic _by_time navigation. Only the
// leaf construction and the description/empty helpers differ per family; the
// tree logic in byTimeNode/byTimeSegmentNode is shared.
type segView[T any, PT segPtr[T]] struct {
	ops         map[string]madmin.Segmented[T, PT] // operation -> time-segmented stats
	metricType  madmin.MetricType
	metricFlags madmin.MetricFlags
	empty       func(*T) bool                                              // no activity -> filtered out
	segDesc     func(total T, interval int, segTime, end time.Time) string // _by_time list row
	opDesc      func(op string, s T, interval int) string                  // per-segment op list row
	opLeaf      func(op string, s T, segTime time.Time, interval int, parent MetricNode, path string) MetricNode
	sumLeaf     func(total T, segTime time.Time, interval int, parent MetricNode, path string) MetricNode
}

// newByTimeNode returns the "_by_time" entry: pick a time segment first, then see
// every operation active in it (with a cross-operation summary), then its stats.
func newByTimeNode[T any, PT segPtr[T]](v segView[T, PT], parent MetricNode, path string) MetricNode {
	return &byTimeNode[T, PT]{view: v, parent: parent, path: path}
}

// totalSegmented aligns every operation onto one timeline and sums them, giving
// the reference grid for the segment list and each slot's cross-operation summary.
// Segmented.Add handles operations whose FirstTime/length differ.
func totalSegmented[T any, PT segPtr[T]](ops map[string]madmin.Segmented[T, PT]) madmin.Segmented[T, PT] {
	var res madmin.Segmented[T, PT]
	for _, op := range sortedKeys(ops) {
		s := ops[op]
		res.Add(&s)
	}
	return res
}

// segmentAt returns the value of s for the segment starting at segTime, matched by
// timestamp against s's own FirstTime/Interval (not a shared slot index).
func segmentAt[T any, PT segPtr[T]](s madmin.Segmented[T, PT], segTime time.Time) (T, bool) {
	var zero T
	if s.Interval <= 0 {
		return zero, false
	}
	for i := range s.Segments {
		if s.FirstTime.Add(time.Duration(i*s.Interval) * time.Second).Equal(segTime) {
			return s.Segments[i], true
		}
	}
	return zero, false
}

// segmentSummary sums every operation's value at segTime (== totalSegmented slot).
func segmentSummary[T any, PT segPtr[T]](ops map[string]madmin.Segmented[T, PT], segTime time.Time) T {
	var total T
	pt := PT(&total)
	for _, op := range sortedKeys(ops) {
		if v, ok := segmentAt(ops[op], segTime); ok {
			pt.Add(&v)
		}
	}
	return total
}

// byTimeNode lists the last-day time segments, newest first, empties filtered.
type byTimeNode[T any, PT segPtr[T]] struct {
	view   segView[T, PT]
	parent MetricNode
	path   string
}

func (node *byTimeNode[T, PT]) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *byTimeNode[T, PT]) GetMetricType() madmin.MetricType   { return node.view.metricType }
func (node *byTimeNode[T, PT]) GetMetricFlags() madmin.MetricFlags { return node.view.metricFlags }
func (node *byTimeNode[T, PT]) GetParent() MetricNode              { return node.parent }
func (node *byTimeNode[T, PT]) GetPath() string                    { return node.path }
func (node *byTimeNode[T, PT]) ShouldPauseRefresh() bool           { return true }
func (node *byTimeNode[T, PT]) GetLeafData() map[string]string     { return nil }

func (node *byTimeNode[T, PT]) GetChildren() []MetricChild {
	total := totalSegmented(node.view.ops)
	var children []MetricChild
	for i := len(total.Segments) - 1; i >= 0; i-- {
		if node.view.empty(&total.Segments[i]) {
			continue
		}
		segTime := total.FirstTime.Add(time.Duration(i*total.Interval) * time.Second)
		end := segTime.Add(time.Duration(total.Interval) * time.Second)
		children = append(children, MetricChild{
			Name:        segTime.UTC().Format("15:04Z"),
			Description: node.view.segDesc(total.Segments[i], total.Interval, segTime, end),
		})
	}
	return children
}

func (node *byTimeNode[T, PT]) GetChild(name string) (MetricNode, error) {
	total := totalSegmented(node.view.ops)
	for i := range total.Segments {
		segTime := total.FirstTime.Add(time.Duration(i*total.Interval) * time.Second)
		if segTime.UTC().Format("15:04Z") == name {
			return &byTimeSegmentNode[T, PT]{
				view:     node.view,
				segTime:  segTime,
				interval: total.Interval,
				parent:   node,
				path:     node.path + "/" + name,
			}, nil
		}
	}
	return nil, fmt.Errorf("time segment not found: %s", name)
}

// byTimeSegmentNode lists the operations active in one time segment (sorted, with an
// _ALL summary first) and shows the cross-operation summary as its own leaf data.
type byTimeSegmentNode[T any, PT segPtr[T]] struct {
	view     segView[T, PT]
	segTime  time.Time
	interval int
	parent   MetricNode
	path     string
}

func (node *byTimeSegmentNode[T, PT]) GetOpts() madmin.MetricsOptions   { return getNodeOpts(node) }
func (node *byTimeSegmentNode[T, PT]) GetMetricType() madmin.MetricType { return node.view.metricType }
func (node *byTimeSegmentNode[T, PT]) GetMetricFlags() madmin.MetricFlags {
	return node.view.metricFlags
}
func (node *byTimeSegmentNode[T, PT]) GetParent() MetricNode    { return node.parent }
func (node *byTimeSegmentNode[T, PT]) GetPath() string          { return node.path }
func (node *byTimeSegmentNode[T, PT]) ShouldPauseRefresh() bool { return true }

func (node *byTimeSegmentNode[T, PT]) GetChildren() []MetricChild {
	children := []MetricChild{{Name: "_ALL", Description: "All operations combined for this time segment"}}
	for _, op := range sortedKeys(node.view.ops) {
		v, ok := segmentAt(node.view.ops[op], node.segTime)
		if !ok || node.view.empty(&v) {
			continue
		}
		children = append(children, MetricChild{
			Name:        op,
			Description: node.view.opDesc(op, v, node.interval),
		})
	}
	return children
}

func (node *byTimeSegmentNode[T, PT]) GetChild(name string) (MetricNode, error) {
	if name == "_ALL" {
		total := segmentSummary(node.view.ops, node.segTime)
		return node.view.sumLeaf(total, node.segTime, node.interval, node, node.path+"/_ALL"), nil
	}
	seg, ok := node.view.ops[name]
	if !ok {
		return nil, fmt.Errorf("operation not found: %s", name)
	}
	v, ok := segmentAt(seg, node.segTime)
	if !ok {
		return nil, fmt.Errorf("operation %s has no data for this time segment", name)
	}
	return node.view.opLeaf(name, v, node.segTime, node.interval, node, node.path+"/"+name), nil
}

func (node *byTimeSegmentNode[T, PT]) GetLeafData() map[string]string {
	total := segmentSummary(node.view.ops, node.segTime)
	return node.view.sumLeaf(total, node.segTime, node.interval, node, node.path).GetLeafData()
}
