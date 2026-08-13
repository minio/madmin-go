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
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/minio/madmin-go/v4"
)

// WarmTierMetricsNode is the navigation node for warm-storage-tier metrics.
type WarmTierMetricsNode struct {
	tier   *madmin.WarmTierMetrics
	parent MetricNode
	path   string
}

// NewWarmTierMetricsNode constructs a new WarmTierMetricsNode.
func NewWarmTierMetricsNode(tier *madmin.WarmTierMetrics, parent MetricNode, path string) *WarmTierMetricsNode {
	return &WarmTierMetricsNode{tier: tier, parent: parent, path: path}
}

func (node *WarmTierMetricsNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *WarmTierMetricsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsTier }
func (node *WarmTierMetricsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *WarmTierMetricsNode) GetParent() MetricNode              { return node.parent }
func (node *WarmTierMetricsNode) GetPath() string                    { return node.path }
func (node *WarmTierMetricsNode) ShouldPauseRefresh() bool           { return false }

// GetChildren lists one child per tier, including idle ones.
func (node *WarmTierMetricsNode) GetChildren() []MetricChild {
	if node.tier == nil || len(node.tier.Tiers) == 0 {
		return []MetricChild{}
	}
	children := make([]MetricChild, 0, len(node.tier.Tiers))
	for _, name := range sortedKeys(node.tier.Tiers) {
		st := node.tier.Tiers[name]
		desc := fmt.Sprintf("%s tier", st.Type)
		if len(st.Ops) == 0 {
			desc += " (configured, no activity)"
		}
		children = append(children, MetricChild{Name: name, Description: desc})
	}
	return children
}

func (node *WarmTierMetricsNode) GetChild(name string) (MetricNode, error) {
	if node.tier == nil {
		return nil, fmt.Errorf("no tier data available")
	}
	if st, ok := node.tier.Tiers[name]; ok {
		return NewWarmTierNode(st, name, node, fmt.Sprintf("%s/%s", node.path, name)), nil
	}
	return nil, fmt.Errorf("child not found: %s", name)
}

func (node *WarmTierMetricsNode) GetLeafData() map[string]string {
	if node.tier == nil {
		return map[string]string{"Status": "Tier metrics not available"}
	}
	t := node.tier
	data := map[string]string{
		"Collected At": t.CollectedAt.Format("2006-01-02 15:04:05"),
		"Nodes":        strconv.Itoa(t.Nodes),
		"Tiers":        strconv.Itoa(len(t.Tiers)),
	}

	// Cluster rollup plus the two states an operator scans for.
	var ops, errs uint64
	var idle, failing []string
	for _, name := range sortedKeys(t.Tiers) {
		st := t.Tiers[name]
		for _, a := range st.Ops {
			ops += a.Count
		}
		var tierErrs uint64
		for _, n := range st.Errors {
			tierErrs += n
		}
		errs += tierErrs
		if len(st.Ops) == 0 && st.LastSuccess.IsZero() {
			idle = append(idle, name)
		}
		if st.Errors[tierErrUnreachableKey] > 0 {
			failing = append(failing, name)
		}
	}
	data["Operations"] = strconv.FormatUint(ops, 10)
	if errs > 0 {
		data["Failures"] = strconv.FormatUint(errs, 10)
	}
	if len(failing) > 0 {
		data["Unreachable"] = strings.Join(failing, ", ")
	}
	if len(idle) > 0 {
		data["Never Used"] = strings.Join(idle, ", ")
	}
	return data
}

// tierErrUnreachableKey is the wire class for "the tier did not answer".
const tierErrUnreachableKey = "unreachable"

// WarmTierNode is one tier.
type WarmTierNode struct {
	tier   madmin.WarmTierStat
	name   string
	parent MetricNode
	path   string
}

// NewWarmTierNode constructs a new WarmTierNode.
func NewWarmTierNode(tier madmin.WarmTierStat, name string, parent MetricNode, path string) *WarmTierNode {
	return &WarmTierNode{tier: tier, name: name, parent: parent, path: path}
}

func (node *WarmTierNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *WarmTierNode) GetMetricType() madmin.MetricType   { return madmin.MetricsTier }
func (node *WarmTierNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *WarmTierNode) GetParent() MetricNode              { return node.parent }
func (node *WarmTierNode) GetPath() string                    { return node.path }
func (node *WarmTierNode) ShouldPauseRefresh() bool           { return false }
func (node *WarmTierNode) GetChildren() []MetricChild         { return []MetricChild{} }

func (node *WarmTierNode) GetChild(name string) (MetricNode, error) {
	return nil, fmt.Errorf("child not found: %s", name)
}

func (node *WarmTierNode) GetLeafData() map[string]string {
	t := node.tier
	data := map[string]string{
		"Name":            node.name,
		"Type":            t.Type,
		"Reporting Nodes": strconv.Itoa(t.N),
	}

	for _, op := range sortedKeys(t.Ops) {
		a := t.Ops[op]
		line := fmt.Sprintf("%d op(s), avg %s, max %s", a.Count,
			durationOf(a.AccTime, a.Count), time.Duration(a.MaxTime).Round(time.Millisecond))
		if a.Bytes > 0 {
			line += ", " + humanize.IBytes(a.Bytes)
		}
		data["Op "+op] = line
	}

	// The remote call alone, next to the last-byte figure above.
	if t.GetTTFB.Count > 0 {
		data["Get TTFB"] = fmt.Sprintf("avg %s, max %s over %d read(s)",
			durationOf(t.GetTTFB.AccTime, t.GetTTFB.Count),
			time.Duration(t.GetTTFB.MaxTime).Round(time.Millisecond), t.GetTTFB.Count)
		// Reads that started and never finished. Pruning a tier between a read's
		// TTFB and its close can invert the two, so this is a guarded subtraction.
		if done := t.Ops["get"].Count; t.GetTTFB.Count > done {
			data["Unfinished Reads"] = strconv.FormatUint(t.GetTTFB.Count-done, 10)
		}
	}

	var errs uint64
	for _, class := range sortedKeys(t.Errors) {
		data["Errors ("+class+")"] = strconv.FormatUint(t.Errors[class], 10)
		errs += t.Errors[class]
	}
	if errs > 0 {
		data["Errors"] = strconv.FormatUint(errs, 10)
	}

	if t.InflightPut > 0 {
		data["In Flight (put)"] = strconv.FormatInt(t.InflightPut, 10)
	}
	if t.InflightDelete > 0 {
		data["In Flight (delete)"] = strconv.FormatInt(t.InflightDelete, 10)
	}

	// Reachability without a prober.
	if t.LastSuccess.IsZero() {
		data["Last Success"] = "never"
	} else {
		data["Last Success"] = t.LastSuccess.Format("2006-01-02 15:04:05")
	}
	if t.LastError != "" {
		data["Last Error"] = t.LastError
		if !t.LastErrorTime.IsZero() {
			data["Last Error At"] = t.LastErrorTime.Format("2006-01-02 15:04:05")
		}
	}
	return data
}

// durationOf renders acc/count as a duration.
func durationOf(acc, count uint64) string {
	if count == 0 {
		return "n/a"
	}
	return time.Duration(acc / count).Round(time.Microsecond).String()
}
