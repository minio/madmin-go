# MinIO Metrics Navigation (mnav)

The `mnav` package provides a tree-based navigation system for MinIO metrics, enabling interactive exploration of complex metric hierarchies. It transforms flat metric data structures from `madmin.RealtimeMetrics` into navigable tree interfaces with breadcrumbs, children listing, and detailed data views.

## Overview

This package bridges the gap between MinIO's raw metrics API and user-friendly navigation interfaces. It's designed to support tools like Project Tricorder that need to present complex metric data in an organized, explorable format.

### Main Entry Point

The primary entry point is `NewRealtimeMetricsNavigator(metrics)`, which creates a root navigator that automatically provides all available metric categories as navigation children. Clients don't need to know the internal metric structure - the navigation system handles discovery and organization automatically.

### Key Components

- **MetricNode Interface**: Core navigation abstraction
- **Navigator Classes**: Transform specific metric types into navigable trees
- **Dynamic Flag Management**: Automatically requests required metric types
- **Hierarchical Organization**: Logical grouping of related metrics

## Architecture

### Core Interface: MetricNode

All navigable elements implement the `MetricNode` interface:

```go
type MetricNode interface {
    // GetChildren returns a list of navigable children of current node
    GetChildren() []MetricChild

    // GetLeafData returns a map of leaf data for current node.
    // Leaf data is data that is not navigable, such as a summary of the current node.
    // Data may have sort keys, which is 'nn:Key' where 'nn' is the numeric order of the key.
    GetLeafData() map[string]string

    // GetMetricType returns the metric type required for the current node data.
	// Generally GetOpts() should be used instead, which combines flags from all parent nodes.
    GetMetricType() madmin.MetricType

    // GetMetricFlags returns the metric flags required for the current node data.
    // Generally GetOpts() should be used instead, which combines flags from all parent nodes.
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
```

### Navigation Flow

```
madmin.RealtimeMetrics → NewRealtimeMetricsNavigator() → Root Navigator
                                        ↓
                            Automatically discovers and provides:
                                        ↓
                        AggregatedMetricsNode / PerNodeMetricsNode
                                        ↓
                    APIMetricsNavigator, DiskMetricsNavigator, ScannerMetricsNode, etc.
```

## Integration with madmin.Metrics

### 1. Metric Collection

The navigation system works with MinIO's real-time metrics API:

```go
// Collect metrics with required flags
opts := madmin.MetricsOptions{
    Type:     madmin.MetricsAPI | madmin.MetricsRPC | madmin.MetricsScanner,
    Interval: time.Second,
}

adminClient.Metrics(ctx, opts, func(metrics madmin.RealtimeMetrics) {
    // Create root navigator - this is the main entry point
    root := mnav.NewRealtimeMetricsNavigator(metrics)

    // The navigator automatically provides all available categories
    children := root.GetChildren()
    // children[0] = {Name: "aggregated", Description: "Aggregated metrics across all nodes"}
    // children[1] = {Name: "by_node", Description: "Per-node metric breakdown"}

    // Navigate to aggregated metrics
    aggregated, _ := root.GetChild("aggregated")

    // Navigate to specific metric type (scanner, API, RPC, etc.)
    scanner, _ := aggregated.GetChild("scanner")

    // Display data - the navigator handles all the complexity
    data := scanner.GetLeafData()
    // data["Active drives"] = "8"
    // data["Objects/min"] = "12,345 (2.1ms avg)"
})
```

### 2. Dynamic Flag Management

Navigators automatically specify which metrics they need:

```go
type APIMetricsNavigator struct {
    // ... fields
}

func (n *APIMetricsNavigator) GetMetricType() madmin.MetricType {
    return madmin.MetricsAPI  // This navigator represents API metrics
}

func (n *APIMetricsNavigator) GetMetricFlags() madmin.MetricFlags {
    return madmin.FlagsByServer  // Need per-server breakdown
}
```

When navigating to a node that requires different metrics, the system can trigger a refresh with appropriate flags.

### 3. Hierarchical Organization

The package organizes metrics into logical hierarchies:

```
root/
├── aggregated/          # Aggregated across all nodes
│   ├── api/             # API operation metrics
│   ├── rpc/             # RPC call statistics
│   ├── disk/            # Disk usage and performance
│   ├── scanner/         # Scanner operation metrics
│   ├── batch_jobs/      # Batch job status and progress
│   └── site_resync/     # Site replication resync
└── by_node/             # Per-node breakdown
    └── node-1/
        ├── api/
        ├── rpc/
        └── disk/
```

The clients do not need to know the exact structure of the metrics, since the navigation system will provide and populate the appropriate children.

### 4. Key Features

#### Data Display
- **Sorted Leaf Data**: Metric data is automatically sorted and formatted for display
- **Text Trimming**: Long keys and values are automatically trimmed to fit available screen width
- **Humanized Formatting**: Numbers, bytes, and durations are displayed in human-readable format

#### Dynamic Refresh Management
- **Automatic Flag Detection**: When navigating to nodes requiring different metric types or flags, the system automatically triggers a refresh
- **Smart Refresh Pausing**: Nodes with rarely-updating data (completed jobs, historical data) pause auto-refresh to reduce overhead
- **Race-Free Navigation**: Synchronous refresh during navigation prevents race conditions

#### Intelligent Refresh Control
- **ShouldPauseRefresh()**: Nodes can indicate when they contain rarely-updating data:
  - Completed batch jobs pause refresh
  - Historical time-based data (RPC last day) pauses refresh
  - Active operations continue refreshing
  - Site resync operations pause refresh when completed/failed

## Navigator Implementation Patterns

### 1. Simple Leaf Navigator

For metrics that don't need sub-navigation:

```go
type SiteResyncMetricsNode struct {
    resync *madmin.SiteResyncMetrics
    parent MetricNode
    path   string
}

func (node *SiteResyncMetricsNode) GetChildren() []MetricChild {
    return []MetricChild{}  // Leaf node - no children
}

func (node *SiteResyncMetricsNode) GetLeafData() map[string]string {
    data := map[string]string{}

    if node.resync != nil {
        data["Status"] = strings.Title(node.resync.ResyncStatus)
        data["Objects synced"] = humanize.Comma(node.resync.ReplicatedCount)
        data["Data synced"] = humanize.Bytes(uint64(node.resync.ReplicatedSize))
        // ... more fields
    }

    return data
}

func (node *SiteResyncMetricsNode) ShouldPauseRefresh() bool {
    // Pause refresh for completed or failed operations
    if node.resync != nil && (node.resync.Complete() || node.resync.ResyncStatus == "failed") {
        return true
    }
    return false
}
```

### 2. Tree Navigator with Sub-nodes

For complex metrics with hierarchical structure:

```go
type BatchJobMetricsNode struct {
    batch  *madmin.BatchJobMetrics
    parent MetricNode
    path   string
}

func (node *BatchJobMetricsNode) GetChildren() []MetricChild {
    var children []MetricChild

    // Create child for each job
    for jobID, job := range node.batch.Jobs {
        status := "running"
        if job.Complete { status = "completed" }
        if job.Failed { status = "failed" }

        children = append(children, MetricChild{
            Name:        jobID,
            Description: fmt.Sprintf("%s job - %s", job.JobType, status),
        })
    }

    return children
}

func (node *BatchJobMetricsNode) GetChild(name string) (MetricNode, error) {
    job, exists := node.batch.Jobs[name]
    if !exists {
        return nil, fmt.Errorf("job not found: %s", name)
    }

    return NewBatchJobNode(&job, node, fmt.Sprintf("%s/%s", node.path, name)), nil
}
```

### 3. Time-series Navigator

For metrics with time-based organization:

```go
type RPCLastDayAllNode struct {
    handlers map[string]madmin.RPCMetrics
    parent   MetricNode
    path     string
}

func (node *RPCLastDayAllNode) GetChildren() []MetricChild {
    // Calculate all time segments across handlers
    allSegments := calculateAllTimeSegments(node.handlers)

    var children []MetricChild
    for _, segment := range allSegments {
        timeStr := formatTimeSegment(segment)
        children = append(children, MetricChild{
            Name:        segment,
            Description: fmt.Sprintf("RPC metrics for %s", timeStr),
        })
    }

    return children
}
```

## File Organization

The package is organized by metric type, with each major metric family having its own file:

```
mnav/
├── README.md           # This documentation
├── mnav.go            # Core interfaces and root navigation
├── api.go             # API metrics navigation (not yet implemented)
├── rpc.go             # RPC metrics navigation
├── disk.go            # Disk metrics navigation
├── scanner.go         # Scanner metrics navigation
├── batch_jobs.go      # Batch job metrics navigation
├── site_resync.go     # Site resync metrics navigation
├── cpu.go             # CPU metrics navigation
├── mem.go             # Memory metrics navigation
├── net.go             # Network metrics navigation
├── os.go              # OS metrics navigation
├── process.go         # Process metrics navigation
├── runtime.go         # Go runtime metrics navigation
└── replication.go     # Replication metrics navigation
```

## Usage Examples

### Basic Navigation

```go
// Create root navigator - main entry point
root := mnav.NewRealtimeMetricsNavigator(metrics)

// The navigator automatically discovers and provides all available metrics
// Navigate to aggregated metrics
aggregated, err := root.GetChild("aggregated")
if err != nil { return err }

// Navigate to scanner metrics (automatically available if present)
scanner, err := aggregated.GetChild("scanner")
if err != nil { return err }

// Get overview data - the navigator formats everything
data := scanner.GetLeafData()
// data["Active drives"] = "8"
// data["Objects/min"] = "12,345 (2.1ms avg)"
```

### Dynamic Metric Requirements

```go
// Check what metrics a node needs
requiredTypes := node.GetMetricType()
requiredFlags := node.GetMetricFlags()

// Collect metrics with appropriate flags
opts := madmin.MetricsOptions{
    Type:     requiredTypes,
    Flags:    requiredFlags,
    Interval: time.Second,
}

// Navigation system can trigger refresh when needed flags change
```

### Breadcrumb Generation

```go
// Generate navigation breadcrumbs
func GenerateBreadcrumbs(node MetricNode) []string {
    var breadcrumbs []string

    current := node
    for current != nil {
        if path := current.GetPath(); path != "" {
            parts := strings.Split(path, "/")
            breadcrumbs = append([]string{parts[len(parts)-1]}, breadcrumbs...)
        }
        current = current.GetParent()
    }

    return breadcrumbs
}
```

## Best Practices

### 1. Error Handling
```go
func (node *MyNode) GetChild(name string) (MetricNode, error) {
    if node.data == nil {
        return nil, fmt.Errorf("no data available")
    }

    child, exists := node.data[name]
    if !exists {
        return nil, fmt.Errorf("child not found: %s", name)
    }

    return NewChildNode(child, node, fmt.Sprintf("%s/%s", node.path, name)), nil
}
```

### 2. Data Formatting
```go
func (node *MyNode) GetLeafData() map[string]string {
    data := map[string]string{}

    if node.metrics != nil {
        // Use humanized formatting
        data["Objects"] = humanize.Comma(node.metrics.ObjectCount)
        data["Data size"] = humanize.Bytes(uint64(node.metrics.DataSize))
        data["Last updated"] = node.metrics.CollectedAt.Format("15:04:05")
    }

    return data
}
```

### 3. Efficient Navigation
```go
func (node *MyNode) GetChildren() []MetricChild {
    if node.data == nil || len(node.data) == 0 {
        return []MetricChild{}  // Return empty slice, not nil
    }

    // Sort children for consistent ordering
    var names []string
    for name := range node.data {
        names = append(names, name)
    }
    sort.Strings(names)

    var children []MetricChild
    for _, name := range names {
        children = append(children, MetricChild{
            Name:        name,
            Description: generateDescription(node.data[name]),
        })
    }

    return children
}
```

## Integration Points

### With Project Tricorder

The navigation system integrates with interactive CLI tools:

```go
type NavigationState struct {
    currentNode  mnav.MetricNode
    currentPath  string
    pathHistory  []string
}

// Initialize with the main entry point
func (ns *NavigationState) Initialize(metrics madmin.RealtimeMetrics) {
    ns.currentNode = mnav.NewRealtimeMetricsNavigator(metrics)
    // The navigator automatically provides all available metric categories
}

func (ns *NavigationState) NavigateInto() error {
    selectedChild := ns.getSelectedChild()

    childNode, err := ns.currentNode.GetChild(selectedChild.Name)
    if err != nil {
        return err
    }

    // Check if new metrics are needed
    opts := childNode.GetOpts()
    if !ns.hasRequiredMetrics(opts) {
        // Trigger refresh with new flags
        ns.Refresh()
    }

    ns.currentNode = childNode
    return nil
}
```

### With Web UIs

The same navigation system can power web interfaces:

```go
func handleAPIRequest(w http.ResponseWriter, r *http.Request) {
    path := strings.Trim(r.URL.Path, "/")

    // Use the main entry point
    root := mnav.NewRealtimeMetricsNavigator(latestMetrics)
    node, err := navigateToPath(root, path)
    if err != nil {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }

    response := struct {
        Path     string            `json:"path"`
        Children []mnav.MetricChild `json:"children,omitempty"`
        Data     map[string]string `json:"data,omitempty"`
    }{
        Path:     node.GetPath(),
        Children: node.GetChildren(), // Automatically discovers available children
        Data:     node.GetLeafData(), // Formatted data ready for display
    }

    json.NewEncoder(w).Encode(response)
}
```
