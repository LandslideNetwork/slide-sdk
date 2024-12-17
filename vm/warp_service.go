// (c) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"fmt"

	tmbytes "github.com/cometbft/cometbft/libs/bytes"
	rpctypes "github.com/cometbft/cometbft/rpc/jsonrpc/types"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	warputils "github.com/landslidenetwork/slide-sdk/utils/warp"
	"github.com/landslidenetwork/slide-sdk/utils/warp/payload"
	"github.com/landslidenetwork/slide-sdk/warp"
)

const failedParseIDPattern = "failed to parse ID %s with error %w"

type ResultGetMessage struct {
	Message []byte `json:"message"`
}

type ResultGetMessageSignature struct {
	Signature []byte `json:"signature"`
}

// API introduces snowman specific functionality to the evm
type API struct {
	vm                            *LandslideVM
	networkID                     uint32
	sourceSubnetID, sourceChainID ids.ID
	backend                       warp.Backend
}

func NewAPI(vm *LandslideVM, networkID uint32, sourceSubnetID ids.ID, sourceChainID ids.ID, backend warp.Backend) *API {
	return &API{
		vm:             vm,
		networkID:      networkID,
		sourceSubnetID: sourceSubnetID,
		sourceChainID:  sourceChainID,
		backend:        backend,
	}
}

// GetMessage returns the Warp message associated with a messageID.
func (a *API) GetMessage(_ *rpctypes.Context, messageID string) (*ResultGetMessage, error) {
	msgID, err := ids.FromString(messageID)
	if err != nil {
		return nil, fmt.Errorf(failedParseIDPattern, messageID, err)
	}
	message, err := a.backend.GetMessage(msgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get message %s with error %w", messageID, err)
	}
	return &ResultGetMessage{Message: message.Bytes()}, nil
}

// GetMessageSignature returns the BLS signature associated with a messageID.
func (a *API) GetMessageSignature(_ *rpctypes.Context, messageID string) (*ResultGetMessageSignature, error) {
	msgID, err := ids.FromString(messageID)
	if err != nil {
		return nil, fmt.Errorf(failedParseIDPattern, messageID, err)
	}
	unsignedMessage, err := a.backend.GetMessage(msgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get message %s with error %w", messageID, err)
	}
	signature, err := a.backend.GetMessageSignature(unsignedMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to get signature for message %s with error %w", messageID, err)
	}
	return &ResultGetMessageSignature{Signature: signature}, nil
}

// GetMessageAggregateSignature fetches the aggregate signature for the requested [messageID]
func (a *API) GetMessageAggregateSignature(ctx context.Context, messageID ids.ID, quorumNum uint64, subnetIDStr string) (signedMessageBytes tmbytes.HexBytes, err error) {
	unsignedMessage, err := a.backend.GetMessage(messageID)
	if err != nil {
		return nil, err
	}
	return a.aggregateSignatures(ctx, unsignedMessage, quorumNum, subnetIDStr)
}

// GetBlockAggregateSignature fetches the aggregate signature for the requested [blockID]
func (a *API) GetBlockAggregateSignature(ctx context.Context, blockID ids.ID, quorumNum uint64, subnetIDStr string) (signedMessageBytes tmbytes.HexBytes, err error) {
	blockHashPayload, err := payload.NewHash(blockID)
	if err != nil {
		return nil, err
	}
	unsignedMessage, err := warputils.NewUnsignedMessage(a.networkID, a.sourceChainID, blockHashPayload.Bytes())
	if err != nil {
		return nil, err
	}

	return a.aggregateSignatures(ctx, unsignedMessage, quorumNum, subnetIDStr)
}

func (a *API) aggregateSignatures(ctx context.Context, unsignedMessage *warputils.UnsignedMessage, quorumNum uint64, subnetIDStr string) (tmbytes.HexBytes, error) {
	// TODO: implement aggregateSignatures
	return nil, nil
}
