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

//go:generate msgp -unexported -d clearomitted -d "tag json" -d "timezone utc" -d "maps binkeys" -file $GOFILE

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
	MetricsReplication
	MetricsProcess

	// MetricsAll must be last.
	// Enables all metrics.
	MetricsAll = 1<<(iota) - 1
)

// Contains returns whether m contains all of x.
func (m MetricType) Contains(x MetricType) bool {
	return m&x == x
}

// MetricFlags is a bitfield representation of different metric flags.
type MetricFlags uint64

const (
	MetricsDayStats     MetricFlags = 1 << (iota) // Include daily statistics
	MetricsByHost                                 // Aggregate metrics by host/node.
	MetricsByDisk                                 // Aggregate metrics by disk.
	MetricsLegacyDiskIO                           // Add legacy disk IO metrics.
	MetricsByDiskSet                              // Aggregate metrics by disk pool+set index.
)

// Contains returns whether m contains all of x.
func (m MetricFlags) Contains(x MetricFlags) bool {
	return m&x == x
}

// Add one or more flags to m.
func (m *MetricFlags) Add(x ...MetricFlags) {
	for _, v := range x {
		*m = *m | v
	}
}

// MetricsOptions are options provided to Metrics call.
type MetricsOptions struct {
	Type         MetricType    // Return only these metric types. Several types can be combined using |. Leave at 0 to return all.
	Flags        MetricFlags   // Flags to control returned metrics.
	N            int           // Maximum number of samples to return. 0 will return endless stream.
	Interval     time.Duration // Interval between samples. Will be rounded up to 1s.
	PoolIdx      []int         // Only include metrics for these pools. Leave empty for all.
	Hosts        []string      // Only include specified hosts. Leave empty for all.
	DrivePoolIdx []int         // Only include metrics for these drive pools. Leave empty for all.
	DriveSetIdx  []int         // Only include metrics for these drive sets (combine with PoolIdx if needed).
	Disks        []string      // Include only specific disks. Leave empty for all.
	ByJobID      string
	ByDepID      string

	// Alternative output merging.
	// Populates maps of the same name in the result.
	ByHost bool // Return individual metrics by host. Deprecated: use MetricsByHost instead.
	ByDisk bool // Return individual metrics by disk. Deprecated: use MetricsByDisk instead.
}

// DriveSetPrefix will be used to select drives from specific sets.
const (
	DriveSetPrefix  = "::drive-set::"
	DrivePoolPrefix = "::drive-pool::"
)

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
		q.Set("by-host", "true") // Legacy flag
		o.Flags.Add(MetricsByDisk)
	}
	for _, v := range o.DriveSetIdx {
		o.Disks = append(o.Disks, fmt.Sprintf(DriveSetPrefix+"%d", v))
	}
	for _, v := range o.DrivePoolIdx {
		o.Disks = append(o.Disks, fmt.Sprintf(DrivePoolPrefix+"%d", v))
	}

	q.Set("disks", strings.Join(o.Disks, ","))
	if o.ByDisk {
		q.Set("by-disk", "true") // Legacy flag
		o.Flags.Add(MetricsByDisk)
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
	q.Set("flags", strconv.FormatUint(uint64(o.Flags), 10))

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

	// ByDiskSet contains disk metrics aggregated by pool+set index.
	ByDiskSet map[int]map[int]DiskMetric `json:"by_disk_set,omitempty"`

	// Final indicates whether this is the final packet and the receiver can exit.
	Final bool `json:"final"`
}

// Merge functionality:
//
// Overall rules: a.Merge(b)
//
// 1. All metrics must be accumulated and must be independent of order of merges.
// 2. If a field is not set in the other, it is not modified.
// 3. If a field is set in both, the value is merged.
// 4. Only a may be mutated.
// 5. 'a' can be the zero value.

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
	if r.ByDiskSet == nil && len(other.ByDiskSet) > 0 {
		r.ByDiskSet = make(map[int]map[int]DiskMetric, len(other.ByDisk))
	}
	for pIdx, pool := range other.ByDiskSet {
		dstp := r.ByDiskSet[pIdx]
		if dstp == nil {
			dstp = make(map[int]DiskMetric, len(pool))
			r.ByDiskSet[pIdx] = dstp
		}
		for sIdx, disks := range pool {
			dsts := dstp[sIdx]
			dsts.Merge(&disks)
			dstp[sIdx] = dsts
		}
	}
}

// Metrics contains all metric types.
type Metrics struct {
	Scanner     *ScannerMetrics     `json:"scanner,omitempty"`
	Disk        *DiskMetric         `json:"disk,omitempty"`
	OS          *OSMetrics          `json:"os,omitempty"`
	BatchJobs   *BatchJobMetrics    `json:"batchJobs,omitempty"`
	SiteResync  *SiteResyncMetrics  `json:"siteResync,omitempty"`
	Net         *NetMetrics         `json:"net,omitempty"`
	Mem         *MemMetrics         `json:"mem,omitempty"`
	CPU         *CPUMetrics         `json:"cpu,omitempty"`
	RPC         *RPCMetrics         `json:"rpc,omitempty"`
	Go          *RuntimeMetrics     `json:"go,omitempty"`
	API         *APIMetrics         `json:"api,omitempty"`
	Replication *ReplicationMetrics `json:"replication,omitempty"`
	Process     *ProcessMetrics     `json:"process,omitempty"`
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
	if r.Replication == nil && other.Replication != nil {
		r.Replication = &ReplicationMetrics{}
	}
	r.Replication.Merge(other.Replication)
	if r.Mem == nil && other.Mem != nil {
		r.Mem = &MemMetrics{}
	}
	r.Mem.Merge(other.Mem)
	if r.CPU == nil && other.CPU != nil {
		r.CPU = &CPUMetrics{}
	}
	r.CPU.Merge(other.CPU)
	if r.Process == nil && other.Process != nil {
		r.Process = &ProcessMetrics{}
	}
	r.Process.Merge(other.Process)
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
	N              int    `json:"n,omitempty"`
	ReadIOs        uint64 `json:"read_ios,omitempty"`
	ReadMerges     uint64 `json:"read_merges,omitempty"`
	ReadSectors    uint64 `json:"read_sectors,omitempty"`
	ReadTicks      uint64 `json:"read_ticks,omitempty"`
	WriteIOs       uint64 `json:"write_ios,omitempty"`
	WriteMerges    uint64 `json:"write_merges,omitempty"`
	WriteSectors   uint64 `json:"write_sectors,omitempty"`
	WriteTicks     uint64 `json:"write_ticks,omitempty"`
	CurrentIOs     uint64 `json:"current_ios,omitempty"`
	TotalTicks     uint64 `json:"total_ticks,omitempty"`
	ReqTicks       uint64 `json:"req_ticks,omitempty"`
	DiscardIOs     uint64 `json:"discard_ios,omitempty"`
	DiscardMerges  uint64 `json:"discard_merges,omitempty"`
	DiscardSectors uint64 `json:"discard_sectors,omitempty"`
	DiscardTicks   uint64 `json:"discard_ticks,omitempty"`
	FlushIOs       uint64 `json:"flush_ios,omitempty"`
	FlushTicks     uint64 `json:"flush_ticks,omitempty"`
}

type DiskIOStatsLegacy struct {
	N              int    `json:"n,omitempty"`
	ReadIOs        uint64 `json:"read_ios,omitempty"`
	ReadMerges     uint64 `json:"read_merges,omitempty"`
	ReadSectors    uint64 `json:"read_sectors,omitempty"`
	ReadTicks      uint64 `json:"read_ticks,omitempty"`
	WriteIOs       uint64 `json:"write_ios,omitempty"`
	WriteMerges    uint64 `json:"write_merges,omitempty"`
	WriteSectors   uint64 `json:"wrte_sectors,omitempty"` // note "spelling"
	WriteTicks     uint64 `json:"write_ticks,omitempty"`
	CurrentIOs     uint64 `json:"current_ios,omitempty"`
	TotalTicks     uint64 `json:"total_ticks,omitempty"`
	ReqTicks       uint64 `json:"req_ticks,omitempty"`
	DiscardIOs     uint64 `json:"discard_ios,omitempty"`
	DiscardMerges  uint64 `json:"discard_merges,omitempty"`
	DiscardSectors uint64 `json:"discard_secotrs,omitempty"` // note "spelling"
	DiscardTicks   uint64 `json:"discard_ticks,omitempty"`
	FlushIOs       uint64 `json:"flush_ios,omitempty"`
	FlushTicks     uint64 `json:"flush_ticks,omitempty"`
}

// Add 'other' to 'd'.
func (d *DiskIOStats) Add(other *DiskIOStats) {
	if other == nil {
		return
	}
	d.N += other.N
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

type (
	SegmentedDiskActions = Segmented[DiskAction, *DiskAction]
	SegmentedDiskIO      = Segmented[DiskIOStats, *DiskIOStats]
)

// DiskMetric contains metrics for one or more disks.
type DiskMetric struct {
	// Time these metrics were collected
	CollectedAt time.Time `json:"collected"`

	// Number of disks
	NDisks int `json:"n_disks"`

	// DiskIdx will be populated if all disks in the metrics have the same drive index.
	DiskIdx *int `json:"disk_idx,omitempty"`

	// SetIdx will be populated if all disks in the metrics are part of the same set.
	SetIdx *int `json:"set_idx,omitempty"`

	// PoolIdx will be populated if all disks in the metrics are part of the same pool.
	PoolIdx *int `json:"pool_idx,omitempty"`

	// Disk states for non-ok disks.
	// See madmin.DriveState for possible values.
	State map[string]int `json:"state,omitempty"`

	// Offline disks
	Offline int `json:"offline,omitempty"`

	// Hanging - drives hanging.
	Hanging int `json:"waiting,omitempty"`

	// Healing disks
	// Deprecated, will be removed in later releases
	Healing int `json:"healing,omitempty"`

	// HealingInfo gives us a high level overview of the drives healing state
	HealingInfo *DriveHealInfo `json:"healingInfo,omitempty"`

	// Cache stats if enabled.
	Cache *CacheStats `json:"cache,omitempty"`

	// Space info.
	Space DriveSpaceInfo `json:"space"`

	// Number of accumulated operations by type.
	LifetimeOps map[string]DiskAction `json:"lifetime_ops,omitempty"`

	// Last minute statistics.
	LastMinute map[string]DiskAction `json:"last_minute,omitempty"`

	// LastDaySegmented contains the segmented metrics for the last day.
	LastDaySegmented map[string]SegmentedDiskActions `json:"last_day,omitempty"`

	// IO stats.
	// Deprecated: use io_min, io_day instead.
	IOStats *DiskIOStatsLegacy `json:"iostats,omitempty"`

	// Rolling window last minute IO stats.
	IOStatsMinute DiskIOStats `json:"io_min"`

	// Rolling window daily IO stats.
	IOStatsDay SegmentedDiskIO `json:"io_day"`
}

type DriveHealInfo struct {
	ItemsHealed uint64    `json:"itemsHealed"`
	ItemsFailed uint64    `json:"itemsFailed"`
	HealID      string    `json:"healID"`
	Finished    bool      `json:"finished"`
	Started     time.Time `json:"started"`
	Updated     time.Time `json:"updated"`
}

// DriveSpaceInfo is the space info of one or more drives.
type DriveSpaceInfo struct {
	N          int               `json:"n"`
	Free       TotalMinMaxUint64 `json:"free"`
	Used       TotalMinMaxUint64 `json:"used"`
	UsedInodes TotalMinMaxUint64 `json:"used_inodes"`
	FreeInodes TotalMinMaxUint64 `json:"free_inodes"`
}

func (d *DriveSpaceInfo) Merge(other DriveSpaceInfo) {
	d.N += other.N
	d.Free.Merge(other.Free, d.N)
	d.Used.Merge(other.Used, d.N)
	d.UsedInodes.Merge(other.UsedInodes, d.N)
	d.FreeInodes.Merge(other.FreeInodes, d.N)
}

//msgp:tuple TotalMinMaxUint64
type TotalMinMaxUint64 struct {
	Total uint64 `json:"total"`
	Min   uint64 `json:"min"`
	Max   uint64 `json:"max"`
}

func (t *TotalMinMaxUint64) SetAll(v uint64) {
	t.Total = v
	t.Min = v
	t.Max = v
}

// Merge 'other' into 't', assuming both are set.
func (t *TotalMinMaxUint64) Merge(other TotalMinMaxUint64, tCnt int) {
	t.Total += other.Total
	if tCnt == 0 || t.Min > other.Min {
		t.Min = other.Min
	}
	t.Max = max(t.Max, other.Max)
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
	if d.PoolIdx == nil && d.NDisks == 0 && other.PoolIdx != nil {
		d.PoolIdx = other.PoolIdx
	} else if other.PoolIdx == nil || d.PoolIdx != nil && other.PoolIdx != nil && *d.PoolIdx != *other.PoolIdx {
		d.PoolIdx = nil
	}
	if d.SetIdx == nil && d.NDisks == 0 && other.SetIdx != nil {
		d.SetIdx = other.SetIdx
	} else if other.SetIdx == nil || d.SetIdx != nil && other.SetIdx != nil && *d.SetIdx != *other.SetIdx || d.PoolIdx == nil {
		d.SetIdx = nil
	}
	if d.DiskIdx == nil && d.NDisks == 0 && other.DiskIdx != nil {
		d.DiskIdx = other.DiskIdx
	} else if other.DiskIdx == nil || d.DiskIdx != nil && other.DiskIdx != nil && *d.DiskIdx != *other.DiskIdx || d.SetIdx == nil {
		d.DiskIdx = nil
	}
	if len(other.State) > 0 {
		if d.State == nil {
			d.State = make(map[string]int, len(other.State))
		}
		for k, v := range other.State {
			d.State[k] = d.State[k] + v
		}
	}
	d.NDisks += other.NDisks
	d.Offline += other.Offline
	d.Healing += other.Healing
	d.Hanging += other.Hanging
	if other.Cache != nil {
		if d.Cache == nil {
			d.Cache = other.Cache
		}
		d.Cache.Merge(other.Cache)
	}
	d.Space.Merge(other.Space)

	if len(other.LifetimeOps) > 0 && d.LifetimeOps == nil {
		d.LifetimeOps = make(map[string]DiskAction, len(other.LifetimeOps))
	}
	for k, v := range other.LifetimeOps {
		t := d.LifetimeOps[k]
		t.Add(&v)
		d.LifetimeOps[k] = t
	}

	if d.LastMinute == nil && len(other.LastMinute) > 0 {
		d.LastMinute = make(map[string]DiskAction, len(other.LastMinute))
	}
	for k, v := range other.LastMinute {
		t := d.LastMinute[k]
		t.Add(&v)
		d.LastMinute[k] = t
	}

	if len(other.LastDaySegmented) > 0 && d.LastDaySegmented == nil {
		d.LastDaySegmented = make(map[string]SegmentedDiskActions, len(other.LastDaySegmented))
	}
	for k, v := range other.LastDaySegmented {
		t := d.LastDaySegmented[k]
		t.Add(&v)
		d.LastDaySegmented[k] = t
	}
	if other.IOStats != nil {
		if d.IOStats == nil {
			d.IOStats = new(DiskIOStatsLegacy)
		}
		a, b := DiskIOStats(*d.IOStats), DiskIOStats(*other.IOStats)
		a.Add(&b)
		c := DiskIOStatsLegacy(a)
		d.IOStats = &c
	}
	d.IOStatsMinute.Add(&other.IOStatsMinute)
	d.IOStatsDay.Add(&other.IOStatsDay)
}

// LifetimeTotal returns the accumulated Disk metrics for all operations
func (d DiskMetric) LifetimeTotal() DiskAction {
	var res DiskAction
	for _, s := range d.LifetimeOps {
		res.Add(&s)
	}
	return res
}

// SensorMetrics aggregated sensor metrics for a single sensor key
type SensorMetrics struct {
	MinTemp         float64 `json:"min_temp"`                   // Minimum temperature seen
	MaxTemp         float64 `json:"max_temp"`                   // Maximum temperature seen
	TotalTemp       float64 `json:"total_temp"`                 // Total temperature for averaging
	Count           int     `json:"count"`                      // Number of readings
	ExceedsCritical int     `json:"exceeds_critical,omitempty"` // Count of readings exceeding critical threshold
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

	// Aggregated temperature sensor metrics by sensor key
	Sensors map[string]SensorMetrics `json:"sensors,omitempty"`
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

	// Merge sensor metrics
	if len(other.Sensors) > 0 {
		if o.Sensors == nil {
			o.Sensors = make(map[string]SensorMetrics)
		}
		for key, otherSensor := range other.Sensors {
			existing := o.Sensors[key]
			// Handle min/max
			if existing.Count == 0 {
				// First data for this sensor
				existing.MinTemp = otherSensor.MinTemp
				existing.MaxTemp = otherSensor.MaxTemp
			} else {
				if otherSensor.MinTemp < existing.MinTemp {
					existing.MinTemp = otherSensor.MinTemp
				}
				if otherSensor.MaxTemp > existing.MaxTemp {
					existing.MaxTemp = otherSensor.MaxTemp
				}
			}
			// Accumulate totals
			existing.TotalTemp += otherSensor.TotalTemp
			existing.Count += otherSensor.Count
			existing.ExceedsCritical += otherSensor.ExceedsCritical
			o.Sensors[key] = existing
		}
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

	// NICs contains interface -> stats map.
	Interfaces map[string]InterfaceStats

	// Deprecated: Does not merge.
	InterfaceName string `json:"interfaceName"`

	// Internode Stats.
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
	for k, v := range other.Interfaces {
		if n.Interfaces == nil {
			n.Interfaces = make(map[string]InterfaceStats, len(other.Interfaces))
		}
		n.Interfaces[k] = n.Interfaces[k].add(v)
	}
	n.NetStats = procfs.NetDevLine(procfsNetDevLine(n.NetStats).add(procfsNetDevLine(other.NetStats)))
}

// InterfaceStats contains accumulated stats for a network interface.
type InterfaceStats struct {
	N                 int `json:"n"`
	procfs.NetDevLine `json:"stats"`
}

func (n InterfaceStats) add(other InterfaceStats) InterfaceStats {
	return InterfaceStats{
		N:          n.N,
		NetDevLine: procfs.NetDevLine(procfsNetDevLine(n.NetDevLine).add(procfsNetDevLine(other.NetDevLine))),
	}
}

//msgp:replace NodeCommon with:nodeCommon

// nodeCommon - use as replacement for NodeCommon
// We do not want to give NodeCommon codegen, since it is used for embedding.
type nodeCommon struct {
	Addr  string `json:"addr"`
	Error string `json:"error,omitempty"`
}

type MemMetrics struct {
	// Time these metrics were collected
	CollectedAt time.Time `json:"collected"`

	Nodes int `json:"nodes"` // Note: Will be zero for older servers.

	Info MemInfo `json:"memInfo"`
}

// Merge other into 'm'.
func (m *MemMetrics) Merge(other *MemMetrics) {
	if other == nil {
		return
	}
	m.Nodes += other.Nodes
	if m.CollectedAt.Before(other.CollectedAt) {
		// Use latest timestamp
		m.CollectedAt = other.CollectedAt
	}
	m.Info.Merge(&other.Info)
}

// MemInfo contains system's RAM and swap information.
type MemInfo struct {
	// NodeCommon shouldn't be used since it cannot be merged.
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

func (m *MemInfo) Merge(other *MemInfo) {
	if other == nil {
		return
	}
	if m.Total == 0 && m.Addr == "" {
		m.NodeCommon = other.NodeCommon
	} else if m.NodeCommon != other.NodeCommon {
		m.NodeCommon = NodeCommon{}
	}
	m.Total += other.Total
	m.Used += other.Used
	m.Free += other.Free
	m.Available += other.Available
	m.Shared += other.Shared
	m.Cache += other.Cache
	m.Buffers += other.Buffers
	m.SwapSpaceTotal += other.SwapSpaceTotal
	m.SwapSpaceFree += other.SwapSpaceFree
	m.Limit += other.Limit
}

//msgp:replace cpu.TimesStat with:cpuTimesStat
//msgp:replace load.AvgStat with:loadAvgStat

type CPUMetrics struct {
	// Time these metrics were collected
	CollectedAt time.Time `json:"collected"`

	Nodes int `json:"nodes"` // Note: May be unset for older servers.

	TimesStat *cpu.TimesStat `json:"timesStat"`
	LoadStat  *load.AvgStat  `json:"loadStat"`
	CPUCount  int            `json:"cpuCount"`

	// Aggregated CPU information
	CPUByModel     map[string]int `json:"cpu_by_model,omitempty"`     // ModelName -> count of CPUs
	TotalMhz       float64        `json:"total_mhz,omitempty"`        // Accumulated MHz
	TotalCores     int            `json:"total_cores,omitempty"`      // Accumulated cores
	TotalCacheSize int64          `json:"total_cache_size,omitempty"` // Accumulated cache size in bytes

	// Aggregated CPU frequency information
	FreqStatsCount          int            `json:"freq_stats_count,omitempty"`           // Number of freq stats (for averaging)
	GovernorFreq            map[string]int `json:"governor_freq,omitempty"`              // Governor -> count
	TotalCurrentFreq        uint64         `json:"total_current_freq,omitempty"`         // Accumulated current freq
	TotalScalingCurrentFreq uint64         `json:"total_scaling_current_freq,omitempty"` // Accumulated scaling current freq
	MinCPUInfoFreq          uint64         `json:"min_freq,omitempty"`                   // Minimum of CpuinfoMinimumFrequency
	MaxCPUInfoFreq          uint64         `json:"max_freq,omitempty"`                   // Maximum of CpuinfoMaximumFrequency
	MinScalingFreq          uint64         `json:"min_scaling_freq,omitempty"`           // Minimum of ScalingMinimumFrequency
	MaxScalingFreq          uint64         `json:"max_scaling_freq,omitempty"`           // Maximum of ScalingMaximumFrequency
}

// Merge other into 'm'.
func (m *CPUMetrics) Merge(other *CPUMetrics) {
	if other == nil {
		return
	}
	m.Nodes += other.Nodes
	if m.CollectedAt.Before(other.CollectedAt) {
		// Use latest timestamp
		m.CollectedAt = other.CollectedAt
	}
	if m.TimesStat != nil && other.TimesStat != nil {
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
	} else if m.TimesStat == nil && other.TimesStat != nil {
		m.TimesStat = other.TimesStat
	}

	if m.LoadStat != nil && other.LoadStat != nil {
		m.LoadStat.Load1 += other.LoadStat.Load1
		m.LoadStat.Load5 += other.LoadStat.Load5
		m.LoadStat.Load15 += other.LoadStat.Load15
	} else if m.LoadStat == nil && other.LoadStat != nil {
		m.LoadStat = other.LoadStat
	}
	m.CPUCount += other.CPUCount

	// Merge aggregated CPU information
	if len(other.CPUByModel) > 0 {
		if m.CPUByModel == nil {
			m.CPUByModel = make(map[string]int)
		}
		for model, count := range other.CPUByModel {
			m.CPUByModel[model] += count
		}
	}
	m.TotalMhz += other.TotalMhz
	m.TotalCores += other.TotalCores
	m.TotalCacheSize += other.TotalCacheSize

	// Merge aggregated CPU frequency information
	if len(other.GovernorFreq) > 0 {
		if m.GovernorFreq == nil {
			m.GovernorFreq = make(map[string]int)
		}
		for governor, count := range other.GovernorFreq {
			m.GovernorFreq[governor] += count
		}
	}
	m.TotalCurrentFreq += other.TotalCurrentFreq
	m.TotalScalingCurrentFreq += other.TotalScalingCurrentFreq

	// Handle min/max frequencies properly
	// Use FreqStatsCount to determine if this is the first merge
	if other.MinCPUInfoFreq > 0 {
		if m.FreqStatsCount == 0 || other.MinCPUInfoFreq < m.MinCPUInfoFreq {
			m.MinCPUInfoFreq = other.MinCPUInfoFreq
		}
	}
	if other.MaxCPUInfoFreq > m.MaxCPUInfoFreq {
		m.MaxCPUInfoFreq = other.MaxCPUInfoFreq
	}
	if other.MinScalingFreq > 0 {
		if m.FreqStatsCount == 0 || other.MinScalingFreq < m.MinScalingFreq {
			m.MinScalingFreq = other.MinScalingFreq
		}
	}
	if other.MaxScalingFreq > m.MaxScalingFreq {
		m.MaxScalingFreq = other.MaxScalingFreq
	}

	m.FreqStatsCount += other.FreqStatsCount
}

// RPCMetrics contains metrics for RPC operations.
// Metrics are collected on the sender side of RPC calls.
type RPCMetrics struct {
	Nodes int `json:"nodes,omitempty"`

	CollectedAt time.Time `json:"collected"`

	// Connection stats accumulated for grid systems running on nodes.
	//nolint:staticcheck // SA5008
	ConnectionStats `json:",flatten"`

	// Last minute operation statistics by handler.
	LastMinute map[string]RPCStats `json:"lastMinute,omitempty"`

	// Last day operation statistics by handler, segmented.
	LastDay map[string]SegmentedRPCMetrics `json:"lastDay,omitempty"`

	ByDestination map[string]ConnectionStats `json:"byDestination,omitempty"`
	ByCaller      map[string]ConnectionStats `json:"byCaller,omitempty"`
}

// Merge other into 'm'.
func (m *RPCMetrics) Merge(other *RPCMetrics) {
	if m == nil || other == nil {
		return
	}
	m.Nodes += other.Nodes
	if m.CollectedAt.Before(other.CollectedAt) {
		// Use latest timestamp
		m.CollectedAt = other.CollectedAt
	}

	m.ConnectionStats.Merge(&other.ConnectionStats)

	for k, v := range other.ByDestination {
		if m.ByDestination == nil {
			m.ByDestination = make(map[string]ConnectionStats, len(other.ByDestination))
		}
		existing := m.ByDestination[k]
		existing.Merge(&v)
		m.ByDestination[k] = existing
	}

	for k, v := range other.ByCaller {
		if m.ByCaller == nil {
			m.ByCaller = make(map[string]ConnectionStats, len(other.ByCaller))
		}
		existing := m.ByCaller[k]
		existing.Merge(&v)
		m.ByCaller[k] = existing
	}

	for k, v := range other.LastMinute {
		if m.LastMinute == nil {
			m.LastMinute = make(map[string]RPCStats, len(other.LastMinute))
		}
		existing := m.LastMinute[k]
		existing.Merge(v)
		m.LastMinute[k] = existing
	}
	for k, v := range other.LastDay {
		if m.LastDay == nil {
			m.LastDay = make(map[string]SegmentedRPCMetrics, len(other.LastDay))
		}
		existing, ok := m.LastDay[k]
		if !ok {
			// Deep copy to avoid sharing slice references
			vCopy := v
			if len(v.Segments) > 0 {
				vCopy.Segments = append([]RPCStats{}, v.Segments...)
			}
			m.LastDay[k] = vCopy
			continue
		}
		existing.Add(&v)
		m.LastDay[k] = existing
	}
}

// LastMinuteTotal returns the total RPCStats for the last minute.
func (m *RPCMetrics) LastMinuteTotal() RPCStats {
	var res RPCStats
	for _, stats := range m.LastMinute {
		res.Merge(stats)
	}
	// Since we are merging across APIs must reset track node count.
	return res
}

// LastDayTotalSegmented returns the total SegmentedRPCMetrics for the last day.
func (m *RPCMetrics) LastDayTotalSegmented() SegmentedRPCMetrics {
	var res SegmentedRPCMetrics
	for _, stats := range m.LastDay {
		res.Add(&stats)
	}
	return res
}

// LastDayTotal returns the accumulated RPCStats for the last day.
func (m *RPCMetrics) LastDayTotal() RPCStats {
	var res RPCStats
	for _, stats := range m.LastDay {
		for _, s := range stats.Segments {
			res.Merge(s)
		}
	}
	return res
}

// ConnectionStats are the overall connection stats.
type ConnectionStats struct {
	Connected        int       `json:"connected,omitempty"`
	Disconnected     int       `json:"disconnected,omitempty"`
	ReconnectCount   int       `json:"reconnectCount,omitempty"` // Total reconnects.
	OutgoingStreams  int       `json:"outgoingStreams,omitempty"`
	IncomingStreams  int       `json:"incomingStreams,omitempty"`
	OutgoingMessages int64     `json:"outgoingMessages,omitempty"`
	IncomingMessages int64     `json:"incomingMessages,omitempty"`
	OutgoingBytes    int64     `json:"outgoingBytes,omitempty"` // Total number of bytes sent.
	IncomingBytes    int64     `json:"incomingBytes,omitempty"` // Total number of bytes received.
	OutQueue         int       `json:"outQueue,omitempty"`
	LastPongTime     time.Time `json:"lastPongTime,omitempty"`
	LastConnectTime  time.Time `json:"lastConnectTime,omitempty"`
	LastPingMS       float64   `json:"lastPingMS,omitempty"`
	MaxPingDurMS     float64   `json:"maxPingDurMS,omitempty"` // Maximum across all merged entries.
}

// Merge other into c.
func (c *ConnectionStats) Merge(other *ConnectionStats) {
	if other == nil {
		return
	}
	c.Connected += other.Connected
	c.Disconnected += other.Disconnected
	c.ReconnectCount += other.ReconnectCount
	c.OutgoingStreams += other.OutgoingStreams
	c.IncomingStreams += other.IncomingStreams
	c.OutgoingMessages += other.OutgoingMessages
	c.IncomingMessages += other.IncomingMessages
	c.OutgoingBytes += other.OutgoingBytes
	c.IncomingBytes += other.IncomingBytes
	c.OutQueue += other.OutQueue
	if c.LastPongTime.Before(other.LastPongTime) {
		c.LastPongTime = other.LastPongTime
		c.LastPingMS = other.LastPingMS
	}
	if c.LastConnectTime.Before(other.LastConnectTime) {
		c.LastConnectTime = other.LastConnectTime
	}
	if c.MaxPingDurMS < other.MaxPingDurMS {
		c.MaxPingDurMS = other.MaxPingDurMS
	}
}

// SegmentedRPCMetrics are segmented RPC metrics.
type SegmentedRPCMetrics = Segmented[RPCStats, *RPCStats]

// RPCStats contains RPC statistics for RPC requests through grid.
type RPCStats struct {
	StartTime       *time.Time `json:"startTime,omitempty"`       // Time range this data covers unless merged from sources with different start times..
	EndTime         *time.Time `json:"endTime,omitempty"`         // Time range this data covers unless merged from sources with different end times.
	WallTimeSecs    float64    `json:"wallTimeSecs,omitempty"`    // Wall time this data covers, accumulated from all nodes.
	Requests        int64      `json:"requests,omitempty"`        // Total number of requests.
	RequestTimeSecs float64    `json:"requestTimeSecs,omitempty"` // Total request time.
	IncomingBytes   int64      `json:"incomingBytes,omitempty"`   // Total number of bytes received.
	OutgoingBytes   int64      `json:"outgoingBytes,omitempty"`   // Total number of bytes sent.
}

// Add 'other' to a.
func (a *RPCStats) Add(other *RPCStats) {
	if other == nil {
		return
	}
	a.Merge(*other)
}

// Merge other into 'a'.
func (a *RPCStats) Merge(other RPCStats) {
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
	a.WallTimeSecs += other.WallTimeSecs
	a.Requests += other.Requests
	a.IncomingBytes += other.IncomingBytes
	a.OutgoingBytes += other.OutgoingBytes
	a.RequestTimeSecs += other.RequestTimeSecs
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
	RequestTimeSecs  float64 `json:"requestTimeSecs,omitempty"` // Total request time.
	ReqReadSecs      float64 `json:"reqReadSecs,omitempty"`     // Total time spent on request reads in seconds.
	RespSecs         float64 `json:"respSecs,omitempty"`        // Total time spent on responses in seconds.
	RespTTFBSecs     float64 `json:"respTtfbSecs,omitempty"`    // Total time spent on TTFB (req read -> response first byte) in seconds.
	ReadBlockedSecs  float64 `json:"readBlocked,omitempty"`     // Time spent waiting for reads from client.
	WriteBlockedSecs float64 `json:"writeBlocked,omitempty"`    // Time spent waiting for writes to client.

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

// Add 'other' to a.
func (a *APIStats) Add(other *APIStats) {
	if other == nil {
		return
	}
	a.Merge(*other)
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
	a.WallTimeSecs += other.WallTimeSecs
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
	a.ReadBlockedSecs += other.ReadBlockedSecs
	a.WriteBlockedSecs += other.WriteBlockedSecs

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

// SegmentedAPIMetrics are segmented API metrics.
type SegmentedAPIMetrics = Segmented[APIStats, *APIStats]

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
			// Deep copy to avoid sharing slice references
			vCopy := v
			if len(v.Segments) > 0 {
				vCopy.Segments = append([]APIStats{}, v.Segments...)
			}
			a.LastDayAPI[k] = vCopy
			continue
		}
		existing.Add(&v)
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
		res.Add(&stats)
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

// Segmenter implement interface on pointers.
type Segmenter[T any] interface {
	msgp.Encodable
	msgp.Marshaler
	msgp.Decodable
	msgp.Unmarshaler
	msgp.Sizer
	Add(*T)
}

//msgp:ignore Segmented

// Segmented contains f type A metrics segmented by time.
// FirstTime must be aligned to a start time that it a multiple of Interval.
type Segmented[T any, PT interface {
	*T
	Segmenter[T]
}] struct {
	Interval  int       `json:"intervalSecs,omitempty"` // Interval covered by each segment in seconds.
	FirstTime time.Time `json:"firstTime,omitzero"`     // Timestamp of first (ie oldest) segment
	Segments  []T       `json:"segments,omitempty"`     // List of DiskAction for each segment ordered by time (oldest first).
}

// Add 'other' to 'a'.
func (s *Segmented[T, PT]) Add(other *Segmented[T, PT]) {
	if other == nil {
		return
	}
	if len(other.Segments) == 0 {
		return
	}
	if len(s.Segments) == 0 {
		// Copy slice to avoid overriding the original segment
		*s = *other
		s.Segments = append([]T{}, other.Segments...)
		return
	}

	// Intervals must match to merge safely.
	if other.Interval == 0 || s.Interval != other.Interval {
		// Cannot merge different resolutions without resampling.
		// Bail out silently as there's no error mechanism here.
		return
	}

	// Fast-path: same start time and same number of segments -> direct in-place merge.
	if s.FirstTime.Equal(other.FirstTime) && len(s.Segments) == len(other.Segments) {
		for i := range s.Segments {
			t := PT(&s.Segments[i])
			t.Add(&other.Segments[i])
		}
		return
	}
	// More complex merge...
	step := time.Duration(s.Interval) * time.Second

	// Determine the unified timeline.
	start := s.FirstTime
	if other.FirstTime.Before(start) {
		start = other.FirstTime
	}

	// Compute end times (exclusive).
	aEnd := s.FirstTime.Add(time.Duration(len(s.Segments)) * step)
	oEnd := other.FirstTime.Add(time.Duration(len(other.Segments)) * step)

	// Total number of slots to cover both series.
	totalSlots := int(oEnd.Sub(start) / step)
	if aEnd.After(oEnd) {
		totalSlots = int(aEnd.Sub(start) / step)
	}

	// Prepare the result slice with zero-value APIStats (acts as empty).
	newSegments := make([]T, totalSlots)

	// Copy/merge 's' into new slice at the proper offset.
	if s.FirstTime.After(start) {
		offset := int(s.FirstTime.Sub(start) / step)
		copy(newSegments[offset:offset+len(s.Segments)], s.Segments)
	} else {
		// s starts at 'start'
		copy(newSegments[:len(s.Segments)], s.Segments)
	}

	// Merge 'other' into new slice at the proper offset.
	otherOffset := int(other.FirstTime.Sub(start) / step)
	for i, s := range other.Segments {
		idx := otherOffset + i
		if idx < 0 || idx >= len(newSegments) {
			continue
		}
		pt := PT(&newSegments[idx])
		pt.Add(&s)
	}

	// Update receiver with merged result.
	s.FirstTime = start
	s.Segments = newSegments
}

// Total returns the total of all segments.
func (s *Segmented[T, PT]) Total() T {
	var res T
	if s == nil {
		return res
	}
	pt := PT(&res)
	for i := range s.Segments {
		pt.Add(&s.Segments[i])
	}
	// Since we are merging across APIs must reset track node count.
	return res
}

// ReplicationMetrics contains metrics for outgoing replication operations.
type ReplicationMetrics struct {
	// Time these metrics were collected
	CollectedAt time.Time `json:"collected"`

	// Nodes responded to the request.
	Nodes int `json:"nodes"`

	// Number of active replication events.
	Active int64 `json:"active,omitempty"`

	// Number of queued replication events.
	Queued int64 `json:"queued,omitempty"`

	Targets map[string]ReplicationTargetStats `json:"targets"`
}

func (m *ReplicationMetrics) Merge(other *ReplicationMetrics) {
	if m == nil || other == nil {
		return
	}
	if m.CollectedAt.Before(other.CollectedAt) {
		m.CollectedAt = other.CollectedAt
	}
	m.Nodes += other.Nodes
	m.Active += other.Active
	m.Queued += other.Queued

	if len(other.Targets) == 0 {
		return
	}
	if m.Targets == nil {
		m.Targets = make(map[string]ReplicationTargetStats, len(other.Targets))
	}
	for k, v := range other.Targets {
		dst := m.Targets[k]
		dst.Merge(&v)
		m.Targets[k] = dst
	}
}

// AllTargets returns aggregated stats for all targets.
func (m *ReplicationMetrics) AllTargets() ReplicationTargetStats {
	var dst ReplicationTargetStats
	for _, v := range m.Targets {
		dst.Merge(&v)
	}
	return dst
}

// ReplicationTargetStats is replication stats for a single target.
type ReplicationTargetStats struct {
	// Nodes responded to the request.
	Nodes int `json:"nodes"`

	// Last hour operation statistics per target.
	LastHour ReplicationStats `json:"last_hour,omitempty"`

	// Last day operation statistics per target, time segmented.
	LastDay *SegmentedReplicationStats `json:"last_day,omitempty"`

	// SinceStart contains operations by target.
	SinceStart ReplicationStats `json:"since_start"`
}

// Merge 'other' into 'r'
func (r *ReplicationTargetStats) Merge(other *ReplicationTargetStats) {
	if r == nil || other == nil || other.Nodes == 0 {
		return
	}
	r.Nodes += other.Nodes
	r.LastHour.Add(&other.LastHour)
	if r.LastDay == nil && other.LastDay != nil {
		var dst SegmentedReplicationStats
		dst.Add(other.LastDay)
		r.LastDay = &dst
	} else {
		r.LastDay.Add(other.LastDay)
	}
	r.SinceStart.Add(&other.SinceStart)
}

// ReplicationStats is the outgoing replication stats.
type ReplicationStats struct {
	Nodes        int        `json:"nodes,omitempty"`        // Number of nodes that have reported data.
	StartTime    *time.Time `json:"startTime,omitempty"`    // Time range this data covers unless merged from sources with different start times..
	EndTime      *time.Time `json:"endTime,omitempty"`      // Time range this data covers unless merged from sources with different end times.
	WallTimeSecs float64    `json:"wallTimeSecs,omitempty"` // Wall time this data covers, accumulated from all nodes.

	// Total number of replication events.
	Events        int64   `json:"events,omitempty"`   // Total number of requests.
	Bytes         int64   `json:"bytes,omitempty"`    // Total number of bytes sent to remote.
	EventTimeSecs float64 `json:"timeSecs,omitempty"` // Accumulated event time

	// Latency from queue time to completion.
	LatencySecs    float64 `json:"latency,omitempty"`    // Accumulated event latency for replication events for all nodes.
	MaxLatencySecs float64 `json:"maxLatency,omitempty"` // Maximum latency for a single node.

	// Replication event types.
	PutObject  int64 `json:"put,omitempty"`        // Total put replication requests.
	UpdateMeta int64 `json:"updateMeta,omitempty"` // Total metadata update requests.
	DelObject  int64 `json:"del,omitempty"`        // Total delete replication requests.
	DelTag     int64 `json:"delTag,omitempty"`     // Number of DELETE tagging request

	PutErrors        int64 `json:"putErrs,omitempty"`    // Replication PutObject event errors.
	UpdateMetaErrors int64 `json:"putTagErrs,omitempty"` // Replication Update Metadata errors.
	DelErrors        int64 `json:"delErrs,omitempty"`    // Replication DelObject event errors.
	DelTagErrors     int64 `json:"delTagErrs,omitempty"` // Replication DelTag event errors.

	// Outcome (if not error)
	Synced    int64 `json:"synced,omitempty"`    // Total synced replication requests (didn't exist on remote).
	AlreadyOK int64 `json:"alreadyOK,omitempty"` // Total already-ok replication requests (already existed on remote).
	Rejected  int64 `json:"rejected,omitempty"`  // Total rejected replication requests.

	// Proxy to remote counted separately.
	ProxyEvents int64 `json:"proxy,omitempty"`       // Number of proxy events.
	ProxyBytes  int64 `json:"proxyBytes,omitempty"`  // Number of bytes transferred from proxy requests.
	ProxyHead   int64 `json:"proxyHead,omitempty"`   // Number of HEAD requests proxied to replication target
	ProxyGet    int64 `json:"proxyGet,omitempty"`    // Number of GET requests proxied to replication target
	ProxyGetTag int64 `json:"proxyGetTag,omitempty"` // Number of GET tagging requests proxied to replication target

	ProxyHeadOK   int64 `json:"proxyHeadOK,omitempty"`   // Proxy HEAD requests that were successful.
	ProxyGetOK    int64 `json:"proxyGetOK,omitempty"`    // Proxy GET requests that were successful.
	ProxyGetTagOK int64 `json:"proxyGetTagOK,omitempty"` // Proxy GET TAG requests that were successful.
}

type SegmentedReplicationStats = Segmented[ReplicationStats, *ReplicationStats]

// Add 'other' to a.
func (a *ReplicationStats) Add(other *ReplicationStats) {
	if other == nil || other.Nodes == 0 {
		return
	}
	// Handle start/end times
	if a.StartTime == nil && a.Events == 0 {
		a.StartTime = other.StartTime
	}
	if a.EndTime == nil && a.Events == 0 {
		a.EndTime = other.EndTime
	}
	if a.StartTime != nil && other.StartTime != nil && !a.StartTime.Equal(*other.StartTime) {
		a.StartTime = nil
	}
	if a.EndTime != nil && other.EndTime != nil && !a.EndTime.Equal(*other.EndTime) {
		a.EndTime = nil
	}

	// Merge counters
	a.Nodes += other.Nodes
	a.WallTimeSecs += other.WallTimeSecs
	a.Events += other.Events
	a.Bytes += other.Bytes
	a.EventTimeSecs += other.EventTimeSecs

	// Event types
	a.PutObject += other.PutObject
	a.UpdateMeta += other.UpdateMeta
	a.DelObject += other.DelObject
	a.DelTag += other.DelTag

	a.LatencySecs += other.LatencySecs
	a.MaxLatencySecs = max(a.MaxLatencySecs, other.MaxLatencySecs)

	a.PutErrors += other.PutErrors
	a.UpdateMetaErrors += other.UpdateMetaErrors
	a.DelErrors += other.DelErrors
	a.DelTagErrors += other.DelTagErrors

	// Outcomes
	a.Synced += other.Synced
	a.AlreadyOK += other.AlreadyOK
	a.Rejected += other.Rejected

	// Proxy events
	a.ProxyEvents += other.ProxyEvents
	a.ProxyBytes += other.ProxyBytes
	a.ProxyHead += other.ProxyHead
	a.ProxyGet += other.ProxyGet
	a.ProxyGetTag += other.ProxyGetTag

	a.ProxyGetOK += other.ProxyGetOK
	a.ProxyGetTagOK += other.ProxyGetTagOK
	a.ProxyHeadOK += other.ProxyHeadOK
}

// ProcessMetrics contains aggregated minio process metrics
type ProcessMetrics struct {
	CollectedAt time.Time `json:"collected_at,omitempty"`
	Nodes       int       `json:"nodes,omitempty"`

	// Aggregated values
	TotalCPUPercent     float64 `json:"total_cpu_percent,omitempty"`
	TotalNumConnections int     `json:"total_num_connections,omitempty"`
	TotalRunningSecs    float64 `json:"total_running_secs,omitempty"`
	TotalNumFDs         int64   `json:"total_num_fds,omitempty"`
	TotalNumThreads     int64   `json:"total_num_threads,omitempty"`
	TotalNice           int64   `json:"total_nice,omitempty"`
	Count               int     `json:"count,omitempty"`

	// Counters for boolean fields
	BackgroundProcesses int `json:"background_processes,omitempty"`
	RunningProcesses    int `json:"running_processes,omitempty"`

	// Aggregated memory info
	MemInfo ProcessMemoryInfo `json:"mem_info,omitempty"`

	// Aggregated IO counters
	IOCounters ProcessIOCounters `json:"io_counters,omitempty"`

	// Aggregated context switches
	NumCtxSwitches ProcessCtxSwitches `json:"num_ctx_switches,omitempty"`

	// Aggregated page faults
	PageFaults ProcessPageFaults `json:"page_faults,omitempty"`

	// Aggregated CPU times
	CPUTimes ProcessCPUTimes `json:"cpu_times,omitempty"`

	// Aggregated memory maps (platform-specific)
	MemMaps ProcessMemoryMaps `json:"mem_maps,omitempty"`
}

// ProcessMemoryInfo represents aggregated memory information
type ProcessMemoryInfo struct {
	RSS    uint64 `json:"rss,omitempty"`
	VMS    uint64 `json:"vms,omitempty"`
	HWM    uint64 `json:"hwm,omitempty"`
	Data   uint64 `json:"data,omitempty"`
	Stack  uint64 `json:"stack,omitempty"`
	Locked uint64 `json:"locked,omitempty"`
	Swap   uint64 `json:"swap,omitempty"`
	Count  int    `json:"count,omitempty"`
	Shared uint64 `json:"shared,omitempty"`
}

// ProcessIOCounters represents aggregated IO counters
type ProcessIOCounters struct {
	ReadCount  uint64 `json:"read_count,omitempty"`
	WriteCount uint64 `json:"write_count,omitempty"`
	ReadBytes  uint64 `json:"read_bytes,omitempty"`
	WriteBytes uint64 `json:"write_bytes,omitempty"`
	Count      int    `json:"count,omitempty"`
}

// ProcessCtxSwitches represents aggregated context switches
type ProcessCtxSwitches struct {
	Voluntary   int64 `json:"voluntary,omitempty"`
	Involuntary int64 `json:"involuntary,omitempty"`
	Count       int   `json:"count,omitempty"`
}

// ProcessPageFaults represents aggregated page faults
type ProcessPageFaults struct {
	MinorFaults      uint64 `json:"minor_faults,omitempty"`
	MajorFaults      uint64 `json:"major_faults,omitempty"`
	ChildMinorFaults uint64 `json:"child_minor_faults,omitempty"`
	ChildMajorFaults uint64 `json:"child_major_faults,omitempty"`
	Count            int    `json:"count,omitempty"`
}

// ProcessCPUTimes represents aggregated CPU times
type ProcessCPUTimes struct {
	User      float64 `json:"user,omitempty"`
	System    float64 `json:"system,omitempty"`
	Idle      float64 `json:"idle,omitempty"`
	Nice      float64 `json:"nice,omitempty"`
	Iowait    float64 `json:"iowait,omitempty"`
	Irq       float64 `json:"irq,omitempty"`
	Softirq   float64 `json:"softirq,omitempty"`
	Steal     float64 `json:"steal,omitempty"`
	Guest     float64 `json:"guest,omitempty"`
	GuestNice float64 `json:"guest_nice,omitempty"`
	Count     int     `json:"count,omitempty"`
}

// ProcessMemoryMaps represents aggregated memory maps (platform-specific)
type ProcessMemoryMaps struct {
	TotalSize         uint64 `json:"total_size,omitempty"`
	TotalRSS          uint64 `json:"total_rss,omitempty"`
	TotalPSS          uint64 `json:"total_pss,omitempty"`
	TotalSharedClean  uint64 `json:"total_shared_clean,omitempty"`
	TotalSharedDirty  uint64 `json:"total_shared_dirty,omitempty"`
	TotalPrivateClean uint64 `json:"total_private_clean,omitempty"`
	TotalPrivateDirty uint64 `json:"total_private_dirty,omitempty"`
	TotalReferenced   uint64 `json:"total_referenced,omitempty"`
	TotalAnonymous    uint64 `json:"total_anonymous,omitempty"`
	TotalSwap         uint64 `json:"total_swap,omitempty"`
	Count             int    `json:"count,omitempty"`
}

// Merge merges process metrics from another ProcessMetrics
func (m *ProcessMetrics) Merge(other *ProcessMetrics) {
	if other == nil {
		return
	}

	// Update timestamp to the latest
	if other.CollectedAt.After(m.CollectedAt) {
		m.CollectedAt = other.CollectedAt
	}

	m.Nodes += other.Nodes
	m.TotalCPUPercent += other.TotalCPUPercent
	m.TotalNumConnections += other.TotalNumConnections
	m.TotalRunningSecs += other.TotalRunningSecs
	m.TotalNumFDs += other.TotalNumFDs
	m.TotalNumThreads += other.TotalNumThreads
	m.TotalNice += other.TotalNice
	m.Count += other.Count

	// Merge boolean counters
	m.BackgroundProcesses += other.BackgroundProcesses
	m.RunningProcesses += other.RunningProcesses

	// Merge memory info
	m.MemInfo.RSS += other.MemInfo.RSS
	m.MemInfo.VMS += other.MemInfo.VMS
	m.MemInfo.HWM += other.MemInfo.HWM
	m.MemInfo.Data += other.MemInfo.Data
	m.MemInfo.Stack += other.MemInfo.Stack
	m.MemInfo.Locked += other.MemInfo.Locked
	m.MemInfo.Swap += other.MemInfo.Swap
	m.MemInfo.Count += other.MemInfo.Count
	m.MemInfo.Shared += other.MemInfo.Shared

	// Merge IO counters
	m.IOCounters.ReadCount += other.IOCounters.ReadCount
	m.IOCounters.WriteCount += other.IOCounters.WriteCount
	m.IOCounters.ReadBytes += other.IOCounters.ReadBytes
	m.IOCounters.WriteBytes += other.IOCounters.WriteBytes
	m.IOCounters.Count += other.IOCounters.Count

	// Merge context switches
	m.NumCtxSwitches.Voluntary += other.NumCtxSwitches.Voluntary
	m.NumCtxSwitches.Involuntary += other.NumCtxSwitches.Involuntary
	m.NumCtxSwitches.Count += other.NumCtxSwitches.Count

	// Merge page faults
	m.PageFaults.MinorFaults += other.PageFaults.MinorFaults
	m.PageFaults.MajorFaults += other.PageFaults.MajorFaults
	m.PageFaults.ChildMinorFaults += other.PageFaults.ChildMinorFaults
	m.PageFaults.ChildMajorFaults += other.PageFaults.ChildMajorFaults
	m.PageFaults.Count += other.PageFaults.Count

	// Merge CPU times
	m.CPUTimes.User += other.CPUTimes.User
	m.CPUTimes.System += other.CPUTimes.System
	m.CPUTimes.Idle += other.CPUTimes.Idle
	m.CPUTimes.Nice += other.CPUTimes.Nice
	m.CPUTimes.Iowait += other.CPUTimes.Iowait
	m.CPUTimes.Irq += other.CPUTimes.Irq
	m.CPUTimes.Softirq += other.CPUTimes.Softirq
	m.CPUTimes.Steal += other.CPUTimes.Steal
	m.CPUTimes.Guest += other.CPUTimes.Guest
	m.CPUTimes.GuestNice += other.CPUTimes.GuestNice
	m.CPUTimes.Count += other.CPUTimes.Count

	// Merge memory maps
	m.MemMaps.TotalSize += other.MemMaps.TotalSize
	m.MemMaps.TotalRSS += other.MemMaps.TotalRSS
	m.MemMaps.TotalPSS += other.MemMaps.TotalPSS
	m.MemMaps.TotalSharedClean += other.MemMaps.TotalSharedClean
	m.MemMaps.TotalSharedDirty += other.MemMaps.TotalSharedDirty
	m.MemMaps.TotalPrivateClean += other.MemMaps.TotalPrivateClean
	m.MemMaps.TotalPrivateDirty += other.MemMaps.TotalPrivateDirty
	m.MemMaps.TotalReferenced += other.MemMaps.TotalReferenced
	m.MemMaps.TotalAnonymous += other.MemMaps.TotalAnonymous
	m.MemMaps.TotalSwap += other.MemMaps.TotalSwap
	m.MemMaps.Count += other.MemMaps.Count
}
