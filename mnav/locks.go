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

	"github.com/minio/madmin-go/v4"
)

// LockMetricsNode is the navigation node for distributed-locking metrics.
type LockMetricsNode struct {
	locks  *madmin.LockMetrics
	parent MetricNode
	path   string
}

// NewLockMetricsNode constructs a new LockMetricsNode.
func NewLockMetricsNode(locks *madmin.LockMetrics, parent MetricNode, path string) *LockMetricsNode {
	return &LockMetricsNode{locks: locks, parent: parent, path: path}
}

func (node *LockMetricsNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *LockMetricsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsLocks }
func (node *LockMetricsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *LockMetricsNode) GetParent() MetricNode              { return node.parent }
func (node *LockMetricsNode) GetPath() string                    { return node.path }
func (node *LockMetricsNode) ShouldPauseRefresh() bool           { return false }

func (node *LockMetricsNode) GetChildren() []MetricChild {
	if node.locks == nil || node.locks.Purge == nil {
		return []MetricChild{}
	}
	return []MetricChild{
		{Name: "purge", Description: "Values sampled by the periodic lock-cleanup pass"},
	}
}

func (node *LockMetricsNode) GetChild(name string) (MetricNode, error) {
	if node.locks == nil {
		return nil, fmt.Errorf("no lock data available")
	}
	if name == "purge" {
		return NewLockPurgeNode(node.locks.Purge, node, fmt.Sprintf("%s/purge", node.path)), nil
	}
	return nil, fmt.Errorf("child not found: %s", name)
}

func (node *LockMetricsNode) GetLeafData() map[string]string {
	if node.locks == nil {
		return map[string]string{"Status": "Lock metrics not available"}
	}
	l := node.locks
	data := map[string]string{
		"Collected At": l.CollectedAt.Format("2006-01-02 15:04:05"),
		"Nodes":        strconv.Itoa(l.Nodes),
		// Resources, not locks: one resource can carry many readers.
		"Locked Resources": strconv.FormatInt(l.Resources, 10),
	}
	if l.Waiting > 0 {
		data["Waiting"] = strconv.FormatInt(l.Waiting, 10)
	}
	// Rising Rejected means contention has become a throughput problem.
	if l.Rejected > 0 {
		data["Rejected"] = strconv.FormatUint(l.Rejected, 10)
	}
	if l.ExpiredTotal > 0 {
		data["Expired (total)"] = strconv.FormatUint(l.ExpiredTotal, 10)
	}
	if l.QuorumLost > 0 {
		data["Quorum Lost"] = strconv.FormatUint(l.QuorumLost, 10)
	}
	return data
}

// LockPurgeNode is what the periodic lock-cleanup pass observed.
type LockPurgeNode struct {
	purge  *madmin.LockPurgeStats
	parent MetricNode
	path   string
}

// NewLockPurgeNode constructs a new LockPurgeNode.
func NewLockPurgeNode(purge *madmin.LockPurgeStats, parent MetricNode, path string) *LockPurgeNode {
	return &LockPurgeNode{purge: purge, parent: parent, path: path}
}

func (node *LockPurgeNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *LockPurgeNode) GetMetricType() madmin.MetricType   { return madmin.MetricsLocks }
func (node *LockPurgeNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *LockPurgeNode) GetParent() MetricNode              { return node.parent }
func (node *LockPurgeNode) GetPath() string                    { return node.path }
func (node *LockPurgeNode) ShouldPauseRefresh() bool           { return false }
func (node *LockPurgeNode) GetChildren() []MetricChild         { return []MetricChild{} }

func (node *LockPurgeNode) GetChild(name string) (MetricNode, error) {
	return nil, fmt.Errorf("child not found: %s", name)
}

func (node *LockPurgeNode) GetLeafData() map[string]string {
	if node.purge == nil {
		return map[string]string{"Status": "no cleanup pass has run yet"}
	}
	p := node.purge
	data := map[string]string{
		// Up to one cleanup interval old; not comparable with the live counters.
		"Sampled At": fmt.Sprintf("%s (%s ago)",
			p.SampledAt.Format("15:04:05"), time.Since(p.SampledAt).Round(time.Second)),
		"Read Locks":  strconv.FormatInt(p.Readers, 10),
		"Write Locks": strconv.FormatInt(p.Writers, 10),
	}
	if p.Expired > 0 {
		data["Expired (last pass)"] = strconv.FormatInt(p.Expired, 10)
	}
	// Age derived here; the wire carries only the timestamp.
	if !p.OldestHeldAt.IsZero() {
		data["Oldest Lock"] = fmt.Sprintf("held %s (since %s)",
			time.Since(p.OldestHeldAt).Round(time.Second),
			p.OldestHeldAt.Format("15:04:05"))
	}
	return data
}
