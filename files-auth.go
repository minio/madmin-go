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
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package madmin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// FilesAuthKind names one kind of AIStor Files gateway authentication
// material. The kind is always declared, never inferred from the bytes.
type FilesAuthKind string

// The kinds of material the gateway accepts.
const (
	// FilesAuthKrb5Keytab is the Kerberos keytab. A merged fleet keytab holds
	// the nfs/<fqdn> principal of every node.
	FilesAuthKrb5Keytab FilesAuthKind = "krb5_keytab"

	// FilesAuthTLSCertChain is the PEM certificate chain for RPC-over-TLS.
	FilesAuthTLSCertChain FilesAuthKind = "tls_cert_chain"

	// FilesAuthTLSPrivateKey is the PEM private key of FilesAuthTLSCertChain.
	FilesAuthTLSPrivateKey FilesAuthKind = "tls_private_key"

	// FilesAuthTLSCABundle is the PEM bundle that client certificates are
	// verified against.
	FilesAuthTLSCABundle FilesAuthKind = "tls_ca_bundle"
)

// Defined reports whether k is one of the kinds named above.
func (k FilesAuthKind) Defined() bool {
	switch k {
	case FilesAuthKrb5Keytab, FilesAuthTLSCertChain, FilesAuthTLSPrivateKey, FilesAuthTLSCABundle:
		return true
	}
	return false
}

// FilesAuthMaterial is one piece of material sent by SetFilesAuth. It is the
// only type in this file that carries material, and it is only ever sent.
type FilesAuthMaterial struct {
	Kind  FilesAuthKind `json:"kind"`
	Bytes []byte        `json:"bytes"`
}

// FilesAuthSetRequest is the body of SetFilesAuth before it is encrypted.
type FilesAuthSetRequest struct {
	Assets []FilesAuthMaterial `json:"assets"`
}

// FilesAuthOutcome is what one node's last attempt to put the stored set in
// force on its gateway ended in.
type FilesAuthOutcome string

// The outcomes a node reports.
const (
	// FilesAuthPending means the node holds a stored set it has not sent yet.
	FilesAuthPending FilesAuthOutcome = "pending"

	// FilesAuthApplied means the gateway serves every asset of the set.
	FilesAuthApplied FilesAuthOutcome = "applied"

	// FilesAuthNothingStored means the cluster holds no material.
	FilesAuthNothingStored FilesAuthOutcome = "nothing-stored"

	// FilesAuthNoDaemon means no gateway daemon is reachable on the node. On a
	// node that serves no share this is the expected state.
	FilesAuthNoDaemon FilesAuthOutcome = "no-daemon"

	// FilesAuthRefused means the admin socket would not admit the server
	// process. It is always a deployment fault.
	FilesAuthRefused FilesAuthOutcome = "refused"

	// FilesAuthUnsupported means the gateway is too old for this operation.
	FilesAuthUnsupported FilesAuthOutcome = "unsupported"

	// FilesAuthRejected means the gateway did not accept the material, or a
	// reload refused it. The material in force before is still in force.
	FilesAuthRejected FilesAuthOutcome = "rejected"

	// FilesAuthReloading means the gateway was running a reload and did not
	// take the material. The node sends it again.
	FilesAuthReloading FilesAuthOutcome = "reloading"

	// FilesAuthRestartRequired means the material would turn TLS on or off,
	// which only a gateway restart does.
	FilesAuthRestartRequired FilesAuthOutcome = "restart-required"

	// FilesAuthSubsystemOff means the gateway subsystem that reads the
	// material is not enabled. The material waits in its slots.
	FilesAuthSubsystemOff FilesAuthOutcome = "off"

	// FilesAuthNotReached means the gateway's reload stopped before it reached
	// the subsystem that reads the material. The material waits in its slots.
	FilesAuthNotReached FilesAuthOutcome = "not-reached"

	// FilesAuthUnconfirmed means the gateway took the material but does not
	// report what is in force.
	FilesAuthUnconfirmed FilesAuthOutcome = "unconfirmed"

	// FilesAuthFailed means the attempt failed for any other reason.
	FilesAuthFailed FilesAuthOutcome = "failed"
)

// FilesAuthAsset describes one piece of material by digest. It never carries
// the material.
type FilesAuthAsset struct {
	Kind FilesAuthKind `json:"kind"`

	// SHA256 is the lowercase hex SHA-256 of the material. For a gateway slot
	// it describes what the slot holds, and it is empty for an empty slot.
	SHA256 string `json:"sha256"`

	// InForceSHA256 is the digest of the material the gateway serves. It is
	// set only for a gateway slot.
	InForceSHA256 string `json:"inForceSha256,omitempty"`

	// Bytes is the length of the material, for an asset of the stored set.
	Bytes int64 `json:"bytes,omitempty"`

	// Path is where the gateway reads the material, for a gateway slot.
	Path string `json:"path,omitempty"`

	// Updated is when the asset last changed in the stored set.
	Updated time.Time `json:"updated,omitzero"`
}

// FilesAuthSubsystemOutcome is a gateway subsystem's outcome for one reload, in
// the gateway's own words (miniohq/files docs/ADMIN-SOCKET.md). A node reports
// its own FilesAuthOutcome beside it, and the two vocabularies differ:
//
//   - FilesAuthSubsystemRefused (the gateway refused the material) is reported
//     at node level as FilesAuthRejected. FilesAuthRefused at node level means
//     the admin socket would not admit the server process.
//   - FilesAuthSubsystemRestartRequired is FilesAuthRestartRequired.
//   - FilesAuthSubsystemNotEnabled ("off") is FilesAuthSubsystemOff.
//   - FilesAuthSubsystemNotReached is FilesAuthNotReached.
type FilesAuthSubsystemOutcome string

// The outcomes a gateway subsystem reports.
const (
	// FilesAuthSubsystemApplied means the subsystem serves the material.
	FilesAuthSubsystemApplied FilesAuthSubsystemOutcome = "applied"

	// FilesAuthSubsystemRefused means the subsystem did not accept the
	// material, and the material in force before is still in force.
	FilesAuthSubsystemRefused FilesAuthSubsystemOutcome = "refused"

	// FilesAuthSubsystemRestartRequired means the material would turn the
	// subsystem on or off, which only a gateway restart does.
	FilesAuthSubsystemRestartRequired FilesAuthSubsystemOutcome = "restart_required"

	// FilesAuthSubsystemNotEnabled means the subsystem is not enabled.
	FilesAuthSubsystemNotEnabled FilesAuthSubsystemOutcome = "off"

	// FilesAuthSubsystemNotReached means the reload stopped before it reached
	// the subsystem.
	FilesAuthSubsystemNotReached FilesAuthSubsystemOutcome = "not_reached"
)

// FilesAuthSubsystem is one gateway subsystem's outcome for the latest reload
// that reached it.
type FilesAuthSubsystem struct {
	// Name is "tls" or "krb5".
	Name string `json:"name"`

	// Outcome is the gateway's own word for what the reload did.
	Outcome FilesAuthSubsystemOutcome `json:"outcome"`

	// Detail says why, for any outcome other than "applied".
	Detail string `json:"detail,omitempty"`

	// Seq is the reload that reported the outcome. 0 is the gateway's startup.
	Seq uint64 `json:"seq"`
}

// FilesAuthGateway is what one gateway reports holding.
type FilesAuthGateway struct {
	// BootID is new each time the gateway starts.
	BootID string `json:"bootId"`

	// Backing is "memfd" or "dir", and empty when the gateway has no store.
	Backing string `json:"backing,omitempty"`

	// ReloadResults reports whether the gateway reports what is in force.
	ReloadResults bool `json:"reloadResults"`

	// ReloadSeq, ReloadRunning and ReloadResult describe the latest reload.
	ReloadSeq     uint64 `json:"reloadSeq"`
	ReloadRunning bool   `json:"reloadRunning,omitempty"`
	ReloadResult  string `json:"reloadResult,omitempty"`

	// Subsystems holds the latest outcome of each subsystem that has one.
	Subsystems []FilesAuthSubsystem `json:"subsystems,omitempty"`

	// Assets holds one entry per slot, with the digest in the slot and the
	// digest in force.
	Assets []FilesAuthAsset `json:"assets,omitempty"`

	// At is when the gateway was read.
	At time.Time `json:"at,omitzero"`
}

// FilesAuthNodeStatus is one node's account of the material.
type FilesAuthNodeStatus struct {
	// Node is the host this answer came from, in host:port form.
	Node string `json:"node,omitempty"`

	// Outcome is how the node's last attempt ended.
	Outcome FilesAuthOutcome `json:"outcome"`

	// Detail explains any outcome other than FilesAuthApplied.
	Detail string `json:"detail,omitempty"`

	// SocketPath is the admin socket that was dialed.
	SocketPath string `json:"socketPath,omitempty"`

	// SetDigest names the stored set the attempt was for.
	SetDigest string `json:"setDigest,omitempty"`

	// At is when the outcome was recorded.
	At time.Time `json:"at,omitzero"`

	// Gateway is what the gateway reported when this call read it. It is nil
	// when the gateway could not be read, and GatewayError then says why.
	Gateway      *FilesAuthGateway `json:"gateway,omitempty"`
	GatewayError string            `json:"gatewayError,omitempty"`

	// Converged reports whether the gateway serves every asset of the stored
	// set.
	Converged bool `json:"converged"`
}

// FilesAuthResponse is the reply of SetFilesAuth and FilesAuthInfo. No member
// carries material.
//
// Count and Total follow the v4 query convention: Count is what Results holds
// and Total is every node the call covered.
type FilesAuthResponse struct {
	// SetDigest names the stored set, and is empty when nothing is stored.
	SetDigest string `json:"setDigest,omitempty"`

	// Assets describes the stored set by digest.
	Assets []FilesAuthAsset `json:"assets"`

	// Results holds one entry per node that answered, including the node that
	// served the request.
	Results []FilesAuthNodeStatus `json:"results"`

	// Count is the number of entries in Results.
	Count int `json:"count"`

	// Total is the number of nodes the call covered.
	Total int `json:"total"`

	// Unreachable names the nodes that reported no state.
	Unreachable []FilesUnreachableNode `json:"unreachable,omitempty"`

	// PeersNotQueried explains why no peer was asked, and is empty when they
	// were.
	PeersNotQueried string `json:"peersNotQueried,omitempty"`
}

// SetFilesAuth stores AIStor Files gateway authentication material and puts it
// in force on every node's gateway. A kind the call names replaces the stored
// material of that kind, and every other stored kind stays.
//
// The body is encrypted with the caller's secret key. The server refuses the
// call when the cluster has no KMS, and checks the merged set before it stores
// it: the private key must belong to the certificate chain, every certificate
// must parse, and the keytab must be an MIT keytab.
//
// The reply describes the stored set by digest and reports every node's
// outcome. A node the cluster cannot reach does not fail the call.
func (adm *AdminClient) SetFilesAuth(ctx context.Context, assets []FilesAuthMaterial) (FilesAuthResponse, error) {
	if len(assets) == 0 {
		return FilesAuthResponse{}, errors.New("at least one asset is required")
	}
	seen := make(map[FilesAuthKind]bool, len(assets))
	for _, a := range assets {
		switch {
		case !a.Kind.Defined():
			return FilesAuthResponse{}, fmt.Errorf("%q is not a files authentication kind", a.Kind)
		case seen[a.Kind]:
			return FilesAuthResponse{}, fmt.Errorf("%s is named twice", a.Kind)
		case len(a.Bytes) == 0:
			return FilesAuthResponse{}, fmt.Errorf("%s is empty", a.Kind)
		}
		seen[a.Kind] = true
	}

	// Look the credential up once: a failed lookup returns its error instead of
	// encrypting under an empty key, and every attempt signs with the same
	// credential that encrypted the body.
	creds, err := adm.credsProvider.GetWithContext(adm.CredContext())
	if err != nil {
		return FilesAuthResponse{}, err
	}
	plain := marshalFilesAuthSetRequest(assets)
	body, err := EncryptData(creds.SecretAccessKey, plain)
	clear(plain)
	if err != nil {
		return FilesAuthResponse{}, err
	}

	resp, err := adm.executeMethod(ctx,
		http.MethodPut,
		requestData{
			relPath: filesAPIPrefix + "/auth",
			content: body,
			creds:   &creds,
		})
	defer closeResponse(resp)
	if err != nil {
		return FilesAuthResponse{}, err
	}
	return decodeFilesAuthResponse(resp)
}

// FilesAuthInfo describes the stored AIStor Files gateway authentication
// material by digest, and reports what every node's gateway holds and serves.
// It never returns material and changes nothing on any node.
func (adm *AdminClient) FilesAuthInfo(ctx context.Context) (FilesAuthResponse, error) {
	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{relPath: filesAPIPrefix + "/auth"})
	defer closeResponse(resp)
	if err != nil {
		return FilesAuthResponse{}, err
	}
	return decodeFilesAuthResponse(resp)
}

func decodeFilesAuthResponse(resp *http.Response) (FilesAuthResponse, error) {
	if resp.StatusCode != http.StatusOK {
		return FilesAuthResponse{}, httpRespToErrorResponse(resp)
	}
	var info FilesAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return FilesAuthResponse{}, err
	}
	return info, nil
}

// marshalFilesAuthSetRequest returns assets as the JSON of a
// FilesAuthSetRequest. It writes into one buffer of its own, so the caller can
// clear the only plaintext copy; encoding/json would leave a second copy in a
// pooled buffer. The caller has checked every kind with Defined, so no kind
// needs escaping.
func marshalFilesAuthSetRequest(assets []FilesAuthMaterial) []byte {
	const head, tail = `{"assets":[`, `]}`
	n := len(head) + len(tail)
	for _, a := range assets {
		n += len(`{"kind":"","bytes":""},`) + len(a.Kind) + base64.StdEncoding.EncodedLen(len(a.Bytes))
	}
	b := make([]byte, 0, n)
	b = append(b, head...)
	for i, a := range assets {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, `{"kind":"`...)
		b = append(b, a.Kind...)
		b = append(b, `","bytes":"`...)
		b = base64.StdEncoding.AppendEncode(b, a.Bytes)
		b = append(b, `"}`...)
	}
	return append(b, tail...)
}
