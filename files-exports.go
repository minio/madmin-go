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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// MaxFilesExportIDsPerQuery is the number of export ids one FilesExportsQuery
// may name. The server applies the same bound and counts every id named, so
// repeating an id does not buy a longer list.
const MaxFilesExportIDsPerQuery = 512

// FilesNodeReach says what one node's AIStor Files gateway admin socket showed.
// A node running no gateway is an ordinary state, while a socket that will not
// admit the server process is a deployment fault.
type FilesNodeReach string

// The states a node-local read can end in.
const (
	// FilesNodeReachUnset is the zero value, which no entry in
	// FilesExportsQueryResponse.Results carries.
	FilesNodeReachUnset FilesNodeReach = ""

	// FilesNodeServing means the daemon answered.
	FilesNodeServing FilesNodeReach = "serving"

	// FilesNodeNoDaemon means no gateway daemon is reachable on the node: the
	// socket is absent, disabled, or nothing is listening on it.
	FilesNodeNoDaemon FilesNodeReach = "no-daemon"

	// FilesNodeRefused means a socket is present and would not admit the server
	// process. It is always a misconfiguration, never a normal state.
	FilesNodeRefused FilesNodeReach = "refused"

	// FilesNodeUnknown means no state was established: nothing was dialed, or
	// the daemon rejected the request itself and so said nothing about any
	// export.
	FilesNodeUnknown FilesNodeReach = "unknown"
)

// FilesExportStatus is one export's lease and capacity document, as the
// gateway daemon holding it reported.
type FilesExportStatus struct {
	// ExportID is the Ganesha Export_Id.
	ExportID uint64 `json:"exportId"`

	// Leasing reports whether ownership leasing is configured for this export.
	// When it is false the other lease fields are zero and only the capacity
	// fields carry meaning.
	Leasing bool `json:"leasing"`

	// Held reports whether this daemon currently holds the ownership lease.
	Held bool `json:"held"`

	// Fenced reports whether the engine has fenced itself and is refusing
	// mutations.
	Fenced bool `json:"fenced"`

	// Epoch is the ownership epoch this daemon is serving under.
	Epoch uint64 `json:"epoch"`

	// OwnerID is an opaque per-process owner identity, for diagnostics only.
	OwnerID string `json:"ownerId"`

	// LastRenew is the time of the last successful lease renewal. It is nil
	// when the lease has never been renewed. Its age measured against TTLSecs
	// says whether ownership is at risk.
	LastRenew *time.Time `json:"lastRenew,omitempty"`

	// TTLSecs is the lease TTL.
	TTLSecs uint64 `json:"ttlSecs"`

	// UsedBytes and LimitBytes are the quota usage and limit. A limit of zero
	// means unlimited.
	UsedBytes  uint64 `json:"usedBytes"`
	LimitBytes uint64 `json:"limitBytes"`

	// UsedInodes and LimitInodes are the same, for inodes.
	UsedInodes  uint64 `json:"usedInodes"`
	LimitInodes uint64 `json:"limitInodes"`

	// TS is the time at which the document was taken.
	TS time.Time `json:"ts"`
}

// FilesExportResult is one export's outcome as one node reported it. At most
// one of Status, NotHeld and Error carries it.
type FilesExportResult struct {
	// ExportID is the export the node was asked about.
	ExportID uint64 `json:"exportId"`

	// Status is the export's lease and capacity document, when the node holds
	// the export.
	Status *FilesExportStatus `json:"status,omitempty"`

	// NotHeld reports that the daemon answered and holds no such export. It is
	// an ordinary answer, not a failure.
	NotHeld bool `json:"notHeld,omitempty"`

	// Error is set when this one export could not be read for any other reason.
	// It does not invalidate the rest of the node's answer.
	Error string `json:"error,omitempty"`
}

// FilesNodeStatus is one node's whole answer.
type FilesNodeStatus struct {
	// Node is the host this answer came from, in host:port form.
	Node string `json:"node,omitempty"`

	// Reach is what the admin socket showed.
	Reach FilesNodeReach `json:"reach"`

	// Detail explains a Reach other than FilesNodeServing, or a read that
	// stopped before every export was tried.
	Detail string `json:"detail,omitempty"`

	// SocketPath is the path that was dialed, for diagnosing a path mismatch
	// between the gateway and the server.
	SocketPath string `json:"socketPath,omitempty"`

	// Truncated says the read stopped before every export named was tried, so
	// Exports is shorter than the list asked for and the missing ids are neither
	// held nor unheld. Detail says why the read stopped.
	Truncated bool `json:"truncated,omitempty"`

	// Exports carries one entry per export read, in the order asked.
	Exports []FilesExportResult `json:"exports,omitempty"`
}

// FilesUnreachableNode is a node a cluster-wide read could not ask. The read
// still succeeds and names the node here.
type FilesUnreachableNode struct {
	Node string `json:"node"`

	// Detail says why the node did not answer.
	Detail string `json:"detail,omitempty"`
}

// FilesExportsQueryResponse is the reply of FilesExportsQuery.
//
// Count is what Results holds and Total is every node the query covered. The
// endpoint does not paginate, so they differ only by the nodes that could not be
// asked.
type FilesExportsQueryResponse struct {
	// Results holds one entry per node that answered, including the node that
	// served the request.
	Results []FilesNodeStatus `json:"results"`

	// Count is the number of entries in Results.
	Count int `json:"count"`

	// Total is the number of nodes the query covered.
	Total int `json:"total"`

	// UnreachableNodes names the nodes the peer call established no state for.
	// They appear here rather than in Results because nothing is known about
	// them beyond the fact that they did not report.
	UnreachableNodes []FilesUnreachableNode `json:"unreachableNodes,omitempty"`

	// PeersNotQueried explains why no peer was asked, and is empty when they
	// were. It carries the cluster-wide condition that stopped the peer read,
	// not the state of any one node.
	PeersNotQueried string `json:"peersNotQueried,omitempty"`
}

// FilesExportsQuery returns the status of the named AIStor Files gateway
// exports on every node the cluster can reach, labeled by node.
//
// At least one export id is required, because version 1 of the gateway's
// admin-socket protocol reads one named export at a time and cannot enumerate
// what a daemon holds. Naming more than MaxFilesExportIDsPerQuery ids returns an
// error before the request is sent.
//
// A node the cluster cannot reach is reported in
// FilesExportsQueryResponse.UnreachableNodes and does not fail the call.
//
// The query reads each node's gateway directly, so it is served under
// /gateway. The /exports routes belong to the export management API, which
// reads the segment after /exports as an export name or id.
func (adm *AdminClient) FilesExportsQuery(ctx context.Context, exportIDs []uint64) (FilesExportsQueryResponse, error) {
	if len(exportIDs) == 0 {
		return FilesExportsQueryResponse{}, errors.New("at least one export id is required")
	}
	if len(exportIDs) > MaxFilesExportIDsPerQuery {
		return FilesExportsQueryResponse{}, fmt.Errorf("%d export ids were named, over the %d one request accepts",
			len(exportIDs), MaxFilesExportIDsPerQuery)
	}

	fields := make([]string, len(exportIDs))
	for idx, exportID := range exportIDs {
		fields[idx] = strconv.FormatUint(exportID, 10)
	}
	values := make(url.Values)
	values.Set("exportId", strings.Join(fields, ","))

	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{
			relPath:     filesAPIPrefix + "/gateway/exports",
			queryValues: values,
		})
	defer closeResponse(resp)
	if err != nil {
		return FilesExportsQueryResponse{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return FilesExportsQueryResponse{}, httpRespToErrorResponse(resp)
	}

	var info FilesExportsQueryResponse
	if err = json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return FilesExportsQueryResponse{}, err
	}

	return info, nil
}
