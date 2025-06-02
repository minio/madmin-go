// Copyright (c) 2015-2025 MinIO, Inc.
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
	"io"
	"net/http"
	"net/url"

	"gopkg.in/yaml.v3"
)

// QOSConfigVersionCurrent is the current version of the QoS configuration.
const QOSConfigVersionCurrent = "v1"

// QOSConfig represents the QoS configuration with a list of rules.
type QOSConfig struct {
	Version string    `yaml:"version"`
	Rules   []QOSRule `yaml:"rules"`
}

type QOSRule struct {
	ID           string `yaml:"id"`
	Label        string `yaml:"label,omitempty"`
	Priority     int    `yaml:"priority"`
	BucketPrefix string `yaml:"bucketPrefix"`
	API          string `yaml:"api"`
	Rate         int64  `yaml:"rate"`
	Burst        int64  `yaml:"burst"` // not required for concurrency limit
	Limit        string `yaml:"limit"` // "concurrency" or "rps"
}

// NewQOSConfig creates a new empty QoS configuration.
func NewQOSConfig() *QOSConfig {
	return &QOSConfig{
		Version: "v1",
		Rules:   []QOSRule{},
	}
}

// GetQOSConfig retrieves the QoS configuration for the MinIO server.
func (adm *AdminClient) GetQOSConfig(ctx context.Context) (*QOSConfig, error) {
	var qosCfg QOSConfig

	queryValues := url.Values{}
	reqData := requestData{
		relPath:     adminAPIPrefix + "/get-qos-config",
		queryValues: queryValues,
	}

	// Execute GET on /minio/admin/v4/get-qos-config
	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
	if err != nil {
		return nil, err
	}
	defer closeResponse(resp)

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return NewQOSConfig(), nil
		}
		return nil, httpRespToErrorResponse(resp)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err = yaml.Unmarshal(b, &qosCfg); err != nil {
		return nil, err
	}

	return &qosCfg, nil
}

// SetQOSConfig sets the QoS configuration for the MinIO server.
func (adm *AdminClient) SetQOSConfig(ctx context.Context, qosCfg QOSConfig) error {
	data, err := yaml.Marshal(qosCfg)
	if err != nil {
		return err
	}

	queryValues := url.Values{}
	reqData := requestData{
		relPath:     adminAPIPrefix + "/set-qos-config",
		queryValues: queryValues,
		content:     data,
	}

	// Execute PUT on /minio/admin/v4/set-qos-config
	resp, err := adm.executeMethod(ctx, http.MethodPut, reqData)
	if err != nil {
		return err
	}
	defer closeResponse(resp)

	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}

	return nil
}
