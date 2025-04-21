// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	_ "embed"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	vmpb "github.com/landslidenetwork/slide-sdk/proto/vm"
	avalancheWarp "github.com/landslidenetwork/slide-sdk/utils/avalanche/warp"
	"github.com/landslidenetwork/slide-sdk/utils/crypto/bls"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	"github.com/landslidenetwork/slide-sdk/utils/message"
	"github.com/stretchr/testify/require"
)

func TestMessageSignatureRequestsToVM(t *testing.T) {
	vm, appSender := NewFreshKvApp(t)
	vmLnd := vm.(*LandslideVM)

	defer func() {
		_, err := vm.Shutdown(context.Background(), &empty.Empty{})
		require.NoError(t, err)
	}()

	chainID, err := ids.ToID(vmLnd.appOpts.ChainID)
	require.NoError(t, err)
	// Generate a new warp unsigned message and add to warp backend
	warpMessage, err := avalancheWarp.NewUnsignedMessage(vmLnd.appOpts.NetworkID, chainID, []byte{1, 2, 3})
	require.NoError(t, err)

	// Add the known message and get its signature to confirm.
	err = vmLnd.warpBackend.AddMessage(warpMessage)
	require.NoError(t, err)
	signature, err := vmLnd.warpBackend.GetMessageSignature(warpMessage)
	require.NoError(t, err)
	var knownSignature [bls.SignatureLen]byte
	copy(knownSignature[:], signature)

	tests := map[string]struct {
		messageID        ids.ID
		expectedResponse [bls.SignatureLen]byte
	}{
		"known": {
			messageID:        warpMessage.ID(),
			expectedResponse: knownSignature,
		},
		"unknown": {
			messageID:        ids.GenerateTestID(),
			expectedResponse: [bls.SignatureLen]byte{},
		},
	}

	for name, test := range tests {
		calledSendAppResponseFn := false
		appSender.SendAppResponseF = func(ctx context.Context, nodeID ids.NodeID, requestID uint32, responseBytes []byte) error {
			calledSendAppResponseFn = true
			var response message.SignatureResponse
			err := message.Codec.Unmarshal(responseBytes, &response)
			require.NoError(t, err)
			require.Equal(t, test.expectedResponse, response.Signature)

			return nil
		}
		t.Run(name, func(t *testing.T) {
			var signatureRequest message.Request = message.MessageSignatureRequest{
				MessageID: test.messageID,
			}

			requestBytes, err := message.Codec.Marshal(&signatureRequest)
			require.NoError(t, err)

			// Send the app request and make sure we called SendAppResponseFn
			deadline := time.Now().Add(60 * time.Second)
			err = vmLnd.Network.AppRequest(context.Background(), ids.GenerateTestNodeID(), 1, deadline, requestBytes)
			require.NoError(t, err)
			require.True(t, calledSendAppResponseFn)
		})
	}
}

func TestBlockSignatureRequestsToVM(t *testing.T) {
	vm, appSender := NewFreshKvApp(t)
	vmLnd := vm.(*LandslideVM)

	defer func() {
		_, err := vm.Shutdown(context.Background(), &empty.Empty{})
		require.NoError(t, err)
	}()

	lastAcceptedID, err := vmLnd.GetBlockIDAtHeight(context.Background(), &vmpb.GetBlockIDAtHeightRequest{Height: uint64(vmLnd.state.LastBlockHeight)})
	require.NoError(t, err)

	blkId, err := ids.ToID(lastAcceptedID.BlkId)
	require.NoError(t, err)

	signature, err := vmLnd.warpBackend.GetBlockSignature(blkId)
	require.NoError(t, err)
	var knownSignature [bls.SignatureLen]byte
	copy(knownSignature[:], signature)

	tests := map[string]struct {
		blockID          ids.ID
		expectedResponse [bls.SignatureLen]byte
	}{
		"known": {
			blockID:          blkId,
			expectedResponse: knownSignature,
		},
		"unknown": {
			blockID:          ids.GenerateTestID(),
			expectedResponse: [bls.SignatureLen]byte{},
		},
	}

	for name, test := range tests {
		calledSendAppResponseFn := false
		appSender.SendAppResponseF = func(ctx context.Context, nodeID ids.NodeID, requestID uint32, responseBytes []byte) error {
			calledSendAppResponseFn = true
			var response message.SignatureResponse
			err := message.Codec.Unmarshal(responseBytes, &response)
			require.NoError(t, err)
			require.Equal(t, test.expectedResponse, response.Signature)

			return nil
		}
		t.Run(name, func(t *testing.T) {
			var signatureRequest message.Request = message.BlockSignatureRequest{
				BlockID: test.blockID,
			}

			requestBytes, err := message.Codec.Marshal(&signatureRequest)
			require.NoError(t, err)

			// Send the app request and make sure we called SendAppResponseFn
			deadline := time.Now().Add(60 * time.Second)
			err = vmLnd.Network.AppRequest(context.Background(), ids.GenerateTestNodeID(), 1, deadline, requestBytes)
			require.NoError(t, err)
			require.True(t, calledSendAppResponseFn)
		})
	}
}
