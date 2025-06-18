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
	"strconv"

	"github.com/tinylib/msgp/msgp"
)

//msgp:clearomitted
//msgp:timezone utc
//go:generate msgp -file $GOFILE

// PaginatedPoolsResponse represents a paginated response for pools
type PaginatedPoolsResponse struct {
	Results []PoolResource `json:"results" msg:"r,omitempty"`
	Count   int            `json:"count" msg:"c"`
	Total   int            `json:"total" msg:"t"`
}

// PaginatedNodesResponse represents a paginated response for nodes
type PaginatedNodesResponse struct {
	Results []NodeResource `json:"results" msg:"r,omitempty"`
	Count   int            `json:"count" msg:"c"`
	Total   int            `json:"total" msg:"t"`
}

// PaginatedDrivesResponse represents a paginated response for drives
type PaginatedDrivesResponse struct {
	Results []DriveResource `json:"results" msg:"r,omitempty"`
	Count   int             `json:"count" msg:"c"`
	Total   int             `json:"total" msg:"t"`
}

// PaginatedErasureSetsResponse represents a paginated response for erasure sets
type PaginatedErasureSetsResponse struct {
	Results []ErasureSetResource `json:"results" msg:"r,omitempty"`
	Count   int                  `json:"count" msg:"c"`
	Total   int                  `json:"total" msg:"t"`
}

type PoolLayout struct {
	Servers       int `json:"servers" msg:"s"`
	Drives        int `json:"drives" msg:"d"`
	DrivesOffline int `json:"drivesOffline" msg:"do"`
	DrivesHealing int `json:"drivesHealing" msg:"dh"`
}

// ClusterResource represents high-level information about the cluster
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
	DeleteMarkerCount int          `json:"deleteMarkerCount" msg:"dc"`
	TotalSize         uint64       `json:"totalSize" msg:"ts"`
	OnlineDrives      int          `json:"onlineDrives" msg:"od"`
	OfflineDrives     int          `json:"offlineDrives" msg:"fd"`
}

// ServicesResourceInfo holds information about services connected to the cluster
type ServicesResourceInfo struct {
	LDAP          LDAP                          `json:"ldap" msg:"l"`
	Logger        []Logger                      `json:"logger,omitempty" msg:"lg,omitempty"`
	Audit         []Audit                       `json:"audit,omitempty" msg:"a,omitempty"`
	Notifications []map[string][]TargetIDStatus `json:"notifications,omitempty" msg:"n,omitempty"`
	KMSStatus     []KMS                         `json:"kms,omitempty" msg:"k,omitempty"`
}

// PoolResource represents information about a storage pool
type PoolResource struct {
	PoolIndex          int      `json:"poolindex" msg:"i"`
	StripeSize         int      `json:"stripeSize" msg:"ss"`
	TotalSets          int      `json:"totalSets" msg:"ts"`
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
	DeleteMarkersCount uint64   `json:"deleteMarkersCount" msg:"dc"`
}

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

// NodeResource represents information about a node
type NodeResource struct {
	Host        string      `json:"host" msg:"h"`
	Version     string      `json:"version" msg:"v"`
	CommitID    string      `json:"commitId" msg:"c"`
	Uptime      int64       `json:"uptime" msg:"u"`
	State       string      `json:"state" msg:"s"`
	TotalDrives int         `json:"totalDrives" msg:"td"`
	DriveCounts DriveCounts `json:"driveCounts" msg:"dc"`
	PoolIndex   int         `json:"poolIndex" msg:"pi"`
	PoolIndexes []int       `json:"poolIndexes,omitempty" msg:"pis,omitempty"`
}

// DriveResource represents information about a drive
type DriveResource struct {
	ID          string `json:"id" msg:"i"`
	DriveIndex  int    `json:"idx" msg:"idx"`
	ServerIndex int    `json:"serverIndex" msg:"sidx"`
	Path        string `json:"path" msg:"p"`
	NodeID      string `json:"nodeId" msg:"ni"`
	PoolIndex   int    `json:"poolIndex" msg:"pi"`
	SetIndex    int    `json:"setIndex" msg:"si"`
	State       string `json:"state" msg:"s"`
	Healing     bool   `json:"healing" msg:"h"`
	Size        uint64 `json:"size" msg:"sz"`
	Used        uint64 `json:"used" msg:"u"`
	Available   uint64 `json:"available" msg:"a"`
	UUID        string `json:"uuid" msg:"uid"`
}

// ErasureSetResource represents information about an erasure set
type ErasureSetResource struct {
	PoolIndex          int      `json:"poolIndex" msg:"pi"`
	SetIndex           int      `json:"setIndex" msg:"si"`
	DriveCount         int      `json:"driveCount" msg:"dc"`
	OfflineDrives      int      `json:"offlineDrives" msg:"od"`
	OnlineDrives       int      `json:"onlineDrives" msg:"ond"`
	HealDisks          int      `json:"healDisks" msg:"hd"`
	ReadQuorum         int      `json:"readQuorum" msg:"rq"`
	WriteQuorum        int      `json:"writeQuorum" msg:"wq"`
	Nodes              []string `json:"nodes,omitempty" msg:"n,omitempty"`
	RawUsage           uint64   `json:"rawUsage" msg:"ru"`
	RawCapacity        uint64   `json:"rawCapacity" msg:"rc"`
	Usage              uint64   `json:"usage" msg:"u"`
	ObjectsCount       uint64   `json:"objectsCount" msg:"oc"`
	VersionsCount      uint64   `json:"versionsCount" msg:"vc"`
	DeleteMarkersCount uint64   `json:"deleteMarkersCount" msg:"dc"`
}

// ClusterResourceOpts ask for additional data from the server
// this is not used at the moment, kept here for future
// extensibility.
//
//msgp:ignore ClusterResourceOpts
type ClusterResourceOpts struct{}

// ClusterQuery - Get high-level information about the cluster
func (adm *AdminClient) ClusterQuery(ctx context.Context, options ...func(*ClusterResourceOpts)) (ClusterResource, error) {
	srvOpts := &ClusterResourceOpts{}

	for _, o := range options {
		o(srvOpts)
	}

	values := make(url.Values)
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
func (adm *AdminClient) ServicesQuery(ctx context.Context, options ...func(*ServicesResourceOpts)) (ServicesResourceInfo, error) {
	srvOpts := &ServicesResourceOpts{}

	for _, o := range options {
		o(srvOpts)
	}

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

// PoolsResourceOpts ask for additional data from the server
// this is not used at the moment, kept here for future
// extensibility.
//
//msgp:ignore PoolsResourceOpts
type PoolsResourceOpts struct {
	Limit  int
	Offset int
	Filter string
}

func (adm *AdminClient) PoolsQuery(ctx context.Context, options *PoolsResourceOpts) (*PaginatedPoolsResponse, error) {
	values := make(url.Values)

	if options != nil {
		// Add pagination and filter parameters if provided
		if options.Limit > 0 {
			values.Set("limit", strconv.Itoa(options.Limit))
		}

		if options.Offset > 0 {
			values.Set("offset", strconv.Itoa(options.Offset))
		}

		if options.Filter != "" {
			values.Set("filter", options.Filter)
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

// NodesResourceOpts ask for additional data from the server
// this is not used at the moment, kept here for future
// extensibility.
//
//msgp:ignore NodesResourceOpts
type NodesResourceOpts struct {
	Limit  int
	Offset int
	Filter string
}

// NodesQuery - Get list of nodes
func (adm *AdminClient) NodesQuery(ctx context.Context, options *NodesResourceOpts) (*PaginatedNodesResponse, error) {
	values := make(url.Values)

	if options != nil {
		// Add pagination and filter parameters if provided
		if options.Limit > 0 {
			values.Set("limit", strconv.Itoa(options.Limit))
		}

		if options.Offset > 0 {
			values.Set("offset", strconv.Itoa(options.Offset))
		}

		if options.Filter != "" {
			values.Set("filter", options.Filter)
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

// DrivesResourceOpts ask for additional data from the server
// this is not used at the moment, kept here for future
// extensibility.
//
//msgp:ignore DrivesResourceOpts
type DrivesResourceOpts struct {
	Limit  int
	Offset int
	Filter string
}

// DrivesQuery - Get list of drives
func (adm *AdminClient) DrivesQuery(ctx context.Context, options *DrivesResourceOpts) (*PaginatedDrivesResponse, error) {
	values := make(url.Values)

	if options != nil {
		// Add pagination and filter parameters if provided
		if options.Limit > 0 {
			values.Set("limit", strconv.Itoa(options.Limit))
		}

		if options.Offset > 0 {
			values.Set("offset", strconv.Itoa(options.Offset))
		}

		if options.Filter != "" {
			values.Set("filter", options.Filter)
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

// ErasureSetsResourceOpts ask for additional data from the server
// this is not used at the moment, kept here for future
// extensibility.
//
//msgp:ignore ErasureSetsResourceOpts
type ErasureSetsResourceOpts struct {
	Limit  int
	Offset int
	Filter string
}

// ErasureSetsQuery - Get list of erasure sets
func (adm *AdminClient) ErasureSetsQuery(ctx context.Context, options *ErasureSetsResourceOpts) (*PaginatedErasureSetsResponse, error) {
	values := make(url.Values)

	if options != nil {
		// Add pagination and filter parameters if provided
		if options.Limit > 0 {
			values.Set("limit", strconv.Itoa(options.Limit))
		}

		if options.Offset > 0 {
			values.Set("offset", strconv.Itoa(options.Offset))
		}

		if options.Filter != "" {
			values.Set("filter", options.Filter)
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
