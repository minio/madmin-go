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
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/minio/minio-go/v7/pkg/s3utils"
)

//msgp:clearomitted
//msgp:tag json
//msgp:timezone utc
//go:generate msgp -file $GOFILE

var (
	// MinIO only supports last two versions
	// you can force an application by setting env MADMIN_API_VERSION=v3 if you want the
	// library to start using v3, which would allow the application to work with MinIO
	// servers that are not yet upgraded to v4 Admin API

	// AdminAPIVersion - admin api version used in the request.
	AdminAPIVersion = func() string {
		if v := os.Getenv("MADMIN_API_VERSION"); v == "v3" {
			return "v3"
		}
		return "v4"
	}()

	// Admin API version prefix
	adminAPIPrefix = "/" + AdminAPIVersion

	// Admin API version prefix for previous version.
	adminAPIOldPrefix = "/v3"

	// kmsAPIVersion - is the latest KMS API version, for KMS requests.
	// NOTE: MinIO only supports last two versions
	kmsAPIVersion = "v1"

	// kms API version prefix
	kmsAPIPrefix = "/" + kmsAPIVersion
)

// getEndpointURL - construct a new endpoint.
func getEndpointURL(endpoint string, secure bool) (*url.URL, error) {
	if strings.Contains(endpoint, ":") {
		host, _, err := net.SplitHostPort(endpoint)
		if err != nil {
			return nil, err
		}
		if !s3utils.IsValidIP(host) && !s3utils.IsValidDomain(host) {
			msg := "Endpoint: " + endpoint + " does not follow ip address or domain name standards."
			return nil, ErrInvalidArgument(msg)
		}
	} else {
		if !s3utils.IsValidIP(endpoint) && !s3utils.IsValidDomain(endpoint) {
			msg := "Endpoint: " + endpoint + " does not follow ip address or domain name standards."
			return nil, ErrInvalidArgument(msg)
		}
	}

	// If secure is false, use 'http' scheme.
	scheme := "https"
	if !secure {
		scheme = "http"
	}

	// Strip the obvious :443 and :80 from the endpoint
	// to avoid the signature mismatch error.
	if secure && strings.HasSuffix(endpoint, ":443") {
		endpoint = strings.TrimSuffix(endpoint, ":443")
	}
	if !secure && strings.HasSuffix(endpoint, ":80") {
		endpoint = strings.TrimSuffix(endpoint, ":80")
	}

	// Construct a secured endpoint URL.
	endpointURLStr := scheme + "://" + endpoint
	endpointURL, err := url.Parse(endpointURLStr)
	if err != nil {
		return nil, err
	}

	// Validate incoming endpoint URL.
	if err := isValidEndpointURL(endpointURL.String()); err != nil {
		return nil, err
	}
	return endpointURL, nil
}

// Verify if input endpoint URL is valid.
func isValidEndpointURL(endpointURL string) error {
	if endpointURL == "" {
		return ErrInvalidArgument("Endpoint url cannot be empty.")
	}
	url, err := url.Parse(endpointURL)
	if err != nil {
		return ErrInvalidArgument("Endpoint url cannot be parsed.")
	}
	if url.Path != "/" && url.Path != "" {
		return ErrInvalidArgument("Endpoint url cannot have fully qualified paths.")
	}
	return nil
}

// closeResponse close non nil response with any response Body.
// convenient wrapper to drain any remaining data on response body.
//
// Subsequently this allows golang http RoundTripper
// to re-use the same connection for future requests.
func closeResponse(resp *http.Response) {
	// Callers should close resp.Body when done reading from it.
	// If resp.Body is not closed, the Client's underlying RoundTripper
	// (typically Transport) may not be able to re-use a persistent TCP
	// connection to the server for a subsequent "keep-alive" request.
	if resp != nil && resp.Body != nil {
		// Drain any remaining Body and then close the connection.
		// Without this closing connection would disallow re-using
		// the same connection for future uses.
		//  - http://stackoverflow.com/a/17961593/4465767
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

// TimedAction contains a number of actions and their accumulated duration in nanoseconds.
type TimedAction struct {
	Count   uint64 `json:"count"`
	AccTime uint64 `json:"acc_time_ns"`
	MinTime uint64 `json:"min_ns,omitempty"`
	MaxTime uint64 `json:"max_ns,omitempty"`
	Bytes   uint64 `json:"bytes,omitempty"`
}

// Avg returns the average time spent on the action.
func (t TimedAction) Avg() time.Duration {
	if t.Count == 0 {
		return 0
	}
	return time.Duration(t.AccTime / t.Count)
}

// AvgBytes returns the average time spent on the action.
func (t TimedAction) AvgBytes() uint64 {
	if t.Count == 0 {
		return 0
	}
	return t.Bytes / t.Count
}

// Merge other into t.
func (t *TimedAction) Merge(other TimedAction) {
	if t.Count == 0 {
		t.MinTime = other.MinTime
	}
	if other.Count > 0 {
		t.MinTime = min(t.MinTime, other.MinTime)
	}
	t.Count += other.Count
	t.AccTime += other.AccTime
	t.Bytes += other.Bytes
	t.MaxTime = max(t.MaxTime, other.MaxTime)
}
