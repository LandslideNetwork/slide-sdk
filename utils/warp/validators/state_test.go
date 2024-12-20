// (c) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validators

import (
	"testing"

	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

func TestGetValidatorSetPrimaryNetwork(t *testing.T) {
	//TODO: implement test
	//require := require.New(t)
	//ctrl := gomock.NewController(t)

	subnetID := ids.GenerateTestID()
	otherSubnetID := ids.GenerateTestID()

	t.Log(subnetID)
	t.Log(otherSubnetID)

	////mockState := validatorsmock.NewState(ctrl)
	////snowCtx := utils.TestSnowContext()
	////snowCtx.SubnetID = mySubnetID
	////snowCtx.ValidatorState = mockState
	//state := NewState(snowCtx.ValidatorState, subnetID, snowCtx.ChainID, false)
	////// Expect that requesting my validator set returns my validator set
	////mockState.EXPECT().GetValidatorSet(gomock.Any(), gomock.Any(), mySubnetID).Return(make(map[ids.NodeID]*validators.GetValidatorOutput), nil)
	//output, err := state.GetValidatorSet(context.Background(), 10, subnetID)
	//require.NoError(err)
	//require.Len(output, 0)
	//
	////// Expect that requesting the Primary Network validator set overrides and returns my validator set
	////mockState.EXPECT().GetValidatorSet(gomock.Any(), gomock.Any(), mySubnetID).Return(make(map[ids.NodeID]*validators.GetValidatorOutput), nil)
	//output, err = state.GetValidatorSet(context.Background(), 10, PrimaryNetworkID)
	//require.NoError(err)
	//require.Len(output, 0)
	////
	////// Expect that requesting other validator set returns that validator set
	////mockState.EXPECT().GetValidatorSet(gomock.Any(), gomock.Any(), otherSubnetID).Return(make(map[ids.NodeID]*validators.GetValidatorOutput), nil)
	//output, err = state.GetValidatorSet(context.Background(), 10, otherSubnetID)
	//require.NoError(err)
	//require.Len(output, 0)
}
