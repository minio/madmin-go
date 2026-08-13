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

//go:generate go tool msgp -unexported -d clearomitted -d "tag json" -d "timezone utc" -d "maps binkeys" -file $GOFILE

import (
	"time"
)

// MemVMStat holds the /proc/vmstat kernel memory-management counters, cumulative
// since boot. Every field is a plain counter and sums across hosts.
//
// Page-unit counters are converted to bytes at collect time so no page size has
// to travel with them. Fault counters are left as counts, because they count
// faults and not pages -- conflating the two is exactly the bug that makes
// gopsutil's vmstat fields unusable.
type MemVMStat struct {
	// SwapInBytes and SwapOutBytes (pswpin/pswpout) are the clearest pre-OOM
	// signal: a box that has started swapping is already in trouble.
	SwapInBytes  uint64 `json:"swap_in_bytes,omitempty"`
	SwapOutBytes uint64 `json:"swap_out_bytes,omitempty"`

	// MajorFaults (pgmajfault) is page-cache and mmap thrash.
	MajorFaults uint64 `json:"major_faults,omitempty"`

	// OOMKill is the retrospective OOM signal, and the only counter that
	// survives the process that was killed.
	OOMKill uint64 `json:"oom_kill,omitempty"`

	// WorkingsetRefault is the canonical thrashing signal: pages evicted and
	// then immediately wanted again, i.e. the cache is too small.
	WorkingsetRefault uint64 `json:"workingset_refault,omitempty"`

	// Direct (synchronous) reclaim, which shows up as allocation-path latency
	// rather than as memory pressure.
	PgScanDirect  uint64 `json:"pgscan_direct,omitempty"`
	PgStealDirect uint64 `json:"pgsteal_direct,omitempty"`

	// Direct compaction and transparent-hugepage collapse failures. These are
	// the mechanism by which fragmentation becomes latency, so they are the
	// counters that confirm what MemFragStats predicts.
	CompactStall           uint64 `json:"compact_stall,omitempty"`
	CompactFail            uint64 `json:"compact_fail,omitempty"`
	THPCollapseAllocFailed uint64 `json:"thp_collapse_alloc_failed,omitempty"`
}

// Merge other into v.
func (v *MemVMStat) Merge(other *MemVMStat) {
	if other == nil {
		return
	}
	v.SwapInBytes += other.SwapInBytes
	v.SwapOutBytes += other.SwapOutBytes
	v.MajorFaults += other.MajorFaults
	v.OOMKill += other.OOMKill
	v.WorkingsetRefault += other.WorkingsetRefault
	v.PgScanDirect += other.PgScanDirect
	v.PgStealDirect += other.PgStealDirect
	v.CompactStall += other.CompactStall
	v.CompactFail += other.CompactFail
	v.THPCollapseAllocFailed += other.THPCollapseAllocFailed
}

// MemCgroupStats is cgroup-v2 memory accounting for this process's cgroup.
//
// Under Kubernetes it is the cgroup OOM killer, not the global one, that
// actually kills the server, so these are the numbers that explain a restart
// the global counters cannot. Every field is a plain sum; nil means the process
// is not under cgroup v2.
type MemCgroupStats struct {
	Current     uint64 `json:"current,omitempty"`
	Peak        uint64 `json:"peak,omitempty"`
	High        uint64 `json:"high,omitempty"`
	SwapCurrent uint64 `json:"swap_current,omitempty"`
	// Max is memory.max; 0 means unlimited.
	Max uint64 `json:"max,omitempty"`

	// Events maps a memory.events key to its cumulative count: "low", "high",
	// "max", "oom", "oom_kill", "oom_group_kill".
	//
	// Keyed rather than fielded because the kernel adds keys over time --
	// oom_group_kill arrived in 5.17 -- and a map absorbs them with no wire
	// change.
	Events map[string]uint64 `json:"events,omitempty"`
}

// Merge other into c.
func (c *MemCgroupStats) Merge(other *MemCgroupStats) {
	if other == nil {
		return
	}
	c.Current += other.Current
	c.Peak += other.Peak
	c.High += other.High
	c.SwapCurrent += other.SwapCurrent
	c.Max += other.Max
	addMap(&c.Events, other.Events)
}

// MemECCStats is the per-node ECC aggregate.
//
// Per-controller and per-DIMM detail stays in the Prometheus surface for
// cardinality reasons; this is the rollup plus the one axis that rollup would
// otherwise lose.
type MemECCStats struct {
	Controllers int `json:"controllers,omitempty"`
	DIMMs       int `json:"dimms,omitempty"`

	Corrected   uint64 `json:"corrected,omitempty"`
	Uncorrected uint64 `json:"uncorrected,omitempty"`

	// HardwareCorruptedBytes is MemInfo's HardwareCorrupted: memory the kernel
	// has retired because it failed. It belongs here rather than beside the
	// other meminfo gauges because it is an ECC outcome, and it sums like the
	// rest of this struct.
	HardwareCorruptedBytes uint64 `json:"hardware_corrupted_bytes,omitempty"`

	// DIMMsWithCorrected and DIMMsWithUncorrected count modules whose
	// respective counter is non-zero. This is the outlier axis without per-DIMM
	// cardinality: "one failing stick" and "the whole bank" produce the same
	// Corrected total and very different counts here.
	DIMMsWithCorrected   int `json:"dimms_with_corrected,omitempty"`
	DIMMsWithUncorrected int `json:"dimms_with_uncorrected,omitempty"`
}

// Merge other into e.
func (e *MemECCStats) Merge(other *MemECCStats) {
	if other == nil {
		return
	}
	e.Controllers += other.Controllers
	e.DIMMs += other.DIMMs
	e.Corrected += other.Corrected
	e.Uncorrected += other.Uncorrected
	e.HardwareCorruptedBytes += other.HardwareCorruptedBytes
	e.DIMMsWithCorrected += other.DIMMsWithCorrected
	e.DIMMsWithUncorrected += other.DIMMsWithUncorrected
}

// MemFragStats is the buddy-allocator view of free memory: not how much is
// free, but how much of it is reachable as contiguous runs.
//
// A host can sit at 40% free and still fail a large allocation, and it surfaces
// as latency -- direct compaction stalls, hugepage collapse failures, see
// MemVMStat.CompactStall -- rather than as memory pressure. That is why it
// belongs in the live view and not only in a scrape target.
//
// Both byte totals are plain sums, so the kernel's unusable-free-space index
// recomputed at any scope -- 1 - FreeBytesLarge/FreeBytes -- is that scope's
// free-memory-weighted mean. The ratio itself is never stored.
type MemFragStats struct {
	// Zones is the number of buddyinfo rows contributing.
	Zones int `json:"zones,omitempty"`

	// PageSize is the host page size in bytes, blanked to 0 once hosts disagree
	// -- at which point Orders is no longer interpretable but the byte totals
	// still are.
	PageSize uint64 `json:"page_size,omitempty"`

	// LargeOrderBytes is the allocation size the Large totals measure against:
	// the smallest PageSize<<order that is at least 2 MiB, which is where the
	// kernel starts compacting.
	//
	// 0 means no buddy order on this kernel reaches the threshold -- a lowered
	// CONFIG_FORCE_MAX_ZONEORDER -- in which case the Large totals are left
	// unpopulated rather than reading as 100% fragmented. Deriving it from
	// PageSize keeps the meaning identical on 64K-page arm64, and carrying it
	// means the threshold can move without renaming a field.
	LargeOrderBytes uint64 `json:"large_order_bytes,omitempty"`

	FreeBytes      uint64 `json:"free_bytes,omitempty"`
	FreeBytesLarge uint64 `json:"free_bytes_large,omitempty"`

	// ByZone maps "<numa-node>/<zone>" to that zone's row. Zone keys collide
	// across hosts, so the aggregate entry is a cluster total and the per-host
	// split is the normal ByHost one. Cardinality is NUMA nodes times zones:
	// a handful of rows that does not grow with cluster size.
	ByZone map[string]MemZoneFrag `json:"by_zone,omitempty"`
}

// MemZoneFrag is one /proc/buddyinfo row.
//
// The byte totals and Orders are not redundant: Orders is only interpretable
// while MemFragStats.PageSize is non-zero, and the byte totals survive a merge
// across hosts with different page sizes. Orders is kept so a support bundle can
// reconstruct the grid at any threshold without a follow-up SSH.
type MemZoneFrag struct {
	FreeBytes      uint64   `json:"free_bytes,omitempty"`
	FreeBytesLarge uint64   `json:"free_bytes_large,omitempty"`
	Orders         []uint64 `json:"orders,omitempty"` // free block count per buddy order, index = order
}

// Add other into z, element-wise, extending Orders to the longer slice.
func (z *MemZoneFrag) Add(other *MemZoneFrag) {
	if other == nil {
		return
	}
	z.FreeBytes += other.FreeBytes
	z.FreeBytesLarge += other.FreeBytesLarge
	if len(other.Orders) > len(z.Orders) {
		z.Orders = append(z.Orders, make([]uint64, len(other.Orders)-len(z.Orders))...)
	}
	for i, v := range other.Orders {
		z.Orders[i] += v
	}
}

// Merge other into f.
func (f *MemFragStats) Merge(other *MemFragStats) {
	if other == nil {
		return
	}
	// PageSize is a config echo: keep the common value, blank it once hosts
	// disagree so a reader cannot misinterpret Orders.
	if f.Zones == 0 {
		f.PageSize = other.PageSize
	} else if f.PageSize != other.PageSize {
		f.PageSize = 0
	}
	f.Zones += other.Zones
	f.LargeOrderBytes = max(f.LargeOrderBytes, other.LargeOrderBytes)
	f.FreeBytes += other.FreeBytes
	f.FreeBytesLarge += other.FreeBytesLarge
	mergeMap(&f.ByZone, other.ByZone)
}

type MemMetrics struct {
	// Time these metrics were collected
	CollectedAt time.Time `json:"collected"`

	Nodes int `json:"nodes"` // Note: Will be zero for older servers.

	Info MemInfo `json:"memInfo"`

	// VMStat holds the kernel memory-management counters, cumulative since boot.
	// Separate from Info because Info is a gauge snapshot and these are counters:
	// the two reduce differently and must not share a struct.
	VMStat *MemVMStat `json:"vmstat,omitempty"`

	// Cgroup is cgroup-v2 accounting for this process's cgroup; nil when not
	// running under cgroup v2. Under Kubernetes this, not Info, is the limit the
	// server is actually killed against.
	Cgroup *MemCgroupStats `json:"cgroup,omitempty"`

	// ECC is the per-node ECC rollup. Per-DIMM detail stays in the Prometheus
	// surface for cardinality reasons.
	ECC *MemECCStats `json:"ecc,omitempty"`

	// Fragmentation is the buddy-allocator view: not how much memory is free,
	// but how much of it is reachable as contiguous runs.
	Fragmentation *MemFragStats `json:"fragmentation,omitempty"`

	LastDay *SegmentedMemMetrics `json:"lastDay,omitempty"`

	// Last hour statistics (1-min segments).
	LastHour *SegmentedMemMetrics `json:"lastHour,omitempty"`
}

// Merge other into 'm'.
func (m *MemMetrics) Merge(other *MemMetrics) {
	if other == nil {
		return
	}
	m.Nodes += other.Nodes
	if m.CollectedAt.Before(other.CollectedAt) {
		// Use latest timestamp
		m.CollectedAt = other.CollectedAt
	}
	m.Info.Merge(&other.Info)
	if other.VMStat != nil {
		if m.VMStat == nil {
			m.VMStat = new(MemVMStat)
		}
		m.VMStat.Merge(other.VMStat)
	}
	if other.Cgroup != nil {
		if m.Cgroup == nil {
			m.Cgroup = new(MemCgroupStats)
		}
		m.Cgroup.Merge(other.Cgroup)
	}
	if other.ECC != nil {
		if m.ECC == nil {
			m.ECC = new(MemECCStats)
		}
		m.ECC.Merge(other.ECC)
	}
	if other.Fragmentation != nil {
		if m.Fragmentation == nil {
			m.Fragmentation = new(MemFragStats)
		}
		m.Fragmentation.Merge(other.Fragmentation)
	}
	if other.LastDay != nil {
		if m.LastDay == nil {
			m.LastDay = new(SegmentedMemMetrics)
		}
		m.LastDay.Add(other.LastDay)
	}
	if other.LastHour != nil {
		if m.LastHour == nil {
			m.LastHour = new(SegmentedMemMetrics)
		}
		m.LastHour.Add(other.LastHour)
	}
}

//msgp:replace NodeCommon with:nodeCommon

// nodeCommon - use as replacement for NodeCommon
// We do not want to give NodeCommon codegen, since it is used for embedding.
type nodeCommon struct {
	Addr  string `json:"addr"`
	Error string `json:"error,omitempty"`
}


// MemInfo contains system's RAM and swap information.
type MemInfo struct {
	// NodeCommon shouldn't be used since it cannot be merged.
	NodeCommon

	Total          uint64 `json:"total,omitempty"`
	Used           uint64 `json:"used,omitempty"`
	Free           uint64 `json:"free,omitempty"`
	Available      uint64 `json:"available,omitempty"`
	Shared         uint64 `json:"shared,omitempty"`
	Cache          uint64 `json:"cache,omitempty"`
	Buffers        uint64 `json:"buffer,omitempty"`
	SwapSpaceTotal uint64 `json:"swap_space_total,omitempty"`
	SwapSpaceFree  uint64 `json:"swap_space_free,omitempty"`
	// Limit will store cgroup limit if configured and
	// less than Total, otherwise same as Total
	Limit uint64 `json:"limit,omitempty"`
}

func (m *MemInfo) Merge(other *MemInfo) {
	if other == nil {
		return
	}
	if m.Total == 0 && m.Addr == "" {
		m.NodeCommon = other.NodeCommon
	} else if m.NodeCommon != other.NodeCommon {
		m.NodeCommon = NodeCommon{}
	}
	m.Total += other.Total
	m.Used += other.Used
	m.Free += other.Free
	m.Available += other.Available
	m.Shared += other.Shared
	m.Cache += other.Cache
	m.Buffers += other.Buffers
	m.SwapSpaceTotal += other.SwapSpaceTotal
	m.SwapSpaceFree += other.SwapSpaceFree
	m.Limit += other.Limit
}

// MemSegment contains compact memory metrics for time-series segmentation.
type MemSegment struct {
	Used      uint64 `json:"used,omitempty"`
	Free      uint64 `json:"free,omitempty"`
	Available uint64 `json:"available,omitempty"`
	Limit     uint64 `json:"limit,omitempty"`
	N         int    `json:"n"`

	// Deltas over the segment of the MemVMStat counters worth trending. Every
	// one is a plain sum, so a segment is directly comparable to its neighbours
	// and to the same segment on another host.
	SwapInBytes       uint64 `json:"swap_in_bytes,omitempty"`
	SwapOutBytes      uint64 `json:"swap_out_bytes,omitempty"`
	MajorFaults       uint64 `json:"major_faults,omitempty"`
	WorkingsetRefault uint64 `json:"workingset_refault,omitempty"`
	CompactStall      uint64 `json:"compact_stall,omitempty"`
	OOMKill           uint64 `json:"oom_kill,omitempty"`

	// Fragmentation gauge sums, with their own divisor: the free-memory totals
	// exist on every Linux host, but the buddyinfo source may be unreadable
	// while the meminfo one is fine, so dividing these by N would under-report.
	// 1 - FragFreeBytesLarge/FragFreeBytes is the segment's free-memory-weighted
	// unusable-free-space index; the ratio itself is never stored.
	FragFreeBytes      uint64 `json:"frag_free_bytes,omitempty"`
	FragFreeBytesLarge uint64 `json:"frag_free_bytes_large,omitempty"`
	FragN              int    `json:"frag_n,omitempty"`
}

// Add other to m for Segmenter interface.
func (m *MemSegment) Add(other *MemSegment) {
	if other == nil {
		return
	}
	m.Used += other.Used
	m.Free += other.Free
	m.Available += other.Available
	m.Limit += other.Limit
	m.N += other.N
	m.SwapInBytes += other.SwapInBytes
	m.SwapOutBytes += other.SwapOutBytes
	m.MajorFaults += other.MajorFaults
	m.WorkingsetRefault += other.WorkingsetRefault
	m.CompactStall += other.CompactStall
	m.OOMKill += other.OOMKill
	m.FragFreeBytes += other.FragFreeBytes
	m.FragFreeBytesLarge += other.FragFreeBytesLarge
	m.FragN += other.FragN
}

// SegmentedMemMetrics are time-segmented memory metrics.
type SegmentedMemMetrics = Segmented[MemSegment, *MemSegment]
