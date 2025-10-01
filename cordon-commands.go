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
	"fmt"
	"net/http"
	"net/url"
	"slices"
)

//msgp:ignore cordonNodeOpts
//msgp:clearomitted
//msgp:tag json
//msgp:timezone utc
//go:generate msgp -file $GOFILE

const (
	CordonAction   = "cordon"
	StateCordoned  = "cordoned"
	UncordonAction = "uncordon"
	DrainAction    = "drain"
	StateDraining  = "draining"
)

// CordonActionValidate validates if the given action is one of the known, allowed actions.
func CordonActionValidate(action string) error {
	validActions := []string{CordonAction, UncordonAction, DrainAction}
	if !slices.Contains(validActions, action) {
		return fmt.Errorf("invalid action '%s', must be one of '%s'", action, validActions)
	}
	return nil
}

// CordonNodeResult represents the result of a cordon, uncordon or drain operation on a node.
// Node will be the targeted node, in the format <host>:<port>.
// Errors will contain any errors that occurred communicating the Cordon to peers, each item in the slice
// will have the node name and the error message from that node, e.g. "node1:9000: <error message>".
type CordonNodeResult struct {
	Node   string   `json:"node"`
	Errors []string `json:"errors,omitempty"`
}

// cordonNodeOpts is used internally to pass options to the cordonAction method.
type cordonNodeOpts struct {
	Action string
	Node   string
}

// Cordon will cordon a node, preventing it from receiving new requests and putting it in a maintenance mode.
// The node name is given in the format <host>:<port>, for example: "node1:9000".
func (adm *AdminClient) Cordon(ctx context.Context, node string) (CordonNodeResult, error) {
	return adm.cordonAction(ctx, cordonNodeOpts{
		Action: CordonAction,
		Node:   node,
	})
}

// Uncordon will uncordon a node, allowing it to receive requests again.
// The node name is given in the format <host>:<port>, for example: "node1:9000".
func (adm *AdminClient) Uncordon(ctx context.Context, node string) (CordonNodeResult, error) {
	return adm.cordonAction(ctx, cordonNodeOpts{
		Action: UncordonAction,
		Node:   node,
	})
}

// Drain will drain a node, preventing it from receiving new requests and allowing existing requests to finish.
// The node name is given in the format <host>:<port>, for example: "node1:9000".
// The node will Cordon itself once the drain is completel and there are 0 remaining connections.
func (adm *AdminClient) Drain(ctx context.Context, node string) (CordonNodeResult, error) {
	return adm.cordonAction(ctx, cordonNodeOpts{
		Action: DrainAction,
		Node:   node,
	})
}

// cordonAction can cordon, drain or uncordon a node
func (adm *AdminClient) cordonAction(ctx context.Context, opts cordonNodeOpts) (CordonNodeResult, error) {
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
