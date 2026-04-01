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

package madmin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"iter"
	"net/http"
	"time"

	"github.com/tinylib/msgp/msgp"
)

//msgp:tag json
//go:generate msgp -d clearomitted -d "timezone utc" $GOFILE

// AlertType represents the type of alert event
//
//msgp:shim AlertType as:string
type AlertType string

const (
	// AlertTypeLicense represents license expiration alerts
	AlertTypeLicense AlertType = "license-expiry"
	// AlertTypeCertificate represents TLS certificate expiration alerts
	AlertTypeCertificate AlertType = "certificate-expiry"
	// AlertTypeConfigMismatch represents server configuration mismatch alerts
	AlertTypeConfigMismatch AlertType = "config-mismatch"
	// AlertTypeErasureSetHealth represents erasure set write-redundancy degradation alerts
	AlertTypeErasureSetHealth AlertType = "erasure-set-health"
	// AlertTypeKMSUnavailable represents KMS connectivity failure alerts
	AlertTypeKMSUnavailable AlertType = "kms-unavailable"
	// AlertTypeStorageCapacity represents storage capacity critical alerts
	AlertTypeStorageCapacity AlertType = "storage-capacity"
)

// Alert represents a single alert event in the system.
// It captures alert information with contextual metadata including deployment and cluster information.
type Alert struct {
	ID           string            `json:"id,omitempty"`
	Type         AlertType         `json:"type"`
	Timestamp    time.Time         `json:"timestamp"`
	Title        string            `json:"title"`
	Message      string            `json:"message"`
	Details      map[string]string `json:"details,omitempty"`
	DeploymentID string            `json:"deploymentId"`
	ClusterName  string            `json:"clusterName"`
	DedupKey     string            `json:"dedupKey,omitempty"`
}

// AlertLogOpts represents options for querying alerts
type AlertLogOpts struct {
	Types      []string      `json:"types,omitempty"`
	Interval   time.Duration `json:"interval,omitempty"`
	MaxPerNode int           `json:"maxPerNode,omitempty"`
}

// GetAlerts returns alerts stored in the system as a streaming msgpack response
// via POST /admin/alerts. Use AlertLogOpts.Interval to control the server-side
// check interval.
func (adm AdminClient) GetAlerts(ctx context.Context, opts AlertLogOpts) iter.Seq2[*Alert, error] {
	return func(yield func(*Alert, error) bool) {
		alertOpts, err := json.Marshal(opts)
		if err != nil {
			yield(nil, err)
			return
		}
		reqData := requestData{
			relPath: adminAPIPrefix + "/alerts",
			content: alertOpts,
		}
		resp, err := adm.executeMethod(ctx, http.MethodPost, reqData)
		if err != nil {
			yield(nil, err)
			return
		}
		defer closeResponse(resp)
		if resp.StatusCode != http.StatusOK {
			yield(nil, httpRespToErrorResponse(resp))
			return
		}
		dec := msgp.NewReader(resp.Body)
		for {
			var alert Alert
			if err = alert.DecodeMsg(dec); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				if !yield(nil, err) {
					return
				}
				continue
			}
			select {
			case <-ctx.Done():
				return
			default:
				if !yield(&alert, nil) {
					return
				}
			}
		}
	}
}
