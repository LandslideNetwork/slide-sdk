// (c) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validators

import (
	"context"

	"github.com/landslidenetwork/slide-sdk/utils/avalanche/validators"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

var (
	_                validators.State = (*State)(nil)
	PrimaryNetworkID                  = ids.Empty
	PlatformChainID                   = ids.Empty
)

// State provides a special case used to handle Avalanche Warp Message verification for messages sent
// from the Primary Network. Subnets have strictly fewer validators than the Primary Network, so we require
// signatures from a threshold of the RECEIVING subnets validator set rather than the full Primary Network
// since the receiving subnets already relies on a majority of its validators being correct.
type State struct {
	validators.State
	mySubnetID                   ids.ID
	sourceChainID                ids.ID
	requirePrimaryNetworkSigners bool
}

// NewState returns a wrapper of [validators.State] which special cases the handling of the Primary Network.
//
// The wrapped state will return the [mySubnetID's] validator set instead of the Primary Network when
// the Primary Network SubnetID is passed in.
func NewState(state validators.State, mySubnetID ids.ID, sourceChainID ids.ID, requirePrimaryNetworkSigners bool) *State {
	return &State{
		State:                        state,
		mySubnetID:                   mySubnetID,
		sourceChainID:                sourceChainID,
		requirePrimaryNetworkSigners: requirePrimaryNetworkSigners,
	}
}

func (s *State) GetValidatorSet(
	ctx context.Context,
	height uint64,
	subnetID ids.ID,
) (map[ids.NodeID]*validators.GetValidatorOutput, error) {
	// If the subnetID is anything other than the Primary Network, or Primary
	// Network signers are required (except P-Chain), this is a direct passthrough.
	usePrimary := s.requirePrimaryNetworkSigners && s.sourceChainID != PlatformChainID
	if usePrimary || subnetID != PrimaryNetworkID {
		return s.State.GetValidatorSet(ctx, height, subnetID)
	}

	// If the requested subnets is the primary network, then we return the validator
	// set for the Subnet that is receiving the message instead.
	return s.State.GetValidatorSet(ctx, height, s.mySubnetID)
}
