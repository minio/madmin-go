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
)

// AlertType represents the type of alert event
type AlertType string

const (
	// AlertTypeLicense represents license expiration alerts
	AlertTypeLicense AlertType = "license-expiry"
	// AlertTypeCertificate represents TLS certificate expiration alerts
	AlertTypeCertificate AlertType = "certificate-expiry"
)

// Alert represents a single alert event in the system.
// It captures alert information with contextual metadata including deployment and cluster information.
type Alert struct {
	Type         AlertType    `json:"type"`
	Timestamp    time.Time    `json:"timestamp"`
	Title        string       `json:"title"`
	Message      string       `json:"message"`
	Details      AlertDetails `json:"details"`
	DeploymentID string       `json:"deploymentId"`
	ClusterName  string       `json:"clusterName"`
}

// AlertDetails is a strongly-typed union for alert-specific details.
// Only one field should be populated based on the alert type.
type AlertDetails struct {
	LicenseExpiry     *LicenseExpiryDetails     `json:"licenseExpiry,omitempty"`
	CertificateExpiry *CertificateExpiryDetails `json:"certificateExpiry,omitempty"`
}

// LicenseExpiryDetails contains license-specific alert information.
type LicenseExpiryDetails struct {
	LicenseID string    `json:"licenseId"`
	ExpiresAt time.Time `json:"expiresAt"`
	State     string    `json:"state"` // "expiring_soon", "grace_period", "read_only", "fully_expired"
}

// CertificateExpiryDetails contains certificate-specific alert information.
type CertificateExpiryDetails struct {
	CommonName      string    `json:"commonName"`
	SerialNumber    string    `json:"serialNumber"`
	NotAfter        time.Time `json:"notAfter"`
	DaysUntilExpiry int       `json:"daysUntilExpiry"`
	DNSNames        []string  `json:"dnsNames"`
}

// AlertLogOpts represents options for querying alerts
type AlertLogOpts struct {
	Types      []string      `json:"types,omitempty"`
	Interval   time.Duration `json:"interval,omitempty"`
	MaxPerNode int           `json:"maxPerNode,omitempty"`
}

// GetAlerts returns alerts stored in the system as a streaming JSON response
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
		dec := json.NewDecoder(resp.Body)
		for {
			var alert Alert
			if err = dec.Decode(&alert); err != nil {
				if errors.Is(err, io.EOF) {
					break
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
