//
// Copyright (c) 2015-2023 MinIO, Inc.
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
	"fmt"
	"net/http"
	"net/url"
	"time"

	jwtgo "github.com/golang-jwt/jwt/v4"
)

const (
	defaultPrometheusJWTExpiry = 100 * 365 * 24 * time.Hour
	libraryMinioURLPrefix      = "/minio"
	prometheusIssuer           = "prometheus"
)

// MetricsClient implements MinIO metrics operations
type MetricsClient struct {
	/// JWT token for authentication
	jwtToken string
	// Indicate whether we are using https or not
	secure bool
	// Parsed endpoint url provided by the user.
	endpointURL *url.URL
	// Needs allocation.
	httpClient *http.Client
}

// metricsRequestData - is container for all the values to make a
// request.
type metricsRequestData struct {
	relativePath string // URL path relative to admin API base endpoint
}

// NewMetricsClient - instantiate minio metrics client honoring Prometheus format
func NewMetricsClient(endpoint string, accessKeyID, secretAccessKey string, secure bool) (*MetricsClient, error) {
	jwtToken, err := getPrometheusToken(accessKeyID, secretAccessKey)
	if err != nil {
		return nil, err
	}

	endpointURL, err := getEndpointURL(endpoint, secure)
	if err != nil {
		return nil, err
	}

	clnt, err := privateNewMetricsClient(endpointURL, jwtToken, secure)
	if err != nil {
		return nil, err
	}
	return clnt, nil
}

// getPrometheusToken creates a JWT from MinIO access and secret keys
func getPrometheusToken(accessKey, secretKey string) (string, error) {
	jwt := jwtgo.NewWithClaims(jwtgo.SigningMethodHS512, jwtgo.RegisteredClaims{
		ExpiresAt: jwtgo.NewNumericDate(time.Now().UTC().Add(defaultPrometheusJWTExpiry)),
		Subject:   accessKey,
		Issuer:    prometheusIssuer,
	})

	token, err := jwt.SignedString([]byte(secretKey))
	if err != nil {
		return "", err
	}
	return token, nil
}

func privateNewMetricsClient(endpointURL *url.URL, jwtToken string, secure bool) (*MetricsClient, error) {
	clnt := new(MetricsClient)
	clnt.jwtToken = jwtToken
	clnt.secure = secure
	clnt.endpointURL = endpointURL
	clnt.httpClient = &http.Client{
		Transport: DefaultTransport(secure),
	}
	return clnt, nil
}

// executeGetRequest - instantiates a Get method and performs the request
func (client MetricsClient) executeGetRequest(ctx context.Context, reqData metricsRequestData) (res *http.Response, err error) {
	req, err := client.newGetRequest(ctx, reqData)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+client.jwtToken)
	return client.httpClient.Do(req)
}

// newGetRequest - instantiate a new HTTP GET request
func (client MetricsClient) newGetRequest(ctx context.Context, reqData metricsRequestData) (req *http.Request, err error) {
	targetURL, err := client.makeTargetURL(reqData)
	if err != nil {
		return nil, err
	}

	return http.NewRequestWithContext(ctx, http.MethodGet, targetURL.String(), nil)
}

// makeTargetURL make a new target url.
func (client MetricsClient) makeTargetURL(r metricsRequestData) (*url.URL, error) {
	if client.endpointURL == nil {
		return nil, fmt.Errorf("enpointURL cannot be nil")
	}

	host := client.endpointURL.Host
	scheme := client.endpointURL.Scheme
	prefix := libraryMinioURLPrefix

	urlStr := scheme + "://" + host + prefix + r.relativePath
	return url.Parse(urlStr)
}
