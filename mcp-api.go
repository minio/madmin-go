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
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MCPPermission is a bitmask of MCP token permissions.
type MCPPermission uint8

const (
	// MCPPermRead grants read access to MCP resources.
	MCPPermRead MCPPermission = 1 << iota
	// MCPPermWrite grants write access to MCP resources.
	MCPPermWrite
	// MCPPermDelete grants delete access to MCP resources.
	MCPPermDelete
	// MCPPermAdmin grants administrative access to MCP resources.
	MCPPermAdmin
	// MCPPermTables grants access to table APIs.
	MCPPermTables

	// mcpPermLast should always be the last.
	mcpPermLast
)

// MCPPermAll includes all permissions.
const MCPPermAll  = mcpPermLast - 1

// Has returns true if p includes all bits in perm.
func (p MCPPermission) Has(perm MCPPermission) bool {
	return p&perm == perm
}

// String returns a comma-separated list of permission names.
func (p MCPPermission) String() string {
	var parts []string
	if p&MCPPermRead != 0 {
		parts = append(parts, "read")
	}
	if p&MCPPermWrite != 0 {
		parts = append(parts, "write")
	}
	if p&MCPPermDelete != 0 {
		parts = append(parts, "delete")
	}
	if p&MCPPermAdmin != 0 {
		parts = append(parts, "admin")
	}
	if p&MCPPermAdmin != 0 {
		parts = append(parts, "tables")
	}
	return strings.Join(parts, ",")
}

// ParseMCPPermissions parses a comma-separated permission string (e.g. "r,w" or "read,write").
func ParseMCPPermissions(s string) MCPPermission {
	var p MCPPermission
	for _, part := range strings.Split(s, ",") {
		switch strings.TrimSpace(strings.ToLower(part)) {
		case "r", "read":
			p |= MCPPermRead
		case "w", "write":
			p |= MCPPermWrite
		case "d", "delete":
			p |= MCPPermDelete
		case "a", "admin":
			p |= MCPPermAdmin
		case "t", "tables":
			p |= MCPPermTables
		case "all":
			p |= MCPPermAll
		}
	}
	return p
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
