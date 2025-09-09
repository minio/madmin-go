//
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
	"encoding/json"
	"net/http"
	"time"
)

// LogRecorderConfig represents the request body for log recorder configuration
type LogRecorderConfig struct {
	API   *LogRecorderAPIConfig   `json:"api,omitempty"`
	Error *LogRecorderErrorConfig `json:"error,omitempty"`
	Audit *LogRecorderAuditConfig `json:"audit,omitempty"`
}

// LogRecorderAPIConfig represents configuration for API log type
type LogRecorderAPIConfig struct {
	Enable        *bool          `json:"enable,omitempty"`
	DriveLimit    *string        `json:"drive_limit,omitempty"` // Human-readable format like "1Gi", "500Mi"
	FlushCount    *int           `json:"flush_count,omitempty"`
	FlushInterval *time.Duration `json:"flush_interval,omitempty"`
}

// LogRecorderErrorConfig represents configuration for Error log type
type LogRecorderErrorConfig struct {
	Enable        *bool          `json:"enable,omitempty"`
	DriveLimit    *string        `json:"drive_limit,omitempty"` // Human-readable format like "1Gi", "500Mi"
	FlushCount    *int           `json:"flush_count,omitempty"`
	FlushInterval *time.Duration `json:"flush_interval,omitempty"`
}

// LogRecorderAuditConfig represents configuration for Audit log type
type LogRecorderAuditConfig struct {
	Enable        *bool          `json:"enable,omitempty"`
	FlushCount    *int           `json:"flush_count,omitempty"`
	FlushInterval *time.Duration `json:"flush_interval,omitempty"`
}

// LogRecorderStatus represents the response for log recorder configuration
type LogRecorderStatus struct {
	API   LogRecorderAPIStatus   `json:"api"`
	Error LogRecorderErrorStatus `json:"error"`
	Audit LogRecorderAuditStatus `json:"audit"`
}

// LogRecorderAPIStatus represents the status of API log type
type LogRecorderAPIStatus struct {
	Enabled       bool          `json:"enabled"`
	DriveLimit    string        `json:"drive_limit,omitempty"` // Human-readable format
	FlushCount    int           `json:"flush_count,omitempty"`
	FlushInterval time.Duration `json:"flush_interval,omitempty"`
}

// LogRecorderErrorStatus represents the status of Error log type
type LogRecorderErrorStatus struct {
	Enabled       bool          `json:"enabled"`
	DriveLimit    string        `json:"drive_limit,omitempty"` // Human-readable format
	FlushCount    int           `json:"flush_count,omitempty"`
	FlushInterval time.Duration `json:"flush_interval,omitempty"`
}

// LogRecorderAuditStatus represents the status of Audit log type
type LogRecorderAuditStatus struct {
	Enabled       bool          `json:"enabled"`
	FlushCount    int           `json:"flush_count,omitempty"`
	FlushInterval time.Duration `json:"flush_interval,omitempty"`
}

// GetLogConfig - returns the log recorder configuration of a MinIO setup
func (adm *AdminClient) GetLogConfig(ctx context.Context) (*LogRecorderStatus, error) {
	// Execute GET on /minio/admin/v3/log-config to get log configuration
	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{relPath: adminAPIPrefix + "/log-config"})
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	// Decrypt the response
	configData, err := DecryptData(adm.getSecretKey(), resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse the configuration
	var status LogRecorderStatus
	if err := json.Unmarshal(configData, &status); err != nil {
		return nil, err
	}

	return &status, nil
}

// SetLogConfig - set the log recorder configuration for the MinIO setup
func (adm *AdminClient) SetLogConfig(ctx context.Context, config *LogRecorderConfig) error {
	if config == nil {
		return ErrInvalidArgument("log configuration cannot be nil")
	}

	// Marshal the configuration
	configBytes, err := json.Marshal(config)
	if err != nil {
		return err
	}

	// Encrypt the configuration
	econfigBytes, err := EncryptData(adm.getSecretKey(), configBytes)
	if err != nil {
		return err
	}

	reqData := requestData{
		relPath: adminAPIPrefix + "/log-config",
		content: econfigBytes,
	}

	// Execute PUT on /minio/admin/v3/log-config to set log configuration
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

// ResetLogConfig - reset the log recorder configuration to defaults
func (adm *AdminClient) ResetLogConfig(ctx context.Context) error {
	reqData := requestData{
		relPath: adminAPIPrefix + "/log-config",
	}

	// Execute DELETE on /minio/admin/v3/log-config to reset configuration
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
