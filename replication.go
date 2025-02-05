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
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"
)

const (
	// ReplicationHealthInfoVersion0 is version 0
	ReplicationHealthInfoVersion0 = ""
	// ReplicationHealthInfoVersion is current health info version.
	ReplicationHealthInfoVersion = ReplicationHealthInfoVersion0
)

// ReplicationHealthInfoVersionStruct - struct for health info version
type ReplicationHealthInfoVersionStruct struct {
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

type ReplicationInfo struct {
	Version   string    `json:"version"`
	Error     string    `json:"error"`
	TimeStamp time.Time `json:"timestamp"`

	SRSites           []ReplicationSite       `json:"sr_sites"`
	BucketReplication []BucketReplicationInfo `json:"bucket_replication"`
}

type ReplicationSite struct {
	Addr string                  `json:"addr"`
	Info SiteReplicationSiteInfo `json:"info"`
}

type SiteReplicationSiteInfo struct {
	MinIOVersion          string                   `json:"minio_version"`
	Uptime                int64                    `json:"uptime"`
	LDAPEnabled           bool                     `json:"ldap_enabled"`
	OpenIDEnabled         bool                     `json:"openid_enabled"`
	ErasureSetID          int                      `json:"setid"`
	PoolID                int                      `json:"poolid"`
	BucketsCount          int                      `json:"buckets_count"`
	BucketReplication     bool                     `json:"bucket_replication"`
	ReplicationWorkers    int                      `json:"replication_workers"`
	MaxReplicationWorkers int                      `json:"max_replication_workers"`
	ReplicationPriority   int                      `json:"replication_priority"`
	Edge                  bool                     `json:"edge"`
	ILMEnabled            bool                     `json:"ilm_enabled"`
	EncryptionEnabled     bool                     `json:"encryption_enabled"`
	ILMExpiryReplication  bool                     `json:"ilm_expiry_replication"`
	SiteHealingLeader     bool                     `json:"site_healing_leader"`
	IsScanning            bool                     `json:"is_scanning"`
	ILMExpiryInProgress   bool                     `json:"ilm_expiry_in_progress"`
	ReplicatedBuckets     []ReplicatedBucket       `json:"replicated_buckets"`
	MissingReplication    []MissingReplicationInfo `json:"missing_replication"`
	ObjetLocking          ObjectLockingInfo        `json:"object_locking"`
	ReplicationConfig     []ReplicationConfigInfo  `json:"replication_config"`
	ReplicationTargets    []ReplicationTarget      `json:"replication_targets"`
}

type ReplicatedBucket struct {
	Name     string              `json:"name"`
	Target   string              `json:"target"`
	Throttle ReplicationThrottle `json:"throttle"`
}

type ReplicationThrottle struct {
	Count     int    `json:"count"`
	Bandwidth string `json:"bandwidth"`
}

type MissingReplicationInfo struct {
	Bucket string            `json:"bucket"`
	Config map[string]string `json:"config"`
}

type ObjectLockingInfo struct {
	Enabled bool     `json:"enabled"`
	Buckets []string `json:"buckets"`
}

type ReplicationConfigInfo struct {
	Bucket string            `json:"bucket"`
	Config map[string]string `json:"config"`
}

type ReplicationTarget struct {
	Addr               string            `json:"addr"`
	Reachable          bool              `json:"reachable"`
	Online             bool              `json:"online"`
	TotalDowntime      int64             `json:"total_downtime"`
	CurrentDowntime    int64             `json:"current_downtime"`
	AdminPermissions   bool              `json:"admin_permissions"`
	SyncReplication    bool              `json:"sync_replication"`
	Proxying           bool              `json:"proxying"`
	TotalProxiedCalls  int64             `json:"total_proxied_calls"`
	MissedProxiedCalls int64             `json:"missed_proxied_calls"`
	HeartbeatErrCount  int64             `json:"heartbeat_err_count"`
	ThrottleLimit      bool              `json:"throttle_limit"`
	XFerRate           TransferRate      `json:"xfer_rate"`
	ThrottleBandwidth  string            `json:"throttle_bandwidth"`
	Edge               bool              `json:"edge"`
	LBEndpoint         bool              `json:"lb_endpoint"`
	Sync               bool              `json:"sync"`
	HeathCheck         HeathCheckDetails `json:"heath_check"`
}

type TransferRate struct {
	Current int64 `json:"current"`
	Average int64 `json:"avg"`
	Maximum int64 `json:"max"`
}

type HeathCheckDetails struct {
	Timestamp time.Time     `json:"timestamp"`
	Latency   string        `json:"latency"`
	Duration  time.Duration `json:"duration"`
}

type BucketReplicationInfo struct {
	Bucket                  string             `json:"bucket"`
	VersionEnabled          bool               `json:"version_enabled"`
	ObjectLocking           bool               `json:"object_locking"`
	Edge                    bool               `json:"edge"`
	Target                  string             `json:"target"`
	ExcludedPrefixes        []string           `json:"excluded_prefixes"`
	DeleteReplication       bool               `json:"delete_replication"`
	DeleteMarkerReplication bool               `json:"delete_marker_replication"`
	ILM                     ReplicationILMInfo `json:"ilm"`
	Encryption              ReplicationEncInfo `json:"encryption"`
	Config                  map[string]string  `json:"config"`
	Resync                  ResyncInfo         `json:"resync"`
}

type ReplicationILMInfo struct {
	Enabled bool   `json:"enabled"`
	Policy  string `json:"policy"`
}

type ReplicationEncInfo struct {
	Enabled bool   `json:"enabled"`
	EncKey  string `json:"enc_key"`
}

type ResyncInfo struct {
	InProgress      bool      `json:"in_progress"`
	StartTime       time.Time `json:"start_time"`
	ProgressPercent int       `json:"progress_percent"`
}

type SRHeathOptions struct{}

// SRHealthInfo - returns site replication diagnostics info
func (adm *AdminClient) SRHealthInfo(ctx context.Context, opts SRHeathOptions) (*http.Response, string, error) {
	v := url.Values{}
	resp, err := adm.executeMethod(
		ctx, "GET", requestData{
			relPath:     adminAPIPrefix + "/replication-healthinfo",
			queryValues: v,
		},
	)
	if err != nil {
		closeResponse(resp)
		return nil, "", err
	}

	if resp.StatusCode != http.StatusOK {
		closeResponse(resp)
		return nil, "", httpRespToErrorResponse(resp)
	}

	decoder := json.NewDecoder(resp.Body)
	var version ReplicationHealthInfoVersionStruct
	if err = decoder.Decode(&version); err != nil {
		closeResponse(resp)
		return nil, "", err
	}

	if version.Error != "" {
		closeResponse(resp)
		return nil, "", errors.New(version.Error)
	}

	switch version.Version {
	case ReplicationHealthInfoVersion:
	default:
		closeResponse(resp)
		return nil, "", errors.New("Upgrade Minio Client to support replication health info version " + version.Version)
	}

	return resp, version.Version, nil
}
