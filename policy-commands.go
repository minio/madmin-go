//
// Copyright (c) 2015-2024 MinIO, Inc.
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
	"net/url"
	"time"
)

// PolicyInfo contains information on a policy.
type PolicyInfo struct {
	PolicyName string
	Policy     json.RawMessage
	CreateDate time.Time `json:",omitempty"`
	UpdateDate time.Time `json:",omitempty"`
}

// MarshalJSON marshaller for JSON
func (pi PolicyInfo) MarshalJSON() ([]byte, error) {
	type aliasPolicyInfo PolicyInfo // needed to avoid recursive marshal
	if pi.CreateDate.IsZero() && pi.UpdateDate.IsZero() {
		return json.Marshal(&struct {
			PolicyName string
			Policy     json.RawMessage
		}{
			PolicyName: pi.PolicyName,
			Policy:     pi.Policy,
		})
	}
	return json.Marshal(aliasPolicyInfo(pi))
}

// InfoCannedPolicy - get info on a policy including timestamps and policy json.
func (adm *AdminClient) InfoCannedPolicy(ctx context.Context, policyName string) (*PolicyInfo, error) {
	queryValues := url.Values{}
	queryValues.Set("name", policyName)
	queryValues.Set("v", "2")

	reqData := requestData{
		relPath:     adminAPIPrefix + "/info-canned-policy",
		queryValues: queryValues,
	}

	// Execute GET on /minio/admin/v4/info-canned-policy
	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)

	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var p PolicyInfo
	err = json.Unmarshal(data, &p)
	return &p, err
}

// ListCannedPolicies - list all configured canned policies.
func (adm *AdminClient) ListCannedPolicies(ctx context.Context) (map[string]json.RawMessage, error) {
	reqData := requestData{
		relPath: adminAPIPrefix + "/list-canned-policies",
	}

	// Execute GET on /minio/admin/v4/list-canned-policies
	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)

	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	policies := make(map[string]json.RawMessage)
	if err = json.Unmarshal(respBytes, &policies); err != nil {
		return nil, err
	}

	return policies, nil
}

// RemoveCannedPolicy - remove a policy for a canned.
func (adm *AdminClient) RemoveCannedPolicy(ctx context.Context, policyName string) error {
	queryValues := url.Values{}
	queryValues.Set("name", policyName)

	reqData := requestData{
		relPath:     adminAPIPrefix + "/remove-canned-policy",
		queryValues: queryValues,
	}

	// Execute DELETE on /minio/admin/v4/remove-canned-policy to remove policy.
	resp, err := adm.executeMethod(ctx, http.MethodDelete, reqData)

	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}

	return nil
}

// AddCannedPolicy - adds a policy for a canned.
func (adm *AdminClient) AddCannedPolicy(ctx context.Context, policyName string, policy []byte) error {
	if len(policy) == 0 {
		return ErrInvalidArgument("policy input cannot be empty")
	}

	queryValues := url.Values{}
	queryValues.Set("name", policyName)

	reqData := requestData{
		relPath:     adminAPIPrefix + "/add-canned-policy",
		queryValues: queryValues,
		content:     policy,
	}

	// Execute PUT on /minio/admin/v4/add-canned-policy to set policy.
	resp, err := adm.executeMethod(ctx, http.MethodPut, reqData)

	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}

	return nil
}

func (adm *AdminClient) attachOrDetachPolicyBuiltin(ctx context.Context, isAttach bool,
	r PolicyAssociationReq,
) (PolicyAssociationResp, error) {
	err := r.IsValid()
	if err != nil {
		return PolicyAssociationResp{}, err
	}

	plainBytes, err := json.Marshal(r)
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
		relPath:       adminAPIPrefix + "/idp/builtin/policy/" + suffix,
		content:       encBytes,
	}

	resp, err := adm.executeMethod(ctx, http.MethodPost, reqData)
	defer closeResponse(resp)
	if err != nil {
		return PolicyAssociationResp{}, err
	}

	// Older minio does not send a response, so we handle that case.

	switch resp.StatusCode {
	case http.StatusOK:
		// Newer/current minio sends a result.
		content, err := DecryptData(adm.getSecretKey(), resp.Body)
		if err != nil {
			return PolicyAssociationResp{}, err
		}

		rsp := PolicyAssociationResp{}
		err = json.Unmarshal(content, &rsp)
		return rsp, err

	case http.StatusCreated, http.StatusNoContent:
		// Older minio - no result sent. TODO(aditya): Remove this case after
		// newer minio is released.
		return PolicyAssociationResp{}, nil

	default:
		// Error response case.
		return PolicyAssociationResp{}, httpRespToErrorResponse(resp)
	}
}

// AttachPolicy - attach policies to a user or group.
func (adm *AdminClient) AttachPolicy(ctx context.Context, r PolicyAssociationReq) (PolicyAssociationResp, error) {
	return adm.attachOrDetachPolicyBuiltin(ctx, true, r)
}

// DetachPolicy - detach policies from a user or group.
func (adm *AdminClient) DetachPolicy(ctx context.Context, r PolicyAssociationReq) (PolicyAssociationResp, error) {
	return adm.attachOrDetachPolicyBuiltin(ctx, false, r)
}

// GetPolicyEntities - returns builtin policy entities.
func (adm *AdminClient) GetPolicyEntities(ctx context.Context, q PolicyEntitiesQuery) (r PolicyEntitiesResult, err error) {
	params := make(url.Values)
	params["user"] = q.Users
	params["group"] = q.Groups
	params["policy"] = q.Policy

	reqData := requestData{
		relPath:     adminAPIPrefix + "/idp/builtin/policy-entities",
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

type AddAzureCannedPolicyReq struct {
	Name       string
	ConfigName string
	Policy     []byte
}

func (r *AddAzureCannedPolicyReq) Validate() error {
	if r.Name == "" {
		return ErrInvalidArgument("Group name is required")
	}
	if len(r.Policy) == 0 {
		return ErrInvalidArgument("Policy is required")
	}
	if r.ConfigName == "" {
		r.ConfigName = Default
	}
	return nil
}

type RemoveAzureCannedPolicyReq struct {
	Name       string
	ConfigName string
}

func (r *RemoveAzureCannedPolicyReq) Validate() error {
	if r.Name == "" {
		return ErrInvalidArgument("Group name is required")
	}
	if r.ConfigName == "" {
		r.ConfigName = Default
	}
	return nil
}

type ListAzureCannedPoliciesReq struct {
	ConfigName  string
	GetAllUUIDs bool // whether to also get policies that exist in MinIO but not in Azure
}

func (r *ListAzureCannedPoliciesReq) Validate() error {
	if r.ConfigName == "" {
		r.ConfigName = Default
	}
	return nil
}

type ListAzureCannedPoliciesResp []InfoAzureCannedPolicyResp

type InfoAzureCannedPolicyReq struct {
	Name       string
	ConfigName string
}

func (r *InfoAzureCannedPolicyReq) Validate() error {
	if r.Name == "" {
		return ErrInvalidArgument("Group name is required")
	}
	if r.ConfigName == "" {
		r.ConfigName = Default
	}
	return nil
}

type InfoAzureCannedPolicyResp struct {
	GroupName string
	PI        PolicyInfo
}

// AddAzureCannedPolicy - adds a policy corresponding to the Azure group ID for the given
// Azure group name
func (adm *AdminClient) AddAzureCannedPolicy(ctx context.Context, r AddAzureCannedPolicyReq) error {
	if err := r.Validate(); err != nil {
		return err
	}

	queryValues := url.Values{}
	queryValues.Set("name", r.Name)
	queryValues.Set("configName", r.ConfigName)

	reqData := requestData{
		relPath:     adminAPIPrefix + "/idp/openid/add-azure-canned-policy",
		queryValues: queryValues,
		content:     r.Policy,
	}

	// Execute PUT on /minio/admin/v4/idp/openid/add-azure-canned-policy to set policy.
	resp, err := adm.executeMethod(ctx, http.MethodPut, reqData)

	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}

	return nil
}

// RemoveAzureCannedPolicy - removes a policy corresponding to the Azure group ID for the given
// Azure group name
func (adm *AdminClient) RemoveAzureCannedPolicy(ctx context.Context, r RemoveAzureCannedPolicyReq) error {
	if err := r.Validate(); err != nil {
		return err
	}

	queryValues := url.Values{}
	queryValues.Set("name", r.Name)
	queryValues.Set("configName", r.ConfigName)

	reqData := requestData{
		relPath:     adminAPIPrefix + "/idp/openid/remove-azure-canned-policy",
		queryValues: queryValues,
	}

	// Execute DELETE on /minio/admin/v4/idp/openid/remove-azure-canned-policy to remove policy.
	resp, err := adm.executeMethod(ctx, http.MethodDelete, reqData)

	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}

	return nil
}

// ListAzureCannedPolicies - lists all Azure canned policies for the given configuration
func (adm *AdminClient) ListAzureCannedPolicies(ctx context.Context, r ListAzureCannedPoliciesReq) (ListAzureCannedPoliciesResp, error) {
	if err := r.Validate(); err != nil {
		return ListAzureCannedPoliciesResp{}, err
	}

	queryValues := url.Values{}
	queryValues.Set("configName", r.ConfigName)
	if r.GetAllUUIDs {
		queryValues.Set("getAllUUIDs", "true")
	}

	reqData := requestData{
		relPath:     adminAPIPrefix + "/idp/openid/list-azure-canned-policies",
		queryValues: queryValues,
	}

	// Execute GET on /minio/admin/v4/idp/openid/list-azure-canned-policies
	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)

	defer closeResponse(resp)
	if err != nil {
		return ListAzureCannedPoliciesResp{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return ListAzureCannedPoliciesResp{}, httpRespToErrorResponse(resp)
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return ListAzureCannedPoliciesResp{}, err
	}

	var response ListAzureCannedPoliciesResp
	if err = json.Unmarshal(respBytes, &response); err != nil {
		return ListAzureCannedPoliciesResp{}, err
	}

	return response, nil
}

// InfoAzureCannedPolicy - gets info on an Azure canned policy including timestamps and policy json
func (adm *AdminClient) InfoAzureCannedPolicy(ctx context.Context, r InfoAzureCannedPolicyReq) (InfoAzureCannedPolicyResp, error) {
	if err := r.Validate(); err != nil {
		return InfoAzureCannedPolicyResp{}, err
	}

	queryValues := url.Values{}
	queryValues.Set("name", r.Name)
	queryValues.Set("configName", r.ConfigName)

	reqData := requestData{
		relPath:     adminAPIPrefix + "/idp/openid/info-azure-canned-policy",
		queryValues: queryValues,
	}

	// Execute GET on /minio/admin/v4/idp/openid/info-azure-canned-policy
	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)

	defer closeResponse(resp)
	if err != nil {
		return InfoAzureCannedPolicyResp{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return InfoAzureCannedPolicyResp{}, httpRespToErrorResponse(resp)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return InfoAzureCannedPolicyResp{}, err
	}

	var p InfoAzureCannedPolicyResp
	err = json.Unmarshal(data, &p)
	return p, err
}
