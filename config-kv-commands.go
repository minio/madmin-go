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
	"net/http"
	"net/url"
)

// DelConfigKV - delete key from server config.
func (adm *AdminClient) DelConfigKV(ctx context.Context, k string) (restart bool, err error) {
	econfigBytes, err := EncryptData(adm.getSecretKey(), []byte(k))
	if err != nil {
		return false, err
	}

	reqData := requestData{
		relPath: adminAPIPrefix + "/del-config-kv",
		content: econfigBytes,
	}

	// Execute DELETE on /minio/admin/v4/del-config-kv to delete config key.
	resp, err := adm.executeMethod(ctx, http.MethodDelete, reqData)

	defer closeResponse(resp)
	if err != nil {
		return false, err
	}

	if resp.StatusCode != http.StatusOK {
		return false, httpRespToErrorResponse(resp)
	}

	return resp.Header.Get(ConfigAppliedHeader) != ConfigAppliedTrue, nil
}

const (
	// ConfigAppliedHeader is the header indicating whether the config was applied without requiring a restart.
	ConfigAppliedHeader = "x-minio-config-applied"

	// ConfigAppliedTrue is the value set in header if the config was applied.
	ConfigAppliedTrue = "true"
)

// SetConfigKV - set key value config to server.
func (adm *AdminClient) SetConfigKV(ctx context.Context, kv string) (restart bool, err error) {
	econfigBytes, err := EncryptData(adm.getSecretKey(), []byte(kv))
	if err != nil {
		return false, err
	}

	reqData := requestData{
		relPath: adminAPIPrefix + "/set-config-kv",
		content: econfigBytes,
	}

	// Execute PUT on /minio/admin/v4/set-config-kv to set config key/value.
	resp, err := adm.executeMethod(ctx, http.MethodPut, reqData)

	defer closeResponse(resp)
	if err != nil {
		return false, err
	}

	if resp.StatusCode != http.StatusOK {
		return false, httpRespToErrorResponse(resp)
	}

	return resp.Header.Get(ConfigAppliedHeader) != ConfigAppliedTrue, nil
}

// GetConfigKV - returns the key, value of the requested key, incoming data is encrypted.
func (adm *AdminClient) GetConfigKV(ctx context.Context, key string) ([]byte, error) {
	v := url.Values{}
	v.Set("key", key)

	// Execute GET on /minio/admin/v4/get-config-kv?key={key} to get value of key.
	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{
			relPath:     adminAPIPrefix + "/get-config-kv",
			queryValues: v,
		})
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	return DecryptData(adm.getSecretKey(), resp.Body)
}

// KVOptions takes specific inputs for KV functions
type KVOptions struct {
	Env bool
}

// GetConfigKVWithOptions - returns the key, value of the requested key, incoming data is encrypted.
func (adm *AdminClient) GetConfigKVWithOptions(ctx context.Context, key string, opts KVOptions) ([]byte, error) {
	v := url.Values{}
	v.Set("key", key)
	if opts.Env {
		v.Set("env", "")
	}

	// Execute GET on /minio/admin/v4/get-config-kv?key={key} to get value of key.
	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{
			relPath:     adminAPIPrefix + "/get-config-kv",
			queryValues: v,
		})
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	defer closeResponse(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	return DecryptData(adm.getSecretKey(), resp.Body)
}
