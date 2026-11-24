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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// sharingOAuthType is the Type of a version 2 profile.
const sharingOAuthType = "oauth_client_credentials"

// SharedShare is a share a recipient can read.
type SharedShare struct {
	Name        string `json:"name"`
	ID          string `json:"id,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

// SharedSchema is a schema in a share.
type SharedSchema struct {
	Name  string `json:"name"`
	Share string `json:"share"`
}

// SharedTable is a table in a share.
type SharedTable struct {
	Name    string `json:"name"`
	Schema  string `json:"schema"`
	Share   string `json:"share"`
	ShareID string `json:"shareId,omitempty"`
	ID      string `json:"id,omitempty"`
}

// SharedVolume is a volume in a share: a folder of files of any format that
// the recipient reads with credentials from GetVolumeCredentials.
type SharedVolume struct {
	Name            string `json:"name"`
	Schema          string `json:"schema"`
	Share           string `json:"share"`
	ShareID         string `json:"shareId,omitempty"`
	ID              string `json:"id,omitempty"`
	StorageLocation string `json:"storageLocation"`
}

// SharedPage is a page of a recipient listing.
type SharedPage[T any] struct {
	Items         []T    `json:"items"`
	NextPageToken string `json:"nextPageToken,omitempty"`
}

// SharingAwsCredentials are temporary S3 credentials. Endpoint, Region and
// PathStyleAccess are an AIStor extension that says where the storage is.
type SharingAwsCredentials struct {
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	SessionToken    string `json:"sessionToken"`
	Endpoint        string `json:"endpoint,omitempty"`
	Region          string `json:"region,omitempty"`
	PathStyleAccess bool   `json:"pathStyleAccess,omitempty"`
}

// SharingTemporaryCredentials are short-lived, read-only credentials for one
// volume. ExpirationTime is in epoch milliseconds.
type SharingTemporaryCredentials struct {
	AwsTempCredentials *SharingAwsCredentials `json:"awsTempCredentials,omitempty"`
	ExpirationTime     int64                  `json:"expirationTime"`
}

// Expiration returns when the credentials expire.
func (c SharingTemporaryCredentials) Expiration() time.Time {
	return time.UnixMilli(c.ExpirationTime)
}

// SharingClient reads shares as a recipient, with the bearer token or OAuth
// client credentials of a profile file.
type SharingClient struct {
	endpoint   *url.URL
	profile    DeltaSharingProfile
	httpClient *http.Client

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

// NewSharingClient returns a client for the server and credentials in
// profile. A nil httpClient uses http.DefaultClient.
func NewSharingClient(profile DeltaSharingProfile, httpClient *http.Client) (*SharingClient, error) {
	u, err := url.Parse(strings.TrimRight(profile.Endpoint, "/"))
	if err != nil {
		return nil, fmt.Errorf("invalid profile endpoint: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" || u.Host == "" {
		return nil, fmt.Errorf("invalid profile endpoint %q: want an http or https URL", profile.Endpoint)
	}
	switch {
	case profile.Type == sharingOAuthType:
		if profile.TokenEndpoint == "" || profile.ClientID == "" || profile.ClientSecret == "" {
			return nil, errors.New("profile has no tokenEndpoint, clientId or clientSecret")
		}
	case profile.BearerToken == "":
		return nil, errors.New("profile has no bearerToken")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &SharingClient{endpoint: u, profile: profile, httpClient: httpClient, accessToken: profile.BearerToken}, nil
}

// token returns the bearer token, fetching a new OAuth access token when the
// profile uses client credentials and the current one is about to expire.
func (c *SharingClient) token(ctx context.Context) (string, error) {
	if c.profile.Type != sharingOAuthType {
		return c.accessToken, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accessToken != "" && time.Until(c.tokenExpiry) > time.Minute {
		return c.accessToken, nil
	}
	form := url.Values{"grant_type": []string{"client_credentials"}}
	if c.profile.Scope != "" {
		form.Set("scope", c.profile.Scope)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.profile.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.profile.ClientID, c.profile.ClientSecret)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer closeResponse(resp)
	if resp.StatusCode != http.StatusOK {
		var body struct {
			Error       string `json:"error"`
			Description string `json:"error_description"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, errorBodyLimit)).Decode(&body)
		return "", ErrorResponse{Code: body.Error, Message: "OAuth token request failed: " + body.Description, StatusCode: resp.StatusCode}
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", fmt.Errorf("invalid OAuth token response: %w", err)
	}
	if tok.AccessToken == "" {
		return "", errors.New("OAuth token response has no access_token")
	}
	c.accessToken = tok.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	return c.accessToken, nil
}

// do sends a request to the Delta Sharing API and decodes a 200 response into
// out.
func (c *SharingClient) do(ctx context.Context, method string, segments []string, query url.Values, out any) error {
	token, err := c.token(ctx)
	if err != nil {
		return err
	}
	escaped := make([]string, len(segments))
	for i, s := range segments {
		escaped[i] = url.PathEscape(s)
	}
	u := c.endpoint.JoinPath(escaped...)
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer closeResponse(resp)
	if resp.StatusCode != http.StatusOK {
		return sharingRespToError(resp)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func listShared[T any](ctx context.Context, c *SharingClient, opts SharingListOptions, segments ...string) (*SharedPage[T], error) {
	var page SharedPage[T]
	if err := c.do(ctx, http.MethodGet, segments, opts.values(), &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// ListShares lists a page of the shares the recipient can read.
func (c *SharingClient) ListShares(ctx context.Context, opts SharingListOptions) (*SharedPage[SharedShare], error) {
	return listShared[SharedShare](ctx, c, opts, "shares")
}

// ListSchemas lists a page of the schemas in a share.
func (c *SharingClient) ListSchemas(ctx context.Context, share string, opts SharingListOptions) (*SharedPage[SharedSchema], error) {
	return listShared[SharedSchema](ctx, c, opts, "shares", share, "schemas")
}

// ListAllTables lists a page of the tables in a share, across its schemas.
func (c *SharingClient) ListAllTables(ctx context.Context, share string, opts SharingListOptions) (*SharedPage[SharedTable], error) {
	return listShared[SharedTable](ctx, c, opts, "shares", share, "all-tables")
}

// ListTables lists a page of the tables in a schema.
func (c *SharingClient) ListTables(ctx context.Context, share, schema string, opts SharingListOptions) (*SharedPage[SharedTable], error) {
	return listShared[SharedTable](ctx, c, opts, "shares", share, "schemas", schema, "tables")
}

// ListAllVolumes lists a page of the volumes in a share, across its schemas.
func (c *SharingClient) ListAllVolumes(ctx context.Context, share string, opts SharingListOptions) (*SharedPage[SharedVolume], error) {
	return listShared[SharedVolume](ctx, c, opts, "shares", share, "all-volumes")
}

// ListVolumes lists a page of the volumes in a schema.
func (c *SharingClient) ListVolumes(ctx context.Context, share, schema string, opts SharingListOptions) (*SharedPage[SharedVolume], error) {
	return listShared[SharedVolume](ctx, c, opts, "shares", share, "schemas", schema, "volumes")
}

// GetVolume returns a volume.
func (c *SharingClient) GetVolume(ctx context.Context, share, schema, volume string) (*SharedVolume, error) {
	var v SharedVolume
	if err := c.do(ctx, http.MethodGet, []string{"shares", share, "schemas", schema, "volumes", volume}, nil, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// GetVolumeCredentials returns temporary credentials that read the files
// under a volume's StorageLocation.
func (c *SharingClient) GetVolumeCredentials(ctx context.Context, share, schema, volume string) (*SharingTemporaryCredentials, error) {
	var creds SharingTemporaryCredentials
	if err := c.do(ctx, http.MethodPost, []string{"shares", share, "schemas", schema, "volumes", volume, "temporary-volume-credentials"}, nil, &creds); err != nil {
		return nil, err
	}
	return &creds, nil
}

// ActivateSharingRecipient redeems a recipient activation link and returns
// the profile file it holds. A link works once. A nil httpClient uses
// http.DefaultClient.
func ActivateSharingRecipient(ctx context.Context, httpClient *http.Client, activationURL string) (*DeltaSharingProfile, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, activationURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer closeResponse(resp)
	if resp.StatusCode != http.StatusOK {
		return nil, sharingRespToError(resp)
	}
	var profile DeltaSharingProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, err
	}
	return &profile, nil
}
