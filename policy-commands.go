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
	"time"
)

// InfoCannedPolicy - expand canned policy into JSON structure.
//
// To be DEPRECATED in favor of the implementation in InfoCannedPolicyV2
func (adm *AdminClient) InfoCannedPolicy(ctx context.Context, policyName string) ([]byte, error) {
	queryValues := url.Values{}
	queryValues.Set("name", policyName)

	reqData := requestData{
		relPath:     adminAPIPrefix + "/info-canned-policy",
		queryValues: queryValues,
	}

	// Execute GET on /minio/admin/v3/info-canned-policy
	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)

	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	return ioutil.ReadAll(resp.Body)
}

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

// InfoCannedPolicyV2 - get info on a policy including timestamps and policy json.
func (adm *AdminClient) InfoCannedPolicyV2(ctx context.Context, policyName string) (*PolicyInfo, error) {
	queryValues := url.Values{}
	queryValues.Set("name", policyName)
	queryValues.Set("v", "2")

	reqData := requestData{
		relPath:     adminAPIPrefix + "/info-canned-policy",
		queryValues: queryValues,
	}

	// Execute GET on /minio/admin/v3/info-canned-policy
	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)

	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	data, err := ioutil.ReadAll(resp.Body)
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

	// Execute GET on /minio/admin/v3/list-canned-policies
	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)

	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	respBytes, err := ioutil.ReadAll(resp.Body)
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

	// Execute DELETE on /minio/admin/v3/remove-canned-policy to remove policy.
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
	if policy == nil {
		return ErrInvalidArgument("policy input cannot be empty")
	}

	queryValues := url.Values{}
	queryValues.Set("name", policyName)

	reqData := requestData{
		relPath:     adminAPIPrefix + "/add-canned-policy",
		queryValues: queryValues,
		content:     policy,
	}

	// Execute PUT on /minio/admin/v3/add-canned-policy to set policy.
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

// SetPolicy - sets the policy for a user or a group.
func (adm *AdminClient) SetPolicy(ctx context.Context, policyName, entityName string, isGroup bool) error {
	queryValues := url.Values{}
	queryValues.Set("policyName", policyName)
	queryValues.Set("userOrGroup", entityName)
	groupStr := "false"
	if isGroup {
		groupStr = "true"
	}
	queryValues.Set("isGroup", groupStr)

	reqData := requestData{
		relPath:     adminAPIPrefix + "/set-user-or-group-policy",
		queryValues: queryValues,
	}

	// Execute PUT on /minio/admin/v3/set-user-or-group-policy to set policy.
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

func (adm *AdminClient) attachOrDetachPolicyBuiltin(ctx context.Context, isGroup, isAttach bool, userOrGroup string, policies []string) error {
	par := PolicyAssociationReq{
		Policies: policies,
	}
	if isGroup {
		par.Group = userOrGroup
	} else {
		par.User = userOrGroup
	}
	err := par.IsValid()
	if err != nil {
		return err
	}

	plainBytes, err := json.Marshal(par)
	if err != nil {
		return err
	}

	encBytes, err := EncryptData(adm.getSecretKey(), plainBytes)
	if err != nil {
		return err
	}

	suffix := "detach"
	if isAttach {
		suffix = "attach"
	}

	reqData := requestData{
		relPath: adminAPIPrefix + "/idp/builtin/policy/" + suffix,
		content: encBytes,
	}

	resp, err := adm.executeMethod(ctx, http.MethodPost, reqData)
	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}

	return nil
}

// AttachPolicyUser - attach policies to a user or group.
func (adm *AdminClient) AttachPolicyUser(ctx context.Context, user string, policies []string) error {
	return adm.attachOrDetachPolicyBuiltin(ctx, false, true, user, policies)
}

// DetachPolicyUser - detach policies from a user or group.
func (adm *AdminClient) DetachPolicyUser(ctx context.Context, user string, policies []string) error {
	return adm.attachOrDetachPolicyBuiltin(ctx, false, false, user, policies)
}

// AttachPolicyGroup - attach policies to a user or group.
func (adm *AdminClient) AttachPolicyGroup(ctx context.Context, user string, policies []string) error {
	return adm.attachOrDetachPolicyBuiltin(ctx, true, true, user, policies)
}

// DetachPolicyGroup - detach policies from a user or group.
func (adm *AdminClient) DetachPolicyGroup(ctx context.Context, user string, policies []string) error {
	return adm.attachOrDetachPolicyBuiltin(ctx, true, false, user, policies)
}
