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
	"fmt"
	"net/http"
	"net/url"
)

// FilesNFSClient is one NFSv4 client of an export, as the gateway on its node
// reports it. FilesClientsResponse keys it by node and then by client id.
type FilesNFSClient struct {
	// Address is the address the client connected from. It is empty when the
	// gateway could not tell.
	Address string `json:"address"`

	// Owner is the owner string the client identified itself with, which for a
	// Linux client names its host. Any byte that is not printable ASCII reads
	// as ?.
	Owner string `json:"owner"`

	// MinorVersion is the NFSv4 minor version the client speaks.
	MinorVersion uint32 `json:"minorVersion"`

	// LeaseAgeSecs is the seconds since the client last renewed its lease. It
	// is 0 while a request of the client's runs.
	LeaseAgeSecs uint64 `json:"leaseAgeSecs"`

	// Opens is how many files the client holds open, on any export of its node.
	Opens uint32 `json:"opens"`

	// HoldsState reports that the client holds opens, locks, delegations or
	// layouts, on any export of its node.
	HoldsState bool `json:"holdsState"`

	// OnExport reports that the client holds state on this export. A client
	// that holds no state anywhere is listed with OnExport false: the gateway
	// cannot tell which export an idle client has mounted.
	OnExport bool `json:"onExport"`

	// Waiting is how many of the client's lock requests are parked on its
	// node, waiting for a range another owner holds: a client stuck behind a
	// lock.
	Waiting uint32 `json:"waiting,omitempty"`

	// LeaseLapsed reports that the client has not renewed its lease within the
	// lease time: it stopped talking, and its gateway expires it shortly.
	LeaseLapsed bool `json:"leaseLapsed,omitempty"`

	// CallbackDown reports that the gateway cannot call the client back, so a
	// delegation or layout it holds cannot be recalled.
	CallbackDown bool `json:"callbackDown,omitempty"`

	// RetriesPerMin is how many requests the client sent again in the last
	// minute, as NFSv4.1 session replays: a high count is a retry storm.
	RetriesPerMin uint32 `json:"retriesPerMin,omitempty"`
}

// FilesRetryStormPerMin is the retries a minute from which a client is
// reported in a retry storm.
const FilesRetryStormPerMin = 30

// NeedsAttention reports whether anything about the client may need an
// operator: it waits for a lock, its lease lapsed, its callbacks are down, or
// it is in a retry storm. None of it alone is a reason to evict it.
func (c FilesNFSClient) NeedsAttention() bool {
	return c.Waiting > 0 || c.LeaseLapsed || c.CallbackDown || c.RetriesPerMin >= FilesRetryStormPerMin
}

// FilesNodeNFSClients is what one node's gateway reported for an export.
type FilesNodeNFSClients struct {
	// NFSClients holds the NFSv4 clients the node serves, keyed by client id:
	// 16 lowercase hex digits, which EvictFilesClient takes together with the
	// node. A client id belongs to the node's gateway, and a client that
	// mounts again after an eviction or a gateway restart gets a new one.
	NFSClients map[string]FilesNFSClient `json:"nfsClients,omitempty"`

	// Truncated reports that the node's gateway serves more NFS clients than
	// fit in one reply, so NFSClients holds only the ones it sent. There is no
	// way to read the rest.
	Truncated bool `json:"truncated,omitempty"`

	// Unreachable is why the node could not be read. NFSClients is empty then.
	Unreachable string `json:"unreachable,omitempty"`
}

// FilesClientsResponse is the reply of FilesExportClients.
type FilesClientsResponse struct {
	ExportID uint64 `json:"exportID"`

	// Nodes holds each node's report, keyed by node. Every node that serves
	// the export is present, with no NFS clients, with some, or unreachable.
	Nodes map[string]FilesNodeNFSClients `json:"nodes"`
}

// MarshalJSON encodes a nil Nodes as {}, so a reply always names its nodes
// object rather than null.
func (r FilesClientsResponse) MarshalJSON() ([]byte, error) {
	type response FilesClientsResponse
	if r.Nodes == nil {
		r.Nodes = map[string]FilesNodeNFSClients{}
	}
	return json.Marshal(response(r))
}

// FilesExportClients returns the NFS clients of an export, as the gateway on
// each node reports them. export is a name or a numeric export id, sent as
// given. A client is listed when it holds state on the export, and when it
// holds no state anywhere, since an idle client cannot be placed on one export.
// A node that cannot be read is reported Unreachable in Nodes and does not
// fail the call.
func (adm *AdminClient) FilesExportClients(ctx context.Context, export string) (FilesClientsResponse, error) {
	relPath, err := filesExportPath(export, "/clients")
	if err != nil {
		return FilesClientsResponse{}, err
	}

	var resp FilesClientsResponse
	err = adm.filesJSON(ctx, requestData{relPath: relPath}, &resp)
	return resp, err
}

// FilesMaxEvictClients is the most client ids one EvictFilesClient call takes.
const FilesMaxEvictClients = 1000

// FilesEvictResult is the reply of EvictFilesClient.
type FilesEvictResult struct {
	ExportID uint64 `json:"exportID"`

	// Nodes holds what each node did, keyed by node: the node the call named,
	// or with none named, each node a named client was found on, or with every
	// client asked for, each node that had any or could not evict them. A
	// client id belongs to one node's gateway, and two nodes can hand out the
	// same id, so outcomes are kept per node.
	Nodes map[string]FilesNodeEvictResult `json:"nodes"`

	// Unplaced holds the named clients the call did not evict because it could
	// not tell which node holds them, keyed by client id: no node lists it
	// (FilesErrClientNotFound, or FilesErrNodeUnreachable while a node could
	// not be read), or several do (FilesErrClientAmbiguous: name the node).
	// Only a call that names no node has any.
	Unplaced map[string]FilesEvictOutcome `json:"unplaced,omitempty"`
}

// MarshalJSON encodes a nil Nodes as {}, so a reply always names its nodes
// object rather than null.
func (r FilesEvictResult) MarshalJSON() ([]byte, error) {
	type result FilesEvictResult
	if r.Nodes == nil {
		r.Nodes = map[string]FilesNodeEvictResult{}
	}
	return json.Marshal(result(r))
}

// FilesNodeEvictResult is what one node did with an eviction.
type FilesNodeEvictResult struct {
	// NFSClients holds what became of each client, keyed by client id.
	NFSClients map[string]FilesEvictOutcome `json:"nfsClients"`

	// Code is why the node evicted none of them, such as
	// FilesErrNodeUnreachable or FilesErrNotImplemented, with Detail. It is
	// only set when the call named no node: a call that names one fails
	// instead.
	Code   string `json:"code,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// MarshalJSON encodes a nil NFSClients as {}.
func (r FilesNodeEvictResult) MarshalJSON() ([]byte, error) {
	type result FilesNodeEvictResult
	if r.NFSClients == nil {
		r.NFSClients = map[string]FilesEvictOutcome{}
	}
	return json.Marshal(result(r))
}

// FilesEvictOutcome is what became of one client of an eviction.
type FilesEvictOutcome struct {
	// Evicting reports that the gateway started the eviction. It finishes
	// shortly after the reply: read FilesExportClients until the client id is
	// gone to see it done. The id is gone only from a complete report of the
	// node: while that node is Unreachable or Truncated, its absence says
	// nothing, so read again.
	Evicting bool `json:"evicting,omitempty"`

	// Code is why the eviction did not start: FilesErrClientNotFound when the
	// gateway holds no client with that id, which is also the answer to a
	// repeat of an eviction that already ran, and FilesErrNodeBusy when the
	// gateway stayed at as many running evictions as it allows for as long as
	// the call waited, so send that id again.
	Code string `json:"code,omitempty"`

	// Detail is the reason behind Code.
	Detail string `json:"detail,omitempty"`
}

// ValidFilesClientID reports whether id is spelled as a client id: 16
// lowercase hex digits.
func ValidFilesClientID(id string) bool {
	if len(id) != 16 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// EvictFilesClient evicts NFS clients of an export: each of clientIDs, or
// with none given, every client the export's list holds. With node set, it
// evicts them from that node's gateway only. With node empty, the server finds
// the node of each named client in the export's list, and asks for a node only
// when several list the same id; and evicting every client with node empty
// evicts every client of the export on every node.
//
// A gateway releases every open, lock, delegation and layout a client holds,
// on every export of its node, since a client id belongs to the node's gateway
// rather than to one export. A client is not barred: one that is still running
// connects again under a new client id, without what it held.
//
// A client that was not evicted is reported with its Code. With node set, the
// call fails when that node could not be asked; with node empty, such a node
// is reported in its Nodes entry instead. Evicting every client is sent as its
// own request, all=true, so a request that names no client is never read as
// one that evicts them all.
func (adm *AdminClient) EvictFilesClient(ctx context.Context, export, node string, clientIDs ...string) (FilesEvictResult, error) {
	if len(clientIDs) > FilesMaxEvictClients {
		return FilesEvictResult{}, fmt.Errorf("%d client ids: one eviction takes at most %d", len(clientIDs), FilesMaxEvictClients)
	}
	query := url.Values{}
	if node != "" {
		query.Set("node", node)
	}
	for _, id := range clientIDs {
		if !ValidFilesClientID(id) {
			return FilesEvictResult{}, fmt.Errorf("invalid client id %q: want 16 lowercase hex digits, as FilesExportClients lists it", id)
		}
		query.Add("client", id)
	}
	if len(clientIDs) == 0 {
		query.Set("all", "true")
	}
	relPath, err := filesExportPath(export, "/clients")
	if err != nil {
		return FilesEvictResult{}, err
	}

	var result FilesEvictResult
	err = adm.filesWrite(ctx, filesWriteRequest{
		method:  http.MethodDelete,
		reqData: requestData{relPath: relPath, queryValues: query},
		out:     &result,
	})
	return result, err
}
