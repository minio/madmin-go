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
	"io"
	"net/http"

	"github.com/dustin/go-humanize"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/prom2json"
)

// MetricsRespBodyLimit sets the top level limit to the size of the
// metrics results supported by this library.
var (
	MetricsRespBodyLimit = int64(humanize.GiByte)
)

// NodeMetrics - returns Node Metrics in Prometheus format
//
//	The client needs to be configured with the endpoint of the desired node
func (client *MetricsClient) NodeMetrics(ctx context.Context) ([]*prom2json.Family, error) {
	return client.GetMetrics(ctx, "node")
}

// ClusterMetrics - returns Cluster Metrics in Prometheus format
func (client *MetricsClient) ClusterMetrics(ctx context.Context) ([]*prom2json.Family, error) {
	return client.GetMetrics(ctx, "cluster")
}

// BucketMetrics - returns Bucket Metrics in Prometheus format
func (client *MetricsClient) BucketMetrics(ctx context.Context) ([]*prom2json.Family, error) {
	return client.GetMetrics(ctx, "bucket")
}

// ResourceMetrics - returns Resource Metrics in Prometheus format
func (client *MetricsClient) ResourceMetrics(ctx context.Context) ([]*prom2json.Family, error) {
	return client.GetMetrics(ctx, "resource")
}

// GetMetrics - returns Metrics of given subsystem in Prometheus format
func (client *MetricsClient) GetMetrics(ctx context.Context, subSystem string) ([]*prom2json.Family, error) {
	reqData := metricsRequestData{
		relativePath: "/v2/metrics/" + subSystem,
	}

	// Execute GET on /minio/v2/metrics/<subSys>
	resp, err := client.executeGetRequest(ctx, reqData)
	if err != nil {
		return nil, err
	}
	defer closeResponse(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	return ParsePrometheusResults(io.LimitReader(resp.Body, MetricsRespBodyLimit))
}

func ParsePrometheusResults(reader io.Reader) (results []*prom2json.Family, err error) {
	// We could do further content-type checks here, but the
	// fallback for now will anyway be the text format
	// version 0.0.4, so just go for it and see if it works.
	var parser expfmt.TextParser
	metricFamilies, err := parser.TextToMetricFamilies(reader)
	if err != nil {
		return nil, fmt.Errorf("reading text format failed: %v", err)
	}
	results = make([]*prom2json.Family, 0, len(metricFamilies))
	for _, mf := range metricFamilies {
		results = append(results, prom2json.NewFamily(mf))
	}
	return results, nil
}
