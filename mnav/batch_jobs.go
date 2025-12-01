package mnav

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/minio/madmin-go/v4"
)

// BatchJobMetricsNode handles navigation for BatchJobMetrics (overview of all jobs)
type BatchJobMetricsNode struct {
	batch  *madmin.BatchJobMetrics
	parent MetricNode
	path   string
}

func (node *BatchJobMetricsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewBatchJobMetricsNode(batch *madmin.BatchJobMetrics, parent MetricNode, path string) *BatchJobMetricsNode {
	return &BatchJobMetricsNode{
		batch:  batch,
		parent: parent,
		path:   path,
	}
}

func (node *BatchJobMetricsNode) GetChildren() []MetricChild {
	if node.batch == nil || len(node.batch.Jobs) == 0 {
		return []MetricChild{}
	}

	var children []MetricChild
	// Sort jobs by status (active first) then by last update
	var jobIDs []string
	for jobID := range node.batch.Jobs {
		jobIDs = append(jobIDs, jobID)
	}

	sort.Slice(jobIDs, func(i, j int) bool {
		jobA := node.batch.Jobs[jobIDs[i]]
		jobB := node.batch.Jobs[jobIDs[j]]

		// Active jobs (not complete, not failed) come first
		activeA := !jobA.Complete && !jobA.Failed
		activeB := !jobB.Complete && !jobB.Failed

		if activeA != activeB {
			return activeA
		}

		// Then sort by last update (most recent first)
		return jobA.LastUpdate.After(jobB.LastUpdate)
	})

	for _, jobID := range jobIDs {
		job := node.batch.Jobs[jobID]
		status := "running"
		if job.Complete {
			status = "completed"
		} else if job.Failed {
			status = "failed"
		}

		description := fmt.Sprintf("%s job - %s", job.JobType, status)
		children = append(children, MetricChild{
			Name:        jobID,
			Description: description,
		})
	}

	return children
}

func (node *BatchJobMetricsNode) GetLeafData() map[string]string {
	if node.batch == nil || len(node.batch.Jobs) == 0 {
		return map[string]string{
			"Status": "No batch jobs available",
		}
	}

	data := map[string]string{}

	// Count jobs by status and type
	var activeCount, completedCount, failedCount int
	jobTypes := make(map[string]int)

	for _, job := range node.batch.Jobs {
		if job.Complete {
			completedCount++
		} else if job.Failed {
			failedCount++
		} else {
			activeCount++
		}

		jobTypes[job.JobType]++
	}

	// Display counts
	if activeCount > 0 {
		data["Active jobs"] = strconv.Itoa(activeCount)
	}
	if completedCount > 0 {
		data["Completed jobs"] = strconv.Itoa(completedCount)
	}
	if failedCount > 0 {
		data["Failed jobs"] = strconv.Itoa(failedCount)
	}

	// Display job types
	if len(jobTypes) > 0 {
		var types []string
		for jobType, count := range jobTypes {
			if count == 1 {
				types = append(types, jobType)
			} else {
				types = append(types, fmt.Sprintf("%s(%d)", jobType, count))
			}
		}
		sort.Strings(types)
		data["Job types"] = strings.Join(types, ", ")
	}

	data["Total jobs"] = strconv.Itoa(len(node.batch.Jobs))
	data["Last updated"] = node.batch.CollectedAt.Format("15:04:05")

	return data
}

func (node *BatchJobMetricsNode) GetMetricType() madmin.MetricType {
	return madmin.MetricsBatchJobs
}

func (node *BatchJobMetricsNode) GetMetricFlags() madmin.MetricFlags {
	return 0
}

func (node *BatchJobMetricsNode) GetParent() MetricNode {
	return node.parent
}

func (node *BatchJobMetricsNode) GetPath() string {
	return node.path
}

func (node *BatchJobMetricsNode) ShouldPauseRefresh() bool {
	// Batch job overview should refresh to show new jobs and status changes
	return false
}

func (node *BatchJobMetricsNode) GetChild(name string) (MetricNode, error) {
	if node.batch == nil || len(node.batch.Jobs) == 0 {
		return nil, fmt.Errorf("no batch jobs available")
	}

	job, exists := node.batch.Jobs[name]
	if !exists {
		return nil, fmt.Errorf("job not found: %s", name)
	}

	return NewBatchJobNode(&job, node, fmt.Sprintf("%s/%s", node.path, name)), nil
}

// BatchJobNode handles navigation for individual batch jobs
type BatchJobNode struct {
	job    *madmin.JobMetric
	parent MetricNode
	path   string
}

func (node *BatchJobNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewBatchJobNode(job *madmin.JobMetric, parent MetricNode, path string) *BatchJobNode {
	return &BatchJobNode{
		job:    job,
		parent: parent,
		path:   path,
	}
}

func (node *BatchJobNode) GetChildren() []MetricChild {
	// This is a leaf node - show all data directly
	return []MetricChild{}
}

func (node *BatchJobNode) GetLeafData() map[string]string {
	if node.job == nil {
		return map[string]string{
			"Status": "No job data available",
		}
	}

	data := map[string]string{}

	// Basic job info
	data["Job type"] = node.job.JobType
	data["Job ID"] = node.job.JobID

	// Status
	if node.job.Complete {
		data["Status"] = "completed"
	} else if node.job.Failed {
		data["Status"] = "failed"
	} else {
		data["Status"] = "running"
	}

	if node.job.Status != "" && node.job.Status != data["Status"] {
		data["Detailed status"] = node.job.Status
	}

	// Timing
	if !node.job.StartTime.IsZero() {
		data["Started"] = node.job.StartTime.Format("15:04:05")

		// Calculate duration
		endTime := node.job.LastUpdate
		if endTime.IsZero() || (!node.job.Complete && !node.job.Failed) {
			endTime = time.Now()
		}
		if !endTime.IsZero() {
			duration := endTime.Sub(node.job.StartTime)
			data["Duration"] = formatJobDuration(duration)
		}
	}

	if !node.job.LastUpdate.IsZero() {
		data["Last updated"] = node.job.LastUpdate.Format("15:04:05")
	}

	if node.job.RetryAttempts > 0 {
		data["Retry attempts"] = strconv.Itoa(node.job.RetryAttempts)
	}

	// Type-specific data
	if node.job.Replicate != nil {
		addReplicateData(data, node.job.Replicate)
	} else if node.job.Expired != nil {
		addExpirationData(data, node.job.Expired)
	} else if node.job.KeyRotate != nil {
		addKeyRotateData(data, node.job.KeyRotate)
	} else if node.job.Catalog != nil {
		addCatalogData(data, node.job.Catalog)
	}

	return data
}

func (node *BatchJobNode) GetMetricType() madmin.MetricType {
	return madmin.MetricsBatchJobs
}

func (node *BatchJobNode) GetMetricFlags() madmin.MetricFlags {
	return 0
}

func (node *BatchJobNode) GetParent() MetricNode {
	return node.parent
}

func (node *BatchJobNode) GetPath() string {
	return node.path
}

func (node *BatchJobNode) ShouldPauseRefresh() bool {
	// Individual job details - if job is completed or failed, no need for frequent refresh
	if node.job != nil && (node.job.Complete || node.job.Failed) {
		return true
	}
	// Keep refreshing while job is running
	return false
}

func (node *BatchJobNode) GetChild(name string) (MetricNode, error) {
	// This is a leaf node
	return nil, fmt.Errorf("batch job is a leaf node - no children available")
}

// Helper functions for type-specific data

func addReplicateData(data map[string]string, info *madmin.ReplicateInfo) {
	if info.Objects > 0 {
		data["Objects synced"] = humanize.Comma(info.Objects)
	}
	if info.ObjectsFailed > 0 {
		data["Objects failed"] = humanize.Comma(info.ObjectsFailed)
	}
	if info.DeleteMarkers > 0 {
		data["Delete markers"] = humanize.Comma(info.DeleteMarkers)
	}
	if info.DeleteMarkersFailed > 0 {
		data["Delete markers failed"] = humanize.Comma(info.DeleteMarkersFailed)
	}
	if info.BytesTransferred > 0 {
		data["Data transferred"] = humanize.Bytes(uint64(info.BytesTransferred))
	}
	if info.BytesFailed > 0 {
		data["Data failed"] = humanize.Bytes(uint64(info.BytesFailed))
	}
	if info.Bucket != "" {
		currentInfo := info.Bucket
		if info.Object != "" {
			// Truncate long object names
			obj := info.Object
			if len(obj) > 30 {
				obj = obj[:27] + "..."
			}
			currentInfo += "/" + obj
		}
		data["Current"] = currentInfo
	}
}

func addExpirationData(data map[string]string, info *madmin.ExpirationInfo) {
	if info.Objects > 0 {
		data["Objects expired"] = humanize.Comma(info.Objects)
	}
	if info.ObjectsFailed > 0 {
		data["Objects failed"] = humanize.Comma(info.ObjectsFailed)
	}
	if info.DeleteMarkers > 0 {
		data["Delete markers"] = humanize.Comma(info.DeleteMarkers)
	}
	if info.DeleteMarkersFailed > 0 {
		data["Delete markers failed"] = humanize.Comma(info.DeleteMarkersFailed)
	}
	if info.Bucket != "" {
		currentInfo := info.Bucket
		if info.Object != "" {
			// Truncate long object names
			obj := info.Object
			if len(obj) > 30 {
				obj = obj[:27] + "..."
			}
			currentInfo += "/" + obj
		}
		data["Current"] = currentInfo
	}
}

func addKeyRotateData(data map[string]string, info *madmin.KeyRotationInfo) {
	if info.Objects > 0 {
		data["Objects rotated"] = humanize.Comma(info.Objects)
	}
	if info.ObjectsFailed > 0 {
		data["Objects failed"] = humanize.Comma(info.ObjectsFailed)
	}
	if info.Bucket != "" {
		currentInfo := info.Bucket
		if info.Object != "" {
			// Truncate long object names
			obj := info.Object
			if len(obj) > 30 {
				obj = obj[:27] + "..."
			}
			currentInfo += "/" + obj
		}
		data["Current"] = currentInfo
	}
}

func addCatalogData(data map[string]string, info *madmin.CatalogInfo) {
	if info.ObjectsScannedCount > 0 {
		data["Objects scanned"] = humanize.Comma(int64(info.ObjectsScannedCount))
	}
	if info.ObjectsMatchedCount > 0 {
		data["Objects matched"] = humanize.Comma(int64(info.ObjectsMatchedCount))
	}
	if info.RecordsWrittenCount > 0 {
		data["Records written"] = humanize.Comma(int64(info.RecordsWrittenCount))
	}
	if info.OutputObjectsCount > 0 {
		data["Output objects"] = humanize.Comma(int64(info.OutputObjectsCount))
	}
	if info.ManifestPathBucket != "" && info.ManifestPathObject != "" {
		data["Manifest"] = info.ManifestPathBucket + "/" + info.ManifestPathObject
	}
	if info.ErrorMsg != "" {
		data["Error"] = info.ErrorMsg
	}

	// Current processing info
	if info.Bucket != "" {
		currentInfo := info.Bucket
		if info.LastObjectScanned != "" {
			obj := info.LastObjectScanned
			if len(obj) > 30 {
				obj = obj[:27] + "..."
			}
			currentInfo += "/" + obj
		}
		data["Current"] = currentInfo
	}
}

// Helper function to format duration in a readable way (avoiding conflict with process.go)
func formatJobDuration(d time.Duration) string {
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
