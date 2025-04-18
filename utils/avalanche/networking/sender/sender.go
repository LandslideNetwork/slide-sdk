// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package sender

import (
	"context"
	"time"

	"github.com/landslidenetwork/slide-sdk/proto/p2p"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/common"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/message"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/networking/router"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/networking/timeout"
	"github.com/landslidenetwork/slide-sdk/utils/subnets"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	"github.com/landslidenetwork/slide-sdk/utils/set"
)

const opLabel = "op"

var (
	_ common.Sender = (*P2PAppSender)(nil)

	opLabels = []string{opLabel}
)

type P2PAppSender struct {
	ChainID    ids.ID
	SubnetID   ids.ID
	NodeID     ids.NodeID
	logger     log.Logger
	router     router.Router
	timeouts   timeout.Manager
	msgCreator message.OutboundMsgBuilder
	sender     ExternalSender // Actually does the sending over the network
	allower    subnets.Allower
	// Counts how many request have failed because the node was benched
	failedDueToBench *prometheus.CounterVec // op
}

func New(chainID ids.ID, subnetID ids.ID, nodeID ids.NodeID, logger log.Logger, timeouts timeout.Manager, msgCreator message.OutboundMsgBuilder, externalSender ExternalSender, router router.Router, allowedNodes set.Set[ids.NodeID]) *P2PAppSender {
	return &P2PAppSender{
		ChainID:    chainID,
		SubnetID:   subnetID,
		NodeID:     nodeID,
		logger:     logger,
		timeouts:   timeouts,
		msgCreator: msgCreator,
		sender:     externalSender,
		failedDueToBench: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "failed_benched",
				Help: "requests dropped because a node was benched",
			},
			opLabels,
		),
		router: router,
		allower: subnets.New(nodeID, subnets.Config{
			AllowedNodes:                allowedNodes,
			ValidatorOnly:               true,
			ProposerMinBlockDelay:       time.Second,
			ProposerNumHistoricalBlocks: 3,
		}),
	}
}

func (s *P2PAppSender) SendAppRequest(ctx context.Context, nodeIDs set.Set[ids.NodeID], requestID uint32, appRequestBytes []byte) error {
	ctx = context.WithoutCancel(ctx)

	// Tell the router to expect a response message or a message notifying
	// that we won't get a response from each of these nodes.
	// We register timeouts for all nodes, regardless of whether we fail
	// to send them a message, to avoid busy looping when disconnected from
	// the internet.
	for nodeID := range nodeIDs {
		inMsg := message.InboundAppError(
			nodeID,
			s.ChainID,
			requestID,
			common.ErrTimeout.Code,
			common.ErrTimeout.Message,
		)
		s.router.RegisterRequest(
			ctx,
			nodeID,
			s.ChainID,
			requestID,
			message.AppResponseOp,
			inMsg,
			p2p.EngineType_ENGINE_TYPE_UNSPECIFIED,
		)
	}

	// Note that this timeout duration won't exactly match the one that gets
	// registered. That's OK.
	deadline := s.timeouts.TimeoutDuration()

	// Sending a message to myself. No need to send it over the network. Just
	// put it right into the router. Do so asynchronously to avoid deadlock.
	if nodeIDs.Contains(s.NodeID) {
		nodeIDs.Remove(s.NodeID)
		inMsg := message.InboundAppRequest(
			s.ChainID,
			requestID,
			deadline,
			appRequestBytes,
			s.NodeID,
		)
		go s.router.HandleInbound(ctx, inMsg)
	}

	// Some of the nodes in [nodeIDs] may be benched. That is, they've been
	// unresponsive so we don't even bother sending messages to them. We just
	// have them immediately fail.
	for nodeID := range nodeIDs {
		if s.timeouts.IsBenched(nodeID) {
			s.failedDueToBench.With(prometheus.Labels{
				opLabel: message.AppRequestOp.String(),
			}).Inc()
			nodeIDs.Remove(nodeID)
			s.timeouts.RegisterRequestToUnreachableValidator()

			// Immediately register a failure. Do so asynchronously to avoid
			// deadlock.
			inMsg := message.InboundAppError(
				nodeID,
				s.ChainID,
				requestID,
				common.ErrTimeout.Code,
				common.ErrTimeout.Message,
			)
			go s.router.HandleInbound(ctx, inMsg)
		}
	}

	// Create the outbound message.
	outMsg, err := s.msgCreator.AppRequest(
		s.ChainID,
		requestID,
		deadline,
		appRequestBytes,
	)

	// Send the message over the network.
	// [sentTo] are the IDs of nodes who may receive the message.
	var sentTo set.Set[ids.NodeID]
	if err == nil {
		sentTo = s.sender.Send(
			outMsg,
			common.SendConfig{
				NodeIDs: nodeIDs,
			},
			s.SubnetID,
			s.allower,
		)
	} else {
		s.logger.Error("failed to build message",
			zap.Stringer("messageOp", message.AppRequestOp),
			zap.Stringer("chainID", s.ChainID),
			zap.Uint32("requestID", requestID),
			zap.Binary("payload", appRequestBytes),
			zap.Error(err),
		)
	}

	for nodeID := range nodeIDs {
		if !sentTo.Contains(nodeID) {
			s.logger.Debug("failed to send message",
				zap.Stringer("messageOp", message.AppRequestOp),
				zap.Stringer("nodeID", nodeID),
				zap.Stringer("chainID", s.ChainID),
				zap.Uint32("requestID", requestID),
			)
		}

		// Register failures for nodes we didn't send a request to.
		s.timeouts.RegisterRequestToUnreachableValidator()
		inMsg := message.InboundAppError(
			nodeID,
			s.ChainID,
			requestID,
			common.ErrTimeout.Code,
			common.ErrTimeout.Message,
		)
		go s.router.HandleInbound(ctx, inMsg)
	}
	return nil
}

func (s *P2PAppSender) SendAppResponse(ctx context.Context, nodeID ids.NodeID, requestID uint32, appResponseBytes []byte) error {
	// TODO implement me
	s.logger.Debug("P2PAppSender SendAppResponse call")
	panic("implement me")
}

func (s *P2PAppSender) SendAppError(ctx context.Context, nodeID ids.NodeID, requestID uint32, errorCode int32, errorMessage string) error {
	// TODO implement me
	panic("implement me")
}

func (s *P2PAppSender) SendAppGossip(ctx context.Context, config common.SendConfig, appGossipBytes []byte) error {
	// TODO implement me
	panic("implement me")
}
