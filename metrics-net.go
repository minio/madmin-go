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
	"time"

	"github.com/prometheus/procfs"
)

//go:generate go tool msgp -unexported -d clearomitted -d "tag json" -d "timezone utc" -d "maps binkeys" -file $GOFILE

//msgp:replace procfs.NetDevLine with:procfsNetDevLine

type NetMetrics struct {
	// Time these metrics were collected
	CollectedAt time.Time `json:"collected"`

	// NICs contains interface -> stats map.
	Interfaces map[string]InterfaceStats

	// Last day delta statistics.
	LastDay *SegmentedInterfaceStats `json:"last_day,omitempty"`

	// Last hour delta statistics (1-min segments).
	LastHour *SegmentedInterfaceStats `json:"last_hour,omitempty"`

	// Deprecated: Does not merge.
	InterfaceName string `json:"interfaceName"`

	// Internode Stats.
	NetStats procfs.NetDevLine `json:"netstats"`

	// RDMA is internode RDMA activity, nil when the transport is not enabled.
	// It rides MetricNet rather than a bit of its own -- see RDMAStats.
	RDMA *RDMAStats `json:"rdma,omitempty"`

	// Stack holds kernel TCP/IP/UDP counters. Absent off Linux.
	Stack *NetStackStats `json:"stack,omitempty"`

	// Conns holds TCP socket-table occupancy and accept-queue pressure.
	Conns *NetConnStats `json:"conns,omitempty"`

	// Links holds the aggregate link state of the physical interfaces. Note
	// this is a different interface set from Interfaces above and the two will
	// not have the same keys -- see NetLinkStats.
	Links *NetLinkStats `json:"links,omitempty"`

	// Headline TCP-stack counters over the last hour and day, populated when
	// MetricsHourStats / MetricsDayStats is requested.
	StackLastHour *SegmentedNetStack `json:"stack_last_hour,omitempty"`
	StackLastDay  *SegmentedNetStack `json:"stack_last_day,omitempty"`
}

// Merge other into 'o'.
func (n *NetMetrics) Merge(other *NetMetrics) {
	if other == nil {
		return
	}
	if n.CollectedAt.Before(other.CollectedAt) {
		// Use latest timestamp
		n.CollectedAt = other.CollectedAt
	}
	for k, v := range other.Interfaces {
		if n.Interfaces == nil {
			n.Interfaces = make(map[string]InterfaceStats, len(other.Interfaces))
		}
		n.Interfaces[k] = n.Interfaces[k].add(v)
	}
	if other.RDMA != nil {
		if n.RDMA == nil {
			n.RDMA = new(RDMAStats)
		}
		n.RDMA.Merge(other.RDMA)
	}
	if other.LastDay != nil && n.LastDay == nil {
		n.LastDay = new(SegmentedInterfaceStats)
	}
	n.LastDay.Add(other.LastDay)
	if other.LastHour != nil && n.LastHour == nil {
		n.LastHour = new(SegmentedInterfaceStats)
	}
	n.LastHour.Add(other.LastHour)
	n.NetStats = procfs.NetDevLine(procfsNetDevLine(n.NetStats).add(procfsNetDevLine(other.NetStats)))

	if other.Stack != nil {
		if n.Stack == nil {
			n.Stack = &NetStackStats{}
		}
		n.Stack.Merge(other.Stack)
	}
	if other.Conns != nil {
		if n.Conns == nil {
			n.Conns = &NetConnStats{}
		}
		n.Conns.Merge(other.Conns)
	}
	if other.Links != nil {
		if n.Links == nil {
			n.Links = &NetLinkStats{}
		}
		n.Links.Merge(other.Links)
	}
	if other.StackLastHour != nil {
		if n.StackLastHour == nil {
			n.StackLastHour = new(SegmentedNetStack)
		}
		n.StackLastHour.Add(other.StackLastHour)
	}
	if other.StackLastDay != nil {
		if n.StackLastDay == nil {
			n.StackLastDay = new(SegmentedNetStack)
		}
		n.StackLastDay.Add(other.StackLastDay)
	}
}

// InterfaceStats contains accumulated stats for a network interface.
type InterfaceStats struct {
	N                 int `json:"n"`
	procfs.NetDevLine `json:"stats"`
}

func (n *InterfaceStats) Add(other *InterfaceStats) {
	if other == nil || n == nil || other.N == 0 {
		return
	}
	n.N = n.N + other.N
	n.NetDevLine = procfs.NetDevLine(procfsNetDevLine(n.NetDevLine).add(procfsNetDevLine(other.NetDevLine)))
}

func (n InterfaceStats) add(other InterfaceStats) InterfaceStats {
	return InterfaceStats{
		N:          n.N + other.N,
		NetDevLine: procfs.NetDevLine(procfsNetDevLine(n.NetDevLine).add(procfsNetDevLine(other.NetDevLine))),
	}
}

// NetConnStats is TCP socket-table occupancy and accept-queue pressure.
type NetConnStats struct {
	// States maps the canonical TCP state name (ESTABLISHED, TIME_WAIT,
	// CLOSE_WAIT, SYN_RECV, ...) to a socket count. Bounded at around eleven
	// keys, and summing gives the cluster distribution.
	//
	// This is not a duplicate of NetStackStats.TCPCurrEstab: that counter is
	// RFC1213 tcpCurrEstab from /proc/net/snmp, while States comes from
	// sock_diag and is present only when netlink enumeration succeeded. The two
	// can legitimately disagree, and either can be absent alone.
	States map[string]uint64 `json:"states,omitempty"`

	// Backlog is accept-queue pressure across every listening socket.
	Backlog *NetListenerBacklogStats `json:"backlog,omitempty"`
}

// Merge other into c.
func (c *NetConnStats) Merge(other *NetConnStats) {
	if other == nil {
		return
	}
	addMap(&c.States, other.States)
	if other.Backlog != nil {
		if c.Backlog == nil {
			c.Backlog = &NetListenerBacklogStats{}
		}
		c.Backlog.Add(other.Backlog)
	}
}

// NetListenerBacklogStats is accept-queue pressure across all listening sockets
// in scope.
//
// Per-listener rows are deliberately not carried: they are keyed by local
// address, which is host-specific, so they cannot merge and would make
// Aggregated meaningless. The configured limit travels alongside the depth so a
// reader computes saturation itself rather than receiving a percentage.
type NetListenerBacklogStats struct {
	// N is the number of listening sockets contributing.
	N int `json:"n,omitempty"`
	// DepthSum is the summed queue depth; the mean is DepthSum/N.
	DepthSum uint64 `json:"depth_sum,omitempty"`
	// DepthMax is the deepest single queue in scope.
	DepthMax uint32 `json:"depth_max,omitempty"`
	// LimitMin is the smallest configured listen backlog in scope, i.e. the
	// first limit a connection burst will hit. The overflow events themselves
	// are NetStackStats.TCPListenOverflows and TCPListenDrops.
	LimitMin uint32 `json:"limit_min,omitempty"`
}

// Add other into b. Both extremes are guarded on N == 0 so the zero value is an
// unconditional identity for Add.
func (b *NetListenerBacklogStats) Add(other *NetListenerBacklogStats) {
	if other == nil || other.N == 0 {
		return
	}
	if b.N == 0 {
		b.DepthMax, b.LimitMin = other.DepthMax, other.LimitMin
	} else {
		b.DepthMax = max(b.DepthMax, other.DepthMax)
		// A zero limit means "not reported"; it must not win the minimum.
		if other.LimitMin > 0 && (b.LimitMin == 0 || other.LimitMin < b.LimitMin) {
			b.LimitMin = other.LimitMin
		}
	}
	b.DepthSum += other.DepthSum
	b.N += other.N
}

// NetLinkStats is the aggregate link state of every physical interface in
// scope, meaning those the kernel exposes a device entry for.
//
// Link state is a set of strings and integers with no summing semantics, and
// interface names collide across hosts, so a map keyed by interface name cannot
// be merged. It is carried as bounded value-to-count maps instead: one map
// yields the distribution, the mean and the outliers at once. A single
// SpeedMbps key means the fabric is homogeneous; a stray 1000 among 100000 is a
// mis-negotiated rail, visible without requesting ByHost.
//
// This is a different interface set from NetMetrics.Interfaces, which is derived
// from the addresses this node serves on. Interfaces is the traffic view and
// includes loopback and bridges; this is the hardware view, where speed, duplex
// and carrier state exist and where a flapping bond slave with no address of its
// own shows up. The two will not have the same keys, by design.
type NetLinkStats struct {
	// N is the number of interfaces contributing. It is also the link total,
	// and N minus OperStates["up"] is the number down, so neither is a field.
	N int `json:"n,omitempty"`

	// OperStates maps the interface operational state ("up", "down",
	// "dormant", "unknown") to an interface count.
	OperStates map[string]int `json:"oper_states,omitempty"`
	// Duplex maps "full", "half" or "unknown" to an interface count.
	Duplex map[string]int `json:"duplex,omitempty"`
	// SpeedMbps maps the negotiated link speed in Mbit/s to an interface count.
	// -1 means unknown, which is normal on virtual interfaces.
	SpeedMbps map[int64]int `json:"speed_mbps,omitempty"`
	// MTU maps the configured MTU in bytes to an interface count. One host
	// missing jumbo frames shows up as a second key.
	MTU map[uint32]int `json:"mtu,omitempty"`

	// CarrierChanges is the cumulative count of link up/down transitions since
	// boot. Non-zero and rising is a flapping link.
	CarrierChanges uint64 `json:"carrier_changes,omitempty"`
}

// Merge other into l.
func (l *NetLinkStats) Merge(other *NetLinkStats) {
	if other == nil {
		return
	}
	l.N += other.N
	addMap(&l.OperStates, other.OperStates)
	addMap(&l.Duplex, other.Duplex)
	addMap(&l.SpeedMbps, other.SpeedMbps)
	addMap(&l.MTU, other.MTU)
	l.CarrierChanges += other.CarrierChanges
}

// NetStackSegment carries the TCP-stack values worth trending.
//
// The delta counters answer "was there a retransmit storm at 03:00?".
// CurrEstab is a gauge, so it carries a sum (the mean is CurrEstabSum/N) and a
// max, which identifies the node that spiked. Mixing reductions is legal here
// because this is a segment: each field's reduction is in its name, and the one
// Add is both the within-bucket time collapse and the across-host merge.
type NetStackSegment struct {
	RetransSegs  uint64 `json:"retrans,omitempty"`
	InErrs       uint64 `json:"in_errs,omitempty"`
	OutRsts      uint64 `json:"out_rsts,omitempty"`
	AttemptFails uint64 `json:"attempt_fails,omitempty"`
	ListenDrops  uint64 `json:"listen_drops,omitempty"`
	SynRetrans   uint64 `json:"syn_retrans,omitempty"`

	CurrEstabSum uint64 `json:"curr_estab_sum,omitempty"`
	CurrEstabMax int64  `json:"curr_estab_max,omitempty"`

	N int `json:"n"`
}

// Add other to s for the Segmenter interface.
func (s *NetStackSegment) Add(other *NetStackSegment) {
	if other == nil || other.N == 0 {
		return
	}
	s.RetransSegs += other.RetransSegs
	s.InErrs += other.InErrs
	s.OutRsts += other.OutRsts
	s.AttemptFails += other.AttemptFails
	s.ListenDrops += other.ListenDrops
	s.SynRetrans += other.SynRetrans
	s.CurrEstabSum += other.CurrEstabSum
	if s.N == 0 {
		s.CurrEstabMax = other.CurrEstabMax
	} else {
		s.CurrEstabMax = max(s.CurrEstabMax, other.CurrEstabMax)
	}
	s.N += other.N
}

// SegmentedNetStack are time-segmented TCP-stack counters.
type SegmentedNetStack = Segmented[NetStackSegment, *NetStackSegment]

// RDMAStats is GPU-Direct / internode RDMA activity, hung off NetMetrics because
// it is a transport and a nil sub-object already says "not enabled".
//
// Every counter is raw and is never differenced server-side: P2pNicStats is filled
// field-by-field across the FFI boundary, so a snapshot can show completions
// leading sends and sent-minus-completed could underflow a uint64.
type RDMAStats struct {
	// Nodes is nodes with RDMA enabled, NICs the total across them. Compared against
	// the host count, Nodes shows a partial rollout.
	Nodes int `json:"nodes,omitempty"`
	NICs  int `json:"nics,omitempty"`

	ImmWritesSent      uint64 `json:"imm_writes_sent,omitempty"`
	ImmWritesCompleted uint64 `json:"imm_writes_completed,omitempty"`
	ImmWritesThrottled uint64 `json:"imm_writes_throttled,omitempty"`
	InflightImmWrites  int64  `json:"inflight_imm_writes,omitempty"`

	SendsSent       uint64 `json:"sends_sent,omitempty"`
	RecvCompletions uint64 `json:"recv_completions,omitempty"`
	RecvCqErrors    uint64 `json:"recv_cq_errors,omitempty"`

	SrqReposts         uint64 `json:"srq_reposts,omitempty"`
	MrRegisterFallback uint64 `json:"mr_register_fallback,omitempty"`

	// Credit-based flow control. Rising CreditWaits is the transport throttling
	// itself, which precedes throughput loss.
	GrantsSent        uint64 `json:"grants_sent,omitempty"`
	GrantsReceived    uint64 `json:"grants_received,omitempty"`
	CreditWaits       uint64 `json:"credit_waits,omitempty"`
	GrantSendTimeouts uint64 `json:"grant_send_timeouts,omitempty"`

	// Per-peer write-slot semaphores as four totals: a per-peer map has cluster-size
	// cardinality and its node-ID keys collide across hosts once merged.
	//
	// PeersSaturated is the outlier axis the ratio loses -- one blocked peer and every
	// peer blocked can give the same in-use/cap ratio.
	WriteSlotsInUse int64 `json:"write_slots_in_use,omitempty"`
	WriteSlotsCap   int64 `json:"write_slots_cap,omitempty"`
	Peers           int   `json:"peers,omitempty"`
	PeersSaturated  int   `json:"peers_saturated,omitempty"`

	// Pools are the Go staging pools keyed by role. The library's own inbound receive
	// pool is inside libp2p_rdma.so and is not exposed.
	Pools map[string]RDMAPoolStats `json:"pools,omitempty"`
}

// RDMAPoolStats is one staging buffer pool. Free and Cap, because the underlying
// channel holds the AVAILABLE slabs; in use is Cap - Free.
type RDMAPoolStats struct {
	FreeBytes int64 `json:"free_bytes,omitempty"`
	CapBytes  int64 `json:"cap_bytes,omitempty"`
}

// Add other into p.
func (p *RDMAPoolStats) Add(other *RDMAPoolStats) {
	if other == nil {
		return
	}
	p.FreeBytes += other.FreeBytes
	p.CapBytes += other.CapBytes
}

// Merge other into r.
func (r *RDMAStats) Merge(other *RDMAStats) {
	if other == nil {
		return
	}
	r.Nodes += other.Nodes
	r.NICs += other.NICs
	r.ImmWritesSent += other.ImmWritesSent
	r.ImmWritesCompleted += other.ImmWritesCompleted
	r.ImmWritesThrottled += other.ImmWritesThrottled
	r.InflightImmWrites += other.InflightImmWrites
	r.SendsSent += other.SendsSent
	r.RecvCompletions += other.RecvCompletions
	r.RecvCqErrors += other.RecvCqErrors
	r.SrqReposts += other.SrqReposts
	r.MrRegisterFallback += other.MrRegisterFallback
	r.GrantsSent += other.GrantsSent
	r.GrantsReceived += other.GrantsReceived
	r.CreditWaits += other.CreditWaits
	r.GrantSendTimeouts += other.GrantSendTimeouts
	r.WriteSlotsInUse += other.WriteSlotsInUse
	r.WriteSlotsCap += other.WriteSlotsCap
	r.Peers += other.Peers
	r.PeersSaturated += other.PeersSaturated
	mergeMap(&r.Pools, other.Pools)
}
