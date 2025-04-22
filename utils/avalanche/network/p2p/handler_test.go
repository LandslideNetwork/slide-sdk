// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package p2p

import (
	"context"
	"testing"
	"time"

	"github.com/cometbft/cometbft/libs/log"

	"github.com/stretchr/testify/require"

	"github.com/landslidenetwork/slide-sdk/utils/avalanche/common"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	"github.com/landslidenetwork/slide-sdk/utils/set"
)

var _ ValidatorSet = (*testValidatorSet)(nil)

type testValidatorSet struct {
	validators set.Set[ids.NodeID]
}

func (t testValidatorSet) Has(_ context.Context, nodeID ids.NodeID) bool {
	return t.validators.Contains(nodeID)
}

func TestValidatorHandlerAppRequest(t *testing.T) {
	nodeID := ids.GenerateTestNodeID()
	validatorSet := set.Of(nodeID)

	tests := []struct {
		name         string
		validatorSet ValidatorSet
		nodeID       ids.NodeID
		expected     *common.AppError
	}{
		{
			name:         "message dropped",
			validatorSet: testValidatorSet{},
			nodeID:       nodeID,
			expected:     ErrNotValidator,
		},
		{
			name: "message handled",
			validatorSet: testValidatorSet{
				validators: validatorSet,
			},
			nodeID: nodeID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			handler := NewValidatorHandler(
				NoOpHandler{},
				tt.validatorSet,
				log.NewNopLogger(),
			)

			_, err := handler.AppRequest(context.Background(), tt.nodeID, time.Time{}, []byte("foobar"))
			require.ErrorIs(err, tt.expected)
		})
	}
}
