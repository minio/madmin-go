//
// Copyright (c) 2015-2022 MinIO, Inc.
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
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"
)

// tierAPI is API path prefix for tier related admin APIs
const tierAPI = "tier"

// AddTierIgnoreInUse adds a new remote tier, ignoring if it's being used by another MinIO deployment.
func (adm *AdminClient) AddTierIgnoreInUse(ctx context.Context, cfg *TierConfig) error {
	return adm.addTier(ctx, cfg, true)
}

// AddTier adds a new remote tier.
func (adm *AdminClient) addTier(ctx context.Context, cfg *TierConfig, ignoreInUse bool) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	encData, err := EncryptData(adm.getSecretKey(), data)
	if err != nil {
		return err
	}

	queryVals := url.Values{}
	queryVals.Set("force", strconv.FormatBool(ignoreInUse))
	reqData := requestData{
		relPath:     path.Join(adminAPIPrefix, tierAPI),
		content:     encData,
		queryValues: queryVals,
	}

	// Execute PUT on /minio/admin/v3/tier to add a remote tier
	resp, err := adm.executeMethod(ctx, http.MethodPut, reqData)
	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusNoContent {
		return httpRespToErrorResponse(resp)
	}
	return nil
}

// AddTier adds a new remote tier.
func (adm *AdminClient) AddTier(ctx context.Context, cfg *TierConfig) error {
	return adm.addTier(ctx, cfg, false)
}

// ListTiers returns a list of remote tiers configured.
func (adm *AdminClient) ListTiers(ctx context.Context) ([]*TierConfig, error) {
	reqData := requestData{
		relPath: path.Join(adminAPIPrefix, tierAPI),
	}

	// Execute GET on /minio/admin/v3/tier to list remote tiers configured.
	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	var tiers []*TierConfig
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return tiers, err
	}

	err = json.Unmarshal(b, &tiers)
	if err != nil {
		return tiers, err
	}

	return tiers, nil
}

// TierCreds is used to pass remote tier credentials in a tier-edit operation.
type TierCreds struct {
	AccessKey string `json:"access,omitempty"`
	SecretKey string `json:"secret,omitempty"`

	AWSRole                     bool   `json:"awsrole"`
	AWSRoleWebIdentityTokenFile string `json:"awsroleWebIdentity,omitempty"`
	AWSRoleARN                  string `json:"awsroleARN,omitempty"`

	AzSP ServicePrincipalAuth `json:"azSP,omitempty"`

	CredsJSON []byte `json:"creds,omitempty"`
}

// EditTier supports updating credentials for the remote tier identified by tierName.
func (adm *AdminClient) EditTier(ctx context.Context, tierName string, creds TierCreds) error {
	data, err := json.Marshal(creds)
	if err != nil {
		return err
	}

	var encData []byte
	encData, err = EncryptData(adm.getSecretKey(), data)
	if err != nil {
		return err
	}

	reqData := requestData{
		relPath: path.Join(adminAPIPrefix, tierAPI, tierName),
		content: encData,
	}

	// Execute POST on /minio/admin/v3/tier/tierName to edit a tier
	// configured.
	resp, err := adm.executeMethod(ctx, http.MethodPost, reqData)
	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusNoContent {
		return httpRespToErrorResponse(resp)
	}

	return nil
}

// RemoveTier removes an empty tier identified by tierName
func (adm *AdminClient) RemoveTier(ctx context.Context, tierName string) error {
	if tierName == "" {
		return ErrTierNameEmpty
	}
	reqData := requestData{
		relPath: path.Join(adminAPIPrefix, tierAPI, tierName),
	}

	// Execute DELETE on /minio/admin/v3/tier/tierName to remove an empty tier.
	resp, err := adm.executeMethod(ctx, http.MethodDelete, reqData)
	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusNoContent {
		return httpRespToErrorResponse(resp)
	}

	return nil
}

// VerifyTier verifies tierName's remote tier config
func (adm *AdminClient) VerifyTier(ctx context.Context, tierName string) error {
	if tierName == "" {
		return ErrTierNameEmpty
	}
	reqData := requestData{
		relPath: path.Join(adminAPIPrefix, tierAPI, tierName),
	}

	// Execute GET on /minio/admin/v3/tier/tierName to verify tierName's config.
	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusNoContent {
		return httpRespToErrorResponse(resp)
	}

	return nil
}

// TierInfo contains tier name, type and statistics
type TierInfo struct {
	Name       string
	Type       string
	Stats      TierStats
	DailyStats DailyTierStats
}

type DailyTierStats struct {
	Bins      [24]TierStats
	UpdatedAt time.Time
}

// TierStats returns per-tier stats of all configured tiers (incl. internal
// hot-tier)
func (adm *AdminClient) TierStats(ctx context.Context) ([]TierInfo, error) {
	reqData := requestData{
		relPath: path.Join(adminAPIPrefix, "tier-stats"),
	}

	// Execute GET on /minio/admin/v3/tier-stats to list tier-stats.
	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	var tierInfos []TierInfo
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return tierInfos, err
	}

	err = json.Unmarshal(b, &tierInfos)
	if err != nil {
		return tierInfos, err
	}

	return tierInfos, nil
}
