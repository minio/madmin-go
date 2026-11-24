//
// Copyright (c) 2015-2026 MinIO, Inc.
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
	"time"
)

// Data object types a share can hold.
const (
	SharingObjectTable  = "TABLE"
	SharingObjectVolume = "VOLUME"
)

// Actions of a SharingShareUpdate.
const (
	SharingUpdateAdd    = "ADD"
	SharingUpdateRemove = "REMOVE"
)

// SharingPrivilegeSelect is the only privilege a recipient can be granted.
const SharingPrivilegeSelect = "SELECT"

// SharingAuthTypeToken is the only recipient authentication type.
const SharingAuthTypeToken = "TOKEN"

// SharingListOptions selects a page of a provider or recipient listing.
// A zero MaxResults lets the server pick the page size.
type SharingListOptions struct {
	MaxResults int
	PageToken  string
}

func (o SharingListOptions) values() url.Values {
	v := url.Values{}
	if o.MaxResults > 0 {
		v.Set("maxResults", strconv.Itoa(o.MaxResults))
	}
	if o.PageToken != "" {
		v.Set("pageToken", o.PageToken)
	}
	return v
}

// SharingDataObject is a table or volume in a share. Name is the source:
// "bucket/path" for a Delta table or a volume, or "warehouse.namespace.table"
// for an AIStor table. SharedAs is the "schema.name" recipients see.
type SharingDataObject struct {
	Name     string `json:"name,omitempty"`
	Type     string `json:"type,omitempty"`
	SharedAs string `json:"sharedAs,omitempty"`
}

// SharingShare is a share as the provider API returns it.
type SharingShare struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	DisplayName string            `json:"displayName,omitempty"`
	Comment     string            `json:"comment,omitempty"`
	Properties  map[string]string `json:"properties,omitempty"`
	OwnerID     string            `json:"ownerId,omitempty"`

	// Objects is set only by GetSharingShare with includeSharedData.
	Objects []SharingDataObject `json:"objects,omitempty"`
}

// CreateSharingShareRequest creates a share through the provider API.
type CreateSharingShareRequest struct {
	Name        string            `json:"name"`
	DisplayName string            `json:"displayName,omitempty"`
	Comment     string            `json:"comment,omitempty"`
	Properties  map[string]string `json:"properties,omitempty"`
}

// SharingShareUpdate adds or removes one data object.
type SharingShareUpdate struct {
	Action     string            `json:"action"`
	DataObject SharingDataObject `json:"dataObject"`
}

// UpdateSharingShareRequest changes a share. Nil fields are left as they
// are. The server applies every REMOVE before any ADD, and applies none of
// the updates if one fails. OwnerID transfers the share and must be the
// request's only change.
type UpdateSharingShareRequest struct {
	DisplayName *string              `json:"displayName,omitempty"`
	Comment     *string              `json:"comment,omitempty"`
	Updates     []SharingShareUpdate `json:"updates,omitempty"`
	OwnerID     *string              `json:"ownerId,omitempty"`
}

// SharingSharesPage is a page of shares.
type SharingSharesPage struct {
	Items         []SharingShare `json:"items"`
	NextPageToken string         `json:"nextPageToken,omitempty"`
}

// SharingGrant is a recipient's privilege on a share.
type SharingGrant struct {
	ShareID       string    `json:"shareId"`
	ShareName     string    `json:"shareName"`
	RecipientID   string    `json:"recipientId"`
	RecipientName string    `json:"recipientName"`
	Privilege     string    `json:"privilege"`
	GrantedBy     string    `json:"grantedBy,omitempty"`
	GrantedAt     time.Time `json:"grantedAt"`
}

// SharingGrantsPage is a page of grants.
type SharingGrantsPage struct {
	Items         []SharingGrant `json:"items"`
	NextPageToken string         `json:"nextPageToken,omitempty"`
}

// SharingGrantChange adds or removes privileges of one recipient on a share.
type SharingGrantChange struct {
	RecipientName string   `json:"recipientName"`
	Add           []string `json:"add,omitempty"`
	Remove        []string `json:"remove,omitempty"`
}

// SharingRecipient is a recipient as the provider API returns it.
// ActivationURL is set only when the recipient is created.
type SharingRecipient struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Comment            string `json:"comment,omitempty"`
	AuthenticationType string `json:"authenticationType"`
	OwnerID            string `json:"ownerId,omitempty"`
	ActivationURL      string `json:"activationUrl,omitempty"`
}

// CreateSharingRecipientRequest creates a recipient. AuthenticationType
// defaults to SharingAuthTypeToken. A zero TokenExpirationDays lets the
// server pick the token lifetime.
type CreateSharingRecipientRequest struct {
	Name                string `json:"name"`
	Comment             string `json:"comment,omitempty"`
	AuthenticationType  string `json:"authenticationType"`
	TokenExpirationDays int64  `json:"tokenExpirationDays,omitempty"`
}

// UpdateSharingRecipientRequest changes a recipient. OwnerID transfers the
// recipient and must be the request's only change.
type UpdateSharingRecipientRequest struct {
	Comment *string `json:"comment,omitempty"`
	OwnerID *string `json:"ownerId,omitempty"`
}

// SharingRecipientsPage is a page of recipients.
type SharingRecipientsPage struct {
	Items         []SharingRecipient `json:"items"`
	NextPageToken string             `json:"nextPageToken,omitempty"`
}

// RotateSharingRecipientTokenRequest issues a recipient a new token.
// ExistingTokenExpireInSeconds is how long the current tokens keep working;
// zero stops them at once.
type RotateSharingRecipientTokenRequest struct {
	TokenExpirationDays          int64 `json:"tokenExpirationDays,omitempty"`
	ExistingTokenExpireInSeconds int64 `json:"existingTokenExpireInSeconds"`
}

// RotateSharingRecipientTokenResponse holds the activation link of the new
// token.
type RotateSharingRecipientTokenResponse struct {
	TokenID       string     `json:"tokenId"`
	RecipientName string     `json:"recipientName"`
	ActivationURL string     `json:"activationUrl"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
}

// sharingError is the error body of the Delta Sharing and provider APIs.
// Code covers an error answered in the admin API's shape.
type sharingError struct {
	ErrorCode string `json:"errorCode"`
	Code      string `json:"Code"`
	Message   string `json:"message"`
}

// sharingRespToError turns an error response of the Delta Sharing or provider
// API into an ErrorResponse, with the API's errorCode as Code.
func sharingRespToError(resp *http.Response) error {
	if resp == nil || resp.Body == nil {
		return ErrInvalidArgument("Response is empty. " + reportIssue)
	}
	defer resp.Body.Close()
	var body sharingError
	var xmlErr ErrorResponse
	if err := decodeErrorResponse(resp, &body, &xmlErr); err != nil {
		return err
	}
	if xmlErr.Code != "" {
		xmlErr.StatusCode = resp.StatusCode
		return xmlErr
	}
	errResp := ErrorResponse{Code: body.ErrorCode, Message: body.Message, StatusCode: resp.StatusCode}
	if errResp.Code == "" {
		errResp.Code = body.Code
	}
	if errResp.Code == "" && errResp.Message == "" {
		errResp.Code = resp.Status
		errResp.Message = resp.Status
	}
	return errResp
}

// providerCall sends a signed request to the provider API and decodes a
// response with one of the want statuses into out, when out is not nil.
func (adm *AdminClient) providerCall(ctx context.Context, method, relPath string, query url.Values, in, out any, want ...int) error {
	reqData := requestData{
		relPath:        "/provider" + relPath,
		queryValues:    query,
		isDeltaSharing: true,
	}
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return err
		}
		reqData.content = data
		reqData.customHeaders = http.Header{"Content-Type": []string{"application/json"}}
	}
	resp, err := adm.executeMethod(ctx, method, reqData)
	defer closeResponse(resp)
	if err != nil {
		return err
	}
	ok := false
	for _, status := range want {
		ok = ok || resp.StatusCode == status
	}
	if !ok {
		return sharingRespToError(resp)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// CreateSharingShare creates a share through the provider API.
func (adm *AdminClient) CreateSharingShare(ctx context.Context, req CreateSharingShareRequest) (*SharingShare, error) {
	var share SharingShare
	if err := adm.providerCall(ctx, http.MethodPost, "/shares", nil, req, &share, http.StatusCreated, http.StatusOK); err != nil {
		return nil, err
	}
	return &share, nil
}

// ListSharingShares lists a page of shares.
func (adm *AdminClient) ListSharingShares(ctx context.Context, opts SharingListOptions) (*SharingSharesPage, error) {
	var page SharingSharesPage
	if err := adm.providerCall(ctx, http.MethodGet, "/shares", opts.values(), nil, &page, http.StatusOK); err != nil {
		return nil, err
	}
	return &page, nil
}

// GetSharingShare returns a share. With includeSharedData it also returns
// the share's tables and volumes in Objects.
func (adm *AdminClient) GetSharingShare(ctx context.Context, share string, includeSharedData bool) (*SharingShare, error) {
	var query url.Values
	if includeSharedData {
		query = url.Values{"include_shared_data": []string{"true"}}
	}
	var out SharingShare
	if err := adm.providerCall(ctx, http.MethodGet, "/shares/"+url.PathEscape(share), query, nil, &out, http.StatusOK); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateSharingShare changes a share and adds or removes its tables and
// volumes. The returned share does not list Objects.
func (adm *AdminClient) UpdateSharingShare(ctx context.Context, share string, req UpdateSharingShareRequest) (*SharingShare, error) {
	var out SharingShare
	if err := adm.providerCall(ctx, http.MethodPatch, "/shares/"+url.PathEscape(share), nil, req, &out, http.StatusOK); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteSharingShare deletes a share and every grant on it.
func (adm *AdminClient) DeleteSharingShare(ctx context.Context, share string) error {
	return adm.providerCall(ctx, http.MethodDelete, "/shares/"+url.PathEscape(share), nil, nil, nil, http.StatusNoContent, http.StatusOK)
}

// ListSharingShareGrants lists a page of the recipients granted a share.
func (adm *AdminClient) ListSharingShareGrants(ctx context.Context, share string, opts SharingListOptions) (*SharingGrantsPage, error) {
	var page SharingGrantsPage
	if err := adm.providerCall(ctx, http.MethodGet, "/shares/"+url.PathEscape(share)+"/permissions", opts.values(), nil, &page, http.StatusOK); err != nil {
		return nil, err
	}
	return &page, nil
}

// UpdateSharingShareGrants grants or revokes recipients' access to a share.
// The server applies the changes in order, or none of them if one fails, and
// returns the first page of grants.
func (adm *AdminClient) UpdateSharingShareGrants(ctx context.Context, share string, changes []SharingGrantChange) (*SharingGrantsPage, error) {
	body := struct {
		Changes []SharingGrantChange `json:"changes"`
	}{Changes: changes}
	var page SharingGrantsPage
	if err := adm.providerCall(ctx, http.MethodPatch, "/shares/"+url.PathEscape(share)+"/permissions", nil, body, &page, http.StatusOK); err != nil {
		return nil, err
	}
	return &page, nil
}

// CreateSharingRecipient creates a recipient. The returned ActivationURL
// works once and gives the recipient its profile file.
func (adm *AdminClient) CreateSharingRecipient(ctx context.Context, req CreateSharingRecipientRequest) (*SharingRecipient, error) {
	if req.AuthenticationType == "" {
		req.AuthenticationType = SharingAuthTypeToken
	}
	var out SharingRecipient
	if err := adm.providerCall(ctx, http.MethodPost, "/recipients", nil, req, &out, http.StatusCreated, http.StatusOK); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListSharingRecipients lists a page of recipients.
func (adm *AdminClient) ListSharingRecipients(ctx context.Context, opts SharingListOptions) (*SharingRecipientsPage, error) {
	var page SharingRecipientsPage
	if err := adm.providerCall(ctx, http.MethodGet, "/recipients", opts.values(), nil, &page, http.StatusOK); err != nil {
		return nil, err
	}
	return &page, nil
}

// GetSharingRecipient returns a recipient.
func (adm *AdminClient) GetSharingRecipient(ctx context.Context, recipient string) (*SharingRecipient, error) {
	var out SharingRecipient
	if err := adm.providerCall(ctx, http.MethodGet, "/recipients/"+url.PathEscape(recipient), nil, nil, &out, http.StatusOK); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateSharingRecipient changes a recipient's comment, or transfers it.
func (adm *AdminClient) UpdateSharingRecipient(ctx context.Context, recipient string, req UpdateSharingRecipientRequest) (*SharingRecipient, error) {
	var out SharingRecipient
	if err := adm.providerCall(ctx, http.MethodPatch, "/recipients/"+url.PathEscape(recipient), nil, req, &out, http.StatusOK); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteSharingRecipient deletes a recipient. Its tokens, and credentials
// already vended to it, stop working.
func (adm *AdminClient) DeleteSharingRecipient(ctx context.Context, recipient string) error {
	return adm.providerCall(ctx, http.MethodDelete, "/recipients/"+url.PathEscape(recipient), nil, nil, nil, http.StatusNoContent, http.StatusOK)
}

// RotateSharingRecipientToken issues a recipient a new token and cancels any
// activation link not yet used.
func (adm *AdminClient) RotateSharingRecipientToken(ctx context.Context, recipient string, req RotateSharingRecipientTokenRequest) (*RotateSharingRecipientTokenResponse, error) {
	var out RotateSharingRecipientTokenResponse
	if err := adm.providerCall(ctx, http.MethodPost, "/recipients/"+url.PathEscape(recipient)+"/rotate-token", nil, req, &out, http.StatusCreated, http.StatusOK); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListSharingRecipientGrants lists a page of the shares a recipient is
// granted.
func (adm *AdminClient) ListSharingRecipientGrants(ctx context.Context, recipient string, opts SharingListOptions) (*SharingGrantsPage, error) {
	var page SharingGrantsPage
	if err := adm.providerCall(ctx, http.MethodGet, "/recipients/"+url.PathEscape(recipient)+"/share-permissions", opts.values(), nil, &page, http.StatusOK); err != nil {
		return nil, err
	}
	return &page, nil
}
