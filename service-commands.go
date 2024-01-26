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
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ServiceRestartV2 - restarts the MinIO cluster
func (adm *AdminClient) ServiceRestartV2(ctx context.Context) error {
	_, err := adm.serviceCallActionV2(ctx, ServiceActionOpts{Action: ServiceActionRestart})
	return err
}

// ServiceStop - stops the MinIO cluster
func (adm *AdminClient) ServiceStopV2(ctx context.Context) error {
	_, err := adm.serviceCallActionV2(ctx, ServiceActionOpts{Action: ServiceActionStop})
	return err
}

// ServiceFreeze - freezes all incoming S3 API calls on MinIO cluster
func (adm *AdminClient) ServiceFreezeV2(ctx context.Context) error {
	_, err := adm.serviceCallActionV2(ctx, ServiceActionOpts{Action: ServiceActionFreeze})
	return err
}

// ServiceUnfreeze - un-freezes all incoming S3 API calls on MinIO cluster
func (adm *AdminClient) ServiceUnfreezeV2(ctx context.Context) error {
	_, err := adm.serviceCallActionV2(ctx, ServiceActionOpts{Action: ServiceActionUnfreeze})
	return err
}

// ServiceAction - type to restrict service-action values
type ServiceAction string

const (
	// ServiceActionRestart represents restart action
	ServiceActionRestart ServiceAction = "restart"
	// ServiceActionStop represents stop action
	ServiceActionStop = "stop"
	// ServiceActionFreeze represents freeze action
	ServiceActionFreeze = "freeze"
	// ServiceActionUnfreeze represents unfreeze a previous freeze action
	ServiceActionUnfreeze = "unfreeze"
)

// ServiceActionOpts specifies the action that the service is requested
// to take, dryRun indicates if the action is a no-op, force indicates
// that server must make best effort to restart the process.
type ServiceActionOpts struct {
	Action ServiceAction
	DryRun bool
}

// ServiceActionPeerResult service peer result
type ServiceActionPeerResult struct {
	Host          string                 `json:"host"`
	Err           string                 `json:"err,omitempty"`
	WaitingDrives map[string]DiskMetrics `json:"waitingDrives,omitempty"`
}

// ServiceActionResult service action result
type ServiceActionResult struct {
	Action  ServiceAction             `json:"action"`
	DryRun  bool                      `json:"dryRun"`
	Results []ServiceActionPeerResult `json:"results,omitempty"`
}

// ServiceAction - specify the type of service action that we are requesting the server to perform
func (adm *AdminClient) ServiceAction(ctx context.Context, opts ServiceActionOpts) (ServiceActionResult, error) {
	return adm.serviceCallActionV2(ctx, opts)
}

// serviceCallActionV2 - call service restart/stop/freeze/unfreeze
func (adm *AdminClient) serviceCallActionV2(ctx context.Context, opts ServiceActionOpts) (ServiceActionResult, error) {
	queryValues := url.Values{}
	queryValues.Set("action", string(opts.Action))
	queryValues.Set("dry-run", strconv.FormatBool(opts.DryRun))
	queryValues.Set("type", "2")

	// Request API to Restart server
	resp, err := adm.executeMethod(ctx,
		http.MethodPost, requestData{
			relPath:     adminAPIPrefix + "/service",
			queryValues: queryValues,
		},
	)
	defer closeResponse(resp)
	if err != nil {
		return ServiceActionResult{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return ServiceActionResult{}, httpRespToErrorResponse(resp)
	}

	srvRes := ServiceActionResult{}
	dec := json.NewDecoder(resp.Body)
	if err = dec.Decode(&srvRes); err != nil {
		return ServiceActionResult{}, err
	}

	return srvRes, nil
}

// ServiceTraceInfo holds http trace
type ServiceTraceInfo struct {
	Trace TraceInfo
	Err   error `json:"-"`
}

// ServiceTraceOpts holds tracing options
type ServiceTraceOpts struct {
	// Trace types:
	S3                bool
	Internal          bool
	Storage           bool
	OS                bool
	Scanner           bool
	Decommission      bool
	Healing           bool
	BatchReplication  bool
	BatchKeyRotation  bool
	BatchExpire       bool
	BatchAll          bool
	Rebalance         bool
	ReplicationResync bool
	Bootstrap         bool
	FTP               bool
	ILM               bool
	OnlyErrors        bool
	Threshold         time.Duration
}

// TraceTypes returns the enabled traces as a bitfield value.
func (t ServiceTraceOpts) TraceTypes() TraceType {
	var tt TraceType
	tt.SetIf(t.S3, TraceS3)
	tt.SetIf(t.Internal, TraceInternal)
	tt.SetIf(t.Storage, TraceStorage)
	tt.SetIf(t.OS, TraceOS)
	tt.SetIf(t.Scanner, TraceScanner)
	tt.SetIf(t.Decommission, TraceDecommission)
	tt.SetIf(t.Healing, TraceHealing)
	if t.BatchAll {
		tt.SetIf(true, TraceBatchReplication)
		tt.SetIf(true, TraceBatchKeyRotation)
		tt.SetIf(true, TraceBatchExpire)
	} else {
		tt.SetIf(t.BatchReplication, TraceBatchReplication)
		tt.SetIf(t.BatchKeyRotation, TraceBatchKeyRotation)
		tt.SetIf(t.BatchExpire, TraceBatchExpire)
	}

	tt.SetIf(t.Rebalance, TraceRebalance)
	tt.SetIf(t.ReplicationResync, TraceReplicationResync)
	tt.SetIf(t.Bootstrap, TraceBootstrap)
	tt.SetIf(t.FTP, TraceFTP)
	tt.SetIf(t.ILM, TraceILM)

	return tt
}

// AddParams will add parameter to url values.
func (t ServiceTraceOpts) AddParams(u url.Values) {
	u.Set("err", strconv.FormatBool(t.OnlyErrors))
	u.Set("threshold", t.Threshold.String())

	u.Set("s3", strconv.FormatBool(t.S3))
	u.Set("internal", strconv.FormatBool(t.Internal))
	u.Set("storage", strconv.FormatBool(t.Storage))
	u.Set("os", strconv.FormatBool(t.OS))
	u.Set("scanner", strconv.FormatBool(t.Scanner))
	u.Set("decommission", strconv.FormatBool(t.Decommission))
	u.Set("healing", strconv.FormatBool(t.Healing))
	u.Set("batch-replication", strconv.FormatBool(t.BatchReplication))
	u.Set("batch-keyrotation", strconv.FormatBool(t.BatchKeyRotation))
	u.Set("batch-expire", strconv.FormatBool(t.BatchExpire))
	if t.BatchAll {
		u.Set("batch-replication", "true")
		u.Set("batch-keyrotation", "true")
		u.Set("batch-expire", "true")
	}
	u.Set("rebalance", strconv.FormatBool(t.Rebalance))
	u.Set("replication-resync", strconv.FormatBool(t.ReplicationResync))
	u.Set("bootstrap", strconv.FormatBool(t.Bootstrap))
	u.Set("ftp", strconv.FormatBool(t.FTP))
	u.Set("ilm", strconv.FormatBool(t.ILM))
}

// ParseParams will parse parameters and set them to t.
func (t *ServiceTraceOpts) ParseParams(r *http.Request) (err error) {
	t.S3 = r.Form.Get("s3") == "true"
	t.OS = r.Form.Get("os") == "true"
	t.Scanner = r.Form.Get("scanner") == "true"
	t.Decommission = r.Form.Get("decommission") == "true"
	t.Healing = r.Form.Get("healing") == "true"
	t.BatchReplication = r.Form.Get("batch-replication") == "true"
	t.BatchKeyRotation = r.Form.Get("batch-keyrotation") == "true"
	t.BatchExpire = r.Form.Get("batch-expire") == "true"
	t.Rebalance = r.Form.Get("rebalance") == "true"
	t.Storage = r.Form.Get("storage") == "true"
	t.Internal = r.Form.Get("internal") == "true"
	t.OnlyErrors = r.Form.Get("err") == "true"
	t.ReplicationResync = r.Form.Get("replication-resync") == "true"
	t.Bootstrap = r.Form.Get("bootstrap") == "true"
	t.FTP = r.Form.Get("ftp") == "true"
	t.ILM = r.Form.Get("ilm") == "true"

	if th := r.Form.Get("threshold"); th != "" {
		d, err := time.ParseDuration(th)
		if err != nil {
			return err
		}
		t.Threshold = d
	}
	return nil
}

// ServiceTrace - listen on http trace notifications.
func (adm AdminClient) ServiceTrace(ctx context.Context, opts ServiceTraceOpts) <-chan ServiceTraceInfo {
	traceInfoCh := make(chan ServiceTraceInfo)
	// Only success, start a routine to start reading line by line.
	go func(traceInfoCh chan<- ServiceTraceInfo) {
		defer close(traceInfoCh)
		for {
			urlValues := make(url.Values)
			opts.AddParams(urlValues)

			reqData := requestData{
				relPath:     adminAPIPrefix + "/trace",
				queryValues: urlValues,
			}
			// Execute GET to call trace handler
			resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
			if err != nil {
				traceInfoCh <- ServiceTraceInfo{Err: err}
				return
			}

			if resp.StatusCode != http.StatusOK {
				closeResponse(resp)
				traceInfoCh <- ServiceTraceInfo{Err: httpRespToErrorResponse(resp)}
				return
			}

			dec := json.NewDecoder(resp.Body)
			for {
				var info traceInfoLegacy
				if err = dec.Decode(&info); err != nil {
					closeResponse(resp)
					traceInfoCh <- ServiceTraceInfo{Err: err}
					break
				}
				// Convert if legacy...
				if info.TraceType == TraceType(0) {
					if strings.HasPrefix(info.FuncName, "s3.") {
						info.TraceType = TraceS3
					} else {
						info.TraceType = TraceInternal
					}
					info.HTTP = &TraceHTTPStats{}
					if info.ReqInfo != nil {
						info.Path = info.ReqInfo.Path
						info.HTTP.ReqInfo = *info.ReqInfo
					}
					if info.RespInfo != nil {
						info.HTTP.RespInfo = *info.RespInfo
					}
					if info.CallStats != nil {
						info.Duration = info.CallStats.Latency
						info.HTTP.CallStats = *info.CallStats
					}
				}
				if info.TraceType == TraceOS && info.OSStats != nil {
					info.Path = info.OSStats.Path
					info.Duration = info.OSStats.Duration
				}
				if info.TraceType == TraceStorage && info.StorageStats != nil {
					info.Path = info.StorageStats.Path
					info.Duration = info.StorageStats.Duration
				}
				select {
				case <-ctx.Done():
					closeResponse(resp)
					return
				case traceInfoCh <- ServiceTraceInfo{Trace: info.TraceInfo}:
				}
			}
		}
	}(traceInfoCh)

	// Returns the trace info channel, for caller to start reading from.
	return traceInfoCh
}
