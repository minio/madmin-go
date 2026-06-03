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
type DStateThreadInfo struct {
	TID   int    `json:"tid"`
	Name  string `json:"name,omitempty"`
	Wchan string `json:"wchan,omitempty"`
	Stack string `json:"stack,omitempty"`
}

// DStateThreadGroup - groups D-state threads by their shared wchan, with a
// representative sample of full stacks. Group sizes give an at-a-glance view
// of where threads are stuck cluster-wide.
type DStateThreadGroup struct {
	Wchan   string             `json:"wchan"`
	Count   int                `json:"count"`
	Samples []DStateThreadInfo `json:"samples,omitempty"`
}

// DStateThreadsDiag - per-node snapshot of threads currently in
// uninterruptible disk sleep, captured as part of `mc support diag`. Helps
// engineers debugging a stuck cluster see where threads are blocked without
// requiring SSH access.
type DStateThreadsDiag struct {
	NodeCommon

	Total  int                 `json:"total"`
	Groups []DStateThreadGroup `json:"groups,omitempty"`
}
