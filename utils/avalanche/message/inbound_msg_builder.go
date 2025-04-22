// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package message

import (
	"time"

	"github.com/landslidenetwork/slide-sdk/proto/p2p"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/timer/mockable"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

var _ InboundMsgBuilder = (*inMsgBuilder)(nil)

type InboundMsgBuilder interface {
	// Parse reads given bytes as InboundMessage
	Parse(
		bytes []byte,
		nodeID ids.NodeID,
		onFinishedHandling func(),
	) (InboundMessage, error)
}

type inMsgBuilder struct {
	builder *msgBuilder
}

func newInboundBuilder(builder *msgBuilder) InboundMsgBuilder {
	return &inMsgBuilder{
		builder: builder,
	}
}

func (b *inMsgBuilder) Parse(bytes []byte, nodeID ids.NodeID, onFinishedHandling func()) (InboundMessage, error) {
	return b.builder.parseInbound(bytes, nodeID, onFinishedHandling)
}

func InboundAppRequest(
	chainID ids.ID,
	requestID uint32,
	deadline time.Duration,
	msg []byte,
	nodeID ids.NodeID,
) InboundMessage {
	return &inboundMessage{
		nodeID: nodeID,
		op:     AppRequestOp,
		message: &p2p.AppRequest{
			ChainId:   chainID[:],
			RequestId: requestID,
			Deadline:  uint64(deadline),
			AppBytes:  msg,
		},
		expiration: time.Now().Add(deadline),
	}
}

func InboundAppError(
	nodeID ids.NodeID,
	chainID ids.ID,
	requestID uint32,
	errorCode int32,
	errorMessage string,
) InboundMessage {
	return &inboundMessage{
		nodeID: nodeID,
		op:     AppErrorOp,
		message: &p2p.AppError{
			ChainId:      chainID[:],
			RequestId:    requestID,
			ErrorCode:    errorCode,
			ErrorMessage: errorMessage,
		},
		expiration: mockable.MaxTime,
	}
}

func InboundAppResponse(
	chainID ids.ID,
	requestID uint32,
	msg []byte,
	nodeID ids.NodeID,
) InboundMessage {
	return &inboundMessage{
		nodeID: nodeID,
		op:     AppResponseOp,
		message: &p2p.AppResponse{
			ChainId:   chainID[:],
			RequestId: requestID,
			AppBytes:  msg,
		},
		expiration: mockable.MaxTime,
	}
}

func encodeIDs(ids []ids.ID, result [][]byte) {
	for i, id := range ids {
		id := id
		result[i] = id[:]
	}
}
