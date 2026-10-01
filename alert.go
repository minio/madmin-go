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
//go:generate go tool msgp -d clearomitted -d "timezone utc" $GOFILE

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
	// AlertTypeLicensedCapacity represents cluster usage approaching the
	// licensed (org-wide) storage capacity
	AlertTypeLicensedCapacity AlertType = "licensed-capacity"
	// AlertTypeBucketQuota represents a bucket's usage approaching its
	// configured hard quota
	AlertTypeBucketQuota AlertType = "bucket-quota"
	// AlertTypeScannerExcessFolders represents alerts for prefixes with excessive sub-folders
	AlertTypeScannerExcessFolders AlertType = "scanner-excess-folders"
	// AlertTypeScannerExcessVersions represents alerts for objects with excessive versions
	AlertTypeScannerExcessVersions AlertType = "scanner-excess-versions"
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

// AlertsInMemoryOnlyHeader is set to "true" on an alert-history response when
// no on-disk history could be read and only alerts still buffered in the
// server's memory are returned.
const AlertsInMemoryOnlyHeader = "x-minio-alerts-inmemory-only"

// AlertsResult is the outcome of ListAlerts.
//
//msgp:ignore AlertsResult
type AlertsResult struct {
	Alerts []Alert
	// InMemoryOnly reports a degraded read: Alerts may be incomplete rather
	// than the full history for the requested window.
	InMemoryOnly bool
}

func (adm AdminClient) openAlerts(ctx context.Context, opts AlertLogOpts) (*http.Response, error) {
	alertOpts, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}
	reqData := requestData{
		relPath: adminAPIPrefix + "/alerts",
		content: alertOpts,
	}
	resp, err := adm.executeMethod(ctx, http.MethodPost, reqData)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer closeResponse(resp)
		return nil, httpRespToErrorResponse(resp)
	}
	return resp, nil
}

// GetAlerts returns alerts stored in the system as a streaming msgpack response
// via POST /admin/alerts. Use AlertLogOpts.Interval to control the server-side
// check interval. Use ListAlerts to also learn whether the read was degraded.
func (adm AdminClient) GetAlerts(ctx context.Context, opts AlertLogOpts) iter.Seq2[*Alert, error] {
	return func(yield func(*Alert, error) bool) {
		resp, err := adm.openAlerts(ctx, opts)
		if err != nil {
			yield(nil, err)
			return
		}
		defer closeResponse(resp)
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

// ListAlerts reads alerts stored in the system via POST /admin/alerts and
// reports whether the server could only serve its in-memory buffer. On a
// decode error mid-stream it returns the alerts decoded so far with the error.
func (adm AdminClient) ListAlerts(ctx context.Context, opts AlertLogOpts) (AlertsResult, error) {
	resp, err := adm.openAlerts(ctx, opts)
	if err != nil {
		return AlertsResult{}, err
	}
	defer closeResponse(resp)

	result := AlertsResult{InMemoryOnly: resp.Header.Get(AlertsInMemoryOnlyHeader) == "true"}
	dec := msgp.NewReader(resp.Body)
	for {
		var alert Alert
		if err := alert.DecodeMsg(dec); err != nil {
			if errors.Is(err, io.EOF) {
				return result, nil
			}
			return result, err
		}
		result.Alerts = append(result.Alerts, alert)
	}
}
