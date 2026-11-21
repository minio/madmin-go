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
	Node     string `json:"node"`

	// NFSClients holds what became of each client the call evicted, keyed by
	// client id: each id it named, or with none named, each client the export's
	// list held on Node.
	NFSClients map[string]FilesEvictOutcome `json:"nfsClients"`
}

// MarshalJSON encodes a nil NFSClients as {}, so a reply always names its
// nfsClients object rather than null.
func (r FilesEvictResult) MarshalJSON() ([]byte, error) {
	type result FilesEvictResult
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

// EvictFilesClient evicts NFS clients of an export from the gateway on node:
// each of clientIDs, or with none given, every client the export's list holds
// on node. The gateway releases every open, lock, delegation and layout a
// client holds, on every export of that node, since a client id belongs to the
// node's gateway rather than to one export. A client is not barred: one that
// is still running connects again under a new client id, without what it held.
//
// A client that was not evicted is reported in the result's NFSClients with
// its Code; the call fails only when the node could not be asked. Evicting
// every client is sent as its own request, all=true, so a request that names
// no client is never read as one that evicts them all.
func (adm *AdminClient) EvictFilesClient(ctx context.Context, export, node string, clientIDs ...string) (FilesEvictResult, error) {
	if node == "" {
		return FilesEvictResult{}, errors.New("the node the clients are connected to is required; read it with FilesExportClients")
	}
	if len(clientIDs) > FilesMaxEvictClients {
		return FilesEvictResult{}, fmt.Errorf("%d client ids: one eviction takes at most %d", len(clientIDs), FilesMaxEvictClients)
	}
	query := url.Values{"node": []string{node}}
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
