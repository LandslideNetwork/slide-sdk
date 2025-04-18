// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package router

import (
	"context"

	"github.com/landslidenetwork/slide-sdk/proto/p2p"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/message"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/networking/benchlist"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

// Router routes consensus messages to the Handler of the consensus
// engine that the messages are intended for
type Router interface {
	ExternalHandler
	InternalHandler
}

// InternalHandler deals with messages internal to this node
type InternalHandler interface {
	benchlist.Benchable

	RegisterRequest(
		ctx context.Context,
		nodeID ids.NodeID,
		chainID ids.ID,
		requestID uint32,
		op message.Op,
		failedMsg message.InboundMessage,
		engineType p2p.EngineType,
	)
}
