//
// MinIO Object Storage (c) 2021 MinIO, Inc.
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
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// SpeedtestResult - response from the server containing speedtest result
type SpeedtestResult struct {
	Uploads   uint64 `json:"uploads"`
	Downloads uint64 `json:"downloads"`
	Endpoint  string `json:"endpoint"`
	Err       error  `json:"err,omitempty"`
}

// SpeedtestOpts provide configurable options for speedtest
type SpeedtestOpts struct {
	Size        int           // Object size used in speed test
	Concurrency int           // Concurrency used in speed test
	Duration    time.Duration // Total duration of the speed test
}

// Speedtest - perform speedtest on the MinIO servers
func (adm *AdminClient) Speedtest(ctx context.Context, opts SpeedtestOpts) ([]SpeedtestResult, error) {
	queryVals := make(url.Values)
	queryVals.Set("size", strconv.Itoa(opts.Size))
	queryVals.Set("duration", opts.Duration.String())
	queryVals.Set("concurrent", strconv.Itoa(opts.Concurrency))
	resp, err := adm.executeMethod(ctx,
		http.MethodPost, requestData{
			relPath:     adminAPIPrefix + "/speedtest",
			queryValues: queryVals,
		})
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var result []SpeedtestResult
	err = json.Unmarshal(respBytes, &result)
	return result, err
}
