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
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestCreateShare(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/minio/admin/v4/delta-sharing/shares" {
			t.Errorf("Expected path /minio/admin/v4/delta-sharing/shares, got %s", r.URL.Path)
		}

		var req CreateShareRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		if req.Name != "test-share" {
			t.Errorf("Expected share name 'test-share', got %s", req.Name)
		}

		response := CreateShareResponse{
			Share: DeltaSharingShare{
				ID:          "share-123",
				Name:        req.Name,
				Description: req.Description,
				Schemas:     req.Schemas,
				CreatedAt:   time.Now(),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client
	client, err := New(server.URL[7:], "access", "secret", false)
	if err != nil {
		t.Fatal(err)
	}

	// Create share request
	req := CreateShareRequest{
		Name:        "test-share",
		Description: "Test share description",
		Schemas: []DeltaSharingSchema{
			{
				Name:        "default",
				Description: "Default schema",
				Tables: []DeltaSharingTable{
					NewDeltaTable("sales", "test-bucket", "sales/"),
				},
			},
		},
	}

	// Test create share
	resp, err := client.CreateShare(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateShare failed: %v", err)
	}

	if resp.Share.Name != "test-share" {
		t.Errorf("Expected share name 'test-share', got %s", resp.Share.Name)
	}

	if resp.Share.ID != "share-123" {
		t.Errorf("Expected share ID 'share-123', got %s", resp.Share.ID)
	}
}

func TestListShares(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if r.URL.Path != "/minio/admin/v4/delta-sharing/shares" {
			t.Errorf("Expected path /minio/admin/v4/delta-sharing/shares, got %s", r.URL.Path)
		}

		response := ListSharesResponse{
			Shares: []DeltaSharingShare{
				{
					ID:          "share-1",
					Name:        "share1",
					Description: "First share",
					CreatedAt:   time.Now(),
				},
				{
					ID:          "share-2",
					Name:        "share2",
					Description: "Second share",
					CreatedAt:   time.Now(),
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client
	client, err := New(server.URL[7:], "access", "secret", false)
	if err != nil {
		t.Fatal(err)
	}

	// Test list shares
	resp, err := client.ListShares(context.Background())
	if err != nil {
		t.Fatalf("ListShares failed: %v", err)
	}

	if len(resp.Shares) != 2 {
		t.Errorf("Expected 2 shares, got %d", len(resp.Shares))
	}

	if resp.Shares[0].Name != "share1" {
		t.Errorf("Expected first share name 'share1', got %s", resp.Shares[0].Name)
	}
}

func TestCreateToken(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/minio/admin/v4/delta-sharing/shares/test-share/tokens" {
			t.Errorf("Expected path /minio/admin/v4/delta-sharing/shares/test-share/tokens, got %s", r.URL.Path)
		}

		var req CreateTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		response := CreateTokenResponse{
			TokenID: "token-123",
			Token:   "dstkn_abcdefghijklmnopqrstuvwxyz",
			Profile: &DeltaSharingProfile{
				ShareCredentialsVersion: 1,
				Endpoint:                "https://minio.example.com/_delta-sharing",
				BearerToken:             "dstkn_abcdefghijklmnopqrstuvwxyz",
			},
		}

		if req.ExpiresAt != nil {
			response.ExpiresAt = req.ExpiresAt
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client
	client, err := New(server.URL[7:], "access", "secret", false)
	if err != nil {
		t.Fatal(err)
	}

	// Create token request
	expiresAt := time.Now().Add(24 * time.Hour)
	req := CreateTokenRequest{
		Description: "Test token",
		ExpiresAt:   &expiresAt,
	}

	// Test create token
	resp, err := client.CreateToken(context.Background(), "test-share", req)
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}

	if resp.TokenID != "token-123" {
		t.Errorf("Expected token ID 'token-123', got %s", resp.TokenID)
	}

	if resp.Profile == nil {
		t.Error("Expected profile to be non-nil")
	} else {
		if resp.Profile.ShareCredentialsVersion != 1 {
			t.Errorf("Expected share credentials version 1, got %d", resp.Profile.ShareCredentialsVersion)
		}
		if resp.Profile.BearerToken == "" {
			t.Error("Expected bearer token to be non-empty")
		}
	}
}

func TestDeleteShare(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("Expected DELETE request, got %s", r.Method)
		}
		if r.URL.Path != "/minio/admin/v4/delta-sharing/shares/test-share" {
			t.Errorf("Expected path /minio/admin/v4/delta-sharing/shares/test-share, got %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	// Create client
	client, err := New(server.URL[7:], "access", "secret", false)
	if err != nil {
		t.Fatal(err)
	}

	// Test delete share
	err = client.DeleteShare(context.Background(), "test-share")
	if err != nil {
		t.Fatalf("DeleteShare failed: %v", err)
	}
}

func TestHelperFunctions(t *testing.T) {
	// Test NewDeltaTable
	deltaTable := NewDeltaTable("sales", "bucket1", "path/to/sales/")
	if deltaTable.Name != "sales" {
		t.Errorf("Expected table name 'sales', got %s", deltaTable.Name)
	}
	if deltaTable.SourceType != "delta" {
		t.Errorf("Expected source type 'delta', got %s", deltaTable.SourceType)
	}
	if deltaTable.Bucket != "bucket1" {
		t.Errorf("Expected bucket 'bucket1', got %s", deltaTable.Bucket)
	}
	if deltaTable.Location != "path/to/sales/" {
		t.Errorf("Expected location 'path/to/sales/', got %s", deltaTable.Location)
	}

	// Test NewUniformTable
	uniformTable := NewUniformTable("inventory", "warehouse1", "retail", "inventory_table")
	if uniformTable.Name != "inventory" {
		t.Errorf("Expected table name 'inventory', got %s", uniformTable.Name)
	}
	if uniformTable.SourceType != "uniform" {
		t.Errorf("Expected source type 'uniform', got %s", uniformTable.SourceType)
	}
	if uniformTable.Warehouse != "warehouse1" {
		t.Errorf("Expected warehouse 'warehouse1', got %s", uniformTable.Warehouse)
	}
	if uniformTable.Namespace != "retail" {
		t.Errorf("Expected namespace 'retail', got %s", uniformTable.Namespace)
	}
	if uniformTable.Table != "inventory_table" {
		t.Errorf("Expected table 'inventory_table', got %s", uniformTable.Table)
	}

	// Test NewSchema
	schema := NewSchema("default", "Default schema", deltaTable, uniformTable)
	if schema.Name != "default" {
		t.Errorf("Expected schema name 'default', got %s", schema.Name)
	}
	if schema.Description != "Default schema" {
		t.Errorf("Expected schema description 'Default schema', got %s", schema.Description)
	}
	if len(schema.Tables) != 2 {
		t.Errorf("Expected 2 tables, got %d", len(schema.Tables))
	}
}

func TestDeltaSharingError(t *testing.T) {
	err := DeltaSharingError{
		ErrorCode: "SHARE_NOT_FOUND",
		Message:   "Share 'test' not found",
	}

	expected := "SHARE_NOT_FOUND: Share 'test' not found"
	if err.Error() != expected {
		t.Errorf("Expected error string '%s', got '%s'", expected, err.Error())
	}
}

func TestUpdateShare(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("Expected PUT request, got %s", r.Method)
		}
		if r.URL.Path != "/minio/admin/v4/delta-sharing/shares/test-share" {
			t.Errorf("Expected path /minio/admin/v4/delta-sharing/shares/test-share, got %s", r.URL.Path)
		}

		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		share := DeltaSharingShare{
			ID:          "share-123",
			Name:        "test-share",
			Description: "Updated description",
			UpdatedAt:   time.Now(),
		}

		if desc, ok := req["description"].(string); ok {
			share.Description = desc
		}

		response := GetShareResponse{
			Share: share,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client
	client, err := New(server.URL[7:], "access", "secret", false)
	if err != nil {
		t.Fatal(err)
	}

	// Test update share
	newDesc := "New description"
	resp, err := client.UpdateShare(context.Background(), "test-share", &newDesc, nil)
	if err != nil {
		t.Fatalf("UpdateShare failed: %v", err)
	}

	if resp.Description != newDesc {
		t.Errorf("Expected description '%s', got '%s'", newDesc, resp.Description)
	}
}

func TestProfileMarshaling(t *testing.T) {
	// Test v1 profile (Bearer token)
	v1Profile := DeltaSharingProfile{
		ShareCredentialsVersion: 1,
		Endpoint:                "https://minio.example.com/_delta-sharing",
		BearerToken:             "dstkn_abcdefg",
	}

	data, err := json.Marshal(v1Profile)
	if err != nil {
		t.Fatalf("Failed to marshal v1 profile: %v", err)
	}

	var decoded DeltaSharingProfile
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal v1 profile: %v", err)
	}

	if !reflect.DeepEqual(v1Profile, decoded) {
		t.Error("v1 profile marshaling/unmarshaling mismatch")
	}

	// Test v2 profile (OAuth)
	v2Profile := DeltaSharingProfile{
		ShareCredentialsVersion: 2,
		Endpoint:                "https://minio.example.com/_delta-sharing",
		TokenEndpoint:           "https://minio.example.com/oauth/token",
		ClientID:                "client-123",
		ClientSecret:            "secret-456",
		Scope:                   "delta-sharing:read",
	}

	data, err = json.Marshal(v2Profile)
	if err != nil {
		t.Fatalf("Failed to marshal v2 profile: %v", err)
	}

	var decodedV2 DeltaSharingProfile
	if err := json.Unmarshal(data, &decodedV2); err != nil {
		t.Fatalf("Failed to unmarshal v2 profile: %v", err)
	}

	if !reflect.DeepEqual(v2Profile, decodedV2) {
		t.Error("v2 profile marshaling/unmarshaling mismatch")
	}
}
