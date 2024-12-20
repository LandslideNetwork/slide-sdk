// (c) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"errors"
	"fmt"
	tmbytes "github.com/cometbft/cometbft/libs/bytes"
	"github.com/cometbft/cometbft/libs/log"
	rpctypes "github.com/cometbft/cometbft/rpc/jsonrpc/types"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	"github.com/landslidenetwork/slide-sdk/utils/validators"
	warputils "github.com/landslidenetwork/slide-sdk/utils/warp"
	"github.com/landslidenetwork/slide-sdk/utils/warp/payload"
	warpValidators "github.com/landslidenetwork/slide-sdk/utils/warp/validators"
	"github.com/landslidenetwork/slide-sdk/warp"
)

const failedParseIDPattern = "failed to parse ID %s with error %w"

var errNoValidators = errors.New("cannot aggregate signatures from subnet with no validators")

type ResultGetMessage struct {
	Message []byte `json:"message"`
}

type ResultGetMessageSignature struct {
	Signature []byte `json:"signature"`
}

// API introduces snowman specific functionality to the evm
type API struct {
	vm                            *LandslideVM
	logger                        log.Logger
	networkID                     uint32
	state                         validators.State
	sourceSubnetID, sourceChainID ids.ID
	backend                       warp.Backend
	//TODO: investigate necessity to set up value according to validation of Primary Network
	// requirePrimaryNetworkSigners returns true if warp messages from the primary
	// network must be signed by the primary network validators.
	// This is necessary when the subnet is not validating the primary network.
	requirePrimaryNetworkSigners bool
}

func NewAPI(vm *LandslideVM, logger log.Logger, networkID uint32, state validators.State, sourceSubnetID ids.ID, sourceChainID ids.ID,
	backend warp.Backend, requirePrimaryNetworkSigners bool) *API {
	return &API{
		vm:                           vm,
		logger:                       logger,
		networkID:                    networkID,
		state:                        state,
		sourceSubnetID:               sourceSubnetID,
		sourceChainID:                sourceChainID,
		backend:                      backend,
		requirePrimaryNetworkSigners: requirePrimaryNetworkSigners,
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
	subnetID := a.sourceSubnetID
	if len(subnetIDStr) > 0 {
		sid, err := ids.FromString(subnetIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse subnetID: %q", subnetIDStr)
		}
		subnetID = sid
	}
	pChainHeight, err := a.state.GetCurrentHeight(ctx)
	if err != nil {
		return nil, err
	}
	state := warpValidators.NewState(a.state, a.sourceSubnetID, a.sourceChainID, a.requirePrimaryNetworkSigners)
	validators, totalWeight, err := warputils.GetCanonicalValidatorSet(ctx, state, pChainHeight, subnetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get validator set: %w", err)
	}
	if len(validators) == 0 {
		return nil, fmt.Errorf("%w (SubnetID: %s, Height: %d)", errNoValidators, subnetID, pChainHeight)
	}

	a.logger.Debug("Fetching signature",
		"sourceSubnetID", subnetID,
		"height", pChainHeight,
		"numValidators", len(validators),
		"totalWeight", totalWeight,
	)
	agg := aggregator.New(aggregator.NewSignatureGetter(a.client), validators, totalWeight)
	return nil, nil
}
