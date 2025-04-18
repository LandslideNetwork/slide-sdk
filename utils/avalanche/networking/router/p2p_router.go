// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package router

import (
	"context"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/landslidenetwork/slide-sdk/proto/p2p"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/message"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/networking/timeout"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	"github.com/landslidenetwork/slide-sdk/utils/version"
	"go.uber.org/zap"
)

// P2PRouter routes incoming messages from the validator network
// to the consensus engines that the messages are intended for.
// Note that consensus engines are uniquely identified by the ID of the chain
// that they are working on.
// Invariant: P-chain must be registered before processing any messages
type P2PRouter struct {
	log log.Logger

	// It is only safe to call [RegisterResponse] with the router lock held. Any
	// other calls to the timeout manager with the router lock held could cause
	// a deadlock because the timeout manager will call Benched and Unbenched.
	timeoutManager timeout.Manager
}

// Initialize the router.
//
// When this router receives an incoming message, it cancels the timeout in
// [timeouts] associated with the request that caused the incoming message, if
// applicable.
func (r *P2PRouter) Initialize(
	log log.Logger,
	timeoutManager timeout.Manager,
) error {
	r.log = log
	r.timeoutManager = timeoutManager
	return nil
}

// RegisterRequest marks that we should expect to receive a reply for a request
// from the given node's [chainID] and
// the reply should have the given requestID.
//
// The type of message we expect is [op].
//
// Every registered request must be cleared either by receiving a valid reply
// and passing it to the appropriate chain or by a timeout.
// This method registers a timeout that calls such methods if we don't get a
// reply in time.
func (r *P2PRouter) RegisterRequest(
	ctx context.Context,
	nodeID ids.NodeID,
	chainID ids.ID,
	requestID uint32,
	op message.Op,
	timeoutMsg message.InboundMessage,
	engineType p2p.EngineType,
) {

}

func (r *P2PRouter) HandleInbound(ctx context.Context, msg message.InboundMessage) {
	nodeID := msg.NodeID()
	op := msg.Op()

	m := msg.Message()
	r.log.Debug("Received P2P message with fields",
		zap.Stringer("nodeID", nodeID),
		zap.Stringer("messageOp", op),
		zap.Stringer("message", m),
	)
}

// Connected routes an incoming notification that a validator was just connected

func (r *P2PRouter) Connected(nodeID ids.NodeID, nodeVersion *version.Application, subnetID ids.ID) {

}

// Disconnected routes an incoming notification that a validator was connected

func (r *P2PRouter) Disconnected(nodeID ids.NodeID) {

}

// Benched routes an incoming notification that a validator was benched
func (r *P2PRouter) Benched(chainID ids.ID, nodeID ids.NodeID) {

}

// Unbenched routes an incoming notification that a validator was just unbenched
func (r *P2PRouter) Unbenched(chainID ids.ID, nodeID ids.NodeID) {

}
