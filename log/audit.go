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

// RecorderAuditDetails captures log recorder configuration details
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
	if s.ServiceName != "" && s.Operation != "" {
		return "Service '" + s.ServiceName + "' " + s.Operation
	}
	if s.Operation == "iam-import" && s.IAMImport != nil {
		return "IAM import completed"
	}
	if s.Operation != "" {
		return "Service " + s.Operation
	}
	return "Service operation performed"
}

// Details returns specific parameter changes for service audit
func (s ServiceAuditDetails) Details() string {
	var parts []string
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
func (l RecorderAuditDetails) Message() string {
	if l.LogType != "" {
		if l.OldEnabled != l.NewEnabled {
			if l.NewEnabled {
				return "Log recorder '" + l.LogType + "' enabled"
			}
			return "Log recorder '" + l.LogType + "' disabled"
		}
		return "Log recorder '" + l.LogType + "' configuration changed"
	}
	return "Log recorder configuration changed"
}

// Details returns specific parameter changes for log recorder audit
func (l RecorderAuditDetails) Details() string {
	var parts []string
	if l.OldLimit != l.NewLimit && l.NewLimit != "" {
		parts = append(parts, "↕"+l.NewLimit)
	}
	if l.OldFlushCount != l.NewFlushCount {
		parts = append(parts, fmt.Sprintf("⊕%d", l.NewFlushCount))
	}
	if l.OldFlushInterval != l.NewFlushInterval && l.NewFlushInterval != "" {
		parts = append(parts, "⏲"+l.NewFlushInterval)
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
