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
	"net/http"
	"net/url"
	"slices"
)

//msgp:ignore CordonNodeOpts
//msgp:clearomitted
//msgp:tag json
//msgp:timezone utc
//go:generate msgp -file $GOFILE

const (
	CordonAction    = "cordon"
	UncordonAction  = "uncordon"
	StateCordoned   = "cordoned"
	StateUncordoned = "uncordoned"
)

func CordonActionValidate(action string) error {
	if !slices.Contains([]string{CordonAction, UncordonAction}, action) {
		return fmt.Errorf("invalid action '%s', must be either '%s' or '%s'", action, CordonAction, UncordonAction)
	}
	return nil
}

type CordonNodeOpts struct {
	Action string
	Node   string
}

type CordonNodeResult struct {
	Node   string   `json:"node"`
	Status string   `json:"status"`
	Errors []string `json:"errors,omitempty"`
}

// Cordon will cordon a node, preventing it from receiving new requests and putting it in a maintenance mode.
func (adm *AdminClient) Cordon(ctx context.Context, node string) (CordonNodeResult, error) {
	return adm.cordonAction(ctx, CordonNodeOpts{
		Action: CordonAction,
		Node:   node,
	})
}

// Uncordon will uncordon a node, allowing it to receive requests again.
func (adm *AdminClient) Uncordon(ctx context.Context, node string) (CordonNodeResult, error) {
	return adm.cordonAction(ctx, CordonNodeOpts{
		Action: UncordonAction,
		Node:   node,
	})
}

// cordonAction can cordon or uncordon a node
func (adm *AdminClient) cordonAction(ctx context.Context, opts CordonNodeOpts) (CordonNodeResult, error) {
	if err := CordonActionValidate(opts.Action); err != nil {
		return CordonNodeResult{}, err
	}
	if opts.Node == "" {
		return CordonNodeResult{}, ErrInvalidArgument("node must be specified")
	}
	queryValues := url.Values{}
	queryValues.Set("action", opts.Action)
	queryValues.Set("node", opts.Node)

	resp, err := adm.executeMethod(ctx,
		http.MethodPost, requestData{
			relPath:     adminAPIPrefix + "/cordon",
			queryValues: queryValues,
		},
	)
	defer closeResponse(resp)
	if err != nil {
		return CordonNodeResult{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return CordonNodeResult{}, httpRespToErrorResponse(resp)
	}

	result := CordonNodeResult{}
	dec := json.NewDecoder(resp.Body)
	if err = dec.Decode(&result); err != nil {
		return CordonNodeResult{}, err
	}
	return result, nil
}
