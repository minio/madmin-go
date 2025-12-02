package mnav

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/minio/madmin-go/v4"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// formatNumber formats large numbers with thousand separators
func formatNumber(n uint64) string {
	return humanize.Comma(int64(n))
}

// formatBytes formats bytes in human readable format
func formatBytes(bytes uint64) string {
	return humanize.Bytes(bytes)
}

// OSMetricsNavigator provides navigation for OS metrics
type OSMetricsNavigator struct {
	os     *madmin.OSMetrics
	parent MetricNode
	path   string
}

func (node *OSMetricsNavigator) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

// NewOSMetricsNavigator creates a new OS metrics navigator
func NewOSMetricsNavigator(os *madmin.OSMetrics, parent MetricNode, path string) *OSMetricsNavigator {
	return &OSMetricsNavigator{os: os, parent: parent, path: path}
}

func (node *OSMetricsNavigator) GetChildren() []MetricChild {
	return []MetricChild{
		{Name: "lifetime_ops", Description: "Accumulated operations since server start"},
		{Name: "last_minute", Description: "Last minute operation statistics"},
		{Name: "sensors", Description: "Temperature sensor metrics"},
	}
}

func (node *OSMetricsNavigator) GetLeafData() map[string]string {
	if node.os == nil {
		return map[string]string{"Status": "OS metrics not available"}
	}

	data := map[string]string{
		"Collected At":           node.os.CollectedAt.Format("2006-01-02 15:04:05"),
		"Operation Types":        strconv.Itoa(len(node.os.LifeTimeOps)),
		"Last Minute Operations": strconv.Itoa(len(node.os.LastMinute.Operations)),
		"Temperature Sensors":    strconv.Itoa(len(node.os.Sensors)),
	}

	// Add totals for lifetime ops
	var totalLifetimeOps uint64
	for _, count := range node.os.LifeTimeOps {
		totalLifetimeOps += count
	}
	data["Total Lifetime Operations"] = formatNumber(totalLifetimeOps)

	return data
}

func (node *OSMetricsNavigator) GetMetricType() madmin.MetricType {
	return madmin.MetricsOS
}

func (node *OSMetricsNavigator) GetMetricFlags() madmin.MetricFlags {
	return 0
}

func (node *OSMetricsNavigator) GetParent() MetricNode {
	return node.parent
}

func (node *OSMetricsNavigator) GetPath() string {
	return node.path
}

func (node *OSMetricsNavigator) ShouldPauseRefresh() bool {
	return false
}

func (node *OSMetricsNavigator) GetChild(name string) (MetricNode, error) {
	switch name {
	case "lifetime_ops":
		return NewOSLifetimeOpsNode(node.os.LifeTimeOps, node, fmt.Sprintf("%s/lifetime_ops", node.path)), nil
	case "last_minute":
		return NewOSLastMinuteNode(node.os.LastMinute.Operations, node, fmt.Sprintf("%s/last_minute", node.path)), nil
	case "sensors":
		return NewOSSensorsNode(node.os.Sensors, node, fmt.Sprintf("%s/sensors", node.path)), nil
	default:
		return nil, fmt.Errorf("child not found: %s", name)
	}
}

type OSMetricsNode struct {
	parent MetricNode
	path   string
}

func (node *OSMetricsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *OSMetricsNode) ShouldPauseUpdates() bool {
	// Legacy method - not used in interface, return false for default behavior
	return false
}

func (node *OSMetricsNode) GetChildren() []MetricChild {
	return []MetricChild{
		{Name: "lifetime_ops", Description: "Accumulated operations since server start"},
		{Name: "last_minute", Description: "Last minute operation statistics"},
		{Name: "sensors", Description: "Temperature sensor metrics"},
	}
}
func (node *OSMetricsNode) GetLeafData() map[string]string     { return nil }
func (node *OSMetricsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsOS }
func (node *OSMetricsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *OSMetricsNode) GetParent() MetricNode              { return node.parent }
func (node *OSMetricsNode) GetPath() string                    { return node.path }

func (node *OSMetricsNode) ShouldPauseRefresh() bool {
	return false
}

func (node *OSMetricsNode) GetChild(name string) (MetricNode, error) {
	return nil, fmt.Errorf("os metric sub-navigation not yet implemented for: %s", name)
}

// OSLifetimeOpsNode handles navigation for OS lifetime operations
type OSLifetimeOpsNode struct {
	ops    map[string]uint64
	parent MetricNode
	path   string
}

func (node *OSLifetimeOpsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewOSLifetimeOpsNode(ops map[string]uint64, parent MetricNode, path string) *OSLifetimeOpsNode {
	return &OSLifetimeOpsNode{ops: ops, parent: parent, path: path}
}

func (node *OSLifetimeOpsNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *OSLifetimeOpsNode) GetLeafData() map[string]string {
	if node.ops == nil {
		return map[string]string{"Operation Types": "0", "Total Operations": "0"}
	}

	data := map[string]string{
		"Operation Types": strconv.Itoa(len(node.ops)),
	}
	var total uint64
	for opType, count := range node.ops {
		// Convert technical operation names to display names
		titleCaser := cases.Title(language.Und)
		displayName := strings.ReplaceAll(titleCaser.String(strings.ReplaceAll(opType, "_", " ")), "Api", "API")
		data[displayName] = formatNumber(count)
		total += count
	}
	data["Total Operations"] = formatNumber(total)
	return data
}

func (node *OSLifetimeOpsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsOS }
func (node *OSLifetimeOpsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *OSLifetimeOpsNode) GetParent() MetricNode              { return node.parent }
func (node *OSLifetimeOpsNode) GetPath() string                    { return node.path }

func (node *OSLifetimeOpsNode) ShouldPauseRefresh() bool {
	return false
}

func (node *OSLifetimeOpsNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("lifetime operations node has no children")
}

// OSLastMinuteNode handles navigation for OS last minute operations
type OSLastMinuteNode struct {
	operations map[string]madmin.TimedAction
	parent     MetricNode
	path       string
}

func (node *OSLastMinuteNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewOSLastMinuteNode(operations map[string]madmin.TimedAction, parent MetricNode, path string) *OSLastMinuteNode {
	return &OSLastMinuteNode{operations: operations, parent: parent, path: path}
}

func (node *OSLastMinuteNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *OSLastMinuteNode) GetLeafData() map[string]string {
	if node.operations == nil {
		return map[string]string{"Operation Types": "0", "Total Operations": "0", "Total Time": "0 ms"}
	}

	data := map[string]string{
		"Operation Types": strconv.Itoa(len(node.operations)),
	}
	var totalCount, totalTime uint64
	for opType, action := range node.operations {
		titleCaser := cases.Title(language.Und)
		displayName := strings.ReplaceAll(titleCaser.String(strings.ReplaceAll(opType, "_", " ")), "Api", "API")

		// Enhanced single-line format with comprehensive timing stats
		var statsParts []string

		// Add operation count
		statsParts = append(statsParts, fmt.Sprintf("%s ops", formatNumber(action.Count)))

		if action.Count > 0 {
			// Calculate average time
			avgTimeMs := float64(action.AccTime) / float64(action.Count) / 1000000 // ns to ms
			statsParts = append(statsParts, fmt.Sprintf("avg: %.2fms", avgTimeMs))

			// Add min/max times
			minTimeMs := float64(action.MinTime) / 1000000
			maxTimeMs := float64(action.MaxTime) / 1000000
			statsParts = append(statsParts, fmt.Sprintf("min: %.2fms", minTimeMs))
			statsParts = append(statsParts, fmt.Sprintf("max: %.2fms", maxTimeMs))

			// Add average bytes if available
			if action.Bytes > 0 {
				avgBytes := action.Bytes / action.Count
				statsParts = append(statsParts, fmt.Sprintf("%s avg", formatBytes(avgBytes)))
			}
		}

		// Combine into single comprehensive line
		data[displayName+" Ops"] = strings.Join(statsParts, ", ")

		totalCount += action.Count
		totalTime += action.AccTime
	}
	data["Total Operations"] = formatNumber(totalCount)
	data["Total Time"] = fmt.Sprintf("%.2f ms", float64(totalTime)/1000000)
	return data
}

func (node *OSLastMinuteNode) GetMetricType() madmin.MetricType   { return madmin.MetricsOS }
func (node *OSLastMinuteNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *OSLastMinuteNode) GetParent() MetricNode              { return node.parent }
func (node *OSLastMinuteNode) GetPath() string                    { return node.path }

func (node *OSLastMinuteNode) ShouldPauseRefresh() bool {
	return false
}

func (node *OSLastMinuteNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("last minute operations node has no children")
}

// OSSensorsNode handles navigation for OS temperature sensors
type OSSensorsNode struct {
	sensors map[string]madmin.SensorMetrics
	parent  MetricNode
	path    string
}

func (node *OSSensorsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewOSSensorsNode(sensors map[string]madmin.SensorMetrics, parent MetricNode, path string) *OSSensorsNode {
	return &OSSensorsNode{sensors: sensors, parent: parent, path: path}
}

func (node *OSSensorsNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *OSSensorsNode) GetLeafData() map[string]string {
	if node.sensors == nil {
		return map[string]string{"Temperature Sensors": "0"}
	}

	data := map[string]string{
		"Temperature Sensors": strconv.Itoa(len(node.sensors)),
	}

	// Calculate aggregate sensor statistics
	var totalReadings, totalExceedsCritical int
	var minTempOverall, maxTempOverall, totalTempOverall float64
	first := true

	for sensorKey, sensor := range node.sensors {
		titleCaser := cases.Title(language.Und)
		displayKey := titleCaser.String(strings.ReplaceAll(sensorKey, "_", " "))

		// Enhanced single-line format with comprehensive sensor stats
		var statsParts []string

		// Calculate average temperature
		avgTemp := float64(0)
		if sensor.Count > 0 {
			avgTemp = sensor.TotalTemp / float64(sensor.Count)
		}

		// Add current/average temperature first
		statsParts = append(statsParts, fmt.Sprintf("%.1f°C", avgTemp))

		// Add detailed breakdown in parentheses
		tempDetails := fmt.Sprintf("avg: %.1f°C, min: %.1f°C, max: %.1f°C, %s readings",
			avgTemp, sensor.MinTemp, sensor.MaxTemp, formatNumber(uint64(sensor.Count)))

		if sensor.ExceedsCritical > 0 {
			tempDetails += fmt.Sprintf(", %s critical", formatNumber(uint64(sensor.ExceedsCritical)))
		}

		statsParts = append(statsParts, fmt.Sprintf("(%s)", tempDetails))

		// Combine into single comprehensive line
		data[displayKey+" Sensor"] = strings.Join(statsParts, " ")

		// Update totals for overall statistics
		totalReadings += sensor.Count
		totalExceedsCritical += sensor.ExceedsCritical
		totalTempOverall += sensor.TotalTemp

		if first {
			minTempOverall = sensor.MinTemp
			maxTempOverall = sensor.MaxTemp
			first = false
		} else {
			if sensor.MinTemp < minTempOverall {
				minTempOverall = sensor.MinTemp
			}
			if sensor.MaxTemp > maxTempOverall {
				maxTempOverall = sensor.MaxTemp
			}
		}
	}

	// Add overall summary
	if totalReadings > 0 {
		data["Total Readings"] = formatNumber(uint64(totalReadings))
		data["Overall Temp Range"] = fmt.Sprintf("%.1f°C to %.1f°C (avg: %.1f°C)",
			minTempOverall, maxTempOverall, totalTempOverall/float64(totalReadings))
		if totalExceedsCritical > 0 {
			data["Total Critical Events"] = formatNumber(uint64(totalExceedsCritical))
		}
	}

	return data
}

func (node *OSSensorsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsOS }
func (node *OSSensorsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *OSSensorsNode) GetParent() MetricNode              { return node.parent }
func (node *OSSensorsNode) GetPath() string                    { return node.path }

func (node *OSSensorsNode) ShouldPauseRefresh() bool {
	return false
}

func (node *OSSensorsNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("sensors node has no children")
}
