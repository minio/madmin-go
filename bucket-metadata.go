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
)

// ExportBucketMetadata makes an admin call to export bucket metadata of a bucket
func (adm *AdminClient) ExportBucketMetadata(ctx context.Context, bucket string) (io.ReadCloser, error) {
	path := adminAPIPrefixV3 + "/export-bucket-metadata"
	queryValues := url.Values{}
	queryValues.Set("bucket", bucket)

	resp, err := adm.executeMethod(ctx,
		http.MethodGet, requestData{
			relPath:     path,
			queryValues: queryValues,
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

// MetaStatus status of metadata import
type MetaStatus struct {
	IsSet bool   `json:"isSet"`
	Err   string `json:"error,omitempty"`
}

// BucketStatus reflects status of bucket metadata import
type BucketStatus struct {
	ObjectLock   MetaStatus `json:"olock"`
	Versioning   MetaStatus `json:"versioning"`
	Policy       MetaStatus `json:"policy"`
	Tagging      MetaStatus `json:"tagging"`
	SSEConfig    MetaStatus `json:"sse"`
	Lifecycle    MetaStatus `json:"lifecycle"`
	Notification MetaStatus `json:"notification"`
	Quota        MetaStatus `json:"quota"`
	Cors         MetaStatus `json:"cors"`
	Err          string     `json:"error,omitempty"`
}

// BucketMetaImportErrs reports on bucket metadata import status.
type BucketMetaImportErrs struct {
	Buckets map[string]BucketStatus `json:"buckets,omitempty"`
}

// ImportBucketMetadata makes an admin call to set bucket metadata of a bucket from imported content
func (adm *AdminClient) ImportBucketMetadata(ctx context.Context, bucket string, contentReader io.ReadCloser) (r BucketMetaImportErrs, err error) {
	content, err := io.ReadAll(contentReader)
	if err != nil {
		return r, err
	}

	path := adminAPIPrefixV3 + "/import-bucket-metadata"
	queryValues := url.Values{}
	queryValues.Set("bucket", bucket)

	resp, err := adm.executeMethod(ctx,
		http.MethodPut, requestData{
			relPath:     path,
			queryValues: queryValues,
			content:     content,
		},
	)
	defer closeResponse(resp)

	if err != nil {
		return r, err
	}

	if resp.StatusCode != http.StatusOK {
		return r, httpRespToErrorResponse(resp)
	}

	err = json.NewDecoder(resp.Body).Decode(&r)
	return r, err
}
