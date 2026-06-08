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

// DStateThreadInfo - identifies a single thread caught in uninterruptible
// disk sleep ("D" state) along with the kernel symbol it was blocked on
// (wchan) and its kernel stack at the moment of capture.
//
// Starttime is the kernel-reported thread start time in clock ticks since
// boot (field 22 of /proc/<tid>/stat); together with TID it forms a stable
// identity that survives PID/TID reuse over the dwell-history window.
//
// DwellSeconds is the minimum continuous time the thread has been in D state
// on the same wchan, derived from consecutive ring-buffer samples on the
// server. BlkIOSecondsInWindow is the cumulative block-I/O wait that
// accumulated during that same window (delayacct_blkio_ticks delta), a
// corroborating signal that the thread was actually waiting on I/O the
// whole time rather than just unlucky-at-sample.
//
// Stuck is true when DwellSeconds >= the server-side threshold; it is the
// single boolean consumers (health-analyzer, dashboards) should key off
// of so the threshold definition lives in one place.
type DStateThreadInfo struct {
	TID                  int     `json:"tid"`
	Starttime            uint64  `json:"starttime,omitempty"`
	Name                 string  `json:"name,omitempty"`
	Wchan                string  `json:"wchan,omitempty"`
	DwellSeconds         int     `json:"dwell_seconds,omitempty"`
	BlkIOSecondsInWindow float64 `json:"blkio_seconds_in_window,omitempty"`
	Stuck                bool    `json:"stuck,omitempty"`
	Stack                string  `json:"stack,omitempty"`
}

// DStateThreadGroup - groups D-state threads by their shared wchan, with a
// representative sample of full stacks. Count is the total threads currently
// in this wchan bucket; StuckCount is the subset of those whose DwellSeconds
// crossed the server's stuck threshold. Group sizes give an at-a-glance view
// of where threads are stuck cluster-wide.
type DStateThreadGroup struct {
	Wchan      string             `json:"wchan"`
	Count      int                `json:"count"`
	StuckCount int                `json:"stuck_count,omitempty"`
	Samples    []DStateThreadInfo `json:"samples,omitempty"`
}

// DStateThreadsDiag - per-node snapshot of threads currently in
// uninterruptible disk sleep, captured as part of `mc support diag`. Helps
// engineers debugging a stuck cluster see where threads are blocked without
// requiring SSH access.
//
// Total is the number of threads in D state at sample time. StuckTotal is
// the subset whose DwellSeconds has crossed StuckThresholdSeconds and is
// the count to alert on. TotalThreads is the size of the per-thread
// enumeration the sample was drawn from (i.e. all threads in the MinIO
// process, regardless of state); Total/TotalThreads gives saturation
// context for dense nodes.
//
// WindowSeconds reports how far back the server's dwell ring buffer
// covered when this snapshot was built — DwellSeconds for any thread is
// capped at this value, so a thread with DwellSeconds == WindowSeconds
// has been in D for at least the entire ring window.
type DStateThreadsDiag struct {
	NodeCommon

	Total                 int                 `json:"total"`
	StuckTotal            int                 `json:"stuck_total,omitempty"`
	TotalThreads          int                 `json:"total_threads,omitempty"`
	WindowSeconds         int                 `json:"window_seconds,omitempty"`
	StuckThresholdSeconds int                 `json:"stuck_threshold_seconds,omitempty"`
	Groups                []DStateThreadGroup `json:"groups,omitempty"`
}
