// Copyright (c) 2015-2025 MinIO, Inc.
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

package mnav

import (
	"fmt"
	"strconv"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/minio/madmin-go/v4"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// SiteResyncMetricsNode handles navigation for SiteResyncMetrics
type SiteResyncMetricsNode struct {
	resync *madmin.SiteResyncMetrics
	parent MetricNode
	path   string
}

func (node *SiteResyncMetricsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewSiteResyncMetricsNode(resync *madmin.SiteResyncMetrics, parent MetricNode, path string) *SiteResyncMetricsNode {
	return &SiteResyncMetricsNode{
		resync: resync,
		parent: parent,
		path:   path,
	}
}

func (node *SiteResyncMetricsNode) GetChildren() []MetricChild {
	// Show all data directly in main view - no child navigation needed
	return []MetricChild{}
}

func (node *SiteResyncMetricsNode) GetLeafData() map[string]string {
	if node.resync == nil {
		return map[string]string{
			"Status": "No resync data available",
		}
	}

	data := map[string]string{}

	// Resync status and basic info
	status := node.resync.ResyncStatus
	if status == "" {
		status = "unknown"
	}
	titleCaser := cases.Title(language.Und)
	data["Status"] = titleCaser.String(status)

	if node.resync.ResyncID != "" {
		data["Resync ID"] = node.resync.ResyncID
	}

	if node.resync.DeplID != "" {
		data["Deployment ID"] = node.resync.DeplID
	}

	// Timing information
	if !node.resync.StartTime.IsZero() {
		data["Started"] = node.resync.StartTime.Format("15:04:05")

		// Calculate duration if still running
		endTime := node.resync.LastUpdate
		if endTime.IsZero() {
			endTime = node.resync.CollectedAt
		}
		if !endTime.IsZero() {
			duration := endTime.Sub(node.resync.StartTime)
			data["Duration"] = formatResyncDuration(duration)
		}
	}

	// Progress metrics
	if node.resync.NumBuckets > 0 {
		data["Total buckets"] = strconv.FormatInt(node.resync.NumBuckets, 10)
	}

	// Replication success metrics
	if node.resync.ReplicatedCount > 0 {
		data["Objects synced"] = humanize.Comma(node.resync.ReplicatedCount)
	}

	if node.resync.ReplicatedSize > 0 {
		data["Data synced"] = humanize.Bytes(uint64(node.resync.ReplicatedSize))
	}

	// Failure metrics
	if node.resync.FailedCount > 0 {
		data["Failed objects"] = humanize.Comma(node.resync.FailedCount)
	}

	if node.resync.FailedSize > 0 {
		data["Failed data"] = humanize.Bytes(uint64(node.resync.FailedSize))
	}

	if len(node.resync.FailedBuckets) > 0 {
		data["Failed buckets"] = fmt.Sprintf("%d buckets", len(node.resync.FailedBuckets))
	}

	// Current processing info
	if node.resync.Bucket != "" {
		currentInfo := node.resync.Bucket
		if node.resync.Object != "" {
			// Truncate long object names
			obj := node.resync.Object
			if len(obj) > 30 {
				obj = obj[:27] + "..."
			}
			currentInfo += "/" + obj
		}
		data["Current"] = currentInfo
	}

	// Collection timestamp
	data["Last updated"] = node.resync.CollectedAt.Format("15:04:05")

	return data
}

func (node *SiteResyncMetricsNode) GetMetricType() madmin.MetricType {
	return madmin.MetricsSiteResync
}

func (node *SiteResyncMetricsNode) GetMetricFlags() madmin.MetricFlags {
	return 0
}

func (node *SiteResyncMetricsNode) GetParent() MetricNode {
	return node.parent
}

func (node *SiteResyncMetricsNode) GetPath() string {
	return node.path
}

func (node *SiteResyncMetricsNode) ShouldPauseRefresh() bool {
	// Site resync operations can take a long time to complete
	// If operation is completed or failed, no need for frequent refresh
	if node.resync != nil && (node.resync.Complete() || node.resync.ResyncStatus == "failed") {
		return true
	}
	// Keep refreshing while operation is active
	return false
}

func (node *SiteResyncMetricsNode) GetChild(_ string) (MetricNode, error) {
	// This is a leaf node - no children
	return nil, fmt.Errorf("site resync is a leaf node - no children available")
}

// Helper function to format duration in a readable way
func formatResyncDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0fm %.0fs", d.Minutes(), float64(d.Seconds())-(d.Minutes()*60))
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) - (hours * 60)
	seconds := int(d.Seconds()) - (hours * 3600) - (minutes * 60)
	return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
}
