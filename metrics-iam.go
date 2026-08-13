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

package madmin

import "time"

//go:generate go tool msgp -unexported -d clearomitted -d "tag json" -d "timezone utc" -d "maps binkeys" -file $GOFILE

// IAMMetrics is the identity inventory and the latency of the store behind it.
type IAMMetrics struct {
	CollectedAt time.Time `json:"collected"`

	Nodes int `json:"nodes"`

	// Cache is the identity inventory. Every node holds the same data, so the merge
	// takes the most recently sampled copy and never sums.
	Cache *IAMCacheStats `json:"cache,omitempty"`

	// Store is IAM persistence latency by operation: "save", "load", "delete",
	// "list". Node-local and summing.
	//
	// This is the refresh and admin path, not the request path -- requests read the
	// in-memory cache -- so a rise here delays IAM changes propagating rather than
	// slowing traffic.
	Store map[string]TimedAction `json:"store,omitempty"`
}

// Merge other into m.
func (m *IAMMetrics) Merge(other *IAMMetrics) {
	if other == nil {
		return
	}
	m.Nodes += other.Nodes
	if m.CollectedAt.Before(other.CollectedAt) {
		m.CollectedAt = other.CollectedAt
	}
	mergeMap(&m.Store, other.Store)

	// Latest-wins on the sample time, never summed.
	if other.Cache == nil {
		return
	}
	if m.Cache == nil || m.Cache.SampledAt.Before(other.Cache.SampledAt) {
		c := *other.Cache
		m.Cache = &c
	}
}

// IAMCacheStats is the identity inventory held in memory on every node.
//
// Cluster-replicated: one cluster-wide answer, not something to divide by the
// node count.
type IAMCacheStats struct {
	// SampledAt is when this copy was read. The merge selects on it, and the read is
	// cached so it can be older than the section's CollectedAt.
	SampledAt time.Time `json:"sampled_at"`

	// Policies excludes the built-in defaults, so a fresh cluster reports zero.
	Policies int `json:"policies,omitempty"`

	RegularUsers    int `json:"regular_users,omitempty"`
	ServiceAccounts int `json:"service_accounts,omitempty"`

	// SvcAccNonRootParent is service accounts not parented to root; those behave
	// differently on credential rotation.
	SvcAccNonRootParent int `json:"svcacc_non_root_parent,omitempty"`

	Groups int `json:"groups,omitempty"`

	UserPolicyMappings  int `json:"user_policy_mappings,omitempty"`
	GroupPolicyMappings int `json:"group_policy_mappings,omitempty"`
	STSPolicyMappings   int `json:"sts_policy_mappings,omitempty"`
}
