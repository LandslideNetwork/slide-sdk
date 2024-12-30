// (c) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package warp

import (
	"context"
	"fmt"

	tmbytes "github.com/cometbft/cometbft/libs/bytes"

	jsonrpc "github.com/cometbft/cometbft/rpc/jsonrpc/client"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

var _ Client = (*client)(nil)

type Client interface {
	GetMessage(ctx context.Context, messageID ids.ID) ([]byte, error)
	GetMessageSignature(ctx context.Context, messageID ids.ID) ([]byte, error)
	GetMessageAggregateSignature(ctx context.Context, messageID ids.ID, quorumNum uint64, subnetIDStr string) ([]byte, error)
	GetBlockSignature(ctx context.Context, blockID ids.ID) ([]byte, error)
	GetBlockAggregateSignature(ctx context.Context, blockID ids.ID, quorumNum uint64, subnetIDStr string) ([]byte, error)
}

// client implementation for interacting with EVM [chain]
type client struct {
	*jsonrpc.Client
}

// NewClient returns a Client for interacting with EVM [chain]
func NewClient(uri, chain string) (Client, error) {
	rpcClient, err := jsonrpc.New(fmt.Sprintf("%s/ext/bc/%s/rpc", uri, chain))
	if err != nil {
		return nil, fmt.Errorf("failed to dial client. err: %w", err)
	}
	return &client{
		Client: rpcClient,
	}, nil
}

func (c *client) GetMessage(ctx context.Context, messageID ids.ID) ([]byte, error) {
	var res tmbytes.HexBytes
	if _, err := c.Call(ctx, "warp_get_message", map[string]interface{}{"messageID": messageID}, &res); err != nil {
		return nil, fmt.Errorf("call to warp_get_message failed. err: %w", err)
	}
	return res, nil
}

func (c *client) GetMessageSignature(ctx context.Context, messageID ids.ID) ([]byte, error) {
	var res tmbytes.HexBytes
	if _, err := c.Call(ctx, "warp_get_message_signature", map[string]interface{}{"messageID": messageID}, &res); err != nil {
		return nil, fmt.Errorf("call to warp_get_message_signature failed. err: %w", err)
	}
	return res, nil
}

func (c *client) GetMessageAggregateSignature(ctx context.Context, messageID ids.ID, quorumNum uint64, subnetIDStr string) ([]byte, error) {
	var res tmbytes.HexBytes
	if _, err := c.Call(ctx, "warp_get_message_aggregate_signature",
		map[string]interface{}{
			"messageID":   messageID,
			"quorumNum":   quorumNum,
			"subnetIDStr": subnetIDStr,
		}, &res); err != nil {
		return nil, fmt.Errorf("call to warp_get_message_aggregate_signature failed. err: %w", err)
	}
	return res, nil
}

func (c *client) GetBlockSignature(ctx context.Context, blockID ids.ID) ([]byte, error) {
	var res tmbytes.HexBytes
	if _, err := c.Call(ctx, "warp_get_block_signature", map[string]interface{}{"blockID": blockID}, &res); err != nil {
		return nil, fmt.Errorf("call to warp_get_block_signature failed. err: %w", err)
	}
	return res, nil
}

func (c *client) GetBlockAggregateSignature(ctx context.Context, blockID ids.ID, quorumNum uint64, subnetIDStr string) ([]byte, error) {
	var res tmbytes.HexBytes
	if _, err := c.Call(ctx, "warp_get_block_aggregate_signature",
		map[string]interface{}{
			"blockID":     blockID,
			"quorumNum":   quorumNum,
			"subnetIDStr": subnetIDStr,
		}, &res); err != nil {
		return nil, fmt.Errorf("call to warp_get_block_aggregate_signature failed. err: %w", err)
	}
	return res, nil
}
