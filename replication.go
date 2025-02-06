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
	"time"

	"github.com/minio/minio-go/v7/pkg/replication"
)

type ReplicationInfo struct {
	Error         string            `json:"error,omitempty"`
	ActiveWorkers WorkerStat        `json:"active_workers,omitempty"`
	Queued        InQueueMetric     `json:"queued,omitempty"`
	ReplicaCount  int64             `json:"replica_count,omitempty"`
	ReplicaSize   int64             `json:"replica_size,omitempty"`
	Proxying      bool              `json:"proxying,omitempty"`
	Proxied       ReplProxyMetric   `json:"proxied,omitempty"`
	Sites         []ReplicationSite `json:"sites,omitempty"`
}

type ReplicationSite struct {
	Addr string                  `json:"addr,omitempty"`
	Info SiteReplicationSiteInfo `json:"info,omitempty"`
}

type SiteReplicationSiteInfo struct {
	Nodes         []MinIONode `json:"nodes,omitempty"`
	LDAPEnabled   bool        `json:"ldap_enabled,omitempty"`
	OpenIDEnabled bool        `json:"openid_enabled,omitempty"`
	BucketsCount  int         `json:"buckets_count,omitempty"`
	// ReplicationWorkers    int                 `json:"replication_workers"`
	// MaxReplicationWorkers int                 `json:"max_replication_workers"`
	Edge                 bool                `json:"edge,omitempty"`
	ILMEnabled           bool                `json:"ilm_enabled,omitempty"`
	EncryptionEnabled    bool                `json:"encryption_enabled,omitempty"`
	ILMExpiryReplication bool                `json:"ilm_expiry_replication,omitempty"`
	ReplicatedBuckets    []ReplicatedBucket  `json:"replicated_buckets,omitempty"`
	ObjetLocking         ObjectLockingInfo   `json:"object_locking,omitempty"`
	Throttle             ReplicationThrottle `json:"throttle,omitempty"`
	ReplicationTargets   []ReplicationTarget `json:"replication_targets,omitempty"`
	ReplicatedCount      int64               `json:"replicated_count,omitempty"`
	ReplicatedSize       int64               `json:"replicated_size,omitempty"`
}

type MinIONode struct {
	Addr                string `json:"addr,omitempty"`
	MinIOVersion        string `json:"minio_version,omitempty"`
	Uptime              int64  `json:"uptime,omitempty"`
	PoolID              int    `json:"poolid,omitempty"`
	SetID               int    `json:"setid,omitempty"`
	SiteHealingLeader   bool   `json:"site_healing_leader,omitempty"`
	IsScanning          bool   `json:"is_scanning,omitempty"`
	ILMExpiryInProgress bool   `json:"ilm_expiry_in_progress,omitempty"`
	// Resync              BucketResyncInfo `json:"resync"`
}

type ReplicatedBucket struct {
	Name            string                `json:"name,omitempty"`
	ReplicationInfo BucketReplicationInfo `json:"replication_info,omitempty"`
}

type MissingReplicationInfo struct {
	Bucket string            `json:"bucket,omitempty"`
	Config map[string]string `json:"config,omitempty"`
}

type ObjectLockingInfo struct {
	Enabled bool     `json:"enabled,omitempty"`
	Buckets []string `json:"buckets,omitempty"`
}

type ReplicationTarget struct {
	SourceBucket      string            `json:"source_bucket,omitempty"`
	TargetBucket      string            `json:"target_bucket,omitempty"`
	Addr              string            `json:"addr,omitempty"`
	Online            bool              `json:"online,omitempty"`
	TotalDowntime     int64             `json:"total_downtime,omitempty"`
	CurrentDowntime   int64             `json:"current_downtime,omitempty"`
	AdminPermissions  bool              `json:"admin_permissions,omitempty"`
	SyncReplication   bool              `json:"sync_replication,omitempty"`
	HeartbeatErrCount int64             `json:"heartbeat_err_count,omitempty"`
	BandwidthLimit    uint64            `json:"bandwidth_limit,omitempty"`
	Latency           TransferLatency   `json:"xfer_rate,omitempty"`
	Edge              bool              `json:"edge,omitempty"`
	LBEndpoint        bool              `json:"lb_endpoint,omitempty"`
	HeathCheck        HeathCheckDetails `json:"heath_check,omitempty"`
}

type TransferLatency struct {
	Current time.Duration `json:"current,omitempty"`
	Average time.Duration `json:"avg,omitempty"`
	Maximum time.Duration `json:"max,omitempty"`
}

type HeathCheckDetails struct {
	Timestamp time.Time     `json:"timestamp,omitempty"`
	Latency   string        `json:"latency,omitempty"`
	Duration  time.Duration `json:"duration,omitempty"`
}

type BucketReplicationInfo struct {
	VersionEnabled          bool                `json:"version_enabled,omitempty"`
	ObjectLocking           bool                `json:"object_locking,omitempty"`
	Edge                    bool                `json:"edge,omitempty"`
	ExcludedPrefixes        []string            `json:"excluded_prefixes,omitempty"`
	DeleteReplication       bool                `json:"delete_replication,omitempty"`
	DeleteMarkerReplication bool                `json:"delete_marker_replication,omitempty"`
	ILM                     ReplicationILMInfo  `json:"ilm,omitempty"`
	Encryption              ReplicationEncInfo  `json:"encryption,omitempty"`
	Config                  replication.Config  `json:"config,omitempty"`
	ReplicationPriority     int                 `json:"replication_priority,omitempty"`
	Resync                  BucketResyncInfo    `json:"resync,omitempty"`
	Throttle                ReplicationThrottle `json:"throttle,omitempty"`
}

type ReplicationILMInfo struct {
	Enabled bool                 `json:"enabled,omitempty"`
	Rules   []ReplicationILMRule `json:"rules,omitempty"`
}

type ReplicationILMRule struct {
	ID         string `json:"id,omitempty"`
	Expiration bool   `json:"expiration,omitempty"`
	Transition bool   `json:"transition,omitempty"`
}

type ReplicationEncInfo struct {
	Enabled  bool            `json:"enabled,omitempty"`
	EncRules []BucketEncRule `json:"enc_rules,omitempty"`
}

type BucketEncRule struct {
	Algorithm string `json:"algorithm,omitempty"`
	EncKey    string `json:"enc_key,omitempty"`
}

type BucketResyncInfo struct {
	InProgress      bool      `json:"in_progress,omitempty"`
	StartTime       time.Time `json:"start_time,omitempty"`
	FailedCount     int64     `json:"failed_count,omitempty"`
	FailedSize      int64     `json:"failed_size,omitempty"`
	ReplicatedCount int64     `json:"replicated_count,omitempty"`
	ReplicatedSize  int64     `json:"replicated_size,omitempty"`
}

type ReplicationThrottle struct {
	IsSet bool   `json:"is_set,omitempty"`
	Limit uint64 `json:"limit,omitempty"`
}
