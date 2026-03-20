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

	"github.com/minio/madmin-go/v4"
)

type NetMetricsNavigator struct {
	net    *madmin.NetMetrics
	parent MetricNode
	path   string
}

func (node *NetMetricsNavigator) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

// NewNetMetricsNavigator creates a new network metrics navigator
func NewNetMetricsNavigator(net *madmin.NetMetrics, parent MetricNode, path string) *NetMetricsNavigator {
	return &NetMetricsNavigator{net: net, parent: parent, path: path}
}

func (node *NetMetricsNavigator) GetPath() string {
	return node.path
}

func (node *NetMetricsNavigator) GetParent() MetricNode {
	return node.parent
}

func (node *NetMetricsNavigator) GetMetricType() madmin.MetricType {
	return madmin.MetricNet
}

func (node *NetMetricsNavigator) GetMetricFlags() madmin.MetricFlags {
	return 0
}

func (node *NetMetricsNavigator) GetChildren() []MetricChild {
	if node.net == nil {
		return []MetricChild{
			{Name: "interfaces", Description: "Network interface statistics"},
			{Name: "internode", Description: "Internode communication stats"},
		}
	}

	var children []MetricChild

	// Add interface nodes
	if len(node.net.Interfaces) > 0 {
		children = append(children, MetricChild{
			Name:        "interfaces",
			Description: fmt.Sprintf("Network interfaces (%d available)", len(node.net.Interfaces)),
		})
	} else {
		children = append(children, MetricChild{
			Name:        "interfaces",
			Description: "Network interfaces",
		})
	}

	// Add internode stats
	children = append(children, MetricChild{
		Name:        "internode",
		Description: "Internode communication statistics",
	})

	children = append(children, MetricChild{Name: "last_day", Description: "Last 24h network statistics"})

	return children
}

func (node *NetMetricsNavigator) GetLeafData() map[string]string {
	if node.net == nil {
		return map[string]string{}
	}

	data := map[string]string{
		"Collection Time": node.net.CollectedAt.Format("2006-01-02 15:04:05"),
		"Interfaces":      fmt.Sprintf("%d", len(node.net.Interfaces)),
	}

	// Add interface summaries
	var totalRxBytes, totalTxBytes int64
	for name, stats := range node.net.Interfaces {
		totalRxBytes += int64(stats.RxBytes)
		totalTxBytes += int64(stats.TxBytes)
		data[fmt.Sprintf("Interface %s RX", name)] = formatBytes(stats.RxBytes)
		data[fmt.Sprintf("Interface %s TX", name)] = formatBytes(stats.TxBytes)
	}

	if totalRxBytes > 0 {
		data["Total RX Bytes"] = formatBytes(uint64(totalRxBytes))
	}
	if totalTxBytes > 0 {
		data["Total TX Bytes"] = formatBytes(uint64(totalTxBytes))
	}

	return data
}

func (node *NetMetricsNavigator) GetChild(name string) (MetricNode, error) {
	switch name {
	case "interfaces":
		return &NetInterfacesNode{
			metrics: node.net,
			parent:  node,
			path:    node.path + "/interfaces",
		}, nil
	case "internode":
		return &NetInternodeNode{
			metrics: node.net,
			parent:  node,
			path:    node.path + "/internode",
		}, nil
	case "last_day":
		return NewNetLastDayNode(node.net.LastDay, node, node.path+"/last_day"), nil
	}
	return nil, fmt.Errorf("child %q not found", name)
}

func (node *NetMetricsNavigator) ShouldPauseRefresh() bool {
	return false
}

// NetInterfacesNode shows network interface stats
type NetInterfacesNode struct {
	metrics *madmin.NetMetrics
	parent  MetricNode
	path    string
}

func (node *NetInterfacesNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *NetInterfacesNode) GetPath() string {
	return node.path
}

func (node *NetInterfacesNode) GetParent() MetricNode {
	return node.parent
}

func (node *NetInterfacesNode) GetMetricType() madmin.MetricType {
	return madmin.MetricNet
}

func (node *NetInterfacesNode) GetMetricFlags() madmin.MetricFlags {
	return 0
}

func (node *NetInterfacesNode) GetChildren() []MetricChild {
	if node.metrics == nil || len(node.metrics.Interfaces) == 0 {
		return []MetricChild{}
	}

	children := make([]MetricChild, 0, len(node.metrics.Interfaces))
	for name, stats := range node.metrics.Interfaces {
		children = append(children, MetricChild{
			Name: name,
			Description: fmt.Sprintf("RX: %s, TX: %s",
				formatBytes(stats.RxBytes),
				formatBytes(stats.TxBytes)),
		})
	}

	return children
}

func (node *NetInterfacesNode) GetLeafData() map[string]string {
	if node.metrics == nil {
		return map[string]string{}
	}

	data := map[string]string{
		"Total Interfaces": strconv.Itoa(len(node.metrics.Interfaces)),
	}

	for name, stats := range node.metrics.Interfaces {
		prefix := fmt.Sprintf("%s ", name)
		data[prefix+"RX Bytes"] = formatBytes(stats.RxBytes)
		data[prefix+"TX Bytes"] = formatBytes(stats.TxBytes)
		data[prefix+"RX Packets"] = formatNumber(stats.RxPackets)
		data[prefix+"TX Packets"] = formatNumber(stats.TxPackets)
		if stats.RxErrors > 0 {
			data[prefix+"RX Errors"] = formatNumber(stats.RxErrors)
		}
		if stats.TxErrors > 0 {
			data[prefix+"TX Errors"] = formatNumber(stats.TxErrors)
		}
	}

	return data
}

func (node *NetInterfacesNode) GetChild(name string) (MetricNode, error) {
	if node.metrics == nil {
		return nil, fmt.Errorf("no metrics available")
	}

	if stats, exists := node.metrics.Interfaces[name]; exists {
		return &NetInterfaceNode{
			interfaceName: name,
			stats:         &stats,
			parent:        node,
			path:          node.path + "/" + name,
		}, nil
	}

	return nil, fmt.Errorf("interface %q not found", name)
}

func (node *NetInterfacesNode) ShouldPauseRefresh() bool {
	return false
}

// NetInterfaceNode shows individual interface stats
type NetInterfaceNode struct {
	interfaceName string
	stats         *madmin.InterfaceStats
	parent        MetricNode
	path          string
}

func (node *NetInterfaceNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *NetInterfaceNode) GetPath() string {
	return node.path
}

func (node *NetInterfaceNode) GetParent() MetricNode {
	return node.parent
}

func (node *NetInterfaceNode) GetMetricType() madmin.MetricType {
	return madmin.MetricNet
}

func (node *NetInterfaceNode) GetMetricFlags() madmin.MetricFlags {
	return 0
}

func (node *NetInterfaceNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *NetInterfaceNode) GetLeafData() map[string]string {
	if node.stats == nil {
		return map[string]string{}
	}

	data := map[string]string{
		"Interface Name": node.interfaceName,
		"RX Bytes":       formatBytes(node.stats.RxBytes),
		"TX Bytes":       formatBytes(node.stats.TxBytes),
		"RX Packets":     formatNumber(node.stats.RxPackets),
		"TX Packets":     formatNumber(node.stats.TxPackets),
	}

	if node.stats.RxErrors > 0 {
		data["RX Errors"] = formatNumber(node.stats.RxErrors)
	}
	if node.stats.TxErrors > 0 {
		data["TX Errors"] = formatNumber(node.stats.TxErrors)
	}
	if node.stats.RxDropped > 0 {
		data["RX Dropped"] = formatNumber(node.stats.RxDropped)
	}
	if node.stats.TxDropped > 0 {
		data["TX Dropped"] = formatNumber(node.stats.TxDropped)
	}

	return data
}

func (node *NetInterfaceNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("interface node has no children")
}

func (node *NetInterfaceNode) ShouldPauseRefresh() bool {
	return false
}

// NetInternodeNode shows internode communication stats
type NetInternodeNode struct {
	metrics *madmin.NetMetrics
	parent  MetricNode
	path    string
}

func (node *NetInternodeNode) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

func (node *NetInternodeNode) GetPath() string {
	return node.path
}

func (node *NetInternodeNode) GetParent() MetricNode {
	return node.parent
}

func (node *NetInternodeNode) GetMetricType() madmin.MetricType {
	return madmin.MetricNet
}

func (node *NetInternodeNode) GetMetricFlags() madmin.MetricFlags {
	return 0
}

func (node *NetInternodeNode) GetChildren() []MetricChild {
	return []MetricChild{}
}

func (node *NetInternodeNode) GetLeafData() map[string]string {
	if node.metrics == nil {
		return map[string]string{}
	}

	netStats := node.metrics.NetStats
	data := map[string]string{
		"Name":       netStats.Name,
		"RX Bytes":   formatBytes(netStats.RxBytes),
		"TX Bytes":   formatBytes(netStats.TxBytes),
		"RX Packets": formatNumber(netStats.RxPackets),
		"TX Packets": formatNumber(netStats.TxPackets),
	}

	if netStats.RxErrors > 0 {
		data["RX Errors"] = formatNumber(netStats.RxErrors)
	}
	if netStats.TxErrors > 0 {
		data["TX Errors"] = formatNumber(netStats.TxErrors)
	}
	if netStats.RxDropped > 0 {
		data["RX Dropped"] = formatNumber(netStats.RxDropped)
	}
	if netStats.TxDropped > 0 {
		data["TX Dropped"] = formatNumber(netStats.TxDropped)
	}

	return data
}

func (node *NetInternodeNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("internode node has no children")
}

func (node *NetInternodeNode) ShouldPauseRefresh() bool {
	return false
}

// NetLastDayNode shows last 24h network statistics
type NetLastDayNode struct {
	segmented *madmin.SegmentedInterfaceStats
	parent    MetricNode
	path      string
}

func NewNetLastDayNode(segmented *madmin.SegmentedInterfaceStats, parent MetricNode, path string) *NetLastDayNode {
	return &NetLastDayNode{segmented: segmented, parent: parent, path: path}
}

func (node *NetLastDayNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *NetLastDayNode) GetPath() string                    { return node.path }
func (node *NetLastDayNode) GetParent() MetricNode              { return node.parent }
func (node *NetLastDayNode) GetMetricType() madmin.MetricType   { return madmin.MetricNet }
func (node *NetLastDayNode) GetMetricFlags() madmin.MetricFlags { return madmin.MetricsDayStats }
func (node *NetLastDayNode) ShouldPauseRefresh() bool           { return true }
func (node *NetLastDayNode) GetChildren() []MetricChild         { return nil }

func (node *NetLastDayNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children")
}

func (node *NetLastDayNode) GetLeafData() map[string]string {
	if node.segmented == nil || len(node.segmented.Segments) == 0 {
		return nil
	}
	data := make(map[string]string)
	idx := 0
	for i := len(node.segmented.Segments) - 1; i >= 0; i-- {
		seg := node.segmented.Segments[i]
		if seg.N == 0 {
			continue
		}
		idx++
		startTime := node.segmented.FirstTime.Add(time.Duration(i*node.segmented.Interval) * time.Second)
		endTime := startTime.Add(time.Duration(node.segmented.Interval) * time.Second)
		name := fmt.Sprintf("%02d: %s->%s", idx, startTime.Local().Format("15:04"), endTime.Local().Format("15:04"))

		avgRx := seg.RxBytes / uint64(seg.N)
		avgTx := seg.TxBytes / uint64(seg.N)
		avgRxPkts := seg.RxPackets / uint64(seg.N)
		avgTxPkts := seg.TxPackets / uint64(seg.N)
		// Calculate Gbps (bytes per interval -> bits per second -> Gbps)
		rxGbps := float64(avgRx) * 8 / float64(node.segmented.Interval) / 1e9
		txGbps := float64(avgTx) * 8 / float64(node.segmented.Interval) / 1e9
		data[name] = fmt.Sprintf("RX: %s (%.2f Gbps, %s pkts), TX: %s (%.2f Gbps, %s pkts)",
			formatBytes(avgRx), rxGbps, formatNumber(avgRxPkts),
			formatBytes(avgTx), txGbps, formatNumber(avgTxPkts))
	}
	return data
}
