package mnav

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/minio/madmin-go/v4"
)

// formatBytes formats bytes in human readable format
func formatRuntimeBytes(bytes uint64) string {
	return humanize.Bytes(bytes)
}

// formatRuntimeNumber formats large numbers with thousand separators
func formatRuntimeNumber(n uint64) string {
	return humanize.Comma(int64(n))
}

// RuntimeMetricsNavigator provides navigation for Go Runtime metrics
type RuntimeMetricsNavigator struct {
	runtime *madmin.RuntimeMetrics
	parent  MetricNode
	path    string
}

func (node *RuntimeMetricsNavigator) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

// NewRuntimeMetricsNavigator creates a new runtime metrics navigator
func NewRuntimeMetricsNavigator(runtime *madmin.RuntimeMetrics, parent MetricNode, path string) *RuntimeMetricsNavigator {
	return &RuntimeMetricsNavigator{runtime: runtime, parent: parent, path: path}
}

func (node *RuntimeMetricsNavigator) GetChildren() []MetricChild {
	return []MetricChild{
		{Name: "gc", Description: "Garbage collection metrics and heap statistics"},
		{Name: "memory", Description: "Memory management and allocation metrics"},
		{Name: "scheduler", Description: "Go scheduler and goroutine metrics"},
		{Name: "cpu_classes", Description: "CPU time breakdown by runtime activities"},
		{Name: "sync", Description: "Synchronization and locking metrics"},
	}
}

func (node *RuntimeMetricsNavigator) GetLeafData() map[string]string {
	if node.runtime == nil {
		return map[string]string{"Status": "Runtime metrics not available"}
	}

	data := map[string]string{
		"Metric Categories": strconv.Itoa(len(node.GetChildren())),
		"Uint64 Metrics":    strconv.Itoa(len(node.runtime.UintMetrics)),
		"Float64 Metrics":   strconv.Itoa(len(node.runtime.FloatMetrics)),
		"Histogram Metrics": strconv.Itoa(len(node.runtime.HistMetrics)),
		"Total Metrics":     strconv.Itoa(len(node.runtime.UintMetrics) + len(node.runtime.FloatMetrics) + len(node.runtime.HistMetrics)),
	}

	// Add high-level summary metrics if available
	if goroutines, ok := node.runtime.UintMetrics["/sched/goroutines:goroutines"]; ok {
		data["Active Goroutines"] = formatRuntimeNumber(goroutines)
	}

	if heapObjects, ok := node.runtime.UintMetrics["/gc/heap/objects:objects"]; ok {
		data["Heap Objects"] = formatRuntimeNumber(heapObjects)
	}

	if heapBytes, ok := node.runtime.UintMetrics["/memory/classes/heap/objects:bytes"]; ok {
		data["Heap Size"] = formatRuntimeBytes(heapBytes)
	}

	return data
}

func (node *RuntimeMetricsNavigator) GetMetricType() madmin.MetricType {
	return madmin.MetricsRuntime
}

func (node *RuntimeMetricsNavigator) GetMetricFlags() madmin.MetricFlags {
	return 0
}

func (node *RuntimeMetricsNavigator) GetParent() MetricNode {
	return node.parent
}

func (node *RuntimeMetricsNavigator) GetPath() string {
	return node.path
}

func (node *RuntimeMetricsNavigator) ShouldPauseRefresh() bool {
	return false
}

func (node *RuntimeMetricsNavigator) GetChild(name string) (MetricNode, error) {
	if node.runtime == nil {
		return nil, fmt.Errorf("no runtime data available")
	}

	switch name {
	case "gc":
		return NewGCMetricsNode(node.runtime, node, fmt.Sprintf("%s/gc", node.path)), nil
	case "memory":
		return NewMemoryMetricsNode(node.runtime, node, fmt.Sprintf("%s/memory", node.path)), nil
	case "scheduler":
		return NewSchedulerMetricsNode(node.runtime, node, fmt.Sprintf("%s/scheduler", node.path)), nil
	case "cpu_classes":
		return NewCPUClassesMetricsNode(node.runtime, node, fmt.Sprintf("%s/cpu_classes", node.path)), nil
	case "sync":
		return NewSyncMetricsNode(node.runtime, node, fmt.Sprintf("%s/sync", node.path)), nil
	default:
		return nil, fmt.Errorf("child not found: %s", name)
	}
}

// GCMetricsNode handles navigation for garbage collection metrics
type GCMetricsNode struct {
	runtime *madmin.RuntimeMetrics
	parent  MetricNode
	path    string
}

func (node *GCMetricsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewGCMetricsNode(runtime *madmin.RuntimeMetrics, parent MetricNode, path string) *GCMetricsNode {
	return &GCMetricsNode{runtime: runtime, parent: parent, path: path}
}

func (node *GCMetricsNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *GCMetricsNode) GetLeafData() map[string]string {
	if node.runtime == nil {
		return map[string]string{"Status": "GC metrics not available"}
	}

	data := map[string]string{}

	// GC Overview
	if cycles, ok := node.runtime.UintMetrics["/gc/cycles/total:gc-cycles"]; ok {
		data["GC Overview"] = fmt.Sprintf("%s total GC cycles completed", formatRuntimeNumber(cycles))
	}

	// Heap Statistics
	if heapObjects, ok := node.runtime.UintMetrics["/gc/heap/objects:objects"]; ok {
		data["Heap Objects"] = fmt.Sprintf("%s objects currently allocated", formatRuntimeNumber(heapObjects))
	}

	if heapGoal, ok := node.runtime.UintMetrics["/gc/heap/goal:bytes"]; ok {
		data["Heap Goal"] = fmt.Sprintf("%s target heap size", formatRuntimeBytes(heapGoal))
	}

	if totalAllocs, ok := node.runtime.UintMetrics["/gc/heap/allocs:objects"]; ok {
		data["Total Allocations"] = fmt.Sprintf("%s objects allocated lifetime", formatRuntimeNumber(totalAllocs))
	}

	if totalFrees, ok := node.runtime.UintMetrics["/gc/heap/frees:objects"]; ok {
		data["Total Deallocations"] = fmt.Sprintf("%s objects freed lifetime", formatRuntimeNumber(totalFrees))
	}

	// GC Performance
	if gcPauseHist, ok := node.runtime.HistMetrics["/gc/pauses:seconds"]; ok {
		totalCount := uint64(0)
		totalSum := float64(0)

		// Calculate total count from all buckets
		for _, count := range gcPauseHist.Counts {
			totalCount += count
		}

		// Calculate weighted sum using bucket midpoints
		for i, count := range gcPauseHist.Counts {
			if i < len(gcPauseHist.Buckets)-1 && count > 0 {
				bucketMidpoint := (gcPauseHist.Buckets[i] + gcPauseHist.Buckets[i+1]) / 2
				totalSum += bucketMidpoint * float64(count)
			}
		}

		data["GC Pause Analysis"] = fmt.Sprintf("%d pause measurements recorded", totalCount)
		if totalCount > 0 {
			// Convert to milliseconds for better readability
			avgPauseMs := (totalSum / float64(totalCount)) * 1000
			data["Average Pause"] = fmt.Sprintf("%.2fms per GC cycle", avgPauseMs)
			data["Total Pause Time"] = fmt.Sprintf("%.2fms cumulative", totalSum*1000)
		}
	}

	// Scan work
	if scanWork, ok := node.runtime.UintMetrics["/gc/scan/heap:bytes"]; ok {
		data["Heap Scan Work"] = fmt.Sprintf("%s scanned during GC", formatRuntimeBytes(scanWork))
	}

	if scanStack, ok := node.runtime.UintMetrics["/gc/scan/stack:bytes"]; ok {
		data["Stack Scan Work"] = fmt.Sprintf("%s stack memory scanned", formatRuntimeBytes(scanStack))
	}

	return data
}

func (node *GCMetricsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsRuntime }
func (node *GCMetricsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *GCMetricsNode) GetParent() MetricNode              { return node.parent }
func (node *GCMetricsNode) GetPath() string                    { return node.path }
func (node *GCMetricsNode) ShouldPauseRefresh() bool {
	return false
}

func (node *GCMetricsNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("GC metrics node has no children")
}

// MemoryMetricsNode handles navigation for memory management metrics
type MemoryMetricsNode struct {
	runtime *madmin.RuntimeMetrics
	parent  MetricNode
	path    string
}

func (node *MemoryMetricsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewMemoryMetricsNode(runtime *madmin.RuntimeMetrics, parent MetricNode, path string) *MemoryMetricsNode {
	return &MemoryMetricsNode{runtime: runtime, parent: parent, path: path}
}

func (node *MemoryMetricsNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *MemoryMetricsNode) GetLeafData() map[string]string {
	if node.runtime == nil {
		return map[string]string{"Status": "Memory metrics not available"}
	}

	data := map[string]string{}

	// Memory Classes Overview
	data["Memory Classes"] = "Go runtime memory breakdown by usage"

	// Heap memory
	if heapObjects, ok := node.runtime.UintMetrics["/memory/classes/heap/objects:bytes"]; ok {
		data["Heap Objects"] = fmt.Sprintf("%s allocated to Go objects", formatRuntimeBytes(heapObjects))
	}

	if heapUnused, ok := node.runtime.UintMetrics["/memory/classes/heap/unused:bytes"]; ok {
		data["Heap Unused"] = fmt.Sprintf("%s heap space available for allocation", formatRuntimeBytes(heapUnused))
	}

	if heapReleased, ok := node.runtime.UintMetrics["/memory/classes/heap/released:bytes"]; ok {
		data["Heap Released"] = fmt.Sprintf("%s returned to OS but not yet reused", formatRuntimeBytes(heapReleased))
	}

	if heapFree, ok := node.runtime.UintMetrics["/memory/classes/heap/free:bytes"]; ok {
		data["Heap Free"] = fmt.Sprintf("%s unallocated heap memory", formatRuntimeBytes(heapFree))
	}

	// Stack memory
	if heapStacks, ok := node.runtime.UintMetrics["/memory/classes/heap/stacks:bytes"]; ok {
		data["Stack Memory"] = fmt.Sprintf("%s allocated to goroutine stacks", formatRuntimeBytes(heapStacks))
	}

	// Runtime structures
	if metadata, ok := node.runtime.UintMetrics["/memory/classes/metadata/mcache/free:bytes"]; ok {
		data["MCache Free"] = fmt.Sprintf("%s free mcache memory", formatRuntimeBytes(metadata))
	}

	if mcacheInuse, ok := node.runtime.UintMetrics["/memory/classes/metadata/mcache/inuse:bytes"]; ok {
		data["MCache InUse"] = fmt.Sprintf("%s active mcache memory", formatRuntimeBytes(mcacheInuse))
	}

	if mspanFree, ok := node.runtime.UintMetrics["/memory/classes/metadata/mspan/free:bytes"]; ok {
		data["MSpan Free"] = fmt.Sprintf("%s free mspan memory", formatRuntimeBytes(mspanFree))
	}

	if mspanInuse, ok := node.runtime.UintMetrics["/memory/classes/metadata/mspan/inuse:bytes"]; ok {
		data["MSpan InUse"] = fmt.Sprintf("%s active mspan memory", formatRuntimeBytes(mspanInuse))
	}

	// Other memory classes
	if otherSystem, ok := node.runtime.UintMetrics["/memory/classes/os-stacks:bytes"]; ok {
		data["OS Stacks"] = fmt.Sprintf("%s OS thread stacks", formatRuntimeBytes(otherSystem))
	}

	if profBuf, ok := node.runtime.UintMetrics["/memory/classes/profiling/buckets:bytes"]; ok {
		data["Profiling Memory"] = fmt.Sprintf("%s profiling bucket memory", formatRuntimeBytes(profBuf))
	}

	return data
}

func (node *MemoryMetricsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsRuntime }
func (node *MemoryMetricsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *MemoryMetricsNode) GetParent() MetricNode              { return node.parent }
func (node *MemoryMetricsNode) GetPath() string                    { return node.path }
func (node *MemoryMetricsNode) ShouldPauseRefresh() bool {
	return false
}

func (node *MemoryMetricsNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("memory metrics node has no children")
}

// SchedulerMetricsNode handles navigation for Go scheduler metrics
type SchedulerMetricsNode struct {
	runtime *madmin.RuntimeMetrics
	parent  MetricNode
	path    string
}

func (node *SchedulerMetricsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewSchedulerMetricsNode(runtime *madmin.RuntimeMetrics, parent MetricNode, path string) *SchedulerMetricsNode {
	return &SchedulerMetricsNode{runtime: runtime, parent: parent, path: path}
}

func (node *SchedulerMetricsNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *SchedulerMetricsNode) GetLeafData() map[string]string {
	if node.runtime == nil {
		return map[string]string{"Status": "Scheduler metrics not available"}
	}

	data := map[string]string{}

	// Goroutine Statistics
	if goroutines, ok := node.runtime.UintMetrics["/sched/goroutines:goroutines"]; ok {
		data["Goroutine Statistics"] = fmt.Sprintf("%s active goroutines", formatRuntimeNumber(goroutines))
	}

	// Scheduling Latencies
	if schedLatency, ok := node.runtime.HistMetrics["/sched/latencies:seconds"]; ok {
		totalCount := uint64(0)
		totalSum := float64(0)

		// Calculate total count from all buckets
		for _, count := range schedLatency.Counts {
			totalCount += count
		}

		// Calculate weighted sum using bucket midpoints
		for i, count := range schedLatency.Counts {
			if i < len(schedLatency.Buckets)-1 && count > 0 {
				bucketMidpoint := (schedLatency.Buckets[i] + schedLatency.Buckets[i+1]) / 2
				totalSum += bucketMidpoint * float64(count)
			}
		}

		data["Scheduling Performance"] = fmt.Sprintf("%d scheduling measurements", totalCount)
		if totalCount > 0 {
			avgLatencyMs := (totalSum / float64(totalCount)) * 1000
			data["Average Scheduling Latency"] = fmt.Sprintf("%.2fms per goroutine schedule", avgLatencyMs)
			data["Total Scheduling Time"] = fmt.Sprintf("%.2fms cumulative", totalSum*1000)
		}
	}

	// Gomaxprocs
	if gomaxprocs, ok := node.runtime.UintMetrics["/sched/gomaxprocs:threads"]; ok {
		data["Maximum Threads"] = fmt.Sprintf("%s maximum OS threads (GOMAXPROCS)", formatRuntimeNumber(gomaxprocs))
	}

	return data
}

func (node *SchedulerMetricsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsRuntime }
func (node *SchedulerMetricsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *SchedulerMetricsNode) GetParent() MetricNode              { return node.parent }
func (node *SchedulerMetricsNode) GetPath() string                    { return node.path }
func (node *SchedulerMetricsNode) ShouldPauseRefresh() bool {
	return false
}

func (node *SchedulerMetricsNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("scheduler metrics node has no children")
}

// CPUClassesMetricsNode handles navigation for CPU time class metrics
type CPUClassesMetricsNode struct {
	runtime *madmin.RuntimeMetrics
	parent  MetricNode
	path    string
}

func (node *CPUClassesMetricsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewCPUClassesMetricsNode(runtime *madmin.RuntimeMetrics, parent MetricNode, path string) *CPUClassesMetricsNode {
	return &CPUClassesMetricsNode{runtime: runtime, parent: parent, path: path}
}

func (node *CPUClassesMetricsNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *CPUClassesMetricsNode) GetLeafData() map[string]string {
	if node.runtime == nil {
		return map[string]string{"Status": "CPU class metrics not available"}
	}

	data := map[string]string{}

	data["00:CPU time breakdown"] = "Time spent by runtime in different activities"

	// GC activities
	if gcAssist, ok := node.runtime.FloatMetrics["/cpu/classes/gc/mark/assist:cpu-seconds"]; ok {
		data["GC Mark Assist"] = fmt.Sprintf("%.2fs assisting GC marking", gcAssist)
	}

	if gcDedicated, ok := node.runtime.FloatMetrics["/cpu/classes/gc/mark/dedicated:cpu-seconds"]; ok {
		data["GC Mark Dedicated"] = fmt.Sprintf("%.2fs dedicated GC marking", gcDedicated)
	}

	if gcFractional, ok := node.runtime.FloatMetrics["/cpu/classes/gc/mark/fractional:cpu-seconds"]; ok {
		data["GC Mark Fractional"] = fmt.Sprintf("%.2fs fractional GC marking", gcFractional)
	}

	if gcTotal, ok := node.runtime.FloatMetrics["/cpu/classes/gc/total:cpu-seconds"]; ok {
		data["GC Total Time"] = fmt.Sprintf("%.2fs total garbage collection", gcTotal)
	}

	// Scavenging
	if scavenge, ok := node.runtime.FloatMetrics["/cpu/classes/scavenge:cpu-seconds"]; ok {
		data["Memory Scavenge"] = fmt.Sprintf("%.2fs memory scavenging", scavenge)
	}

	// User time
	if userTime, ok := node.runtime.FloatMetrics["/cpu/classes/user:cpu-seconds"]; ok {
		data["User Code"] = fmt.Sprintf("%.2fs executing user Go code", userTime)
	}

	// Idle time
	if idleTime, ok := node.runtime.FloatMetrics["/cpu/classes/idle:cpu-seconds"]; ok {
		data["Idle Time"] = fmt.Sprintf("%.2fs runtime idle", idleTime)
	}

	return data
}

func (node *CPUClassesMetricsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsRuntime }
func (node *CPUClassesMetricsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *CPUClassesMetricsNode) GetParent() MetricNode              { return node.parent }
func (node *CPUClassesMetricsNode) GetPath() string                    { return node.path }
func (node *CPUClassesMetricsNode) ShouldPauseRefresh() bool {
	return false
}

func (node *CPUClassesMetricsNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("CPU classes metrics node has no children")
}

// SyncMetricsNode handles navigation for synchronization metrics
type SyncMetricsNode struct {
	runtime *madmin.RuntimeMetrics
	parent  MetricNode
	path    string
}

func (node *SyncMetricsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewSyncMetricsNode(runtime *madmin.RuntimeMetrics, parent MetricNode, path string) *SyncMetricsNode {
	return &SyncMetricsNode{runtime: runtime, parent: parent, path: path}
}

func (node *SyncMetricsNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *SyncMetricsNode) GetLeafData() map[string]string {
	if node.runtime == nil {
		return map[string]string{"Status": "Synchronization metrics not available"}
	}

	data := map[string]string{}

	// Mutex wait time
	if mutexWait, ok := node.runtime.FloatMetrics["/sync/mutex/wait/total:seconds"]; ok {
		data["00:Sync Overhead"] = fmt.Sprintf("%.2fs total mutex wait time", mutexWait)
	}

	// Check for any other sync-related metrics that might be available
	syncMetricCount := 0
	for key := range node.runtime.FloatMetrics {
		if strings.HasPrefix(key, "/sync/") {
			syncMetricCount++
		}
	}
	for key := range node.runtime.UintMetrics {
		if strings.HasPrefix(key, "/sync/") {
			syncMetricCount++
		}
	}

	if syncMetricCount > 0 {
		data["Available Sync Metrics"] = fmt.Sprintf("%d synchronization metrics tracked", syncMetricCount)
	} else {
		data["Status"] = "No synchronization metrics available in current Go version"
	}

	return data
}

func (node *SyncMetricsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsRuntime }
func (node *SyncMetricsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *SyncMetricsNode) GetParent() MetricNode              { return node.parent }
func (node *SyncMetricsNode) GetPath() string                    { return node.path }
func (node *SyncMetricsNode) ShouldPauseRefresh() bool {
	return false
}

func (node *SyncMetricsNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("sync metrics node has no children")
}
