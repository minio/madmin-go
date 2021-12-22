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

	"github.com/minio/minio-go/v7/pkg/replication"
)

// PeerSite - represents a cluster/site to be added to the set of replicated
// sites.
type PeerSite struct {
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

// SiteReplicationAdd - sends the SR add API call.
func (adm *AdminClient) SiteReplicationAdd(ctx context.Context, sites []PeerSite) (ReplicateAddStatus, error) {
	sitesBytes, err := json.Marshal(sites)
	if err != nil {
		return ReplicateAddStatus{}, nil
	}
	encBytes, err := EncryptData(adm.getSecretKey(), sitesBytes)
	if err != nil {
		return ReplicateAddStatus{}, err
	}

	reqData := requestData{
		relPath: adminAPIPrefix + "/site-replication/add",
		content: encBytes,
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

// SiteReplicationInfo - contains cluster replication information.
type SiteReplicationInfo struct {
	Enabled                 bool       `json:"enabled"`
	Name                    string     `json:"name,omitempty"`
	Sites                   []PeerInfo `json:"sites,omitempty"`
	ServiceAccountAccessKey string     `json:"serviceAccountAccessKey,omitempty"`
}

// SiteReplicationInfo - returns cluster replication information.
func (adm *AdminClient) SiteReplicationInfo(ctx context.Context) (info SiteReplicationInfo, err error) {
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

// SRPeerJoinReq - arg body for SRPeerJoin
type SRPeerJoinReq struct {
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

// SRPeerJoin - used only by minio server to send SR join requests to peer
// servers.
func (adm *AdminClient) SRPeerJoin(ctx context.Context, r SRPeerJoinReq) error {
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

// SRPeerBucketOps - tells peers to create bucket and setup replication.
func (adm *AdminClient) SRPeerBucketOps(ctx context.Context, bucket string, op BktOp, opts map[string]string) error {
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

// SRIAMItem.Type constants.
const (
	SRIAMItemPolicy        = "policy"
	SRIAMItemSvcAcc        = "service-account"
	SRIAMItemSTSAcc        = "sts-account"
	SRIAMItemPolicyMapping = "policy-mapping"
)

// SRSvcAccCreate - create operation
type SRSvcAccCreate struct {
	Parent        string                 `json:"parent"`
	AccessKey     string                 `json:"accessKey"`
	SecretKey     string                 `json:"secretKey"`
	Groups        []string               `json:"groups"`
	Claims        map[string]interface{} `json:"claims"`
	SessionPolicy json.RawMessage        `json:"sessionPolicy"`
	Status        string                 `json:"status"`
}

// SRSvcAccUpdate - update operation
type SRSvcAccUpdate struct {
	AccessKey     string          `json:"accessKey"`
	SecretKey     string          `json:"secretKey"`
	Status        string          `json:"status"`
	SessionPolicy json.RawMessage `json:"sessionPolicy"`
}

// SRSvcAccDelete - delete operation
type SRSvcAccDelete struct {
	AccessKey string `json:"accessKey"`
}

// SRSvcAccChange - sum-type to represent an svc account change.
type SRSvcAccChange struct {
	Create *SRSvcAccCreate `json:"crSvcAccCreate"`
	Update *SRSvcAccUpdate `json:"crSvcAccUpdate"`
	Delete *SRSvcAccDelete `json:"crSvcAccDelete"`
}

// SRPolicyMapping - represents mapping of a policy to a user or group.
type SRPolicyMapping struct {
	UserOrGroup string `json:"userOrGroup"`
	IsGroup     bool   `json:"isGroup"`
	Policy      string `json:"policy"`
}

// SRSTSCredential - represents an STS credential to be replicated.
type SRSTSCredential struct {
	AccessKey    string `json:"accessKey"`
	SecretKey    string `json:"secretKey"`
	SessionToken string `json:"sessionToken"`
}

// SRIAMItem - represents an IAM object that will be copied to a peer.
type SRIAMItem struct {
	Type string `json:"type"`

	// Name and Policy below are used when Type == SRIAMItemPolicy
	Name   string          `json:"name"`
	Policy json.RawMessage `json:"policy"`

	// Used when Type == SRIAMItemPolicyMapping
	PolicyMapping *SRPolicyMapping `json:"policyMapping"`

	// Used when Type == SRIAMItemSvcAcc
	SvcAccChange *SRSvcAccChange `json:"serviceAccountChange"`

	// Used when Type = SRIAMItemSTSAcc
	STSCredential *SRSTSCredential `json:"stsCredential"`
}

// SRPeerReplicateIAMItem - copies an IAM object to a peer cluster.
func (adm *AdminClient) SRPeerReplicateIAMItem(ctx context.Context, item SRIAMItem) error {
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

// SRBucketMeta.Type constants
const (
	SRBucketMetaTypePolicy           = "policy"
	SRBucketMetaTypeTags             = "tags"
	SRBucketMetaTypeObjectLockConfig = "object-lock-config"
	SRBucketMetaTypeSSEConfig        = "sse-config"
)

// SRBucketMeta - represents a bucket metadata change that will be copied to a
// peer.
type SRBucketMeta struct {
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

// SRPeerReplicateBucketMeta - copies a bucket metadata change to a peer
// cluster.
func (adm *AdminClient) SRPeerReplicateBucketMeta(ctx context.Context, item SRBucketMeta) error {
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

// SRBucketInfo - returns all the bucket metadata available for bucket
type SRBucketInfo struct {
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
	// replication config in json representation
	ReplicationConfig *string `json:"replicationConfig,omitempty"`
}

// IDPSettings contains key IDentity Provider settings to validate that all
// peers have the same configuration.
type IDPSettings struct {
	IsLDAPEnabled          bool
	LDAPUserDNSearchBase   string
	LDAPUserDNSearchFilter string
	LDAPGroupSearchBase    string
	LDAPGroupSearchFilter  string
}

// SRPeerGetIDPSettings - fetches IDP settings from the server.
func (adm *AdminClient) SRPeerGetIDPSettings(ctx context.Context) (info IDPSettings, err error) {
	reqData := requestData{
		relPath: adminAPIPrefix + "/site-replication/peer/idp-settings",
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

// SRInfo gets replication metadata for a site
type SRInfo struct {
	Enabled        bool
	Name           string
	DeploymentID   string
	Buckets        map[string]SRBucketInfo       // map of bucket metadata info
	Policies       map[string]json.RawMessage    //  map of IAM policy name to content
	UserPolicies   map[string]SRPolicyMapping    // map of username -> user policy mapping
	GroupPolicies  map[string]SRPolicyMapping    // map of groupname -> group policy mapping
	ReplicationCfg map[string]replication.Config // map of bucket -> replication config
}

// SRMetaInfo - returns replication metadata info for a site.
func (adm *AdminClient) SRMetaInfo(ctx context.Context) (info SRInfo, err error) {
	reqData := requestData{
		relPath: adminAPIPrefix + "/site-replication/metainfo",
	}

	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
	defer closeResponse(resp)
	if err != nil {
		return info, err
	}

	if resp.StatusCode != http.StatusOK {
		return info, httpRespToErrorResponse(resp)
	}

	err = json.NewDecoder(resp.Body).Decode(&info)
	return info, err
}

// SRStatusInfo returns detailed status on site replication status
type SRStatusInfo struct {
	Enabled          bool
	MaxBuckets       int                                        // maximum buckets seen across sites
	MaxUsers         int                                        // maximum users seen across sites
	MaxGroups        int                                        // maximum groups seen across sites
	MaxPolicies      int                                        // maximum policies across sites
	Sites            map[string]PeerInfo                        // deployment->sitename
	StatsSummary     map[string]SRSiteSummary                   // map of deployment id -> site stat
	BucketMismatches map[string]map[string]SRBucketStatsSummary // map of bucket to slice of deployment IDs with stats
	PolicyMismatches map[string]map[string]SRPolicyStatsSummary // map of policy name to slice of deployment IDs with policy summary
	UserMismatches   map[string]map[string]SRUserStatsSummary   // map of user name to slice of deployment IDs with user mismatches
	GroupMismatches  map[string]map[string]SRGroupStatsSummary  // map of group name to slice of deployment IDs with group mismatches
}

// SRPolicyStatsSummary has status of policy replication misses
type SRPolicyStatsSummary struct {
	DeploymentID   string
	PolicyMismatch bool
	PolicyMissing  bool
}

// SRUserStatsSummary has status of user replication misses
type SRUserStatsSummary struct {
	DeploymentID   string
	PolicyMismatch bool
	UserMissing    bool
}

// SRGroupStatsSummary has status of group replication misses
type SRGroupStatsSummary struct {
	DeploymentID   string
	PolicyMismatch bool
	GroupMissing   bool
}

// SRBucketStatsSummary has status of bucket metadata replication misses
type SRBucketStatsSummary struct {
	DeploymentID           string
	HasBucket              bool
	TagMismatch            bool
	OLockConfigMismatch    bool
	PolicyMismatch         bool
	SSEConfigMismatch      bool
	HasReplicationCfg      bool
	ReplicationCfgMismatch bool
}

// SRSiteSummary holds the count of replicated items in site replication
type SRSiteSummary struct {
	ReplicatedBuckets        int // count of buckets replicated across sites
	ReplicatedTags           int // count of buckets with tags replicated across sites
	ReplicatedBucketPolicies int // count of policies replicated across sites
	ReplicatedIAMPolicies    int // count of IAM policies replicated across sites
	ReplicatedUsers          int // count of users replicated across sites
	ReplicatedGroups         int // count of groups replicated across sites
	ReplicatedLockConfig     int // count of object lock config replicated across sites
	ReplicatedSSEConfig      int
	TotalBucketsCount        int // total buckets on this site
	TotalTagsCount           int // total count of buckets with tags on this site
	TotalBucketPoliciesCount int // total count of buckets with bucket policies for this site
	TotalIAMPoliciesCount    int // total count of IAM policies for this site
	TotalLockConfigCount     int // total count of buckets with object lock config for this site
	TotalSSEConfigCount      int // total count of buckets with SSE config
	TotalUsersCount          int // total number of users seen on this site
	TotalGroupsCount         int // total number of groups seen on this site
}

// SRStatusInfo - returns site replication status
func (adm *AdminClient) SRStatusInfo(ctx context.Context) (info SRStatusInfo, err error) {
	reqData := requestData{
		relPath: adminAPIPrefix + "/site-replication/status",
	}

	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
	defer closeResponse(resp)
	if err != nil {
		return info, err
	}

	if resp.StatusCode != http.StatusOK {
		return info, httpRespToErrorResponse(resp)
	}

	err = json.NewDecoder(resp.Body).Decode(&info)
	return info, err
}

// ReplicateEditStatus - returns status of edit request.
type ReplicateEditStatus struct {
	Success   bool   `json:"success"`
	Status    string `json:"status"`
	ErrDetail string `json:"errorDetail,omitempty"`
}

// SiteReplicationEdit - sends the SR edit API call.
func (adm *AdminClient) SiteReplicationEdit(ctx context.Context, site PeerInfo) (ReplicateEditStatus, error) {
	sitesBytes, err := json.Marshal(site)
	if err != nil {
		return ReplicateEditStatus{}, nil
	}
	encBytes, err := EncryptData(adm.getSecretKey(), sitesBytes)
	if err != nil {
		return ReplicateEditStatus{}, err
	}

	reqData := requestData{
		relPath: adminAPIPrefix + "/site-replication/edit",
		content: encBytes,
	}

	resp, err := adm.executeMethod(ctx, http.MethodPut, reqData)
	defer closeResponse(resp)
	if err != nil {
		return ReplicateEditStatus{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return ReplicateEditStatus{}, httpRespToErrorResponse(resp)
	}

	var res ReplicateEditStatus
	err = json.NewDecoder(resp.Body).Decode(&res)
	return res, err
}

// SRPeerEdit - used only by minio server to update peer endpoint
// for a server already in the site replication setup
func (adm *AdminClient) SRPeerEdit(ctx context.Context, pi PeerInfo) error {

	b, err := json.Marshal(pi)
	if err != nil {
		return err
	}

	reqData := requestData{
		relPath: adminAPIPrefix + "/site-replication/peer/edit",
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
