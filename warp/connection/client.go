// (c) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package connection

import (
	"context"
	"fmt"
	"github.com/cometbft/cometbft/libs/bytes"

	rpcclient "github.com/cometbft/cometbft/rpc/jsonrpc/client"
	"github.com/consideritdone/landslidevm/utils/ids"
)

var _ Client = (*client)(nil)

type Client interface {
	//GetMessageSignature(ctx context.Context, messageID ids.ID) ([]byte, error)
	//GetMessageAggregateSignature(ctx context.Context, messageID ids.ID, quorumNum uint64) ([]byte, error)
	GetBlockSignature(ctx context.Context, blockID ids.ID) ([]byte, error)
	//GetBlockAggregateSignature(ctx context.Context, blockID ids.ID, quorumNum uint64) ([]byte, error)
}

// client implementation for interacting with EVM [chain]
type client struct {
	client *rpcclient.Client
}

// NewClient returns a Client for interacting with EVM [chain]
func NewClient(uri, chain string) (Client, error) {
	innerClient, err := rpcclient.New(fmt.Sprintf("%s/ext/bc/%s/rpc", uri, chain))
	if err != nil {
		return nil, fmt.Errorf("failed to dial client. err: %w", err)
	}
	return &client{
		client: innerClient,
	}, nil
}

//
//func (c *client) GetMessageSignature(ctx context.Context, messageID ids.ID) ([]byte, error) {
//	var res hexutil.Bytes
//	if err := c.client.CallContext(ctx, &res, "warp_getMessageSignature", messageID); err != nil {
//		return nil, fmt.Errorf("call to warp_getMessageSignature failed. err: %w", err)
//	}
//	return res, nil
//}
//
//func (c *client) GetMessageAggregateSignature(ctx context.Context, messageID ids.ID, quorumNum uint64) ([]byte, error) {
//	var res hexutil.Bytes
//	if err := c.client.CallContext(ctx, &res, "warp_getMessageAggregateSignature", messageID, quorumNum); err != nil {
//		return nil, fmt.Errorf("call to warp_getMessageAggregateSignature failed. err: %w", err)
//	}
//	return res, nil
//}

func (c *client) GetBlockSignature(ctx context.Context, blockID ids.ID) ([]byte, error) {
	var err error
	var res bytes.HexBytes
	_, err = c.client.Call(ctx, "get_block_signature", map[string]interface{}{"blockID": blockID}, &res)
	if err != nil {
		return nil, fmt.Errorf("call to get_block_signature failed. err: %w", err)
	}
	return res, nil
}

//
//func (c *client) GetBlockAggregateSignature(ctx context.Context, blockID ids.ID, quorumNum uint64) ([]byte, error) {
//	var res hexutil.Bytes
//	if err := c.client.CallContext(ctx, &res, "warp_getBlockAggregateSignature", blockID, quorumNum); err != nil {
//		return nil, fmt.Errorf("call to warp_getBlockAggregateSignature failed. err: %w", err)
//	}
//	return res, nil
//}
