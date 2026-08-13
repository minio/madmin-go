//
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
//

package mnav

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/minio/madmin-go/v4"
)

// DistJobMetricsNode is the navigation node for distributed (server-pool) jobs.
type DistJobMetricsNode struct {
	jobs   *madmin.DistJobMetrics
	parent MetricNode
	path   string
}

// NewDistJobMetricsNode constructs a new DistJobMetricsNode.
func NewDistJobMetricsNode(jobs *madmin.DistJobMetrics, parent MetricNode, path string) *DistJobMetricsNode {
	return &DistJobMetricsNode{jobs: jobs, parent: parent, path: path}
}

func (node *DistJobMetricsNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *DistJobMetricsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsDistJobs }
func (node *DistJobMetricsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *DistJobMetricsNode) GetParent() MetricNode              { return node.parent }
func (node *DistJobMetricsNode) GetPath() string                    { return node.path }
func (node *DistJobMetricsNode) ShouldPauseRefresh() bool           { return false }

// GetChildren lists one child per running job.
func (node *DistJobMetricsNode) GetChildren() []MetricChild {
	if node.jobs == nil || len(node.jobs.Jobs) == 0 {
		return []MetricChild{}
	}
	ids := make([]string, 0, len(node.jobs.Jobs))
	for id := range node.jobs.Jobs {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	children := make([]MetricChild, 0, len(ids))
	for _, id := range ids {
		job := node.jobs.Jobs[id]
		children = append(children, MetricChild{
			Name:        id,
			DisplayName: fmt.Sprintf("%s (%s)", id, job.Type),
			Description: fmt.Sprintf("%s on pool %d, %s", job.Type, job.PoolIdx, job.State),
		})
	}
	return children
}

// GetLeafData summarises every running job.
func (node *DistJobMetricsNode) GetLeafData() map[string]string {
	if node.jobs == nil || len(node.jobs.Jobs) == 0 {
		return map[string]string{"Status": "No distributed jobs running"}
	}
	data := map[string]string{}
	idx := 0
	add := func(k, v string) {
		data[fmt.Sprintf("%02d:%s", idx, k)] = v
		idx++
	}
	add("Jobs", fmt.Sprintf("%d", len(node.jobs.Jobs)))
	if !node.jobs.CollectedAt.IsZero() {
		add("Collection Time", node.jobs.CollectedAt.Format("15:04:05"))
	}

	ids := make([]string, 0, len(node.jobs.Jobs))
	for id := range node.jobs.Jobs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		add(id, describeDistJob(node.jobs.Jobs[id]))
	}
	return data
}

// GetChild descends into one job.
func (node *DistJobMetricsNode) GetChild(name string) (MetricNode, error) {
	if node.jobs == nil {
		return nil, fmt.Errorf("no distributed job metrics available")
	}
	job, ok := node.jobs.Jobs[name]
	if !ok {
		return nil, fmt.Errorf("unknown distributed job: %s", name)
	}
	return &distJobNode{job: job, parent: node, path: node.path + "/" + name}, nil
}

// distJobNode is one job, with its per-node breakdown.
type distJobNode struct {
	job    madmin.DistJobProgress
	parent MetricNode
	path   string
}

func (node *distJobNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *distJobNode) GetMetricType() madmin.MetricType   { return madmin.MetricsDistJobs }
func (node *distJobNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *distJobNode) GetParent() MetricNode              { return node.parent }
func (node *distJobNode) GetPath() string                    { return node.path }
func (node *distJobNode) ShouldPauseRefresh() bool           { return false }
func (node *distJobNode) GetChildren() []MetricChild         { return []MetricChild{} }

func (node *distJobNode) GetChild(name string) (MetricNode, error) {
	return nil, fmt.Errorf("unknown child: %s", name)
}

func (node *distJobNode) GetLeafData() map[string]string {
	data := map[string]string{}
	idx := 0
	add := func(k, v string) {
		data[fmt.Sprintf("%02d:%s", idx, k)] = v
		idx++
	}

	job := node.job
	add("Type", job.Type.String())
	add("State", job.State)
	add("Pool", fmt.Sprintf("%d", job.PoolIdx))
	if !job.StartTime.IsZero() {
		add("Started", job.StartTime.Format("2006-01-02 15:04:05"))
		// Elapsed is derived from the two timestamps rather than sent.
		end := job.UpdatedAt
		if !job.EndTime.IsZero() {
			end = job.EndTime
		}
		if !end.IsZero() && end.After(job.StartTime) {
			add("Elapsed", formatDuration(end.Sub(job.StartTime)))
		}
	}

	add("Items", fmt.Sprintf("%s done", humanize.Comma(job.ItemsDone)))
	if job.ItemsFailed > 0 || job.ItemsSkipped > 0 {
		add("Items Other", fmt.Sprintf("%s failed, %s skipped",
			humanize.Comma(job.ItemsFailed), humanize.Comma(job.ItemsSkipped)))
	}
	add("Bytes", describeDistJobBytes(job))
	if job.ItemsUnrecoverable > 0 || job.BytesUnrecoverable > 0 {
		// A subset of the failures, not an addition to them.
		add("Unrecoverable", fmt.Sprintf("%s objects, %s (possible data loss)",
			humanize.Comma(job.ItemsUnrecoverable), humanize.IBytes(uint64(job.BytesUnrecoverable))))
	}
	if job.CurrentBucket != "" {
		pos := job.CurrentBucket
		if job.CurrentObject != "" {
			pos += "/" + job.CurrentObject
		}
		add("Current", pos)
	}

	for i, n := range job.Nodes {
		state := "online"
		if !n.Online {
			state = fmt.Sprintf("offline after %d failures", n.ConsecutiveFails)
		}
		add(fmt.Sprintf("Node %d", i), fmt.Sprintf("%s: %s, %s items, %s (%s)",
			n.Host, state, humanize.Comma(n.ItemsDone),
			humanize.IBytes(uint64(max(n.BytesDone, 0))), currentWorkOf(n)))
	}
	return data
}

func currentWorkOf(n madmin.DistJobNodeStatus) string {
	if n.CurrentBucket == "" {
		return "idle"
	}
	return fmt.Sprintf("set %d, %s", n.CurrentSet, n.CurrentBucket)
}

// describeDistJob renders one job on a single line.
func describeDistJob(job madmin.DistJobProgress) string {
	parts := []string{
		job.Type.String(),
		job.State,
		fmt.Sprintf("pool %d", job.PoolIdx),
		fmt.Sprintf("%s items", humanize.Comma(job.ItemsDone)),
		describeDistJobBytes(job),
	}
	if job.ItemsFailed > 0 {
		parts = append(parts, fmt.Sprintf("%s failed", humanize.Comma(job.ItemsFailed)))
	}
	if len(job.Nodes) > 0 {
		parts = append(parts, fmt.Sprintf("%d nodes", len(job.Nodes)))
	}
	return strings.Join(parts, ", ")
}

// describeDistJobBytes renders progress against the job's own denominator.
// Percent is computed here rather than sent, since each job type defines its own
// denominator.
func describeDistJobBytes(job madmin.DistJobProgress) string {
	done := humanize.IBytes(uint64(max(job.BytesDone, 0)))
	if job.BytesTotal <= 0 {
		return done
	}
	pct := float64(job.BytesDone) / float64(job.BytesTotal) * 100
	return fmt.Sprintf("%s of %s (%.1f%%)", done, humanize.IBytes(uint64(job.BytesTotal)), pct)
}
