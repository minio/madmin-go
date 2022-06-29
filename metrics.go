//
// MinIO Object Storage (c) 2022 MinIO, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
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
	"sort"
	"strconv"
	"strings"
	"time"
)

// MetricType is a bitfield representation of different metric types.
type MetricType uint32

// MetricsNone indicates no metrics.
const MetricsNone MetricType = 0

const (
	MetricsScanner MetricType = 1 << (iota)
	MetricsDisk
	MetricsOS

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
	Hosts      []string           `json:"hosts"`
	Aggregated Metrics            `json:"aggregated"`
	ByHost     map[string]Metrics `json:"by_host,omitempty"`
	// Final indicates whether this is the final packet and the receiver can exit.
	Final bool `json:"final"`
}

// Metrics contains all metric types.
type Metrics struct {
	Scanner *ScannerMetrics `json:"scanner,omitempty"`
	Disk    *DiskMetric     `json:"disk,omitempty"`
	OS      *OSMetrics      `json:"os,omitempty"`
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
}

// ScannerMetrics contains scanner information.
type ScannerMetrics struct {
	// Time these metrics were collected
	CollectedAt time.Time `json:"collected"`

	// Current scanner cycle
	CurrentCycle uint64 `json:"current_cycle"`

	// Start time of current cycle
	CurrentStarted time.Time `json:"current_started"`

	// History of when last cycles completed
	CyclesCompletedAt []time.Time `json:"cycle_complete_times"`

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

// TimedAction contains a number of actions and their accumulated duration in nanoseconds.
type TimedAction struct {
	Count   uint64 `json:"count"`
	AccTime uint64 `json:"acc_time_ns"`
	Bytes   uint64 `json:"bytes,omitempty"`
}

// Avg returns the average time spent on the action.
func (t TimedAction) Avg() time.Duration {
	if t.Count == 0 {
		return 0
	}
	return time.Duration(t.AccTime / t.Count)
}

// AvgBytes returns the average time spent on the action.
func (t TimedAction) AvgBytes() uint64 {
	if t.Count == 0 {
		return 0
	}
	return t.Bytes / t.Count
}

// Merge other into t.
func (t *TimedAction) Merge(other TimedAction) {
	t.Count += other.Count
	t.AccTime += other.AccTime
	t.Bytes += other.Bytes
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
