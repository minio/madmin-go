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
	"fmt"
	"net/url"
	"testing"

	jwtgo "github.com/golang-jwt/jwt/v4"
)

func TestMakeTargetUrlBuildsURLWithClientAndRelativePath(t *testing.T) {
	clnt := MetricsClient{
		endpointURL: &url.URL{
			Host:   "localhost:9000",
			Scheme: "http",
		},
	}
	requestData := metricsRequestData{
		relativePath: "/some/path",
	}

	targetURL, err := clnt.makeTargetURL(requestData)
	if err != nil {
		t.Errorf("error not expected, got: %v", err)
	}

	expectedURL := "http://localhost:9000/minio/some/path"
	if expectedURL != targetURL.String() {
		t.Errorf("target url: %s  not equal to expected url: %s", targetURL, expectedURL)
	}
}

func TestMakeTargetUrlReturnsErrorIfEndpointURLNotSet(t *testing.T) {
	clnt := MetricsClient{}
	requestData := metricsRequestData{
		relativePath: "/some/path",
	}

	_, err := clnt.makeTargetURL(requestData)
	if err == nil {
		t.Errorf("error expected got nil")
	}
}

func TestMakeTargetUrlReturnsErrorOnURLParse(t *testing.T) {
	clnt := MetricsClient{
		endpointURL: &url.URL{},
	}
	requestData := metricsRequestData{
		relativePath: "/some/path",
	}

	_, err := clnt.makeTargetURL(requestData)
	if err == nil {
		t.Errorf("error expected got nil")
	}
}

func TestPrivteNewMetricsClientInstantiatesMetricsClientWithRequiredFields(t *testing.T) {
	endpointURL := &url.URL{}
	jwtToken := "someToken"
	secure := true

	clnt, err := privateNewMetricsClient(endpointURL, jwtToken, secure)
	if err != nil {
		t.Errorf("error not expected, got: %v", err)
	}
	if clnt.endpointURL != endpointURL {
		t.Errorf("clnt.endpointURL: %s  not equal to endpointURL: %s", clnt.endpointURL, endpointURL)
	}
	if clnt.jwtToken != jwtToken {
		t.Errorf("clnt.jwtToken: %s  not equal to jwtToken: %s", clnt.jwtToken, jwtToken)
	}
	if clnt.secure != secure {
		t.Errorf("clnt.secure: %v  not equal to secure: %v", clnt.secure, secure)
	}
	if clnt.httpClient.Transport == nil {
		t.Errorf("clnt.Transport expecting not nil")
	}
}

func TestGetPrometheusTokenReturnsValidJwtTokenFromAccessAndSecretKey(t *testing.T) {
	accessKey := "myaccessKey"
	secretKey := "mysecretKey"

	jwtToken, err := getPrometheusToken(accessKey, secretKey)
	if err != nil {
		t.Errorf("error not expected, got: %v", err)
	}

	token, err := jwtgo.Parse(jwtToken, func(token *jwtgo.Token) (interface{}, error) {
		// Set same signing method used in our function
		if _, ok := token.Method.(*jwtgo.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretKey), nil
	})

	if err != nil {
		t.Errorf("error not expected, got: %v", err)
	}
	if !token.Valid {
		t.Errorf("invalid token: %s", jwtToken)
	}
}
