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
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/dustin/go-humanize"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
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
	parser := expfmt.NewTextParser(model.UTF8Validation)
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

// Prometheus v2 metrics
var (
	ClusterV2Metrics = []string{
		// Cluster capacity metrics
		"minio_cluster_bucket_total",
		"minio_cluster_capacity_raw_free_bytes",
		"minio_cluster_capacity_raw_total_bytes",
		"minio_cluster_capacity_usable_free_bytes",
		"minio_cluster_capacity_usable_total_bytes",
		"minio_cluster_objects_size_distribution",
		"minio_cluster_objects_version_distribution",
		"minio_cluster_usage_object_total",
		"minio_cluster_usage_total_bytes",
		"minio_cluster_usage_version_total",
		"minio_cluster_usage_deletemarker_total",
		// Cluster drive metrics
		"minio_cluster_drive_offline_total",
		"minio_cluster_drive_online_total",
		"minio_cluster_drive_total",
		// Cluster health metrics
		"minio_cluster_nodes_offline_total",
		"minio_cluster_nodes_online_total",
		"minio_cluster_write_quorum",
		"minio_cluster_health_status",
		"minio_cluster_health_erasure_set_healing_drives",
		"minio_cluster_health_erasure_set_online_drives",
		"minio_cluster_health_erasure_set_read_quorum",
		"minio_cluster_health_erasure_set_write_quorum",
		"minio_cluster_health_erasure_set_status",
		// S3 API requests metrics
		"minio_s3_requests_incoming_total",
		"minio_s3_requests_inflight_total",
		"minio_s3_requests_rejected_auth_total",
		"minio_s3_requests_rejected_header_total",
		"minio_s3_requests_rejected_invalid_total",
		"minio_s3_requests_rejected_timestamp_total",
		"minio_s3_requests_total",
		"minio_s3_requests_waiting_total",
		"minio_s3_requests_ttfb_seconds_distribution",
		"minio_s3_traffic_received_bytes",
		"minio_s3_traffic_sent_bytes",
		// Scanner metrics
		"minio_node_scanner_bucket_scans_finished",
		"minio_node_scanner_bucket_scans_started",
		"minio_node_scanner_directories_scanned",
		"minio_node_scanner_objects_scanned",
		"minio_node_scanner_versions_scanned",
		"minio_node_syscall_read_total",
		"minio_node_syscall_write_total",
		"minio_usage_last_activity_nano_seconds",
		// Inter node metrics
		"minio_inter_node_traffic_dial_avg_time",
		"minio_inter_node_traffic_received_bytes",
		"minio_inter_node_traffic_sent_bytes",
		// Process metrics
		"minio_node_process_cpu_total_seconds",
		"minio_node_process_resident_memory_bytes",
		"minio_node_process_starttime_seconds",
		"minio_node_process_uptime_seconds",
		// File descriptor metrics
		"minio_node_file_descriptor_limit_total",
		"minio_node_file_descriptor_open_total",
		// Node metrics
		"minio_node_go_routine_total",
		"minio_node_io_rchar_bytes",
		"minio_node_io_read_bytes",
		"minio_node_io_wchar_bytes",
		"minio_node_io_write_bytes",
	}
	ReplicationV2Metrics = []string{
		// Cluster replication metrics
		"minio_cluster_replication_last_hour_failed_bytes",
		"minio_cluster_replication_last_hour_failed_count",
		"minio_cluster_replication_last_minute_failed_bytes",
		"minio_cluster_replication_last_minute_failed_count",
		"minio_cluster_replication_total_failed_bytes",
		"minio_cluster_replication_total_failed_count",
		"minio_cluster_replication_received_bytes",
		"minio_cluster_replication_received_count",
		"minio_cluster_replication_sent_bytes",
		"minio_cluster_replication_sent_count",
		"minio_cluster_replication_proxied_get_requests_total",
		"minio_cluster_replication_proxied_head_requests_total",
		"minio_cluster_replication_proxied_delete_tagging_requests_total",
		"minio_cluster_replication_proxied_get_tagging_requests_total",
		"minio_cluster_replication_proxied_put_tagging_requests_total",
		"minio_cluster_replication_proxied_get_requests_failures",
		"minio_cluster_replication_proxied_head_requests_failures",
		"minio_cluster_replication_proxied_delete_tagging_requests_failures",
		"minio_cluster_replication_proxied_get_tagging_requests_failures",
		"minio_cluster_replication_proxied_put_tagging_requests_failures",
		// Node replication metrics
		"minio_node_replication_current_active_workers",
		"minio_node_replication_average_active_workers",
		"minio_node_replication_max_active_workers",
		"minio_node_replication_link_online",
		"minio_node_replication_link_offline_duration_seconds",
		"minio_node_replication_link_downtime_duration_seconds",
		"minio_node_replication_average_link_latency_ms",
		"minio_node_replication_max_link_latency_ms",
		"minio_node_replication_current_link_latency_ms",
		"minio_node_replication_current_transfer_rate",
		"minio_node_replication_average_transfer_rate",
		"minio_node_replication_max_transfer_rate",
		"minio_node_replication_last_minute_queued_count",
		"minio_node_replication_last_minute_queued_bytes",
		"minio_node_replication_average_queued_count",
		"minio_node_replication_average_queued_bytes",
		"minio_node_replication_max_queued_bytes",
		"minio_node_replication_max_queued_count",
		"minio_node_replication_recent_backlog_count",
	}
	BucketV2Metrics = []string{
		// Bucket metrics
		"minio_bucket_objects_size_distribution",
		"minio_bucket_objects_version_distribution",
		"minio_bucket_traffic_received_bytes",
		"minio_bucket_traffic_sent_bytes",
		"minio_bucket_usage_object_total",
		"minio_bucket_usage_version_total",
		"minio_bucket_usage_deletemarker_total",
		"minio_bucket_usage_total_bytes",
		"minio_bucket_requests_inflight_total",
		"minio_bucket_requests_total",
		"minio_bucket_requests_ttfb_seconds_distribution",
		// Bucket replication metrics
		"minio_bucket_replication_last_minute_failed_bytes",
		"minio_bucket_replication_last_minute_failed_count",
		"minio_bucket_replication_last_hour_failed_bytes",
		"minio_bucket_replication_last_hour_failed_count",
		"minio_bucket_replication_total_failed_bytes",
		"minio_bucket_replication_total_failed_count",
		"minio_bucket_replication_latency_ms",
		"minio_bucket_replication_received_bytes",
		"minio_bucket_replication_received_count",
		"minio_bucket_replication_sent_bytes",
		"minio_bucket_replication_sent_count",
		"minio_bucket_replication_proxied_get_requests_total",
		"minio_bucket_replication_proxied_head_requests_total",
		"minio_bucket_replication_proxied_delete_tagging_requests_total",
		"minio_bucket_replication_proxied_get_tagging_requests_total",
		"minio_bucket_replication_proxied_put_tagging_requests_total",
		"minio_bucket_replication_proxied_get_requests_failures",
		"minio_bucket_replication_proxied_head_requests_failures",
		"minio_bucket_replication_proxied_delete_tagging_requests_failures",
		"minio_bucket_replication_proxied_get_tagging_requests_failures",
		"minio_bucket_replication_proxied_put_tagging_requests_failures",
	}
	NodeV2Metrics = []string{
		"minio_node_drive_free_bytes",
		"minio_node_drive_free_inodes",
		"minio_node_drive_latency_us",
		"minio_node_drive_offline_total",
		"minio_node_drive_online_total",
		"minio_node_drive_total",
		"minio_node_drive_total_bytes",
		"minio_node_drive_used_bytes",
		"minio_node_drive_errors_timeout",
		"minio_node_drive_errors_ioerror",
		"minio_node_drive_errors_availability",
		"minio_node_drive_io_waiting",
	}
	ResourceV2Metrics = []string{
		"minio_node_drive_total_bytes",
		"minio_node_drive_used_bytes",
		"minio_node_drive_total_inodes  ",
		"minio_node_drive_used_inodes",
		"minio_node_drive_reads_per_sec",
		"minio_node_drive_reads_kb_per_sec",
		"minio_node_drive_reads_await",
		"minio_node_drive_writes_per_sec",
		"minio_node_drive_writes_kb_per_sec",
		"minio_node_drive_writes_await",
		"minio_node_drive_perc_util",
		"minio_node_if_rx_bytes",
		"minio_node_if_rx_bytes_avg",
		"minio_node_if_rx_bytes_max",
		"minio_node_if_rx_errors",
		"minio_node_if_rx_errors_avg",
		"minio_node_if_rx_errors_max",
		"minio_node_if_tx_bytes",
		"minio_node_if_tx_bytes_avg",
		"minio_node_if_tx_bytes_max",
		"minio_node_if_tx_errors",
		"minio_node_if_tx_errors_avg",
		"minio_node_if_tx_errors_max",
		"minio_node_cpu_avg_user",
		"minio_node_cpu_avg_user_avg",
		"minio_node_cpu_avg_user_max",
		"minio_node_cpu_avg_system",
		"minio_node_cpu_avg_system_avg",
		"minio_node_cpu_avg_system_max",
		"minio_node_cpu_avg_idle",
		"minio_node_cpu_avg_idle_avg",
		"minio_node_cpu_avg_idle_max",
		"minio_node_cpu_avg_iowait",
		"minio_node_cpu_avg_iowait_avg",
		"minio_node_cpu_avg_iowait_max",
		"minio_node_cpu_avg_nice",
		"minio_node_cpu_avg_nice_avg",
		"minio_node_cpu_avg_nice_max",
		"minio_node_cpu_avg_steal",
		"minio_node_cpu_avg_steal_avg",
		"minio_node_cpu_avg_steal_max",
		"minio_node_cpu_avg_load1",
		"minio_node_cpu_avg_load1_avg",
		"minio_node_cpu_avg_load1_max",
		"minio_node_cpu_avg_load1_perc",
		"minio_node_cpu_avg_load1_perc_avg",
		"minio_node_cpu_avg_load1_perc_max",
		"minio_node_cpu_avg_load5",
		"minio_node_cpu_avg_load5_avg",
		"minio_node_cpu_avg_load5_max",
		"minio_node_cpu_avg_load5_perc",
		"minio_node_cpu_avg_load5_perc_avg",
		"minio_node_cpu_avg_load5_perc_max",
		"minio_node_cpu_avg_load15",
		"minio_node_cpu_avg_load15_avg",
		"minio_node_cpu_avg_load15_max",
		"minio_node_cpu_avg_load15_perc",
		"minio_node_cpu_avg_load15_perc_avg",
		"minio_node_cpu_avg_load15_perc_max",
		"minio_node_mem_available",
		"minio_node_mem_available_avg",
		"minio_node_mem_available_max",
		"minio_node_mem_buffers",
		"minio_node_mem_buffers_avg",
		"minio_node_mem_buffers_max",
		"minio_node_mem_cache",
		"minio_node_mem_cache_avg",
		"minio_node_mem_cache_max",
		"minio_node_mem_free",
		"minio_node_mem_free_avg",
		"minio_node_mem_free_max",
		"minio_node_mem_shared",
		"minio_node_mem_shared_avg",
		"minio_node_mem_shared_max",
		"minio_node_mem_total",
		"minio_node_mem_total_avg",
		"minio_node_mem_total_max",
		"minio_node_mem_used",
		"minio_node_mem_used_avg",
		"minio_node_mem_used_max",
		"minio_node_mem_used_perc",
		"minio_node_mem_used_perc_avg",
		"minio_node_mem_used_perc_max",
	}
)
