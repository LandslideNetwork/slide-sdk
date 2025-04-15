// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package sender

import (
	"context"
	"github.com/landslidenetwork/slide-sdk/proto/p2p"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/common"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/message"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/networking/router"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/networking/timeout"
	"github.com/landslidenetwork/slide-sdk/utils/subnets"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"time"

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
			//s.subnets,
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
	//TODO implement me
	s.logger.Debug("P2PAppSender SendAppResponse call")
	panic("implement me")
}

func (s *P2PAppSender) SendAppError(ctx context.Context, nodeID ids.NodeID, requestID uint32, errorCode int32, errorMessage string) error {
	//TODO implement me
	panic("implement me")
}

func (s *P2PAppSender) SendAppGossip(ctx context.Context, config common.SendConfig, appGossipBytes []byte) error {
	//TODO implement me
	panic("implement me")
}

//
//import (
//	"context"
//
//	"github.com/prometheus/client_golang/prometheus"
//	"go.uber.org/zap"
//
//	"github.com/ava-labs/avalanchego/message"
//	"github.com/ava-labs/avalanchego/proto/pb/p2p"
//	"github.com/ava-labs/avalanchego/snow"
//	"github.com/ava-labs/avalanchego/snow/networking/router"
//	"github.com/ava-labs/avalanchego/snow/networking/timeout"
//	"github.com/ava-labs/avalanchego/subnets"
//	"github.com/ava-labs/avalanchego/utils/logging"
//	"github.com/landslidenetwork/slide-sdk/utils/common"
//	"github.com/landslidenetwork/slide-sdk/utils/ids"
//	"github.com/landslidenetwork/slide-sdk/utils/set"
//)
//
//const opLabel = "op"
//
//var (
//	_ common.Sender = (*sender)(nil)
//
//	opLabels = []string{opLabel}
//)

// sender is a wrapper around an ExternalSender.
// Messages to this node are put directly into [router] rather than
// being sent over the network via the wrapped ExternalSender.
// sender registers outbound requests with [router] so that [router]
// fires a timeout if we don't get a response to the request.
//type sender struct{

//	ctx        *snow.ConsensusContext
//	msgCreator message.OutboundMsgBuilder
//
//	sender   ExternalSender // Actually does the sending over the network
//	router   router.Router
//	timeouts timeout.Manager
//
//	// Counts how many request have failed because the node was benched
//	failedDueToBench *prometheus.CounterVec // op
//
//	engineType p2p.EngineType
//	subnets     subnets.Subnet
//}
//
//func New(
//	ctx *snow.ConsensusContext,
//	msgCreator message.OutboundMsgBuilder,
//	externalSender ExternalSender,
//	router router.Router,
//	timeouts timeout.Manager,
//	engineType p2p.EngineType,
//	subnets subnets.Subnet,
//	reg prometheus.Registerer,
//) (common.Sender, error) {
//	s := &sender{
//		ctx:        ctx,
//		msgCreator: msgCreator,
//		sender:     externalSender,
//		router:     router,
//		timeouts:   timeouts,
//		failedDueToBench: prometheus.NewCounterVec(
//			prometheus.CounterOpts{
//				Name: "failed_benched",
//				Help: "requests dropped because a node was benched",
//			},
//			opLabels,
//		),
//		engineType: engineType,
//		subnets:     subnets,
//	}
//	return s, reg.Register(s.failedDueToBench)
//}
//
//func (s *sender) SendGetStateSummaryFrontier(ctx context.Context, nodeIDs set.Set[ids.NodeID], requestID uint32) {
//	ctx = context.WithoutCancel(ctx)
//
//	// Note that this timeout duration won't exactly match the one that gets
//	// registered. That's OK.
//	deadline := s.timeouts.TimeoutDuration()
//
//	// Tell the router to expect a response message or a message notifying
//	// that we won't get a response from each of these nodes.
//	// We register timeouts for all nodes, regardless of whether we fail
//	// to send them a message, to avoid busy looping when disconnected from
//	// the internet.
//	for nodeID := range nodeIDs {
//		inMsg := message.InternalGetStateSummaryFrontierFailed(
//			nodeID,
//			s.ctx.ChainID,
//			requestID,
//		)
//		s.router.RegisterRequest(
//			ctx,
//			nodeID,
//			s.ctx.ChainID,
//			requestID,
//			message.StateSummaryFrontierOp,
//			inMsg,
//			p2p.EngineType_ENGINE_TYPE_UNSPECIFIED,
//		)
//	}
//
//	// Sending a message to myself. No need to send it over the network.
//	// Just put it right into the router. Asynchronously to avoid deadlock.
//	if nodeIDs.Contains(s.ctx.NodeID) {
//		nodeIDs.Remove(s.ctx.NodeID)
//		inMsg := message.InboundGetStateSummaryFrontier(
//			s.ctx.ChainID,
//			requestID,
//			deadline,
//			s.ctx.NodeID,
//		)
//		go s.router.HandleInbound(ctx, inMsg)
//	}
//
//	// Create the outbound message.
//	outMsg, err := s.msgCreator.GetStateSummaryFrontier(
//		s.ctx.ChainID,
//		requestID,
//		deadline,
//	)
//
//	// Send the message over the network.
//	var sentTo set.Set[ids.NodeID]
//	if err == nil {
//		sentTo = s.sender.Send(
//			outMsg,
//			common.SendConfig{
//				NodeIDs: nodeIDs,
//			},
//			s.ctx.SubnetID,
//			s.subnets,
//		)
//	} else {
//		s.ctx.Log.Error("failed to build message",
//			zap.Stringer("messageOp", message.GetStateSummaryFrontierOp),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Duration("deadline", deadline),
//			zap.Error(err),
//		)
//	}
//
//	for nodeID := range nodeIDs {
//		if !sentTo.Contains(nodeID) {
//			s.ctx.Log.Debug("failed to send message",
//				zap.Stringer("messageOp", message.GetStateSummaryFrontierOp),
//				zap.Stringer("nodeID", nodeID),
//				zap.Stringer("chainID", s.ctx.ChainID),
//				zap.Uint32("requestID", requestID),
//			)
//		}
//	}
//}
//
//func (s *sender) SendStateSummaryFrontier(ctx context.Context, nodeID ids.NodeID, requestID uint32, summary []byte) {
//	ctx = context.WithoutCancel(ctx)
//
//	// Sending this message to myself.
//	if nodeID == s.ctx.NodeID {
//		inMsg := message.InboundStateSummaryFrontier(
//			s.ctx.ChainID,
//			requestID,
//			summary,
//			nodeID,
//		)
//		go s.router.HandleInbound(ctx, inMsg)
//		return
//	}
//
//	// Create the outbound message.
//	outMsg, err := s.msgCreator.StateSummaryFrontier(
//		s.ctx.ChainID,
//		requestID,
//		summary,
//	)
//	if err != nil {
//		s.ctx.Log.Error("failed to build message",
//			zap.Stringer("messageOp", message.StateSummaryFrontierOp),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Binary("summaryBytes", summary),
//			zap.Error(err),
//		)
//		return
//	}
//
//	// Send the message over the network.
//	nodeIDs := set.Of(nodeID)
//	sentTo := s.sender.Send(
//		outMsg,
//		common.SendConfig{
//			NodeIDs: nodeIDs,
//		},
//		s.ctx.SubnetID,
//		s.subnets,
//	)
//	if sentTo.Len() == 0 {
//		if s.ctx.Log.Enabled(logging.Verbo) {
//			s.ctx.Log.Verbo("failed to send message",
//				zap.Stringer("messageOp", message.StateSummaryFrontierOp),
//				zap.Stringer("nodeID", nodeID),
//				zap.Stringer("chainID", s.ctx.ChainID),
//				zap.Uint32("requestID", requestID),
//				zap.Binary("summary", summary),
//			)
//		} else {
//			s.ctx.Log.Debug("failed to send message",
//				zap.Stringer("messageOp", message.StateSummaryFrontierOp),
//				zap.Stringer("nodeID", nodeID),
//				zap.Stringer("chainID", s.ctx.ChainID),
//				zap.Uint32("requestID", requestID),
//			)
//		}
//	}
//}
//
//func (s *sender) SendGetAcceptedStateSummary(ctx context.Context, nodeIDs set.Set[ids.NodeID], requestID uint32, heights []uint64) {
//	ctx = context.WithoutCancel(ctx)
//
//	// Note that this timeout duration won't exactly match the one that gets
//	// registered. That's OK.
//	deadline := s.timeouts.TimeoutDuration()
//
//	// Tell the router to expect a response message or a message notifying
//	// that we won't get a response from each of these nodes.
//	// We register timeouts for all nodes, regardless of whether we fail
//	// to send them a message, to avoid busy looping when disconnected from
//	// the internet.
//	for nodeID := range nodeIDs {
//		inMsg := message.InternalGetAcceptedStateSummaryFailed(
//			nodeID,
//			s.ctx.ChainID,
//			requestID,
//		)
//		s.router.RegisterRequest(
//			ctx,
//			nodeID,
//			s.ctx.ChainID,
//			requestID,
//			message.AcceptedStateSummaryOp,
//			inMsg,
//			p2p.EngineType_ENGINE_TYPE_UNSPECIFIED,
//		)
//	}
//
//	// Sending a message to myself. No need to send it over the network.
//	// Just put it right into the router. Asynchronously to avoid deadlock.
//	if nodeIDs.Contains(s.ctx.NodeID) {
//		nodeIDs.Remove(s.ctx.NodeID)
//		inMsg := message.InboundGetAcceptedStateSummary(
//			s.ctx.ChainID,
//			requestID,
//			heights,
//			deadline,
//			s.ctx.NodeID,
//		)
//		go s.router.HandleInbound(ctx, inMsg)
//	}
//
//	// Create the outbound message.
//	outMsg, err := s.msgCreator.GetAcceptedStateSummary(
//		s.ctx.ChainID,
//		requestID,
//		deadline,
//		heights,
//	)
//
//	// Send the message over the network.
//	var sentTo set.Set[ids.NodeID]
//	if err == nil {
//		sentTo = s.sender.Send(
//			outMsg,
//			common.SendConfig{
//				NodeIDs: nodeIDs,
//			},
//			s.ctx.SubnetID,
//			s.subnets,
//		)
//	} else {
//		s.ctx.Log.Error("failed to build message",
//			zap.Stringer("messageOp", message.GetAcceptedStateSummaryOp),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Uint64s("heights", heights),
//			zap.Error(err),
//		)
//	}
//
//	for nodeID := range nodeIDs {
//		if !sentTo.Contains(nodeID) {
//			s.ctx.Log.Debug("failed to send message",
//				zap.Stringer("messageOp", message.GetAcceptedStateSummaryOp),
//				zap.Stringer("nodeID", nodeID),
//				zap.Stringer("chainID", s.ctx.ChainID),
//				zap.Uint32("requestID", requestID),
//				zap.Uint64s("heights", heights),
//			)
//		}
//	}
//}
//
//func (s *sender) SendAcceptedStateSummary(ctx context.Context, nodeID ids.NodeID, requestID uint32, summaryIDs []ids.ID) {
//	ctx = context.WithoutCancel(ctx)
//
//	if nodeID == s.ctx.NodeID {
//		inMsg := message.InboundAcceptedStateSummary(
//			s.ctx.ChainID,
//			requestID,
//			summaryIDs,
//			nodeID,
//		)
//		go s.router.HandleInbound(ctx, inMsg)
//		return
//	}
//
//	// Create the outbound message.
//	outMsg, err := s.msgCreator.AcceptedStateSummary(
//		s.ctx.ChainID,
//		requestID,
//		summaryIDs,
//	)
//	if err != nil {
//		s.ctx.Log.Error("failed to build message",
//			zap.Stringer("messageOp", message.AcceptedStateSummaryOp),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Stringers("summaryIDs", summaryIDs),
//			zap.Error(err),
//		)
//		return
//	}
//
//	// Send the message over the network.
//	nodeIDs := set.Of(nodeID)
//	sentTo := s.sender.Send(
//		outMsg,
//		common.SendConfig{
//			NodeIDs: nodeIDs,
//		},
//		s.ctx.SubnetID,
//		s.subnets,
//	)
//	if sentTo.Len() == 0 {
//		s.ctx.Log.Debug("failed to send message",
//			zap.Stringer("messageOp", message.AcceptedStateSummaryOp),
//			zap.Stringer("nodeID", nodeID),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Stringers("summaryIDs", summaryIDs),
//		)
//	}
//}
//
//func (s *sender) SendGetAcceptedFrontier(ctx context.Context, nodeIDs set.Set[ids.NodeID], requestID uint32) {
//	ctx = context.WithoutCancel(ctx)
//
//	// Note that this timeout duration won't exactly match the one that gets
//	// registered. That's OK.
//	deadline := s.timeouts.TimeoutDuration()
//
//	// Tell the router to expect a response message or a message notifying
//	// that we won't get a response from each of these nodes.
//	// We register timeouts for all nodes, regardless of whether we fail
//	// to send them a message, to avoid busy looping when disconnected from
//	// the internet.
//	for nodeID := range nodeIDs {
//		inMsg := message.InternalGetAcceptedFrontierFailed(
//			nodeID,
//			s.ctx.ChainID,
//			requestID,
//		)
//		s.router.RegisterRequest(
//			ctx,
//			nodeID,
//			s.ctx.ChainID,
//			requestID,
//			message.AcceptedFrontierOp,
//			inMsg,
//			p2p.EngineType_ENGINE_TYPE_UNSPECIFIED,
//		)
//	}
//
//	// Sending a message to myself. No need to send it over the network.
//	// Just put it right into the router. Asynchronously to avoid deadlock.
//	if nodeIDs.Contains(s.ctx.NodeID) {
//		nodeIDs.Remove(s.ctx.NodeID)
//		inMsg := message.InboundGetAcceptedFrontier(
//			s.ctx.ChainID,
//			requestID,
//			deadline,
//			s.ctx.NodeID,
//		)
//		go s.router.HandleInbound(ctx, inMsg)
//	}
//
//	// Create the outbound message.
//	outMsg, err := s.msgCreator.GetAcceptedFrontier(
//		s.ctx.ChainID,
//		requestID,
//		deadline,
//	)
//
//	// Send the message over the network.
//	var sentTo set.Set[ids.NodeID]
//	if err == nil {
//		sentTo = s.sender.Send(
//			outMsg,
//			common.SendConfig{
//				NodeIDs: nodeIDs,
//			},
//			s.ctx.SubnetID,
//			s.subnets,
//		)
//	} else {
//		s.ctx.Log.Error("failed to build message",
//			zap.Stringer("messageOp", message.GetAcceptedFrontierOp),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Duration("deadline", deadline),
//			zap.Error(err),
//		)
//	}
//
//	for nodeID := range nodeIDs {
//		if !sentTo.Contains(nodeID) {
//			s.ctx.Log.Debug("failed to send message",
//				zap.Stringer("messageOp", message.GetAcceptedFrontierOp),
//				zap.Stringer("nodeID", nodeID),
//				zap.Stringer("chainID", s.ctx.ChainID),
//				zap.Uint32("requestID", requestID),
//			)
//		}
//	}
//}
//
//func (s *sender) SendAcceptedFrontier(ctx context.Context, nodeID ids.NodeID, requestID uint32, containerID ids.ID) {
//	ctx = context.WithoutCancel(ctx)
//
//	// Sending this message to myself.
//	if nodeID == s.ctx.NodeID {
//		inMsg := message.InboundAcceptedFrontier(
//			s.ctx.ChainID,
//			requestID,
//			containerID,
//			nodeID,
//		)
//		go s.router.HandleInbound(ctx, inMsg)
//		return
//	}
//
//	// Create the outbound message.
//	outMsg, err := s.msgCreator.AcceptedFrontier(
//		s.ctx.ChainID,
//		requestID,
//		containerID,
//	)
//	if err != nil {
//		s.ctx.Log.Error("failed to build message",
//			zap.Stringer("messageOp", message.AcceptedFrontierOp),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Stringer("containerID", containerID),
//			zap.Error(err),
//		)
//		return
//	}
//
//	// Send the message over the network.
//	nodeIDs := set.Of(nodeID)
//	sentTo := s.sender.Send(
//		outMsg,
//		common.SendConfig{
//			NodeIDs: nodeIDs,
//		},
//		s.ctx.SubnetID,
//		s.subnets,
//	)
//	if sentTo.Len() == 0 {
//		s.ctx.Log.Debug("failed to send message",
//			zap.Stringer("messageOp", message.AcceptedFrontierOp),
//			zap.Stringer("nodeID", nodeID),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Stringer("containerID", containerID),
//		)
//	}
//}
//
//func (s *sender) SendGetAccepted(ctx context.Context, nodeIDs set.Set[ids.NodeID], requestID uint32, containerIDs []ids.ID) {
//	ctx = context.WithoutCancel(ctx)
//
//	// Note that this timeout duration won't exactly match the one that gets
//	// registered. That's OK.
//	deadline := s.timeouts.TimeoutDuration()
//
//	// Tell the router to expect a response message or a message notifying
//	// that we won't get a response from each of these nodes.
//	// We register timeouts for all nodes, regardless of whether we fail
//	// to send them a message, to avoid busy looping when disconnected from
//	// the internet.
//	for nodeID := range nodeIDs {
//		inMsg := message.InternalGetAcceptedFailed(
//			nodeID,
//			s.ctx.ChainID,
//			requestID,
//		)
//		s.router.RegisterRequest(
//			ctx,
//			nodeID,
//			s.ctx.ChainID,
//			requestID,
//			message.AcceptedOp,
//			inMsg,
//			p2p.EngineType_ENGINE_TYPE_UNSPECIFIED,
//		)
//	}
//
//	// Sending a message to myself. No need to send it over the network.
//	// Just put it right into the router. Asynchronously to avoid deadlock.
//	if nodeIDs.Contains(s.ctx.NodeID) {
//		nodeIDs.Remove(s.ctx.NodeID)
//		inMsg := message.InboundGetAccepted(
//			s.ctx.ChainID,
//			requestID,
//			deadline,
//			containerIDs,
//			s.ctx.NodeID,
//		)
//		go s.router.HandleInbound(ctx, inMsg)
//	}
//
//	// Create the outbound message.
//	outMsg, err := s.msgCreator.GetAccepted(
//		s.ctx.ChainID,
//		requestID,
//		deadline,
//		containerIDs,
//	)
//
//	// Send the message over the network.
//	var sentTo set.Set[ids.NodeID]
//	if err == nil {
//		sentTo = s.sender.Send(
//			outMsg,
//			common.SendConfig{
//				NodeIDs: nodeIDs,
//			},
//			s.ctx.SubnetID,
//			s.subnets,
//		)
//	} else {
//		s.ctx.Log.Error("failed to build message",
//			zap.Stringer("messageOp", message.GetAcceptedOp),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Stringers("containerIDs", containerIDs),
//			zap.Error(err),
//		)
//	}
//
//	for nodeID := range nodeIDs {
//		if !sentTo.Contains(nodeID) {
//			s.ctx.Log.Debug("failed to send message",
//				zap.Stringer("messageOp", message.GetAcceptedOp),
//				zap.Stringer("nodeID", nodeID),
//				zap.Stringer("chainID", s.ctx.ChainID),
//				zap.Uint32("requestID", requestID),
//				zap.Stringers("containerIDs", containerIDs),
//			)
//		}
//	}
//}
//
//func (s *sender) SendAccepted(ctx context.Context, nodeID ids.NodeID, requestID uint32, containerIDs []ids.ID) {
//	ctx = context.WithoutCancel(ctx)
//
//	if nodeID == s.ctx.NodeID {
//		inMsg := message.InboundAccepted(
//			s.ctx.ChainID,
//			requestID,
//			containerIDs,
//			nodeID,
//		)
//		go s.router.HandleInbound(ctx, inMsg)
//		return
//	}
//
//	// Create the outbound message.
//	outMsg, err := s.msgCreator.Accepted(s.ctx.ChainID, requestID, containerIDs)
//	if err != nil {
//		s.ctx.Log.Error("failed to build message",
//			zap.Stringer("messageOp", message.AcceptedOp),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Stringers("containerIDs", containerIDs),
//			zap.Error(err),
//		)
//		return
//	}
//
//	// Send the message over the network.
//	nodeIDs := set.Of(nodeID)
//	sentTo := s.sender.Send(
//		outMsg,
//		common.SendConfig{
//			NodeIDs: nodeIDs,
//		},
//		s.ctx.SubnetID,
//		s.subnets,
//	)
//	if sentTo.Len() == 0 {
//		s.ctx.Log.Debug("failed to send message",
//			zap.Stringer("messageOp", message.AcceptedOp),
//			zap.Stringer("nodeID", nodeID),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Stringers("containerIDs", containerIDs),
//		)
//	}
//}
//
//func (s *sender) SendGetAncestors(ctx context.Context, nodeID ids.NodeID, requestID uint32, containerID ids.ID) {
//	ctx = context.WithoutCancel(ctx)
//
//	// Tell the router to expect a response message or a message notifying
//	// that we won't get a response from this node.
//	inMsg := message.InternalGetAncestorsFailed(
//		nodeID,
//		s.ctx.ChainID,
//		requestID,
//		s.engineType,
//	)
//	s.router.RegisterRequest(
//		ctx,
//		nodeID,
//		s.ctx.ChainID,
//		requestID,
//		message.AncestorsOp,
//		inMsg,
//		s.engineType,
//	)
//
//	// Sending a GetAncestors to myself will fail. To avoid constantly sending
//	// myself requests when not connected to any peers, we rely on the timeout
//	// firing to deliver the GetAncestorsFailed message.
//	if nodeID == s.ctx.NodeID {
//		return
//	}
//
//	// [nodeID] may be benched. That is, they've been unresponsive so we don't
//	// even bother sending requests to them. We just have them immediately fail.
//	if s.timeouts.IsBenched(nodeID, s.ctx.ChainID) {
//		s.failedDueToBench.With(prometheus.Labels{
//			opLabel: message.GetAncestorsOp.String(),
//		}).Inc()
//		s.timeouts.RegisterRequestToUnreachableValidator()
//		go s.router.HandleInbound(ctx, inMsg)
//		return
//	}
//
//	// Note that this timeout duration won't exactly match the one that gets
//	// registered. That's OK.
//	deadline := s.timeouts.TimeoutDuration()
//	// Create the outbound message.
//	outMsg, err := s.msgCreator.GetAncestors(
//		s.ctx.ChainID,
//		requestID,
//		deadline,
//		containerID,
//		s.engineType,
//	)
//	if err != nil {
//		s.ctx.Log.Error("failed to build message",
//			zap.Stringer("messageOp", message.GetAncestorsOp),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Stringer("containerID", containerID),
//			zap.Error(err),
//		)
//
//		go s.router.HandleInbound(ctx, inMsg)
//		return
//	}
//
//	// Send the message over the network.
//	nodeIDs := set.Of(nodeID)
//	sentTo := s.sender.Send(
//		outMsg,
//		common.SendConfig{
//			NodeIDs: nodeIDs,
//		},
//		s.ctx.SubnetID,
//		s.subnets,
//	)
//	if sentTo.Len() == 0 {
//		s.ctx.Log.Debug("failed to send message",
//			zap.Stringer("messageOp", message.GetAncestorsOp),
//			zap.Stringer("nodeID", nodeID),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Stringer("containerID", containerID),
//		)
//
//		s.timeouts.RegisterRequestToUnreachableValidator()
//		go s.router.HandleInbound(ctx, inMsg)
//	}
//}
//
//func (s *sender) SendAncestors(_ context.Context, nodeID ids.NodeID, requestID uint32, containers [][]byte) {
//	// Create the outbound message.
//	outMsg, err := s.msgCreator.Ancestors(s.ctx.ChainID, requestID, containers)
//	if err != nil {
//		s.ctx.Log.Error("failed to build message",
//			zap.Stringer("messageOp", message.AncestorsOp),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Int("numContainers", len(containers)),
//			zap.Error(err),
//		)
//		return
//	}
//
//	// Send the message over the network.
//	nodeIDs := set.Of(nodeID)
//	sentTo := s.sender.Send(
//		outMsg,
//		common.SendConfig{
//			NodeIDs: nodeIDs,
//		},
//		s.ctx.SubnetID,
//		s.subnets,
//	)
//	if sentTo.Len() == 0 {
//		s.ctx.Log.Debug("failed to send message",
//			zap.Stringer("messageOp", message.AncestorsOp),
//			zap.Stringer("nodeID", nodeID),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Int("numContainers", len(containers)),
//		)
//	}
//}
//
//func (s *sender) SendGet(ctx context.Context, nodeID ids.NodeID, requestID uint32, containerID ids.ID) {
//	ctx = context.WithoutCancel(ctx)
//
//	// Tell the router to expect a response message or a message notifying
//	// that we won't get a response from this node.
//	inMsg := message.InternalGetFailed(
//		nodeID,
//		s.ctx.ChainID,
//		requestID,
//	)
//	s.router.RegisterRequest(
//		ctx,
//		nodeID,
//		s.ctx.ChainID,
//		requestID,
//		message.PutOp,
//		inMsg,
//		p2p.EngineType_ENGINE_TYPE_UNSPECIFIED,
//	)
//
//	// Sending a Get to myself always fails.
//	if nodeID == s.ctx.NodeID {
//		go s.router.HandleInbound(ctx, inMsg)
//		return
//	}
//
//	// [nodeID] may be benched. That is, they've been unresponsive so we don't
//	// even bother sending requests to them. We just have them immediately fail.
//	if s.timeouts.IsBenched(nodeID, s.ctx.ChainID) {
//		s.failedDueToBench.With(prometheus.Labels{
//			opLabel: message.GetOp.String(),
//		}).Inc()
//		s.timeouts.RegisterRequestToUnreachableValidator()
//		go s.router.HandleInbound(ctx, inMsg)
//		return
//	}
//
//	// Note that this timeout duration won't exactly match the one that gets
//	// registered. That's OK.
//	deadline := s.timeouts.TimeoutDuration()
//	// Create the outbound message.
//	outMsg, err := s.msgCreator.Get(
//		s.ctx.ChainID,
//		requestID,
//		deadline,
//		containerID,
//	)
//
//	// Send the message over the network.
//	var sentTo set.Set[ids.NodeID]
//	if err == nil {
//		nodeIDs := set.Of(nodeID)
//		sentTo = s.sender.Send(
//			outMsg,
//			common.SendConfig{
//				NodeIDs: nodeIDs,
//			},
//			s.ctx.SubnetID,
//			s.subnets,
//		)
//	} else {
//		s.ctx.Log.Error("failed to build message",
//			zap.Stringer("messageOp", message.GetOp),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Duration("deadline", deadline),
//			zap.Stringer("containerID", containerID),
//			zap.Error(err),
//		)
//	}
//
//	if sentTo.Len() == 0 {
//		s.ctx.Log.Debug("failed to send message",
//			zap.Stringer("messageOp", message.GetOp),
//			zap.Stringer("nodeID", nodeID),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Stringer("containerID", containerID),
//		)
//
//		s.timeouts.RegisterRequestToUnreachableValidator()
//		go s.router.HandleInbound(ctx, inMsg)
//	}
//}
//
//func (s *sender) SendPut(_ context.Context, nodeID ids.NodeID, requestID uint32, container []byte) {
//	// Create the outbound message.
//	outMsg, err := s.msgCreator.Put(s.ctx.ChainID, requestID, container)
//	if err != nil {
//		s.ctx.Log.Error("failed to build message",
//			zap.Stringer("messageOp", message.PutOp),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Binary("container", container),
//			zap.Error(err),
//		)
//		return
//	}
//
//	// Send the message over the network.
//	nodeIDs := set.Of(nodeID)
//	sentTo := s.sender.Send(
//		outMsg,
//		common.SendConfig{
//			NodeIDs: nodeIDs,
//		},
//		s.ctx.SubnetID,
//		s.subnets,
//	)
//	if sentTo.Len() == 0 {
//		if s.ctx.Log.Enabled(logging.Verbo) {
//			s.ctx.Log.Verbo("failed to send message",
//				zap.Stringer("messageOp", message.PutOp),
//				zap.Stringer("nodeID", nodeID),
//				zap.Stringer("chainID", s.ctx.ChainID),
//				zap.Uint32("requestID", requestID),
//				zap.Binary("container", container),
//			)
//		} else {
//			s.ctx.Log.Debug("failed to send message",
//				zap.Stringer("messageOp", message.PutOp),
//				zap.Stringer("nodeID", nodeID),
//				zap.Stringer("chainID", s.ctx.ChainID),
//				zap.Uint32("requestID", requestID),
//			)
//		}
//	}
//}
//
//func (s *sender) SendPushQuery(
//	ctx context.Context,
//	nodeIDs set.Set[ids.NodeID],
//	requestID uint32,
//	container []byte,
//	requestedHeight uint64,
//) {
//	ctx = context.WithoutCancel(ctx)
//
//	// Tell the router to expect a response message or a message notifying
//	// that we won't get a response from each of these nodes.
//	// We register timeouts for all nodes, regardless of whether we fail
//	// to send them a message, to avoid busy looping when disconnected from
//	// the internet.
//	for nodeID := range nodeIDs {
//		inMsg := message.InternalQueryFailed(
//			nodeID,
//			s.ctx.ChainID,
//			requestID,
//		)
//		s.router.RegisterRequest(
//			ctx,
//			nodeID,
//			s.ctx.ChainID,
//			requestID,
//			message.ChitsOp,
//			inMsg,
//			p2p.EngineType_ENGINE_TYPE_UNSPECIFIED,
//		)
//	}
//
//	// Note that this timeout duration won't exactly match the one that gets
//	// registered. That's OK.
//	deadline := s.timeouts.TimeoutDuration()
//
//	// Sending a message to myself. No need to send it over the network. Just
//	// put it right into the router. Do so asynchronously to avoid deadlock.
//	if nodeIDs.Contains(s.ctx.NodeID) {
//		nodeIDs.Remove(s.ctx.NodeID)
//		inMsg := message.InboundPushQuery(
//			s.ctx.ChainID,
//			requestID,
//			deadline,
//			container,
//			requestedHeight,
//			s.ctx.NodeID,
//		)
//		go s.router.HandleInbound(ctx, inMsg)
//	}
//
//	// Some of [nodeIDs] may be benched. That is, they've been unresponsive so
//	// we don't even bother sending messages to them. We just have them
//	// immediately fail.
//	for nodeID := range nodeIDs {
//		if s.timeouts.IsBenched(nodeID, s.ctx.ChainID) {
//			s.failedDueToBench.With(prometheus.Labels{
//				opLabel: message.PushQueryOp.String(),
//			}).Inc()
//			nodeIDs.Remove(nodeID)
//			s.timeouts.RegisterRequestToUnreachableValidator()
//
//			// Immediately register a failure. Do so asynchronously to avoid
//			// deadlock.
//			inMsg := message.InternalQueryFailed(
//				nodeID,
//				s.ctx.ChainID,
//				requestID,
//			)
//			go s.router.HandleInbound(ctx, inMsg)
//		}
//	}
//
//	// Create the outbound message.
//	outMsg, err := s.msgCreator.PushQuery(
//		s.ctx.ChainID,
//		requestID,
//		deadline,
//		container,
//		requestedHeight,
//	)
//
//	// Send the message over the network.
//	// [sentTo] are the IDs of validators who may receive the message.
//	var sentTo set.Set[ids.NodeID]
//	if err == nil {
//		sentTo = s.sender.Send(
//			outMsg,
//			common.SendConfig{
//				NodeIDs: nodeIDs,
//			},
//			s.ctx.SubnetID,
//			s.subnets,
//		)
//	} else {
//		s.ctx.Log.Error("failed to build message",
//			zap.Stringer("messageOp", message.PushQueryOp),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Binary("container", container),
//			zap.Uint64("requestedHeight", requestedHeight),
//			zap.Error(err),
//		)
//	}
//
//	for nodeID := range nodeIDs {
//		if !sentTo.Contains(nodeID) {
//			if s.ctx.Log.Enabled(logging.Verbo) {
//				s.ctx.Log.Verbo("failed to send message",
//					zap.Stringer("messageOp", message.PushQueryOp),
//					zap.Stringer("nodeID", nodeID),
//					zap.Stringer("chainID", s.ctx.ChainID),
//					zap.Uint32("requestID", requestID),
//					zap.Binary("container", container),
//					zap.Uint64("requestedHeight", requestedHeight),
//				)
//			} else {
//				s.ctx.Log.Debug("failed to send message",
//					zap.Stringer("messageOp", message.PushQueryOp),
//					zap.Stringer("nodeID", nodeID),
//					zap.Stringer("chainID", s.ctx.ChainID),
//					zap.Uint32("requestID", requestID),
//					zap.Uint64("requestedHeight", requestedHeight),
//				)
//			}
//
//			// Register failures for nodes we didn't send a request to.
//			s.timeouts.RegisterRequestToUnreachableValidator()
//			inMsg := message.InternalQueryFailed(
//				nodeID,
//				s.ctx.ChainID,
//				requestID,
//			)
//			go s.router.HandleInbound(ctx, inMsg)
//		}
//	}
//}
//
//func (s *sender) SendPullQuery(
//	ctx context.Context,
//	nodeIDs set.Set[ids.NodeID],
//	requestID uint32,
//	containerID ids.ID,
//	requestedHeight uint64,
//) {
//	ctx = context.WithoutCancel(ctx)
//
//	// Tell the router to expect a response message or a message notifying
//	// that we won't get a response from each of these nodes.
//	// We register timeouts for all nodes, regardless of whether we fail
//	// to send them a message, to avoid busy looping when disconnected from
//	// the internet.
//	for nodeID := range nodeIDs {
//		inMsg := message.InternalQueryFailed(
//			nodeID,
//			s.ctx.ChainID,
//			requestID,
//		)
//		s.router.RegisterRequest(
//			ctx,
//			nodeID,
//			s.ctx.ChainID,
//			requestID,
//			message.ChitsOp,
//			inMsg,
//			p2p.EngineType_ENGINE_TYPE_UNSPECIFIED,
//		)
//	}
//
//	// Note that this timeout duration won't exactly match the one that gets
//	// registered. That's OK.
//	deadline := s.timeouts.TimeoutDuration()
//
//	// Sending a message to myself. No need to send it over the network. Just
//	// put it right into the router. Do so asynchronously to avoid deadlock.
//	if nodeIDs.Contains(s.ctx.NodeID) {
//		nodeIDs.Remove(s.ctx.NodeID)
//		inMsg := message.InboundPullQuery(
//			s.ctx.ChainID,
//			requestID,
//			deadline,
//			containerID,
//			requestedHeight,
//			s.ctx.NodeID,
//		)
//		go s.router.HandleInbound(ctx, inMsg)
//	}
//
//	// Some of the nodes in [nodeIDs] may be benched. That is, they've been
//	// unresponsive so we don't even bother sending messages to them. We just
//	// have them immediately fail.
//	for nodeID := range nodeIDs {
//		if s.timeouts.IsBenched(nodeID, s.ctx.ChainID) {
//			s.failedDueToBench.With(prometheus.Labels{
//				opLabel: message.PullQueryOp.String(),
//			}).Inc()
//			nodeIDs.Remove(nodeID)
//			s.timeouts.RegisterRequestToUnreachableValidator()
//			// Immediately register a failure. Do so asynchronously to avoid
//			// deadlock.
//			inMsg := message.InternalQueryFailed(
//				nodeID,
//				s.ctx.ChainID,
//				requestID,
//			)
//			go s.router.HandleInbound(ctx, inMsg)
//		}
//	}
//
//	// Create the outbound message.
//	outMsg, err := s.msgCreator.PullQuery(
//		s.ctx.ChainID,
//		requestID,
//		deadline,
//		containerID,
//		requestedHeight,
//	)
//
//	// Send the message over the network.
//	var sentTo set.Set[ids.NodeID]
//	if err == nil {
//		sentTo = s.sender.Send(
//			outMsg,
//			common.SendConfig{
//				NodeIDs: nodeIDs,
//			},
//			s.ctx.SubnetID,
//			s.subnets,
//		)
//	} else {
//		s.ctx.Log.Error("failed to build message",
//			zap.Stringer("messageOp", message.PullQueryOp),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Duration("deadline", deadline),
//			zap.Stringer("containerID", containerID),
//			zap.Uint64("requestedHeight", requestedHeight),
//			zap.Error(err),
//		)
//	}
//
//	for nodeID := range nodeIDs {
//		if !sentTo.Contains(nodeID) {
//			s.ctx.Log.Debug("failed to send message",
//				zap.Stringer("messageOp", message.PullQueryOp),
//				zap.Stringer("nodeID", nodeID),
//				zap.Stringer("chainID", s.ctx.ChainID),
//				zap.Uint32("requestID", requestID),
//				zap.Stringer("containerID", containerID),
//				zap.Uint64("requestedHeight", requestedHeight),
//			)
//
//			// Register failures for nodes we didn't send a request to.
//			s.timeouts.RegisterRequestToUnreachableValidator()
//			inMsg := message.InternalQueryFailed(
//				nodeID,
//				s.ctx.ChainID,
//				requestID,
//			)
//			go s.router.HandleInbound(ctx, inMsg)
//		}
//	}
//}
//
//func (s *sender) SendChits(
//	ctx context.Context,
//	nodeID ids.NodeID,
//	requestID uint32,
//	preferredID ids.ID,
//	preferredIDAtHeight ids.ID,
//	acceptedID ids.ID,
//	acceptedHeight uint64,
//) {
//	ctx = context.WithoutCancel(ctx)
//
//	// If [nodeID] is myself, send this message directly
//	// to my own router rather than sending it over the network
//	if nodeID == s.ctx.NodeID {
//		inMsg := message.InboundChits(
//			s.ctx.ChainID,
//			requestID,
//			preferredID,
//			preferredIDAtHeight,
//			acceptedID,
//			nodeID,
//		)
//		go s.router.HandleInbound(ctx, inMsg)
//		return
//	}
//
//	// Create the outbound message.
//	outMsg, err := s.msgCreator.Chits(s.ctx.ChainID, requestID, preferredID, preferredIDAtHeight, acceptedID, acceptedHeight)
//	if err != nil {
//		s.ctx.Log.Error("failed to build message",
//			zap.Stringer("messageOp", message.ChitsOp),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Stringer("preferredID", preferredID),
//			zap.Stringer("preferredIDAtHeight", preferredIDAtHeight),
//			zap.Stringer("acceptedID", acceptedID),
//			zap.Error(err),
//		)
//		return
//	}
//
//	// Send the message over the network.
//	nodeIDs := set.Of(nodeID)
//	sentTo := s.sender.Send(
//		outMsg,
//		common.SendConfig{
//			NodeIDs: nodeIDs,
//		},
//		s.ctx.SubnetID,
//		s.subnets,
//	)
//	if sentTo.Len() == 0 {
//		s.ctx.Log.Debug("failed to send message",
//			zap.Stringer("messageOp", message.ChitsOp),
//			zap.Stringer("nodeID", nodeID),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Stringer("preferredID", preferredID),
//			zap.Stringer("preferredIDAtHeight", preferredIDAtHeight),
//			zap.Stringer("acceptedID", acceptedID),
//		)
//	}
//}
//
//func (s *sender) SendAppRequest(ctx context.Context, nodeIDs set.Set[ids.NodeID], requestID uint32, appRequestBytes []byte) error {
//	ctx = context.WithoutCancel(ctx)
//
//	// Tell the router to expect a response message or a message notifying
//	// that we won't get a response from each of these nodes.
//	// We register timeouts for all nodes, regardless of whether we fail
//	// to send them a message, to avoid busy looping when disconnected from
//	// the internet.
//	for nodeID := range nodeIDs {
//		inMsg := message.InboundAppError(
//			nodeID,
//			s.ctx.ChainID,
//			requestID,
//			common.ErrTimeout.Code,
//			common.ErrTimeout.Message,
//		)
//		s.router.RegisterRequest(
//			ctx,
//			nodeID,
//			s.ctx.ChainID,
//			requestID,
//			message.AppResponseOp,
//			inMsg,
//			p2p.EngineType_ENGINE_TYPE_UNSPECIFIED,
//		)
//	}
//
//	// Note that this timeout duration won't exactly match the one that gets
//	// registered. That's OK.
//	deadline := s.timeouts.TimeoutDuration()
//
//	// Sending a message to myself. No need to send it over the network. Just
//	// put it right into the router. Do so asynchronously to avoid deadlock.
//	if nodeIDs.Contains(s.ctx.NodeID) {
//		nodeIDs.Remove(s.ctx.NodeID)
//		inMsg := message.InboundAppRequest(
//			s.ctx.ChainID,
//			requestID,
//			deadline,
//			appRequestBytes,
//			s.ctx.NodeID,
//		)
//		go s.router.HandleInbound(ctx, inMsg)
//	}
//
//	// Some of the nodes in [nodeIDs] may be benched. That is, they've been
//	// unresponsive so we don't even bother sending messages to them. We just
//	// have them immediately fail.
//	for nodeID := range nodeIDs {
//		if s.timeouts.IsBenched(nodeID, s.ctx.ChainID) {
//			s.failedDueToBench.With(prometheus.Labels{
//				opLabel: message.AppRequestOp.String(),
//			}).Inc()
//			nodeIDs.Remove(nodeID)
//			s.timeouts.RegisterRequestToUnreachableValidator()
//
//			// Immediately register a failure. Do so asynchronously to avoid
//			// deadlock.
//			inMsg := message.InboundAppError(
//				nodeID,
//				s.ctx.ChainID,
//				requestID,
//				common.ErrTimeout.Code,
//				common.ErrTimeout.Message,
//			)
//			go s.router.HandleInbound(ctx, inMsg)
//		}
//	}
//
//	// Create the outbound message.
//	outMsg, err := s.msgCreator.AppRequest(
//		s.ctx.ChainID,
//		requestID,
//		deadline,
//		appRequestBytes,
//	)
//
//	// Send the message over the network.
//	// [sentTo] are the IDs of nodes who may receive the message.
//	var sentTo set.Set[ids.NodeID]
//	if err == nil {
//		sentTo = s.sender.Send(
//			outMsg,
//			common.SendConfig{
//				NodeIDs: nodeIDs,
//			},
//			s.ctx.SubnetID,
//			s.subnets,
//		)
//	} else {
//		s.ctx.Log.Error("failed to build message",
//			zap.Stringer("messageOp", message.AppRequestOp),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Binary("payload", appRequestBytes),
//			zap.Error(err),
//		)
//	}
//
//	for nodeID := range nodeIDs {
//		if !sentTo.Contains(nodeID) {
//			if s.ctx.Log.Enabled(logging.Verbo) {
//				s.ctx.Log.Verbo("failed to send message",
//					zap.Stringer("messageOp", message.AppRequestOp),
//					zap.Stringer("nodeID", nodeID),
//					zap.Stringer("chainID", s.ctx.ChainID),
//					zap.Uint32("requestID", requestID),
//					zap.Binary("payload", appRequestBytes),
//				)
//			} else {
//				s.ctx.Log.Debug("failed to send message",
//					zap.Stringer("messageOp", message.AppRequestOp),
//					zap.Stringer("nodeID", nodeID),
//					zap.Stringer("chainID", s.ctx.ChainID),
//					zap.Uint32("requestID", requestID),
//				)
//			}
//
//			// Register failures for nodes we didn't send a request to.
//			s.timeouts.RegisterRequestToUnreachableValidator()
//			inMsg := message.InboundAppError(
//				nodeID,
//				s.ctx.ChainID,
//				requestID,
//				common.ErrTimeout.Code,
//				common.ErrTimeout.Message,
//			)
//			go s.router.HandleInbound(ctx, inMsg)
//		}
//	}
//	return nil
//}
//
//func (s *sender) SendAppResponse(ctx context.Context, nodeID ids.NodeID, requestID uint32, appResponseBytes []byte) error {
//	ctx = context.WithoutCancel(ctx)
//
//	if nodeID == s.ctx.NodeID {
//		inMsg := message.InboundAppResponse(
//			s.ctx.ChainID,
//			requestID,
//			appResponseBytes,
//			nodeID,
//		)
//		go s.router.HandleInbound(ctx, inMsg)
//		return nil
//	}
//
//	// Create the outbound message.
//	outMsg, err := s.msgCreator.AppResponse(
//		s.ctx.ChainID,
//		requestID,
//		appResponseBytes,
//	)
//	if err != nil {
//		s.ctx.Log.Error("failed to build message",
//			zap.Stringer("messageOp", message.AppResponseOp),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Binary("payload", appResponseBytes),
//			zap.Error(err),
//		)
//		return nil
//	}
//
//	// Send the message over the network.
//	nodeIDs := set.Of(nodeID)
//	sentTo := s.sender.Send(
//		outMsg,
//		common.SendConfig{
//			NodeIDs: nodeIDs,
//		},
//		s.ctx.SubnetID,
//		s.subnets,
//	)
//	if sentTo.Len() == 0 {
//		if s.ctx.Log.Enabled(logging.Verbo) {
//			s.ctx.Log.Verbo("failed to send message",
//				zap.Stringer("messageOp", message.AppResponseOp),
//				zap.Stringer("nodeID", nodeID),
//				zap.Stringer("chainID", s.ctx.ChainID),
//				zap.Uint32("requestID", requestID),
//				zap.Binary("payload", appResponseBytes),
//			)
//		} else {
//			s.ctx.Log.Debug("failed to send message",
//				zap.Stringer("messageOp", message.AppResponseOp),
//				zap.Stringer("nodeID", nodeID),
//				zap.Stringer("chainID", s.ctx.ChainID),
//				zap.Uint32("requestID", requestID),
//			)
//		}
//	}
//	return nil
//}
//
//func (s *sender) SendAppError(ctx context.Context, nodeID ids.NodeID, requestID uint32, errorCode int32, errorMessage string) error {
//	ctx = context.WithoutCancel(ctx)
//
//	if nodeID == s.ctx.NodeID {
//		inMsg := message.InboundAppError(
//			nodeID,
//			s.ctx.ChainID,
//			requestID,
//			errorCode,
//			errorMessage,
//		)
//		go s.router.HandleInbound(ctx, inMsg)
//		return nil
//	}
//
//	// Create the outbound message.
//	outMsg, err := s.msgCreator.AppError(
//		s.ctx.ChainID,
//		requestID,
//		errorCode,
//		errorMessage,
//	)
//	if err != nil {
//		s.ctx.Log.Error("failed to build message",
//			zap.Stringer("messageOp", message.AppErrorOp),
//			zap.Stringer("nodeID", nodeID),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Uint32("requestID", requestID),
//			zap.Int32("errorCode", errorCode),
//			zap.String("errorMessage", errorMessage),
//			zap.Error(err),
//		)
//		return nil
//	}
//
//	// Send the message over the network.
//	sentTo := s.sender.Send(
//		outMsg,
//		common.SendConfig{
//			NodeIDs: set.Of(nodeID),
//		},
//		s.ctx.SubnetID,
//		s.subnets,
//	)
//	if sentTo.Len() == 0 {
//		if s.ctx.Log.Enabled(logging.Verbo) {
//			s.ctx.Log.Verbo("failed to send message",
//				zap.Stringer("messageOp", message.AppErrorOp),
//				zap.Stringer("nodeID", nodeID),
//				zap.Stringer("chainID", s.ctx.ChainID),
//				zap.Uint32("requestID", requestID),
//				zap.Int32("errorCode", errorCode),
//				zap.String("errorMessage", errorMessage),
//			)
//		} else {
//			s.ctx.Log.Debug("failed to send message",
//				zap.Stringer("messageOp", message.AppErrorOp),
//				zap.Stringer("nodeID", nodeID),
//				zap.Stringer("chainID", s.ctx.ChainID),
//				zap.Uint32("requestID", requestID),
//				zap.Int32("errorCode", errorCode),
//				zap.String("errorMessage", errorMessage),
//			)
//		}
//	}
//	return nil
//}
//
//func (s *sender) SendAppGossip(
//	_ context.Context,
//	config common.SendConfig,
//	appGossipBytes []byte,
//) error {
//	// Create the outbound message.
//	outMsg, err := s.msgCreator.AppGossip(s.ctx.ChainID, appGossipBytes)
//	if err != nil {
//		s.ctx.Log.Error("failed to build message",
//			zap.Stringer("messageOp", message.AppGossipOp),
//			zap.Stringer("chainID", s.ctx.ChainID),
//			zap.Binary("payload", appGossipBytes),
//			zap.Error(err),
//		)
//		return nil
//	}
//
//	sentTo := s.sender.Send(
//		outMsg,
//		config,
//		s.ctx.SubnetID,
//		s.subnets,
//	)
//	if sentTo.Len() == 0 {
//		if s.ctx.Log.Enabled(logging.Verbo) {
//			s.ctx.Log.Verbo("failed to send message",
//				zap.Stringer("messageOp", message.AppGossipOp),
//				zap.Stringer("chainID", s.ctx.ChainID),
//				zap.Binary("payload", appGossipBytes),
//			)
//		} else {
//			s.ctx.Log.Debug("failed to send message",
//				zap.Stringer("messageOp", message.AppGossipOp),
//				zap.Stringer("chainID", s.ctx.ChainID),
//			)
//		}
//	}
//	return nil
//}
