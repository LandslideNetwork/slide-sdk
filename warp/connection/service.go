// (c) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package connection

import (
	"context"
	"errors"
	"fmt"
	"github.com/cometbft/cometbft/libs/bytes"
	rpctypes "github.com/cometbft/cometbft/rpc/jsonrpc/types"
	"github.com/consideritdone/landslidevm/jsonrpc"
	"github.com/consideritdone/landslidevm/utils/ids"
	"github.com/consideritdone/landslidevm/warp"
)

var errNoValidators = errors.New("cannot aggregate signatures from subnet with no validators")

// API introduces snowman specific functionality to the evm
type API struct {
	networkID                     uint32
	sourceSubnetID, sourceChainID ids.ID
	backend                       Backend
	//state                         *validators.State
	//client                        peer.NetworkClient
}

func NewAPI(networkID uint32, sourceSubnetID ids.ID, sourceChainID ids.ID /*, state *validators.State */, backend Backend /*client peer.NetworkClient*/) *API {
	return &API{
		networkID:      networkID,
		sourceSubnetID: sourceSubnetID,
		sourceChainID:  sourceChainID,
		backend:        backend,
		//state:          state,
		//client:         client,
	}
}

func (api *API) Routes() map[string]*jsonrpc.RPCFunc {
	return map[string]*jsonrpc.RPCFunc{
		// warp API
		"warp_1":              jsonrpc.NewRPCFunc(api.GetMessageSignature, "messageID"),
		"get_block_signature": jsonrpc.NewRPCFunc(api.GetBlockSignature, "blockID"),
		"warp_3":              jsonrpc.NewRPCFunc(api.GetMessageAggregateSignature, "messageID, quorumNum"),
		"warp_4":              jsonrpc.NewRPCFunc(api.GetBlockAggregateSignature, "blockID, quorumNum"),
	}
}

// GetMessageSignature returns the BLS signature associated with a messageID.
func (api *API) GetMessageSignature(ctx context.Context, messageID ids.ID) (bytes.HexBytes, error) {
	//signature, err := a.backend.GetMessageSignature(messageID)
	//if err != nil {
	//	return nil, fmt.Errorf("failed to get signature for message %s with error %w", messageID, err)
	//}
	//return signature[:], nil
	return nil, nil
}

// GetBlockSignature returns the BLS signature associated with a blockID.
func (api *API) GetBlockSignature(_ *rpctypes.Context, blockID ids.ID) (bytes.HexBytes, error) {
	signature, err := api.backend.GetBlockSignature(blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to get signature for block %s with error %w", blockID, err)
	}
	return signature[:], nil
}

// GetMessageAggregateSignature fetches the aggregate signature for the requested [messageID]
func (api *API) GetMessageAggregateSignature(ctx context.Context, messageID ids.ID, quorumNum uint64) (signedMessageBytes bytes.HexBytes, err error) {
	//unsignedMessage, err := a.backend.GetMessage(messageID)
	//if err != nil {
	//	return nil, err
	//}
	//return a.aggregateSignatures(ctx, unsignedMessage, quorumNum)
	return nil, nil
}

// GetBlockAggregateSignature fetches the aggregate signature for the requested [blockID]
func (api *API) GetBlockAggregateSignature(ctx context.Context, blockID ids.ID, quorumNum uint64) (signedMessageBytes bytes.HexBytes, err error) {
	//blockHashPayload, err := payload.NewHash(blockID)
	//if err != nil {
	//	return nil, err
	//}
	//unsignedMessage, err := warp.NewUnsignedMessage(a.networkID, a.sourceChainID, blockHashPayload.Bytes())
	//if err != nil {
	//	return nil, err
	//}
	//
	//return a.aggregateSignatures(ctx, unsignedMessage, quorumNum)
	return nil, nil
}

func (api *API) aggregateSignatures(ctx context.Context, unsignedMessage *warp.UnsignedMessage, quorumNum uint64) (bytes.HexBytes, error) {
	//pChainHeight, err := a.state.GetCurrentHeight(ctx)
	//if err != nil {
	//	return nil, err
	//}
	//
	//log.Debug("Fetching signature",
	//	"a.subnetID", a.sourceSubnetID,
	//	"height", pChainHeight,
	//)
	//validators, totalWeight, err := warp.GetCanonicalValidatorSet(ctx, a.state, pChainHeight, a.sourceSubnetID)
	//if err != nil {
	//	return nil, fmt.Errorf("failed to get validator set: %w", err)
	//}
	//if len(validators) == 0 {
	//	return nil, fmt.Errorf("%w (SubnetID: %s, Height: %d)", errNoValidators, a.sourceSubnetID, pChainHeight)
	//}
	//
	//agg := aggregator.New(aggregator.NewSignatureGetter(a.client), validators, totalWeight)
	//signatureResult, err := agg.AggregateSignatures(ctx, unsignedMessage, quorumNum)
	//if err != nil {
	//	return nil, err
	//}
	//// TODO: return the signature and total weight as well to the caller for more complete details
	//// Need to decide on the best UI for this and write up documentation with the potential
	//// gotchas that could impact signed messages becoming invalid.
	//return hexutil.Bytes(signatureResult.Message.Bytes()), nil
	return nil, nil
}
