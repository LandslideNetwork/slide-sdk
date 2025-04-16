// (c) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"

	"github.com/cometbft/cometbft/libs/log"

	"github.com/landslidenetwork/slide-sdk/utils/codec"
	warpHandlers "github.com/landslidenetwork/slide-sdk/utils/evm/warp/handlers"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	"github.com/landslidenetwork/slide-sdk/utils/message"
	"github.com/landslidenetwork/slide-sdk/warp"
)

var _ message.RequestHandler = &networkHandler{}

type networkHandler struct {
	signatureRequestHandler *warpHandlers.SignatureRequestHandler
}

// newNetworkHandler constructs the handler for serving network requests.
func newNetworkHandler(
	warpBackend warp.Backend,
	networkCodec codec.Manager,
	logger log.Logger,
) message.RequestHandler {
	return &networkHandler{
		signatureRequestHandler: warpHandlers.NewSignatureRequestHandler(warpBackend, networkCodec, logger),
	}
}

func (n networkHandler) HandleMessageSignatureRequest(ctx context.Context, nodeID ids.NodeID, requestID uint32, messageSignatureRequest message.MessageSignatureRequest) ([]byte, error) {
	return n.signatureRequestHandler.OnMessageSignatureRequest(ctx, nodeID, requestID, messageSignatureRequest)
}

func (n networkHandler) HandleBlockSignatureRequest(ctx context.Context, nodeID ids.NodeID, requestID uint32, blockSignatureRequest message.BlockSignatureRequest) ([]byte, error) {
	return n.signatureRequestHandler.OnBlockSignatureRequest(ctx, nodeID, requestID, blockSignatureRequest)
}
