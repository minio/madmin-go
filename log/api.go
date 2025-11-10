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
	Version   string            `json:"version"`
	Time      time.Time         `json:"time"`
	Node      string            `json:"node,omitempty"`
	Origin    Origin            `json:"origin,omitempty"`
	Type      APIType           `json:"type,omitempty"`
	Name      string            `json:"name,omitempty"`
	Bucket    string            `json:"bucket,omitempty"`
	Object    string            `json:"object,omitempty"`
	VersionID string            `json:"versionId,omitempty"`
	Tags      map[string]string `json:"tags,omitempty"`
	CallInfo  *CallInfo         `json:"callInfo,omitempty"`
}

// CallInfo represents the info for the external call
type CallInfo struct {
	HTTPStatusCode    int                    `json:"httpStatusCode,omitempty"`
	InputBytes        int64                  `json:"rx,omitempty"`
	OutputBytes       int64                  `json:"tx,omitempty"`
	HeaderBytes       int64                  `json:"txHeaders,omitempty"`
	TimeToFirstByte   string                 `json:"timeToFirstByte,omitempty"`
	RequestReadTime   string                 `json:"requestReadTime,omitempty"`
	ResponseWriteTime string                 `json:"responseWriteTime,omitempty"`
	RequestTime       string                 `json:"requestTime,omitempty"`
	TimeToResponse    string                 `json:"timeToResponse,omitempty"`
	ReadBlocked       string                 `json:"readBlocked,omitempty"`
	WriteBlocked      string                 `json:"writeBlocked,omitempty"`
	SourceHost        string                 `json:"sourceHost,omitempty"`
	RequestID         string                 `json:"requestID,omitempty"`
	UserAgent         string                 `json:"userAgent,omitempty"`
	ReqPath           string                 `json:"requestPath,omitempty"`
	ReqHost           string                 `json:"requestHost,omitempty"`
	ReqClaims         map[string]interface{} `json:"requestClaims,omitempty"`
	ReqQuery          map[string]string      `json:"requestQuery,omitempty"`
	ReqHeader         map[string]string      `json:"requestHeader,omitempty"`
	RespHeader        map[string]string      `json:"responseHeader,omitempty"`
	AccessKey         string                 `json:"accessKey,omitempty"`
	ParentUser        string                 `json:"parentUser,omitempty"`
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
		toInterfaceMap("requestClaims", c.ReqClaims),
		toMap("requestQuery", c.ReqQuery),
		toMap("requestHeader", c.ReqHeader),
		toMap("responseHeader", c.RespHeader),
		toString("accessKey", c.AccessKey),
		toString("parentUser", c.ParentUser),
	}
	values = filterAndSort(values)
	return strings.Join(values, ",")
}
