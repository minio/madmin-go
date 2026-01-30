// Copyright (c) 2015-2025 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package log

import (
	"fmt"
	"strings"
	"time"
)

//msgp:tag json
//go:generate msgp -d clearomitted -d "timezone utc" $GOFILE

//msgp:replace Origin with:string

// Origin for the API event
type Origin string

const (
	OriginClient          Origin = "client"
	OriginSiteReplication Origin = "site-replication"
	OriginILM             Origin = "ilm"
	OriginBatch           Origin = "batch"
	OriginRebalance       Origin = "rebalance"
	OriginReplicate       Origin = "replicate"
	OriginDecommission    Origin = "decommission"
	OriginHeal            Origin = "heal"
)

type APIType string

const (
	APITypeObject APIType = "object"
	APITypeBucket APIType = "bucket"
	APITypeAdmin  APIType = "admin"
	APITypeAuth   APIType = "auth"
)

// API represents the api event
type API struct {
	Version   string            `json:"version" parquet:"version"`
	Time      time.Time         `json:"time" parquet:"time,timestamp(microsecond)"`
	Node      string            `json:"node,omitempty" parquet:"node,optional"`
	Origin    Origin            `json:"origin,omitempty" parquet:"origin,optional"`
	Type      APIType           `json:"type,omitempty" parquet:"type,optional"`
	Name      string            `json:"name,omitempty" parquet:"name,optional"`
	Bucket    string            `json:"bucket,omitempty" parquet:"bucket,optional"`
	Object    string            `json:"object,omitempty" parquet:"object,optional"`
	VersionID string            `json:"versionId,omitempty" parquet:"versionId,optional"`
	Tags      map[string]string `json:"tags,omitempty" parquet:"tags,optional"`
	CallInfo  *CallInfo         `json:"callInfo,omitempty" parquet:"callInfo,optional"`
}

// CallInfo represents the info for the external call
type CallInfo struct {
	HTTPStatusCode    int               `json:"httpStatusCode,omitempty" parquet:"httpStatusCode,optional"`
	InputBytes        int64             `json:"rx,omitempty" parquet:"inputBytes,optional"`
	OutputBytes       int64             `json:"tx,omitempty" parquet:"outputBytes,optional"`
	HeaderBytes       int64             `json:"txHeaders,omitempty" parquet:"headerBytes,optional"`
	TimeToFirstByte   string            `json:"timeToFirstByte,omitempty" parquet:"timeToFirstByte,optional"`
	RequestReadTime   string            `json:"requestReadTime,omitempty" parquet:"requestReadTime,optional"`
	ResponseWriteTime string            `json:"responseWriteTime,omitempty" parquet:"responseWriteTime,optional"`
	RequestTime       string            `json:"requestTime,omitempty" parquet:"requestTime,optional"`
	TimeToResponse    string            `json:"timeToResponse,omitempty" parquet:"timeToResponse,optional"`
	ReadBlocked       string            `json:"readBlocked,omitempty" parquet:"readBlocked,optional"`
	WriteBlocked      string            `json:"writeBlocked,omitempty" parquet:"writeBlocked,optional"`
	SourceHost        string            `json:"sourceHost,omitempty" parquet:"sourceHost,optional"`
	RequestID         string            `json:"requestID,omitempty" parquet:"requestId,optional"`
	UserAgent         string            `json:"userAgent,omitempty" parquet:"userAgent,optional"`
	ReqPath           string            `json:"requestPath,omitempty" parquet:"reqPath,optional"`
	ReqHost           string            `json:"requestHost,omitempty" parquet:"reqHost,optional"`
	ReqClaims         map[string]string `json:"requestClaims,omitempty" parquet:"reqClaims,optional"`
	ReqQuery          map[string]string `json:"requestQuery,omitempty" parquet:"reqQuery,optional"`
	ReqHeader         map[string]string `json:"requestHeader,omitempty" parquet:"reqHeader,optional"`
	RespHeader        map[string]string `json:"responseHeader,omitempty" parquet:"respHeader,optional"`
	AccessKey         string            `json:"accessKey,omitempty" parquet:"accessKey,optional"`
	ParentUser        string            `json:"parentUser,omitempty" parquet:"parentUser,optional"`
}

// String provides a canonical representation for API
func (a API) String() string {
	values := []string{
		toString("version", a.Version),
		toTime("time", a.Time),
		toString("node", a.Node),
		toString("origin", string(a.Origin)),
		toString("type", string(a.Type)),
		toString("name", a.Name),
		toString("bucket", a.Bucket),
		toString("object", a.Object),
		toString("versionId", a.VersionID),
		toMap("tags", a.Tags),
	}
	if a.CallInfo != nil {
		values = append(values, fmt.Sprintf("callInfo={%s}", a.CallInfo.String()))
	}
	values = filterAndSort(values)
	return strings.Join(values, ",")
}

// String returns the canonical string for CallInfo
func (c CallInfo) String() string {
	values := []string{
		toInt("httpStatusCode", c.HTTPStatusCode),
		toInt64("rx", c.InputBytes),
		toInt64("tx", c.OutputBytes),
		toInt64("txHeaders", c.HeaderBytes),
		toString("timeToFirstByte", c.TimeToFirstByte),
		toString("requestReadTime", c.RequestReadTime),
		toString("responseWriteTime", c.ResponseWriteTime),
		toString("requestTime", c.RequestTime),
		toString("timeToResponse", c.TimeToResponse),
		toString("readBlocked", c.ReadBlocked),
		toString("writeBlocked", c.WriteBlocked),
		toString("sourceHost", c.SourceHost),
		toString("requestID", c.RequestID),
		toString("userAgent", c.UserAgent),
		toString("requestPath", c.ReqPath),
		toString("requestHost", c.ReqHost),
		toMap("requestClaims", c.ReqClaims),
		toMap("requestQuery", c.ReqQuery),
		toMap("requestHeader", c.ReqHeader),
		toMap("responseHeader", c.RespHeader),
		toString("accessKey", c.AccessKey),
		toString("parentUser", c.ParentUser),
	}
	values = filterAndSort(values)
	return strings.Join(values, ",")
}
