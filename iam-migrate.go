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
	Skipped []string `json:"skipped,omitempty"`
	Removed []string `json:"removed,omitempty"`
	Added   []string `json:"added,omitmepty"`
	Failed  []string `json:"failed,omitmpty"`
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
