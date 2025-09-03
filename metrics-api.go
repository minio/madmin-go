package madmin

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// ClusterAPIStats is a simplified version of madmin.APIStats that is used to
// report cluster-wide API metrics.
type ClusterAPIStats struct {
	// Time these metrics were collected
	CollectedAt time.Time `json:"collected"`

	// Nodes responded to the request.
	Nodes int `json:"nodes"`

	// Errors will contain any errors encountered while collecting the metrics.
	Errors []string `json:"errors,omitempty"`

	// Number of active requests.
	ActiveRequests int64 `json:"activeRequests,omitempty"`

	// Number of queued requests.
	QueuedRequests int64 `json:"queuedRequests,omitempty"`

	// lastMinute is the combined stats for the last minute.
	LastMinute APIStats `json:"lastMinute"`

	// LastDay is the combined stats for the last day.
	LastDay APIStats `json:"lastDay"`

	// LastDaySegmented are the stats for the last day, accumulated in time segments.
	LastDaySegmented SegmentedAPIMetrics `json:"lastDaySegmented"`
}

// ClusterAPIStats makes an admin call to retrieve general API metrics.
func (adm *AdminClient) ClusterAPIStats(ctx context.Context) (res *ClusterAPIStats, err error) {
	path := adminAPIPrefix + "/api/stats"

	resp, err := adm.executeMethod(ctx,
		http.MethodGet, requestData{
			relPath: path,
		},
	)
	if err != nil {
		return nil, err
	}
	defer closeResponse(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	res = &ClusterAPIStats{}
	err = json.NewDecoder(resp.Body).Decode(res)
	return res, err
}
