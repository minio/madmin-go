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

	"github.com/minio/minio-go/v7/pkg/throttle"
)

// BucketThrottle holds bucket throttle 
type BucketThrottle struct {
	ConcurrentRequestsCount uint64   `json:"concurrentRequestsCount"` // Indicates concurrent no of requets
	APIs                    []string `json:"apis"`                    // Indicates S3 APIs for which the above throttle to be applied
}

// IsValid returns false if throttle configuration is invalid
func (t BucketThrottle) IsValid() bool {
	if t.ConcurrentRequestsCount > 0 && len(t.APIs) > 0 {
		return true
	}
	// empty throttle configuration is invalid
	return false
}

// GetBucketThrottle - get throttle info for a bucket
func (adm *AdminClient) GetBucketThrottle(ctx context.Context, bucket string) (t throttle.Configuration, err error) {
	queryValues := url.Values{}
	queryValues.Set("bucket", bucket)

	reqData := requestData{
		relPath:     adminAPIPrefix + "/get-bucket-throttle",
		queryValues: queryValues,
	}

	// Execute GET on /minio/admin/v3/get-throttle
	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)

	defer closeResponse(resp)
	if err != nil {
		return t, err
	}

	if resp.StatusCode != http.StatusOK {
		return t, httpRespToErrorResponse(resp)
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return t, err
	}
	if err = json.Unmarshal(b, &t); err != nil {
		return t, err
	}

	return t, nil
}

// SetBucketThrottle - sets a bucket's throttle values
func (adm *AdminClient) SetBucketThrottle(ctx context.Context, bucket string, t throttle.Configuration) error {
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}

	queryValues := url.Values{}
	queryValues.Set("bucket", bucket)

	reqData := requestData{
		relPath:     adminAPIPrefix + "/set-bucket-throttle",
		queryValues: queryValues,
		content:     data,
	}

	// Execute PUT on /minio/admin/v3/set-bucket-throttle
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
