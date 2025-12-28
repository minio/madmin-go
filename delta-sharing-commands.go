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
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// DeltaSharingTable represents a Delta table or Iceberg table (via UniForm)
type DeltaSharingTable struct {
	Name       string `json:"name"`
	SourceType string `json:"sourceType"` // "delta" or "uniform"

	// For Delta tables
	Bucket   string `json:"bucket,omitempty"`
	Location string `json:"location,omitempty"`

	// For Iceberg tables (UniForm)
	Warehouse string `json:"warehouse,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Table     string `json:"table,omitempty"`
}

// DeltaSharingSchema represents a schema containing tables
type DeltaSharingSchema struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Tables      []DeltaSharingTable `json:"tables"`
}

// DeltaSharingShare represents a Delta Sharing share
type DeltaSharingShare struct {
	ID          string               `json:"id,omitempty"`
	Name        string               `json:"name"`
	Description string               `json:"description,omitempty"`
	Schemas     []DeltaSharingSchema `json:"schemas"`
	CreatedAt   time.Time            `json:"createdAt,omitempty"`
	UpdatedAt   time.Time            `json:"updatedAt,omitempty"`
}

// CreateShareRequest represents the request to create a new share
type CreateShareRequest struct {
	Name        string               `json:"name"`
	Description string               `json:"description,omitempty"`
	Schemas     []DeltaSharingSchema `json:"schemas"`
}

// CreateShareResponse represents the response from creating a share
type CreateShareResponse struct {
	Share DeltaSharingShare `json:"share"`
}

// ListSharesResponse represents the response from listing shares
type ListSharesResponse struct {
	Shares []DeltaSharingShare `json:"shares"`
}

// DeltaSharingToken represents an access token for a share
type DeltaSharingToken struct {
	TokenID     string               `json:"tokenId"`
	Token       string               `json:"token,omitempty"`
	Description string               `json:"description,omitempty"`
	ShareID     string               `json:"shareId,omitempty"`
	ShareName   string               `json:"shareName,omitempty"`
	CreatedAt   time.Time            `json:"createdAt,omitempty"`
	ExpiresAt   *time.Time           `json:"expiresAt,omitempty"`
	Profile     *DeltaSharingProfile `json:"profile,omitempty"`
}

// DeltaSharingProfile represents the Delta Sharing profile for clients
type DeltaSharingProfile struct {
	ShareCredentialsVersion int    `json:"shareCredentialsVersion"`
	Endpoint                string `json:"endpoint"`
	BearerToken             string `json:"bearerToken,omitempty"`

	// OAuth 2.0 fields (version 2)
	TokenEndpoint string `json:"tokenEndpoint,omitempty"`
	ClientID      string `json:"clientId,omitempty"`
	ClientSecret  string `json:"clientSecret,omitempty"`
	Scope         string `json:"scope,omitempty"`
}

// CreateTokenRequest represents the request to create a new token
type CreateTokenRequest struct {
	Description string     `json:"description,omitempty"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
}

// CreateTokenResponse represents the response from creating a token
type CreateTokenResponse struct {
	TokenID   string               `json:"tokenId"`
	Token     string               `json:"token"`
	ExpiresAt *time.Time           `json:"expiresAt,omitempty"`
	Profile   *DeltaSharingProfile `json:"profile"`
}

// ListTokensResponse represents the response from listing tokens
type ListTokensResponse struct {
	Tokens []DeltaSharingToken `json:"tokens"`
}

// CreateShare creates a new Delta Sharing share
func (adm *AdminClient) CreateShare(ctx context.Context, req CreateShareRequest) (*CreateShareResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	reqData := requestData{
		relPath: adminAPIPrefix + "/delta-sharing/shares",
		content: data,
	}

	resp, err := adm.executeMethod(ctx, http.MethodPost, reqData)
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, httpRespToErrorResponse(resp)
	}

	var result CreateShareResponse
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ListShares lists all Delta Sharing shares
func (adm *AdminClient) ListShares(ctx context.Context) (*ListSharesResponse, error) {
	reqData := requestData{
		relPath: adminAPIPrefix + "/delta-sharing/shares",
	}

	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	var result ListSharesResponse
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetShareResponse represents the response from getting a share
type GetShareResponse struct {
	Share DeltaSharingShare `json:"share"`
}

// GetShare retrieves details of a specific share
func (adm *AdminClient) GetShare(ctx context.Context, shareName string) (*DeltaSharingShare, error) {
	reqData := requestData{
		relPath: adminAPIPrefix + "/delta-sharing/shares/" + url.PathEscape(shareName),
	}

	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	var result GetShareResponse
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Share, nil
}

// UpdateShare updates a share's description or schemas
func (adm *AdminClient) UpdateShare(ctx context.Context, shareName string, description *string, schemas []DeltaSharingSchema) (*DeltaSharingShare, error) {
	req := make(map[string]interface{})
	if description != nil {
		req["description"] = *description
	}
	if schemas != nil {
		req["schemas"] = schemas
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	reqData := requestData{
		relPath: adminAPIPrefix + "/delta-sharing/shares/" + url.PathEscape(shareName),
		content: data,
	}

	resp, err := adm.executeMethod(ctx, http.MethodPut, reqData)
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	var result GetShareResponse
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Share, nil
}

// DeleteShare deletes a share and all its tokens
func (adm *AdminClient) DeleteShare(ctx context.Context, shareName string) error {
	reqData := requestData{
		relPath: adminAPIPrefix + "/delta-sharing/shares/" + url.PathEscape(shareName),
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

// CreateToken creates a new access token for a share
func (adm *AdminClient) CreateToken(ctx context.Context, shareName string, req CreateTokenRequest) (*CreateTokenResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	reqData := requestData{
		relPath: adminAPIPrefix + "/delta-sharing/shares/" + url.PathEscape(shareName) + "/tokens",
		content: data,
	}

	resp, err := adm.executeMethod(ctx, http.MethodPost, reqData)
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, httpRespToErrorResponse(resp)
	}

	var result CreateTokenResponse
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ListTokens lists all tokens for a share
func (adm *AdminClient) ListTokens(ctx context.Context, shareName string) (*ListTokensResponse, error) {
	reqData := requestData{
		relPath: adminAPIPrefix + "/delta-sharing/shares/" + url.PathEscape(shareName) + "/tokens",
	}

	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	var result ListTokensResponse
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// DeleteToken deletes a specific token
func (adm *AdminClient) DeleteToken(ctx context.Context, tokenID string) error {
	reqData := requestData{
		relPath: adminAPIPrefix + "/delta-sharing/tokens/" + url.PathEscape(tokenID),
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

// NewDeltaTable creates a Delta table configuration
func NewDeltaTable(name, bucket, location string) DeltaSharingTable {
	return DeltaSharingTable{
		Name:       name,
		SourceType: "delta",
		Bucket:     bucket,
		Location:   location,
	}
}

// NewUniformTable creates an Iceberg UniForm table configuration
func NewUniformTable(name, warehouse, namespace, table string) DeltaSharingTable {
	return DeltaSharingTable{
		Name:       name,
		SourceType: "uniform",
		Warehouse:  warehouse,
		Namespace:  namespace,
		Table:      table,
	}
}

// NewSchema creates a schema with tables
func NewSchema(name, description string, tables ...DeltaSharingTable) DeltaSharingSchema {
	return DeltaSharingSchema{
		Name:        name,
		Description: description,
		Tables:      tables,
	}
}

// DeltaSharingError represents an error response from the Delta Sharing API
type DeltaSharingError struct {
	ErrorCode string `json:"errorCode"`
	Message   string `json:"message"`
}

func (e DeltaSharingError) Error() string {
	return fmt.Sprintf("%s: %s", e.ErrorCode, e.Message)
}
