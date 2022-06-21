//
// MinIO Object Storage (c) 2022 MinIO, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package madmin

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

const (
	minioWriteQuorumHeader     = "x-minio-write-quorum"
	minIOHealingDrives         = "x-minio-healing-drives"
	clusterCheckEndpoint       = "/minio/health/cluster"
	clusterReadCheckEndpoint   = "/minio/health/cluster/read"
	maintanenceURLParameterKey = "maintenance"
)

// HealthResult represents the cluster health result
type HealthResult struct {
	Healthy         bool
	MaintenanceMode bool
	WriteQuorum     int
	HealingDrives   int
}

// HealthOpts represents the input options for the health check
type HealthOpts struct {
	ClusterRead bool
	Maintenance bool
}

// Healthy will hit `/minio/health/cluster` and `/minio/health/cluster/ready` anonymous APIs to check the cluster health
func (an *AnonymousClient) Healthy(ctx context.Context, opts HealthOpts) (result HealthResult, err error) {
	if opts.ClusterRead {
		return an.clusterReadCheck(ctx)
	}
	return an.clusterCheck(ctx, opts.Maintenance)
}

func (an *AnonymousClient) clusterCheck(ctx context.Context, maintenance bool) (result HealthResult, err error) {
	urlValues := make(url.Values)
	if maintenance {
		urlValues.Set(maintanenceURLParameterKey, "true")
	}

	resp, err := an.executeMethod(ctx, http.MethodGet, requestData{
		relPath:     clusterCheckEndpoint,
		queryValues: urlValues,
	})
	defer closeResponse(resp)
	if err != nil {
		return result, err
	}

	if resp != nil {
		writeQuorumStr := resp.Header.Get(minioWriteQuorumHeader)
		if writeQuorumStr != "" {
			result.WriteQuorum, err = strconv.Atoi(writeQuorumStr)
			if err != nil {
				return result, err
			}
		}
		healingDrivesStr := resp.Header.Get(minIOHealingDrives)
		if healingDrivesStr != "" {
			result.HealingDrives, err = strconv.Atoi(healingDrivesStr)
			if err != nil {
				return result, err
			}
		}
		switch resp.StatusCode {
		case http.StatusOK:
			result.Healthy = true
		case http.StatusPreconditionFailed:
			result.MaintenanceMode = true
		default:
			// Not Healthy
		}
	}
	return result, nil
}

func (an *AnonymousClient) clusterReadCheck(ctx context.Context) (result HealthResult, err error) {
	resp, err := an.executeMethod(ctx, http.MethodGet, requestData{
		relPath: clusterReadCheckEndpoint,
	})
	defer closeResponse(resp)
	if err != nil {
		return result, err
	}

	if resp != nil {
		switch resp.StatusCode {
		case http.StatusOK:
			result.Healthy = true
		default:
			// Not Healthy
		}
	}
	return result, nil
}
