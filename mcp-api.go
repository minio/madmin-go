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
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MCPPermission is a bitmask of MCP token permissions.
// Each bit encodes a (feature, access) pair: bit = 1 << permID.
//
//	             read  write  delete
//	objects        0      1       2
//	admin          3      4       5
//	tables         6      7       8
//	buckets        9     10      11
type MCPPermission uint32

// Per-feature, per-access permissions.
const (
	MCPPermObjectsRead   MCPPermission = 1 << 0
	MCPPermObjectsWrite  MCPPermission = 1 << 1
	MCPPermObjectsDelete MCPPermission = 1 << 2
	MCPPermAdminRead     MCPPermission = 1 << 3
	MCPPermAdminWrite    MCPPermission = 1 << 4
	MCPPermAdminDelete   MCPPermission = 1 << 5
	MCPPermTablesRead    MCPPermission = 1 << 6
	MCPPermTablesWrite   MCPPermission = 1 << 7
	MCPPermTablesDelete  MCPPermission = 1 << 8
	MCPPermBucketsRead   MCPPermission = 1 << 9
	MCPPermBucketsWrite  MCPPermission = 1 << 10
	MCPPermBucketsDelete MCPPermission = 1 << 11
)

// Feature groups.
const (
	MCPPermObjects = MCPPermObjectsRead | MCPPermObjectsWrite | MCPPermObjectsDelete
	MCPPermAdmin   = MCPPermAdminRead | MCPPermAdminWrite | MCPPermAdminDelete
	MCPPermTables  = MCPPermTablesRead | MCPPermTablesWrite | MCPPermTablesDelete
	MCPPermBuckets = MCPPermBucketsRead | MCPPermBucketsWrite | MCPPermBucketsDelete
)

// Access-type groups.
const (
	MCPPermRead   = MCPPermObjectsRead | MCPPermAdminRead | MCPPermTablesRead | MCPPermBucketsRead
	MCPPermWrite  = MCPPermObjectsWrite | MCPPermAdminWrite | MCPPermTablesWrite | MCPPermBucketsWrite
	MCPPermDelete = MCPPermObjectsDelete | MCPPermAdminDelete | MCPPermTablesDelete | MCPPermBucketsDelete
)

// MCPPermAll includes all permissions.
const MCPPermAll = MCPPermObjects | MCPPermAdmin | MCPPermTables | MCPPermBuckets

// Has returns true if p includes all bits in perm.
func (p MCPPermission) Has(perm MCPPermission) bool {
	return p&perm == perm
}

type mcpFeature struct {
	name    string
	r, w, d MCPPermission
}

var mcpFeatures = [...]mcpFeature{
	{"objects", MCPPermObjectsRead, MCPPermObjectsWrite, MCPPermObjectsDelete},
	{"admin", MCPPermAdminRead, MCPPermAdminWrite, MCPPermAdminDelete},
	{"tables", MCPPermTablesRead, MCPPermTablesWrite, MCPPermTablesDelete},
	{"buckets", MCPPermBucketsRead, MCPPermBucketsWrite, MCPPermBucketsDelete},
}

// String returns a compact permission string, e.g. "objects:rw,admin:rwd".
func (p MCPPermission) String() string {
	if p == MCPPermAll {
		return "all"
	}
	var parts []string
	for _, f := range mcpFeatures {
		var s string
		if p&f.r != 0 {
			s += "r"
		}
		if p&f.w != 0 {
			s += "w"
		}
		if p&f.d != 0 {
			s += "d"
		}
		if s != "" {
			parts = append(parts, f.name+":"+s)
		}
	}
	return strings.Join(parts, ",")
}

// ParseMCPPermissions parses a comma-separated permission string.
// Each token is "feature:access" where access is any combo of r/w/d,
// e.g. "objects:rw,admin:r,tables:rwd" or just "all".
func ParseMCPPermissions(s string) (MCPPermission, error) {
	var p MCPPermission
	for _, part := range strings.Split(s, ",") {
		tok := strings.TrimSpace(strings.ToLower(part))
		if tok == "" {
			continue
		}
		if tok == "all" {
			return MCPPermAll, nil
		}
		feat, access, ok := strings.Cut(tok, ":")
		if !ok {
			return p, fmt.Errorf("invalid permission %q: expected feature:access", part)
		}
		f, err := mcpFeatureByName(feat)
		if err != nil {
			return p, err
		}
		for _, c := range access {
			switch c {
			case 'r':
				p |= f.r
			case 'w':
				p |= f.w
			case 'd':
				p |= f.d
			default:
				return p, fmt.Errorf("invalid access flag %q in %q", string(c), part)
			}
		}
	}
	return p, nil
}

func mcpFeatureByName(name string) (mcpFeature, error) {
	for _, f := range mcpFeatures {
		if f.name == name {
			return f, nil
		}
	}
	return mcpFeature{}, fmt.Errorf("unknown feature %q", name)
}

// CreateMCPTokenRequest is the request body for creating an MCP token.
type CreateMCPTokenRequest struct {
	// AccessKey is the MinIO access key to associate the token with.
	AccessKey string `json:"accessKey,omitempty"`
	// Description is an optional human-readable label for the token.
	Description string `json:"description,omitempty"`
	// Permissions is the bitmask of granted permissions.
	Permissions MCPPermission `json:"permissions"`
	// Expiry is the duration string (e.g. "24h") after which the token expires.
	Expiry string `json:"expiry,omitempty"`
}

// CreateMCPTokenResponse is the response from creating an MCP token.
type CreateMCPTokenResponse struct {
	// TokenID is the unique identifier of the created token.
	TokenID string `json:"tokenId"`
	// Token is the bearer token value, only returned at creation time.
	Token string `json:"token"`
	// Permissions is the bitmask of granted permissions.
	Permissions MCPPermission `json:"permissions"`
	// ExpiresAt is the expiration time, if set.
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	// CreatedAt is when the token was created.
	CreatedAt time.Time `json:"createdAt"`
}

// MCPTokenInfo describes an existing MCP token.
type MCPTokenInfo struct {
	// TokenID is the unique identifier of the token.
	TokenID string `json:"tokenId"`
	// AccessKey is the MinIO access key associated with this token.
	AccessKey string `json:"accessKey"`
	// Description is an optional human-readable label.
	Description string `json:"description,omitempty"`
	// Permissions is the bitmask of granted permissions.
	Permissions MCPPermission `json:"permissions"`
	// CreatedAt is when the token was created.
	CreatedAt time.Time `json:"createdAt"`
	// ExpiresAt is the expiration time, if set.
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	// LastUsed is when the token was last used, if ever.
	LastUsed *time.Time `json:"lastUsed,omitempty"`
	// Revoked indicates whether the token has been revoked.
	Revoked bool `json:"revoked,omitempty"`
}

// IsExpired returns true if the token has a set expiry that is in the past.
func (t *MCPTokenInfo) IsExpired() bool {
	return t.ExpiresAt != nil && t.ExpiresAt.Before(time.Now())
}

// ListMCPTokensResponse is the response from listing MCP tokens.
type ListMCPTokensResponse struct {
	// Tokens is the list of MCP tokens.
	Tokens []MCPTokenInfo `json:"tokens"`
}

// CreateMCPToken creates a new MCP access token.
func (adm *AdminClient) CreateMCPToken(ctx context.Context, req CreateMCPTokenRequest) (*CreateMCPTokenResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	reqData := requestData{
		relPath: adminAPIPrefix + "/mcp/tokens",
		content: data,
	}

	resp, err := adm.executeMethod(ctx, http.MethodPost, reqData)
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	var result CreateMCPTokenResponse
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ListMCPTokens returns all MCP tokens.
func (adm *AdminClient) ListMCPTokens(ctx context.Context) (*ListMCPTokensResponse, error) {
	reqData := requestData{
		relPath: adminAPIPrefix + "/mcp/tokens",
	}

	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	var result ListMCPTokensResponse
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetMCPToken retrieves a single MCP token by its ID.
func (adm *AdminClient) GetMCPToken(ctx context.Context, tokenID string) (*MCPTokenInfo, error) {
	reqData := requestData{
		relPath: adminAPIPrefix + "/mcp/tokens/" + url.PathEscape(tokenID),
	}

	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	var result MCPTokenInfo
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// DeleteMCPToken revokes and deletes an MCP token by its ID.
func (adm *AdminClient) DeleteMCPToken(ctx context.Context, tokenID string) error {
	reqData := requestData{
		relPath: adminAPIPrefix + "/mcp/tokens/" + url.PathEscape(tokenID),
	}

	resp, err := adm.executeMethod(ctx, http.MethodDelete, reqData)
	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}

	return nil
}
