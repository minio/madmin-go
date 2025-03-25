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
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/minio/minio-go/v7/pkg/tags"
)

// AccountAccess contains information about
type AccountAccess struct {
	Read  bool `json:"read"`
	Write bool `json:"write"`
}

// BucketDetails provides information about features currently
// turned-on per bucket.
type BucketDetails struct {
	Versioning          bool         `json:"versioning"`
	VersioningSuspended bool         `json:"versioningSuspended"`
	Locking             bool         `json:"locking"`
	Replication         bool         `json:"replication"`
	Tagging             *tags.Tags   `json:"tags"`
	Quota               *BucketQuota `json:"quota"`
}

// BucketAccessInfo represents bucket usage of a bucket, and its relevant
// access type for an account
type BucketAccessInfo struct {
	Name                    string            `json:"name"`
	Size                    uint64            `json:"size"`
	Objects                 uint64            `json:"objects"`
	ObjectSizesHistogram    map[string]uint64 `json:"objectHistogram"`
	ObjectVersionsHistogram map[string]uint64 `json:"objectsVersionsHistogram"`
	Details                 *BucketDetails    `json:"details"`
	PrefixUsage             map[string]uint64 `json:"prefixUsage"`
	Created                 time.Time         `json:"created"`
	Access                  AccountAccess     `json:"access"`
}

// AccountInfo represents the account usage info of an
// account across buckets.
type AccountInfo struct {
	AccountName string
	Server      BackendInfo
	Policy      json.RawMessage // Use iam/policy.Parse to parse the result, to be done by the caller.
	Buckets     []BucketAccessInfo
}

// AccountOpts allows for configurable behavior with "prefix-usage"
type AccountOpts struct {
	PrefixUsage bool
}

// AccountInfo returns the usage info for the authenticating account.
func (adm *AdminClient) AccountInfo(ctx context.Context, opts AccountOpts) (AccountInfo, error) {
	q := make(url.Values)
	if opts.PrefixUsage {
		q.Set("prefix-usage", "true")
	}
	resp, err := adm.executeMethod(ctx, http.MethodGet,
		requestData{
			relPath:     adminAPIPrefix + "/accountinfo",
			queryValues: q,
		},
	)
	defer closeResponse(resp)
	if err != nil {
		return AccountInfo{}, err
	}

	// Check response http status code
	if resp.StatusCode != http.StatusOK {
		return AccountInfo{}, httpRespToErrorResponse(resp)
	}

	// Unmarshal the server's json response
	var accountInfo AccountInfo

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return AccountInfo{}, err
	}

	err = json.Unmarshal(respBytes, &accountInfo)
	if err != nil {
		return AccountInfo{}, err
	}

	return accountInfo, nil
}

// AccountStatus - account status.
type AccountStatus string

// Account status per user.
const (
	AccountEnabled  AccountStatus = "enabled"
	AccountDisabled AccountStatus = "disabled"
)

// UserAuthType indicates the type of authentication for the user.
type UserAuthType string

// Valid values for UserAuthType.
const (
	BuiltinUserAuthType UserAuthType = "builtin"
	LDAPUserAuthType                 = "ldap"
)

// UserAuthInfo contains info about how the user is authenticated.
type UserAuthInfo struct {
	Type UserAuthType `json:"type"`

	// Specifies the external server that authenticated the server (empty for
	// builtin IDP)
	AuthServer string `json:"authServer,omitempty"`

	// Specifies the user ID as present in the external auth server (e.g. in
	// OIDC could be the email of the user). For builtin, this would be the same
	// as the access key.
	AuthServerUserID string `json:"authServerUserID,omitempty"`
}

// UserInfo carries information about long term users.
type UserInfo struct {
	AuthInfo   *UserAuthInfo `json:"userAuthInfo,omitempty"`
	SecretKey  string        `json:"secretKey,omitempty"`
	PolicyName string        `json:"policyName,omitempty"`
	Status     AccountStatus `json:"status"`
	MemberOf   []string      `json:"memberOf,omitempty"`
	UpdatedAt  time.Time     `json:"updatedAt,omitempty"`
}

// RemoveUser - remove a user.
func (adm *AdminClient) RemoveUser(ctx context.Context, accessKey string) error {
	queryValues := url.Values{}
	queryValues.Set("accessKey", accessKey)

	reqData := requestData{
		relPath:     adminAPIPrefix + "/remove-user",
		queryValues: queryValues,
	}

	// Execute DELETE on /minio/admin/v3/remove-user to remove a user.
	resp, err := adm.executeMethod(ctx, http.MethodDelete, reqData)

	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return httpRespToErrorResponse(resp)
	}

	return nil
}

// ListUsers - list all users.
func (adm *AdminClient) ListUsers(ctx context.Context) (map[string]UserInfo, error) {
	reqData := requestData{
		relPath: adminAPIPrefix + "/list-users",
	}

	// Execute GET on /minio/admin/v3/list-users
	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)

	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	data, err := DecryptData(adm.getSecretKey(), resp.Body)
	if err != nil {
		return nil, err
	}

	users := make(map[string]UserInfo)
	if err = json.Unmarshal(data, &users); err != nil {
		return nil, err
	}

	return users, nil
}

// GetUserInfo - get info on a user
func (adm *AdminClient) GetUserInfo(ctx context.Context, name string) (u UserInfo, err error) {
	queryValues := url.Values{}
	queryValues.Set("accessKey", name)

	reqData := requestData{
		relPath:     adminAPIPrefix + "/user-info",
		queryValues: queryValues,
	}

	// Execute GET on /minio/admin/v3/user-info
	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)

	defer closeResponse(resp)
	if err != nil {
		return u, err
	}

	if resp.StatusCode != http.StatusOK {
		return u, httpRespToErrorResponse(resp)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return u, err
	}

	if err = json.Unmarshal(b, &u); err != nil {
		return u, err
	}

	return u, nil
}

// AddOrUpdateUserReq allows to update
//   - user details such as secret key
//   - account status.
//   - optionally a comma separated list of policies
//     to be applied for the user.
type AddOrUpdateUserReq struct {
	SecretKey string        `json:"secretKey,omitempty"`
	Policy    string        `json:"policy,omitempty"`
	Status    AccountStatus `json:"status"`
}

// SetUserReq - update user secret key, account status or policies.
func (adm *AdminClient) SetUserReq(ctx context.Context, accessKey string, req AddOrUpdateUserReq) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	econfigBytes, err := EncryptData(adm.getSecretKey(), data)
	if err != nil {
		return err
	}

	queryValues := url.Values{}
	queryValues.Set("accessKey", accessKey)

	reqData := requestData{
		relPath:     adminAPIPrefix + "/add-user",
		queryValues: queryValues,
		content:     econfigBytes,
	}

	// Execute PUT on /minio/admin/v3/add-user to set a user.
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

// SetUser - update user secret key or account status.
func (adm *AdminClient) SetUser(ctx context.Context, accessKey, secretKey string, status AccountStatus) error {
	return adm.SetUserReq(ctx, accessKey, AddOrUpdateUserReq{
		SecretKey: secretKey,
		Status:    status,
	})
}

// AddUser - adds a user.
func (adm *AdminClient) AddUser(ctx context.Context, accessKey, secretKey string) error {
	return adm.SetUser(ctx, accessKey, secretKey, AccountEnabled)
}

// SetUserStatus - adds a status for a user.
func (adm *AdminClient) SetUserStatus(ctx context.Context, accessKey string, status AccountStatus) error {
	queryValues := url.Values{}
	queryValues.Set("accessKey", accessKey)
	queryValues.Set("status", string(status))

	reqData := requestData{
		relPath:     adminAPIPrefix + "/set-user-status",
		queryValues: queryValues,
	}

	// Execute PUT on /minio/admin/v3/set-user-status to set status.
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

// AddServiceAccountReq is the request options of the add service account admin call
type AddServiceAccountReq struct {
	Policy     json.RawMessage `json:"policy,omitempty"` // Parsed value from iam/policy.Parse()
	TargetUser string          `json:"targetUser,omitempty"`
	AccessKey  string          `json:"accessKey,omitempty"`
	SecretKey  string          `json:"secretKey,omitempty"`

	// Name for this access key
	Name string `json:"name,omitempty"`
	// Description for this access key
	Description string `json:"description,omitempty"`
	// Time at which this access key expires
	Expiration *time.Time `json:"expiration,omitempty"`

	// Deprecated: use description instead
	Comment string `json:"comment,omitempty"`
}

var serviceAcctValidNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*`)

func validateSAName(name string) error {
	if name == "" {
		return nil
	}
	if len(name) > 32 {
		return errors.New("name must not be longer than 32 characters")
	}
	if !serviceAcctValidNameRegex.MatchString(name) {
		return errors.New("name must contain only ASCII letters, digits, underscores and hyphens and must start with a letter")
	}
	return nil
}

func validateSADescription(desc string) error {
	if desc == "" {
		return nil
	}
	if len(desc) > 256 {
		return errors.New("description must be at most 256 bytes long")
	}
	return nil
}

var timeSentinel = time.Unix(0, 0).UTC()

func validateSAExpiration(expiration *time.Time) error {
	// Zero value is valid, it means no expiration.
	if expiration == nil || expiration.UTC().IsZero() || expiration.UTC().Equal(timeSentinel) {
		return nil
	}
	if expiration.Before(time.Now()) {
		return errors.New("the expiration time should be in the future")
	}
	return nil
}

// Validate validates the request parameters.
func (r *AddServiceAccountReq) Validate() error {
	if err := validateSAName(r.Name); err != nil {
		return err
	}

	if err := validateSAExpiration(r.Expiration); err != nil {
		return err
	}
	return validateSADescription(r.Description)
}

// AddServiceAccountResp is the response body of the add service account admin call
type AddServiceAccountResp struct {
	Credentials Credentials `json:"credentials"`
}

// AddServiceAccount - creates a new service account belonging to the user sending
// the request while restricting the service account permission by the given policy document.
func (adm *AdminClient) AddServiceAccount(ctx context.Context, opts AddServiceAccountReq) (Credentials, error) {
	if err := opts.Validate(); err != nil {
		return Credentials{}, err
	}
	data, err := json.Marshal(opts)
	if err != nil {
		return Credentials{}, err
	}

	econfigBytes, err := EncryptData(adm.getSecretKey(), data)
	if err != nil {
		return Credentials{}, err
	}

	reqData := requestData{
		relPath: adminAPIPrefix + "/add-service-account",
		content: econfigBytes,
	}

	// Execute PUT on /minio/admin/v3/add-service-account to set a user.
	resp, err := adm.executeMethod(ctx, http.MethodPut, reqData)
	defer closeResponse(resp)
	if err != nil {
		return Credentials{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return Credentials{}, httpRespToErrorResponse(resp)
	}

	data, err = DecryptData(adm.getSecretKey(), resp.Body)
	if err != nil {
		return Credentials{}, err
	}

	var serviceAccountResp AddServiceAccountResp
	if err = json.Unmarshal(data, &serviceAccountResp); err != nil {
		return Credentials{}, err
	}
	return serviceAccountResp.Credentials, nil
}

// AddServiceAccountLDAP - AddServiceAccount with extra features, restricted to LDAP users.
func (adm *AdminClient) AddServiceAccountLDAP(ctx context.Context, opts AddServiceAccountReq) (Credentials, error) {
	if err := opts.Validate(); err != nil {
		return Credentials{}, err
	}
	data, err := json.Marshal(opts)
	if err != nil {
		return Credentials{}, err
	}

	econfigBytes, err := EncryptData(adm.getSecretKey(), data)
	if err != nil {
		return Credentials{}, err
	}

	reqData := requestData{
		relPath: adminAPIPrefix + "/idp/ldap/add-service-account",
		content: econfigBytes,
	}
	resp, err := adm.executeMethod(ctx, http.MethodPut, reqData)
	defer closeResponse(resp)
	if err != nil {
		return Credentials{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return Credentials{}, httpRespToErrorResponse(resp)
	}

	data, err = DecryptData(adm.getSecretKey(), resp.Body)
	if err != nil {
		return Credentials{}, err
	}

	var serviceAccountResp AddServiceAccountResp
	if err = json.Unmarshal(data, &serviceAccountResp); err != nil {
		return Credentials{}, err
	}
	return serviceAccountResp.Credentials, nil
}

// UpdateServiceAccountReq is the request options of the edit service account admin call
type UpdateServiceAccountReq struct {
	NewPolicy      json.RawMessage `json:"newPolicy,omitempty"` // Parsed policy from iam/policy.Parse
	NewSecretKey   string          `json:"newSecretKey,omitempty"`
	NewStatus      string          `json:"newStatus,omitempty"`
	NewName        string          `json:"newName,omitempty"`
	NewDescription string          `json:"newDescription,omitempty"`
	NewExpiration  *time.Time      `json:"newExpiration,omitempty"`
}

func (u *UpdateServiceAccountReq) Validate() error {
	if err := validateSAName(u.NewName); err != nil {
		return err
	}

	if err := validateSAExpiration(u.NewExpiration); err != nil {
		return err
	}
	return validateSADescription(u.NewDescription)
}

// UpdateServiceAccount - edit an existing service account
func (adm *AdminClient) UpdateServiceAccount(ctx context.Context, accessKey string, opts UpdateServiceAccountReq) error {
	if err := opts.Validate(); err != nil {
		return err
	}
	data, err := json.Marshal(opts)
	if err != nil {
		return err
	}

	econfigBytes, err := EncryptData(adm.getSecretKey(), data)
	if err != nil {
		return err
	}

	queryValues := url.Values{}
	queryValues.Set("accessKey", accessKey)

	reqData := requestData{
		relPath:     adminAPIPrefix + "/update-service-account",
		content:     econfigBytes,
		queryValues: queryValues,
	}

	// Execute POST on /minio/admin/v3/update-service-account to edit a service account
	resp, err := adm.executeMethod(ctx, http.MethodPost, reqData)
	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusNoContent {
		return httpRespToErrorResponse(resp)
	}

	return nil
}

type ServiceAccountInfo struct {
	ParentUser    string     `json:"parentUser"`
	AccountStatus string     `json:"accountStatus"`
	ImpliedPolicy bool       `json:"impliedPolicy"`
	AccessKey     string     `json:"accessKey"`
	Name          string     `json:"name,omitempty"`
	Description   string     `json:"description,omitempty"`
	Expiration    *time.Time `json:"expiration,omitempty"`
}

// ListServiceAccountsResp is the response body of the list service accounts call
type ListServiceAccountsResp struct {
	Accounts []ServiceAccountInfo `json:"accounts"`
}

// ListServiceAccounts - list service accounts belonging to the specified user
func (adm *AdminClient) ListServiceAccounts(ctx context.Context, user string) (ListServiceAccountsResp, error) {
	queryValues := url.Values{}
	queryValues.Set("user", user)

	reqData := requestData{
		relPath:     adminAPIPrefix + "/list-service-accounts",
		queryValues: queryValues,
	}

	// Execute GET on /minio/admin/v3/list-service-accounts
	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
	defer closeResponse(resp)
	if err != nil {
		return ListServiceAccountsResp{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return ListServiceAccountsResp{}, httpRespToErrorResponse(resp)
	}

	data, err := DecryptData(adm.getSecretKey(), resp.Body)
	if err != nil {
		return ListServiceAccountsResp{}, err
	}

	var listResp ListServiceAccountsResp
	if err = json.Unmarshal(data, &listResp); err != nil {
		return ListServiceAccountsResp{}, err
	}
	return listResp, nil
}

type ListAccessKeysResp struct {
	ServiceAccounts []ServiceAccountInfo `json:"serviceAccounts"`
	STSKeys         []ServiceAccountInfo `json:"stsKeys"`
}

const (
	AccessKeyListUsersOnly  = "users-only"
	AccessKeyListSTSOnly    = "sts-only"
	AccessKeyListSvcaccOnly = "svcacc-only"
	AccessKeyListAll        = "all"
)

// ListAccessKeysOpts - options for listing access keys
type ListAccessKeysOpts struct {
	ListType   string
	All        bool
	ConfigName string // For OpenID
}

// ListAccessKeysBulk - list access keys belonging to the given users or all users
func (adm *AdminClient) ListAccessKeysBulk(ctx context.Context, users []string, opts ListAccessKeysOpts) (map[string]ListAccessKeysResp, error) {
	if len(users) > 0 && opts.All {
		return nil, errors.New("either specify users or all, not both")
	}

	queryValues := url.Values{}
	queryValues.Set("listType", opts.ListType)
	queryValues["users"] = users
	if opts.All {
		queryValues.Set("all", "true")
	}

	reqData := requestData{
		relPath:     adminAPIPrefix + "/list-access-keys-bulk",
		queryValues: queryValues,
	}

	// Execute GET on /minio/admin/v3/list-access-keys-bulk
	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	data, err := DecryptData(adm.getSecretKey(), resp.Body)
	if err != nil {
		return nil, err
	}

	listResp := make(map[string]ListAccessKeysResp)
	if err = json.Unmarshal(data, &listResp); err != nil {
		return nil, err
	}
	return listResp, nil
}

type OpenIDUserAccessKeys struct {
	MinioAccessKey  string               `json:"minioAccessKey"`
	ID              string               `json:"ID"`
	ReadableName    string               `json:"readableName"`
	ServiceAccounts []ServiceAccountInfo `json:"serviceAccounts"`
	STSKeys         []ServiceAccountInfo `json:"stsKeys"`
}

type ListAccessKeysOpenIDResp struct {
	ConfigName string                 `json:"configName"`
	Users      []OpenIDUserAccessKeys `json:"users"`
}

// ListAccessKeysOpenIDBulk - list access keys belonging to the given users or all users
func (adm *AdminClient) ListAccessKeysOpenIDBulk(ctx context.Context, users []string, opts ListAccessKeysOpts) ([]ListAccessKeysOpenIDResp, error) {
	if len(users) > 0 && opts.All {
		return nil, errors.New("either specify users or all, not both")
	}

	queryValues := url.Values{}
	queryValues.Set("listType", opts.ListType)
	queryValues["users"] = users
	if opts.All {
		queryValues.Set("all", "true")
	}
	if opts.ConfigName != "" {
		queryValues.Set("configName", opts.ConfigName)
	}

	reqData := requestData{
		relPath:     adminAPIPrefix + "/idp/openid/list-access-keys-bulk",
		queryValues: queryValues,
	}

	// Execute GET on /minio/admin/v3/list-access-keys-bulk
	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	data, err := DecryptData(adm.getSecretKey(), resp.Body)
	if err != nil {
		return nil, err
	}

	var listResp []ListAccessKeysOpenIDResp
	if err = json.Unmarshal(data, &listResp); err != nil {
		return nil, err
	}
	return listResp, nil
}

// InfoServiceAccountResp is the response body of the info service account call
type InfoServiceAccountResp struct {
	ParentUser    string     `json:"parentUser"`
	AccountStatus string     `json:"accountStatus"`
	ImpliedPolicy bool       `json:"impliedPolicy"`
	Policy        string     `json:"policy"`
	Name          string     `json:"name,omitempty"`
	Description   string     `json:"description,omitempty"`
	Expiration    *time.Time `json:"expiration,omitempty"`
}

// InfoServiceAccount - returns the info of service account belonging to the specified user
func (adm *AdminClient) InfoServiceAccount(ctx context.Context, accessKey string) (InfoServiceAccountResp, error) {
	queryValues := url.Values{}
	queryValues.Set("accessKey", accessKey)

	reqData := requestData{
		relPath:     adminAPIPrefix + "/info-service-account",
		queryValues: queryValues,
	}

	// Execute GET on /minio/admin/v3/info-service-account
	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
	defer closeResponse(resp)
	if err != nil {
		return InfoServiceAccountResp{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return InfoServiceAccountResp{}, httpRespToErrorResponse(resp)
	}

	data, err := DecryptData(adm.getSecretKey(), resp.Body)
	if err != nil {
		return InfoServiceAccountResp{}, err
	}

	var infoResp InfoServiceAccountResp
	if err = json.Unmarshal(data, &infoResp); err != nil {
		return InfoServiceAccountResp{}, err
	}
	return infoResp, nil
}

// DeleteServiceAccount - delete a specified service account. The server will reject
// the request if the service account does not belong to the user initiating the request
func (adm *AdminClient) DeleteServiceAccount(ctx context.Context, serviceAccount string) error {
	queryValues := url.Values{}
	queryValues.Set("accessKey", serviceAccount)

	reqData := requestData{
		relPath:     adminAPIPrefix + "/delete-service-account",
		queryValues: queryValues,
	}

	// Execute DELETE on /minio/admin/v3/delete-service-account
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

// TemporaryAccountInfoResp is the response body of the info temporary call
type TemporaryAccountInfoResp InfoServiceAccountResp

// TemporaryAccountInfo - returns the info of a temporary account
func (adm *AdminClient) TemporaryAccountInfo(ctx context.Context, accessKey string) (TemporaryAccountInfoResp, error) {
	queryValues := url.Values{}
	queryValues.Set("accessKey", accessKey)

	reqData := requestData{
		relPath:     adminAPIPrefix + "/temporary-account-info",
		queryValues: queryValues,
	}

	// Execute GET on /minio/admin/v3/temporary-account-info
	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
	defer closeResponse(resp)
	if err != nil {
		return TemporaryAccountInfoResp{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return TemporaryAccountInfoResp{}, httpRespToErrorResponse(resp)
	}

	data, err := DecryptData(adm.getSecretKey(), resp.Body)
	if err != nil {
		return TemporaryAccountInfoResp{}, err
	}

	var infoResp TemporaryAccountInfoResp
	if err = json.Unmarshal(data, &infoResp); err != nil {
		return TemporaryAccountInfoResp{}, err
	}
	return infoResp, nil
}

// User provider types
const (
	BuiltinProvider     = "builtin"
	LDAPProvider        = "ldap"
	OpenIDProvider      = "openid"
	K8SProvider         = "k8s"
	CertificateProvider = "tls"
	CustomTokenProvider = "custom"
)

// RevokeTokensReq is the request options of the revoke tokens admin call.
// If User is empty, the requestor's tokens are revoked.
// If requestor is STS, leaving TokenRevokeType empty revokes requestor's type of tokens.
type RevokeTokensReq struct {
	User            string `json:"user"`
	TokenRevokeType string `json:"tokenRevokeType"`
	FullRevoke      bool   `json:"fullRevoke"`
}

func (r *RevokeTokensReq) Validate() error {
	if r.User != "" && r.TokenRevokeType == "" && !r.FullRevoke {
		return errors.New("one of TokenRevokeType or FullRevoke must be set when User is set")
	}
	if r.TokenRevokeType != "" && r.FullRevoke {
		return errors.New("only one of TokenRevokeType or FullRevoke must be set, not both")
	}
	return nil
}

func (adm *AdminClient) revokeTokens(ctx context.Context, opts RevokeTokensReq, provider string) error {
	queryValues := url.Values{}
	queryValues.Set("user", opts.User)
	queryValues.Set("tokenRevokeType", opts.TokenRevokeType)
	if opts.FullRevoke {
		queryValues.Set("fullRevoke", "true")
	}

	reqData := requestData{
		relPath:     adminAPIPrefix + "/revoke-tokens/" + provider,
		queryValues: queryValues,
	}

	// Execute POST on /minio/admin/v3/revoke-tokens/{provider}
	resp, err := adm.executeMethod(ctx, http.MethodPost, reqData)
	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusNoContent {
		return httpRespToErrorResponse(resp)
	}

	return nil
}

// RevokeTokens - revokes tokens for the specified builtin user, or
// for an external (LDAP, OpenID, etc.) user being sent by one of its STS credentials.
func (adm *AdminClient) RevokeTokens(ctx context.Context, opts RevokeTokensReq) error {
	return adm.revokeTokens(ctx, opts, BuiltinProvider)
}

// RevokeTokensLDAP - revokes tokens for the specified LDAP user.
func (adm *AdminClient) RevokeTokensLDAP(ctx context.Context, opts RevokeTokensReq) error {
	return adm.revokeTokens(ctx, opts, LDAPProvider)
}

type LDAPSpecificAccessKeyInfo struct {
	Username string `json:"username"`
}

type OpenIDSpecificAccessKeyInfo struct {
	ConfigName       string `json:"configName"`
	UserID           string `json:"userID"`
	UserIDClaim      string `json:"userIDClaim"`
	DisplayName      string `json:"displayName,omitempty"`
	DisplayNameClaim string `json:"displayNameClaim,omitempty"`
}

// InfoAccessKeyResp is the response body of the info access key call
type InfoAccessKeyResp struct {
	AccessKey string

	InfoServiceAccountResp

	UserType           string                      `json:"userType"`
	UserProvider       string                      `json:"userProvider"`
	LDAPSpecificInfo   LDAPSpecificAccessKeyInfo   `json:"ldapSpecificInfo,omitempty"`
	OpenIDSpecificInfo OpenIDSpecificAccessKeyInfo `json:"openIDSpecificInfo,omitempty"`
}

// InfoAccessKey - returns the info of an access key
func (adm *AdminClient) InfoAccessKey(ctx context.Context, accessKey string) (InfoAccessKeyResp, error) {
	queryValues := url.Values{}
	queryValues.Set("accessKey", accessKey)

	reqData := requestData{
		relPath:     adminAPIPrefix + "/info-access-key",
		queryValues: queryValues,
	}

	// Execute GET on /minio/admin/v3/info-access-key
	resp, err := adm.executeMethod(ctx, http.MethodGet, reqData)
	defer closeResponse(resp)
	if err != nil {
		return InfoAccessKeyResp{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return InfoAccessKeyResp{}, httpRespToErrorResponse(resp)
	}

	data, err := DecryptData(adm.getSecretKey(), resp.Body)
	if err != nil {
		return InfoAccessKeyResp{}, err
	}

	var infoResp InfoAccessKeyResp
	if err = json.Unmarshal(data, &infoResp); err != nil {
		return InfoAccessKeyResp{}, err
	}
	return infoResp, nil
}
