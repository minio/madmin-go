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

// DStateThreadInfo - one thread caught in uninterruptible disk sleep.
//
// Starttime is the kernel-reported start time in clock ticks (proc stat
// field 22); pairing it with TID gives a stable identity even when TIDs
// are reused over the server's dwell-history window.
//
// DwellSeconds is the minimum continuous time the thread has been in D
// on the same wchan. BlkIOSecondsInWindow is the corroborating
// delayacct_blkio_ticks delta. Stuck is the server-thresholded boolean
// so consumers don't redefine "stuck" locally.
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

// DStateThreadGroup groups D-state threads sharing a wchan. Count is all
// threads currently in this bucket; StuckCount is the subset past the
// server's dwell threshold.
type DStateThreadGroup struct {
	Wchan      string             `json:"wchan"`
	Count      int                `json:"count"`
	StuckCount int                `json:"stuck_count,omitempty"`
	Samples    []DStateThreadInfo `json:"samples,omitempty"`
}

// DStateThreadsDiag is the per-node snapshot of threads in
// uninterruptible disk sleep, captured by `mc support diag`.
//
// StuckTotal is the alertable number (Total can be non-zero on a healthy
// box doing I/O). TotalThreads gives saturation context for dense nodes.
// WindowSeconds bounds DwellSeconds — a thread reporting
// DwellSeconds == WindowSeconds has been in D for at least the entire
// ring window.
type DStateThreadsDiag struct {
	NodeCommon

	Total                 int                 `json:"total"`
	StuckTotal            int                 `json:"stuck_total,omitempty"`
	TotalThreads          int                 `json:"total_threads,omitempty"`
	WindowSeconds         int                 `json:"window_seconds,omitempty"`
	StuckThresholdSeconds int                 `json:"stuck_threshold_seconds,omitempty"`
	Groups                []DStateThreadGroup `json:"groups,omitempty"`
}
