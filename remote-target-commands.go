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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ARN is a struct to define arn.
type ARN struct {
	Type   ServiceType
	ID     string
	Region string
	Bucket string
}

// Empty returns true if arn struct is empty
func (a ARN) Empty() bool {
	return !a.Type.IsValid()
}

func (a ARN) String() string {
	return fmt.Sprintf("arn:minio:%s:%s:%s:%s", a.Type, a.Region, a.ID, a.Bucket)
}

// ParseARN return ARN struct from string in arn format.
func ParseARN(s string) (*ARN, error) {
	// ARN must be in the format of arn:minio:<Type>:<REGION>:<ID>:<remote-bucket>
	if !strings.HasPrefix(s, "arn:minio:") {
		return nil, fmt.Errorf("invalid ARN %s", s)
	}

	tokens := strings.Split(s, ":")
	if len(tokens) != 6 {
		return nil, fmt.Errorf("invalid ARN %s", s)
	}

	if tokens[4] == "" || tokens[5] == "" {
		return nil, fmt.Errorf("invalid ARN %s", s)
	}

	return &ARN{
		Type:   ServiceType(tokens[2]),
		Region: tokens[3],
		ID:     tokens[4],
		Bucket: tokens[5],
	}, nil
}

// ListRemoteTargets - gets target(s) for this bucket
func (adm *AdminClient) ListRemoteTargets(ctx context.Context, bucket, arnType string) (targets []BucketTarget, err error) {
	queryValues := url.Values{}
	queryValues.Set("bucket", bucket)
	queryValues.Set("type", arnType)

	reqData := requestData{
		relPath:     adminAPIPrefixV4 + "/list-remote-targets",
		queryValues: queryValues,
	}

	// Execute GET on /minio/admin/v4/list-remote-targets
	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)

	defer closeResponse(resp)
	if err != nil {
		return targets, err
	}

	if resp.StatusCode != http.StatusOK {
		return targets, httpRespToErrorResponse(resp)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return targets, err
	}
	if err = json.Unmarshal(b, &targets); err != nil {
		return targets, err
	}
	return targets, nil
}

// SetRemoteTarget sets up a remote target for this bucket
func (adm *AdminClient) SetRemoteTarget(ctx context.Context, bucket string, target *BucketTarget) (string, error) {
	data, err := json.Marshal(target)
	if err != nil {
		return "", err
	}
	encData, err := EncryptData(adm.getSecretKey(), data)
	if err != nil {
		return "", err
	}
	queryValues := url.Values{}
	queryValues.Set("bucket", bucket)

	reqData := requestData{
		relPath:     adminAPIPrefixV4 + "/set-remote-target",
		queryValues: queryValues,
		content:     encData,
	}

	// Execute PUT on /minio/admin/v4/set-remote-target to set a target for this bucket of specific arn type.
	resp, err := adm.executeMethod(ctx, http.MethodPut, reqData)

	defer closeResponse(resp)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", httpRespToErrorResponse(resp)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var arn string
	if err = json.Unmarshal(b, &arn); err != nil {
		return "", err
	}
	return arn, nil
}

// TargetUpdateType -  type of update on the remote target
type TargetUpdateType int

const (
	// CredentialsUpdateType update creds
	CredentialsUpdateType TargetUpdateType = 1 + iota
	// SyncUpdateType update synchronous replication setting
	SyncUpdateType
	// ProxyUpdateType update proxy setting
	ProxyUpdateType
	// BandwidthLimitUpdateType update bandwidth limit
	BandwidthLimitUpdateType
	// HealthCheckDurationUpdateType update health check duration
	HealthCheckDurationUpdateType
	// PathUpdateType update Path
	PathUpdateType
	// ResetUpdateType sets ResetBeforeDate and ResetID on a bucket target
	ResetUpdateType
	// EdgeUpdateType sets bucket target as a recipent of edge traffic
	EdgeUpdateType
	// EdgeExpiryUpdateType sets bucket target to sync before expiry
	EdgeExpiryUpdateType
)

// GetTargetUpdateOps returns a slice of update operations being
// performed with `mc admin bucket remote edit`
func GetTargetUpdateOps(values url.Values) []TargetUpdateType {
	var ops []TargetUpdateType
	if values.Get("update") != "true" {
		return ops
	}
	if values.Get("creds") == "true" {
		ops = append(ops, CredentialsUpdateType)
	}
	if values.Get("sync") == "true" {
		ops = append(ops, SyncUpdateType)
	}
	if values.Get("proxy") == "true" {
		ops = append(ops, ProxyUpdateType)
	}
	if values.Get("healthcheck") == "true" {
		ops = append(ops, HealthCheckDurationUpdateType)
	}
	if values.Get("bandwidth") == "true" {
		ops = append(ops, BandwidthLimitUpdateType)
	}
	if values.Get("path") == "true" {
		ops = append(ops, PathUpdateType)
	}
	if values.Get("edge") == "true" {
		ops = append(ops, EdgeUpdateType)
	}
	if values.Get("edgeSyncBeforeExpiry") == "true" {
		ops = append(ops, EdgeExpiryUpdateType)
	}
	return ops
}

// UpdateRemoteTarget updates credentials for a remote bucket target
func (adm *AdminClient) UpdateRemoteTarget(ctx context.Context, target *BucketTarget, ops ...TargetUpdateType) (string, error) {
	if target == nil {
		return "", fmt.Errorf("target cannot be nil")
	}
	data, err := json.Marshal(target)
	if err != nil {
		return "", err
	}
	encData, err := EncryptData(adm.getSecretKey(), data)
	if err != nil {
		return "", err
	}
	queryValues := url.Values{}
	queryValues.Set("bucket", target.SourceBucket)
	queryValues.Set("update", "true")

	for _, op := range ops {
		switch op {
		case CredentialsUpdateType:
			queryValues.Set("creds", "true")
		case SyncUpdateType:
			queryValues.Set("sync", "true")
		case ProxyUpdateType:
			queryValues.Set("proxy", "true")
		case BandwidthLimitUpdateType:
			queryValues.Set("bandwidth", "true")
		case HealthCheckDurationUpdateType:
			queryValues.Set("healthcheck", "true")
		case PathUpdateType:
			queryValues.Set("path", "true")
		case EdgeUpdateType:
			queryValues.Set("edge", "true")
		case EdgeExpiryUpdateType:
			queryValues.Set("edgeSyncBeforeExpiry", "true")
		}
	}

	reqData := requestData{
		relPath:     adminAPIPrefixV4 + "/set-remote-target",
		queryValues: queryValues,
		content:     encData,
	}

	// Execute PUT on /minio/admin/v4/set-remote-target to set a target for this bucket of specific arn type.
	resp, err := adm.executeMethod(ctx, http.MethodPut, reqData)

	defer closeResponse(resp)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", httpRespToErrorResponse(resp)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var arn string
	if err = json.Unmarshal(b, &arn); err != nil {
		return "", err
	}
	return arn, nil
}

// RemoveRemoteTarget removes a remote target associated with particular ARN for this bucket
func (adm *AdminClient) RemoveRemoteTarget(ctx context.Context, bucket, arn string) error {
	queryValues := url.Values{}
	queryValues.Set("bucket", bucket)
	queryValues.Set("arn", arn)

	reqData := requestData{
		relPath:     adminAPIPrefixV4 + "/remove-remote-target",
		queryValues: queryValues,
	}

	// Execute PUT on /minio/admin/v4/remove-remote-target to remove a target for this bucket
	// with specific ARN
	resp, err := adm.executeMethod(ctx, http.MethodDelete, reqData)
	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusNoContent {
		return httpRespToErrorResponse(resp)
	}
	return nil
}
