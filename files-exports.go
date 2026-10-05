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

// MaxFilesDaemonExports is the most exports one node's gateway lists in a
// FilesGatewayExports reply. A gateway serving more lists its lowest ids and
// sets FilesDaemonExports.Truncated.
const MaxFilesDaemonExports = 4096

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

	// FilesNodeUnknown means no state was established: nothing was dialed, the
	// daemon rejected the request itself and so said nothing about any export,
	// or, for FilesGatewayExports, the list could not be read.
	FilesNodeUnknown FilesNodeReach = "unknown"
)

// FilesExportStatus is one export's lease and capacity document, as the
// gateway daemon holding it reported.
type FilesExportStatus struct {
	// ExportID is the Ganesha Export_Id.
	ExportID uint64 `json:"exportID"`

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
	ExportID uint64 `json:"exportID"`

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
	// held nor unheld. Detail says why the read stopped. A FilesGatewayExports
	// reply never sets it; a cut listing sets Daemon.Truncated instead.
	Truncated bool `json:"truncated,omitempty"`

	// Exports carries one entry per export read, in the order asked.
	Exports []FilesExportResult `json:"exports,omitempty"`

	// Daemon lists the exports the node's gateway serves. It is set only in a
	// FilesGatewayExports reply, when the daemon answered.
	Daemon *FilesDaemonExports `json:"daemon,omitempty"`
}

// FilesDaemonExports is the exports one node's gateway daemon serves.
type FilesDaemonExports struct {
	// Exports is the Export_Id of every export the daemon serves, sorted. It is
	// empty when the daemon serves none.
	Exports []uint64 `json:"exports"`

	// Truncated reports that the daemon serves more than the
	// MaxFilesDaemonExports exports one reply lists. Exports then holds the
	// lowest MaxFilesDaemonExports ids, and there is no way to read the rest.
	Truncated bool `json:"truncated,omitempty"`

	// BootID changes each time the gateway daemon restarts.
	BootID string `json:"bootId"`

	// TS is the time at which the list was taken. It is nil when the daemon sent
	// no time.
	TS *time.Time `json:"ts,omitempty"`
}

// MarshalJSON encodes an empty Exports as [] rather than null, so a daemon
// serving nothing reads the same on every node.
func (d FilesDaemonExports) MarshalJSON() ([]byte, error) {
	type daemon FilesDaemonExports
	if d.Exports == nil {
		d.Exports = []uint64{}
	}
	return json.Marshal(daemon(d))
}

// FilesUnreachableNode is a node a cluster-wide read could not ask. The read
// still succeeds and names the node here.
type FilesUnreachableNode struct {
	Node string `json:"node"`

	// Detail says why the node did not answer.
	Detail string `json:"detail,omitempty"`
}

// FilesExportsQueryResponse is the reply of FilesExportsQuery and of
// FilesGatewayExports.
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

// filesGatewayExportsPath is the route of both gateway reads. They read each
// node's gateway directly, so they are served under /gateway. The /exports
// routes belong to the export management API, which reads the segment after
// /exports as an export name or id.
const filesGatewayExportsPath = filesAPIPrefix + "/gateway/exports"

// FilesExportsQuery returns the status of the named AIStor Files gateway
// exports on every node the cluster can reach, labeled by node.
//
// At least one export id is required, so a caller whose filter matched nothing
// gets an error rather than the listing FilesGatewayExports returns. Naming more
// than MaxFilesExportIDsPerQuery ids also returns an error before the request is
// sent.
//
// A node the cluster cannot reach is reported in
// FilesExportsQueryResponse.UnreachableNodes and does not fail the call.
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
	values.Set("exportID", strings.Join(fields, ","))

	var info FilesExportsQueryResponse
	if err := adm.getFilesJSON(ctx, filesGatewayExportsPath, values, &info); err != nil {
		return FilesExportsQueryResponse{}, err
	}
	return info, nil
}

// FilesGatewayExports returns what the AIStor Files gateway on every node the
// cluster can reach serves, labeled by node, in FilesNodeStatus.Daemon. A node
// whose gateway predates the listing reports FilesNodeUnknown and no Daemon.
//
// A node the cluster cannot reach is reported in
// FilesExportsQueryResponse.UnreachableNodes and does not fail the call.
//
// A server that predates the exportID parameter refuses both this call and
// FilesExportsQuery with an ErrorResponse carrying FilesErrInvalidRequest.
func (adm *AdminClient) FilesGatewayExports(ctx context.Context) (FilesExportsQueryResponse, error) {
	var info FilesExportsQueryResponse
	if err := adm.getFilesJSON(ctx, filesGatewayExportsPath, nil, &info); err != nil {
		return FilesExportsQueryResponse{}, err
	}
	return info, nil
}

// The types below belong to the AIStor Files management API, served under
// /minio/admin/files/v1 (miniohq/files docs/MANAGEMENT-API.md).

// The error codes the Files management API answers with. A caller switches on
// ErrorResponse.Code, which ToErrorResponse returns; the message is for a human
// and may change.
const (
	// FilesErrInvalidRequest is a malformed body, an unknown field, or a value
	// out of range. HTTP 400.
	FilesErrInvalidRequest = "InvalidRequest"

	// FilesErrAccessDenied means the credential lacks the action. The message
	// names it. HTTP 403.
	FilesErrAccessDenied = "AccessDenied"

	// FilesErrExportNotFound means no export has that name or id. HTTP 404.
	FilesErrExportNotFound = "ExportNotFound"

	// FilesErrExportAlreadyExists means the name, the pseudo path or a pinned
	// id is taken. HTTP 409.
	FilesErrExportAlreadyExists = "ExportAlreadyExists"

	// FilesErrExportInUse means a removal was refused: the export is serving,
	// or has children. HTTP 409.
	FilesErrExportInUse = "ExportInUse"

	// FilesErrNodeUnreachable means the owning node did not answer a write.
	// HTTP 503.
	FilesErrNodeUnreachable = "NodeUnreachable"

	// FilesErrNotImplemented means the owning node is older than the control
	// plane. HTTP 501.
	FilesErrNotImplemented = "NotImplemented"

	// FilesErrInternalError is a fault in AIStor. HTTP 500.
	FilesErrInternalError = "InternalError"
)

// FilesExportPhase is where an export's desired state, held by AIStor, meets
// what its assigned node reports. It is the status field of an export, and is
// unrelated to FilesExportStatus, the lease document a gateway reports.
type FilesExportPhase string

// The phases an export can be in.
const (
	// FilesExportServing means the assigned node is answering for the export.
	FilesExportServing FilesExportPhase = "serving"

	// FilesExportPending means AIStor holds the export and no node reports
	// holding it yet.
	FilesExportPending FilesExportPhase = "pending"

	// FilesExportMissing means the assigned node answers and does not hold the
	// export: an apply that did not take.
	FilesExportMissing FilesExportPhase = "missing"

	// FilesExportFenced means the assigned node holds the export and refuses
	// mutations.
	FilesExportFenced FilesExportPhase = "fenced"

	// FilesExportUnreachable means the assigned node did not answer: a node
	// problem, kept apart from FilesExportMissing.
	FilesExportUnreachable FilesExportPhase = "unreachable"
)

// FilesAccessType is the access a client of an export gets.
type FilesAccessType string

// The access types an export or an access rule grants.
const (
	FilesAccessRW   FilesAccessType = "rw"
	FilesAccessRO   FilesAccessType = "ro"
	FilesAccessNone FilesAccessType = "none"
)

// FilesSquash says which client users an export maps to the anonymous user. It
// applies to every client of the export.
type FilesSquash string

// The squash modes.
const (
	// FilesSquashRoot maps root.
	FilesSquashRoot FilesSquash = "root"

	// FilesSquashNone maps no one.
	FilesSquashNone FilesSquash = "none"

	// FilesSquashAll maps everyone.
	FilesSquashAll FilesSquash = "all"
)

// FilesAccessRule grants AccessType to the clients it names. Rules are
// evaluated in order and the first match wins, so the order of a rule list is
// the policy: never sort one or remove duplicates from it.
type FilesAccessRule struct {
	// Clients holds IPs, CIDRs, hostnames, wildcards such as
	// "*.corp.example.com", @netgroups, or "*".
	Clients []string `json:"clients"`

	AccessType FilesAccessType `json:"accessType"`
}

// FilesExport is one export: its configuration, with Status for where it is
// running and UsedBytes for how full it is. GetFilesExport fills every field,
// and a ListFilesExports entry may leave the configuration-only fields
// AccessType, Squash and AccessRules empty.
type FilesExport struct {
	// Name is the export's key in this API. It is never all digits.
	Name string `json:"name"`

	// ExportID is the Ganesha Export_Id AIStor allocated.
	ExportID uint64 `json:"exportID"`

	// Pseudo is the path clients mount. It cannot change.
	Pseudo string `json:"pseudo"`

	// Node is the node the export is assigned to.
	Node string `json:"node"`

	Status FilesExportPhase `json:"status"`

	// AccessType is the default access for an export with no access rules.
	AccessType FilesAccessType `json:"accessType,omitempty"`

	Squash FilesSquash `json:"squash,omitempty"`

	// QuotaBytes is the byte limit. Zero is unlimited.
	QuotaBytes uint64 `json:"quotaBytes"`

	// AccessRules are in evaluation order. An export with any rule refuses a
	// client none of them matches.
	AccessRules []FilesAccessRule `json:"accessRules,omitempty"`

	// UsedBytes is nil when the assigned node could not be reached. Status
	// says so.
	UsedBytes *uint64 `json:"usedBytes,omitempty"`
}

// MaxFilesExportsPerPage is the most exports one page of ListFilesExports or
// of the fleet FilesExportStats holds. The server also uses it when no limit is
// given, and clamps a larger one to it.
const MaxFilesExportsPerPage = 1000

// FilesListOptions narrows ListFilesExports and picks its page. An empty field
// does not narrow.
type FilesListOptions struct {
	Node   string
	Status FilesExportPhase

	// Limit is the most records the page covers. Zero or less leaves it to
	// the server, which uses MaxFilesExportsPerPage.
	Limit int

	// ContinuationToken is the previous page's NextContinuationToken, and
	// empty for the first page. It is valid only with the Node and Status it
	// was issued with.
	ContinuationToken string
}

// FilesExportList is the reply of ListFilesExports.
type FilesExportList struct {
	// Exports holds every export AIStor knows that the options admit,
	// including one on a node that did not answer.
	Exports []FilesExport `json:"exports"`

	// UnreachableNodes names the nodes that did not answer. Their exports read
	// FilesExportUnreachable and carry no usage.
	UnreachableNodes []FilesUnreachableNode `json:"unreachableNodes,omitempty"`

	// NextContinuationToken asks for the next page. It is empty on the last
	// page, and only its being empty ends the list: a page with a Status
	// filter can hold fewer exports than Limit, or none, and still carry one.
	NextContinuationToken string `json:"nextContinuationToken,omitempty"`
}

// MarshalJSON encodes a nil Exports as [], so a server that lists no export
// replies with an empty list rather than null.
func (l FilesExportList) MarshalJSON() ([]byte, error) {
	type list FilesExportList
	if l.Exports == nil {
		l.Exports = []FilesExport{}
	}
	return json.Marshal(list(l))
}

// FilesStatsOptions narrows the fleet form of FilesExportStats and picks its
// page. The per-export form takes none of them, and FilesExportStats refuses
// each there.
type FilesStatsOptions struct {
	// Node, when set, reports only the exports assigned to that node.
	Node string

	// Limit and ContinuationToken page the fleet form, as for
	// FilesListOptions.
	Limit             int
	ContinuationToken string
}

// FilesExportCapacity is one export's capacity. It carries no throughput
// counters.
type FilesExportCapacity struct {
	Name     string           `json:"name"`
	ExportID uint64           `json:"exportID"`
	Node     string           `json:"node"`
	Status   FilesExportPhase `json:"status"`

	// UsedBytes is nil when the assigned node could not be reached.
	UsedBytes *uint64 `json:"usedBytes,omitempty"`

	// LimitBytes is the limit in force. Zero is unlimited.
	LimitBytes uint64 `json:"limitBytes"`
}

// FilesStatsList is the reply of FilesExportStats.
type FilesStatsList struct {
	// Stats holds one entry per export reported.
	Stats []FilesExportCapacity `json:"stats"`

	// UnreachableNodes names the nodes that did not answer. Their exports read
	// FilesExportUnreachable and carry no usage.
	UnreachableNodes []FilesUnreachableNode `json:"unreachableNodes,omitempty"`

	// NextContinuationToken asks for the next page, and is empty on the last.
	NextContinuationToken string `json:"nextContinuationToken,omitempty"`
}

// MarshalJSON encodes a nil Stats as [], so a server that reports no export
// replies with an empty list rather than null.
func (l FilesStatsList) MarshalJSON() ([]byte, error) {
	type list FilesStatsList
	if l.Stats == nil {
		l.Stats = []FilesExportCapacity{}
	}
	return json.Marshal(list(l))
}

// filesStatsSegment is the path segment of the fleet stats endpoint. It is
// routed ahead of an export, so it is a reserved export name.
const filesStatsSegment = "stats"

// ListFilesExports returns one page of the exports AIStor holds, in export-id
// order, with the live status and usage of each. A node that did not answer is
// named in FilesExportList.UnreachableNodes and does not fail the call.
//
// To list every export, call again with FilesExportList.NextContinuationToken
// in opts.ContinuationToken until it is empty.
func (adm *AdminClient) ListFilesExports(ctx context.Context, opts FilesListOptions) (FilesExportList, error) {
	values := make(url.Values)
	if opts.Node != "" {
		values.Set("node", opts.Node)
	}
	if opts.Status != "" {
		values.Set("status", string(opts.Status))
	}
	setFilesPage(values, opts.Limit, opts.ContinuationToken)

	var list FilesExportList
	err := adm.getFilesJSON(ctx, filesAPIPrefix+"/exports", values, &list)
	return list, err
}

// GetFilesExport returns the configuration and live state of one export.
// export is a name or a numeric export id, sent as given: the client does not
// resolve it.
func (adm *AdminClient) GetFilesExport(ctx context.Context, export string) (FilesExport, error) {
	if export == "" {
		return FilesExport{}, errors.New("an export name or id is required")
	}
	segment, err := filesExportSegment(export)
	if err != nil {
		return FilesExport{}, err
	}

	var info FilesExport
	err = adm.getFilesJSON(ctx, filesAPIPrefix+"/exports/"+segment, nil, &info)
	return info, err
}

// filesExportSegment returns export escaped as one path segment. It refuses
// the names that would reach a route other than the export's: the reserved
// stats segment, and "." and "..", which url.PathEscape leaves as they are and
// a router that cleans paths resolves.
func filesExportSegment(export string) (string, error) {
	switch export {
	case filesStatsSegment, ".", "..":
		return "", fmt.Errorf("%q is reserved and names no export", export)
	}
	return url.PathEscape(export), nil
}

// FilesExportStats returns the capacity of one export, or one page of the
// capacity of every export when export is empty. export is a name or a numeric
// export id, sent as given. A node that did not answer is named in
// FilesStatsList.UnreachableNodes and does not fail the call.
//
// The fleet form pages as ListFilesExports does. opts applies only to it, so
// any of its fields is refused with an export.
func (adm *AdminClient) FilesExportStats(ctx context.Context, export string, opts FilesStatsOptions) (FilesStatsList, error) {
	relPath := filesAPIPrefix + "/exports/" + filesStatsSegment
	if export != "" {
		if opts != (FilesStatsOptions{}) {
			return FilesStatsList{}, errors.New("a node, a limit and a continuation token apply only to the stats of every export, not of one")
		}
		segment, err := filesExportSegment(export)
		if err != nil {
			return FilesStatsList{}, err
		}
		relPath = filesAPIPrefix + "/exports/" + segment + "/" + filesStatsSegment
	}
	values := make(url.Values)
	if opts.Node != "" {
		values.Set("node", opts.Node)
	}
	setFilesPage(values, opts.Limit, opts.ContinuationToken)

	var stats FilesStatsList
	err := adm.getFilesJSON(ctx, relPath, values, &stats)
	return stats, err
}

// setFilesPage adds the paging parameters of a fleet read. A limit of zero or
// less is left to the server.
func setFilesPage(values url.Values, limit int, token string) {
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	}
	if token != "" {
		values.Set("continuation-token", token)
	}
}

// getFilesJSON sends a GET to a Files management API path and decodes its JSON
// reply into v. Any status other than 200 is returned as an ErrorResponse
// carrying the server's code.
func (adm *AdminClient) getFilesJSON(ctx context.Context, relPath string, values url.Values, v any) error {
	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{
			relPath:     relPath,
			queryValues: values,
		})
	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}

	return json.NewDecoder(resp.Body).Decode(v)
}
