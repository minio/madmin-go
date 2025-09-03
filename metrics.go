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
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/tinylib/msgp/msgp"
)

//msgp:clearomitted
//msgp:tag json
//msgp:timezone utc
//go:generate msgp -unexported -file $GOFILE

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
	MetricsAPI

	// MetricsAll must be last.
	// Enables all metrics.
	MetricsAll = 1<<(iota) - 1
)

// MetricsOptions are options provided to Metrics call.
type MetricsOptions struct {
	Type        MetricType    // Return only these metric types. Several types can be combined using |. Leave at 0 to return all.
	N           int           // Maximum number of samples to return. 0 will return endless stream.
	Interval    time.Duration // Interval between samples. Will be rounded up to 1s.
	PoolIdx     []int         // Only include metrics for these pools. Leave empty for all.
	Hosts       []string      // Only include specified hosts. Leave empty for all.
	DriveSetIdx []int         // Only include metrics for these drive sets (combine with PoolIdx if needed).
	Disks       []string      // Include only specific disks. Leave empty for all.
	ByJobID     string
	ByDepID     string

	// Alternative output merging.
	// Populates maps of the same name in the result.
	ByHost bool // Return individual metrics by host.
	ByDisk bool // Return individual metrics by disk.
	ByPool bool // Return individual metrics by pool.
}

// DriveSetPrefix will be used to select drives from specific sets.
const DriveSetPrefix = "::drive-set::"

// Metrics makes an admin call to retrieve metrics.
// The provided function is called for each received entry.
func (adm *AdminClient) Metrics(ctx context.Context, o MetricsOptions, out func(RealtimeMetrics)) (err error) {
	path := adminAPIPrefix + "/metrics"
	q := make(url.Values)
	q.Set("types", strconv.FormatUint(uint64(o.Type), 10))
	q.Set("n", strconv.Itoa(o.N))
	q.Set("interval", o.Interval.String())
	q.Set("hosts", strings.Join(o.Hosts, ","))
	if o.ByHost {
		q.Set("by-host", "true")
	}
	for _, v := range o.DriveSetIdx {
		o.Disks = append(o.Disks, fmt.Sprintf(DriveSetPrefix+"%d", v))
	}
	q.Set("disks", strings.Join(o.Disks, ","))
	if o.ByDisk {
		q.Set("by-disk", "true")
	}
	if o.ByPool {
		q.Set("by-pool", "true")
	}
	if o.ByJobID != "" {
		q.Set("by-jobID", o.ByJobID)
	}
	if o.ByDepID != "" {
		q.Set("by-depID", o.ByDepID)
	}
	if len(o.PoolIdx) > 0 {
		str := make([]string, len(o.PoolIdx))
		for i, id := range o.PoolIdx {
			str[i] = strconv.Itoa(id)
		}
		q.Set("pool-idx", strings.Join(str, ","))
	}

	resp, err := adm.executeMethod(ctx,
		http.MethodGet, requestData{
			customHeaders: map[string][]string{
				"Accept": {"application/vnd.msgpack"},
			},
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

	// Choose decoder based on content type
	var decodeOne func(m *RealtimeMetrics) error
	switch resp.Header.Get("Content-Type") {
	case "application/vnd.msgpack":
		dec := msgp.NewReader(resp.Body)
		decodeOne = func(m *RealtimeMetrics) error {
			return m.DecodeMsg(dec)
		}
	default:
		dec := json.NewDecoder(resp.Body)
		decodeOne = func(m *RealtimeMetrics) error {
			return dec.Decode(m)
		}
	}
	for {
		var m RealtimeMetrics
		err := decodeOne(&m)
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
	Hosts []string `json:"hosts"`

	// Aggregated contains aggregated metrics for all hosts
	Aggregated Metrics `json:"aggregated"`

	// ByHost contains metrics for each host if requested.
	ByHost map[string]Metrics `json:"by_host,omitempty"`

	// ByDisk contains metrics for each disk if requested.
	ByDisk map[string]DiskMetric `json:"by_disk,omitempty"`

	// ByPool contains metrics for each pool if requested.
	// Map key is a parseable integer of the pool index.
	ByPool map[string]Metrics `json:"by_pool,omitempty"`

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
	API        *APIMetrics        `json:"api,omitempty"`
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
	if r.API == nil && other.API != nil {
		r.API = &APIMetrics{}
	}
	r.API.Merge(other.API)
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

	// Aggregate per pool metrics
	if r.ByPool == nil && len(other.ByPool) > 0 {
		r.ByPool = make(map[string]Metrics, len(other.ByPool))
	}
	for id, m := range other.ByPool {
		if p, ok := r.ByPool[id]; ok {
			p.Merge(&m)
			r.ByPool[id] = p
		} else {
			r.ByPool[id] = m
		}
	}
}

// ScannerMetrics contains scanner information.
type ScannerMetrics struct {
	// Time these metrics were collected
	CollectedAt time.Time `json:"collected"`

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

	// Excessive prefixes.
	// Paths that have been marked as having excessive number of entries within the last 24 hours.
	ExcessivePrefixes []string `json:"excessive,omitempty"`
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

	if len(other.ExcessivePrefixes) > 0 {
		// Merge and remove duplicates
		merged := make(map[string]struct{}, len(s.ExcessivePrefixes)+len(other.ExcessivePrefixes))
		for _, prefix := range s.ExcessivePrefixes {
			merged[prefix] = struct{}{}
		}
		// Add other excessive prefixes
		for _, prefix := range other.ExcessivePrefixes {
			merged[prefix] = struct{}{}
		}
		s.ExcessivePrefixes = make([]string, 0, len(merged))
		for prefix := range merged {
			s.ExcessivePrefixes = append(s.ExcessivePrefixes, prefix)
		}
		sort.Strings(s.ExcessivePrefixes)
	}
}

// DiskIOStats contains IO stats of a single drive
type DiskIOStats struct {
	ReadIOs        uint64 `json:"read_ios,omitempty"`
	ReadMerges     uint64 `json:"read_merges,omitempty"`
	ReadSectors    uint64 `json:"read_sectors,omitempty"`
	ReadTicks      uint64 `json:"read_ticks,omitempty"`
	WriteIOs       uint64 `json:"write_ios,omitempty"`
	WriteMerges    uint64 `json:"write_merges,omitempty"`
	WriteSectors   uint64 `json:"wrte_sectors,omitempty"`
	WriteTicks     uint64 `json:"write_ticks,omitempty"`
	CurrentIOs     uint64 `json:"current_ios,omitempty"`
	TotalTicks     uint64 `json:"total_ticks,omitempty"`
	ReqTicks       uint64 `json:"req_ticks,omitempty"`
	DiscardIOs     uint64 `json:"discard_ios,omitempty"`
	DiscardMerges  uint64 `json:"discard_merges,omitempty"`
	DiscardSectors uint64 `json:"discard_secotrs,omitempty"`
	DiscardTicks   uint64 `json:"discard_ticks,omitempty"`
	FlushIOs       uint64 `json:"flush_ios,omitempty"`
	FlushTicks     uint64 `json:"flush_ticks,omitempty"`
}

// Merge 'other' into 'd'.
func (d *DiskIOStats) Merge(other DiskIOStats) {
	d.ReadIOs += other.ReadIOs
	d.ReadMerges += other.ReadMerges
	d.ReadSectors += other.ReadSectors
	d.ReadTicks += other.ReadTicks
	d.WriteIOs += other.WriteIOs
	d.WriteMerges += other.WriteMerges
	d.WriteSectors += other.WriteSectors
	d.WriteTicks += other.WriteTicks
	d.CurrentIOs += other.CurrentIOs
	d.TotalTicks += other.TotalTicks
	d.ReqTicks += other.ReqTicks
	d.DiscardIOs += other.DiscardIOs
	d.DiscardMerges += other.DiscardMerges
	d.DiscardSectors += other.DiscardSectors
	d.DiscardTicks += other.DiscardTicks
	d.FlushIOs += other.FlushIOs
	d.FlushTicks += other.FlushTicks
}

// DiskMetric contains metrics for one or more disks.
type DiskMetric struct {
	// Time these metrics were collected
	CollectedAt time.Time `json:"collected"`

	// Number of disks
	NDisks int `json:"n_disks"`

	// SetIdx will be populated if all disks in the metrics are part of the same set.
	SetIdx *int `json:"set_idx,omitempty"`

	// PoolIdx will be populated if all disks in the metrics are part of the same pool.
	PoolIdx *int `json:"pool_idx,omitempty"`

	// Offline disks
	Offline int `json:"offline,omitempty"`

	// Healing disks
	Healing int `json:"healing,omitempty"`

	// Number of accumulated operations by type since server restart.
	// FIXME: This is nil now due to eos #1088
	LifeTimeOps map[string]uint64 `json:"life_time_ops,omitempty"`

	// Last minute statistics.
	// FIXME: This is nil now due to eos #1088
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
	if d.NDisks == 0 {
		*d = *other
		return
	}
	if d.CollectedAt.Before(other.CollectedAt) {
		// Use latest timestamp
		d.CollectedAt = other.CollectedAt
	}
	// PoolIdx and SetIdx must match for all disks in the metrics.
	if d.SetIdx == nil && d.NDisks == 0 && other.SetIdx != nil {
		d.SetIdx = other.SetIdx
	} else if d.SetIdx != nil && other.SetIdx != nil && *d.SetIdx != *other.SetIdx {
		d.SetIdx = nil
	}
	if d.PoolIdx == nil && d.NDisks == 0 && other.PoolIdx != nil {
		d.PoolIdx = other.PoolIdx
	} else if d.PoolIdx != nil && other.PoolIdx != nil && *d.PoolIdx != *other.PoolIdx {
		d.PoolIdx = nil
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
	d.IOStats.Merge(other.IOStats)
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

	Complete bool   `json:"complete"`
	Failed   bool   `json:"failed"`
	Status   string `json:"status"`

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
	Objects             int64 `json:"objects"`
	ObjectsFailed       int64 `json:"objectsFailed"`
	DeleteMarkers       int64 `json:"deleteMarkers"`
	DeleteMarkersFailed int64 `json:"deleteMarkersFailed"`
	BytesTransferred    int64 `json:"bytesTransferred"`
	BytesFailed         int64 `json:"bytesFailed"`
}

type ExpirationInfo struct {
	// Last bucket/object key rotated
	Bucket string `json:"lastBucket"`
	Object string `json:"lastObject"`

	// Verbose information
	Objects             int64 `json:"objects"`
	ObjectsFailed       int64 `json:"objectsFailed"`
	DeleteMarkers       int64 `json:"deleteMarkers"`
	DeleteMarkersFailed int64 `json:"deleteMarkersFailed"`
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
	Bucket            string `json:"bucket"`
	LastBucketScanned string `json:"lastBucketScanned,omitempty"` // Deprecated 07/01/2025; Replaced by `bucket`
	LastObjectScanned string `json:"lastObjectScanned"`
	LastBucketMatched string `json:"lastBucketMatched,omitempty"` // Deprecated 07/01/2025; Replaced by `bucket`
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

	// Used to resume catalog jobs
	LastObjectWritten string            `json:"lastObjectWritten,omitempty"`
	OutputFiles       []CatalogDataFile `json:"outputFiles,omitempty"`
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
	// Use latest metrics
	if o.Jobs == nil {
		o.Jobs = make(map[string]JobMetric, len(other.Jobs))
	}
	for k, v := range other.Jobs {
		if exists, ok := o.Jobs[k]; !ok || exists.LastUpdate.Before(v.LastUpdate) {
			o.Jobs[k] = v
		}
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

// APIStats contains accumulated statistics for the API on a number of nodes.
type APIStats struct {
	Nodes         int        `json:"nodes,omitempty"`         // Number of nodes that have reported data.
	StartTime     *time.Time `json:"startTime,omitempty"`     // Time range this data covers unless merged from sources with different start times..
	EndTime       *time.Time `json:"endTime,omitempty"`       // Time range this data covers unless merged from sources with different end times.
	WallTimeSecs  float64    `json:"wallTimeSecs,omitempty"`  // Wall time this data covers, accumulated from all nodes.
	Requests      int64      `json:"requests,omitempty"`      // Total number of requests.
	IncomingBytes int64      `json:"incomingBytes,omitempty"` // Total number of bytes received.
	OutgoingBytes int64      `json:"outgoingBytes,omitempty"` // Total number of bytes sent.
	Errors4xx     int        `json:"errors_4xx,omitempty"`    // Total number of 4xx (client request) errors.
	Errors5xx     int        `json:"errors_5xx,omitempty"`    // Total number of 5xx (serverside) errors.
	Canceled      int64      `json:"canceled,omitempty"`      // Requests that were canceled before they finished processing.

	// Request times
	RequestTimeSecs float64 `json:"requestTimeSecs,omitempty"` // Total request time.
	ReqReadSecs     float64 `json:"reqReadSecs,omitempty"`     // Total time spent on request reads in seconds.
	RespSecs        float64 `json:"respSecs,omitempty"`        // Total time spent on responses in seconds.
	RespTTFBSecs    float64 `json:"respTtfbSecs,omitempty"`    // Total time spent on TTFB (req read -> response first byte) in seconds.

	// Request times min/max
	RequestTimeSecsMin float64 `json:"requestTimeSecsMin,omitempty"` // Min request time.
	RequestTimeSecsMax float64 `json:"requestTimeSecsMax,omitempty"` // Max request time.
	ReqReadSecsMin     float64 `json:"reqReadSecsMin,omitempty"`     // Min time spent on request reads in seconds.
	ReqReadSecsMax     float64 `json:"reqReadSecsMax,omitempty"`     // Max time spent on request reads in seconds.
	RespSecsMin        float64 `json:"respSecsMin,omitempty"`        // Min time spent on responses in seconds.
	RespSecsMax        float64 `json:"respSecsMax,omitempty"`        // Max time spent on responses in seconds.
	RespTTFBSecsMin    float64 `json:"respTtfbSecsMin,omitempty"`    // Min time spent on TTFB (req read -> response first byte) in seconds.
	RespTTFBSecsMax    float64 `json:"respTtfbSecsMax,omitempty"`    // Max time spent on TTFB (req read -> response first byte) in seconds.

	Rejected RejectedAPIStats `json:"rejected,omitempty"`
}

// RejectedAPIStats contains statistics for rejected requests.
type RejectedAPIStats struct {
	Auth           int64 `json:"auth,omitempty"`           // Total number of rejected authentication requests.
	RequestsTime   int64 `json:"requestsTime,omitempty"`   // Requests that were rejected due to outdated request signature.
	Header         int64 `json:"header,omitempty"`         // Requests that were rejected due to header signature.
	Invalid        int64 `json:"invalid,omitempty"`        // Requests that were rejected due to invalid request signature.
	NotImplemented int64 `json:"notImplemented,omitempty"` // Requests that were rejected due to not implemented API.
}

// Merge other into 'a'.
func (a *APIStats) Merge(other APIStats) {
	if a.StartTime == nil && a.Requests == 0 {
		a.StartTime = other.StartTime
	}
	if a.EndTime == nil && a.Requests == 0 {
		a.EndTime = other.EndTime
	}
	if a.StartTime != nil && other.StartTime != nil && !a.StartTime.Equal(*other.StartTime) {
		a.StartTime = nil
	}
	if a.EndTime != nil && other.EndTime != nil && !a.EndTime.Equal(*other.EndTime) {
		a.EndTime = nil
	}

	a.Nodes += other.Nodes
	a.Requests += other.Requests
	a.IncomingBytes += other.IncomingBytes
	a.OutgoingBytes += other.OutgoingBytes
	a.RequestTimeSecs += other.RequestTimeSecs
	a.ReqReadSecs += other.ReqReadSecs
	a.RespSecs += other.RespSecs
	a.RespTTFBSecs += other.RespTTFBSecs
	a.Errors4xx += other.Errors4xx
	a.Errors5xx += other.Errors5xx
	a.Canceled += other.Canceled
	a.Rejected.Auth += other.Rejected.Auth
	a.Rejected.RequestsTime += other.Rejected.RequestsTime
	a.Rejected.Header += other.Rejected.Header
	a.Rejected.Invalid += other.Rejected.Invalid
	a.Rejected.NotImplemented += other.Rejected.NotImplemented

	if a.Requests == 0 && other.Requests == 0 {
		return
	}

	// Find 2 to min/max. If we have 1, just use that twice
	at := *a
	bt := other
	if a.Requests == other.Requests {
		at = bt
	}
	if other.Requests == 0 {
		bt = at
	}
	a.RequestTimeSecsMin = min(at.RequestTimeSecsMin, bt.RequestTimeSecsMin)
	a.RequestTimeSecsMax = max(at.RequestTimeSecsMax, bt.RequestTimeSecsMax)
	a.ReqReadSecsMin = min(at.ReqReadSecsMin, bt.ReqReadSecsMin)
	a.ReqReadSecsMax = max(at.ReqReadSecsMax, bt.ReqReadSecsMax)
	a.RespSecsMin = min(at.RespSecsMin, bt.RespSecsMin)
	a.RespSecsMax = max(at.RespSecsMax, bt.RespSecsMax)
	a.RespTTFBSecsMin = min(at.RespTTFBSecsMin, bt.RespTTFBSecsMin)
	a.RespTTFBSecsMax = max(at.RespTTFBSecsMax, bt.RespTTFBSecsMax)
}

// SegmentedAPIMetrics contains metrics for API operations, segmented by time.
// FirstTime must be aligned to a start time that it a multiple of Interval.
type SegmentedAPIMetrics struct {
	Interval  int        `json:"intervalSecs"` // Interval covered by each segment in seconds.
	FirstTime time.Time  `json:"firstTime"`    // Timestamp of first (ie oldest) segment
	Segments  []APIStats `json:"segments"`     // List of APIStats for each segment ordered by time (oldest first).
}

// Merge other into 'a'.
func (a *SegmentedAPIMetrics) Merge(other SegmentedAPIMetrics) {
	if len(other.Segments) == 0 {
		return
	}
	if len(a.Segments) == 0 {
		*a = other
		return
	}

	// Intervals must match to merge safely.
	if other.Interval == 0 || a.Interval != other.Interval {
		// Cannot merge different resolutions without resampling.
		// Bail out silently as there's no error mechanism here.
		return
	}

	// Fast-path: same start time and same number of segments -> direct in-place merge.
	if a.FirstTime.Equal(other.FirstTime) && len(a.Segments) == len(other.Segments) {
		for i := range a.Segments {
			a.Segments[i].Merge(other.Segments[i])
		}
		return
	}
	// More complex merge...
	step := time.Duration(a.Interval) * time.Second

	// Determine the unified timeline.
	start := a.FirstTime
	if other.FirstTime.Before(start) {
		start = other.FirstTime
	}

	// Compute end times (exclusive).
	aEnd := a.FirstTime.Add(time.Duration(len(a.Segments)) * step)
	oEnd := other.FirstTime.Add(time.Duration(len(other.Segments)) * step)

	// Total number of slots to cover both series.
	totalSlots := int(oEnd.Sub(start) / step)
	if aEnd.After(oEnd) {
		totalSlots = int(aEnd.Sub(start) / step)
	}

	// Prepare the result slice with zero-value APIStats (acts as empty).
	newSegments := make([]APIStats, totalSlots)

	// Copy/merge 'a' into new slice at the proper offset.
	if a.FirstTime.After(start) {
		offset := int(a.FirstTime.Sub(start) / step)
		copy(newSegments[offset:offset+len(a.Segments)], a.Segments)
	} else {
		// a starts at 'start'
		copy(newSegments[:len(a.Segments)], a.Segments)
	}

	// Merge 'other' into new slice at the proper offset.
	otherOffset := int(other.FirstTime.Sub(start) / step)
	for i, s := range other.Segments {
		idx := otherOffset + i
		if idx < 0 || idx >= len(newSegments) {
			continue
		}
		newSegments[idx].Merge(s)
	}

	// Update receiver with merged result.
	a.FirstTime = start
	a.Segments = newSegments
}

func (a *SegmentedAPIMetrics) Total(nodes ...int) APIStats {
	var res APIStats
	if a == nil {
		return res
	}
	for _, stat := range a.Segments {
		res.Merge(stat)
	}
	// Since we are merging across APIs must reset track node count.
	if len(nodes) > 0 {
		res.Nodes = nodes[0]
	} else {
		res.Nodes = 0
	}
	return res
}

// APIMetrics contains metrics for API operations.
type APIMetrics struct {
	// Time these metrics were collected
	CollectedAt time.Time `json:"collected"`

	// Nodes responded to the request.
	Nodes int `json:"nodes"`

	// Number of active requests.
	ActiveRequests int64 `json:"activeRequests,omitempty"`

	// Number of queued requests.
	QueuedRequests int64 `json:"queuedRequests,omitempty"`

	// Last minute operation statistics by API.
	LastMinuteAPI map[string]APIStats `json:"lastMinuteApi,omitempty"`

	// Last day operation statistics by API, segmented.
	LastDayAPI map[string]SegmentedAPIMetrics `json:"lastDayApi,omitempty"`

	// SinceStart contains operation statistics since server(s) started.
	SinceStart APIStats `json:"since_start"`
}

func (a *APIMetrics) Merge(b *APIMetrics) {
	if b == nil {
		return
	}
	if a.CollectedAt.Before(b.CollectedAt) {
		a.CollectedAt = b.CollectedAt
	}
	a.Nodes += b.Nodes
	a.ActiveRequests += b.ActiveRequests
	a.QueuedRequests += b.QueuedRequests

	for k, v := range b.LastMinuteAPI {
		if a.LastMinuteAPI == nil {
			a.LastMinuteAPI = make(map[string]APIStats, len(b.LastMinuteAPI))
		}
		existing := a.LastMinuteAPI[k]
		existing.Merge(v)
		a.LastMinuteAPI[k] = existing
	}
	for k, v := range b.LastDayAPI {
		if a.LastDayAPI == nil {
			a.LastDayAPI = make(map[string]SegmentedAPIMetrics, len(b.LastDayAPI))
		}
		existing, ok := a.LastDayAPI[k]
		if !ok {
			a.LastDayAPI[k] = v
			continue
		}
		existing.Merge(v)
		a.LastDayAPI[k] = existing
	}
	a.SinceStart.Merge(b.SinceStart)
}

// LastMinuteTotal returns the total APIStats for the last minute.
func (a APIMetrics) LastMinuteTotal() APIStats {
	var res APIStats
	for _, stats := range a.LastMinuteAPI {
		res.Merge(stats)
	}
	// Since we are merging across APIs must reset track node count.
	res.Nodes = a.Nodes
	return res
}

// LastDayTotalSegmented returns the total SegmentedAPIMetrics for the last day.
// There will be no node-count for values.
func (a APIMetrics) LastDayTotalSegmented() SegmentedAPIMetrics {
	var res SegmentedAPIMetrics
	for _, stats := range a.LastDayAPI {
		res.Merge(stats)
	}
	// Since we are merging across APIs must reset track node count.
	for i := range res.Segments {
		res.Segments[i].Nodes = a.Nodes
	}
	return res
}

// LastDayTotal returns the accumulated APIStats for the last day.
func (a APIMetrics) LastDayTotal() APIStats {
	var res APIStats
	for _, stats := range a.LastDayAPI {
		for _, s := range stats.Segments {
			res.Merge(s)
		}
	}
	// Since we are merging across APIs must reset track node count.
	res.Nodes = a.Nodes

	return res
}
