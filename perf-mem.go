//
// Copyright (c) 2015-2026 MinIO, Inc.
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
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// MemPerfTopologySource records where a node's memory topology came from, so a
// consumer can tell a real theoretical figure from an absent one rather than
// silently trusting a zero.
type MemPerfTopologySource string

const (
	// MemPerfTopologyDMI - read from SMBIOS type 17 memory device entries.
	MemPerfTopologyDMI MemPerfTopologySource = "dmi"

	// MemPerfTopologyEDAC - derived from the kernel EDAC memory controller tree,
	// which reports sizes and channel layout but not clocks.
	MemPerfTopologyEDAC MemPerfTopologySource = "edac"

	// MemPerfTopologyUnavailable - neither source was readable; only the
	// measured bandwidth is meaningful.
	MemPerfTopologyUnavailable MemPerfTopologySource = "unavailable"
)

// MemPerfDIMM describes one populated memory device as the firmware reports it.
type MemPerfDIMM struct {
	Locator       string `json:"locator,omitempty"`
	SizeBytes     uint64 `json:"sizeBytes,omitempty"`
	MaxSpeedMTs   uint32 `json:"maxSpeedMTs,omitempty"`
	ConfiguredMTs uint32 `json:"configuredMTs,omitempty"`
}

// MemPerfNodeResult holds one server's memory bandwidth measurement alongside
// the topology it should be judged against.
//
// BandwidthBps is what the hardware actually delivered under a saturating load;
// TheoreticalBps is what the populated DIMMs allow. The ratio between them is
// the number worth acting on: a node far below its own theoretical figure is
// usually mispopulated or downclocked, which no amount of software tuning will
// recover.
type MemPerfNodeResult struct {
	Endpoint string `json:"endpoint"`

	// Measured under a saturating multi-threaded load.
	BandwidthBps uint64        `json:"bandwidthBps"`
	PerNUMABps   []uint64      `json:"perNumaBps,omitempty"`
	Threads      int           `json:"threads,omitempty"`
	BufferBytes  uint64        `json:"bufferBytes,omitempty"`
	Duration     time.Duration `json:"duration,omitempty"`

	// Best-effort topology. Absent fields mean unreadable, not zero.
	TheoreticalBps uint64                `json:"theoreticalBps,omitempty"`
	TopologySource MemPerfTopologySource `json:"topologySource,omitempty"`
	Channels       int                   `json:"channels,omitempty"`
	ConfiguredMTs  uint32                `json:"configuredMTs,omitempty"`
	MaxSpeedMTs    uint32                `json:"maxSpeedMTs,omitempty"`
	NUMANodes      int                   `json:"numaNodes,omitempty"`
	DIMMs          []MemPerfDIMM         `json:"dimms,omitempty"`

	Error string `json:"error,omitempty"`
}

// Efficiency returns measured bandwidth as a fraction of theoretical, or 0 when
// the topology could not be read. Values below roughly 0.65 are worth
// investigating before trusting any other benchmark from the node.
func (r MemPerfNodeResult) Efficiency() float64 {
	if r.TheoreticalBps == 0 {
		return 0
	}
	return float64(r.BandwidthBps) / float64(r.TheoreticalBps)
}

// Downclocked reports whether the DIMMs are running below their rated speed.
// Theoretical bandwidth is computed from the configured clock, so a downclocked
// node can show good efficiency while still leaving bandwidth on the table.
func (r MemPerfNodeResult) Downclocked() bool {
	return r.MaxSpeedMTs > 0 && r.ConfiguredMTs > 0 && r.ConfiguredMTs < r.MaxSpeedMTs
}

// MemPerfResult - aggregate results from all servers.
type MemPerfResult struct {
	NodeResults []MemPerfNodeResult `json:"nodeResults"`
}

// MemPerfOpts provide configurable options for the memory bandwidth test.
type MemPerfOpts struct {
	// Duration of the saturating load. Zero picks the server default.
	Duration time.Duration

	// Threads issuing traffic. Zero uses every available core, which is what
	// saturation requires on a many-core host.
	Threads int

	// BufferSize is the per-thread working set. It must exceed the last-level
	// cache or the test measures cache, not memory. Zero picks the server
	// default.
	BufferSize uint64
}

// MemPerf runs a memory bandwidth test on every MinIO server.
//
// The test deliberately saturates memory for its duration and will slow
// anything else running on those nodes; treat it as a deployment validation
// step rather than something to run against a live workload.
func (adm *AdminClient) MemPerf(ctx context.Context, opts MemPerfOpts) (result MemPerfResult, err error) {
	queryVals := make(url.Values)
	if opts.Duration > 0 {
		queryVals.Set("duration", opts.Duration.String())
	}
	if opts.Threads > 0 {
		queryVals.Set("threads", strconv.Itoa(opts.Threads))
	}
	if opts.BufferSize > 0 {
		queryVals.Set("buffersize", strconv.FormatUint(opts.BufferSize, 10))
	}

	resp, err := adm.executeMethod(ctx,
		http.MethodPost, requestData{
			relPath:     adminAPIPrefix + "/speedtest/mem",
			queryValues: queryVals,
		})
	if err != nil {
		return result, err
	}
	defer closeResponse(resp)

	if resp.StatusCode != http.StatusOK {
		return result, httpRespToErrorResponse(resp)
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}
