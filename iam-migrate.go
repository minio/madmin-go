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
	"io"
	"net/http"
)

type ImportIAMResult struct {
	Skipped IAMEntities    `json:"skipped,omitempty"`
	Removed IAMEntities    `json:"removed,omitempty"`
	Added   IAMEntities    `json:"added,omitmepty"`
	Failed  IAMErrEntities `json:"failed,omitmpty"`
}

type IAMEntities struct {
	Policies        []string              `json:"policies,omitmepty"`
	Users           []string              `json:"users,omitmepty"`
	Groups          []string              `json:"groups,omitempty"`
	ServiceAccounts []string              `json:"serviceAccounts,omitempty"`
	UserPolicies    []map[string][]string `json:"userPolicies,omitempty"`
	GroupPolicies   []map[string][]string `json:"groupPolicies,omitempty"`
	STSPolicies     []map[string][]string `json:"stsPolicies,omitempty"`
}

type IAMErrEntities struct {
	Policies        []ErrEntity    `json:"policies,omitempty"`
	Users           []ErrEntity    `json:"users,omitempty"`
	Groups          []ErrEntity    `json:"groups,omitempty"`
	ServiceAccounts []ErrEntity    `json:"serviceAccounts,omitempty"`
	UserPolicies    []ErrPolEntity `json:"userPolicies,omitempty"`
	GroupPolicies   []ErrPolEntity `json:"groupPolicies,omitempty"`
	STSPolicies     []ErrPolEntity `json:"stsPolicies,omitempty"`
}

type ErrEntity struct {
	Name  string `json:"name,omitempty"`
	Error error  `json:"error,omitempty"`
}

type ErrPolEntity struct {
	PolicyMap map[string][]string `json:"policyMap,omitempty"`
	Error     error               `json:"error,omitempty"`
}

// ExportIAM makes an admin call to export IAM data
func (adm *AdminClient) ExportIAM(ctx context.Context) (io.ReadCloser, error) {
	path := adminAPIPrefix + "/export-iam"

	resp, err := adm.executeMethod(ctx,
		http.MethodGet, requestData{
			relPath: path,
		},
	)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		closeResponse(resp)
		return nil, httpRespToErrorResponse(resp)
	}
	return resp.Body, nil
}

// ImportIAM makes an admin call to setup IAM  from imported content
func (adm *AdminClient) ImportIAM(ctx context.Context, contentReader io.ReadCloser) error {
	content, err := io.ReadAll(contentReader)
	if err != nil {
		return err
	}

	path := adminAPIPrefix + "/import-iam"
	resp, err := adm.executeMethod(ctx,
		http.MethodPut, requestData{
			relPath: path,
			content: content,
		},
	)
	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}

	return nil
}

// ImportIAMV2 makes an admin call to setup IAM  from imported content
func (adm *AdminClient) ImportIAMV2(ctx context.Context, contentReader io.ReadCloser) (iamr ImportIAMResult, err error) {
	content, err := io.ReadAll(contentReader)
	if err != nil {
		return iamr, err
	}

	path := adminAPIPrefix + "/import-iam-v2"
	resp, err := adm.executeMethod(ctx,
		http.MethodPut, requestData{
			relPath: path,
			content: content,
		},
	)
	defer closeResponse(resp)
	if err != nil {
		return iamr, err
	}

	if resp.StatusCode != http.StatusOK {
		return iamr, httpRespToErrorResponse(resp)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return iamr, err
	}

	if err = json.Unmarshal(b, &iamr); err != nil {
		return iamr, err
	}

	return iamr, nil
}
