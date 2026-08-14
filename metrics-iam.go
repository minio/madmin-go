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

// IAMMetrics is the identity inventory, the cost of authorizing against it, and
// the latency of the store behind it.
//
// Three time bases, none of them cumulative since process start:
//
//   - Cache is instantaneous, a reading of the inventory as it stands.
//   - Auth and Store are the sliding last minute.
//   - LastHour and LastDay are segmented windows, present only when asked for.
type IAMMetrics struct {
	CollectedAt time.Time `json:"collected"`

	Nodes int `json:"nodes"`

	// Cache is the identity inventory. Every node holds the same data, so the merge
	// takes the most recently sampled copy and never sums.
	Cache *IAMCacheStats `json:"cache,omitempty"`

	// Auth is what authorizing requests cost over the last minute. This is the
	// request path.
	Auth *IAMAuthStats `json:"auth,omitempty"`

	// Store is IAM persistence latency over the last minute by operation: "save",
	// "load", "delete", "list". Node-local and summing.
	//
	// This is the refresh and admin path, not the request path -- requests read the
	// in-memory cache -- so a rise here delays IAM changes propagating rather than
	// slowing traffic. It is also low-frequency, so an idle minute reports nothing
	// and the windows below are where it is normally read.
	Store map[string]TimedAction `json:"store,omitempty"`

	// LastHour is 1-minute segments over the last hour, set only when
	// MetricsHourStats is requested. Non-nil with no segments means the window was
	// requested and nothing has completed in it yet.
	LastHour *SegmentedIAMMetrics `json:"lastHour,omitempty"`

	// LastDay is 15-minute segments over the last day, set only when
	// MetricsDayStats is requested.
	LastDay *SegmentedIAMMetrics `json:"lastDay,omitempty"`
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

	if other.Auth != nil {
		if m.Auth == nil {
			m.Auth = &IAMAuthStats{}
		}
		m.Auth.Merge(other.Auth)
	}

	// A requested-but-empty window is non-nil with no segments, so presence is
	// tested rather than length: dropping it here would read downstream as "not
	// requested".
	if other.LastHour != nil {
		if m.LastHour == nil {
			m.LastHour = &SegmentedIAMMetrics{}
		}
		m.LastHour.Add(other.LastHour)
	}
	if other.LastDay != nil {
		if m.LastDay == nil {
			m.LastDay = &SegmentedIAMMetrics{}
		}
		m.LastDay.Add(other.LastDay)
	}

	// Latest-wins on the sample time, never summed.
	if other.Cache == nil {
		return
	}
	if m.Cache == nil || m.Cache.SampledAt.Before(other.Cache.SampledAt) {
		c := *other.Cache
		m.Cache = &c
	}
}

// Resolution paths an authorization can take, in the order IAMSys.IsAllowed
// tests them. They are the keys of IAMAuthStats.ByPath and the split the
// segments carry, because their costs differ by orders of magnitude: a plugin
// call crosses the network, an owner check does nothing at all.
const (
	// IAMPathPlugin is an external authorization plugin. When one is configured it
	// answers every request, so this path is all-or-nothing per deployment.
	IAMPathPlugin = "plugin"

	// IAMPathOwner is the root or account owner, allowed without evaluating anything.
	IAMPathOwner = "owner"

	// IAMPathSTS is a temporary credential, resolved through its session policy and
	// then its parent's.
	IAMPathSTS = "sts"

	// IAMPathSvcAcct is a service account, resolved through its parent.
	IAMPathSvcAcct = "svcacct"

	// IAMPathUser is a regular user, resolved from the policy mappings.
	IAMPathUser = "user"
)

// IAMAuthStats is what authorizing requests cost over the last minute, split by
// how the identity had to be resolved.
//
// Everything here is node-local and summing.
type IAMAuthStats struct {
	// ByPath is keyed by the IAMPath constants. A path nobody took is absent rather
	// than zero, so a deployment with no plugin configured never reports one.
	ByPath map[string]TimedAction `json:"by_path,omitempty"`

	// Denied is authorizations that resolved and refused. A rise here is a client
	// or policy change, not a server fault.
	Denied uint64 `json:"denied,omitempty"`

	// Errors is authorizations that could not be resolved at all, which are refused
	// without a policy decision having been reached.
	Errors uint64 `json:"errors,omitempty"`

	// CacheMiss is regular-user authorizations that had to build their merged policy
	// rather than reuse a cached one. Against the IAMPathUser count it gives the
	// hit rate; sustained misses mean the cache is being invalidated faster than it
	// is filled.
	CacheMiss uint64 `json:"cache_miss,omitempty"`
}

// Merge other into s.
func (s *IAMAuthStats) Merge(other *IAMAuthStats) {
	if other == nil {
		return
	}
	mergeMap(&s.ByPath, other.ByPath)
	s.Denied += other.Denied
	s.Errors += other.Errors
	s.CacheMiss += other.CacheMiss
}

// SegmentedIAMMetrics is a time-segmented view of IAM activity.
type SegmentedIAMMetrics = Segmented[IAMSegment, *IAMSegment]

// IAMSegment is IAM activity over one time segment.
//
// Both axes it carries are plain sums, so segments merge across nodes and
// rescale to a coarser interval by addition. The one exception is MaxNanos,
// merged worst-wins.
//
// The inventory is deliberately absent: it is a gauge, and summing a gauge's
// samples over a segment means nothing.
//
// The paths are fixed fields rather than a map keyed by the IAMPath constants,
// because a map would be repeated in all 156 segments a node carries.
type IAMSegment struct {
	// Authorization counts and their summed latency, per resolution path. The mean
	// for a path is its Nanos over its Count, and its share of traffic is its Count
	// over the total.
	PluginCount  uint64 `json:"plugin_count,omitempty"`
	PluginNanos  uint64 `json:"plugin_ns,omitempty"`
	OwnerCount   uint64 `json:"owner_count,omitempty"`
	OwnerNanos   uint64 `json:"owner_ns,omitempty"`
	STSCount     uint64 `json:"sts_count,omitempty"`
	STSNanos     uint64 `json:"sts_ns,omitempty"`
	SvcAcctCount uint64 `json:"svcacct_count,omitempty"`
	SvcAcctNanos uint64 `json:"svcacct_ns,omitempty"`
	UserCount    uint64 `json:"user_count,omitempty"`
	UserNanos    uint64 `json:"user_ns,omitempty"`

	// MaxNanos is the slowest single authorization in the segment, whatever path it
	// took. One figure rather than one per path: a configured plugin answers every
	// request, so at most one path is ever in contention for it.
	MaxNanos uint64 `json:"max_ns,omitempty"`

	Denied    uint64 `json:"denied,omitempty"`
	Errors    uint64 `json:"errors,omitempty"`
	CacheMiss uint64 `json:"cache_miss,omitempty"`

	// Persistence counts and their summed latency. This is the refresh and admin
	// path, and it is infrequent enough that the segments are the only place it can
	// be seen at all.
	SaveCount   uint64 `json:"save_count,omitempty"`
	SaveNanos   uint64 `json:"save_ns,omitempty"`
	LoadCount   uint64 `json:"load_count,omitempty"`
	LoadNanos   uint64 `json:"load_ns,omitempty"`
	DeleteCount uint64 `json:"delete_count,omitempty"`
	DeleteNanos uint64 `json:"delete_ns,omitempty"`
	ListCount   uint64 `json:"list_count,omitempty"`
	ListNanos   uint64 `json:"list_ns,omitempty"`

	// StoreErrors is failed persistence operations, all four kinds together: a
	// failing IAM store is one condition however it surfaced.
	StoreErrors uint64 `json:"store_errors,omitempty"`

	// N is contributing nodes.
	N int `json:"n"`
}

// Add other into s.
func (s *IAMSegment) Add(other *IAMSegment) {
	if other == nil {
		return
	}
	s.PluginCount += other.PluginCount
	s.PluginNanos += other.PluginNanos
	s.OwnerCount += other.OwnerCount
	s.OwnerNanos += other.OwnerNanos
	s.STSCount += other.STSCount
	s.STSNanos += other.STSNanos
	s.SvcAcctCount += other.SvcAcctCount
	s.SvcAcctNanos += other.SvcAcctNanos
	s.UserCount += other.UserCount
	s.UserNanos += other.UserNanos
	s.MaxNanos = max(s.MaxNanos, other.MaxNanos)
	s.Denied += other.Denied
	s.Errors += other.Errors
	s.CacheMiss += other.CacheMiss
	s.SaveCount += other.SaveCount
	s.SaveNanos += other.SaveNanos
	s.LoadCount += other.LoadCount
	s.LoadNanos += other.LoadNanos
	s.DeleteCount += other.DeleteCount
	s.DeleteNanos += other.DeleteNanos
	s.ListCount += other.ListCount
	s.ListNanos += other.ListNanos
	s.StoreErrors += other.StoreErrors
	s.N += other.N
}

// AuthCount is every authorization recorded in the segment, all paths together.
func (s IAMSegment) AuthCount() uint64 {
	return s.PluginCount + s.OwnerCount + s.STSCount + s.SvcAcctCount + s.UserCount
}

// AuthNanos is the summed authorization latency in the segment, all paths
// together.
func (s IAMSegment) AuthNanos() uint64 {
	return s.PluginNanos + s.OwnerNanos + s.STSNanos + s.SvcAcctNanos + s.UserNanos
}

// StoreCount is every persistence operation recorded in the segment.
func (s IAMSegment) StoreCount() uint64 {
	return s.SaveCount + s.LoadCount + s.DeleteCount + s.ListCount
}

// StoreNanos is the summed persistence latency in the segment.
func (s IAMSegment) StoreNanos() uint64 {
	return s.SaveNanos + s.LoadNanos + s.DeleteNanos + s.ListNanos
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
