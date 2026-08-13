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

// IAMMetricsNode is the navigation node for identity and access management.
type IAMMetricsNode struct {
	iam    *madmin.IAMMetrics
	parent MetricNode
	path   string
}

// NewIAMMetricsNode constructs a new IAMMetricsNode.
func NewIAMMetricsNode(iam *madmin.IAMMetrics, parent MetricNode, path string) *IAMMetricsNode {
	return &IAMMetricsNode{iam: iam, parent: parent, path: path}
}

func (node *IAMMetricsNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *IAMMetricsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsIAM }
func (node *IAMMetricsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *IAMMetricsNode) GetParent() MetricNode              { return node.parent }
func (node *IAMMetricsNode) GetPath() string                    { return node.path }
func (node *IAMMetricsNode) ShouldPauseRefresh() bool           { return false }
func (node *IAMMetricsNode) GetChildren() []MetricChild         { return []MetricChild{} }

func (node *IAMMetricsNode) GetChild(name string) (MetricNode, error) {
	return nil, fmt.Errorf("child not found: %s", name)
}

func (node *IAMMetricsNode) GetLeafData() map[string]string {
	if node.iam == nil {
		return map[string]string{"Status": "IAM metrics not available"}
	}
	m := node.iam
	data := map[string]string{
		"Collected At": m.CollectedAt.Format("2006-01-02 15:04:05"),
		"Nodes":        strconv.Itoa(m.Nodes),
	}

	if c := m.Cache; c != nil {
		// Cluster-wide values, not per node.
		data["Policies"] = strconv.Itoa(c.Policies)
		data["Users"] = strconv.Itoa(c.RegularUsers)
		data["Service Accounts"] = fmt.Sprintf("%d (%d not parented to root)",
			c.ServiceAccounts, c.SvcAccNonRootParent)
		data["Groups"] = strconv.Itoa(c.Groups)
		data["Policy Mappings"] = fmt.Sprintf("%d user, %d group, %d STS",
			c.UserPolicyMappings, c.GroupPolicyMappings, c.STSPolicyMappings)
		// Cached, so it can lag the section's own timestamp.
		data["Inventory Sampled"] = fmt.Sprintf("%s ago",
			time.Since(c.SampledAt).Round(time.Second))
	} else {
		data["Inventory"] = "not available (node still initialising)"
	}

	// Persistence latency: the refresh and admin path, not the request path.
	for _, op := range sortedKeys(m.Store) {
		a := m.Store[op]
		data["Store "+op] = fmt.Sprintf("%d op(s), avg %s, max %s", a.Count,
			durationOf(a.AccTime, a.Count),
			time.Duration(a.MaxTime).Round(time.Millisecond))
	}
	return data
}
