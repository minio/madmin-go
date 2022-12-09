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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7/pkg/set"
)

// AddOrUpdateIDPConfig - creates a new or updates an existing IDP
// configuration on the server.
func (adm *AdminClient) AddOrUpdateIDPConfig(ctx context.Context, cfgType, cfgName, cfgData string, update bool) (restart bool, err error) {
	encBytes, err := EncryptData(adm.getSecretKey(), []byte(cfgData))
	if err != nil {
		return false, err
	}

	method := http.MethodPut
	if update {
		method = http.MethodPost
	}

	if cfgName == "" {
		cfgName = Default
	}

	h := make(http.Header, 1)
	h.Add("Content-Type", "application/octet-stream")
	reqData := requestData{
		customHeaders: h,
		relPath:       strings.Join([]string{adminAPIPrefix, "idp-config", cfgType, cfgName}, "/"),
		content:       encBytes,
	}

	resp, err := adm.executeMethod(ctx, method, reqData)
	defer closeResponse(resp)
	if err != nil {
		return false, err
	}

	// FIXME: Remove support for this older API in 2023-04 (about 6 months).
	//
	// Attempt to fall back to older IDP API.
	if resp.StatusCode == http.StatusUpgradeRequired {
		// close old response
		closeResponse(resp)

		// Fallback is needed for `mc admin idp set myminio openid ...` only, as
		// this was the only released API supported in the older version.

		queryParams := make(url.Values, 2)
		queryParams.Set("type", cfgType)
		queryParams.Set("name", cfgName)
		reqData := requestData{
			customHeaders: h,
			relPath:       adminAPIPrefix + "/idp-config",
			queryValues:   queryParams,
			content:       encBytes,
		}
		resp, err = adm.executeMethod(ctx, http.MethodPut, reqData)
		defer closeResponse(resp)
		if err != nil {
			return false, err
		}
	}

	if resp.StatusCode != http.StatusOK {
		return false, httpRespToErrorResponse(resp)
	}

	return resp.Header.Get(ConfigAppliedHeader) != ConfigAppliedTrue, nil
}

// IDPCfgInfo represents a single configuration or related parameter
type IDPCfgInfo struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	IsCfg bool   `json:"isCfg"`
	IsEnv bool   `json:"isEnv"` // relevant only when isCfg=true
}

// IDPConfig contains IDP configuration information returned by server.
type IDPConfig struct {
	Type string       `json:"type"`
	Name string       `json:"name,omitempty"`
	Info []IDPCfgInfo `json:"info"`
}

// Constants for IDP configuration types.
const (
	OpenidIDPCfg string = "openid"
	LDAPIDPCfg   string = "ldap"
)

// ValidIDPConfigTypes - set of valid IDP configs.
var ValidIDPConfigTypes = set.CreateStringSet(OpenidIDPCfg, LDAPIDPCfg)

// GetIDPConfig - fetch IDP config from server.
func (adm *AdminClient) GetIDPConfig(ctx context.Context, cfgType, cfgName string) (c IDPConfig, err error) {
	if !ValidIDPConfigTypes.Contains(cfgType) {
		return c, fmt.Errorf("Invalid config type: %s", cfgType)
	}

	if cfgName == "" {
		cfgName = Default
	}

	reqData := requestData{
		relPath: strings.Join([]string{adminAPIPrefix, "idp-config", cfgType, cfgName}, "/"),
	}

	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
	defer closeResponse(resp)
	if err != nil {
		return c, err
	}

	// FIXME: Remove support for this older API in 2023-04 (about 6 months).
	//
	// Attempt to fall back to older IDP API.
	if resp.StatusCode == http.StatusUpgradeRequired {
		// close old response
		closeResponse(resp)

		queryParams := make(url.Values, 2)
		queryParams.Set("type", cfgType)
		queryParams.Set("name", cfgName)
		reqData := requestData{
			relPath:     adminAPIPrefix + "/idp-config",
			queryValues: queryParams,
		}
		resp, err = adm.executeMethod(ctx, http.MethodGet, reqData)
		defer closeResponse(resp)
		if err != nil {
			return c, err
		}
	}

	if resp.StatusCode != http.StatusOK {
		return c, httpRespToErrorResponse(resp)
	}

	content, err := DecryptData(adm.getSecretKey(), resp.Body)
	if err != nil {
		return c, err
	}

	err = json.Unmarshal(content, &c)
	return c, err
}

// IDPListItem - represents an item in the List IDPs call.
type IDPListItem struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	RoleARN string `json:"roleARN,omitempty"`
}

// ListIDPConfig - list IDP configuration on the server.
func (adm *AdminClient) ListIDPConfig(ctx context.Context, cfgType string) ([]IDPListItem, error) {
	if !ValidIDPConfigTypes.Contains(cfgType) {
		return nil, fmt.Errorf("Invalid config type: %s", cfgType)
	}

	reqData := requestData{
		relPath: strings.Join([]string{adminAPIPrefix, "idp-config", cfgType}, "/"),
	}

	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	// FIXME: Remove support for this older API in 2023-04 (about 6 months).
	//
	// Attempt to fall back to older IDP API.
	if resp.StatusCode == http.StatusUpgradeRequired {
		// close old response
		closeResponse(resp)

		queryParams := make(url.Values, 2)
		queryParams.Set("type", cfgType)
		reqData := requestData{
			relPath:     adminAPIPrefix + "/idp-config",
			queryValues: queryParams,
		}
		resp, err = adm.executeMethod(ctx, http.MethodGet, reqData)
		defer closeResponse(resp)
		if err != nil {
			return nil, err
		}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	content, err := DecryptData(adm.getSecretKey(), resp.Body)
	if err != nil {
		return nil, err
	}

	var lst []IDPListItem
	err = json.Unmarshal(content, &lst)
	return lst, err
}

// DeleteIDPConfig - delete an IDP configuration on the server.
func (adm *AdminClient) DeleteIDPConfig(ctx context.Context, cfgType, cfgName string) (restart bool, err error) {
	if cfgName == "" {
		cfgName = Default
	}
	reqData := requestData{
		relPath: strings.Join([]string{adminAPIPrefix, "idp-config", cfgType, cfgName}, "/"),
	}

	resp, err := adm.executeMethod(ctx, http.MethodDelete, reqData)
	defer closeResponse(resp)
	if err != nil {
		return false, err
	}

	// FIXME: Remove support for this older API in 2023-04 (about 6 months).
	//
	// Attempt to fall back to older IDP API.
	if resp.StatusCode == http.StatusUpgradeRequired {
		// close old response
		closeResponse(resp)

		queryParams := make(url.Values, 2)
		queryParams.Set("type", cfgType)
		queryParams.Set("name", cfgName)
		reqData := requestData{
			relPath:     adminAPIPrefix + "/idp-config",
			queryValues: queryParams,
		}
		resp, err = adm.executeMethod(ctx, http.MethodDelete, reqData)
		defer closeResponse(resp)
		if err != nil {
			return false, err
		}
	}

	if resp.StatusCode != http.StatusOK {
		return false, httpRespToErrorResponse(resp)
	}

	return resp.Header.Get(ConfigAppliedHeader) != ConfigAppliedTrue, nil
}

// PolicyEntitiesResult - contains response to a policy entities query.
type PolicyEntitiesResult struct {
	Timestamp      time.Time             `json:"timestamp"`
	UserMappings   []UserPolicyEntities  `json:"userMappings,omitempty"`
	GroupMappings  []GroupPolicyEntities `json:"groupMappings,omitempty"`
	PolicyMappings []PolicyEntities      `json:"policyMappings,omitempty"`
}

// UserPolicyEntities - user -> policies mapping
type UserPolicyEntities struct {
	User     string   `json:"user"`
	Policies []string `json:"policies"`
}

// GroupPolicyEntities - group -> policies mapping
type GroupPolicyEntities struct {
	Group    string   `json:"group"`
	Policies []string `json:"policies"`
}

// PolicyEntities - policy -> user+group mapping
type PolicyEntities struct {
	Policy string   `json:"policy"`
	Users  []string `json:"users"`
	Groups []string `json:"groups"`
}

// PolicyEntitiesQuery - contains request info for policy entities query.
type PolicyEntitiesQuery struct {
	Users  []string
	Groups []string
	Policy []string
}

// GetLDAPPolicyEntities - returns LDAP policy entities.
func (adm *AdminClient) GetLDAPPolicyEntities(ctx context.Context,
	q PolicyEntitiesQuery,
) (r PolicyEntitiesResult, err error) {
	params := make(url.Values)
	params["user"] = q.Users
	params["group"] = q.Groups
	params["policy"] = q.Policy

	reqData := requestData{
		relPath:     adminAPIPrefix + "/idp/ldap/policy-entities",
		queryValues: params,
	}

	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
	defer closeResponse(resp)
	if err != nil {
		return r, err
	}

	if resp.StatusCode != http.StatusOK {
		return r, httpRespToErrorResponse(resp)
	}

	content, err := DecryptData(adm.getSecretKey(), resp.Body)
	if err != nil {
		return r, err
	}

	err = json.Unmarshal(content, &r)
	return r, err
}

// PolicyAssociationResp - result of a policy association request.
type PolicyAssociationResp struct {
	PoliciesAttached []string `json:"policiesAttached,omitempty"`
	PoliciesDetached []string `json:"policiesDetached,omitempty"`

	UpdatedAt time.Time `json:"updatedAt"`
}

// PolicyAssociationReq - request to attach/detach policies from/to a user or
// group.
type PolicyAssociationReq struct {
	Policies []string `json:"policies"`

	// Exactly one of the following must be non-empty in a valid request.
	User  string `json:"user,omitempty"`
	Group string `json:"group,omitempty"`
}

// IsValid validates the object and returns a reason for when it is not.
func (p PolicyAssociationReq) IsValid() error {
	if len(p.Policies) == 0 {
		return errors.New("no policy names were given")
	}
	for _, p := range p.Policies {
		if p == "" {
			return errors.New("an empty policy name was given")
		}
	}

	if p.User == "" && p.Group == "" {
		return errors.New("no user or group association was given")
	}

	if p.User != "" && p.Group != "" {
		return errors.New("either a group or a user association must be given, not both")
	}

	return nil
}

// AttachPolicyLDAP - client call to attach policies for LDAP.
func (adm *AdminClient) AttachPolicyLDAP(ctx context.Context, par PolicyAssociationReq) (PolicyAssociationResp, error) {
	return adm.attachOrDetachPolicyLDAP(ctx, true, par)
}

// DetachPolicyLDAP - client call to detach policies for LDAP.
func (adm *AdminClient) DetachPolicyLDAP(ctx context.Context, par PolicyAssociationReq) (PolicyAssociationResp, error) {
	return adm.attachOrDetachPolicyLDAP(ctx, false, par)
}

func (adm *AdminClient) attachOrDetachPolicyLDAP(ctx context.Context, isAttach bool,
	par PolicyAssociationReq,
) (PolicyAssociationResp, error) {
	plainBytes, err := json.Marshal(par)
	if err != nil {
		return PolicyAssociationResp{}, err
	}

	encBytes, err := EncryptData(adm.getSecretKey(), plainBytes)
	if err != nil {
		return PolicyAssociationResp{}, err
	}

	suffix := "detach"
	if isAttach {
		suffix = "attach"
	}
	h := make(http.Header, 1)
	h.Add("Content-Type", "application/octet-stream")
	reqData := requestData{
		customHeaders: h,
		relPath:       adminAPIPrefix + "/idp/ldap/policy/" + suffix,
		content:       encBytes,
	}

	resp, err := adm.executeMethod(ctx, http.MethodPost, reqData)
	defer closeResponse(resp)
	if err != nil {
		return PolicyAssociationResp{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return PolicyAssociationResp{}, httpRespToErrorResponse(resp)
	}

	content, err := DecryptData(adm.getSecretKey(), resp.Body)
	if err != nil {
		return PolicyAssociationResp{}, err
	}

	r := PolicyAssociationResp{}
	err = json.Unmarshal(content, &r)
	return r, err
}
