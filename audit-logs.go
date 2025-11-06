//
// Copyright (c) 2015-2025 MinIO, Inc.
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
	"iter"
	"net/http"
	"time"

	"github.com/minio/madmin-go/v4/log"
	"github.com/tinylib/msgp/msgp"
)

// AuditLogOpts represents the options for the audit logs
type AuditLogOpts struct {
	Node       string            `json:"node,omitempty"`
	API        string            `json:"api,omitempty"`
	Bucket     string            `json:"bucket,omitempty"`
	Interval   time.Duration     `json:"interval,omitempty"`
	Category   log.AuditCategory `json:"category,omitempty"`
	MaxPerNode int               `json:"maxPerNode,omitempty"`
}

// GetAuditLogs fetches the persisted audit logs from MinIO
func (adm AdminClient) GetAuditLogs(ctx context.Context, opts AuditLogOpts) iter.Seq2[log.Audit, error] {
	return func(yield func(log.Audit, error) bool) {
		auditOpts, err := json.Marshal(opts)
		if err != nil {
			yield(log.Audit{}, err)
			return
		}
		reqData := requestData{
			relPath: adminAPIPrefix + "/logs/audit",
			content: auditOpts,
		}
		resp, err := adm.executeMethod(ctx, http.MethodPost, reqData)
		if err != nil {
			yield(log.Audit{}, err)
			return
		}
		if resp.StatusCode != http.StatusOK {
			yield(log.Audit{}, httpRespToErrorResponse(resp))
			return
		}
		dec := msgp.NewReader(resp.Body)
		for {
			var info log.Audit
			if err = info.DecodeMsg(dec); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				continue
			}
			select {
			case <-ctx.Done():
				return
			default:
				yield(info, nil)
			}
		}
	}
}
