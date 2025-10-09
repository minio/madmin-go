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
	"fmt"
	"strings"
	"time"
)

var sensitiveFields = []string{"password", "secret", "secretkey", "accesskey", "token"}

func redactIfSensitive(key, value string) string {
	lowerKey := strings.ToLower(key)
	for _, field := range sensitiveFields {
		if strings.Contains(lowerKey, field) {
			return "***REDACTED***"
		}
	}
	return value
}

//msgp:tag json
//go:generate msgp -d clearomitted -d "timezone utc" $GOFILE

// AuditCategory represents the category of audit event
//
//msgp:shim AuditCategory as:string
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
	AuditCategoryHeal           AuditCategory = "heal"
	AuditCategoryBatch          AuditCategory = "batch"
)

// AuditAction represents the type of action performed
//
//msgp:shim AuditAction as:string
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

// AuditDetails is a union type containing category-specific audit details.
// Only one field should be populated based on the audit event category.
type AuditDetails struct {
	Config          *ConfigAuditDetails          `json:"config,omitempty"`
	User            *UserAuditDetails            `json:"user,omitempty"`
	ServiceAccount  *ServiceAccountAuditDetails  `json:"serviceAccount,omitempty"`
	Policy          *PolicyAuditDetails          `json:"policy,omitempty"`
	Group           *GroupAuditDetails           `json:"group,omitempty"`
	BucketConfig    *BucketConfigAuditDetails    `json:"bucketConfig,omitempty"`
	BucketQuota     *BucketQuotaAuditDetails     `json:"bucketQuota,omitempty"`
	BucketQOS       *BucketQOSAuditDetails       `json:"bucketQOS,omitempty"`
	BucketInventory *BucketInventoryAuditDetails `json:"bucketInventory,omitempty"`
	Tier            *TierAuditDetails            `json:"tier,omitempty"`
	Service         *ServiceAuditDetails         `json:"service,omitempty"`
	KMS             *KMSAuditDetails             `json:"kms,omitempty"`
	Pool            *PoolAuditDetails            `json:"pool,omitempty"`
	SiteRepl        *SiteReplicationAuditDetails `json:"siteRepl,omitempty"`
	IDP             *IDPAuditDetails             `json:"idp,omitempty"`
	Recorder        *RecorderAuditDetails        `json:"recorder,omitempty"`
	Heal            *HealAuditDetails            `json:"heal,omitempty"`
	Batch           *BatchAuditDetails           `json:"batch,omitempty"`
}

// Audit represents the user triggered audit events.
// It captures administrative operations performed on the MinIO cluster with contextual metadata.
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
	Details    *AuditDetails          `json:"details,omitempty"`
}

// ConfigAuditDetails captures config mutation details.
// It tracks changes to server configuration settings including subsystem, target, key, and value changes.
type ConfigAuditDetails struct {
	SubSystem string `json:"subSystem,omitempty"`
	Target    string `json:"target,omitempty"`
	Key       string `json:"key,omitempty"`
	OldValue  string `json:"oldValue,omitempty"`
	NewValue  string `json:"newValue,omitempty"`
}

// Redact redacts sensitive fields in ConfigAuditDetails
func (c *ConfigAuditDetails) Redact() {
	c.OldValue = redactIfSensitive(c.Key, c.OldValue)
	c.NewValue = redactIfSensitive(c.Key, c.NewValue)
}

// UserAuditDetails captures user mutation details.
// It tracks changes to user accounts including status, credentials, policies, and group memberships.
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

// Redact redacts sensitive fields in UserAuditDetails
func (u *UserAuditDetails) Redact() {
	u.OldValue = redactIfSensitive(u.Field, u.OldValue)
	u.NewValue = redactIfSensitive(u.Field, u.NewValue)
}

// ServiceAccountAuditDetails captures service account details.
// It tracks changes to service accounts including parent user, policies, expiration, and secret key updates.
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

// Redact redacts sensitive fields in ServiceAccountAuditDetails
func (s *ServiceAccountAuditDetails) Redact() {}

// PolicyAuditDetails captures policy mutation details.
// It tracks IAM policy changes including policy content, attachments, and detachments for users or groups.
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

// Redact redacts sensitive fields in PolicyAuditDetails
func (p *PolicyAuditDetails) Redact() {}

// GroupAuditDetails captures group mutation details.
// It tracks changes to IAM groups including member additions, removals, and status changes.
type GroupAuditDetails struct {
	GroupName      string   `json:"groupName"`
	MembersAdded   []string `json:"membersAdded,omitempty"`
	MembersRemoved []string `json:"membersRemoved,omitempty"`
	OldStatus      string   `json:"oldStatus,omitempty"`
	NewStatus      string   `json:"newStatus,omitempty"`
}

// Redact redacts sensitive fields in GroupAuditDetails
func (g *GroupAuditDetails) Redact() {}

// BucketConfigAuditDetails captures bucket configuration changes.
// It tracks modifications to bucket settings like lifecycle, replication, encryption, versioning, and tags.
type BucketConfigAuditDetails struct {
	BucketName   string   `json:"bucketName"`
	ConfigType   string   `json:"configType,omitempty"`
	OldConfig    string   `json:"oldConfig,omitempty"`
	NewConfig    string   `json:"newConfig,omitempty"`
	TargetBucket string   `json:"targetBucket,omitempty"`
	TagKeys      []string `json:"tagKeys,omitempty"`
	TagCount     int      `json:"tagCount,omitempty"`
}

// Redact redacts sensitive fields in BucketConfigAuditDetails
func (b *BucketConfigAuditDetails) Redact() {}

// ServiceAuditDetails captures service operation details.
// It tracks MinIO service operations like restart, update, IAM import/export, and cluster management actions.
type ServiceAuditDetails struct {
	ServiceName string            `json:"serviceName,omitempty"`
	Operation   string            `json:"operation,omitempty"`
	Status      string            `json:"status,omitempty"`
	Legacy      bool              `json:"legacy,omitempty"`
	IAMImport   *IAMImportDetails `json:"iamImport,omitempty"`
}

// Redact redacts sensitive fields in ServiceAuditDetails
func (s *ServiceAuditDetails) Redact() {}

// IAMImportDetails captures IAM import operation counts.
// It tracks the number of users, policies, groups, and service accounts added, removed, skipped, or failed during IAM imports.
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

// KMSAuditDetails captures KMS operation details.
// It tracks Key Management Service operations like key creation, deletion, and encryption/decryption activities.
type KMSAuditDetails struct {
	KeyID     string `json:"keyId,omitempty"`
	Operation string `json:"operation,omitempty"`
}

// Redact redacts sensitive fields in KMSAuditDetails
func (k *KMSAuditDetails) Redact() {}

// PoolAuditDetails captures pool operation details.
// It tracks storage pool operations like expansion, decommission, and rebalancing across multiple endpoints.
type PoolAuditDetails struct {
	PoolIndex int      `json:"poolIndex,omitempty"`
	Endpoints []string `json:"endpoints,omitempty"`
	Operation string   `json:"operation,omitempty"`
}

// Redact redacts sensitive fields in PoolAuditDetails
func (p *PoolAuditDetails) Redact() {}

// SiteReplicationAuditDetails captures site replication details.
// It tracks multi-site replication operations including site additions, removals, and replication status changes.
type SiteReplicationAuditDetails struct {
	SiteName  string   `json:"siteName,omitempty"`
	Endpoint  string   `json:"endpoint,omitempty"`
	Operation string   `json:"operation,omitempty"`
	Sites     []string `json:"sites,omitempty"`
}

// Redact redacts sensitive fields in SiteReplicationAuditDetails
func (s *SiteReplicationAuditDetails) Redact() {}

// IDPAuditDetails captures identity provider configuration details.
// It tracks changes to IDP configurations like LDAP, OpenID, or SAML settings including credentials and endpoints.
type IDPAuditDetails struct {
	IDPName   string `json:"idpName,omitempty"`
	IDPType   string `json:"idpType,omitempty"`
	ConfigKey string `json:"configKey,omitempty"`
	OldValue  string `json:"oldValue,omitempty"`
	NewValue  string `json:"newValue,omitempty"`
}

// Redact redacts sensitive fields in IDPAuditDetails
func (i *IDPAuditDetails) Redact() {
	i.OldValue = redactIfSensitive(i.ConfigKey, i.OldValue)
	i.NewValue = redactIfSensitive(i.ConfigKey, i.NewValue)
}

// RecorderAuditDetails captures log recorder configuration details.
// It tracks changes to audit/error log recorder settings including enable status, limits, flush intervals, and batch sizes.
type RecorderAuditDetails struct {
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

// Redact redacts sensitive fields in RecorderAuditDetails
func (r *RecorderAuditDetails) Redact() {}

// HealAuditDetails captures heal operation details.
// It tracks data healing operations that scan and repair inconsistent or missing objects in buckets.
type HealAuditDetails struct {
	Operation string `json:"operation,omitempty"`
	Bucket    string `json:"bucket,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
}

// Redact redacts sensitive fields in HealAuditDetails
func (h *HealAuditDetails) Redact() {}

// BatchAuditDetails captures batch job operation details.
// It tracks batch operations like replication jobs, key rotation, and object expiration tasks.
type BatchAuditDetails struct {
	JobID   string `json:"jobID,omitempty"`
	JobType string `json:"jobType,omitempty"`
	User    string `json:"user,omitempty"`
}

// Redact redacts sensitive fields in BatchAuditDetails
func (b *BatchAuditDetails) Redact() {}

// Message returns a human-readable message for batch audit
func (b BatchAuditDetails) Message() string {
	if b.JobType != "" && b.JobID != "" {
		return "Batch job '" + b.JobType + "' (" + b.JobID + ")"
	}
	if b.JobID != "" {
		return "Batch job " + b.JobID
	}
	return "Batch job operation"
}

// Details returns specific parameter changes for batch audit
func (b BatchAuditDetails) Details() string {
	var parts []string
	if b.User != "" {
		parts = append(parts, "user:"+b.User)
	}
	return strings.Join(parts, " ")
}

// BucketQuotaAuditDetails captures bucket quota configuration changes.
// It tracks changes to bucket storage quotas including size limits and quota type (hard/FIFO).
type BucketQuotaAuditDetails struct {
	BucketName string `json:"bucketName"`
	QuotaSize  uint64 `json:"quotaSize,omitempty"`
	QuotaType  string `json:"quotaType,omitempty"`
}

// Redact redacts sensitive fields in BucketQuotaAuditDetails
func (q *BucketQuotaAuditDetails) Redact() {}

// Message returns a human-readable message for bucket quota audit
func (q BucketQuotaAuditDetails) Message() string {
	if q.BucketName != "" {
		return "Bucket quota for '" + q.BucketName + "'"
	}
	return "Bucket quota modified"
}

// Details returns specific parameter changes for bucket quota audit
func (q BucketQuotaAuditDetails) Details() string {
	var parts []string
	if q.QuotaSize > 0 {
		parts = append(parts, fmt.Sprintf("size:%d", q.QuotaSize))
	}
	if q.QuotaType != "" {
		parts = append(parts, "type:"+q.QuotaType)
	}
	return strings.Join(parts, " ")
}

// BucketQOSAuditDetails captures bucket QoS configuration changes.
// It tracks Quality of Service settings for buckets including rate limits, burst sizes, and priority rules for API operations.
type BucketQOSAuditDetails struct {
	BucketName string          `json:"bucketName"`
	Enabled    bool            `json:"enabled"`
	Rules      []QOSRuleDetail `json:"rules,omitempty"`
}

// QOSRuleDetail captures details of a single QoS rule.
// Each rule defines rate limiting for specific object prefixes or API operations with priority levels and burst capacities.
type QOSRuleDetail struct {
	ID           string `json:"id,omitempty"`
	Label        string `json:"label,omitempty"`
	Priority     int    `json:"priority,omitempty"`
	ObjectPrefix string `json:"objectPrefix,omitempty"`
	API          string `json:"api,omitempty"`
	Rate         int64  `json:"rate,omitempty"`
	Burst        int64  `json:"burst,omitempty"`
	LimitType    string `json:"limitType,omitempty"`
}

// Redact redacts sensitive fields in BucketQOSAuditDetails
func (q *BucketQOSAuditDetails) Redact() {}

// Message returns a human-readable message for bucket QoS audit
func (q BucketQOSAuditDetails) Message() string {
	if q.BucketName != "" {
		status := "disabled"
		if q.Enabled {
			status = "enabled"
		}
		return "Bucket QoS for '" + q.BucketName + "' " + status
	}
	return "Bucket QoS modified"
}

// Details returns specific parameter changes for bucket QoS audit
func (q BucketQOSAuditDetails) Details() string {
	if len(q.Rules) > 0 {
		return fmt.Sprintf("rules:%d", len(q.Rules))
	}
	return ""
}

// BucketInventoryAuditDetails captures bucket inventory configuration changes.
// It tracks bucket inventory report settings including destination bucket, schedule, and inventory configuration IDs.
type BucketInventoryAuditDetails struct {
	BucketName        string `json:"bucketName"`
	InventoryID       string `json:"inventoryID,omitempty"`
	DestinationBucket string `json:"destinationBucket,omitempty"`
	Schedule          string `json:"schedule,omitempty"`
}

// Redact redacts sensitive fields in BucketInventoryAuditDetails
func (i *BucketInventoryAuditDetails) Redact() {}

// Message returns a human-readable message for bucket inventory audit
func (i BucketInventoryAuditDetails) Message() string {
	if i.BucketName != "" && i.InventoryID != "" {
		return "Bucket inventory '" + i.InventoryID + "' for '" + i.BucketName + "'"
	}
	if i.BucketName != "" {
		return "Bucket inventory for '" + i.BucketName + "'"
	}
	return "Bucket inventory modified"
}

// Details returns specific parameter changes for bucket inventory audit
func (i BucketInventoryAuditDetails) Details() string {
	var parts []string
	if i.DestinationBucket != "" {
		parts = append(parts, "dest:"+i.DestinationBucket)
	}
	if i.Schedule != "" {
		parts = append(parts, "schedule:"+i.Schedule)
	}
	return strings.Join(parts, " ")
}

// TierAuditDetails captures tier configuration changes.
// It tracks remote tier configurations for lifecycle transitions including S3, Azure, GCS, and MinIO tiers.
type TierAuditDetails struct {
	TierName string `json:"tierName"`
	TierType string `json:"tierType,omitempty"`
}

// Redact redacts sensitive fields in TierAuditDetails
func (t *TierAuditDetails) Redact() {}

// Message returns a human-readable message for tier audit
func (t TierAuditDetails) Message() string {
	if t.TierName != "" && t.TierType != "" {
		return "Tier '" + t.TierName + "' (" + t.TierType + ")"
	}
	if t.TierName != "" {
		return "Tier '" + t.TierName + "'"
	}
	return "Tier configuration modified"
}

// Details returns specific parameter changes for tier audit
func (t TierAuditDetails) Details() string {
	return ""
}

// String returns a simple string representation for Audit (required by eos LogEntry interface)
func (a Audit) String() string {
	return fmt.Sprintf("audit: category=%s action=%s api=%s", a.Category, a.Action, a.APIName)
}

// Message returns a short summary of the config mutation
func (c ConfigAuditDetails) Message() string {
	if c.SubSystem == "" {
		return "Configuration changed"
	}
	subsys := c.SubSystem
	if len(subsys) > 0 {
		subsys = strings.ToUpper(subsys[:1]) + subsys[1:]
	}
	if c.Target != "" {
		return subsys + " target '" + c.Target + "' configuration changed"
	}
	return subsys + " configuration changed"
}

// Details returns specific parameter changes for config audit
func (c ConfigAuditDetails) Details() string {
	var parts []string
	if c.Key != "" {
		parts = append(parts, c.Key)
	}
	if c.OldValue != "" && c.NewValue != "" {
		parts = append(parts, truncate(c.OldValue, 20)+" → "+truncate(c.NewValue, 20))
	} else if c.NewValue != "" {
		parts = append(parts, "→ "+truncate(c.NewValue, 20))
	}
	return strings.Join(parts, " ")
}

// Message returns a short summary of the user mutation
func (u UserAuditDetails) Message() string {
	if u.UserName == "" {
		return "User modified"
	}
	userType := ""
	if u.UserType != "" {
		userType = " (" + u.UserType + ")"
	}
	if u.Field != "" {
		return "User '" + u.UserName + "'" + userType + " " + u.Field + " changed"
	}
	if u.OldStatus != u.NewStatus && u.NewStatus != "" {
		return "User '" + u.UserName + "'" + userType + " status changed to " + u.NewStatus
	}
	return "User '" + u.UserName + "'" + userType + " modified"
}

// Details returns specific parameter changes for user audit
func (u UserAuditDetails) Details() string {
	var parts []string
	if u.OldValue != "" && u.NewValue != "" {
		parts = append(parts, truncate(u.OldValue, 15)+" → "+truncate(u.NewValue, 15))
	} else if u.NewValue != "" {
		parts = append(parts, "→ "+truncate(u.NewValue, 15))
	}
	if u.OldStatus != "" && u.NewStatus != "" {
		parts = append(parts, u.OldStatus+" → "+u.NewStatus)
	} else if u.NewStatus != "" {
		parts = append(parts, "→ "+u.NewStatus)
	}
	if len(u.Policies) > 0 {
		parts = append(parts, "$"+strings.Join(u.Policies, ","))
	}
	if len(u.Groups) > 0 {
		parts = append(parts, "@"+strings.Join(u.Groups, ","))
	}
	return strings.Join(parts, " ")
}

// Message returns a short summary of the service account mutation
func (s ServiceAccountAuditDetails) Message() string {
	if s.AccountName == "" {
		return "Service account modified"
	}
	if s.ParentUser != "" {
		return "Service account '" + s.AccountName + "' (parent: " + s.ParentUser + ") modified"
	}
	return "Service account '" + s.AccountName + "' modified"
}

// Details returns specific parameter changes for service account audit
func (s ServiceAccountAuditDetails) Details() string {
	var parts []string
	if s.UpdatedStatus != "" {
		parts = append(parts, "→ "+s.UpdatedStatus)
	}
	if s.UpdatedPolicy && len(s.Policies) > 0 {
		parts = append(parts, "$"+strings.Join(s.Policies, ","))
	}
	if s.UpdatedExpiry && !s.Expiration.IsZero() {
		parts = append(parts, "⏱ "+s.Expiration.Format("2006-01-02"))
	}
	if s.UpdatedSecretKey {
		parts = append(parts, "🔑")
	}
	return strings.Join(parts, " ")
}

// Message returns a short summary of the policy mutation
func (p PolicyAuditDetails) Message() string {
	if p.PolicyName == "" {
		return "Policy modified"
	}
	if p.Operation != "" {
		if p.User != "" {
			return "Policy " + p.Operation + " for user '" + p.User + "'"
		}
		if p.Group != "" {
			return "Policy " + p.Operation + " for group '" + p.Group + "'"
		}
		return "Policy '" + p.PolicyName + "' " + p.Operation
	}
	return "Policy '" + p.PolicyName + "' modified"
}

// Details returns specific parameter changes for policy audit
func (p PolicyAuditDetails) Details() string {
	var parts []string
	if p.User != "" {
		parts = append(parts, "@"+p.User)
	}
	if p.Group != "" {
		parts = append(parts, "@@"+p.Group)
	}
	if len(p.PoliciesAttached) > 0 {
		parts = append(parts, "+"+strings.Join(p.PoliciesAttached, ","))
	}
	if len(p.PoliciesDetached) > 0 {
		parts = append(parts, "-"+strings.Join(p.PoliciesDetached, ","))
	}
	if p.OldPolicy != "" && p.NewPolicy != "" {
		parts = append(parts, "{...} → {...}")
	} else if p.NewPolicy != "" {
		parts = append(parts, "→ {...}")
	}
	return strings.Join(parts, " ")
}

// Message returns a short summary of the group mutation
func (g GroupAuditDetails) Message() string {
	if g.GroupName == "" {
		return "Group modified"
	}
	if len(g.MembersAdded) > 0 {
		return "Group '" + g.GroupName + "' members added"
	}
	if len(g.MembersRemoved) > 0 {
		return "Group '" + g.GroupName + "' members removed"
	}
	if g.OldStatus != g.NewStatus && g.NewStatus != "" {
		return "Group '" + g.GroupName + "' status changed to " + g.NewStatus
	}
	return "Group '" + g.GroupName + "' modified"
}

// Details returns specific parameter changes for group audit
func (g GroupAuditDetails) Details() string {
	var parts []string
	if len(g.MembersAdded) > 0 {
		parts = append(parts, "+"+strings.Join(g.MembersAdded, ","))
	}
	if len(g.MembersRemoved) > 0 {
		parts = append(parts, "-"+strings.Join(g.MembersRemoved, ","))
	}
	if g.OldStatus != "" && g.NewStatus != "" {
		parts = append(parts, g.OldStatus+" → "+g.NewStatus)
	} else if g.NewStatus != "" {
		parts = append(parts, "→ "+g.NewStatus)
	}
	return strings.Join(parts, " ")
}

// Message returns a short summary of the bucket config mutation
func (b BucketConfigAuditDetails) Message() string {
	if b.BucketName == "" {
		return "Bucket configuration changed"
	}
	if b.ConfigType != "" {
		return "Bucket '" + b.BucketName + "' " + b.ConfigType + " configuration changed"
	}
	return "Bucket '" + b.BucketName + "' configuration changed"
}

// Details returns specific parameter changes for bucket config audit
func (b BucketConfigAuditDetails) Details() string {
	var parts []string
	if b.TargetBucket != "" {
		parts = append(parts, "→ "+b.TargetBucket)
	}
	if len(b.TagKeys) > 0 {
		parts = append(parts, "#"+strings.Join(b.TagKeys, ","))
	} else if b.TagCount > 0 {
		parts = append(parts, fmt.Sprintf("#%d", b.TagCount))
	}
	if b.OldConfig != "" && b.NewConfig != "" {
		parts = append(parts, truncate(b.OldConfig, 20)+" → "+truncate(b.NewConfig, 20))
	} else if b.NewConfig != "" {
		parts = append(parts, "→ "+truncate(b.NewConfig, 20))
	}
	return strings.Join(parts, " ")
}

// Message returns a short summary of the service operation
func (s ServiceAuditDetails) Message() string {
	prefix := ""
	if s.Legacy {
		prefix = "[legacy] "
	}
	if s.ServiceName != "" && s.Operation != "" {
		return prefix + "Service '" + s.ServiceName + "' " + s.Operation
	}
	if s.Operation == "iam-import" && s.IAMImport != nil {
		return prefix + "IAM import completed"
	}
	if s.Operation != "" {
		return prefix + "Service " + s.Operation
	}
	return prefix + "Service operation performed"
}

// Details returns specific parameter changes for service audit
func (s ServiceAuditDetails) Details() string {
	var parts []string
	if s.Legacy {
		parts = append(parts, "⚠legacy")
	}
	if s.Status != "" {
		parts = append(parts, s.Status)
	}
	if s.IAMImport != nil {
		if s.IAMImport.UsersAdded > 0 {
			parts = append(parts, fmt.Sprintf("@+%d", s.IAMImport.UsersAdded))
		}
		if s.IAMImport.PoliciesAdded > 0 {
			parts = append(parts, fmt.Sprintf("$+%d", s.IAMImport.PoliciesAdded))
		}
		if s.IAMImport.GroupsAdded > 0 {
			parts = append(parts, fmt.Sprintf("@@+%d", s.IAMImport.GroupsAdded))
		}
		if s.IAMImport.SvcAcctsAdded > 0 {
			parts = append(parts, fmt.Sprintf("svc+%d", s.IAMImport.SvcAcctsAdded))
		}
		if s.IAMImport.UsersRemoved > 0 {
			parts = append(parts, fmt.Sprintf("@-%d", s.IAMImport.UsersRemoved))
		}
		if s.IAMImport.PoliciesRemoved > 0 {
			parts = append(parts, fmt.Sprintf("$-%d", s.IAMImport.PoliciesRemoved))
		}
		if s.IAMImport.GroupsRemoved > 0 {
			parts = append(parts, fmt.Sprintf("@@-%d", s.IAMImport.GroupsRemoved))
		}
		if s.IAMImport.SvcAcctsRemoved > 0 {
			parts = append(parts, fmt.Sprintf("svc-%d", s.IAMImport.SvcAcctsRemoved))
		}
	}
	return strings.Join(parts, " ")
}

// Message returns a short summary of the KMS operation
func (k KMSAuditDetails) Message() string {
	if k.KeyID != "" && k.Operation != "" {
		return "KMS key '" + k.KeyID + "' " + k.Operation
	}
	if k.Operation != "" {
		return "KMS " + k.Operation
	}
	return "KMS operation performed"
}

// Details returns specific parameter changes for KMS audit
func (k KMSAuditDetails) Details() string {
	if k.KeyID != "" {
		return truncate(k.KeyID, 30)
	}
	return ""
}

// Message returns a short summary of the pool operation
func (p PoolAuditDetails) Message() string {
	if p.Operation != "" {
		return "Pool " + fmt.Sprintf("%d", p.PoolIndex) + " " + p.Operation
	}
	return "Pool " + fmt.Sprintf("%d", p.PoolIndex) + " modified"
}

// Details returns specific parameter changes for pool audit
func (p PoolAuditDetails) Details() string {
	if len(p.Endpoints) > 0 {
		return fmt.Sprintf("⊙×%d", len(p.Endpoints))
	}
	return ""
}

// Message returns a short summary of the site replication operation
func (s SiteReplicationAuditDetails) Message() string {
	if s.SiteName != "" && s.Operation != "" {
		return "Site '" + s.SiteName + "' " + s.Operation
	}
	if s.Operation != "" {
		return "Site replication " + s.Operation
	}
	return "Site replication operation performed"
}

// Details returns specific parameter changes for site replication audit
func (s SiteReplicationAuditDetails) Details() string {
	var parts []string
	if s.Endpoint != "" {
		parts = append(parts, truncate(s.Endpoint, 30))
	}
	if len(s.Sites) > 0 {
		parts = append(parts, fmt.Sprintf("⇄×%d", len(s.Sites)))
	}
	return strings.Join(parts, " ")
}

// Message returns a short summary of the IDP configuration change
func (i IDPAuditDetails) Message() string {
	if i.IDPName != "" && i.IDPType != "" {
		return "IDP '" + i.IDPName + "' (" + i.IDPType + ") configuration changed"
	}
	if i.IDPName != "" {
		return "IDP '" + i.IDPName + "' configuration changed"
	}
	return "IDP configuration changed"
}

// Details returns specific parameter changes for IDP audit
func (i IDPAuditDetails) Details() string {
	var parts []string
	if i.ConfigKey != "" {
		parts = append(parts, i.ConfigKey)
	}
	if i.OldValue != "" && i.NewValue != "" {
		parts = append(parts, truncate(i.OldValue, 15)+" → "+truncate(i.NewValue, 15))
	} else if i.NewValue != "" {
		parts = append(parts, "→ "+truncate(i.NewValue, 15))
	}
	return strings.Join(parts, " ")
}

// Message returns a short summary of the log recorder configuration change
func (r RecorderAuditDetails) Message() string {
	if r.LogType != "" {
		if r.OldEnabled != r.NewEnabled {
			if r.NewEnabled {
				return "Log recorder '" + r.LogType + "' enabled"
			}
			return "Log recorder '" + r.LogType + "' disabled"
		}
		return "Log recorder '" + r.LogType + "' configuration changed"
	}
	return "Log recorder configuration changed"
}

// Details returns specific parameter changes for log recorder audit
func (r RecorderAuditDetails) Details() string {
	var parts []string
	if r.OldLimit != r.NewLimit && r.NewLimit != "" {
		parts = append(parts, "↕"+r.NewLimit)
	}
	if r.OldFlushCount != r.NewFlushCount {
		parts = append(parts, fmt.Sprintf("⊕%d", r.NewFlushCount))
	}
	if r.OldFlushInterval != r.NewFlushInterval && r.NewFlushInterval != "" {
		parts = append(parts, "⏲"+r.NewFlushInterval)
	}
	return strings.Join(parts, " ")
}

// Message returns a short summary of the heal operation
func (h HealAuditDetails) Message() string {
	if h.Bucket != "" && h.Prefix != "" {
		return "Heal bucket '" + h.Bucket + "' prefix '" + h.Prefix + "'"
	}
	if h.Bucket != "" {
		return "Heal bucket '" + h.Bucket + "'"
	}
	if h.Operation != "" {
		return "Heal " + h.Operation
	}
	return "Heal operation performed"
}

// Details returns specific parameter changes for heal audit
func (h HealAuditDetails) Details() string {
	var parts []string
	if h.Bucket != "" {
		parts = append(parts, "♦"+h.Bucket)
	}
	if h.Prefix != "" {
		parts = append(parts, truncate(h.Prefix, 30))
	}
	return strings.Join(parts, " ")
}

// truncate truncates a string to a maximum length, adding "..." if truncated
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return "..."
	}
	return s[:maxLen-3] + "..."
}
