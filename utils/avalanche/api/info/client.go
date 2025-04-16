// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package info

import (
	"context"

	"github.com/landslidenetwork/slide-sdk/utils/avalanche/json"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/rpc"
)

var _ Client = (*client)(nil)

// Client interface for an Info API Client.
// See also AwaitBootstrapped.
type Client interface {
	GetNetworkID(context.Context, ...rpc.Option) (uint32, error)
}

// Client implementation for an Info API Client
type client struct {
	requester rpc.EndpointRequester
}

// NewClient returns a new Info API Client
func NewClient(uri string) Client {
	return &client{requester: rpc.NewEndpointRequester(
		uri + "/ext/info",
	)}
}

// GetNetworkIDReply are the results from calling GetNetworkID
type GetNetworkIDReply struct {
	NetworkID json.Uint32 `json:"networkID"`
}

func (c *client) GetNetworkID(ctx context.Context, options ...rpc.Option) (uint32, error) {
	res := &GetNetworkIDReply{}
	err := c.requester.SendRequest(ctx, "info.getNetworkID", struct{}{}, res, options...)
	return uint32(res.NetworkID), err
}
