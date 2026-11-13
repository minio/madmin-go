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
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package madmin

import (
	"context"
	"encoding/json"
	"time"
)

// FilesCellState says whether a cell runs on the node its export's cell map
// gives it.
type FilesCellState string

const (
	// FilesCellServing means the node the cell map names runs the cell.
	FilesCellServing FilesCellState = "serving"

	// FilesCellNotServing means the node the cell map names answered and does
	// not run the cell. Detail gives the node's reason when it has one, such as
	// a take that waits for the previous owner or one that failed.
	FilesCellNotServing FilesCellState = "not-serving"

	// FilesCellUnreported means the node the cell map names could not be read,
	// so nothing is known about the cell. Detail says why.
	FilesCellUnreported FilesCellState = "unreported"
)

// FilesCellHealth is one cell's health as the gateway that runs it reported.
type FilesCellHealth struct {
	// TookOver reports that the cell's lease was still in place when the
	// gateway took the cell, so its last owner did not give it up. A lost node
	// leaves one, and so does the same node's crashed run. A planned move does
	// not.
	TookOver bool `json:"tookOver"`

	// ServingSince is when the gateway started to serve the cell.
	ServingSince *time.Time `json:"servingSince,omitempty"`

	// Leasing, Held and Epoch are the cell's lease, as in FilesExportStatus.
	// Fenced is whether the cell answers no call, as when its gateway lost the
	// lease, handed the cell off, or has gone a lease TTL without renewing it.
	Leasing bool   `json:"leasing"`
	Held    bool   `json:"held"`
	Fenced  bool   `json:"fenced"`
	Epoch   uint64 `json:"epoch"`

	// LastRenew is the time of the lease's last renewal. It is nil when the
	// lease was never renewed, and when Leasing is false.
	LastRenew *time.Time `json:"lastRenew,omitempty"`

	// Paging reports that the cell still reads in the snapshot image it was
	// taken over from. It serves meanwhile.
	Paging bool `json:"paging"`

	// UsedBytes and UsedInodes are what the cell holds.
	UsedBytes  uint64 `json:"usedBytes"`
	UsedInodes uint64 `json:"usedInodes"`

	// Shipping reports that the cell ships its op-log to the tier.
	Shipping bool `json:"shipping"`

	// UnshippedSince is nil when Shipping is false, and when a ship pass since
	// the gateway opened the cell has put every change on the tier. Otherwise
	// it is no later than the oldest change not on the tier yet. The gateway
	// cannot date a change made before the open. When the op-log holds changes
	// at the open, UnshippedSince is the time of the open until such a pass,
	// even when those changes are already on the tier. Its age bounds what a
	// takeover of the cell would lose of changes made since the open.
	UnshippedSince *time.Time `json:"unshippedSince,omitempty"`

	// ShipFailed reports that the last ship to the tier failed.
	ShipFailed bool `json:"shipFailed"`

	// DRDegraded reports that the tier's copy of the op-log has a gap. It
	// clears once a snapshot covers the gap and shipping has caught up.
	DRDegraded bool `json:"drDegraded"`

	// LockGrace reports that the cell takes only lock reclaims, as it does for
	// a while after it starts or is taken over.
	LockGrace bool `json:"lockGrace"`

	// SalvageWaits is how many takeovers the cell still waits on, because the
	// old owner may return with changes it never shipped.
	SalvageWaits uint32 `json:"salvageWaits"`

	// SalvageSince is when the oldest takeover the cell waits on happened. It
	// is nil when the cell waits on none.
	SalvageSince *time.Time `json:"salvageSince,omitempty"`

	// TS is when the gateway on Node began its report. Every cell that node
	// runs carries the same TS.
	TS time.Time `json:"ts"`
}

// FilesCell is one cell of an export: where the cell map puts it and, when
// that node runs it, its health.
type FilesCell struct {
	Cell int `json:"cell"`

	// Node is the node the cell map gives the cell.
	Node string `json:"node"`

	// Home is where the cell runs when every node is up. A cell moved off a
	// lost node goes back home once that node has stayed up.
	Home string `json:"home"`

	// AtHome reports that Node is Home.
	AtHome bool `json:"atHome"`

	State FilesCellState `json:"state"`

	// Detail explains a State other than FilesCellServing.
	Detail string `json:"detail,omitempty"`

	// Health is the cell's health. It is set only when State is
	// FilesCellServing.
	Health *FilesCellHealth `json:"health,omitempty"`

	// AlsoOn names other nodes whose gateway also reports the cell, such as
	// an old owner that has not given it up yet.
	AlsoOn []string `json:"alsoOn,omitempty"`
}

// FilesCellsResponse is the reply of FilesCells.
type FilesCellsResponse struct {
	ExportID uint64 `json:"exportID"`

	// Generation is the cell map's generation. It rises with every change to
	// the map.
	Generation uint64 `json:"generation"`

	// Cells holds one entry per cell of the export, in cell order.
	Cells []FilesCell `json:"cells"`

	// UnreachableNodes names the nodes that could not be read. The cells the
	// map gives them read FilesCellUnreported.
	UnreachableNodes []FilesUnreachableNode `json:"unreachableNodes,omitempty"`
}

// MarshalJSON encodes a nil Cells as [], so an export with no cell map
// replies with an empty list rather than null.
func (r FilesCellsResponse) MarshalJSON() ([]byte, error) {
	type response FilesCellsResponse
	if r.Cells == nil {
		r.Cells = []FilesCell{}
	}
	return json.Marshal(response(r))
}

// FilesCells returns where each cell of an export runs and how healthy it is,
// as the gateway on each node reports it. export is a name or a numeric export
// id, sent as given. A node that cannot be read is named in
// FilesCellsResponse.UnreachableNodes and does not fail the call.
func (adm *AdminClient) FilesCells(ctx context.Context, export string) (FilesCellsResponse, error) {
	relPath, err := filesExportPath(export, "/cells")
	if err != nil {
		return FilesCellsResponse{}, err
	}

	var resp FilesCellsResponse
	err = adm.filesJSON(ctx, requestData{relPath: relPath}, &resp)
	return resp, err
}
