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

// ReplDiagInfo represents the replication diagnostic information to ba captured
// part of health diagnostic information
type ReplDiagInfo struct {
	Error               string               `json:"error,omitempty"`
	SREnabled           bool                 `json:"site_replication_enabled"`
	Sites               []ReplDiagSite       `json:"sites,omitempty"`
	RDReplicatedBuckets []ReplDiagReplBucket `json:"replicated_buckets,omitempty"`
	SRMetrics           []ReplDiagSRMetric   `json:"sr_metrics,omitempty"`
}

// ReplDiagSite represents the replication site information
type ReplDiagSite struct {
	Addr         string           `json:"addr,omitempty"`
	DeploymentID string           `json:"deployment_id"`
	Info         ReplDiagSiteInfo `json:"info,omitempty"`
}

// ReplDiagSiteInfo represents the replication diagnostic information for a site
type ReplDiagSiteInfo struct {
	Nodes                []ReplDiagNode   `json:"nodes,omitempty"`
	LDAPEnabled          bool             `json:"ldap_enabled,omitempty"`
	OpenIDEnabled        bool             `json:"openid_enabled,omitempty"`
	BucketsCount         int              `json:"buckets_count,omitempty"`
	Edge                 bool             `json:"edge,omitempty"`
	ILMEnabled           bool             `json:"ilm_enabled,omitempty"`
	EncryptionEnabled    bool             `json:"encryption_enabled,omitempty"`
	ILMExpiryReplication bool             `json:"ilm_expiry_replication,omitempty"`
	ObjectLockingEnabled bool             `json:"object_locking_enabled,omitempty"`
	Throttle             ReplDiagThrottle `json:"throttle,omitempty"`
	ReplicatedCount      int64            `json:"replicated_count,omitempty"`
	ReplicatedSize       int64            `json:"replicated_size,omitempty"`
	ResyncStatus         string           `json:"resync_status"`
}

// ReplDiagNode represents the replication diagnostic information for a node
// in a site
type ReplDiagNode struct {
	Addr                string `json:"addr,omitempty"`
	MinIOVersion        string `json:"minio_version,omitempty"`
	Uptime              int64  `json:"uptime,omitempty"`
	PoolID              int    `json:"poolid,omitempty"`
	IsLeader            bool   `json:"is_leader,omitempty"`
	ILMExpiryInProgress bool   `json:"ilm_expiry_in_progress,omitempty"`
}

// ReplDiagReplBucket represents the replication diagnostic information for a bucket
type ReplDiagReplBucket struct {
	Name               string                     `json:"name,omitempty"`
	ReplicationInfo    ReplDiagBucketReplInfo     `json:"replication_info,omitempty"`
	ReplicationTargets []ReplDiagBucketReplTarget `json:"replication_targets,omitempty"`
}

// ReplDiagBucketReplTarget represents the replication target information for a bucket
type ReplDiagBucketReplTarget struct {
	SourceBucket              string                  `json:"source_bucket,omitempty"`
	TargetBucket              string                  `json:"target_bucket,omitempty"`
	Addr                      string                  `json:"addr,omitempty"`
	Online                    bool                    `json:"online,omitempty"`
	TotalDowntime             time.Duration           `json:"total_downtime,omitempty"`
	CurrentDowntime           time.Duration           `json:"current_downtime,omitempty"`
	Permissions               ObjectManagePermissions `json:"permissions,omitempty"`
	SyncReplication           bool                    `json:"sync_replication,omitempty"`
	HeartbeatErrCount         int64                   `json:"heartbeat_err_count,omitempty"`
	BandwidthLimit            uint64                  `json:"bandwidth_limit,omitempty"`
	Latency                   LatencyStat             `json:"xfer_rate,omitempty"`
	Edge                      bool                    `json:"edge,omitempty"`
	HealthCheckDuration       time.Duration           `json:"heath_check,omitempty"`
	DisableProxying           bool                    `json:"disable_proxying"`
	DeleteReplication         bool                    `json:"delete_replication,omitempty"`
	DeleteMarkerReplication   bool                    `json:"delete_marker_replication,omitempty"`
	ReplicationPriority       int                     `json:"replication_priority,omitempty"`
	ExistingObjectReplication bool                    `json:"existing_object_replication,omitempty"`
	MetadataSync              bool                    `json:"metadata_sync,omitempty"`
}

// ObjectManagePermissions represents the permissions for managing objects on a target site
type ObjectManagePermissions struct {
	DeleteObjectAllowed       bool `json:"delete_object_allowed,omitempty"`
	PuObjectAllowed           bool `json:"put_object_allowed,omitempty"`
	PutObjectRetentionAllowed bool `json:"put_object_retention_allowed,omitempty"`
	PutObjectLegalHoldAllowed bool `json:"put_object_legal_hold_allowed,omitempty"`
}

// ReplDiagBucketReplInfo represents the metadata for a replicated bucket
type ReplDiagBucketReplInfo struct {
	VersionEnabled   bool                     `json:"version_enabled,omitempty"`
	ObjectLocking    bool                     `json:"object_locking,omitempty"`
	ExcludedPrefixes []string                 `json:"excluded_prefixes,omitempty"`
	ILM              ReplDiagILMInfo          `json:"ilm,omitempty"`
	Encryption       ReplDiagEncInfo          `json:"encryption,omitempty"`
	Config           replication.Config       `json:"config,omitempty"`
	Resync           ReplDiagBucketResyncInfo `json:"resync,omitempty"`
}

// ReplDiagILMInfo represents the ILM rules details for a replicated bucket
type ReplDiagILMInfo struct {
	Enabled bool              `json:"enabled,omitempty"`
	Rules   []ReplDiagILMRule `json:"rules,omitempty"`
}

// ReplDiagILMRule represents individual ILM rule details for a replicated bucket
type ReplDiagILMRule struct {
	ID         string `json:"id,omitempty"`
	Expiration bool   `json:"expiration,omitempty"`
	Transition bool   `json:"transition,omitempty"`
}

// ReplDiagEncInfo represents the encryption information for a replicated bucket
type ReplDiagEncInfo struct {
	Enabled  bool            `json:"enabled,omitempty"`
	EncRules []BucketEncInfo `json:"enc_rules,omitempty"`
}

// BucketEncInfo represents the encryption information for a replicated bucket
type BucketEncInfo struct {
	Algorithm string `json:"algorithm,omitempty"`
	EncKey    string `json:"enc_key,omitempty"`
}

// ReplDiagBucketResyncInfo represents resync counters for a replicated bucket
type ReplDiagBucketResyncInfo struct {
	InProgress      bool      `json:"in_progress,omitempty"`
	StartTime       time.Time `json:"start_time,omitempty"`
	FailedCount     int64     `json:"failed_count,omitempty"`
	FailedSize      int64     `json:"failed_size,omitempty"`
	ReplicatedCount int64     `json:"replicated_count,omitempty"`
	ReplicatedSize  int64     `json:"replicated_size,omitempty"`
}

// ReplDiagThrottle represents the replication throttle information
type ReplDiagThrottle struct {
	IsSet bool   `json:"is_set,omitempty"`
	Limit uint64 `json:"limit,omitempty"`
}

// ReplDiagSRMetric represents the site replication metrics for a site
type ReplDiagSRMetric struct {
	Node          string          `json:"node,omitempty"`
	ActiveWorkers WorkerStat      `json:"active_workers,omitempty"`
	Queued        InQueueMetric   `json:"queued,omitempty"`
	ReplicaCount  int64           `json:"replica_count,omitempty"`
	ReplicaSize   int64           `json:"replica_size,omitempty"`
	Proxying      bool            `json:"proxying,omitempty"`
	Proxied       ReplProxyMetric `json:"proxied,omitempty"`
}
