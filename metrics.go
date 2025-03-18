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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime/metrics"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/procfs"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/load"
)

//msgp:clearomitted
//msgp:tag json
//go:generate msgp -unexported

// MetricType is a bitfield representation of different metric types.
type MetricType uint32

// MetricsNone indicates no metrics.
const MetricsNone MetricType = 0

const (
	MetricsScanner MetricType = 1 << (iota)
	MetricsDisk
	MetricsOS
	MetricsBatchJobs
	MetricsSiteResync
	MetricNet
	MetricsMem
	MetricsCPU
	MetricsRPC
	MetricsRuntime

	// MetricsAll must be last.
	// Enables all metrics.
	MetricsAll = 1<<(iota) - 1
)

// MetricsOptions are options provided to Metrics call.
type MetricsOptions struct {
	Type     MetricType    // Return only these metric types. Several types can be combined using |. Leave at 0 to return all.
	N        int           // Maximum number of samples to return. 0 will return endless stream.
	Interval time.Duration // Interval between samples. Will be rounded up to 1s.
	Hosts    []string      // Leave empty for all
	ByHost   bool          // Return metrics by host.
	Disks    []string
	ByDisk   bool
	ByJobID  string
	ByDepID  string
}

// Metrics makes an admin call to retrieve metrics.
// The provided function is called for each received entry.
func (adm *AdminClient) Metrics(ctx context.Context, o MetricsOptions, out func(RealtimeMetrics)) (err error) {
	path := fmt.Sprintf(adminAPIPrefix + "/metrics")
	q := make(url.Values)
	q.Set("types", strconv.FormatUint(uint64(o.Type), 10))
	q.Set("n", strconv.Itoa(o.N))
	q.Set("interval", o.Interval.String())
	q.Set("hosts", strings.Join(o.Hosts, ","))
	if o.ByHost {
		q.Set("by-host", "true")
	}
	q.Set("disks", strings.Join(o.Disks, ","))
	if o.ByDisk {
		q.Set("by-disk", "true")
	}
	if o.ByJobID != "" {
		q.Set("by-jobID", o.ByJobID)
	}
	if o.ByDepID != "" {
		q.Set("by-depID", o.ByDepID)
	}

	resp, err := adm.executeMethod(ctx,
		http.MethodGet, requestData{
			relPath:     path,
			queryValues: q,
		},
	)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		closeResponse(resp)
		return httpRespToErrorResponse(resp)
	}
	defer closeResponse(resp)
	dec := json.NewDecoder(resp.Body)
	for {
		var m RealtimeMetrics
		err := dec.Decode(&m)
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = io.ErrUnexpectedEOF
			}
			return err
		}
		out(m)
		if m.Final {
			break
		}
	}
	return nil
}

// Contains returns whether m contains all of x.
func (m MetricType) Contains(x MetricType) bool {
	return m&x == x
}

// RealtimeMetrics provides realtime metrics.
// This is intended to be expanded over time to cover more types.
type RealtimeMetrics struct {
	// Error indicates an error occurred.
	Errors []string `json:"errors,omitempty"`
	// Hosts indicates the scanned hosts
	Hosts      []string              `json:"hosts"`
	Aggregated Metrics               `json:"aggregated"`
	ByHost     map[string]Metrics    `json:"by_host,omitempty"`
	ByDisk     map[string]DiskMetric `json:"by_disk,omitempty"`
	// Final indicates whether this is the final packet and the receiver can exit.
	Final bool `json:"final"`
}

// Metrics contains all metric types.
type Metrics struct {
	Scanner    *ScannerMetrics    `json:"scanner,omitempty"`
	Disk       *DiskMetric        `json:"disk,omitempty"`
	OS         *OSMetrics         `json:"os,omitempty"`
	BatchJobs  *BatchJobMetrics   `json:"batchJobs,omitempty"`
	SiteResync *SiteResyncMetrics `json:"siteResync,omitempty"`
	Net        *NetMetrics        `json:"net,omitempty"`
	Mem        *MemMetrics        `json:"mem,omitempty"`
	CPU        *CPUMetrics        `json:"cpu,omitempty"`
	RPC        *RPCMetrics        `json:"rpc,omitempty"`
	Go         *RuntimeMetrics    `json:"go,omitempty"`
}

// Merge other into r.
func (r *Metrics) Merge(other *Metrics) {
	if other == nil {
		return
	}
	if r.Scanner == nil && other.Scanner != nil {
		r.Scanner = &ScannerMetrics{}
	}
	r.Scanner.Merge(other.Scanner)

	if r.Disk == nil && other.Disk != nil {
		r.Disk = &DiskMetric{}
	}
	r.Disk.Merge(other.Disk)

	if r.OS == nil && other.OS != nil {
		r.OS = &OSMetrics{}
	}
	r.OS.Merge(other.OS)
	if r.BatchJobs == nil && other.BatchJobs != nil {
		r.BatchJobs = &BatchJobMetrics{}
	}
	r.BatchJobs.Merge(other.BatchJobs)

	if r.SiteResync == nil && other.SiteResync != nil {
		r.SiteResync = &SiteResyncMetrics{}
	}
	r.SiteResync.Merge(other.SiteResync)

	if r.Net == nil && other.Net != nil {
		r.Net = &NetMetrics{}
	}
	r.Net.Merge(other.Net)
	if r.RPC == nil && other.RPC != nil {
		r.RPC = &RPCMetrics{}
	}
	r.RPC.Merge(other.RPC)
	if r.Go == nil && other.Go != nil {
		r.Go = &RuntimeMetrics{}
	}
	r.Go.Merge(other.Go)
}

// Merge will merge other into r.
func (r *RealtimeMetrics) Merge(other *RealtimeMetrics) {
	if other == nil {
		return
	}

	if len(other.Errors) > 0 {
		r.Errors = append(r.Errors, other.Errors...)
	}

	if r.ByHost == nil && len(other.ByHost) > 0 {
		r.ByHost = make(map[string]Metrics, len(other.ByHost))
	}
	for host, metrics := range other.ByHost {
		r.ByHost[host] = metrics
	}

	r.Hosts = append(r.Hosts, other.Hosts...)
	r.Aggregated.Merge(&other.Aggregated)
	sort.Strings(r.Hosts)

	// Gather per disk metrics
	if r.ByDisk == nil && len(other.ByDisk) > 0 {
		r.ByDisk = make(map[string]DiskMetric, len(other.ByDisk))
	}
	for disk, metrics := range other.ByDisk {
		r.ByDisk[disk] = metrics
	}
}

// ScannerMetrics contains scanner information.
type ScannerMetrics struct {
	// Time these metrics were collected
	CollectedAt time.Time `json:"collected"`

	CurrentCycle      uint64      `json:"current_cycle"`        // Deprecated Mar 2024
	CurrentStarted    time.Time   `json:"current_started"`      // Deprecated Mar 2024
	CyclesCompletedAt []time.Time `json:"cycle_complete_times"` // Deprecated Mar 2024

	// Number of buckets currently scanning
	OngoingBuckets int `json:"ongoing_buckets"`

	// Stats per bucket, a map between bucket name and scan stats in all erasure sets
	PerBucketStats map[string][]BucketScanInfo `json:"per_bucket_stats,omitempty"`

	// Number of accumulated operations by type since server restart.
	LifeTimeOps map[string]uint64 `json:"life_time_ops,omitempty"`

	// Number of accumulated ILM operations by type since server restart.
	LifeTimeILM map[string]uint64 `json:"ilm_ops,omitempty"`

	// Last minute operation statistics.
	LastMinute struct {
		// Scanner actions.
		Actions map[string]TimedAction `json:"actions,omitempty"`
		// ILM actions.
		ILM map[string]TimedAction `json:"ilm,omitempty"`
	} `json:"last_minute"`

	// Currently active path(s) being scanned.
	ActivePaths []string `json:"active,omitempty"`
}

// Merge other into 's'.
func (s *ScannerMetrics) Merge(other *ScannerMetrics) {
	if other == nil {
		return
	}

	if s.CollectedAt.Before(other.CollectedAt) {
		// Use latest timestamp
		s.CollectedAt = other.CollectedAt
	}

	if s.OngoingBuckets < other.OngoingBuckets {
		s.OngoingBuckets = other.OngoingBuckets
	}

	if s.PerBucketStats == nil {
		s.PerBucketStats = make(map[string][]BucketScanInfo)
	}
	for bucket, otherSt := range other.PerBucketStats {
		if len(otherSt) == 0 {
			continue
		}
		_, ok := s.PerBucketStats[bucket]
		if !ok {
			s.PerBucketStats[bucket] = otherSt
		}
	}

	if s.CurrentCycle < other.CurrentCycle {
		s.CurrentCycle = other.CurrentCycle
		s.CyclesCompletedAt = other.CyclesCompletedAt
		s.CurrentStarted = other.CurrentStarted
	}
	if len(other.CyclesCompletedAt) > len(s.CyclesCompletedAt) {
		s.CyclesCompletedAt = other.CyclesCompletedAt
	}

	// Regular ops
	if len(other.LifeTimeOps) > 0 && s.LifeTimeOps == nil {
		s.LifeTimeOps = make(map[string]uint64, len(other.LifeTimeOps))
	}
	for k, v := range other.LifeTimeOps {
		total := s.LifeTimeOps[k] + v
		s.LifeTimeOps[k] = total
	}
	if s.LastMinute.Actions == nil && len(other.LastMinute.Actions) > 0 {
		s.LastMinute.Actions = make(map[string]TimedAction, len(other.LastMinute.Actions))
	}
	for k, v := range other.LastMinute.Actions {
		total := s.LastMinute.Actions[k]
		total.Merge(v)
		s.LastMinute.Actions[k] = total
	}

	// ILM
	if len(other.LifeTimeILM) > 0 && s.LifeTimeILM == nil {
		s.LifeTimeILM = make(map[string]uint64, len(other.LifeTimeILM))
	}
	for k, v := range other.LifeTimeILM {
		total := s.LifeTimeILM[k] + v
		s.LifeTimeILM[k] = total
	}
	if s.LastMinute.ILM == nil && len(other.LastMinute.ILM) > 0 {
		s.LastMinute.ILM = make(map[string]TimedAction, len(other.LastMinute.ILM))
	}
	for k, v := range other.LastMinute.ILM {
		total := s.LastMinute.ILM[k]
		total.Merge(v)
		s.LastMinute.ILM[k] = total
	}
	s.ActivePaths = append(s.ActivePaths, other.ActivePaths...)
	sort.Strings(s.ActivePaths)
}

// DiskIOStats contains IO stats of a single drive
type DiskIOStats struct {
	ReadIOs        uint64 `json:"read_ios"`
	ReadMerges     uint64 `json:"read_merges"`
	ReadSectors    uint64 `json:"read_sectors"`
	ReadTicks      uint64 `json:"read_ticks"`
	WriteIOs       uint64 `json:"write_ios"`
	WriteMerges    uint64 `json:"write_merges"`
	WriteSectors   uint64 `json:"wrte_sectors"`
	WriteTicks     uint64 `json:"write_ticks"`
	CurrentIOs     uint64 `json:"current_ios"`
	TotalTicks     uint64 `json:"total_ticks"`
	ReqTicks       uint64 `json:"req_ticks"`
	DiscardIOs     uint64 `json:"discard_ios"`
	DiscardMerges  uint64 `json:"discard_merges"`
	DiscardSectors uint64 `json:"discard_secotrs"`
	DiscardTicks   uint64 `json:"discard_ticks"`
	FlushIOs       uint64 `json:"flush_ios"`
	FlushTicks     uint64 `json:"flush_ticks"`
}

// DiskMetric contains metrics for one or more disks.
type DiskMetric struct {
	// Time these metrics were collected
	CollectedAt time.Time `json:"collected"`

	// Number of disks
	NDisks int `json:"n_disks"`

	// Offline disks
	Offline int `json:"offline,omitempty"`

	// Healing disks
	Healing int `json:"healing,omitempty"`

	// Number of accumulated operations by type since server restart.
	LifeTimeOps map[string]uint64 `json:"life_time_ops,omitempty"`

	// Last minute statistics.
	LastMinute struct {
		Operations map[string]TimedAction `json:"operations,omitempty"`
	} `json:"last_minute"`

	IOStats DiskIOStats `json:"iostats,omitempty"`
}

// Merge other into 's'.
func (d *DiskMetric) Merge(other *DiskMetric) {
	if other == nil {
		return
	}
	if d.CollectedAt.Before(other.CollectedAt) {
		// Use latest timestamp
		d.CollectedAt = other.CollectedAt
	}
	d.NDisks += other.NDisks
	d.Offline += other.Offline
	d.Healing += other.Healing

	if len(other.LifeTimeOps) > 0 && d.LifeTimeOps == nil {
		d.LifeTimeOps = make(map[string]uint64, len(other.LifeTimeOps))
	}
	for k, v := range other.LifeTimeOps {
		total := d.LifeTimeOps[k] + v
		d.LifeTimeOps[k] = total
	}

	if d.LastMinute.Operations == nil && len(other.LastMinute.Operations) > 0 {
		d.LastMinute.Operations = make(map[string]TimedAction, len(other.LastMinute.Operations))
	}
	for k, v := range other.LastMinute.Operations {
		total := d.LastMinute.Operations[k]
		total.Merge(v)
		d.LastMinute.Operations[k] = total
	}
}

// OSMetrics contains metrics for OS operations.
type OSMetrics struct {
	// Time these metrics were collected
	CollectedAt time.Time `json:"collected"`

	// Number of accumulated operations by type since server restart.
	LifeTimeOps map[string]uint64 `json:"life_time_ops,omitempty"`

	// Last minute statistics.
	LastMinute struct {
		Operations map[string]TimedAction `json:"operations,omitempty"`
	} `json:"last_minute"`
}

// Merge other into 'o'.
func (o *OSMetrics) Merge(other *OSMetrics) {
	if other == nil {
		return
	}
	if o.CollectedAt.Before(other.CollectedAt) {
		// Use latest timestamp
		o.CollectedAt = other.CollectedAt
	}

	if len(other.LifeTimeOps) > 0 && o.LifeTimeOps == nil {
		o.LifeTimeOps = make(map[string]uint64, len(other.LifeTimeOps))
	}
	for k, v := range other.LifeTimeOps {
		total := o.LifeTimeOps[k] + v
		o.LifeTimeOps[k] = total
	}

	if o.LastMinute.Operations == nil && len(other.LastMinute.Operations) > 0 {
		o.LastMinute.Operations = make(map[string]TimedAction, len(other.LastMinute.Operations))
	}
	for k, v := range other.LastMinute.Operations {
		total := o.LastMinute.Operations[k]
		total.Merge(v)
		o.LastMinute.Operations[k] = total
	}
}

// BatchJobMetrics contains metrics for batch operations
type BatchJobMetrics struct {
	// Time these metrics were collected
	CollectedAt time.Time `json:"collected"`

	// Jobs by ID.
	Jobs map[string]JobMetric
}

type JobMetric struct {
	JobID         string    `json:"jobID"`
	JobType       string    `json:"jobType"`
	StartTime     time.Time `json:"startTime"`
	LastUpdate    time.Time `json:"lastUpdate"`
	RetryAttempts int       `json:"retryAttempts"`

	Complete bool `json:"complete"`
	Failed   bool `json:"failed"`

	// Specific job type data:
	Replicate *ReplicateInfo   `json:"replicate,omitempty"`
	KeyRotate *KeyRotationInfo `json:"rotation,omitempty"`
	Expired   *ExpirationInfo  `json:"expired,omitempty"`
	Catalog   *CatalogInfo     `json:"catalog,omitempty"`
}

type ReplicateInfo struct {
	// Last bucket/object batch replicated
	Bucket string `json:"lastBucket"`
	Object string `json:"lastObject"`

	// Verbose information
	Objects          int64 `json:"objects"`
	ObjectsFailed    int64 `json:"objectsFailed"`
	BytesTransferred int64 `json:"bytesTransferred"`
	BytesFailed      int64 `json:"bytesFailed"`
}

type ExpirationInfo struct {
	// Last bucket/object key rotated
	Bucket string `json:"lastBucket"`
	Object string `json:"lastObject"`

	// Verbose information
	Objects       int64 `json:"objects"`
	ObjectsFailed int64 `json:"objectsFailed"`
}

type KeyRotationInfo struct {
	// Last bucket/object key rotated
	Bucket string `json:"lastBucket"`
	Object string `json:"lastObject"`

	// Verbose information
	Objects       int64 `json:"objects"`
	ObjectsFailed int64 `json:"objectsFailed"`
}

type CatalogInfo struct {
	LastBucketScanned string `json:"lastBucketScanned"`
	LastObjectScanned string `json:"lastObjectScanned"`
	LastBucketMatched string `json:"lastBucketMatched"`
	LastObjectMatched string `json:"lastObjectMatched"`

	ObjectsScannedCount uint64 `json:"objectsScannedCount"`
	ObjectsMatchedCount uint64 `json:"objectsMatchedCount"`

	// Represents the number of objects' metadata that were written to output
	// objects.
	RecordsWrittenCount uint64 `json:"recordsWrittenCount"`
	// Represents the number of output objects created.
	OutputObjectsCount uint64 `json:"outputObjectsCount"`
	// Manifest file path (part of the output of a catalog job)
	ManifestPathBucket string `json:"manifestPathBucket"`
	ManifestPathObject string `json:"manifestPathObject"`

	// Error message
	ErrorMsg string `json:"errorMsg"`
}

// Merge other into 'o'.
func (o *BatchJobMetrics) Merge(other *BatchJobMetrics) {
	if other == nil || len(other.Jobs) == 0 {
		return
	}
	if o.CollectedAt.Before(other.CollectedAt) {
		// Use latest timestamp
		o.CollectedAt = other.CollectedAt
	}
	if o.Jobs == nil {
		o.Jobs = make(map[string]JobMetric, len(other.Jobs))
	}
	// Job
	for k, v := range other.Jobs {
		o.Jobs[k] = v
	}
}

// SiteResyncMetrics contains metrics for site resync operation
type SiteResyncMetrics struct {
	// Time these metrics were collected
	CollectedAt time.Time `json:"collected"`
	// Status of resync operation
	ResyncStatus string    `json:"resyncStatus,omitempty"`
	StartTime    time.Time `json:"startTime"`
	LastUpdate   time.Time `json:"lastUpdate"`
	NumBuckets   int64     `json:"numBuckets"`
	ResyncID     string    `json:"resyncID"`
	DeplID       string    `json:"deplID"`

	// Completed size in bytes
	ReplicatedSize int64 `json:"completedReplicationSize"`
	// Total number of objects replicated
	ReplicatedCount int64 `json:"replicationCount"`
	// Failed size in bytes
	FailedSize int64 `json:"failedReplicationSize"`
	// Total number of failed operations
	FailedCount int64 `json:"failedReplicationCount"`
	// Buckets that could not be synced
	FailedBuckets []string `json:"failedBuckets"`
	// Last bucket/object replicated.
	Bucket string `json:"bucket,omitempty"`
	Object string `json:"object,omitempty"`
}

func (o SiteResyncMetrics) Complete() bool {
	return strings.ToLower(o.ResyncStatus) == "completed"
}

// Merge other into 'o'.
func (o *SiteResyncMetrics) Merge(other *SiteResyncMetrics) {
	if other == nil {
		return
	}
	if o.CollectedAt.Before(other.CollectedAt) {
		// Use latest
		*o = *other
	}
}

type NetMetrics struct {
	// Time these metrics were collected
	CollectedAt time.Time `json:"collected"`

	// net of Interface
	InterfaceName string `json:"interfaceName"`

	NetStats procfs.NetDevLine `json:"netstats"`
}

//msgp:replace procfs.NetDevLine with:procfsNetDevLine

// Merge other into 'o'.
func (n *NetMetrics) Merge(other *NetMetrics) {
	if other == nil {
		return
	}
	if n.CollectedAt.Before(other.CollectedAt) {
		// Use latest timestamp
		n.CollectedAt = other.CollectedAt
	}
	n.NetStats.RxBytes += other.NetStats.RxBytes
	n.NetStats.RxPackets += other.NetStats.RxPackets
	n.NetStats.RxErrors += other.NetStats.RxErrors
	n.NetStats.RxDropped += other.NetStats.RxDropped
	n.NetStats.RxFIFO += other.NetStats.RxFIFO
	n.NetStats.RxFrame += other.NetStats.RxFrame
	n.NetStats.RxCompressed += other.NetStats.RxCompressed
	n.NetStats.RxMulticast += other.NetStats.RxMulticast
	n.NetStats.TxBytes += other.NetStats.TxBytes
	n.NetStats.TxPackets += other.NetStats.TxPackets
	n.NetStats.TxErrors += other.NetStats.TxErrors
	n.NetStats.TxDropped += other.NetStats.TxDropped
	n.NetStats.TxFIFO += other.NetStats.TxFIFO
	n.NetStats.TxCollisions += other.NetStats.TxCollisions
	n.NetStats.TxCarrier += other.NetStats.TxCarrier
	n.NetStats.TxCompressed += other.NetStats.TxCompressed
}

//msgp:replace NodeCommon with:nodeCommon

// nodeCommon - use as replacement for NodeCommon
// We do not want to give NodeCommon codegen, since it is used for embedding.
type nodeCommon struct {
	Addr  string `json:"addr"`
	Error string `json:"error,omitempty"`
}

// MemInfo contains system's RAM and swap information.
type MemInfo struct {
	NodeCommon

	Total          uint64 `json:"total,omitempty"`
	Used           uint64 `json:"used,omitempty"`
	Free           uint64 `json:"free,omitempty"`
	Available      uint64 `json:"available,omitempty"`
	Shared         uint64 `json:"shared,omitempty"`
	Cache          uint64 `json:"cache,omitempty"`
	Buffers        uint64 `json:"buffer,omitempty"`
	SwapSpaceTotal uint64 `json:"swap_space_total,omitempty"`
	SwapSpaceFree  uint64 `json:"swap_space_free,omitempty"`
	// Limit will store cgroup limit if configured and
	// less than Total, otherwise same as Total
	Limit uint64 `json:"limit,omitempty"`
}

type MemMetrics struct {
	// Time these metrics were collected
	CollectedAt time.Time `json:"collected"`

	Info MemInfo `json:"memInfo"`
}

// Merge other into 'm'.
func (m *MemMetrics) Merge(other *MemMetrics) {
	if m.CollectedAt.Before(other.CollectedAt) {
		// Use latest timestamp
		m.CollectedAt = other.CollectedAt
	}

	m.Info.Total += other.Info.Total
	m.Info.Available += other.Info.Available
	m.Info.SwapSpaceTotal += other.Info.SwapSpaceTotal
	m.Info.SwapSpaceFree += other.Info.SwapSpaceFree
	m.Info.Limit += other.Info.Limit
}

//msgp:replace cpu.TimesStat with:cpuTimesStat
//msgp:replace load.AvgStat with:loadAvgStat

type CPUMetrics struct {
	// Time these metrics were collected
	CollectedAt time.Time `json:"collected"`

	TimesStat *cpu.TimesStat `json:"timesStat"`
	LoadStat  *load.AvgStat  `json:"loadStat"`
	CPUCount  int            `json:"cpuCount"`
}

// Merge other into 'm'.
func (m *CPUMetrics) Merge(other *CPUMetrics) {
	if m.CollectedAt.Before(other.CollectedAt) {
		// Use latest timestamp
		m.CollectedAt = other.CollectedAt
	}
	m.TimesStat.User += other.TimesStat.User
	m.TimesStat.System += other.TimesStat.System
	m.TimesStat.Idle += other.TimesStat.Idle
	m.TimesStat.Nice += other.TimesStat.Nice
	m.TimesStat.Iowait += other.TimesStat.Iowait
	m.TimesStat.Irq += other.TimesStat.Irq
	m.TimesStat.Softirq += other.TimesStat.Softirq
	m.TimesStat.Steal += other.TimesStat.Steal
	m.TimesStat.Guest += other.TimesStat.Guest
	m.TimesStat.GuestNice += other.TimesStat.GuestNice

	m.LoadStat.Load1 += other.LoadStat.Load1
	m.LoadStat.Load5 += other.LoadStat.Load5
	m.LoadStat.Load15 += other.LoadStat.Load15
}

// RPCMetrics contains metrics for RPC operations.
type RPCMetrics struct {
	CollectedAt      time.Time `json:"collectedAt"`
	Connected        int       `json:"connected"`
	ReconnectCount   int       `json:"reconnectCount"`
	Disconnected     int       `json:"disconnected"`
	OutgoingStreams  int       `json:"outgoingStreams"`
	IncomingStreams  int       `json:"incomingStreams"`
	OutgoingBytes    int64     `json:"outgoingBytes"`
	IncomingBytes    int64     `json:"incomingBytes"`
	OutgoingMessages int64     `json:"outgoingMessages"`
	IncomingMessages int64     `json:"incomingMessages"`
	OutQueue         int       `json:"outQueue"`
	LastPongTime     time.Time `json:"lastPongTime"`
	LastPingMS       float64   `json:"lastPingMS"`
	MaxPingDurMS     float64   `json:"maxPingDurMS"` // Maximum across all merged entries.
	LastConnectTime  time.Time `json:"lastConnectTime"`

	ByDestination map[string]RPCMetrics `json:"byDestination,omitempty"`
	ByCaller      map[string]RPCMetrics `json:"byCaller,omitempty"`
}

// Merge other into 'm'.
func (m *RPCMetrics) Merge(other *RPCMetrics) {
	if m == nil || other == nil {
		return
	}
	if m.CollectedAt.Before(other.CollectedAt) {
		// Use latest timestamp
		m.CollectedAt = other.CollectedAt
	}
	if m.LastConnectTime.Before(other.LastConnectTime) {
		m.LastConnectTime = other.LastConnectTime
	}
	m.Connected += other.Connected
	m.Disconnected += other.Disconnected
	m.ReconnectCount += other.ReconnectCount
	m.OutgoingStreams += other.OutgoingStreams
	m.IncomingStreams += other.IncomingStreams
	m.OutgoingBytes += other.OutgoingBytes
	m.IncomingBytes += other.IncomingBytes
	m.OutgoingMessages += other.OutgoingMessages
	m.IncomingMessages += other.IncomingMessages
	m.OutQueue += other.OutQueue
	if m.LastPongTime.Before(other.LastPongTime) {
		m.LastPongTime = other.LastPongTime
		m.LastPingMS = other.LastPingMS
	}
	if m.MaxPingDurMS < other.MaxPingDurMS {
		m.MaxPingDurMS = other.MaxPingDurMS
	}
	for k, v := range other.ByDestination {
		if m.ByDestination == nil {
			m.ByDestination = make(map[string]RPCMetrics, len(other.ByDestination))
		}
		existing := m.ByDestination[k]
		existing.Merge(&v)
		m.ByDestination[k] = existing
	}
	for k, v := range other.ByCaller {
		if m.ByCaller == nil {
			m.ByCaller = make(map[string]RPCMetrics, len(other.ByCaller))
		}
		existing := m.ByCaller[k]
		existing.Merge(&v)
		m.ByCaller[k] = existing
	}
}

//msgp:replace metrics.Float64Histogram with:localF64H

// local copy of localF64H, can be casted to/from metrics.Float64Histogram
type localF64H struct {
	Counts  []uint64  `json:"counts,omitempty"`
	Buckets []float64 `json:"buckets,omitempty"`
}

// RuntimeMetrics contains metrics for the go runtime.
// See more at https://pkg.go.dev/runtime/metrics
type RuntimeMetrics struct {
	// UintMetrics contains KindUint64 values
	UintMetrics map[string]uint64 `json:"uintMetrics,omitempty"`

	// FloatMetrics contains KindFloat64 values
	FloatMetrics map[string]float64 `json:"floatMetrics,omitempty"`

	// HistMetrics contains KindFloat64Histogram values
	HistMetrics map[string]metrics.Float64Histogram `json:"histMetrics,omitempty"`

	// N tracks the number of merged entries.
	N int `json:"n"`
}

// Merge other into 'm'.
func (m *RuntimeMetrics) Merge(other *RuntimeMetrics) {
	if m == nil || other == nil {
		return
	}
	if m.UintMetrics == nil {
		m.UintMetrics = make(map[string]uint64, len(other.UintMetrics))
	}
	if m.FloatMetrics == nil {
		m.FloatMetrics = make(map[string]float64, len(other.FloatMetrics))
	}
	if m.HistMetrics == nil {
		m.HistMetrics = make(map[string]metrics.Float64Histogram, len(other.HistMetrics))
	}
	for k, v := range other.UintMetrics {
		m.UintMetrics[k] += v
	}
	for k, v := range other.FloatMetrics {
		m.FloatMetrics[k] += v
	}
	for k, v := range other.HistMetrics {
		existing := m.HistMetrics[k]
		if len(existing.Buckets) == 0 {
			m.HistMetrics[k] = v
			continue
		}
		// TODO: Technically, I guess we may have differing buckets,
		// but they should be the same for the runtime.
		if len(existing.Buckets) == len(v.Buckets) {
			for i, count := range v.Counts {
				existing.Counts[i] += count
			}
		}
	}
	m.N += other.N
}
