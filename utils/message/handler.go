// (c) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package message

import (
	"context"

	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

var (
	_ RequestHandler = NoopRequestHandler{}
)

// RequestHandler interface handles incoming requests from peers
// Must have methods in format of handleType(context.Context, ids.NodeID, uint32, request Type) error
// so that the Request object of relevant Type can invoke its respective handle method
// on this struct.
// Also see GossipHandler for implementation style.
type RequestHandler interface {
	HandleMessageSignatureRequest(ctx context.Context, nodeID ids.NodeID, requestID uint32, signatureRequest MessageSignatureRequest) ([]byte, error)
	HandleBlockSignatureRequest(ctx context.Context, nodeID ids.NodeID, requestID uint32, signatureRequest BlockSignatureRequest) ([]byte, error)
}

// ResponseHandler handles response for a sent request
// Only one of OnResponse or OnFailure is called for a given requestID, not both
type ResponseHandler interface {
	// OnResponse is invoked when the peer responded to a request
	OnResponse(response []byte) error
	// OnFailure is invoked when there was a failure in processing a request
	OnFailure() error
}

type NoopRequestHandler struct{}

func (NoopRequestHandler) HandleMessageSignatureRequest(ctx context.Context, nodeID ids.NodeID, requestID uint32, signatureRequest MessageSignatureRequest) ([]byte, error) {
	return nil, nil
}

func (NoopRequestHandler) HandleBlockSignatureRequest(ctx context.Context, nodeID ids.NodeID, requestID uint32, signatureRequest BlockSignatureRequest) ([]byte, error) {
	return nil, nil
}
