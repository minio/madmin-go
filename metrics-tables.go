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

import (
	"maps"
	"time"
)

//go:generate go tool msgp -unexported -d clearomitted -d "tag json" -d "timezone utc" -d "maps binkeys" -file $GOFILE

// Non-traffic aggregates for the realtime tables section.
//
// These three carry three different reductions, which is why they are three
// sub-structs rather than fields on TableAPIMetrics: the catalog inventory is
// replicated to every node and must never be summed, the transaction counters
// are per-node and must be, and the maintenance jobs are leader-owned and must
// be selected between.

// TableCatalog is the cluster-wide catalog inventory.
//
// Every node reads the same registries, so these counts are identical
// everywhere: Merge takes the copy with the later SampledAt and NEVER sums.
// Summing would multiply every count by the node count, which looks entirely
// plausible and is wrong.
//
// Not time-segmented: inventory moves on human timescales, so a 15-minute point
// sample buys nothing over the current value. ActiveTxns is the one that spikes,
// and it is trended through TableHealth instead.
type TableCatalog struct {
	// SampledAt is when the inventory was gathered, and is the Merge selector.
	SampledAt time.Time `json:"sampled,omitzero"`

	Warehouses   int `json:"warehouses,omitempty"`
	Namespaces   int `json:"namespaces,omitempty"`
	Tables       int `json:"tables,omitempty"`
	Views        int `json:"views,omitempty"`
	StagedTables int `json:"staged_tables,omitempty"`
	ActiveTxns   int `json:"active_txns,omitempty"`
}

// TableHealth carries node-local transaction counters, cumulative since process
// start. Every field sums across hosts.
type TableHealth struct {
	// N is the number of nodes reporting.
	N int `json:"n"`

	// ZombieTxns counts transactions found abandoned by a crashed writer;
	// RecoveryOps counts the recovery passes that cleaned them up.
	ZombieTxns  uint64 `json:"zombie_txns,omitempty"`
	RecoveryOps uint64 `json:"recovery_ops,omitempty"`

	// Rollbacks maps the trigger to a count: "recovery" means a crashed
	// writer's transaction was rolled back, "commit" means a commit lost a
	// race. Those are very different stories, so they are not pre-summed. The
	// sum over this map is the total, so there is no total field.
	Rollbacks map[string]uint64 `json:"rollbacks,omitempty"`

	// VendedCreds maps the storage-credential vending outcome ("issued",
	// "denied", "insecure", "unscopable", "session-policy", "error") to a
	// count. Bounded by construction, and the auth-side signal that pairs with
	// the 4xx counts in the traffic stat.
	VendedCreds map[string]uint64 `json:"vended_creds,omitempty"`

	// PeerRPCs is compaction's peer-RPC count. It lives here, not on
	// TableMaintenanceJob, because it is the one maintenance counter that is
	// genuinely per-node: mixing it into a leader-owned struct would make that
	// struct's reduction ambiguous.
	PeerRPCs uint64 `json:"peer_rpcs,omitempty"`
}

// Add other into h.
func (h *TableHealth) Add(other *TableHealth) {
	if other == nil {
		return
	}
	h.N += other.N
	h.ZombieTxns += other.ZombieTxns
	h.RecoveryOps += other.RecoveryOps
	addMap(&h.Rollbacks, other.Rollbacks)
	addMap(&h.VendedCreds, other.VendedCreds)
	h.PeerRPCs += other.PeerRPCs
}

// TableMaintenanceJob is one background table-maintenance job.
//
// Each job runs under its own leader lock, so exactly one node's copy is
// authoritative -- but which node that is changes on failover, and a demoted
// leader keeps its stale totals in memory. Merge therefore selects per key and
// never sums: summing would double-count every cycle a former leader ever ran.
// Because every node collects (these are atomic loads) and Merge picks the
// winner, this is failover-proof without a leader lookup.
//
// Counters reset on failover: the new leader's atomics start at zero. State that
// in UI copy rather than trying to reconcile it inside the metric.
//
// Not time-segmented: Cycles, LastRun and LastCycleSecs already encode the
// history a post-mortem needs.
type TableMaintenanceJob struct {
	Cycles          uint64 `json:"cycles,omitempty"`
	TablesProcessed uint64 `json:"tables_processed,omitempty"`
	Errors          uint64 `json:"errors,omitempty"`
	Retries         uint64 `json:"retries,omitempty"`

	// ConfigsEnabled of ConfigsTotal warehouses have this job turned on.
	ConfigsEnabled int64 `json:"configs_enabled,omitempty"`
	ConfigsTotal   int64 `json:"configs_total,omitempty"`

	// Running reports a cycle in progress right now. LastRun is when the last
	// cycle completed, so a job running its first cycle has a zero LastRun.
	Running       bool      `json:"running,omitempty"`
	LastRun       time.Time `json:"last_run,omitzero"`
	LastCycleSecs float64   `json:"last_cycle_secs,omitempty"`

	// Work maps a job-type-specific unit of work to its cumulative count:
	// "snapshots_expired" for snapshot_expiration, "orphan_files_marked" for
	// unreferenced_file_removal, and "files_rewritten" / "files_added" /
	// "tables_skipped" for compaction.
	//
	// Keyed rather than fielded so a new maintenance job type adds keys instead
	// of wire fields, and so no job carries another job's structurally-zero
	// counters.
	Work map[string]uint64 `json:"work,omitempty"`
}

// fresherThan reports whether j is the more authoritative report of the same
// maintenance job than k.
//
// A node currently running a cycle wins, because that is the live leader.
// Otherwise the most recently completed cycle wins, then the highest cycle
// count. Ordering on Running first matters: a demoted leader can hold a *higher*
// lifetime cycle count than the node that has since taken over, so selecting on
// "most cycles" alone would pin the reported state to a node that stopped
// running the job. Immediately after a failover, before the new leader completes
// its first cycle, the demoted node's totals can still win; that is
// self-correcting within one cycle.
//
// The trailing comparisons exist only to make the selection a strict total
// order, so Merge is independent of merge order even when two reports tie.
func (j TableMaintenanceJob) fresherThan(k TableMaintenanceJob) bool {
	if j.Running != k.Running {
		return j.Running
	}
	if !j.LastRun.Equal(k.LastRun) {
		return j.LastRun.After(k.LastRun)
	}
	if j.Cycles != k.Cycles {
		return j.Cycles > k.Cycles
	}
	if j.TablesProcessed != k.TablesProcessed {
		return j.TablesProcessed > k.TablesProcessed
	}
	return j.Errors > k.Errors
}

// TableAPIMetrics holds traffic for all active tables aggregated across nodes.
type TableAPIMetrics struct {
	// Time these metrics were collected
	CollectedAt time.Time `json:"collected"`

	// Nodes responding with data
	Nodes int `json:"nodes"`

	// LastMinute is the aggregate over the last minute across all tables.
	LastMinute *TableAPIStat `json:"lastMinute,omitempty"`

	// LastHour is the aggregate over the last hour.
	// Populated only when MetricsHourStats is requested.
	LastHour *SegmentedTableIO `json:"lastHour,omitempty"`

	// LastDay is the aggregate over the last 24 hours.
	// Populated only when MetricsDayStats is requested.
	LastDay *SegmentedTableIO `json:"lastDay,omitempty"`

	// TopWarehouses contains the top 25 warehouses by request count if MetricsTopWarehouses is set.
	TopWarehouses *TopTableIO `json:"topW,omitempty"`

	// TopNamespaces contains the top 25 namespaces by request count if MetricsTopNamespaces is set.
	TopNamespaces *TopTableIO `json:"topN,omitempty"`

	// TopTables contains the top 25 tables by request count if MetricsTopTables is set.
	TopTables *TopTableIO `json:"topT,omitempty"`

	// Catalog is the cluster-wide catalog inventory. Populated only when
	// MetricsTablesCatalog is requested, and only by the node holding the
	// tables stats leader lock -- the walk behind it is object-layer I/O.
	Catalog *TableCatalog `json:"catalog,omitempty"`

	// Health is the node-local transaction counters, summed across hosts.
	Health *TableHealth `json:"health,omitempty"`

	// Maintenance maps a background maintenance job type
	// ("snapshot_expiration", "unreferenced_file_removal", "compaction") to
	// its state. Leader-owned: selected between, never summed.
	Maintenance map[string]TableMaintenanceJob `json:"maintenance,omitempty"`
}

// Merge folds other into t. CollectedAt takes the later timestamp;
// Nodes is summed; LastMinute is added; LastHour and LastDay are
// merged segment-wise; Top* groups are merged per-ranking and trimmed
// back to tableTopTrimTarget once any ranked list grows past tableTopTrimThreshold.
func (t *TableAPIMetrics) Merge(other *TableAPIMetrics) {
	if other == nil {
		return
	}
	if other.CollectedAt.After(t.CollectedAt) {
		t.CollectedAt = other.CollectedAt
	}
	t.Nodes += other.Nodes
	if other.LastMinute != nil {
		if t.LastMinute == nil {
			t.LastMinute = &TableAPIStat{}
		}
		t.LastMinute.Add(other.LastMinute)
	}
	if other.LastHour != nil {
		if t.LastHour == nil {
			t.LastHour = &SegmentedTableIO{}
		}
		t.LastHour.Merge(other.LastHour)
	}
	if other.LastDay != nil {
		if t.LastDay == nil {
			t.LastDay = &SegmentedTableIO{}
		}
		t.LastDay.Merge(other.LastDay)
	}
	t.TopWarehouses = mergeTopGroup(t.TopWarehouses, other.TopWarehouses, (*TableIOMetrics).KeyWarehouse)
	t.TopNamespaces = mergeTopGroup(t.TopNamespaces, other.TopNamespaces, (*TableIOMetrics).KeyNamespace)
	t.TopTables = mergeTopGroup(t.TopTables, other.TopTables, (*TableIOMetrics).KeyTable)

	// Replicated singleton: the freshest sample wins, never a sum. A node
	// that has not sampled yet reports nothing and cannot blank a real one.
	if other.Catalog != nil && (t.Catalog == nil || other.Catalog.SampledAt.After(t.Catalog.SampledAt)) {
		c := *other.Catalog
		t.Catalog = &c
	}
	if other.Health != nil {
		if t.Health == nil {
			t.Health = &TableHealth{}
		}
		t.Health.Add(other.Health)
	}
	// Leader-owned: select the authoritative report per job, never sum.
	for k, o := range other.Maintenance {
		if t.Maintenance == nil {
			t.Maintenance = make(map[string]TableMaintenanceJob, len(other.Maintenance))
		}
		if cur, ok := t.Maintenance[k]; !ok || o.fresherThan(cur) {
			// Clone Work: storing o wholesale would share the source report's map,
			// so a caller mutating the aggregate would reach back into it.
			o.Work = maps.Clone(o.Work)
			t.Maintenance[k] = o
		}
	}
}

// TopN re-ranks every ranked list in each Top* group and trims to n entries.
// Pass n <= 0 to leave the lists unchanged.
func (t *TableAPIMetrics) TopN(n int) {
	if n <= 0 {
		return
	}
	t.TopWarehouses.TopN(n)
	t.TopNamespaces.TopN(n)
	t.TopTables.TopN(n)
}

// TableIOMetrics holds traffic for a table across time windows.
type TableIOMetrics struct {
	// Table will be populated with table name if only one table.
	Table *string `json:"table"`

	// Namespace will be populated if all data is from the same namespace
	Namespace *string `json:"namespace"`

	// Warehouse will be populated if all data is from the same warehouse
	Warehouse *string `json:"warehouse"`

	TableAPIStat `msg:",flatten"`
}

// Merge sums other into m. Identity fields (Table/Namespace/Warehouse) are
// copied verbatim on the first non-empty merge and collapsed to nil on
// disagreement thereafter, preserving the "set iff single value" contract.
func (m *TableIOMetrics) Merge(other *TableIOMetrics) {
	if other == nil || other.IsZero() {
		return
	}
	if m.IsZero() {
		m.Table = other.Table
		m.Namespace = other.Namespace
		m.Warehouse = other.Warehouse
	} else {
		m.Table = mergeIdentityField(m.Table, other.Table)
		m.Namespace = mergeIdentityField(m.Namespace, other.Namespace)
		m.Warehouse = mergeIdentityField(m.Warehouse, other.Warehouse)
	}
	m.Add(&other.TableAPIStat)
}

type SegmentedTableIO struct {
	// IntervalSecs is the duration of each slot in seconds.
	IntervalSecs int `json:"intervalSecs"`

	// FirstTime is the timestamp of the oldest slot.
	FirstTime time.Time `json:"firstTime"`

	// Per-category counts; one slot per IntervalSecs.
	Reads           []int64   `json:"reads,omitempty"`
	Writes          []int64   `json:"writes,omitempty"`
	BytesIn         []int64   `json:"bytesIn,omitempty"`
	BytesOut        []int64   `json:"bytesOut,omitempty"`
	NotOK           []int64   `json:"notOk,omitempty"`
	RequestTimeSecs []float32 `json:"timeSecs,omitempty"`
	RespTTFBSecs    []float32 `json:"ttfbSecs,omitempty"`
	// Errors5xx and Canceled are subsets of NotOK; see TableAPIStat.
	Errors5xx []int64 `json:"err5,omitempty"`
	Canceled  []int64 `json:"cancel,omitempty"`
}

// Merge folds other into s. Slots are right-aligned and summed so the most
// recent slot always aligns; FirstTime extends to the earliest reported.
func (s *SegmentedTableIO) Merge(other *SegmentedTableIO) {
	if other == nil {
		return
	}
	if s.IntervalSecs == 0 {
		s.IntervalSecs = other.IntervalSecs
	}
	if s.FirstTime.IsZero() || (!other.FirstTime.IsZero() && other.FirstTime.Before(s.FirstTime)) {
		s.FirstTime = other.FirstTime
	}
	s.Reads = addSlices(s.Reads, other.Reads)
	s.Writes = addSlices(s.Writes, other.Writes)
	s.BytesIn = addSlices(s.BytesIn, other.BytesIn)
	s.BytesOut = addSlices(s.BytesOut, other.BytesOut)
	s.NotOK = addSlices(s.NotOK, other.NotOK)
	s.RequestTimeSecs = addSlices(s.RequestTimeSecs, other.RequestTimeSecs)
	s.RespTTFBSecs = addSlices(s.RespTTFBSecs, other.RespTTFBSecs)
	// addSlices right-aligns and extends, so a peer on an older build that
	// sends shorter (or absent) slices merges without panicking.
	s.Errors5xx = addSlices(s.Errors5xx, other.Errors5xx)
	s.Canceled = addSlices(s.Canceled, other.Canceled)
}

// AsTableIOStat returns one TableAPIStat per slot, right-aligned so the most
// recent slot is at the last index. Per-category slices shorter than the
// longest are aligned to the right (their oldest slots map to leading zeros).
func (s *SegmentedTableIO) AsTableIOStat() []TableAPIStat {
	if s == nil {
		return nil
	}
	n := len(s.Reads)
	for _, l := range []int{
		len(s.Writes), len(s.BytesIn), len(s.BytesOut), len(s.NotOK),
		len(s.RequestTimeSecs), len(s.RespTTFBSecs),
		len(s.Errors5xx), len(s.Canceled),
	} {
		if l > n {
			n = l
		}
	}
	if n == 0 {
		return nil
	}
	res := make([]TableAPIStat, n)
	for i, v := range s.Reads {
		res[n-len(s.Reads)+i].Reads = v
	}
	for i, v := range s.Writes {
		res[n-len(s.Writes)+i].Writes = v
	}
	for i, v := range s.BytesIn {
		res[n-len(s.BytesIn)+i].BytesIn = v
	}
	for i, v := range s.BytesOut {
		res[n-len(s.BytesOut)+i].BytesOut = v
	}
	for i, v := range s.NotOK {
		res[n-len(s.NotOK)+i].NotOK = v
	}
	for i, v := range s.RequestTimeSecs {
		res[n-len(s.RequestTimeSecs)+i].RequestTimeSecs = float64(v)
	}
	for i, v := range s.RespTTFBSecs {
		res[n-len(s.RespTTFBSecs)+i].RespTTFBSecs = float64(v)
	}
	for i, v := range s.Errors5xx {
		res[n-len(s.Errors5xx)+i].Errors5xx = v
	}
	for i, v := range s.Canceled {
		res[n-len(s.Canceled)+i].Canceled = v
	}
	return res
}

// TableAPIStat holds read/write counts and byte totals for a
// table over one time window.
// Read/Write is decided by the server based on API type.
type TableAPIStat struct {
	Reads           int64   `json:"r,omitempty"`     // Requests classified as reads
	Writes          int64   `json:"w,omitempty"`     // Requests classified as writes
	BytesIn         int64   `json:"in,omitempty"`    // Bytes in the Request body
	BytesOut        int64   `json:"out,omitempty"`   // Bytes in the Response body
	NotOK           int64   `json:"err,omitempty"`   // Response >= status code 400
	RequestTimeSecs float64 `json:"rSecs,omitempty"` // Total request time in seconds
	RespTTFBSecs    float64 `json:"ttfb,omitempty"`  // Total time spent on TTFB in seconds(req read -> response first byte) in seconds.

	// Errors5xx is the subset of NotOK with status >= 500, and Canceled the
	// subset whose client went away (status 499).
	//
	// NotOK remains the >= 400 total so no existing consumer breaks, which
	// makes this the one sum-and-part pair in the tables section. The identity
	// is 4xx = NotOK - Errors5xx - Canceled: 499 is counted in NotOK but is a
	// disconnect rather than a server error, matching how the per-operation API
	// stats classify it.
	Errors5xx int64 `json:"err5,omitempty"`
	Canceled  int64 `json:"cancel,omitempty"`
}

// Add sums other into t in place.
func (t *TableAPIStat) Add(other *TableAPIStat) {
	if other == nil {
		return
	}
	t.Reads += other.Reads
	t.Writes += other.Writes
	t.BytesIn += other.BytesIn
	t.BytesOut += other.BytesOut
	t.RequestTimeSecs += other.RequestTimeSecs
	t.RespTTFBSecs += other.RespTTFBSecs
	t.NotOK += other.NotOK
	t.Errors5xx += other.Errors5xx
	t.Canceled += other.Canceled
}

// IsZero reports whether all counters are zero.
func (t *TableAPIStat) IsZero() bool {
	return t == nil || t.Reads == 0 && t.Writes == 0
}

// KeyWarehouse returns the identity key used when merging warehouse-level
// Top entries.
func (m *TableIOMetrics) KeyWarehouse() string {
	return ptrStr(m.Warehouse)
}

// KeyNamespace returns the identity key used when merging namespace-level
// Top entries.
func (m *TableIOMetrics) KeyNamespace() string {
	return ptrStr(m.Warehouse) + tableKeySep + ptrStr(m.Namespace)
}

// KeyTable returns the identity key used when merging table-level Top
// entries.
func (m *TableIOMetrics) KeyTable() string {
	return ptrStr(m.Warehouse) + tableKeySep + ptrStr(m.Namespace) + tableKeySep + ptrStr(m.Table)
}
