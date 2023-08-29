//
// Copyright (c) 2015-2023 MinIO, Inc.
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
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"time"
)

// SiteNetPerfNodeResult  - stats from each server
type SiteNetPerfNodeResult struct {
	Endpoint        string                       `json:"endpoint"`
	TX              uint64                       `json:"tx"` // transfer rate in bytes
	TXTotalDuration time.Duration                `json:"txTotalDuration"`
	RX              uint64                       `json:"rx"` // received rate in bytes
	RXTotalDuration time.Duration                `json:"rxTotalDuration"`
	TotalConn       uint64                       `json:"totalConn"`
	Error           string                       `json:"error,omitempty"`
	Latency         SiteNetPerfNodeLatencyResult `json:"latency"`
}

type SiteNetPerfNodeLatencyResult struct {
	Error             string        `json:"error,omitempty"`
	Endpoint          string        `json:"endpoint"`
	TotalResponseTime time.Duration `json:"totalResponseTime,omitempty"`
	TimeToFirstByte   time.Duration `json:"timeToFirstByte,omitempty"`
	TotalRequest      int           `json:"totalRequest"`
}

// SiteNetPerfResult  - aggregate results from all servers
type SiteNetPerfResult struct {
	NodeResults []SiteNetPerfNodeResult `json:"nodeResults"`
}

// SiteReplicationPerf - perform site-replication on the MinIO servers
func (adm *AdminClient) SiteReplicationPerf(ctx context.Context, duration time.Duration) (result SiteNetPerfResult, err error) {
	queryVals := make(url.Values)
	queryVals.Set("duration", duration.String())

	resp, err := adm.executeMethod(ctx,
		http.MethodPost, requestData{
			relPath:     adminAPIPrefix + "/speedtest/site",
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

// SiteNetPerfReader - for TTFB and TotalResponseTime reader (4KB stream)
type SiteNetPerfReader struct {
	TimeToFirstByte   time.Duration
	TotalResponseTime time.Duration
	buf               []byte
	startTime         time.Time
	readCounter       uint64
}

func (s *SiteNetPerfReader) Start() {
	// transform 4kb
	buf := make([]byte, 4*1024)
	rand.Read(buf)
	s.buf = buf
	s.startTime = time.Now()
}

func (s *SiteNetPerfReader) Read(p []byte) (n int, err error) {
	s.readCounter++
	switch s.readCounter {
	case 1:
		firstN := copy(p, s.buf[:1])
		s.TimeToFirstByte = time.Since(s.startTime)
		n = copy(p, s.buf[1:])
		n += firstN
	default:
		return 0, io.EOF
	}
	return n, nil
}

func (s *SiteNetPerfReader) End() {
	s.TotalResponseTime = time.Since(s.startTime)
}

var _ io.Reader = &SiteNetPerfReader{}
