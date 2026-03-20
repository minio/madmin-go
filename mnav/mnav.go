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
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/minio/madmin-go/v4"
)

// MetricNavigator provides navigation functionality
type MetricNavigator interface {
	Navigate(path string) (MetricNode, error)
	Root() MetricNode
}

// MetricNode interface for navigation
type MetricNode interface {
	// GetChildren returns a list of navigable children of current node
	GetChildren() []MetricChild

	// GetLeafData returns a map of leaf data for current node.
	// Leaf data is data that is not navigable, such as a summary of the current node.
	// Data may have sort keys, which is 'nn:Key' where 'nn' is the numeric order of the key.
	GetLeafData() map[string]string

	// GetMetricType returns the metric type required for the current node data.
	GetMetricType() madmin.MetricType

	// GetMetricFlags returns the metric flags required for the current node data.
	GetMetricFlags() madmin.MetricFlags

	// GetParent returns the parent node of the current node as set when creating the node.
	GetParent() MetricNode

	// GetPath returns the path of the current node relative to the root node as set when creating the node.
	GetPath() string

	// GetChild returns the child node with the given name.
	GetChild(name string) (MetricNode, error)

	// ShouldPauseRefresh returns true if the node should be paused from refreshing.
	// This will be enabled when data isn't expected to be updated for a while.
	ShouldPauseRefresh() bool

	// GetOpts returns the metrics options for the current node.
	// This includes all parent nodes.
	GetOpts() madmin.MetricsOptions
}

// MetricChild represents a navigable child
type MetricChild struct {
	Name        string // Navigation key (path-safe, may be URL-encoded)
	DisplayName string // Human-readable name for display (optional, defaults to Name)
	Description string
}

// GetDisplayName returns the display name, falling back to Name if DisplayName is empty
func (c MetricChild) GetDisplayName() string {
	if c.DisplayName != "" {
		return c.DisplayName
	}
	return c.Name
}

// RealtimeMetricsNavigator implements MetricNavigator for RealtimeMetrics
type RealtimeMetricsNavigator struct {
	metrics *madmin.RealtimeMetrics
}

// NewRealtimeMetricsNavigator creates a new navigator for RealtimeMetrics
func NewRealtimeMetricsNavigator(metrics *madmin.RealtimeMetrics) MetricNavigator {
	return &RealtimeMetricsNavigator{metrics: metrics}
}

// Navigate to a path and return the node at that location
func (nav *RealtimeMetricsNavigator) Navigate(path string) (MetricNode, error) {
	if path == "" || path == "/" {
		return nav.Root(), nil
	}

	// Split path and navigate
	parts := strings.Split(strings.Trim(path, "/"), "/")
	node := nav.Root()

	for _, part := range parts {
		if part == "" {
			continue
		}
		var err error
		node, err = node.GetChild(part)
		if err != nil {
			return nil, fmt.Errorf("path not found: %s", path)
		}
	}

	return node, nil
}

// Root returns the root node
func (nav *RealtimeMetricsNavigator) Root() MetricNode {
	return &RealtimeMetricsNode{metrics: nav.metrics}
}

// RealtimeMetricsNode represents the root node of RealtimeMetrics
type RealtimeMetricsNode struct {
	metrics *madmin.RealtimeMetrics
}

func getNodeOpts(node MetricNode) madmin.MetricsOptions {
	var opts madmin.MetricsOptions
	opts.Type = node.GetMetricType()
	opts.Flags = node.GetMetricFlags()
	parent := node.GetParent()
	if parent != nil {
		// This will also fetch from parent.
		pOpts := parent.GetOpts()
		opts.Type |= pOpts.Type
		opts.Flags |= pOpts.Flags
		opts.Hosts = append(opts.Hosts, pOpts.Hosts...)
		opts.Disks = append(opts.Disks, pOpts.Disks...)
		opts.DrivePoolIdx = append(opts.DrivePoolIdx, pOpts.DrivePoolIdx...)
		opts.DriveSetIdx = append(opts.DriveSetIdx, pOpts.DriveSetIdx...)
	}
	return opts
}

func (node *RealtimeMetricsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *RealtimeMetricsNode) ShouldPauseUpdates() bool {
	// Legacy method - not used in interface, return false for default behavior
	return false
}

func (node *RealtimeMetricsNode) GetChildren() []MetricChild {
	return []MetricChild{
		{Name: "api", Description: "API operation metrics"},
		{Name: "drive", Description: "Drive usage and performance metrics"},
		{Name: "rpc", Description: "RPC call statistics"},
		{Name: "net", Description: "Network interface metrics"},
		{Name: "os", Description: "Operating system metrics"},
		{Name: "cpu", Description: "CPU usage and performance metrics"},
		{Name: "mem", Description: "Memory usage metrics"},
		{Name: "go", Description: "Go runtime metrics"},
		{Name: "process", Description: "Process-level system metrics"},
		{Name: "replication", Description: "Replication metrics"},
		{Name: "scanner", Description: "Scanner-related metrics"},
		{Name: "batch_jobs", Description: "Batch job execution metrics"},
		{Name: "site_resync", Description: "Site replication resync metrics"},
		{Name: "by_host", Description: "Metrics broken down by individual host"},
		{Name: "by_drive", Description: "Metrics broken down by individual drive"},
		{Name: "by_drive_set", Description: "Metrics broken down by drive set"},
	}
}

func (node *RealtimeMetricsNode) GetLeafData() map[string]string {
	data := map[string]string{
		"Collection Status": map[bool]string{true: "Complete", false: "Partial"}[node.metrics.Final],
		"Active Hosts":      strconv.Itoa(len(node.metrics.Hosts)),
	}

	if len(node.metrics.Errors) > 0 {
		data["Collection Errors"] = strconv.Itoa(len(node.metrics.Errors))
	}

	// Show error summary (limited to first 3 errors)
	for i, err := range node.metrics.Errors {
		if i < 3 {
			data[fmt.Sprintf("Error %d", i+1)] = err
		}
	}

	return data
}

func (node *RealtimeMetricsNode) GetMetricType() madmin.MetricType {
	return madmin.MetricsNone // All types available at root
}

func (node *RealtimeMetricsNode) GetMetricFlags() madmin.MetricFlags {
	return 0
}

func (node *RealtimeMetricsNode) GetParent() MetricNode {
	return nil // Root node has no parent
}

func (node *RealtimeMetricsNode) GetPath() string {
	return "/"
}

func (node *RealtimeMetricsNode) ShouldPauseRefresh() bool {
	return false // Default behavior - don't pause refresh
}

func (node *RealtimeMetricsNode) GetChild(name string) (MetricNode, error) {
	switch name {
	// Individual metric types - route directly from root
	case "scanner":
		return NewScannerMetricsNode(node.metrics.Aggregated.Scanner, node, "scanner"), nil
	case "drive":
		return NewDiskMetricsNavigator(node.metrics.Aggregated.Disk, node, "drive", madmin.MetricsOptions{}), nil
	case "os":
		return NewOSMetricsNavigator(node.metrics.Aggregated.OS, node, "os"), nil
	case "batch_jobs":
		return NewBatchJobMetricsNode(node.metrics.Aggregated.BatchJobs, node, "batch_jobs"), nil
	case "site_resync":
		return NewSiteResyncMetricsNode(node.metrics.Aggregated.SiteResync, node, "site_resync"), nil
	case "net":
		return NewNetMetricsNavigator(node.metrics.Aggregated.Net, node, "net"), nil
	case "mem":
		return NewMemMetricsNavigator(node.metrics.Aggregated.Mem, node, "mem"), nil
	case "cpu":
		return NewCPUMetricsNavigator(node.metrics.Aggregated.CPU, node, "cpu"), nil
	case "rpc":
		return &RPCMetricsNode{rpc: node.metrics.Aggregated.RPC, parent: node, path: "rpc"}, nil
	case "go":
		return NewRuntimeMetricsNavigator(node.metrics.Aggregated.Go, node, "go"), nil
	case "api":
		return &APIMetricsNode{api: node.metrics.Aggregated.API, parent: node, path: "api"}, nil
	case "replication":
		return NewReplicationMetricsNode(node.metrics.Aggregated.Replication, node, "replication"), nil
	case "process":
		return NewProcessMetricsNode(node.metrics.Aggregated.Process, node, "process"), nil

	// Grouping nodes - preserved as-is
	case "by_host":
		mapNode := &MapNode{
			data:        node.metrics.ByHost,
			metricType:  madmin.MetricsNone,
			metricFlags: madmin.MetricsByHost,
			parent:      node,
			path:        "by_host",
		}
		mapNode.nodeFactory = func(key string, value interface{}) MetricNode {
			if metrics, ok := value.(madmin.Metrics); ok {
				return &MetricsNode{
					metrics: &metrics, parent: mapNode, path: fmt.Sprintf("by_host/%s", key),
					opts: madmin.MetricsOptions{Type: madmin.MetricsNone, Flags: madmin.MetricsByHost, Hosts: []string{key}},
				}
			}
			return nil
		}
		return mapNode, nil
	case "by_drive":
		mapNode := &MapNode{
			data:        node.metrics.ByDisk,
			metricType:  madmin.MetricsDisk,
			metricFlags: madmin.MetricsByDisk,
			parent:      node,
			path:        "by_drive",
		}
		mapNode.nodeFactory = func(key string, value interface{}) MetricNode {
			if diskMetric, ok := value.(madmin.DiskMetric); ok {
				// URL-encode the disk name to handle slashes and special characters
				encodedKey := url.PathEscape(key)
				return NewDiskMetricsNavigator(&diskMetric, mapNode, fmt.Sprintf("by_drive/%s", encodedKey), madmin.MetricsOptions{Flags: madmin.MetricsByDisk, Disks: []string{key}})
			}
			return nil
		}
		return mapNode, nil
	case "by_drive_set":
		return &DiskSetMapNode{
			data:        node.metrics.ByDiskSet,
			metricType:  madmin.MetricsDisk,
			metricFlags: madmin.MetricsByDiskSet,
			parent:      node,
			path:        "by_drive_set",
		}, nil
	default:
		return nil, fmt.Errorf("child not found: %s", name)
	}
}

// MetricsNode handles navigation within a Metrics struct
type MetricsNode struct {
	metrics *madmin.Metrics
	parent  MetricNode
	path    string
	opts    madmin.MetricsOptions
}

func (node *MetricsNode) GetOpts() madmin.MetricsOptions {
	return node.opts
}

func (node *MetricsNode) ShouldPauseUpdates() bool {
	return false
}

func (node *MetricsNode) GetChildren() []MetricChild {
	return []MetricChild{
		{Name: "api", Description: "API operation metrics"},
		{Name: "drive", Description: "Drive usage and performance metrics"},
		{Name: "rpc", Description: "RPC call statistics"},
		{Name: "net", Description: "Network interface metrics"},
		{Name: "os", Description: "Operating system metrics"},
		{Name: "cpu", Description: "CPU usage and performance metrics"},
		{Name: "mem", Description: "Memory usage metrics"},
		{Name: "go", Description: "Go runtime metrics"},
		{Name: "process", Description: "Process-level system metrics"},
		{Name: "replication", Description: "Replication metrics"},
		{Name: "scanner", Description: "Scanner-related metrics"},
		{Name: "batch_jobs", Description: "Batch job execution metrics"},
		{Name: "site_resync", Description: "Site replication resync metrics"},
	}
}

func (node *MetricsNode) GetLeafData() map[string]string {
	return nil // MetricsNode is a navigation node, not a leaf
}

func (node *MetricsNode) GetMetricType() madmin.MetricType {
	return node.opts.Type
}

func (node *MetricsNode) GetMetricFlags() madmin.MetricFlags {
	return node.opts.Flags
}

func (node *MetricsNode) GetParent() MetricNode {
	return node.parent
}

func (node *MetricsNode) GetPath() string {
	return node.path
}

func (node *MetricsNode) ShouldPauseRefresh() bool {
	return false
}

func (node *MetricsNode) GetChild(name string) (MetricNode, error) {
	switch name {
	case "scanner":
		return NewScannerMetricsNode(node.metrics.Scanner, node, fmt.Sprintf("%s/scanner", node.path)), nil
	case "drive":
		return NewDiskMetricsNavigator(node.metrics.Disk, node, fmt.Sprintf("%s/drive", node.path), madmin.MetricsOptions{}), nil
	case "os":
		return NewOSMetricsNavigator(node.metrics.OS, node, fmt.Sprintf("%s/os", node.path)), nil
	case "batch_jobs":
		return NewBatchJobMetricsNode(node.metrics.BatchJobs, node, fmt.Sprintf("%s/batch_jobs", node.path)), nil
	case "site_resync":
		return NewSiteResyncMetricsNode(node.metrics.SiteResync, node, fmt.Sprintf("%s/site_resync", node.path)), nil
	case "net":
		return NewNetMetricsNavigator(node.metrics.Net, node, fmt.Sprintf("%s/net", node.GetPath())), nil
	case "mem":
		return NewMemMetricsNavigator(node.metrics.Mem, node, fmt.Sprintf("%s/mem", node.path)), nil
	case "cpu":
		return NewCPUMetricsNavigator(node.metrics.CPU, node, fmt.Sprintf("%s/cpu", node.path)), nil
	case "rpc":
		return &RPCMetricsNode{rpc: node.metrics.RPC, parent: node, path: fmt.Sprintf("%s/rpc", node.path)}, nil
	case "go":
		return NewRuntimeMetricsNavigator(node.metrics.Go, node, fmt.Sprintf("%s/go", node.path)), nil
	case "api":
		return &APIMetricsNode{api: node.metrics.API, parent: node, path: fmt.Sprintf("%s/api", node.path)}, nil
	case "replication":
		return NewReplicationMetricsNode(node.metrics.Replication, node, fmt.Sprintf("%s/replication", node.path)), nil
	case "process":
		return NewProcessMetricsNode(node.metrics.Process, node, fmt.Sprintf("%s/process", node.path)), nil
	default:
		return nil, fmt.Errorf("child not found: %s", name)
	}
}

// MapNode handles dynamic map-based navigation
type MapNode struct {
	data        interface{}
	metricType  madmin.MetricType
	metricFlags madmin.MetricFlags
	parent      MetricNode
	path        string
	nodeFactory func(key string, value interface{}) MetricNode
}

func (node *MapNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *MapNode) ShouldPauseUpdates() bool {
	// Legacy method - not used in interface, return false for default behavior
	return false
}

func (node *MapNode) GetChildren() []MetricChild {
	switch data := node.data.(type) {
	case map[string]madmin.Metrics:
		children := make([]MetricChild, 0, len(data))
		keys := make([]string, 0, len(data))
		// Extract and sort keys
		for k := range data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		// Create children in sorted order
		for _, k := range keys {
			childName := k
			displayName := k
			// For by_drive nodes, URL-encode the name to handle slashes and special characters
			if strings.Contains(node.path, "by_drive") {
				childName = url.PathEscape(k)
				// Keep original name for display
			}
			children = append(children, MetricChild{
				Name:        childName,
				DisplayName: displayName,
				Description: fmt.Sprintf("Metrics for %s", k),
			})
		}
		return children
	case map[string]madmin.DiskMetric:
		var children []MetricChild
		var keys []string
		// Extract and sort keys
		for k := range data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		// Create children in sorted order
		for _, k := range keys {
			childName := k
			displayName := k
			// For by_drive nodes, URL-encode the name to handle slashes and special characters
			if strings.Contains(node.path, "by_drive") {
				childName = url.PathEscape(k)
				// Keep original name for display
			}
			disk := data[k]
			children = append(children, MetricChild{
				Name:        childName,
				DisplayName: displayName,
				Description: fmt.Sprintf("%d queued; r:%d w:%d d:%d f:%d IO per minute", disk.IOStatsMinute.CurrentIOs, disk.IOStatsMinute.ReadIOs, disk.IOStatsMinute.WriteIOs, disk.IOStatsMinute.DiscardIOs, disk.IOStatsMinute.FlushIOs),
			})
		}
		return children
	case map[int]map[int]madmin.DiskMetric:
		var children []MetricChild
		var keys []int
		// Extract and sort keys
		for k := range data {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		// Create children in sorted order
		for _, k := range keys {
			children = append(children, MetricChild{Name: fmt.Sprintf("%d", k), Description: fmt.Sprintf("Drive set %d", k)})
		}
		return children
	default:
		return []MetricChild{}
	}
}

func (node *MapNode) GetLeafData() map[string]string {
	// Return empty data - no information displayed for by_host/by_disk navigation
	return map[string]string{}
}

func (node *MapNode) GetPath() string {
	return node.path
}

func (node *MapNode) ShouldPauseRefresh() bool {
	return false
}

func (node *MapNode) GetMetricType() madmin.MetricType {
	return node.metricType
}

func (node *MapNode) GetMetricFlags() madmin.MetricFlags {
	return node.metricFlags
}

func (node *MapNode) GetParent() MetricNode {
	return node.parent
}

func (node *MapNode) GetChild(name string) (MetricNode, error) {
	switch data := node.data.(type) {
	case map[string]madmin.Metrics:
		// For by_drive nodes, URL-decode the name to get the original disk name
		decodedName := name
		if strings.Contains(node.path, "by_drive") {
			if decoded, err := url.PathUnescape(name); err == nil {
				decodedName = decoded
			}
		}

		if value, exists := data[decodedName]; exists {
			return node.nodeFactory(decodedName, value), nil
		}
	case map[string]madmin.DiskMetric:
		// For by_drive nodes, URL-decode the name to get the original disk name
		decodedName := name
		if strings.Contains(node.path, "by_drive") {
			if decoded, err := url.PathUnescape(name); err == nil {
				decodedName = decoded
			}
		}

		if value, exists := data[decodedName]; exists {
			return node.nodeFactory(decodedName, value), nil
		}
	case map[int]map[int]madmin.DiskMetric:
		// This is handled by DiskSetMapNode
		return nil, fmt.Errorf("use DiskSetMapNode for nested drive set maps")
	}
	return nil, fmt.Errorf("key not found: %s", name)
}

// DiskSetMapNode handles the nested map structure for ByDiskSet
type DiskSetMapNode struct {
	data        map[int]map[int]madmin.DiskMetric
	metricType  madmin.MetricType
	metricFlags madmin.MetricFlags
	parent      MetricNode
	path        string
}

func (node *DiskSetMapNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *DiskSetMapNode) ShouldPauseUpdates() bool {
	// Legacy method - not used in interface, return false for default behavior
	return false
}

func (node *DiskSetMapNode) GetChildren() []MetricChild {
	children := make([]MetricChild, 0, len(node.data))
	for poolID, pool := range node.data {
		// Calculate pool-level statistics for better description
		var poolDisks int
		poolSets := len(pool)
		var poolHealthyDisks int
		var poolCurrentIOs uint64
		for _, diskSet := range pool {
			poolDisks += diskSet.NDisks
			poolHealthyDisks += (diskSet.NDisks - diskSet.Offline - diskSet.Hanging - diskSet.Healing)
			poolCurrentIOs += diskSet.IOStatsMinute.CurrentIOs
		}

		description := fmt.Sprintf("Pool %d with %d sets, %d drives (%d healthy), %d current IOs",
			poolID, poolSets, poolDisks, poolHealthyDisks, poolCurrentIOs)

		children = append(children, MetricChild{
			Name:        fmt.Sprintf("pool_%d", poolID),
			Description: description,
		})
	}
	return children
}

func (node *DiskSetMapNode) GetLeafData() map[string]string {
	data := map[string]string{}

	// Calculate aggregated statistics across all pools and sets
	var totalSets int
	var totalDisks int
	var totalHealthyDisks int
	var totalOfflineDisks int
	var totalHealingDisks int
	var totalHangingDisks int
	var totalCapacity, totalUsed uint64
	var totalOps uint64
	var totalBytes uint64

	// First pass: calculate pool-level aggregated metrics
	poolMetrics := make(map[int]struct {
		sets    int
		ops     uint64
		accTime float64
		ioOps   uint64
		ioBytes uint64
	})

	for poolID, pool := range node.data {
		poolStat := poolMetrics[poolID]
		for setID, diskSet := range pool {
			totalSets++
			totalDisks += diskSet.NDisks
			totalHealthyDisks += (diskSet.NDisks - diskSet.Offline - diskSet.Hanging - diskSet.Healing)
			totalOfflineDisks += diskSet.Offline
			totalHealingDisks += diskSet.Healing
			totalHangingDisks += diskSet.Hanging

			// Aggregate storage space
			totalCapacity += diskSet.Space.Free.Total + diskSet.Space.Used.Total
			totalUsed += diskSet.Space.Used.Total

			// Aggregate operations from last minute for cluster totals
			for _, action := range diskSet.LastMinute {
				totalOps += action.Count
				totalBytes += action.Bytes
				poolStat.ops += action.Count
				poolStat.accTime += action.AccTime
			}

			// Aggregate pool-level metrics
			poolStat.sets++

			// Aggregate current IO statistics for this pool
			ioStat := diskSet.IOStatsMinute
			poolStat.ioOps += ioStat.ReadIOs + ioStat.WriteIOs + ioStat.DiscardIOs + ioStat.FlushIOs
			poolStat.ioBytes += ioStat.ReadSectors + ioStat.WriteSectors + ioStat.DiscardSectors // Sectors represent data transferred

			_ = setID // Mark as used
		}
		poolMetrics[poolID] = poolStat
	}

	// Second pass: create pool-level display entries
	for poolID, poolStat := range poolMetrics {
		var opsDisplay string
		var ioDisplay string

		// Calculate performance metrics for this pool
		if poolStat.ops > 0 && poolStat.accTime > 0 {
			opsPerSec := float64(poolStat.ops) / poolStat.accTime
			avgTimeMs := (poolStat.accTime / float64(poolStat.ops)) * 1000
			opsDisplay = fmt.Sprintf("%.1f ops/s, %.2fms avg", opsPerSec, avgTimeMs)
		} else {
			opsDisplay = "No recent activity"
		}

		// Calculate current IO metrics for this pool
		if poolStat.ioOps > 0 || poolStat.ioBytes > 0 {
			ioDisplay = fmt.Sprintf(", %s IO/s", humanize.Bytes(poolStat.ioBytes))
		} else {
			ioDisplay = ", No current IO"
		}

		poolLabel := fmt.Sprintf("Pool %d", poolID)
		poolValue := fmt.Sprintf("%s%s (%d sets)", opsDisplay, ioDisplay, poolStat.sets)
		data[poolLabel] = poolValue
	}

	// Summary statistics
	data["00:Cluster Summary"] = fmt.Sprintf("%d pools, %d sets, %d total drives",
		len(node.data), totalSets, totalDisks)

	if totalDisks > 0 {
		healthPercent := float64(totalHealthyDisks) / float64(totalDisks) * 100.0
		var healthStatus string
		switch {
		case healthPercent >= 95:
			healthStatus = "Excellent"
		case healthPercent >= 85:
			healthStatus = "Good"
		case healthPercent >= 70:
			healthStatus = "Warning"
		default:
			healthStatus = "Critical"
		}

		data["01:Drive Health"] = fmt.Sprintf("%s - %d of %d drives healthy (%.1f%%)",
			healthStatus, totalHealthyDisks, totalDisks, healthPercent)

		if totalOfflineDisks > 0 || totalHangingDisks > 0 || totalHealingDisks > 0 {
			var issues []string
			if totalOfflineDisks > 0 {
				issues = append(issues, fmt.Sprintf("%d offline", totalOfflineDisks))
			}
			if totalHangingDisks > 0 {
				issues = append(issues, fmt.Sprintf("%d hanging", totalHangingDisks))
			}
			if totalHealingDisks > 0 {
				issues = append(issues, fmt.Sprintf("%d healing", totalHealingDisks))
			}
			data["02:Issues"] = strings.Join(issues, ", ")
		}
	}

	// Storage capacity summary
	if totalCapacity > 0 {
		usagePercent := float64(totalUsed) / float64(totalCapacity) * 100.0
		data["03:Storage Capacity"] = fmt.Sprintf("%s used of %s total (%.1f%% used)",
			humanize.Bytes(totalUsed), humanize.Bytes(totalCapacity), usagePercent)
	}

	// Activity summary
	if totalOps > 0 {
		data["03:Recent Activity"] = fmt.Sprintf("%s operations, %s transferred (last minute)",
			humanize.Comma(int64(totalOps)), humanize.Bytes(totalBytes))
	}

	return data
}

func (node *DiskSetMapNode) GetMetricType() madmin.MetricType {
	return node.metricType
}

func (node *DiskSetMapNode) GetMetricFlags() madmin.MetricFlags {
	return node.metricFlags
}

func (node *DiskSetMapNode) GetParent() MetricNode {
	return node.parent
}

func (node *DiskSetMapNode) GetPath() string {
	return node.path
}

func (node *DiskSetMapNode) ShouldPauseRefresh() bool {
	return false
}

func (node *DiskSetMapNode) GetChild(name string) (MetricNode, error) {
	if !strings.HasPrefix(name, "pool_") {
		return nil, fmt.Errorf("invalid pool name format: %s", name)
	}

	poolIDStr := strings.TrimPrefix(name, "pool_")
	var poolID int
	if _, err := fmt.Sscanf(poolIDStr, "%d", &poolID); err != nil {
		return nil, fmt.Errorf("invalid pool ID: %s", poolIDStr)
	}

	if sets, exists := node.data[poolID]; exists {
		return NewDiskSetPoolNavigator(poolID, sets, node.metricType, node.metricFlags, node, fmt.Sprintf("%s/pool_%d", node.path, poolID)), nil
	}

	return nil, fmt.Errorf("pool not found: %d", poolID)
}

// DiskSetPoolNavigator provides enhanced navigation for disk set pools
type DiskSetPoolNavigator struct {
	poolID      int
	poolSets    map[int]madmin.DiskMetric
	metricType  madmin.MetricType
	metricFlags madmin.MetricFlags
	parent      MetricNode
	path        string
}

func NewDiskSetPoolNavigator(poolID int, poolSets map[int]madmin.DiskMetric, metricType madmin.MetricType, metricFlags madmin.MetricFlags, parent MetricNode, path string) *DiskSetPoolNavigator {
	return &DiskSetPoolNavigator{
		poolID:      poolID,
		poolSets:    poolSets,
		metricType:  metricType,
		metricFlags: metricFlags,
		parent:      parent,
		path:        path,
	}
}

func (node *DiskSetPoolNavigator) ShouldPauseUpdates() bool {
	return false
}

func (node *DiskSetPoolNavigator) GetChildren() []MetricChild {
	if node.poolSets == nil {
		return []MetricChild{}
	}
	children := make([]MetricChild, 0, len(node.poolSets))
	for setID, diskSet := range node.poolSets {
		healthyDisks := diskSet.NDisks - diskSet.Offline - diskSet.Hanging - diskSet.Healing
		currentIOs := diskSet.IOStatsMinute.CurrentIOs
		description := fmt.Sprintf("Set %d with %d drives (%d healthy), %d current IOs",
			setID, diskSet.NDisks, healthyDisks, currentIOs)

		children = append(children, MetricChild{
			Name:        fmt.Sprintf("set_%d", setID),
			Description: description,
		})
	}

	// Sort children by set ID for consistent ordering
	sort.Slice(children, func(i, j int) bool {
		return children[i].Name < children[j].Name
	})

	return children
}

func (node *DiskSetPoolNavigator) GetLeafData() map[string]string {
	data := map[string]string{}

	// Pool-level aggregated statistics
	totalSets := len(node.poolSets)
	var totalDisks int
	var totalHealthyDisks int
	var totalOfflineDisks int
	var totalHealingDisks int
	var totalHangingDisks int
	var totalCapacity, totalUsed uint64
	var totalOps uint64
	var totalBytes uint64

	for setID, diskSet := range node.poolSets {
		totalDisks += diskSet.NDisks
		totalHealthyDisks += (diskSet.NDisks - diskSet.Offline - diskSet.Hanging - diskSet.Healing)
		totalOfflineDisks += diskSet.Offline
		totalHealingDisks += diskSet.Healing
		totalHangingDisks += diskSet.Hanging

		// Aggregate storage space
		totalCapacity += diskSet.Space.Free.Total + diskSet.Space.Used.Total
		totalUsed += diskSet.Space.Used.Total

		// Aggregate operations from last minute
		for _, action := range diskSet.LastMinute {
			totalOps += action.Count
			totalBytes += action.Bytes
		}

		// Individual set performance metrics
		var setOps uint64
		var setTime float64
		for _, action := range diskSet.LastMinute {
			setOps += action.Count
			setTime += action.AccTime
		}

		var opsDisplay string
		if setOps > 0 && setTime > 0 {
			opsPerSec := float64(setOps) / setTime
			avgTimeMs := (setTime / float64(setOps)) * 1000 // Convert to milliseconds
			opsDisplay = fmt.Sprintf("%.1f ops/s, %.2fms avg time", opsPerSec, avgTimeMs)
		} else {
			opsDisplay = "No recent activity"
		}

		data[fmt.Sprintf("Set %d", setID)] = opsDisplay
	}

	// Pool summary
	data["00:Pool Summary"] = fmt.Sprintf("Pool %d: %d sets, %d total drives",
		node.poolID, totalSets, totalDisks)

	if totalDisks > 0 {
		healthPercent := float64(totalHealthyDisks) / float64(totalDisks) * 100.0
		data["01:Pool Health"] = fmt.Sprintf("%d of %d drives healthy (%.1f%%)",
			totalHealthyDisks, totalDisks, healthPercent)

		if totalOfflineDisks > 0 || totalHangingDisks > 0 || totalHealingDisks > 0 {
			var issues []string
			if totalOfflineDisks > 0 {
				issues = append(issues, fmt.Sprintf("%d offline", totalOfflineDisks))
			}
			if totalHangingDisks > 0 {
				issues = append(issues, fmt.Sprintf("%d hanging", totalHangingDisks))
			}
			if totalHealingDisks > 0 {
				issues = append(issues, fmt.Sprintf("%d healing", totalHealingDisks))
			}
			data["02:Pool Issues"] = strings.Join(issues, ", ")
		}
	}

	// Pool storage capacity
	if totalCapacity > 0 {
		usagePercent := float64(totalUsed) / float64(totalCapacity) * 100.0
		data["03:Pool Storage"] = fmt.Sprintf("%s used of %s total (%.1f%% used)",
			humanize.Bytes(totalUsed), humanize.Bytes(totalCapacity), usagePercent)
	}

	// Pool activity summary
	if totalOps > 0 {
		data["04:Pool Activity"] = fmt.Sprintf("%s operations, %s transferred (last minute)",
			humanize.Comma(int64(totalOps)), humanize.Bytes(totalBytes))
	}

	return data
}

func (node *DiskSetPoolNavigator) GetMetricType() madmin.MetricType   { return node.metricType }
func (node *DiskSetPoolNavigator) GetMetricFlags() madmin.MetricFlags { return node.metricFlags }
func (node *DiskSetPoolNavigator) GetParent() MetricNode              { return node.parent }
func (node *DiskSetPoolNavigator) GetPath() string                    { return node.path }
func (node *DiskSetPoolNavigator) ShouldPauseRefresh() bool           { return false }

func (node *DiskSetPoolNavigator) GetChild(name string) (MetricNode, error) {
	if !strings.HasPrefix(name, "set_") {
		return nil, fmt.Errorf("invalid set name format: %s", name)
	}

	setIDStr := strings.TrimPrefix(name, "set_")
	var setID int
	if _, err := fmt.Sscanf(setIDStr, "%d", &setID); err != nil {
		return nil, fmt.Errorf("invalid set ID: %s", setIDStr)
	}

	if diskMetric, exists := node.poolSets[setID]; exists {
		opts := getNodeOpts(node)
		opts.DriveSetIdx = append(opts.DriveSetIdx, setID)
		return NewDiskMetricsNavigator(&diskMetric, node, fmt.Sprintf("%s/set_%d", node.path, setID), opts), nil
	}

	return nil, fmt.Errorf("set not found: %d", setID)
}

func (node *DiskSetPoolNavigator) GetOpts() madmin.MetricsOptions {
	opts := getNodeOpts(node)
	opts.PoolIdx = append(opts.PoolIdx, node.poolID)
	return opts
}
