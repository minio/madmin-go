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
)

// CRAdd - argument type for the CR add command.
type CRAdd struct {
	Clusters []PeerCluster `json:"clusters"`
}

// PeerCluster - represents a cluster to be added to the set of replicated
// clusters.
type PeerCluster struct {
	Name      string `json:"name"`
	Endpoint  string `json:"endpoints"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
}

// Meaningful values for ReplicateAddStatus.Status
const (
	ReplicateAddStatusSuccess = "Requested sites were configured for replication successfully."
	ReplicateAddStatusPartial = "Some sites could not be configured for replication."
)

// ReplicateAddStatus - returns status of add request.
type ReplicateAddStatus struct {
	Success                 bool   `json:"success"`
	Status                  string `json:"status"`
	ErrDetail               string `json:"errorDetail,omitempty"`
	InitialSyncErrorMessage string `json:"initialSyncErrorMessage,omitempty"`
}

// ClusterReplicateAdd - sends the CR add API call.
func (adm *AdminClient) ClusterReplicateAdd(ctx context.Context, addArg CRAdd) (ReplicateAddStatus, error) {
	crAddBytes, err := json.Marshal(addArg)
	if err != nil {
		return ReplicateAddStatus{}, nil
	}
	eCRAddBytes, err := EncryptData(adm.getSecretKey(), crAddBytes)
	if err != nil {
		return ReplicateAddStatus{}, err
	}

	reqData := requestData{
		relPath: adminAPIPrefix + "/site-replication/add",
		content: eCRAddBytes,
	}

	resp, err := adm.executeMethod(ctx, http.MethodPut, reqData)
	defer closeResponse(resp)
	if err != nil {
		return ReplicateAddStatus{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return ReplicateAddStatus{}, httpRespToErrorResponse(resp)
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ReplicateAddStatus{}, err
	}

	var res ReplicateAddStatus
	if err = json.Unmarshal(b, &res); err != nil {
		return ReplicateAddStatus{}, err
	}

	return res, nil
}

// ClusterReplicateInfo - contains cluster replication information.
type ClusterReplicateInfo struct {
	Enabled                 bool       `json:"enabled"`
	Name                    string     `json:"name,omitempty"`
	Clusters                []PeerInfo `json:"clusters,omitempty"`
	ServiceAccountAccessKey string     `json:"serviceAccountAccessKey,omitempty"`
}

// ClusterReplicateInfo - returns cluster replication information.
func (adm *AdminClient) ClusterReplicateInfo(ctx context.Context) (info ClusterReplicateInfo, err error) {
	reqData := requestData{
		relPath: adminAPIPrefix + "/site-replication/info",
	}

	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
	defer closeResponse(resp)
	if err != nil {
		return info, err
	}

	if resp.StatusCode != http.StatusOK {
		return info, httpRespToErrorResponse(resp)
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return info, err
	}

	err = json.Unmarshal(b, &info)
	return info, err
}

// CRInternalJoinReq - arg body for CRInternalJoin
type CRInternalJoinReq struct {
	SvcAcctAccessKey string              `json:"svcAcctAccessKey"`
	SvcAcctSecretKey string              `json:"svcAcctSecretKey"`
	SvcAcctParent    string              `json:"svcAcctParent"`
	Peers            map[string]PeerInfo `json:"peers"`
}

// PeerInfo - contains some properties of a cluster peer.
type PeerInfo struct {
	Endpoint string `json:"endpoint"`
	Name     string `json:"name"`
	// Deployment ID is useful as it is immutable - though endpoint may
	// change.
	DeploymentID string `json:"deploymentID"`
}

// CRInternalJoin - used only by minio server to send CR join requests to peer
// servers.
func (adm *AdminClient) CRInternalJoin(ctx context.Context, r CRInternalJoinReq) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	encBuf, err := EncryptData(adm.getSecretKey(), b)
	if err != nil {
		return err
	}

	reqData := requestData{
		relPath: adminAPIPrefix + "/site-replication/peer/join",
		content: encBuf,
	}

	resp, err := adm.executeMethod(ctx, http.MethodPut, reqData)
	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}

	return nil
}

// BktOp represents the bucket operation being requested.
type BktOp string

// BktOp value constants.
const (
	// make bucket and enable versioning
	MakeWithVersioningBktOp BktOp = "make-with-versioning"
	// add replication configuration
	ConfigureReplBktOp BktOp = "configure-replication"
	// delete bucket (forceDelete = off)
	DeleteBucketBktOp BktOp = "delete-bucket"
	// delete bucket (forceDelete = on)
	ForceDeleteBucketBktOp BktOp = "force-delete-bucket"
)

// CRInternalBucketOps - tells peers to create bucket and setup replication.
func (adm *AdminClient) CRInternalBucketOps(ctx context.Context, bucket string, op BktOp, opts map[string]string) error {
	v := url.Values{}
	v.Add("bucket", bucket)
	v.Add("operation", string(op))

	// For make-bucket, bucket options may be sent via `opts`
	if op == MakeWithVersioningBktOp {
		for k, val := range opts {
			v.Add(k, val)
		}
	}
	reqData := requestData{
		queryValues: v,
		relPath:     adminAPIPrefix + "/site-replication/peer/bucket-ops",
	}

	resp, err := adm.executeMethod(ctx, http.MethodPut, reqData)
	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}

	return nil
}

// CRIAMItem.Type constants.
const (
	CRIAMItemPolicy        = "policy"
	CRIAMItemSvcAcc        = "service-account"
	CRIAMItemPolicyMapping = "policy-mapping"
)

// CRSvcAccCreate - create operation
type CRSvcAccCreate struct {
	Parent        string          `json:"parent"`
	AccessKey     string          `json:"accessKey"`
	SecretKey     string          `json:"secretKey"`
	Groups        []string        `json:"groups"`
	LDAPUser      string          `json:"ldapUser"`
	SessionPolicy json.RawMessage `json:"sessionPolicy"`
	Status        string          `json:"status"`
}

// CRSvcAccUpdate - update operation
type CRSvcAccUpdate struct {
	AccessKey     string          `json:"accessKey"`
	SecretKey     string          `json:"secretKey"`
	Status        string          `json:"status"`
	SessionPolicy json.RawMessage `json:"sessionPolicy"`
}

// CRSvcAccDelete - delete operation
type CRSvcAccDelete struct {
	AccessKey string `json:"accessKey"`
}

// CRSvcAccChange - sum-type to represent an svc account change.
type CRSvcAccChange struct {
	Create *CRSvcAccCreate `json:"crSvcAccCreate"`
	Update *CRSvcAccUpdate `json:"crSvcAccUpdate"`
	Delete *CRSvcAccDelete `json:"crSvcAccDelete"`
}

// CRPolicyMapping - represents mapping of a policy to a user or group.
type CRPolicyMapping struct {
	UserOrGroup string `json:"userOrGroup"`
	IsGroup     bool   `json:"isGroup"`
	Policy      string `json:"policy"`
}

// CRIAMItem - represents an IAM object that will be copied to a peer.
type CRIAMItem struct {
	Type string `json:"type"`

	// Name and Policy below are used when Type == CRIAMItemPolicy
	Name   string          `json:"name"`
	Policy json.RawMessage `json:"policy"`

	// Used when Type == CRIAMItemPolicyMapping
	PolicyMapping *CRPolicyMapping `json:"policyMapping"`

	// Used when Type == CRIAMItemSvcAcc
	SvcAccChange *CRSvcAccChange `json:"serviceAccountChange"`
}

// CRInternalReplicateIAMItem - copies an IAM object to a peer cluster.
func (adm *AdminClient) CRInternalReplicateIAMItem(ctx context.Context, item CRIAMItem) error {
	b, err := json.Marshal(item)
	if err != nil {
		return err
	}
	reqData := requestData{
		relPath: adminAPIPrefix + "/site-replication/peer/iam-item",
		content: b,
	}

	resp, err := adm.executeMethod(ctx, http.MethodPut, reqData)
	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}

	return nil
}

// CRBucketMeta.Type constants
const (
	CRBucketMetaTypePolicy           = "policy"
	CRBucketMetaTypeTags             = "tags"
	CRBucketMetaTypeObjectLockConfig = "object-lock-config"
	CRBucketMetaTypeSSEConfig        = "sse-config"
)

// CRBucketMeta - represents a bucket metadata change that will be copied to a
// peer.
type CRBucketMeta struct {
	Type   string          `json:"type"`
	Bucket string          `json:"bucket"`
	Policy json.RawMessage `json:"policy,omitempty"`

	// Since tags does not have a json representation, we use its xml byte
	// representation directly.
	Tags *string `json:"tags,omitempty"`

	// Since object lock does not have a json representation, we use its xml
	// byte representation.
	ObjectLockConfig *string `json:"objectLockConfig,omitempty"`

	// Since SSE config does not have a json representation, we use its xml
	// byte respresentation.
	SSEConfig *string `json:"sseConfig,omitempty"`
}

// CRInternalReplicateBucketMeta - copies a bucket metadata change to a peer
// cluster.
func (adm *AdminClient) CRInternalReplicateBucketMeta(ctx context.Context, item CRBucketMeta) error {
	b, err := json.Marshal(item)
	if err != nil {
		return err
	}
	reqData := requestData{
		relPath: adminAPIPrefix + "/site-replication/peer/bucket-meta",
		content: b,
	}

	resp, err := adm.executeMethod(ctx, http.MethodPut, reqData)
	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}

	return nil
}
