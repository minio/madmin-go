//
// Copyright (c) 2015-2025 MinIO, Inc.
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
	"context"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/tinylib/msgp/msgp"
)

//go:generate msgp -d clearomitted -d "timezone utc" -file $GOFILE

// PaginatedPoolsResponse represents a paginated response for pools
type PaginatedPoolsResponse struct {
	Results      []PoolResource `json:"results" msg:"r,omitempty"`
	Count        int            `json:"count" msg:"c"`
	Total        int            `json:"total" msg:"t"`
	Offset       int            `json:"offset" msg:"o"`
	Sort         string         `json:"sort" msg:"s"`
	SortReversed bool           `json:"sortReversed" msg:"sr"`
}

// PaginatedNodesResponse represents a paginated response for nodes
type PaginatedNodesResponse struct {
	Results        []NodeResource           `json:"results" msg:"r,omitempty"`
	ResultsSummary NodesQueryResultsSummary `json:"resultsSummary" msg:"rs"`
	Count          int                      `json:"count" msg:"c"`
	Total          int                      `json:"total" msg:"t"`
	Offset         int                      `json:"offset" msg:"o"`
	Sort           string                   `json:"sort" msg:"s"`
	SortReversed   bool                     `json:"sortReversed" msg:"sr"`

	// Aggregated are the metrics aggregated for all filtered nodes,
	// not just the results.
	Aggregated *Metrics `json:"aggregated,omitempty" msg:"m,omitempty"`
}

// NodesQueryResultsSummary contains summary statistics for all nodes in the cluster
type NodesQueryResultsSummary struct {
	Offline      int `json:"offline" msg:"off"`
	Initializing int `json:"initializing" msg:"ini"`
	Online       int `json:"online" msg:"on"`
	Restarting   int `json:"restarting" msg:"rs"`
	Unknown      int `json:"unknown" msg:"un"`
}

// PaginatedDrivesResponse represents a paginated response for drives
type PaginatedDrivesResponse struct {
	Results      []DriveResource `json:"results" msg:"r,omitempty"`
	Count        int             `json:"count" msg:"c"`
	Total        int             `json:"total" msg:"t"`
	Offset       int             `json:"offset" msg:"o"`
	Sort         string          `json:"sort" msg:"s"`
	SortReversed bool            `json:"sortReversed" msg:"sr"`

	// Aggregated are the metrics aggregated for all filtered drives,
	// not just the results.
	// Always returned, though day metrics are only available if the option is set.
	Aggregated DiskMetric `json:"aggregated" msg:"m"`
}

// DrivesQuerySummary contains a summary for all drives, ignoring pagination and query limits.
type DrivesQuerySummary struct {
	StateOk          int `json:"stateOk" msg:"s"`
	StateOffline     int `json:"stateOffline" msg:"so"`
	StateCorrupt     int `json:"stateCorrupt" msg:"sc"`
	StateMissing     int `json:"stateMissing" msg:"sm"`
	StatePermission  int `json:"statePermission" msg:"sp"`
	StateFaulty      int `json:"stateFaulty" msg:"sf"`
	StateRootMount   int `json:"stateRootMount" msg:"srm"`
	StateUnknown     int `json:"stateUnknown" msg:"sun"`
	StateUnformatted int `json:"stateUnformatted" msg:"suf"`
}

// ErasureSetsQueryResultsSummary contains summary statistics for all erasure sets in the cluster
type ErasureSetsQueryResultsSummary struct {
	Ok       int `json:"ok" msg:"ok"`
	Warning  int `json:"warning" msg:"w"`
	Critical int `json:"critical" msg:"cr"`
	Unusable int `json:"unusable" msg:"un"`
}

// PaginatedErasureSetsResponse represents a paginated response for erasure sets
type PaginatedErasureSetsResponse struct {
	Results        []ErasureSetResource           `json:"results" msg:"r,omitempty"`
	ResultsSummary ErasureSetsQueryResultsSummary `json:"resultsSummary" msg:"rs"`
	Count          int                            `json:"count" msg:"c"`
	Total          int                            `json:"total" msg:"t"`
	Offset         int                            `json:"offset" msg:"o"`
	Sort           string                         `json:"sort" msg:"s"`
	SortReversed   bool                           `json:"sortReversed" msg:"sr"`
}

// PoolLayout contains layout information for a storage pool including server and drive counts
type PoolLayout struct {
	Servers       int `json:"servers" msg:"s"`
	Drives        int `json:"drives" msg:"d"`
	DrivesOffline int `json:"drivesOffline" msg:"do"`
	DrivesHealing int `json:"drivesHealing" msg:"dh"`
}

// ClusterResource represents comprehensive high-level information about the MinIO cluster
type ClusterResource struct {
	Mode              string       `json:"mode" msg:"m"`
	Domains           []string     `json:"domains,omitempty" msg:"d,omitempty"`
	Region            string       `json:"region,omitempty" msg:"r,omitempty"`
	DeploymentID      string       `json:"deploymentId" msg:"di"`
	PoolCount         int          `json:"poolCount" msg:"pc"`
	PoolsLayout       []PoolLayout `json:"poolsLayout,omitempty" msg:"pl,omitempty"`
	NodeCount         int          `json:"nodeCount" msg:"nc"`
	DriveCount        int          `json:"driveCount" msg:"dc"`
	SetCount          int          `json:"setCount" msg:"sc"`
	BucketCount       int          `json:"bucketCount" msg:"bc"`
	ObjectCount       int          `json:"objectCount" msg:"oc"`
	VersionCount      int          `json:"versionCount" msg:"vc"`
	DeleteMarkerCount int          `json:"deleteMarkerCount" msg:"dmc"`
	TotalSize         uint64       `json:"totalSize" msg:"ts"`
	OnlineDrives      int          `json:"onlineDrives" msg:"od"`
	OfflineDrives     int          `json:"offlineDrives" msg:"fd"`
	RawTotalBytes     uint64       `json:"rawTotalBytes" msg:"rtb"`
	RawFreeBytes      uint64       `json:"rawFreeBytes" msg:"rfb"`
	UsableTotalBytes  uint64       `json:"usableTotalBytes" msg:"utb"`
	UsableFreeBytes   uint64       `json:"UsableFreeBytes" msg:"ufb"`
	// Metrics contains the metrics aggregated for cluster if requested.
	Metrics *Metrics `json:"metrics,omitempty" msg:"met,omitempty"`
}

// ServicesResourceInfo holds information about external services and integrations connected to the cluster
type ServicesResourceInfo struct {
	LDAP          LDAP                          `json:"ldap" msg:"l"`
	Logger        []Logger                      `json:"logger,omitempty" msg:"lg,omitempty"`
	Audit         []Audit                       `json:"audit,omitempty" msg:"a,omitempty"`
	Notifications []map[string][]TargetIDStatus `json:"notifications,omitempty" msg:"n,omitempty"`
	KMSStatus     []KMS                         `json:"kms,omitempty" msg:"k,omitempty"`
}

// PoolResource represents detailed information about a storage pool including capacity, usage, and drive statistics
type PoolResource struct {
	PoolIndex          int      `json:"poolindex" msg:"i"`
	StripeSize         int      `json:"stripeSize" msg:"ss"`
	TotalSets          int      `json:"totalSets" msg:"ts"`
	Parity             int      `json:"parity" msg:"p"`
	WriteQuorum        int      `json:"writeQuorum" msg:"wq"`
	ReadQuorum         int      `json:"readQuorum" msg:"rq"`
	Nodes              []string `json:"nodes,omitempty" msg:"n,omitempty"`
	DriveCount         int      `json:"driveCount" msg:"dc"`
	DrivesOnline       int      `json:"drivesOnline" msg:"do"`
	DrivesOffline      int      `json:"drivesOffline" msg:"doff"`
	DrivesHealing      int      `json:"drivesHealing" msg:"dh"`
	NodeCount          int      `json:"nodeCount" msg:"nc"`
	RawUsage           uint64   `json:"rawUsage" msg:"ru"`
	RawCapacity        uint64   `json:"rawCapacity" msg:"rc"`
	Usage              uint64   `json:"usage" msg:"u"`
	ObjectsCount       uint64   `json:"objectsCount" msg:"oc"`
	VersionsCount      uint64   `json:"versionsCount" msg:"vc"`
	DeleteMarkersCount uint64   `json:"deleteMarkersCount" msg:"dmc"`
}

// DriveCounts contains counts of drives categorized by their operational state
type DriveCounts struct {
	Ok          int `json:"ok" msg:"ok"`
	Offline     int `json:"offline" msg:"of"`
	Corrupt     int `json:"corrupt" msg:"cr"`
	Missing     int `json:"missing" msg:"mi"`
	Permission  int `json:"permission" msg:"pe"`
	Faulty      int `json:"faulty" msg:"fa"`
	RootMount   int `json:"rootMount" msg:"ro"`
	Unknown     int `json:"unknown" msg:"un"`
	Unformatted int `json:"unformatted" msg:"uf"`
}

// NodeResource represents detailed information about a MinIO server node including version, state, and drive counts
type NodeResource struct {
	Host        string        `json:"host" msg:"h"`
	Version     string        `json:"version" msg:"v"`
	CommitID    string        `json:"commitId" msg:"c"`
	Uptime      int64         `json:"uptime" msg:"u"`
	State       string        `json:"state" msg:"s"`
	TotalDrives int           `json:"totalDrives" msg:"td"`
	DriveCounts DriveCounts   `json:"driveCounts" msg:"dc"`
	PoolIndex   int           `json:"poolIndex" msg:"pi"`
	PoolIndexes []int         `json:"poolIndexes,omitempty" msg:"pis,omitempty"`
	HostInfo    *HostInfoStat `json:"hostInfo,omitempty" msg:"hi,omitempty"`

	// Metrics contains the metrics aggregated for node if requested.
	Metrics *Metrics `json:"metrics,omitempty" msg:"m,omitempty"`
}

// DriveResource represents detailed information about a storage drive including capacity, usage, and metrics
type DriveResource struct {
	ID             string      `json:"id" msg:"i"`
	DriveIndex     int         `json:"idx" msg:"idx"`
	ServerIndex    int         `json:"serverIndex" msg:"sidx"`
	Path           string      `json:"path" msg:"p"`
	NodeID         string      `json:"nodeId" msg:"ni"`
	PoolIndex      int         `json:"poolIndex" msg:"pi"`
	SetIndex       int         `json:"setIndex" msg:"si"`
	State          string      `json:"state" msg:"s"`
	Healing        bool        `json:"healing" msg:"h"`
	Size           uint64      `json:"size" msg:"sz"`
	Used           uint64      `json:"used" msg:"u"`
	PercentageUsed float64     `json:"percentageUsed" msg:"pu"`
	Available      uint64      `json:"available" msg:"a"`
	InodesFree     uint64      `json:"inodesFree" msg:"if"`
	InodesUsed     uint64      `json:"inodesUsed" msg:"iu"`
	UUID           string      `json:"uuid" msg:"uid"`
	Metrics        *DiskMetric `json:"metrics,omitempty" msg:"m,omitempty"`
}

// ErasureSetResource represents detailed information about an erasure coding set including drive counts and capacity
type ErasureSetResource struct {
	PoolIndex          int                 `json:"poolIndex" msg:"pi"`
	SetIndex           int                 `json:"setIndex" msg:"si"`
	DriveCount         int                 `json:"driveCount" msg:"dc"`
	Nodes              []string            `json:"nodes,omitempty" msg:"n,omitempty"`
	OfflineNodes       []string            `json:"offlineNodes,omitempty" msg:"on,omitempty"`
	RawUsage           uint64              `json:"rawUsage" msg:"ru"`
	RawCapacity        uint64              `json:"rawCapacity" msg:"rc"`
	Usage              uint64              `json:"usage" msg:"u"`
	ObjectsCount       uint64              `json:"objectsCount" msg:"oc"`
	VersionsCount      uint64              `json:"versionsCount" msg:"vc"`
	DeleteMarkersCount uint64              `json:"deleteMarkersCount" msg:"dmc"`
	State              string              `json:"state" msg:"st"`
	Drives             []Disk              `json:"drives,omitempty" msg:"d,omitempty"`
	DriveStates        DriveResourceStates `json:"driveStates" msg:"ds"`

	// Deprecated (to be removed in future releases)
	OfflineDrives int `json:"offlineDrives" msg:"od"`
	OnlineDrives  int `json:"onlineDrives" msg:"ond"`
	HealDisks     int `json:"healDisks" msg:"hd"`
}

// DriveResourceStates contains the possible states for erasure set drives
type DriveResourceStates struct {
	Ok          int `json:"ok" msg:"ok"`
	Offline     int `json:"offline" msg:"off"`
	Corrupt     int `json:"corrupt" msg:"cor"`
	Missing     int `json:"missing" msg:"mis"`
	Permission  int `json:"permission" msg:"per"`
	Faulty      int `json:"faulty" msg:"fau"`
	RootMount   int `json:"rootMount" msg:"rm"`
	Unknown     int `json:"unknown" msg:"unk"`
	Unformatted int `json:"unformatted" msg:"unf"`
	Healing     int `json:"healing" msg:"hl"`
}

// ClusterSummaryUsage contains storage usage statistics for the cluster
type ClusterSummaryUsage struct {
	RawCapacity  int64 `json:"rawCapacity" msg:"rc"`
	RawAvailable int64 `json:"rawAvailable" msg:"ra"`
	RawUsage     int64 `json:"rawUsage" msg:"ru"`
	Capacity     int64 `json:"capacity" msg:"c"`
	Available    int64 `json:"available" msg:"a"`
	Usage        int64 `json:"usage" msg:"u"`
}

// PoolsSummaryUsage contains storage usage statistics for the pools
type PoolsSummaryUsage struct {
	RawCapacity  int64   `json:"rawCapacity" msg:"rc"`
	RawAvailable int64   `json:"rawAvailable" msg:"ra"`
	RawUsage     int64   `json:"rawUsage" msg:"ru"`
	Capacity     int64   `json:"capacity" msg:"c"`
	Available    int64   `json:"available" msg:"a"`
	Usage        int64   `json:"usage" msg:"u"`
	Efficiency   float64 `json:"efficiency" msg:"e"`
}

// ClusterSummaryCount contains resource counts with status breakdown
type ClusterSummaryCount struct {
	Total   int `json:"total" msg:"t"`
	Online  int `json:"online" msg:"on"`
	Offline int `json:"offline" msg:"off"`
	Healing int `json:"healing" msg:"hl"`
}

// ServersSummaryCount contains resource counts with status breakdown
type ServersSummaryCount struct {
	Total   int `json:"total" msg:"t"`
	Online  int `json:"online" msg:"on"`
	Offline int `json:"offline" msg:"off"`
	Healing int `json:"healing" msg:"hl"`
}

// DrivesSummaryCount contains drive counts with status breakdown
type DrivesSummaryCount struct {
	Total   int `json:"total" msg:"t"`
	Online  int `json:"online" msg:"on"`
	Offline int `json:"offline" msg:"off"`
	Healing int `json:"healing" msg:"hl"`
}

// PoolDetails contains detailed configuration and statistics for a storage pool
type PoolDetails struct {
	TotalServers       int `json:"totalServers" msg:"ts"`
	TotalObjects       int `json:"totalObjects" msg:"to"`
	TotalDeleteMarkers int `json:"totalDeleteMarkers" msg:"tdm"`
	TotalVersions      int `json:"totalVersions" msg:"tv"`
	ErasureSets        int `json:"erasureSets" msg:"es"`
	DrivesPerSet       int `json:"drivesPerSet" msg:"dps"`
	Parity             int `json:"parity" msg:"p"`
}

// PoolSummary contains summary information for a storage pool including usage and drive statistics
type PoolSummary struct {
	Index   int                `json:"index" msg:"idx"`
	Usage   PoolsSummaryUsage  `json:"usage" msg:"us"`
	Drives  DrivesSummaryCount `json:"drives" msg:"drv"`
	Details PoolDetails        `json:"details" msg:"dtls"`
}

// ClusterSummaryResponse contains a comprehensive summary of cluster resources and statistics
type ClusterSummaryResponse struct {
	Encryption   bool                `json:"encryption" msg:"enc"`
	Version      string              `json:"version" msg:"ver"`
	DeploymentID string              `json:"deploymentID" msg:"did"`
	Region       string              `json:"region" msg:"reg"`
	Domains      []string            `json:"domains" msg:"dom"`
	Mode         string              `json:"mode" msg:"mod"`
	Usage        ClusterSummaryUsage `json:"usage" msg:"us"`
	Servers      ServersSummaryCount `json:"servers" msg:"srv"`
	Drives       DrivesSummaryCount  `json:"drives" msg:"drv"`
	Pools        []PoolSummary       `json:"pools" msg:"pls"`
}

// ClusterSummaryResourceOpts ask for additional data from the server
// this is not used at the moment, kept here for future
// extensibility.
//
//msgp:ignore ClusterSummaryResourceOpts
type ClusterSummaryResourceOpts struct{}

func (adm *AdminClient) ClusterSummaryQuery(ctx context.Context, _ ClusterSummaryResourceOpts) (ClusterSummaryResponse, error) {
	values := make(url.Values)
	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{
			relPath:     adminAPIPrefix + "/cluster-summary",
			queryValues: values,
		})
	defer closeResponse(resp)
	if err != nil {
		return ClusterSummaryResponse{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return ClusterSummaryResponse{}, httpRespToErrorResponse(resp)
	}

	var info ClusterSummaryResponse
	if err = info.DecodeMsg(msgp.NewReader(resp.Body)); err != nil {
		return ClusterSummaryResponse{}, err
	}

	return info, nil
}

// ClusterResourceOpts ask for additional data from the server
// this is not used at the moment, kept here for future
// extensibility.
//
//msgp:ignore ClusterResourceOpts
type ClusterResourceOpts struct {
	// Metrics will include per-node metrics in the response if set
	Metrics OptionalMetrics
}

// ClusterQuery - Get high-level information about the cluster
func (adm *AdminClient) ClusterQuery(ctx context.Context, options ClusterResourceOpts) (ClusterResource, error) {
	values := make(url.Values)
	options.Metrics.apply(values)
	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{
			relPath:     adminAPIPrefix + "/query/cluster",
			queryValues: values,
		})
	defer closeResponse(resp)
	if err != nil {
		return ClusterResource{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return ClusterResource{}, httpRespToErrorResponse(resp)
	}

	var info ClusterResource
	if err = info.DecodeMsg(msgp.NewReader(resp.Body)); err != nil {
		return ClusterResource{}, err
	}

	return info, nil
}

// ServicesResourceOpts ask for additional data from the server
// this is not used at the moment, kept here for future
// extensibility.
//
//msgp:ignore ServicesResourceOpts
type ServicesResourceOpts struct{}

// ServicesQuery - Get information about services connected to the cluster
func (adm *AdminClient) ServicesQuery(ctx context.Context, _ ServicesResourceOpts) (ServicesResourceInfo, error) {
	values := make(url.Values)
	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{
			relPath:     adminAPIPrefix + "/query/services",
			queryValues: values,
		})
	defer closeResponse(resp)
	if err != nil {
		return ServicesResourceInfo{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return ServicesResourceInfo{}, httpRespToErrorResponse(resp)
	}

	var info ServicesResourceInfo
	if err = info.DecodeMsg(msgp.NewReader(resp.Body)); err != nil {
		return ServicesResourceInfo{}, err
	}

	return info, nil
}

// PoolsResourceOpts contains the available options for the PoolsQuery API
//
//msgp:ignore PoolsResourceOpts
type PoolsResourceOpts struct {
	// Limit defaults to 100 if set to 0.
	// A limit of -1 will return all results.
	Limit  int
	Offset int
	Filter string
	// Sort fields contained in PoolResource.
	//
	// Example: PoolsResourceOpts.Sort = "PoolIndex"
	// Assuming the value of PoolIndex is of a supported value type.
	//
	// Supported Values Types: int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, string
	Sort string
	// SortReversed will only take effect if Sort is defined
	SortReversed bool
}

func (adm *AdminClient) PoolsQuery(ctx context.Context, options *PoolsResourceOpts) (*PaginatedPoolsResponse, error) {
	values := make(url.Values)

	if options != nil {
		values.Set("limit", strconv.Itoa(options.Limit))

		if options.Offset > 0 {
			values.Set("offset", strconv.Itoa(options.Offset))
		}
		if options.Filter != "" {
			values.Set("filter", options.Filter)
		}
		if options.Sort != "" {
			values.Set("sort", options.Sort)
		}
		if options.SortReversed {
			values.Set("sortReversed", "true")
		} else {
			values.Set("sortReversed", "false")
		}
	}

	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{
			relPath:     adminAPIPrefix + "/query/pools",
			queryValues: values,
		})
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	// Parse the paginated response using msgp
	var poolsResp PaginatedPoolsResponse
	if err = poolsResp.DecodeMsg(msgp.NewReader(resp.Body)); err != nil {
		return nil, err
	}

	return &poolsResp, nil
}

// NodesResourceOpts contains the available options for the NodesQuery API
//
//msgp:ignore NodesResourceOpts
type NodesResourceOpts struct {
	// Limit defaults to 100 if set to 0.
	// A limit of -1 will return all results.
	Limit  int
	Offset int
	Filter string
	// Sort fields contained in NodeResource.
	//
	// Example: NodesResourceOpts.Sort = "PoolIndex"
	// Assuming the value of PoolIndex is of a supported value type.
	//
	// Supported Values Types: int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, string
	Sort string
	// SortReversed will only take effect if Sort is defined
	SortReversed bool

	// Metrics will include per-node metrics in the response if set
	Metrics OptionalMetrics
}

// NodesQuery - Get list of nodes
func (adm *AdminClient) NodesQuery(ctx context.Context, options *NodesResourceOpts) (*PaginatedNodesResponse, error) {
	values := make(url.Values)

	if options != nil {
		options.Metrics.apply(values)
		// Add pagination and filter parameters if provided
		values.Set("limit", strconv.Itoa(options.Limit))

		if options.Offset > 0 {
			values.Set("offset", strconv.Itoa(options.Offset))
		}

		if options.Filter != "" {
			values.Set("filter", options.Filter)
		}
		if options.Sort != "" {
			values.Set("sort", options.Sort)
		}
		if options.SortReversed {
			values.Set("sortReversed", "true")
		} else {
			values.Set("sortReversed", "false")
		}
	}

	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{
			relPath:     adminAPIPrefix + "/query/nodes",
			queryValues: values,
		})
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	var nodesResp PaginatedNodesResponse
	if err = nodesResp.DecodeMsg(msgp.NewReader(resp.Body)); err != nil {
		return nil, err
	}

	return &nodesResp, nil
}

// DrivesResourceOpts contains the available options for the DrivesQuery API
//
//msgp:ignore DrivesResourceOpts
type DrivesResourceOpts struct {
	// Limit defaults to 100 if set to 0.
	// A limit of -1 will return all results.
	Limit  int
	Offset int
	Filter string
	// Sort fields contained in DriveResource.
	//
	// Example: DrivesResourceOpts.Sort = "ServerIndex"
	// Assuming the value of ServerIndex is of a supported value type.
	//
	// Supported Values Types: int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, string
	Sort string
	// SortReversed will only take effect if Sort is defined
	SortReversed bool
	Metrics      bool // Include per-drive metrics in the response
	LastMinute   bool // Include rolling 1 minute drive metrics. Requires Metrics.
	LastDay      bool // Include segmented 1 day drive metrics. Requires Metrics.
}

// DrivesQuery - Get list of drives
func (adm *AdminClient) DrivesQuery(ctx context.Context, options *DrivesResourceOpts) (*PaginatedDrivesResponse, error) {
	values := make(url.Values)

	if options != nil {
		// Add pagination and filter parameters if provided
		values.Set("limit", strconv.Itoa(options.Limit))

		if options.Offset > 0 {
			values.Set("offset", strconv.Itoa(options.Offset))
		}

		if options.Filter != "" {
			values.Set("filter", options.Filter)
		}
		if options.Sort != "" {
			values.Set("sort", options.Sort)
		}
		if options.SortReversed {
			values.Set("sortReversed", "true")
		} else {
			values.Set("sortReversed", "false")
		}
		if options.Metrics {
			values.Set("metrics", "true")
		}
		if options.LastMinute {
			values.Set("1m", "true")
		}
		if options.LastDay {
			values.Set("24h", "true")
		}
	}

	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{
			relPath:     adminAPIPrefix + "/query/drives",
			queryValues: values,
		})
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	var drivesResp PaginatedDrivesResponse
	if err = drivesResp.DecodeMsg(msgp.NewReader(resp.Body)); err != nil {
		return nil, err
	}

	return &drivesResp, nil
}

// ErasureSetsResourceOpts contains the available options for the ErasureSetsQuery API
//
//msgp:ignore ErasureSetsResourceOpts
type ErasureSetsResourceOpts struct {
	// Limit defaults to 100 if set to 0.
	// A limit of -1 will return all results.
	Limit  int
	Offset int
	Filter string
	// Sort fields contained in ErasureSetResource.
	//
	// Example: ErasureSetsResourceOpts.Sort = "SetIndex"
	// Assuming the value of SetIndex is of a supported value type.
	//
	// Supported Values Types: int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, string
	Sort string
	// SortReversed will only take effect if Sort is defined
	SortReversed bool
}

// ErasureSetsQuery - Get list of erasure sets
func (adm *AdminClient) ErasureSetsQuery(ctx context.Context, options *ErasureSetsResourceOpts) (*PaginatedErasureSetsResponse, error) {
	values := make(url.Values)

	if options != nil {
		// Add pagination and filter parameters if provided
		values.Set("limit", strconv.Itoa(options.Limit))

		if options.Offset > 0 {
			values.Set("offset", strconv.Itoa(options.Offset))
		}

		if options.Filter != "" {
			values.Set("filter", options.Filter)
		}
		if options.Sort != "" {
			values.Set("sort", options.Sort)
		}
		if options.SortReversed {
			values.Set("sortReversed", "true")
		} else {
			values.Set("sortReversed", "false")
		}
	}

	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{
			relPath:     adminAPIPrefix + "/query/sets",
			queryValues: values,
		})
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	var setsResp PaginatedErasureSetsResponse
	if err = setsResp.DecodeMsg(msgp.NewReader(resp.Body)); err != nil {
		return nil, err
	}

	return &setsResp, nil
}

// SortSlice sorts a slice of structs based on a specified field path using reflection.
// The field parameter supports dot notation for nested fields (e.g., "Field1.Field2").
// Supported field types: string, int, uint, float variants, and pointers to these types.
// The field must be exported. Nil values are sorted as "less than" non-nil values.
// If reversed is true, the sort order is reversed.
func SortSlice[T any](slice []T, field string, reversed bool) {
	if field == "" {
		return
	}

	parts := strings.Split(field, ".")
	sort.SliceStable(slice, func(i, j int) bool {
		valI, valINil := dereferenceValue(reflect.ValueOf(slice[i]))
		valJ, valJNil := dereferenceValue(reflect.ValueOf(slice[j]))

		if valINil {
			return !reversed
		}
		if valJNil {
			return reversed
		}

		fieldI := resolveFieldPath(valI, parts)
		fieldJ := resolveFieldPath(valJ, parts)

		lessThan, ok := compareFields(fieldI, fieldJ)
		if !ok {
			return false
		}

		if reversed {
			// For stable reverse sorting, we need to check if j < i
			greaterThan, _ := compareFields(fieldJ, fieldI)
			return greaterThan
		}
		return lessThan
	})
}

// resolveFieldPath traverses a dotted field path on a struct value.
// Field lookups are case-insensitive. Returns an invalid Value if the path
// cannot be fully resolved or if a nil pointer is encountered.
func resolveFieldPath(v reflect.Value, parts []string) reflect.Value {
	current := v
	for i, fieldName := range parts {
		// Unwrap any pointers at this level
		for current.Kind() == reflect.Ptr {
			if current.IsNil() {
				return reflect.Value{}
			}
			current = current.Elem()
		}

		if current.Kind() != reflect.Struct {
			return reflect.Value{}
		}

		// Find field by case-insensitive name
		field := findFieldCaseInsensitive(current, fieldName)
		if !field.IsValid() {
			return reflect.Value{}
		}

		// For intermediate fields, dereference pointers before continuing
		if i < len(parts)-1 {
			current = field
		} else {
			// Return the final field as-is (may be pointer or value)
			return field
		}
	}
	return current
}

// findFieldCaseInsensitive finds a struct field by name, case-insensitively
func findFieldCaseInsensitive(v reflect.Value, name string) reflect.Value {
	typ := v.Type()
	for i := range typ.NumField() {
		if strings.EqualFold(typ.Field(i).Name, name) {
			return v.Field(i)
		}
	}
	return reflect.Value{}
}

// compareFields compares two field values that are either primitives or pointers to primitives.
// Returns (result, true) if comparison is possible, (false, false) if types are unsupported.
// Nil values are considered less than non-nil values.
func compareFields(a, b reflect.Value) (bool, bool) {
	// Dereference pointers if needed
	aVal, aIsNil := dereferenceValue(a)
	bVal, bIsNil := dereferenceValue(b)

	// Handle nil cases
	if aIsNil && bIsNil {
		return false, true // equal
	}
	if aIsNil {
		return true, true // nil < non-nil
	}
	if bIsNil {
		return false, true // non-nil > nil
	}

	// Compare based on kind
	switch aVal.Kind() {
	case reflect.String:
		return strings.ToLower(aVal.String()) < strings.ToLower(bVal.String()), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return aVal.Int() < bVal.Int(), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return aVal.Uint() < bVal.Uint(), true
	case reflect.Float32, reflect.Float64:
		return aVal.Float() < bVal.Float(), true
	default:
		return false, false
	}
}

// dereferenceValue unwraps a pointer value and returns (value, isNil)
func dereferenceValue(v reflect.Value) (reflect.Value, bool) {
	if !v.IsValid() {
		return reflect.Value{}, true
	}
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return reflect.Value{}, true
		}
		return v.Elem(), false
	}
	return v, false
}

// OptionalMetrics indicates optional metrics to include in the response.
type OptionalMetrics struct {
	Types MetricType
	Flags MetricFlags
}

func (o OptionalMetrics) apply(q url.Values) {
	if o.Types != 0 {
		q.Set("metric-types", strconv.FormatUint(uint64(o.Types), 10))
		q.Set("metric-flags", strconv.FormatUint(uint64(o.Flags), 10))
	}
}

// Add adds the given metrics to the OptionalMetrics.
func (o *OptionalMetrics) Add(m ...MetricType) {
	for _, t := range m {
		o.Types = o.Types | t
	}
}

// AddFlags adds the given flags to the OptionalMetrics.
func (o *OptionalMetrics) AddFlags(f ...MetricFlags) {
	for _, f := range f {
		o.Flags = o.Flags | f
	}
}

func (o *OptionalMetrics) Parse(q url.Values) {
	if t, err := strconv.ParseUint(q.Get("metric-types"), 10, 64); err == nil {
		o.Types = MetricType(t)
	}
	if f, err := strconv.ParseUint(q.Get("metric-flags"), 10, 64); err == nil {
		o.Flags = MetricFlags(f)
	}
}
