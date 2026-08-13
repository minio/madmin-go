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

//go:generate go tool msgp -d clearomitted -d "tag json" -d "timezone utc" -file $GOFILE

// NetStackStats contains kernel TCP/IP/UDP stack counters sourced from
// /proc/net/snmp and /proc/net/netstat. All counters are cumulative
// since boot; TCPCurrEstab is a point-in-time gauge.
type NetStackStats struct {
	// From /proc/net/snmp - present on all supported kernels.
	TCPActiveOpens  uint64 `json:"tcp_active_opens"`
	TCPPassiveOpens uint64 `json:"tcp_passive_opens"`
	TCPAttemptFails uint64 `json:"tcp_attempt_fails"`
	TCPEstabResets  uint64 `json:"tcp_estab_resets"`
	TCPCurrEstab    int64  `json:"tcp_curr_estab"`
	TCPInSegs       uint64 `json:"tcp_in_segs"`
	TCPOutSegs      uint64 `json:"tcp_out_segs"`
	TCPRetransSegs  uint64 `json:"tcp_retrans_segs"`
	TCPInErrs       uint64 `json:"tcp_in_errs"`
	TCPOutRsts      uint64 `json:"tcp_out_rsts"`
	IPInDiscards    uint64 `json:"ip_in_discards"`
	IPInHdrErrors   uint64 `json:"ip_in_hdr_errors"`
	UDPInErrors     uint64 `json:"udp_in_errors"`
	UDPRcvbufErrors uint64 `json:"udp_rcvbuf_errors"`
	UDPSndbufErrors uint64 `json:"udp_sndbuf_errors"`

	// From /proc/net/netstat (TcpExt) - availability varies by kernel
	// version; nil means the counter is absent on this kernel.
	TCPListenDrops      *uint64 `json:"tcp_listen_drops,omitempty"`
	TCPListenOverflows  *uint64 `json:"tcp_listen_overflows,omitempty"`
	TCPSynRetrans       *uint64 `json:"tcp_syn_retrans,omitempty"`
	TCPSyncookiesSent   *uint64 `json:"tcp_syncookies_sent,omitempty"`
	TCPSyncookiesRecv   *uint64 `json:"tcp_syncookies_recv,omitempty"`
	TCPSyncookiesFailed *uint64 `json:"tcp_syncookies_failed,omitempty"`
	TCPAbortOnTimeout   *uint64 `json:"tcp_abort_on_timeout,omitempty"`
	TCPAbortOnData      *uint64 `json:"tcp_abort_on_data,omitempty"`
	TCPSpuriousRTOs     *uint64 `json:"tcp_spurious_rtos,omitempty"`
	TCPLossUndo         *uint64 `json:"tcp_loss_undo,omitempty"`
	TCPOFOQueue         *uint64 `json:"tcp_ofo_queue,omitempty"`
	TCPRenoRecovery     *uint64 `json:"tcp_reno_recovery,omitempty"`
	TCPLostRetransmit   *uint64 `json:"tcp_lost_retransmit,omitempty"`
}

// Merge other into s.
//
// The always-present counters sum. TCPCurrEstab sums too: it is a gauge, but it
// counts sockets, so a cluster total is meaningful.
//
// The optional TcpExt counters are pointers because nil means "this kernel does
// not expose it", which must never collapse to zero: a nil in other leaves s
// untouched, and a nil in s adopts other's value. A cluster where only some
// kernels expose a counter therefore reports the sum over the kernels that do,
// rather than silently reading as a smaller number than reality.
func (s *NetStackStats) Merge(other *NetStackStats) {
	if other == nil {
		return
	}

	s.TCPActiveOpens += other.TCPActiveOpens
	s.TCPPassiveOpens += other.TCPPassiveOpens
	s.TCPAttemptFails += other.TCPAttemptFails
	s.TCPEstabResets += other.TCPEstabResets
	s.TCPCurrEstab += other.TCPCurrEstab
	s.TCPInSegs += other.TCPInSegs
	s.TCPOutSegs += other.TCPOutSegs
	s.TCPRetransSegs += other.TCPRetransSegs
	s.TCPInErrs += other.TCPInErrs
	s.TCPOutRsts += other.TCPOutRsts
	s.IPInDiscards += other.IPInDiscards
	s.IPInHdrErrors += other.IPInHdrErrors
	s.UDPInErrors += other.UDPInErrors
	s.UDPRcvbufErrors += other.UDPRcvbufErrors
	s.UDPSndbufErrors += other.UDPSndbufErrors

	addOptCounter(&s.TCPListenDrops, other.TCPListenDrops)
	addOptCounter(&s.TCPListenOverflows, other.TCPListenOverflows)
	addOptCounter(&s.TCPSynRetrans, other.TCPSynRetrans)
	addOptCounter(&s.TCPSyncookiesSent, other.TCPSyncookiesSent)
	addOptCounter(&s.TCPSyncookiesRecv, other.TCPSyncookiesRecv)
	addOptCounter(&s.TCPSyncookiesFailed, other.TCPSyncookiesFailed)
	addOptCounter(&s.TCPAbortOnTimeout, other.TCPAbortOnTimeout)
	addOptCounter(&s.TCPAbortOnData, other.TCPAbortOnData)
	addOptCounter(&s.TCPSpuriousRTOs, other.TCPSpuriousRTOs)
	addOptCounter(&s.TCPLossUndo, other.TCPLossUndo)
	addOptCounter(&s.TCPOFOQueue, other.TCPOFOQueue)
	addOptCounter(&s.TCPRenoRecovery, other.TCPRenoRecovery)
	addOptCounter(&s.TCPLostRetransmit, other.TCPLostRetransmit)
}

// NetConnStates contains TCP socket counts grouped by connection state
// plus accept-queue depths for listening sockets, sourced from netlink
// sock_diag (with a /proc/net/tcp fallback).
//
// This is the raw per-node collector shape. Realtime metrics carry
// NetConnStats instead, because Listeners is keyed by a host-specific local
// address and so cannot be merged across nodes.
type NetConnStates struct {
	// States maps canonical TCP state names (ESTABLISHED, TIME_WAIT,
	// CLOSE_WAIT, ...) to socket counts.
	States map[string]uint64 `json:"states,omitempty"`

	// Listeners holds per-listener accept-queue depths. Empty when the
	// /proc/net/tcp fallback was used (queue depths require sock_diag).
	Listeners []NetListenerBacklog `json:"listeners,omitempty"`
}

// NetListenerBacklog describes the accept-queue depth of one listening
// TCP socket.
type NetListenerBacklog struct {
	// LocalAddr is the listen address, e.g. "0.0.0.0:9000".
	LocalAddr string `json:"local_addr"`
	// Backlog is the current accept queue depth.
	Backlog uint32 `json:"backlog"`
	// MaxBacklog is the configured listen backlog limit.
	MaxBacklog uint32 `json:"max_backlog"`
}

// NetIfaceLink describes the link state of one network interface,
// sourced from /sys/class/net/<iface>/.
//
// This is the raw per-node collector shape. Realtime metrics carry NetLinkStats
// instead: interface names collide across hosts and these fields have no
// summing semantics, so a name-keyed map cannot be merged.
type NetIfaceLink struct {
	Name string `json:"name"`
	// OperState is the interface operational state ("up", "down", ...).
	OperState string `json:"oper_state,omitempty"`
	// SpeedMbps is the negotiated link speed; -1 when unknown (common
	// on virtual interfaces).
	SpeedMbps int64 `json:"speed_mbps"`
	// Duplex is "full", "half" or "unknown".
	Duplex string `json:"duplex,omitempty"`
	// MTU is the configured maximum transmission unit in bytes.
	MTU uint32 `json:"mtu"`
	// CarrierChanges counts link up/down transitions since boot.
	CarrierChanges uint64 `json:"carrier_changes"`
}
