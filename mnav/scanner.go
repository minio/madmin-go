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
	"sort"
	"strconv"

	"github.com/dustin/go-humanize"
	"github.com/minio/madmin-go/v4"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// ScannerMetricsNode handles navigation for ScannerMetrics
type ScannerMetricsNode struct {
	scanner *madmin.ScannerMetrics
	parent  MetricNode
	path    string
}

func (node *ScannerMetricsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

// NewScannerMetricsNode creates a new ScannerMetricsNode
func NewScannerMetricsNode(scanner *madmin.ScannerMetrics, parent MetricNode, path string) *ScannerMetricsNode {
	return &ScannerMetricsNode{
		scanner: scanner,
		parent:  parent,
		path:    path,
	}
}

func (node *ScannerMetricsNode) GetChildren() []MetricChild {
	if node.scanner == nil {
		return []MetricChild{}
	}

	return []MetricChild{
		{Name: "lifetime_ops", Description: "Accumulated operations since server start"},
		{Name: "lifetime_ilm", Description: "Accumulated ILM operations since server start"},
		{Name: "last_minute", Description: "Last minute operation statistics"},
		{Name: "active_paths", Description: "Currently active scan paths"},
		{Name: "excessive_paths", Description: "Paths marked as having excessive entries"},
	}
}

func (node *ScannerMetricsNode) GetLeafData() map[string]string {
	if node.scanner == nil {
		return map[string]string{"Status": "No scanner metrics available"}
	}

	data := map[string]string{}

	// Scanning Overview
	data["Scanning buckets"] = fmt.Sprintf("%d", node.scanner.OngoingBuckets)
	data["Active drives"] = strconv.Itoa(len(node.scanner.ActivePaths))
	data["Big prefixes"] = strconv.Itoa(len(node.scanner.ExcessivePrefixes))
	data["Total buckets"] = strconv.Itoa(len(node.scanner.PerBucketStats)) + " currently scanning."

	// Last Minute Statistics (if available)
	if len(node.scanner.LastMinute.Actions) > 0 {

		var actionNames []string
		for actionName := range node.scanner.LastMinute.Actions {
			actionNames = append(actionNames, actionName)
		}
		sort.Strings(actionNames)

		for _, actionName := range actionNames {
			action := node.scanner.LastMinute.Actions[actionName]
			if action.Count > 0 {
				avgTimeMs := float64(action.AccTime) / float64(action.Count) / 1e6 // Convert to milliseconds

				switch actionName {
				case "scan":
					data["Objects/min"] = fmt.Sprintf("%s (%.1fms avg)",
						humanize.Comma(int64(action.Count)), avgTimeMs)
				case "versions":
					data["Versions/min"] = fmt.Sprintf("%s (%.1fms avg)",
						humanize.Comma(int64(action.Count)), avgTimeMs)
				case "heal":
					data["Heal checks/min"] = fmt.Sprintf("%s (%.1fms avg)",
						humanize.Comma(int64(action.Count)), avgTimeMs)
				case "read":
					data["Metadata/min"] = fmt.Sprintf("%s (%.1fms avg)",
						humanize.Comma(int64(action.Count)), avgTimeMs)
				case "check-replication":
					data["Replication/min"] = fmt.Sprintf("%s (%.1fms avg)",
						humanize.Comma(int64(action.Count)), avgTimeMs)
				case "verify-deleted":
					data["Verify del/min"] = fmt.Sprintf("%s (%.1fms avg)",
						humanize.Comma(int64(action.Count)), avgTimeMs)
				case "yield":
					totalTimeS := float64(action.AccTime) / 1e9
					data["Yield/min"] = fmt.Sprintf("%.1fs total", totalTimeS)
				default:
					titleCaser := cases.Title(language.Und)
					data[titleCaser.String(actionName)+"/min"] = fmt.Sprintf("%s (%.1fms avg)",
						humanize.Comma(int64(action.Count)), avgTimeMs)
				}
			}
		}
	}

	// ILM Statistics (if available)
	if len(node.scanner.LastMinute.ILM) > 0 {
		var totalILMCount uint64
		var totalILMTime uint64
		for _, ilmAction := range node.scanner.LastMinute.ILM {
			totalILMCount += ilmAction.Count
			totalILMTime += ilmAction.AccTime
		}
		if totalILMCount > 0 {
			avgILMTime := float64(totalILMTime) / float64(totalILMCount) / 1e6
			data["ILM/min"] = fmt.Sprintf("%s (%.1fms avg)",
				humanize.Comma(int64(totalILMCount)), avgILMTime)
		}
	}

	// Lifetime Totals
	var totalLifetimeOps, totalLifetimeILM uint64
	for _, count := range node.scanner.LifeTimeOps {
		totalLifetimeOps += count
	}
	for _, count := range node.scanner.LifeTimeILM {
		totalLifetimeILM += count
	}

	if totalLifetimeOps > 0 {
		data["Lifetime ops"] = humanize.Comma(int64(totalLifetimeOps))
	}
	if totalLifetimeILM > 0 {
		data["Lifetime ILM"] = humanize.Comma(int64(totalLifetimeILM))
	}

	// System Info
	data["Last updated"] = node.scanner.CollectedAt.Format("15:04:05")

	return data
}

func (node *ScannerMetricsNode) GetMetricType() madmin.MetricType {
	return madmin.MetricsScanner
}

func (node *ScannerMetricsNode) GetMetricFlags() madmin.MetricFlags {
	return 0
}

func (node *ScannerMetricsNode) GetParent() MetricNode {
	return node.parent
}

func (node *ScannerMetricsNode) GetPath() string {
	return node.path
}

func (node *ScannerMetricsNode) ShouldPauseRefresh() bool {
	return false
}

func (node *ScannerMetricsNode) GetChild(name string) (MetricNode, error) {
	if node.scanner == nil {
		return nil, fmt.Errorf("no scanner data available")
	}

	switch name {
	case "lifetime_ops":
		return NewScannerLifetimeOpsNode(node.scanner.LifeTimeOps, node, fmt.Sprintf("%s/lifetime_ops", node.path)), nil
	case "lifetime_ilm":
		return NewScannerLifetimeILMNode(node.scanner.LifeTimeILM, node, fmt.Sprintf("%s/lifetime_ilm", node.path)), nil
	case "last_minute":
		return NewScannerLastMinuteNode(&node.scanner.LastMinute, node, fmt.Sprintf("%s/last_minute", node.path)), nil
	case "active_paths":
		return NewScannerPathsNode(node.scanner.ActivePaths, "active", node, fmt.Sprintf("%s/active_paths", node.path)), nil
	case "excessive_paths":
		return NewScannerPathsNode(node.scanner.ExcessivePrefixes, "excessive", node, fmt.Sprintf("%s/excessive_paths", node.path)), nil
	default:
		return nil, fmt.Errorf("child not found: %s", name)
	}
}

// Helper nodes for scanner sub-components

type ScannerLifetimeOpsNode struct {
	ops    map[string]uint64
	parent MetricNode
	path   string
}

func (node *ScannerLifetimeOpsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewScannerLifetimeOpsNode(ops map[string]uint64, parent MetricNode, path string) *ScannerLifetimeOpsNode {
	return &ScannerLifetimeOpsNode{ops: ops, parent: parent, path: path}
}

func (node *ScannerLifetimeOpsNode) GetChildren() []MetricChild {
	// No children - all operation data should be displayed as leaf data
	return []MetricChild{}
}

func (node *ScannerLifetimeOpsNode) GetLeafData() map[string]string {
	data := map[string]string{}

	if len(node.ops) == 0 {
		data["Status"] = "No lifetime operations recorded"
		return data
	}

	var total uint64
	opTypes := make([]string, 0, len(node.ops))
	for opType := range node.ops {
		opTypes = append(opTypes, opType)
	}
	sort.Strings(opTypes)

	// Display each operation type with formatted count
	for _, opType := range opTypes {
		count := node.ops[opType]
		data[opType] = fmt.Sprintf("%s operations", humanize.Comma(int64(count)))
		total += count
	}

	// Add total if multiple operation types
	if len(opTypes) > 1 {
		data["Total Operations"] = fmt.Sprintf("%s operations", humanize.Comma(int64(total)))
	}

	return data
}

func (node *ScannerLifetimeOpsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsScanner }
func (node *ScannerLifetimeOpsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *ScannerLifetimeOpsNode) GetParent() MetricNode              { return node.parent }
func (node *ScannerLifetimeOpsNode) GetPath() string                    { return node.path }
func (node *ScannerLifetimeOpsNode) ShouldPauseRefresh() bool {
	return false
}

func (node *ScannerLifetimeOpsNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children available - operation counts are displayed as leaf data")
}

type ScannerLifetimeILMNode struct {
	ilm    map[string]uint64
	parent MetricNode
	path   string
}

func (node *ScannerLifetimeILMNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewScannerLifetimeILMNode(ilm map[string]uint64, parent MetricNode, path string) *ScannerLifetimeILMNode {
	return &ScannerLifetimeILMNode{ilm: ilm, parent: parent, path: path}
}

func (node *ScannerLifetimeILMNode) GetChildren() []MetricChild {
	if node.ilm == nil || len(node.ilm) == 0 {
		return []MetricChild{}
	}
	children := make([]MetricChild, 0, len(node.ilm))
	for ilmType := range node.ilm {
		children = append(children, MetricChild{
			Name:        ilmType,
			Description: fmt.Sprintf("Count for ILM operation type %s", ilmType),
		})
	}
	sort.Slice(children, func(i, j int) bool {
		return children[i].Name < children[j].Name
	})
	return children
}

func (node *ScannerLifetimeILMNode) GetLeafData() map[string]string {
	data := map[string]string{
		"ilm_types": strconv.Itoa(len(node.ilm)),
	}
	var total uint64
	for ilmType, count := range node.ilm {
		data[ilmType] = strconv.FormatUint(count, 10)
		total += count
	}
	data["total"] = strconv.FormatUint(total, 10)
	return data
}

func (node *ScannerLifetimeILMNode) GetMetricType() madmin.MetricType   { return madmin.MetricsScanner }
func (node *ScannerLifetimeILMNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *ScannerLifetimeILMNode) GetParent() MetricNode              { return node.parent }
func (node *ScannerLifetimeILMNode) GetPath() string                    { return node.path }
func (node *ScannerLifetimeILMNode) ShouldPauseRefresh() bool {
	return false
}

func (node *ScannerLifetimeILMNode) GetChild(name string) (MetricNode, error) {
	if count, exists := node.ilm[name]; exists {
		return NewScannerOpCountNode(name, count, node, fmt.Sprintf("%s/%s", node.path, name)), nil
	}
	return nil, fmt.Errorf("ILM operation type not found: %s", name)
}

type ScannerLastMinuteNode struct {
	lastMinute *struct {
		Actions map[string]madmin.TimedAction `json:"actions,omitempty"`
		ILM     map[string]madmin.TimedAction `json:"ilm,omitempty"`
	}
	parent MetricNode
	path   string
}

func (node *ScannerLastMinuteNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewScannerLastMinuteNode(lastMinute *struct {
	Actions map[string]madmin.TimedAction `json:"actions,omitempty"`
	ILM     map[string]madmin.TimedAction `json:"ilm,omitempty"`
}, parent MetricNode, path string,
) *ScannerLastMinuteNode {
	return &ScannerLastMinuteNode{lastMinute: lastMinute, parent: parent, path: path}
}

func (node *ScannerLastMinuteNode) GetChildren() []MetricChild {
	var children []MetricChild
	// Only show ILM as a child if it has data - actions should be displayed inline
	if len(node.lastMinute.ILM) > 0 {
		children = append(children, MetricChild{
			Name:        "ilm",
			Description: "ILM actions performed in the last minute",
		})
	}
	return children
}

func (node *ScannerLastMinuteNode) GetLeafData() map[string]string {
	data := map[string]string{}

	// Add action statistics directly
	if len(node.lastMinute.Actions) > 0 {
		var totalCount, totalTime uint64
		var actionTypes []string
		for actionType := range node.lastMinute.Actions {
			actionTypes = append(actionTypes, actionType)
		}
		sort.Strings(actionTypes)

		for _, actionType := range actionTypes {
			action := node.lastMinute.Actions[actionType]
			if action.Count > 0 {
				avgTime := float64(action.AccTime) / float64(action.Count)
				data[actionType] = fmt.Sprintf("%d operations, %.2f ms avg", action.Count, avgTime/1e6)
				totalCount += action.Count
				totalTime += action.AccTime
			} else {
				data[actionType] = "0 operations"
			}
		}

		// Add total if multiple action types
		if len(actionTypes) > 1 && totalCount > 0 {
			avgTime := float64(totalTime) / float64(totalCount)
			data["Total Actions"] = fmt.Sprintf("%d operations, %.2f ms avg", totalCount, avgTime/1e6)
		}
	} else {
		data["Actions"] = "No scanner actions in the last minute"
	}

	// Add ILM summary
	if len(node.lastMinute.ILM) > 0 {
		data["ILM Types Available"] = strconv.Itoa(len(node.lastMinute.ILM))
	}

	return data
}

func (node *ScannerLastMinuteNode) GetMetricType() madmin.MetricType   { return madmin.MetricsScanner }
func (node *ScannerLastMinuteNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *ScannerLastMinuteNode) GetParent() MetricNode              { return node.parent }
func (node *ScannerLastMinuteNode) GetPath() string                    { return node.path }
func (node *ScannerLastMinuteNode) ShouldPauseRefresh() bool {
	return false
}

func (node *ScannerLastMinuteNode) GetChild(name string) (MetricNode, error) {
	switch name {
	case "ilm":
		return NewScannerTimedActionsNode(node.lastMinute.ILM, node, fmt.Sprintf("%s/ilm", node.path)), nil
	default:
		return nil, fmt.Errorf("child not found: %s", name)
	}
}

type ScannerTimedActionsNode struct {
	actions map[string]madmin.TimedAction
	parent  MetricNode
	path    string
}

func (node *ScannerTimedActionsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewScannerTimedActionsNode(actions map[string]madmin.TimedAction, parent MetricNode, path string) *ScannerTimedActionsNode {
	return &ScannerTimedActionsNode{actions: actions, parent: parent, path: path}
}

func (node *ScannerTimedActionsNode) GetChildren() []MetricChild {
	// No children - all action data should be displayed as leaf data
	return []MetricChild{}
}

func (node *ScannerTimedActionsNode) GetLeafData() map[string]string {
	data := map[string]string{}

	if len(node.actions) == 0 {
		data["Status"] = "No scanner actions recorded in the last minute"
		return data
	}

	var totalCount, totalTime uint64

	// Sort action types for consistent display
	actionTypes := make([]string, 0, len(node.actions))
	for actionType := range node.actions {
		actionTypes = append(actionTypes, actionType)
	}
	sort.Strings(actionTypes)

	// Display stats for each action type
	for _, actionType := range actionTypes {
		action := node.actions[actionType]

		if action.Count > 0 {
			avgTime := float64(action.AccTime) / float64(action.Count)
			data[actionType] = fmt.Sprintf("%d operations, %.2f ms avg", action.Count, avgTime/1e6) // Convert nanoseconds to milliseconds
			totalCount += action.Count
			totalTime += action.AccTime
		} else {
			data[actionType] = "0 operations"
		}
	}

	// Add totals if there are multiple action types
	if len(actionTypes) > 1 {
		if totalCount > 0 {
			avgTime := float64(totalTime) / float64(totalCount)
			data["Total"] = fmt.Sprintf("%d operations, %.2f ms avg", totalCount, avgTime/1e6)
		} else {
			data["Total"] = "0 operations"
		}
	}

	return data
}

func (node *ScannerTimedActionsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsScanner }
func (node *ScannerTimedActionsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *ScannerTimedActionsNode) GetParent() MetricNode              { return node.parent }
func (node *ScannerTimedActionsNode) GetPath() string                    { return node.path }
func (node *ScannerTimedActionsNode) ShouldPauseRefresh() bool {
	return false
}

func (node *ScannerTimedActionsNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children available - actions are displayed as leaf data")
}

type ScannerTimedActionNode struct {
	actionType string
	action     *madmin.TimedAction
	parent     MetricNode
	path       string
}

func (node *ScannerTimedActionNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewScannerTimedActionNode(actionType string, action *madmin.TimedAction, parent MetricNode, path string) *ScannerTimedActionNode {
	return &ScannerTimedActionNode{actionType: actionType, action: action, parent: parent, path: path}
}

func (node *ScannerTimedActionNode) GetChildren() []MetricChild { return []MetricChild{} }
func (node *ScannerTimedActionNode) GetLeafData() map[string]string {
	return map[string]string{
		"action_type": node.actionType,
		"count":       strconv.FormatUint(node.action.Count, 10),
		"acc_time":    strconv.FormatUint(node.action.AccTime, 10),
		"min_time":    strconv.FormatUint(node.action.MinTime, 10),
		"max_time":    strconv.FormatUint(node.action.MaxTime, 10),
		"bytes":       strconv.FormatUint(node.action.Bytes, 10),
	}
}
func (node *ScannerTimedActionNode) GetMetricType() madmin.MetricType   { return madmin.MetricsScanner }
func (node *ScannerTimedActionNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *ScannerTimedActionNode) GetParent() MetricNode              { return node.parent }
func (node *ScannerTimedActionNode) GetPath() string                    { return node.path }
func (node *ScannerTimedActionNode) ShouldPauseRefresh() bool {
	return false
}

func (node *ScannerTimedActionNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("timed action is a leaf node")
}

type ScannerPathsNode struct {
	paths    []string
	pathType string // "active" or "excessive"
	parent   MetricNode
	path     string
}

func (node *ScannerPathsNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewScannerPathsNode(paths []string, pathType string, parent MetricNode, path string) *ScannerPathsNode {
	return &ScannerPathsNode{paths: paths, pathType: pathType, parent: parent, path: path}
}

func (node *ScannerPathsNode) GetChildren() []MetricChild { return []MetricChild{} }
func (node *ScannerPathsNode) GetLeafData() map[string]string {
	data := map[string]string{
		"path_type":  node.pathType,
		"path_count": strconv.Itoa(len(node.paths)),
	}
	for i, path := range node.paths {
		data[fmt.Sprintf("path_%d", i)] = path
	}
	return data
}
func (node *ScannerPathsNode) GetMetricType() madmin.MetricType   { return madmin.MetricsScanner }
func (node *ScannerPathsNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *ScannerPathsNode) GetParent() MetricNode              { return node.parent }
func (node *ScannerPathsNode) GetPath() string                    { return node.path }
func (node *ScannerPathsNode) ShouldPauseRefresh() bool {
	return false
}

func (node *ScannerPathsNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("paths node is a leaf node")
}

type ScannerOpCountNode struct {
	opType string
	count  uint64
	parent MetricNode
	path   string
}

func (node *ScannerOpCountNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func NewScannerOpCountNode(opType string, count uint64, parent MetricNode, path string) *ScannerOpCountNode {
	return &ScannerOpCountNode{opType: opType, count: count, parent: parent, path: path}
}

func (node *ScannerOpCountNode) GetChildren() []MetricChild { return []MetricChild{} }
func (node *ScannerOpCountNode) GetLeafData() map[string]string {
	return map[string]string{
		"operation_type": node.opType,
		"count":          strconv.FormatUint(node.count, 10),
	}
}
func (node *ScannerOpCountNode) GetMetricType() madmin.MetricType   { return madmin.MetricsScanner }
func (node *ScannerOpCountNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *ScannerOpCountNode) GetParent() MetricNode              { return node.parent }
func (node *ScannerOpCountNode) GetPath() string                    { return node.path }
func (node *ScannerOpCountNode) ShouldPauseRefresh() bool {
	return false
}

func (node *ScannerOpCountNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("operation count is a leaf node")
}
