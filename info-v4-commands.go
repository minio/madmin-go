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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/tinylib/msgp/msgp"
)

//msgp:clearomitted
//msgp:tag json
//go:generate msgp -file $GOFILE

// ClusterInfo cluster level information
type ClusterInfo struct {
	Version      string `msg:"version"`
	DeploymentID string `msg:"deploymentID"`
	SiteName     string `msg:"siteName"`
	SiteRegion   string `msg:"siteRegion"`
	License      struct {
		Organization string `msg:"org"`
		Type         string `msg:"type"`
		Expiry       string `msg:"expiry"`
	} `msg:"license"`
	Platform string     `msg:"platform"`
	Domain   []string   `msg:"domain"`
	Pools    []PoolInfo `msg:"pools"`
	Metrics  struct {
		Buckets       uint64 `msg:"buckets"`
		Objects       uint64 `msg:"objects"`
		Versions      uint64 `msg:"versions"`
		DeleteMarkers uint64 `msg:"deleteMarkers"`
		Usage         uint64 `msg:"usage"`
	} `msg:"metrics"`
}

// PoolInfo per pool specific information
type PoolInfo struct {
	Index int `msg:"index"`
	Nodes struct {
		Total   int `msg:"total"`
		Offline int `msg:"offline"`
	} `msg:"nodes"`
	Drives struct {
		PerNodeTotal   int `msg:"perNode"`
		PerNodeOffline int `msg:"perNodeOffline"`
	}
	TotalSets   int `msg:"numberOfSets"`
	StripeSize  int `msg:"stripeSize"`
	WriteQuorum int `msg:"writeQuorum"`
	ReadQuorum  int `msg:"readQuorum"`

	// Optional value, not returned in ClusterInfo, PoolList API calls
	Hosts []string `msg:"hosts,omitempty"`
}

// ClusterInfoOpts ask for additional data from the server
// this is not used at the moment, kept here for future
// extensibility.
//
//msgp:ignore ClusterInfoOpts
type ClusterInfoOpts struct{}

// ClusterInfo - Connect to a minio server and call Server Admin Info Management API
// to fetch server's information represented by infoMessage structure
func (adm *AdminClient) ClusterInfo(ctx context.Context, options ...func(*ClusterInfoOpts)) (ClusterInfo, error) {
	srvOpts := &ClusterInfoOpts{}

	for _, o := range options {
		o(srvOpts)
	}

	values := make(url.Values)
	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{
			relPath:     adminAPIPrefixV4 + "/cluster",
			queryValues: values,
		})
	defer closeResponse(resp)
	if err != nil {
		return ClusterInfo{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return ClusterInfo{}, httpRespToErrorResponse(resp)
	}

	var info ClusterInfo
	if err = info.DecodeMsg(msgp.NewReader(resp.Body)); err != nil {
		return ClusterInfo{}, err
	}

	return info, nil
}

// PoolInfoOpts ask for additional data from the server
// this is not used at the moment, kept here for future
// extensibility.
//
//msgp:ignore PoolInfoOpts
type PoolInfoOpts struct{}

// PoolList list all the pools on the server
func (adm *AdminClient) PoolList(ctx context.Context, options ...func(*PoolInfoOpts)) (pools []PoolInfo, err error) {
	srvOpts := &PoolInfoOpts{}

	for _, o := range options {
		o(srvOpts)
	}

	values := make(url.Values)
	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{
			relPath:     adminAPIPrefixV4 + "/pool",
			queryValues: values,
		})
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	mr := msgp.NewReader(resp.Body)
	for {
		var info PoolInfo
		if err = info.DecodeMsg(mr); err != nil {
			if errors.Is(err, io.EOF) {
				err = nil
			}
			break
		}
		pools = append(pools, info)
	}

	return pools, err
}

// PoolInfo returns pool information about a specific pool referenced by poolIndex
func (adm *AdminClient) PoolInfo(ctx context.Context, poolIndex int, options ...func(*PoolInfoOpts)) (PoolInfo, error) {
	srvOpts := &PoolInfoOpts{}

	for _, o := range options {
		o(srvOpts)
	}

	values := make(url.Values)
	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{
			relPath:     adminAPIPrefixV4 + fmt.Sprintf("/pool/%d", poolIndex),
			queryValues: values,
		})
	defer closeResponse(resp)
	if err != nil {
		return PoolInfo{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return PoolInfo{}, httpRespToErrorResponse(resp)
	}

	var info PoolInfo
	if err = info.DecodeMsg(msgp.NewReader(resp.Body)); err != nil {
		return PoolInfo{}, err
	}

	return info, nil
}

// SetInfoOpts ask for additional data from the server
// this is not used at the moment, kept here for future
// extensibility.
//
//msgp:ignore SetInfoOpts
type SetInfoOpts struct {
	Metrics bool
}

// ExtendedErasureSetInfo provides information per erasure set
type ExtendedErasureSetInfo struct {
	ID                 int    `json:"id"`
	RawUsage           uint64 `json:"rawUsage"`
	RawCapacity        uint64 `json:"rawCapacity"`
	Usage              uint64 `json:"usage"`
	ObjectsCount       uint64 `json:"objectsCount"`
	VersionsCount      uint64 `json:"versionsCount"`
	DeleteMarkersCount uint64 `json:"deleteMarkersCount"`
	HealDisks          int    `json:"healDisks"`
	Drives             []Disk `json:"drives,omitempty"`
}

func (adm *AdminClient) SetInfo(ctx context.Context, poolIndex int, setIndex int, options ...func(*SetInfoOpts)) (ExtendedErasureSetInfo, error) {
	srvOpts := &SetInfoOpts{}

	for _, o := range options {
		o(srvOpts)
	}

	values := make(url.Values)
	if srvOpts.Metrics {
		values.Add("metrics", "true")
	}

	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{
			relPath:     adminAPIPrefixV4 + fmt.Sprintf("/set/%d/%d", poolIndex, setIndex),
			queryValues: values,
		})
	defer closeResponse(resp)
	if err != nil {
		return ExtendedErasureSetInfo{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return ExtendedErasureSetInfo{}, httpRespToErrorResponse(resp)
	}

	var info ExtendedErasureSetInfo
	if err = info.DecodeMsg(msgp.NewReader(resp.Body)); err != nil {
		return ExtendedErasureSetInfo{}, err
	}

	return info, nil
}

// DriveInfoOpts ask for additional data from the server
// this is not used at the moment, kept here for future
// extensibility.
//
//msgp:ignore DriveInfoOpts
type DiskInfoOpts struct{}

// DiskInfo returns pool information about a specific pool referenced by poolIndex
func (adm *AdminClient) DiskInfo(ctx context.Context, poolIndex, setIndex, diskIndex int, options ...func(*DiskInfoOpts)) (Disk, error) {
	srvOpts := &DiskInfoOpts{}

	for _, o := range options {
		o(srvOpts)
	}

	values := make(url.Values)
	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{
			relPath:     adminAPIPrefixV4 + fmt.Sprintf("/drive/%d/%d/%d", poolIndex, setIndex, diskIndex),
			queryValues: values,
		})
	defer closeResponse(resp)
	if err != nil {
		return Disk{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return Disk{}, httpRespToErrorResponse(resp)
	}

	var disk Disk
	if err = disk.DecodeMsg(msgp.NewReader(resp.Body)); err != nil {
		return Disk{}, err
	}

	return disk, nil
}
