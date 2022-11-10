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
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// NetperfNodeResult - stats from each server
type NetperfNodeResult struct {
	Endpoint string `json:"endpoint"`
	TX       uint64 `json:"tx"`
	RX       uint64 `json:"rx"`
	Error    string `json:"error,omitempty"`
}

// NetperfResult - aggregate results from all servers
type NetperfResult struct {
	NodeResults []NetperfNodeResult `json:"nodeResults"`
}

// Netperf - perform netperf on the MinIO servers
func (adm *AdminClient) Netperf(ctx context.Context, duration time.Duration) (result NetperfResult, err error) {
	queryVals := make(url.Values)
	queryVals.Set("duration", duration.String())

	resp, err := adm.executeMethod(ctx,
		http.MethodPost, requestData{
			relPath:     adminAPIPrefix + "/speedtest/net",
			queryValues: queryVals,
		})
	defer closeResponse(resp)
	if err != nil {
		return result, err
	}
	if resp.StatusCode != http.StatusOK {
		return result, httpRespToErrorResponse(resp)
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}

type NetperfClientResult struct {
	Endpoint  string `json:"endpoint"`
	Bandwidth uint64 `json:"bw"`
	Error     string `json:"error"`
}

// Reader to read random data.
type netperfReader struct {
	n   uint64
	eof chan struct{}
	buf []byte
}

func (m *netperfReader) BytesRead() uint64 {
	return atomic.LoadUint64(&m.n)
}

func (m *netperfReader) Read(b []byte) (int, error) {
	select {
	case <-m.eof:
		return 0, io.EOF
	default:
	}
	n := copy(b, m.buf)
	atomic.AddUint64(&m.n, uint64(n))
	return n, nil
}

// NetperfClient - perform network benchmark from client to server.
func (adm *AdminClient) NetperfClient(ctx context.Context, duration time.Duration) (result NetperfClientResult, err error) {
	r := &netperfReader{eof: make(chan struct{})}
	r.buf = make([]byte, 128*(1<<10))
	rand.Read(r.buf)

	connectionsPerPeer := 16

	errStr := ""
	var wg sync.WaitGroup

	for i := 0; i < connectionsPerPeer; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			resp, err := adm.executeMethod(ctx,
				http.MethodPost, requestData{
					relPath:       adminAPIPrefix + "/speedtest/netclient",
					contentReader: io.NopCloser(r),
				})
			closeResponse(resp)

			if err != nil {
				errStr = err.Error()
			} else if resp.StatusCode != http.StatusOK {
				errStr = resp.Status
			}
		}()
	}

	time.Sleep(duration)
	close(r.eof)
	wg.Wait()

	bw := ((r.BytesRead() / uint64(duration.Milliseconds())) * 1000)
	if errStr != "" {
		return NetperfClientResult{
			Endpoint:  adm.endpointURL.String(),
			Bandwidth: bw,
			Error:     errStr,
		}, nil
	}

	return NetperfClientResult{
		Endpoint:  adm.endpointURL.String(),
		Bandwidth: bw,
	}, nil
}
