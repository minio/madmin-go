//
// Copyright (c) 2015-2026 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.
//

package madmin

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
)

// CompressionMatch is a single key/value filter for object tags or metadata in
// a per-bucket compression rule.
type CompressionMatch struct {
	Key      string `json:"key"`
	Value    string `json:"value,omitempty"`
	Contains bool   `json:"contains,omitempty"`
}

// CompressionRule is a per-bucket compression match filter.
type CompressionRule struct {
	Prefixes   []string           `json:"prefixes,omitempty"`
	Extensions []string           `json:"extensions,omitempty"`
	MimeTypes  []string           `json:"mimeTypes,omitempty"`
	Tags       []CompressionMatch `json:"tags,omitempty"`
	Metadata   []CompressionMatch `json:"metadata,omitempty"`
	MinSize    int64              `json:"minSize,omitempty"`
	MaxSize    int64              `json:"maxSize,omitempty"`
	Exclude    bool               `json:"exclude,omitempty"`
}

// BucketCompressionConfig is the per-bucket compression policy: an ordered rule
// list evaluated first-match-wins. It wholly overrides the global compression
// settings for the bucket.
type BucketCompressionConfig struct {
	Enabled        bool              `json:"enabled"`
	AllowEncrypted bool              `json:"allowEncrypted,omitempty"`
	Rules          []CompressionRule `json:"rules,omitempty"`
}

// GetBucketCompression returns the per-bucket compression configuration, or nil
// when the bucket has none (global compression settings then apply).
func (adm *AdminClient) GetBucketCompression(ctx context.Context, bucket string) (*BucketCompressionConfig, error) {
	queryValues := url.Values{}
	queryValues.Set("bucket", bucket)

	resp, err := adm.executeMethod(ctx, http.MethodGet, requestData{
		relPath:     adminAPIPrefix + "/get-bucket-compression",
		queryValues: queryValues,
	})
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(b)) == 0 || string(bytes.TrimSpace(b)) == "null" {
		return nil, nil
	}
	cfg := &BucketCompressionConfig{}
	if err = json.Unmarshal(b, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// SetBucketCompression sets the per-bucket compression configuration.
func (adm *AdminClient) SetBucketCompression(ctx context.Context, bucket string, cfg *BucketCompressionConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	queryValues := url.Values{}
	queryValues.Set("bucket", bucket)

	resp, err := adm.executeMethod(ctx, http.MethodPut, requestData{
		relPath:     adminAPIPrefix + "/set-bucket-compression",
		queryValues: queryValues,
		content:     data,
	})
	defer closeResponse(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}
	return nil
}

// DeleteBucketCompression removes the per-bucket compression configuration,
// reverting the bucket to the global compression settings.
func (adm *AdminClient) DeleteBucketCompression(ctx context.Context, bucket string) error {
	queryValues := url.Values{}
	queryValues.Set("bucket", bucket)

	resp, err := adm.executeMethod(ctx, http.MethodDelete, requestData{
		relPath:     adminAPIPrefix + "/delete-bucket-compression",
		queryValues: queryValues,
	})
	defer closeResponse(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}
	return nil
}
