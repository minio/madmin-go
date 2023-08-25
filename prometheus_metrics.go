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
	"io"
	"net/http"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/prom2json"
)

const (
	metricsRespBodyLimit = 10 << 20 // 10 MiB
)

// NodeMetrics - returns Node Metrics in Prometheus format
//
//	The client needs to be configured with the endpoint of the desired node
func (client *MetricsClient) NodeMetrics(ctx context.Context) ([]*prom2json.Family, error) {
	reqData := metricsRequestData{
		relativePath: "/v2/metrics/node",
	}

	// Execute GET on /minio/v2/metrics/node
	resp, err := client.executeRequest(ctx, reqData)
	if err != nil {
		return nil, err
	}
	defer closeResponse(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	return parsePrometheusResults(io.LimitReader(resp.Body, metricsRespBodyLimit))
}

// ClusterMetrics - returns Cluster Metrics in Prometheus format
func (client *MetricsClient) ClusterMetrics(ctx context.Context) ([]*prom2json.Family, error) {
	reqData := metricsRequestData{
		relativePath: "/v2/metrics/cluster",
	}

	// Execute GET on /minio/v2/metrics/cluster
	resp, err := client.executeRequest(ctx, reqData)
	if err != nil {
		return nil, err
	}
	defer closeResponse(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	return parsePrometheusResults(io.LimitReader(resp.Body, metricsRespBodyLimit))
}

func parsePrometheusResults(reader io.Reader) (results []*prom2json.Family, err error) {
	mfChan := make(chan *dto.MetricFamily)
	errChan := make(chan error)

	go func() {
		defer close(errChan)
		err = prom2json.ParseReader(reader, mfChan)
		if err != nil {
			errChan <- err
		}
	}()

	for mf := range mfChan {
		results = append(results, prom2json.NewFamily(mf))
	}
	if err := <-errChan; err != nil {
		return nil, err
	}
	return results, nil
}
