// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package common

import (
	"context"
	"time"

	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

type AppHandler interface {
	AppRequestHandler
	AppResponseHandler
	AppGossipHandler
}

type AppRequestHandler interface {
	// Notify this engine of a request for an AppResponse with the same
	// requestID.
	//
	// The meaning of request, and what should be sent in response to it, is
	// application (VM) specific.
	//
	// It is not guaranteed that request is well-formed or valid.
	//
	// This function can be called by any node at any time.
	AppRequest(
		ctx context.Context,
		nodeID ids.NodeID,
		requestID uint32,
		deadline time.Time,
		request []byte,
	) error
}

type AppResponseHandler interface {
	// Notify this engine of the response to a previously sent AppRequest with
	// the same requestID.
	//
	// The meaning of response is application (VM) specifc.
	//
	// It is not guaranteed that response is well-formed or valid.
	AppResponse(
		ctx context.Context,
		nodeID ids.NodeID,
		requestID uint32,
		response []byte,
	) error

	// Notify this engine that an AppRequest it issued has failed.
	//
	// This function will be called if an AppRequest message with nodeID and
	// requestID was previously sent by this engine and will not receive a
	// response.
	AppRequestFailed(
		ctx context.Context,
		nodeID ids.NodeID,
		requestID uint32,
		appErr *AppError,
	) error
}

type AppGossipHandler interface {
	// Notify this engine of a gossip message from nodeID.
	//
	// The meaning of msg is application (VM) specific, and the VM defines how
	// to react to this message.
	//
	// This message is not expected in response to any event, and it does not
	// need to be responded to.
	AppGossip(
		ctx context.Context,
		nodeID ids.NodeID,
		msg []byte,
	) error
}
