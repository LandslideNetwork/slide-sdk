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
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/validators"
	warp2 "github.com/landslidenetwork/slide-sdk/utils/avalanche/warp"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/warp/payload"
	"github.com/landslidenetwork/slide-sdk/utils/evm/warp/aggregator"
	warpValidators "github.com/landslidenetwork/slide-sdk/utils/evm/warp/validators"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	"github.com/landslidenetwork/slide-sdk/warp"
)

const (
	failedParseIDPattern   = "failed to parse ID %s with error %w"
	failedParseWARPMessage = "failed to parse warp message %s with error %w"
)

type ResultAddMessage struct {
	MessageID string `json:"messageID"`
}

var errNoValidators = errors.New("cannot aggregate signatures from subnets with no validators")

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
	valState                      *warpValidators.State
	sourceSubnetID, sourceChainID ids.ID
	backend                       warp.Backend
	signatureGetter               aggregator.SignatureGetter
	// TODO: investigate necessity to set up value according to validation of Primary Network
	// requirePrimaryNetworkSigners returns true if warp messages from the primary
	// network must be signed by the primary network validators.
	// This is necessary when the subnets is not validating the primary network.
	requirePrimaryNetworkSigners bool
}

func NewAPI(vm *LandslideVM, logger log.Logger, networkID uint32, state validators.State, sourceSubnetID ids.ID, sourceChainID ids.ID,
	backend warp.Backend, sigGetter *aggregator.NetworkSignatureGetter, rpcClients map[ids.NodeID]warp.Client, requirePrimaryNetworkSigners bool) *API {
	var signatureGetter aggregator.SignatureGetter
	if sigGetter != nil {
		signatureGetter = sigGetter
	} else {
		signatureGetter = warp.NewAPIFetcher(rpcClients)
	}
	return &API{
		vm:                           vm,
		logger:                       logger,
		networkID:                    networkID,
		valState:                     warpValidators.NewState(state, sourceSubnetID, sourceChainID, requirePrimaryNetworkSigners),
		sourceSubnetID:               sourceSubnetID,
		sourceChainID:                sourceChainID,
		backend:                      backend,
		signatureGetter:              signatureGetter,
		requirePrimaryNetworkSigners: requirePrimaryNetworkSigners,
	}
}

// AddMessage returns the Warp message associated with a messageID.
func (a *API) AddMessage(_ *rpctypes.Context, message []byte) (*ResultAddMessage, error) {
	msg, err := warp2.ParseUnsignedMessage(message)
	if err != nil {
		return nil, fmt.Errorf(failedParseWARPMessage, message, err)
	}
	err = a.backend.AddMessage(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to add message {ID: %s} with error %w", msg.ID().String(), err)
	}
	return &ResultAddMessage{MessageID: msg.ID().String()}, nil
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

// GetBlockSignature returns the BLS signature associated with a blockID.
func (a *API) GetBlockSignature(ctx context.Context, blockID ids.ID) (tmbytes.HexBytes, error) {
	signature, err := a.backend.GetBlockSignature(blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to get signature for block %s with error %w", blockID, err)
	}
	return signature, nil
}

// GetBlockAggregateSignature fetches the aggregate signature for the requested [blockID]
func (a *API) GetBlockAggregateSignature(ctx context.Context, blockID ids.ID, quorumNum uint64, subnetIDStr string) (signedMessageBytes tmbytes.HexBytes, err error) {
	blockHashPayload, err := payload.NewHash(blockID)
	if err != nil {
		return nil, err
	}
	unsignedMessage, err := warp2.NewUnsignedMessage(a.networkID, a.sourceChainID, blockHashPayload.Bytes())
	if err != nil {
		return nil, err
	}

	return a.aggregateSignatures(ctx, unsignedMessage, quorumNum, subnetIDStr)
}

func (a *API) aggregateSignatures(ctx context.Context, unsignedMessage *warp2.UnsignedMessage, quorumNum uint64, subnetIDStr string) (tmbytes.HexBytes, error) {
	subnetID := a.sourceSubnetID
	if len(subnetIDStr) > 0 {
		sid, err := ids.FromString(subnetIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse subnetID: %q", subnetIDStr)
		}
		subnetID = sid
	}
	pChainHeight, err := a.valState.GetCurrentHeight(ctx)
	if err != nil {
		return nil, err
	}
	// Get the validator set at the given height.
	vdrSet, err := a.valState.GetValidatorSet(ctx, pChainHeight, subnetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get validator set: %w", err)
	}

	// Convert the validator set into the canonical ordering.
	validators, totalWeight, err := warp2.FlattenValidatorSet(vdrSet)
	if err != nil {
		return nil, fmt.Errorf("failed to convert the validator set into the canonical ordering: %w", err)
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
	agg := aggregator.New(a.signatureGetter, a.logger, validators, totalWeight)
	signatureResult, err := agg.AggregateSignatures(ctx, unsignedMessage, quorumNum)
	if err != nil {
		return nil, err
	}
	// TODO: return the signature and total weight as well to the caller for more complete details
	// Need to decide on the best UI for this and write up documentation with the potential
	// gotchas that could impact signed messages becoming invalid.
	return signatureResult.Message.Bytes(), nil
}
