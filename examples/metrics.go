//go:build ignore

/*
 * MinIO Metrics Collection Example
 *
 * This example demonstrates how to use the AdminClient.Metrics() function to collect
 * real-time metrics from MinIO servers. It supports various metric types including
 * memory, CPU, disk, network, process, and OS sensors.
 *
 * Features:
 * - Command-line configuration for endpoint, credentials, and collection parameters
 * - Support for multiple metric types (mem, cpu, disk, net, process, os)
 * - JSON or human-readable output formats
 * - Ability to target specific hosts in a cluster
 * - Configurable collection duration and intervals
 * - Per-host breakdown in verbose mode
 *
 * Usage Examples:
 *   go run metrics.go -endpoint localhost:9000 -access-key admin -secret-key password
 *   go run metrics.go -types mem,cpu,process -duration 30s -json
 *   go run metrics.go -hosts server1:9000,server2:9000 -v
 *
 * The example showcases the new ProcessMetrics feature that aggregates process
 * information including CPU usage, memory consumption, I/O stats, and more.
 */

//
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
//

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/minio/madmin-go/v4"
)

// Command line flags
var (
	endpoint     = flag.String("endpoint", "localhost:9000", "MinIO server endpoint")
	accessKey    = flag.String("access-key", "", "MinIO access key")
	secretKey    = flag.String("secret-key", "", "MinIO secret key")
	useSSL       = flag.Bool("ssl", false, "Use HTTPS connection")
	insecure     = flag.Bool("insecure", false, "Skip SSL certificate verification")
	interval     = flag.Duration("interval", 1*time.Second, "Interval between metric collections")
	metricType   = flag.String("types", "mem,cpu", "Comma-separated metric types: all,mem,cpu,disk,net,process,os,scanner,batchjobs,siteresync,rpc,runtime,api,replication")
	hosts        = flag.String("hosts", "", "Comma-separated list of specific hosts (optional)")
	disks        = flag.String("disks", "", "Comma-separated list of specific disks (optional)")
	pools        = flag.String("pools", "", "Comma-separated pool indices (optional)")
	diskSets     = flag.String("disk-sets", "", "Comma-separated disk set indices (optional)")
	jsonOutput   = flag.Bool("json", false, "Output in JSON format")
	verbose      = flag.Bool("v", false, "Verbose output")
	byHost       = flag.Bool("by-host", false, "Return individual metrics by host")
	byDisk       = flag.Bool("by-disk", false, "Return individual metrics by disk")
	dailyStats   = flag.Bool("daily-stats", false, "Include daily/historic statistics")
	legacyDiskIO = flag.Bool("legacy-disk-io", false, "Add legacy disk I/O metrics")
	maxSamples   = flag.Int("max-samples", 0, "Maximum number of samples (0 for unlimited)")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nCollect and display MinIO server metrics with comprehensive options.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Basic usage with memory and CPU metrics\n")
		fmt.Fprintf(os.Stderr, "  %s -endpoint play.min.io -access-key minioadmin -secret-key minioadmin\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Collect all metric types for 30 seconds\n")
		fmt.Fprintf(os.Stderr, "  %s -types all -duration 30s\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # JSON output with specific hosts and by-host breakdown\n")
		fmt.Fprintf(os.Stderr, "  %s -hosts server1:9000,server2:9000 -by-host -json\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Monitor specific disk with daily statistics\n")
		fmt.Fprintf(os.Stderr, "  %s -types disk -disks /dev/sda1 -daily-stats -by-disk\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # API and RPC metrics with pool filtering\n")
		fmt.Fprintf(os.Stderr, "  %s -types api,rpc -pools 0,1 -max-samples 5\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Metric Types:\n")
		fmt.Fprintf(os.Stderr, "  all         - All available metrics\n")
		fmt.Fprintf(os.Stderr, "  mem         - Memory usage metrics\n")
		fmt.Fprintf(os.Stderr, "  cpu         - CPU utilization metrics\n")
		fmt.Fprintf(os.Stderr, "  disk        - Disk I/O and usage metrics\n")
		fmt.Fprintf(os.Stderr, "  net         - Network interface metrics\n")
		fmt.Fprintf(os.Stderr, "  process     - Process metrics\n")
		fmt.Fprintf(os.Stderr, "  os          - OS metrics including sensors\n")
		fmt.Fprintf(os.Stderr, "  scanner     - Scanner operation metrics\n")
		fmt.Fprintf(os.Stderr, "  batchjobs   - Batch job metrics\n")
		fmt.Fprintf(os.Stderr, "  siteresync  - Site replication sync metrics\n")
		fmt.Fprintf(os.Stderr, "  rpc         - RPC call metrics\n")
		fmt.Fprintf(os.Stderr, "  runtime     - Go runtime metrics\n")
		fmt.Fprintf(os.Stderr, "  api         - API operation metrics\n")
		fmt.Fprintf(os.Stderr, "  replication - Replication metrics\n")
		fmt.Fprintf(os.Stderr, "\nFlags:\n")
		fmt.Fprintf(os.Stderr, "  -by-host      - Return individual metrics by host\n")
		fmt.Fprintf(os.Stderr, "  -by-disk      - Return individual metrics by disk\n")
		fmt.Fprintf(os.Stderr, "  -daily-stats  - Include daily/historic statistics\n")
		fmt.Fprintf(os.Stderr, "  -legacy-disk-io - Add legacy disk I/O metrics\n")
	}

	flag.Parse()

	// Get credentials from environment if not provided via flags
	if *accessKey == "" {
		*accessKey = os.Getenv("MINIO_ACCESS_KEY")
		if *accessKey == "" {
			*accessKey = os.Getenv("MINIO_ROOT_USER")
		}
	}
	if *secretKey == "" {
		*secretKey = os.Getenv("MINIO_SECRET_KEY")
		if *secretKey == "" {
			*secretKey = os.Getenv("MINIO_ROOT_PASSWORD")
		}
	}

	if *accessKey == "" || *secretKey == "" {
		fmt.Fprintf(os.Stderr, "Error: Access key and secret key are required\n")
		fmt.Fprintf(os.Stderr, "Provide them via -access-key/-secret-key flags or MINIO_ACCESS_KEY/MINIO_SECRET_KEY environment variables\n")
		os.Exit(1)
	}

	// Parse metric types
	types := parseMetricTypes(*metricType)
	if types == madmin.MetricsNone {
		fmt.Fprintf(os.Stderr, "Error: No valid metric types specified\n")
		os.Exit(1)
	}

	// Parse hosts if provided
	var hostList []string
	if *hosts != "" {
		hostList = strings.Split(*hosts, ",")
		for i, host := range hostList {
			hostList[i] = strings.TrimSpace(host)
		}
	}

	// Parse disks if provided
	var diskList []string
	if *disks != "" {
		diskList = strings.Split(*disks, ",")
		for i, disk := range diskList {
			diskList[i] = strings.TrimSpace(disk)
		}
	}

	// Parse pool indices if provided
	var poolList []int
	if *pools != "" {
		poolStrs := strings.Split(*pools, ",")
		for _, poolStr := range poolStrs {
			if poolIdx, err := strconv.Atoi(strings.TrimSpace(poolStr)); err == nil {
				poolList = append(poolList, poolIdx)
			} else {
				fmt.Fprintf(os.Stderr, "Warning: Invalid pool index '%s'\n", poolStr)
			}
		}
	}

	// Parse disk set indices if provided
	var diskSetList []int
	if *diskSets != "" {
		diskSetStrs := strings.Split(*diskSets, ",")
		for _, diskSetStr := range diskSetStrs {
			if diskSetIdx, err := strconv.Atoi(strings.TrimSpace(diskSetStr)); err == nil {
				diskSetList = append(diskSetList, diskSetIdx)
			} else {
				fmt.Fprintf(os.Stderr, "Warning: Invalid disk set index '%s'\n", diskSetStr)
			}
		}
	}

	// Create admin client
	madmClnt, err := madmin.New(*endpoint, *accessKey, *secretKey, *useSSL)
	if err != nil {
		log.Fatalln("Failed to create admin client:", err)
	}

	// Note: For SSL verification settings, use the client's built-in options
	// The client already uses the provided credentials from New()

	// Build flags
	var flags madmin.MetricFlags
	if *dailyStats {
		flags |= madmin.MetricsDayStats
	}
	if *byHost {
		flags |= madmin.MetricsByHost
	}
	if *byDisk {
		flags |= madmin.MetricsByDisk
	}
	if *legacyDiskIO {
		flags |= madmin.MetricsLegacyDiskIO
	}

	if *verbose {
		fmt.Printf("Connecting to: %s\n", *endpoint)
		fmt.Printf("Metric types: %s\n", metricTypeToString(types))
		fmt.Printf("Interval: %v\n", *interval)
		fmt.Printf("Max samples: %d\n", *maxSamples)
		if len(hostList) > 0 {
			fmt.Printf("Hosts: %v\n", hostList)
		}
		if len(diskList) > 0 {
			fmt.Printf("Disks: %v\n", diskList)
		}
		if len(poolList) > 0 {
			fmt.Printf("Pools: %v\n", poolList)
		}
		if len(diskSetList) > 0 {
			fmt.Printf("Disk Sets: %v\n", diskSetList)
		}
		if flags != 0 {
			fmt.Printf("Flags: by-host=%t, by-disk=%t, daily-stats=%t, legacy-io=%t\n",
				*byHost, *byDisk, *dailyStats, *legacyDiskIO)
		}
		fmt.Printf("JSON output: %v\n", *jsonOutput)
		fmt.Println("---")
	}

	// Create metrics options
	opts := madmin.MetricsOptions{
		Type:        types,
		Flags:       flags,
		N:           *maxSamples,
		Interval:    *interval,
		PoolIdx:     poolList,
		Hosts:       hostList,
		DriveSetIdx: diskSetList,
		Disks:       diskList,
		ByHost:      *byHost, // Legacy flag for compatibility
		ByDisk:      *byDisk, // Legacy flag for compatibility
	}

	// Context with timeout
	ctx := context.Background()

	// Counter for metrics received
	metricsReceived := 0
	startTime := time.Now()

	// Collect metrics
	err = madmClnt.Metrics(ctx, opts, func(metrics madmin.RealtimeMetrics) {
		metricsReceived++

		if *jsonOutput {
			// JSON output
			data, err := json.MarshalIndent(metrics, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
				return
			}
			fmt.Printf("=== Metrics Sample %d ===\n", metricsReceived)
			fmt.Println(string(data))
			fmt.Println()
		} else {
			// Human-readable output
			displayMetrics(metrics, metricsReceived)
		}
	})

	if err != nil {
		log.Fatalln("Error collecting metrics:", err)
	}

	elapsed := time.Since(startTime)
	if *verbose {
		fmt.Printf("\nCollection completed. Received %d metric samples in %v\n", metricsReceived, elapsed)
	}
}

// metricTypeToString converts MetricType bitfield to readable string representation
func metricTypeToString(mt madmin.MetricType) string {
	if mt == madmin.MetricsAll {
		return "all"
	}
	if mt == madmin.MetricsNone {
		return "none"
	}

	var parts []string

	if mt&madmin.MetricsMem != 0 {
		parts = append(parts, "mem")
	}
	if mt&madmin.MetricsCPU != 0 {
		parts = append(parts, "cpu")
	}
	if mt&madmin.MetricsDisk != 0 {
		parts = append(parts, "disk")
	}
	if mt&madmin.MetricNet != 0 {
		parts = append(parts, "net")
	}
	if mt&madmin.MetricsProcess != 0 {
		parts = append(parts, "process")
	}
	if mt&madmin.MetricsOS != 0 {
		parts = append(parts, "os")
	}
	if mt&madmin.MetricsScanner != 0 {
		parts = append(parts, "scanner")
	}
	if mt&madmin.MetricsBatchJobs != 0 {
		parts = append(parts, "batchjobs")
	}
	if mt&madmin.MetricsSiteResync != 0 {
		parts = append(parts, "siteresync")
	}
	if mt&madmin.MetricsRPC != 0 {
		parts = append(parts, "rpc")
	}
	if mt&madmin.MetricsRuntime != 0 {
		parts = append(parts, "runtime")
	}
	if mt&madmin.MetricsAPI != 0 {
		parts = append(parts, "api")
	}
	if mt&madmin.MetricsReplication != 0 {
		parts = append(parts, "replication")
	}

	if len(parts) == 0 {
		return fmt.Sprintf("unknown(0x%x)", uint64(mt))
	}

	return strings.Join(parts, ",")
}

// parseMetricTypes converts comma-separated metric type string to MetricType bitfield
func parseMetricTypes(typeStr string) madmin.MetricType {
	var result madmin.MetricType

	types := strings.Split(typeStr, ",")
	for _, t := range types {
		switch strings.TrimSpace(strings.ToLower(t)) {
		case "all":
			return madmin.MetricsAll
		case "mem", "memory":
			result |= madmin.MetricsMem
		case "cpu":
			result |= madmin.MetricsCPU
		case "disk":
			result |= madmin.MetricsDisk
		case "net", "network":
			result |= madmin.MetricNet
		case "process", "proc":
			result |= madmin.MetricsProcess
		case "os":
			result |= madmin.MetricsOS
		case "scanner":
			result |= madmin.MetricsScanner
		case "batchjobs", "batch-jobs", "batch":
			result |= madmin.MetricsBatchJobs
		case "siteresync", "site-resync", "resync":
			result |= madmin.MetricsSiteResync
		case "rpc":
			result |= madmin.MetricsRPC
		case "runtime", "go":
			result |= madmin.MetricsRuntime
		case "api":
			result |= madmin.MetricsAPI
		case "replication", "repl":
			result |= madmin.MetricsReplication
		default:
			fmt.Fprintf(os.Stderr, "Warning: Unknown metric type '%s'\n", t)
		}
	}

	return result
}

// displayMetrics shows metrics in human-readable format
func displayMetrics(metrics madmin.RealtimeMetrics, sampleNum int) {
	fmt.Printf("=== Metrics Sample %d ===\n", sampleNum)
	fmt.Printf("Timestamp: %s\n", time.Now().Format(time.RFC3339))
	fmt.Printf("Hosts: %d\n", len(metrics.Hosts))

	// Display aggregated metrics
	agg := metrics.Aggregated

	// Memory metrics
	if agg.Mem != nil {
		fmt.Printf("\n--- Memory Metrics ---\n")
		fmt.Printf("Nodes: %d\n", agg.Mem.Nodes)
		fmt.Printf("Total: %s\n", formatBytes(agg.Mem.Info.Total))
		fmt.Printf("Used: %s (%.1f%%)\n", formatBytes(agg.Mem.Info.Used),
			float64(agg.Mem.Info.Used)/float64(agg.Mem.Info.Total)*100)
		fmt.Printf("Free: %s\n", formatBytes(agg.Mem.Info.Free))
		fmt.Printf("Available: %s\n", formatBytes(agg.Mem.Info.Available))
		// Note: Cached and Buffers fields may not be available on all platforms
	}

	// CPU metrics
	if agg.CPU != nil {
		fmt.Printf("\n--- CPU Metrics ---\n")
		fmt.Printf("Nodes: %d\n", agg.CPU.Nodes)
		if agg.CPU.TimesStat != nil {
			fmt.Printf("User: %.2f%%, System: %.2f%%, Idle: %.2f%%\n",
				agg.CPU.TimesStat.User, agg.CPU.TimesStat.System, agg.CPU.TimesStat.Idle)
		}
		if agg.CPU.LoadStat != nil {
			fmt.Printf("Load Average: %.2f, %.2f, %.2f\n",
				agg.CPU.LoadStat.Load1, agg.CPU.LoadStat.Load5, agg.CPU.LoadStat.Load15)
		}

		// Display aggregated CPU information
		if len(agg.CPU.CPUByModel) > 0 {
			fmt.Printf("CPU Models:\n")
			for model, count := range agg.CPU.CPUByModel {
				fmt.Printf("  %s: %d cores\n", model, count)
			}
		}
		if agg.CPU.TotalMhz > 0 {
			fmt.Printf("Total MHz: %.0f\n", agg.CPU.TotalMhz)
		}
		if agg.CPU.TotalCores > 0 {
			fmt.Printf("Total Cores: %d\n", agg.CPU.TotalCores)
		}
		if agg.CPU.TotalCacheSize > 0 {
			fmt.Printf("Total Cache: %s\n", formatBytes(uint64(agg.CPU.TotalCacheSize)))
		}
	}

	// Disk metrics
	if agg.Disk != nil {
		fmt.Printf("\n--- Disk Metrics ---\n")
		fmt.Printf("Number of Disks: %d\n", agg.Disk.NDisks)
		if agg.Disk.Space.Used.Total > 0 {
			fmt.Printf("Used Space: %s\n", formatBytes(agg.Disk.Space.Used.Total))
		}
		if agg.Disk.Space.Free.Total > 0 {
			fmt.Printf("Free Space: %s\n", formatBytes(agg.Disk.Space.Free.Total))
		}
		if agg.Disk.Offline > 0 {
			fmt.Printf("Offline Disks: %d\n", agg.Disk.Offline)
		}
		if agg.Disk.Hanging > 0 {
			fmt.Printf("Hanging Disks: %d\n", agg.Disk.Hanging)
		}
	}

	// Network metrics
	if agg.Net != nil {
		fmt.Printf("\n--- Network Metrics ---\n")
		fmt.Printf("Number of Interfaces: %d\n", len(agg.Net.Interfaces))
		// Network interface details are available but complex to display
		fmt.Printf("Network interface data collected\n")
	}

	// Process metrics (new feature)
	if agg.Process != nil {
		fmt.Printf("\n--- Process Metrics ---\n")
		fmt.Printf("Nodes: %d, Processes: %d\n", agg.Process.Nodes, agg.Process.Count)
		fmt.Printf("CPU: %.2f%%, Connections: %d\n",
			agg.Process.TotalCPUPercent, agg.Process.TotalNumConnections)
		fmt.Printf("Running Time: %.0fs\n", agg.Process.TotalRunningSecs)
		fmt.Printf("FDs: %d, Threads: %d\n", agg.Process.TotalNumFDs, agg.Process.TotalNumThreads)
		fmt.Printf("Background: %d, Running: %d\n",
			agg.Process.BackgroundProcesses, agg.Process.RunningProcesses)
		if agg.Process.MemInfo.RSS > 0 {
			fmt.Printf("Memory RSS: %s, VMS: %s\n",
				formatBytes(agg.Process.MemInfo.RSS), formatBytes(agg.Process.MemInfo.VMS))
		}
		if agg.Process.IOCounters.ReadBytes > 0 || agg.Process.IOCounters.WriteBytes > 0 {
			fmt.Printf("I/O: Read %s, Write %s\n",
				formatBytes(agg.Process.IOCounters.ReadBytes),
				formatBytes(agg.Process.IOCounters.WriteBytes))
		}
	}

	// OS metrics with sensors
	if agg.OS != nil {
		fmt.Printf("\n--- OS Metrics ---\n")
		if len(agg.OS.Sensors) > 0 {
			fmt.Printf("Temperature Sensors: %d\n", len(agg.OS.Sensors))
			for key, sensor := range agg.OS.Sensors {
				if sensor.Count > 0 {
					avgTemp := float64(sensor.TotalTemp) / float64(sensor.Count)
					fmt.Printf("  %s: Avg=%.1f°C, Min=%.1f°C, Max=%.1f°C\n",
						key, avgTemp, sensor.MinTemp, sensor.MaxTemp)
				}
			}
		} else {
			fmt.Printf("No temperature sensors detected\n")
		}

		// OS operations
		if len(agg.OS.LifeTimeOps) > 0 {
			fmt.Printf("Lifetime Operations: %d types\n", len(agg.OS.LifeTimeOps))
		}
	}

	// Scanner metrics
	if agg.Scanner != nil {
		fmt.Printf("\n--- Scanner Metrics ---\n")
		fmt.Printf("Scanner data available (use -json for details)\n")
	}

	// Batch Jobs metrics
	if agg.BatchJobs != nil {
		fmt.Printf("\n--- Batch Jobs Metrics ---\n")
		fmt.Printf("Batch job data available (use -json for details)\n")
	}

	// Site Resync metrics
	if agg.SiteResync != nil {
		fmt.Printf("\n--- Site Resync Metrics ---\n")
		fmt.Printf("Site resync data available (use -json for details)\n")
	}

	// RPC metrics
	if agg.RPC != nil {
		fmt.Printf("\n--- RPC Metrics ---\n")
		fmt.Printf("RPC call data available (use -json for details)\n")
	}

	// Runtime (Go) metrics
	if agg.Go != nil {
		fmt.Printf("\n--- Runtime Metrics ---\n")
		fmt.Printf("Go runtime data available (use -json for details)\n")
	}

	// API metrics
	if agg.API != nil {
		fmt.Printf("\n--- API Metrics ---\n")
		fmt.Printf("API operation data available (use -json for details)\n")
	}

	// Replication metrics
	if agg.Replication != nil {
		fmt.Printf("\n--- Replication Metrics ---\n")
		fmt.Printf("Replication data available (use -json for details)\n")
	}

	// Show individual host information if verbose
	if *verbose && len(metrics.ByHost) > 1 {
		fmt.Printf("\n--- Per-Host Summary ---\n")
		for host, hostMetrics := range metrics.ByHost {
			fmt.Printf("%s: ", host)
			parts := []string{}
			if hostMetrics.Mem != nil && hostMetrics.Mem.Info.Total > 0 {
				parts = append(parts, fmt.Sprintf("Mem: %s", formatBytes(hostMetrics.Mem.Info.Total)))
			}
			if hostMetrics.CPU != nil && hostMetrics.CPU.TimesStat != nil {
				parts = append(parts, fmt.Sprintf("CPU: %.1f%%", 100-hostMetrics.CPU.TimesStat.Idle))
			}
			if hostMetrics.Disk != nil {
				parts = append(parts, fmt.Sprintf("Disks: %d", hostMetrics.Disk.NDisks))
			}
			fmt.Println(strings.Join(parts, ", "))
		}
	}

	fmt.Println(strings.Repeat("-", 50))
}

// formatBytes formats byte counts in human-readable format
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
