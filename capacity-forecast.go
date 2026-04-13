//
// Copyright (c) 2015-2026 MinIO, Inc.
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
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package madmin

import (
	"context"
	"encoding/json"
	"net/http"
)

// CapacityForecast contains storage capacity predictions based on
// historical daily snapshots collected by the scanner.
type CapacityForecast struct {
	CurrentUsedBytes   uint64  `json:"currentUsedBytes"`
	CurrentTotalBytes  uint64  `json:"currentTotalBytes"`
	CurrentUsedPercent float64 `json:"currentUsedPercent"`
	DaysUntil80Pct     float64 `json:"daysUntil80Pct"`
	DaysUntil90Pct     float64 `json:"daysUntil90Pct"`
	DaysUntil100Pct    float64 `json:"daysUntil100Pct"`
	GrowthRatePerDay   int64   `json:"growthRatePerDay"`
	DataPointCount     int     `json:"dataPointCount"`

	// Worst-case prediction based on the largest single-day growth
	// observed between any two consecutive data points.
	MinDaysUntilFull float64 `json:"minDaysUntilFull"`

	// Confidence metrics for the linear regression.
	RSquared float64 `json:"rSquared"` // 0-1, goodness of fit
	Variance float64 `json:"variance"` // variance of daily usedFraction deltas

	// Short-window (14-day) regression for recency-weighted predictions.
	RecentGrowthRatePerDay float64 `json:"recentGrowthRatePerDay"`
	RecentDaysUntilFull    float64 `json:"recentDaysUntilFull"`
}

// CapacityForecast returns a storage capacity forecast based on
// historical daily snapshots.
func (adm *AdminClient) CapacityForecast(ctx context.Context) (CapacityForecast, error) {
	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{
			relPath: adminAPIPrefix + "/capacity-forecast",
		})
	defer closeResponse(resp)
	if err != nil {
		return CapacityForecast{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return CapacityForecast{}, httpRespToErrorResponse(resp)
	}

	var f CapacityForecast
	if err = json.NewDecoder(resp.Body).Decode(&f); err != nil {
		return CapacityForecast{}, err
	}

	return f, nil
}
