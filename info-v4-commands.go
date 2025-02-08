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
		Buckets       int   `msg:"buckets"`
		Objects       int   `msg:"objects"`
		Versions      int   `msg:"versions"`
		DeleteMarkers int   `msg:"deleteMarkers"`
		Usage         int64 `msg:"usage"`
	} `msg:"metrics"`
}

// PoolInfo per pool specific information
type PoolInfo struct {
	Index int `msg:"index"`
	Nodes struct {
		Total   int `msg:"total"`
		Offline int `msg:"offline"`
	} `msg: "nodes"`
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

	// Check response http status code
	if resp.StatusCode != http.StatusOK {
		return ClusterInfo{}, httpRespToErrorResponse(resp)
	}

	// Unmarshal the server's msgp response
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

	// Check response http status code
	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	// Unmarshal the server's msgp response
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

	// Check response http status code
	if resp.StatusCode != http.StatusOK {
		return PoolInfo{}, httpRespToErrorResponse(resp)
	}

	// Unmarshal the server's msgp response
	var info PoolInfo
	if err = info.DecodeMsg(msgp.NewReader(resp.Body)); err != nil {
		return PoolInfo{}, err
	}

	return info, nil
}
