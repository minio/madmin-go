//
// Copyright (c) 2015-2026 MinIO, Inc.
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
	"io"
	"net/http"
	"strings"
)

// ObjectAccess is the level of object access an ObjectGrant gives.
type ObjectAccess string

// Object access levels.
const (
	ObjectAccessRead      ObjectAccess = "read"
	ObjectAccessWrite     ObjectAccess = "write"
	ObjectAccessReadWrite ObjectAccess = "readwrite"
)

// TableAccess is the level of table access a TableGrant gives.
type TableAccess string

// Table access levels.
const (
	TableAccessQuery      TableAccess = "query"
	TableAccessQueryWrite TableAccess = "query-write"
	TableAccessManage     TableAccess = "manage"
)

// ObjectGrant gives access to the objects in a bucket, or in one folder of it.
type ObjectGrant struct {
	Access ObjectAccess `json:"access"`
	// Bucket is an exact bucket name or a pattern ending in "*".
	Bucket string `json:"bucket"`
	// Prefix is the folder the grant covers, with its sub-folders. Empty
	// means the whole bucket.
	Prefix string `json:"prefix,omitempty"`
	// Browse also allows listing each parent folder of Prefix, which exposes
	// the names of its siblings.
	Browse bool `json:"browse,omitempty"`
}

// TableGrant gives access to the tables in a table bucket, a namespace or a
// single table.
type TableGrant struct {
	Access TableAccess `json:"access"`
	// Warehouse is an exact table bucket name or a pattern ending in "*".
	Warehouse string `json:"warehouse"`
	// Namespace is dot-joined for nested namespaces, such as "orders.eu", and
	// covers the namespace and every namespace nested under it.
	Namespace string `json:"namespace,omitempty"`
	Table     string `json:"table,omitempty"`
}

// BucketAdmin gives administrative rights on a bucket.
type BucketAdmin struct {
	Bucket       string `json:"bucket"`
	Settings     bool   `json:"settings,omitempty"`
	AccessPolicy bool   `json:"accessPolicy,omitempty"`
	ObjectLock   bool   `json:"objectLock,omitempty"`
	Delete       bool   `json:"delete,omitempty"`
}

// TableBucketAdmin gives administrative rights on a table bucket.
type TableBucketAdmin struct {
	Warehouse string `json:"warehouse"`
	Settings  bool   `json:"settings,omitempty"`
	Delete    bool   `json:"delete,omitempty"`
}

// PolicyGrants is the set of rules a simple policy is compiled from.
type PolicyGrants struct {
	Objects          []ObjectGrant      `json:"objects,omitempty"`
	Tables           []TableGrant       `json:"tables,omitempty"`
	BucketAdmin      []BucketAdmin      `json:"bucketAdmin,omitempty"`
	TableBucketAdmin []TableBucketAdmin `json:"tableBucketAdmin,omitempty"`
	// CreateBuckets lists the names or trailing-* patterns of buckets that
	// may be created.
	CreateBuckets []string `json:"createBuckets,omitempty"`
	// CreateTableBuckets lists the names or trailing-* patterns of table
	// buckets that may be created.
	CreateTableBuckets []string `json:"createTableBuckets,omitempty"`
}

// IsEmpty reports whether g has no rules.
func (g PolicyGrants) IsEmpty() bool {
	return len(g.Objects) == 0 &&
		len(g.Tables) == 0 &&
		len(g.BucketAdmin) == 0 &&
		len(g.TableBucketAdmin) == 0 &&
		len(g.CreateBuckets) == 0 &&
		len(g.CreateTableBuckets) == 0
}

// PolicySummaryKind is the area of access a PolicySummaryLine describes.
type PolicySummaryKind string

// Policy summary kinds.
const (
	PolicySummaryObject PolicySummaryKind = "object"
	PolicySummaryTable  PolicySummaryKind = "table"
	PolicySummaryAdmin  PolicySummaryKind = "admin"
)

// PolicySummaryLine is one plain-English line describing what a policy allows.
type PolicySummaryLine struct {
	Kind PolicySummaryKind `json:"kind"`
	Text string            `json:"text"`
}

// PolicyWarningCode identifies the kind of a PolicyWarning. Codes are stable.
type PolicyWarningCode string

// Policy warning codes.
const (
	PolicyWarningBucketNotVersioned  PolicyWarningCode = "BucketNotVersioned"
	PolicyWarningBucketNotFound      PolicyWarningCode = "BucketNotFound"
	PolicyWarningTableBucketNotFound PolicyWarningCode = "TableBucketNotFound"
	PolicyWarningBucketPattern       PolicyWarningCode = "BucketPattern"
	PolicyWarningTableBucketPattern  PolicyWarningCode = "TableBucketPattern"
	PolicyWarningAnonymousAccess     PolicyWarningCode = "AnonymousAccessPolicy"
	PolicyWarningObjectLock          PolicyWarningCode = "ObjectLockDefaults"
	PolicyWarningDeleteBucket        PolicyWarningCode = "DeleteBucket"
	PolicyWarningDeleteTableBucket   PolicyWarningCode = "DeleteTableBucket"
	PolicyWarningBucketSettings      PolicyWarningCode = "BucketSettings"
)

// PolicyWarning flags a consequence of a rule set that the admin should know
// about before saving it.
type PolicyWarning struct {
	Code   PolicyWarningCode `json:"code"`
	Bucket string            `json:"bucket,omitempty"`
	// Matches lists, for a pattern warning, the names that match the pattern
	// today.
	Matches []string `json:"matches,omitempty"`
	Message string   `json:"message"`
}

// PolicyGrantsFieldError is one validation problem in a rule set, tied to a
// field path such as "objects[1].prefix".
type PolicyGrantsFieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// PolicyGrantsError lists the validation problems in a rule set.
type PolicyGrantsError struct {
	Errors []PolicyGrantsFieldError
}

// Error implements the error interface.
func (e *PolicyGrantsError) Error() string {
	var sb strings.Builder
	sb.WriteString("invalid policy rules")
	for i, fe := range e.Errors {
		if i == 0 {
			sb.WriteString(": ")
		} else {
			sb.WriteString("; ")
		}
		if fe.Field != "" {
			sb.WriteString(fe.Field)
			sb.WriteString(": ")
		}
		sb.WriteString(fe.Message)
	}
	return sb.String()
}

// PolicyGrantsCompileRequest is the body of a policy-grants/compile request.
type PolicyGrantsCompileRequest struct {
	Grants PolicyGrants `json:"grants"`
}

// PolicyGrantsCompileResult is the outcome of compiling a rule set. When the
// rule set is invalid, Errors is set and Policy, Summary and SuggestedName
// are empty.
type PolicyGrantsCompileResult struct {
	Policy        json.RawMessage          `json:"policy,omitempty"`
	Summary       []PolicySummaryLine      `json:"summary,omitempty"`
	Warnings      []PolicyWarning          `json:"warnings,omitempty"`
	SuggestedName string                   `json:"suggestedName,omitempty"`
	Errors        []PolicyGrantsFieldError `json:"errors,omitempty"`
}

// Err returns a *PolicyGrantsError listing r.Errors, or nil when the rule set
// compiled.
func (r PolicyGrantsCompileResult) Err() error {
	if len(r.Errors) == 0 {
		return nil
	}
	return &PolicyGrantsError{Errors: r.Errors}
}

// CompilePolicyGrants compiles a rule set into an IAM policy without storing
// anything. Validation problems in g are returned in the result with a nil
// error; check them with the result's Err method. Save the returned policy
// with AddCannedPolicy.
func (adm *AdminClient) CompilePolicyGrants(ctx context.Context, g PolicyGrants) (PolicyGrantsCompileResult, error) {
	body, err := json.Marshal(PolicyGrantsCompileRequest{Grants: g})
	if err != nil {
		return PolicyGrantsCompileResult{}, err
	}

	reqData := requestData{
		relPath: adminAPIPrefix + "/policy-grants/compile",
		content: body,
	}

	resp, err := adm.executeMethod(ctx, http.MethodPost, reqData)
	defer closeResponse(resp)
	if err != nil {
		return PolicyGrantsCompileResult{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return PolicyGrantsCompileResult{}, httpRespToErrorResponse(resp)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return PolicyGrantsCompileResult{}, err
	}

	var result PolicyGrantsCompileResult
	if err = json.Unmarshal(data, &result); err != nil {
		return PolicyGrantsCompileResult{}, err
	}
	return result, nil
}
