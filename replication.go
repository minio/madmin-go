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
)

// ReplDiagInfo represents the replication diagnostic information to be captured
// as part of health diagnostic information
type ReplDiagInfo struct {
	Error                  string                                     `json:"error,omitempty"`
	SREnabled              bool                                       `json:"site_replication_enabled"`
	TotalUsers             int                                        `json:"total_users,omitempty"`
	SyncPendingUsers       int                                        `json:"sync_pending_users,omitempty"`
	TotalGroups            int                                        `json:"total_groups,omitempty"`
	SyncPendingGroups      int                                        `json:"sync_pending_groups,omitempty"`
	TotalPolicies          int                                        `json:"total_policies,omitempty"`
	SyncPendingPolicies    int                                        `json:"sync_pending_policies,omitempty"`
	TotalILMExpRules       int                                        `json:"total_ilm_exp_rules,omitempty"`
	SyncPendingILMExpRules int                                        `json:"sync_pending_ilm_exp_rules,omitempty"`
	TotalBuckets           int                                        `json:"total_buckets,omitempty"`
	SyncPendingBuckets     int                                        `json:"sync_pending_buckets,omitempty"`
	Errors                 Counter                                    `json:"errors,omitempty"`
	Retries                Counter                                    `json:"retries,omitempty"`
	Sites                  []ReplDiagSite                             `json:"sites,omitempty"`
	ReplBuckets            []ReplDiagBucket                           `json:"repl_buckets,omitempty"`
	UserPolMismatches      map[string]map[string]SRPolicyStatsSummary `json:"user_policy_mismatches,omitempty"`
	GroupPolMismatches     map[string]map[string]SRGroupStatsSummary  `json:"group_policy_mismatches,omitempty"`
}

// ReplDiagInfoV2 represents the replication diagnostic information to be captured
// as part of health diagnostic information
type ReplDiagInfoV2 struct {
	Error                  string                          `json:"error,omitempty"`
	SREnabled              bool                            `json:"site_replication_enabled"`
	TotalUsers             int                             `json:"total_users,omitempty"`
	SyncPendingUsers       CountWithList                   `json:"sync_pending_users,omitempty"`
	TotalGroups            int                             `json:"total_groups,omitempty"`
	SyncPendingGroups      CountWithList                   `json:"sync_pending_groups,omitempty"`
	TotalPolicies          int                             `json:"total_policies,omitempty"`
	SyncPendingPolicies    CountWithList                   `json:"sync_pending_policies,omitempty"`
	TotalILMExpRules       int                             `json:"total_ilm_exp_rules,omitempty"`
	SyncPendingILMExpRules CountWithList                   `json:"sync_pending_ilm_exp_rules,omitempty"`
	TotalBuckets           int                             `json:"total_buckets,omitempty"`
	SyncPendingBuckets     CountWithList                   `json:"sync_pending_buckets,omitempty"`
	Errors                 Counter                         `json:"errors,omitempty"`
	Retries                Counter                         `json:"retries,omitempty"`
	Sites                  []ReplDiagSite                  `json:"sites,omitempty"`
	ReplBuckets            []ReplDiagBucketV2              `json:"repl_buckets,omitempty"`
	UserPolMismatches      map[string]SRPolicyStatsSummary `json:"user_policy_mismatches,omitempty"`
	GroupPolMismatches     map[string]SRGroupStatsSummary  `json:"group_policy_mismatches,omitempty"`
}

// CountWithList is a type that holds a count and a list of items
type CountWithList struct {
	Count int      `json:"count"`
	List  []string `json:"list,omitempty"`
}

// ReplDiagSite represents the replication site information
type ReplDiagSite struct {
	Addr         string `json:"addr,omitempty"`
	DeploymentID string `json:"deployment_id"`
	Online       bool   `json:"online"`
}

// ReplDiagBucket represents the replication target information for a bucket
type ReplDiagBucket struct {
	Name               string                          `json:"name,omitempty"`
	MetadataMismatches map[string]SRBucketStatsSummary `json:"metadata_mismatches,omitempty"`
	Targets            []BucketReplTarget              `json:"targets,omitempty"`
}

// ReplDiagBucketV2 represents the replication target information for a bucket
type ReplDiagBucketV2 struct {
	Name               string               `json:"name,omitempty"`
	MetadataMismatches SRBucketStatsSummary `json:"metadata_mismatches,omitempty"`
	Targets            []BucketReplTarget   `json:"targets,omitempty"`
}

type BucketReplTarget struct {
	SourceBucket    string        `json:"source_bucket,omitempty"`
	TargetBucket    string        `json:"target_bucket,omitempty"`
	Addr            string        `json:"addr,omitempty"`
	Online          bool          `json:"online"`
	TotalDowntime   time.Duration `json:"total_downtime,omitempty"`
	CurrentDowntime time.Duration `json:"current_downtime,omitempty"`
}
