// Copyright (c) 2015-2025 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package log

import (
	"strings"
	"time"
)

//msgp:tag json
//go:generate msgp -d clearomitted -d "timezone utc" $GOFILE

// AuditCategory represents the category of audit event
type AuditCategory string

const (
	AuditCategoryConfig         AuditCategory = "config"
	AuditCategoryUser           AuditCategory = "user"
	AuditCategoryServiceAccount AuditCategory = "service-account"
	AuditCategoryPolicy         AuditCategory = "policy"
	AuditCategoryGroup          AuditCategory = "group"
	AuditCategoryBucket         AuditCategory = "bucket"
	AuditCategoryLifecycle      AuditCategory = "lifecycle"
	AuditCategoryReplication    AuditCategory = "replication"
	AuditCategoryNotification   AuditCategory = "notification"
	AuditCategoryEncryption     AuditCategory = "encryption"
	AuditCategoryCORS           AuditCategory = "cors"
	AuditCategoryVersioning     AuditCategory = "versioning"
	AuditCategoryService        AuditCategory = "service"
	AuditCategoryKMS            AuditCategory = "kms"
	AuditCategorySiteRepl       AuditCategory = "site-replication"
	AuditCategoryPool           AuditCategory = "pool"
	AuditCategoryIDP            AuditCategory = "idp"
	AuditCategoryLogRecorder    AuditCategory = "log-recorder"
)

// AuditAction represents the type of action performed
type AuditAction string

const (
	AuditActionCreate  AuditAction = "create"
	AuditActionUpdate  AuditAction = "update"
	AuditActionDelete  AuditAction = "delete"
	AuditActionEnable  AuditAction = "enable"
	AuditActionDisable AuditAction = "disable"
	AuditActionSet     AuditAction = "set"
	AuditActionReset   AuditAction = "reset"
	AuditActionRestore AuditAction = "restore"
	AuditActionClear   AuditAction = "clear"
	AuditActionStart   AuditAction = "start"
	AuditActionStop    AuditAction = "stop"
	AuditActionRestart AuditAction = "restart"
	AuditActionAttach  AuditAction = "attach"
	AuditActionDetach  AuditAction = "detach"
)

// Audit represents the user triggered audit events
type Audit struct {
	Version    string                 `json:"version"`
	Time       time.Time              `json:"time"`
	Node       string                 `json:"node,omitempty"`
	APIName    string                 `json:"apiName,omitempty"`
	Category   AuditCategory          `json:"category,omitempty"`
	Action     AuditAction            `json:"action,omitempty"`
	Bucket     string                 `json:"bucket,omitempty"`
	Tags       map[string]string      `json:"tags,omitempty"`
	RequestID  string                 `json:"requestID,omitempty"`
	ReqClaims  map[string]interface{} `json:"requestClaims,omitempty"`
	SourceHost string                 `json:"sourceHost,omitempty"`
	AccessKey  string                 `json:"accessKey,omitempty"`
	ParentUser string                 `json:"parentUser,omitempty"`
	Details    map[string]interface{} `json:"details,omitempty"`
}

// ConfigAuditDetails captures config mutation details
type ConfigAuditDetails struct {
	SubSystem string `json:"subSystem,omitempty"`
	Target    string `json:"target,omitempty"`
	Key       string `json:"key,omitempty"`
	OldValue  string `json:"oldValue,omitempty"`
	NewValue  string `json:"newValue,omitempty"`
}

// UserAuditDetails captures user mutation details
type UserAuditDetails struct {
	UserName  string   `json:"userName"`
	UserType  string   `json:"userType,omitempty"`
	Field     string   `json:"field,omitempty"`
	OldValue  string   `json:"oldValue,omitempty"`
	NewValue  string   `json:"newValue,omitempty"`
	OldStatus string   `json:"oldStatus,omitempty"`
	NewStatus string   `json:"newStatus,omitempty"`
	Policies  []string `json:"policies,omitempty"`
	Groups    []string `json:"groups,omitempty"`
}

// ServiceAccountAuditDetails captures service account details
type ServiceAccountAuditDetails struct {
	AccountName      string    `json:"accountName"`
	ParentUser       string    `json:"parentUser,omitempty"`
	Policies         []string  `json:"policies,omitempty"`
	Expiration       time.Time `json:"expiration,omitempty"`
	UpdatedName      string    `json:"updatedName,omitempty"`
	UpdatedStatus    string    `json:"updatedStatus,omitempty"`
	UpdatedPolicy    bool      `json:"updatedPolicy,omitempty"`
	UpdatedExpiry    bool      `json:"updatedExpiry,omitempty"`
	UpdatedSecretKey bool      `json:"updatedSecretKey,omitempty"`
}

// PolicyAuditDetails captures policy mutation details
type PolicyAuditDetails struct {
	PolicyName       string   `json:"policyName"`
	OldPolicy        string   `json:"oldPolicy,omitempty"`
	NewPolicy        string   `json:"newPolicy,omitempty"`
	Operation        string   `json:"operation,omitempty"`
	User             string   `json:"user,omitempty"`
	Group            string   `json:"group,omitempty"`
	PoliciesAttached []string `json:"policiesAttached,omitempty"`
	PoliciesDetached []string `json:"policiesDetached,omitempty"`
}

// GroupAuditDetails captures group mutation details
type GroupAuditDetails struct {
	GroupName      string   `json:"groupName"`
	MembersAdded   []string `json:"membersAdded,omitempty"`
	MembersRemoved []string `json:"membersRemoved,omitempty"`
	OldStatus      string   `json:"oldStatus,omitempty"`
	NewStatus      string   `json:"newStatus,omitempty"`
}

// BucketConfigAuditDetails captures bucket configuration changes
type BucketConfigAuditDetails struct {
	BucketName   string   `json:"bucketName"`
	ConfigType   string   `json:"configType,omitempty"`
	OldConfig    string   `json:"oldConfig,omitempty"`
	NewConfig    string   `json:"newConfig,omitempty"`
	TargetBucket string   `json:"targetBucket,omitempty"`
	TagKeys      []string `json:"tagKeys,omitempty"`
	TagCount     int      `json:"tagCount,omitempty"`
}

// ServiceAuditDetails captures service operation details
type ServiceAuditDetails struct {
	ServiceName string            `json:"serviceName,omitempty"`
	Operation   string            `json:"operation,omitempty"`
	Status      string            `json:"status,omitempty"`
	IAMImport   *IAMImportDetails `json:"iamImport,omitempty"`
}

// IAMImportDetails captures IAM import operation counts
type IAMImportDetails struct {
	UsersAdded      int `json:"usersAdded,omitempty"`
	PoliciesAdded   int `json:"policiesAdded,omitempty"`
	GroupsAdded     int `json:"groupsAdded,omitempty"`
	SvcAcctsAdded   int `json:"svcAcctsAdded,omitempty"`
	UsersRemoved    int `json:"usersRemoved,omitempty"`
	PoliciesRemoved int `json:"policiesRemoved,omitempty"`
	GroupsRemoved   int `json:"groupsRemoved,omitempty"`
	SvcAcctsRemoved int `json:"svcAcctsRemoved,omitempty"`
	UsersSkipped    int `json:"usersSkipped,omitempty"`
	PoliciesSkipped int `json:"policiesSkipped,omitempty"`
	GroupsSkipped   int `json:"groupsSkipped,omitempty"`
	SvcAcctsSkipped int `json:"svcAcctsSkipped,omitempty"`
	UsersFailed     int `json:"usersFailed,omitempty"`
	PoliciesFailed  int `json:"policiesFailed,omitempty"`
	GroupsFailed    int `json:"groupsFailed,omitempty"`
	SvcAcctsFailed  int `json:"svcAcctsFailed,omitempty"`
}

// KMSAuditDetails captures KMS operation details
type KMSAuditDetails struct {
	KeyID     string `json:"keyId,omitempty"`
	Operation string `json:"operation,omitempty"`
}

// PoolAuditDetails captures pool operation details
type PoolAuditDetails struct {
	PoolIndex int      `json:"poolIndex,omitempty"`
	Endpoints []string `json:"endpoints,omitempty"`
	Operation string   `json:"operation,omitempty"`
}

// SiteReplicationAuditDetails captures site replication details
type SiteReplicationAuditDetails struct {
	SiteName  string   `json:"siteName,omitempty"`
	Endpoint  string   `json:"endpoint,omitempty"`
	Operation string   `json:"operation,omitempty"`
	Sites     []string `json:"sites,omitempty"`
}

// IDPAuditDetails captures identity provider configuration details
type IDPAuditDetails struct {
	IDPName   string `json:"idpName,omitempty"`
	IDPType   string `json:"idpType,omitempty"`
	ConfigKey string `json:"configKey,omitempty"`
	OldValue  string `json:"oldValue,omitempty"`
	NewValue  string `json:"newValue,omitempty"`
}

// LogRecorderAuditDetails captures log recorder configuration details
type LogRecorderAuditDetails struct {
	LogType          string `json:"logType,omitempty"`
	OldEnabled       bool   `json:"oldEnabled,omitempty"`
	NewEnabled       bool   `json:"newEnabled,omitempty"`
	OldLimit         string `json:"oldLimit,omitempty"`
	NewLimit         string `json:"newLimit,omitempty"`
	OldFlushCount    int    `json:"oldFlushCount,omitempty"`
	NewFlushCount    int    `json:"newFlushCount,omitempty"`
	OldFlushInterval string `json:"oldFlushInterval,omitempty"`
	NewFlushInterval string `json:"newFlushInterval,omitempty"`
}

// String returns a canonical string for Audit
func (a Audit) String() string {
	values := []string{
		toString("version", a.Version),
		toTime("time", a.Time),
		toString("node", a.Node),
		toString("apiName", a.APIName),
		toString("category", string(a.Category)),
		toString("action", string(a.Action)),
		toString("bucket", a.Bucket),
		toMap("tags", a.Tags),
		toString("requestID", a.RequestID),
		toInterfaceMap("requestClaims", a.ReqClaims),
		toString("sourceHost", a.SourceHost),
		toString("accessKey", a.AccessKey),
		toString("parentUser", a.ParentUser),
		toInterfaceMap("details", a.Details),
	}
	values = filterAndSort(values)
	return strings.Join(values, ",")
}
