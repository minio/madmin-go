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
	"fmt"
	"net/url"
	"time"
)

//msgp:clearomitted
//msgp:tag json
//msgp:timezone utc
//go:generate msgp

// BucketTargets represents a slice of bucket targets by type and endpoint
type BucketTargets struct {
	Targets []BucketTarget `json:"targets"`
}

// Empty returns true if struct is empty.
func (t BucketTargets) Empty() bool {
	if len(t.Targets) == 0 {
		return true
	}
	empty := true
	for _, t := range t.Targets {
		if !t.Empty() {
			return false
		}
	}
	return empty
}

// ServiceType represents service type
type ServiceType string

const (
	// ReplicationService specifies replication service
	ReplicationService ServiceType = "replication"
)

// IsValid returns true if ARN type represents replication
func (t ServiceType) IsValid() bool {
	return t == ReplicationService
}

// BucketTarget represents the target bucket and site association.
type BucketTarget struct {
	SourceBucket         string        `json:"sourcebucket"`
	Endpoint             string        `json:"endpoint"`
	Credentials          *Credentials  `json:"credentials"`
	TargetBucket         string        `json:"targetbucket"`
	Secure               bool          `json:"secure"`
	Path                 string        `json:"path,omitempty"`
	API                  string        `json:"api,omitempty"`
	Arn                  string        `json:"arn,omitempty"`
	Type                 ServiceType   `json:"type"`
	Region               string        `json:"region,omitempty"`
	BandwidthLimit       int64         `json:"bandwidthlimit,omitempty"`
	ReplicationSync      bool          `json:"replicationSync"`
	StorageClass         string        `json:"storageclass,omitempty"`
	HealthCheckDuration  time.Duration `json:"healthCheckDuration,omitempty"`
	DisableProxy         bool          `json:"disableProxy"`
	ResetBeforeDate      time.Time     `json:"resetBeforeDate,omitempty"`
	ResetID              string        `json:"resetID,omitempty"`
	TotalDowntime        time.Duration `json:"totalDowntime"`
	LastOnline           time.Time     `json:"lastOnline"`
	Online               bool          `json:"isOnline"`
	Latency              LatencyStat   `json:"latency"`
	DeploymentID         string        `json:"deploymentID,omitempty"`
	Edge                 bool          `json:"edge"`                 // target is recipient of edge traffic
	EdgeSyncBeforeExpiry bool          `json:"edgeSyncBeforeExpiry"` // must replicate to edge before expiry
	OfflineCount         int64         `json:"offlineCount"`
}

// Credentials holds access and secret keys.
type Credentials struct {
	AccessKey    string    `xml:"AccessKeyId" json:"accessKey,omitempty"`
	SecretKey    string    `xml:"SecretAccessKey" json:"secretKey,omitempty"`
	SessionToken string    `xml:"SessionToken" json:"sessionToken,omitempty"`
	Expiration   time.Time `xml:"Expiration" json:"expiration,omitempty"`
}

// Clone returns shallow clone of BucketTarget without secret key in credentials
func (t *BucketTarget) Clone() BucketTarget {
	return BucketTarget{
		SourceBucket:         t.SourceBucket,
		Endpoint:             t.Endpoint,
		TargetBucket:         t.TargetBucket,
		Credentials:          &Credentials{AccessKey: t.Credentials.AccessKey},
		Secure:               t.Secure,
		Path:                 t.Path,
		API:                  t.API,
		Arn:                  t.Arn,
		Type:                 t.Type,
		Region:               t.Region,
		BandwidthLimit:       t.BandwidthLimit,
		ReplicationSync:      t.ReplicationSync,
		StorageClass:         t.StorageClass, // target storage class
		HealthCheckDuration:  t.HealthCheckDuration,
		DisableProxy:         t.DisableProxy,
		ResetBeforeDate:      t.ResetBeforeDate,
		ResetID:              t.ResetID,
		TotalDowntime:        t.TotalDowntime,
		LastOnline:           t.LastOnline,
		Online:               t.Online,
		Latency:              t.Latency,
		DeploymentID:         t.DeploymentID,
		Edge:                 t.Edge,
		EdgeSyncBeforeExpiry: t.EdgeSyncBeforeExpiry,
		OfflineCount:         t.OfflineCount,
	}
}

// URL returns target url
func (t BucketTarget) URL() *url.URL {
	scheme := "http"
	if t.Secure {
		scheme = "https"
	}
	return &url.URL{
		Scheme: scheme,
		Host:   t.Endpoint,
	}
}

// Empty returns true if struct is empty.
func (t BucketTarget) Empty() bool {
	return t.String() == "" || t.Credentials == nil
}

func (t *BucketTarget) String() string {
	return fmt.Sprintf("%s %s", t.Endpoint, t.TargetBucket)
}
