// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package message

import (
	"github.com/cometbft/cometbft/libs/log"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/landslidenetwork/slide-sdk/proto/p2p"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/compression"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

func Test_newMsgBuilder(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	mb, err := newMsgBuilder(
		log.NewNopLogger(),
		prometheus.NewRegistry(),
		10*time.Second,
	)
	require.NoError(err)
	require.NotNil(mb)
}

func TestInboundMsgBuilder(t *testing.T) {
	var (
		chainID          = ids.GenerateTestID()
		requestID uint32 = 12345
		deadline         = time.Hour
		nodeID           = ids.GenerateTestNodeID()
		//summary                    = []byte{9, 8, 7}
		appBytes = []byte{1, 3, 3, 7}
		//container                  = []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}
		//containerIDs               = []ids.ID{ids.GenerateTestID(), ids.GenerateTestID()}
		//requestedHeight     uint64 = 999
		//acceptedContainerID        = ids.GenerateTestID()
		//summaryIDs                 = []ids.ID{ids.GenerateTestID(), ids.GenerateTestID()}
		//heights                    = []uint64{1000, 2000}
	)

	t.Run(
		"InboundAppRequest",
		func(t *testing.T) {
			require := require.New(t)

			start := time.Now()
			msg := InboundAppRequest(
				chainID,
				requestID,
				deadline,
				appBytes,
				nodeID,
			)
			end := time.Now()

			require.Equal(AppRequestOp, msg.Op())
			require.Equal(nodeID, msg.NodeID())
			require.False(msg.Expiration().Before(start.Add(deadline)))
			require.False(end.Add(deadline).Before(msg.Expiration()))
			require.IsType(&p2p.AppRequest{}, msg.Message())
			innerMsg := msg.Message().(*p2p.AppRequest)
			require.Equal(chainID[:], innerMsg.ChainId)
			require.Equal(requestID, innerMsg.RequestId)
			require.Equal(appBytes, innerMsg.AppBytes)
		},
	)

	//TODO: implement
	//t.Run(
	//	"InboundAppResponse",
	//	func(t *testing.T) {
	//		require := require.New(t)
	//
	//		msg := InboundAppResponse(
	//			chainID,
	//			requestID,
	//			appBytes,
	//			nodeID,
	//		)
	//
	//		require.Equal(AppResponseOp, msg.Op())
	//		require.Equal(nodeID, msg.NodeID())
	//		require.Equal(mockable.MaxTime, msg.Expiration())
	//		require.IsType(&p2p.AppResponse{}, msg.Message())
	//		innerMsg := msg.Message().(*p2p.AppResponse)
	//		require.Equal(chainID[:], innerMsg.ChainId)
	//		require.Equal(requestID, innerMsg.RequestId)
	//		require.Equal(appBytes, innerMsg.AppBytes)
	//	},
	//)
}

func TestAppError(t *testing.T) {
	require := require.New(t)

	mb, err := newMsgBuilder(
		log.NewNopLogger(),
		prometheus.NewRegistry(),
		time.Second,
	)
	require.NoError(err)

	nodeID := ids.GenerateTestNodeID()
	chainID := ids.GenerateTestID()
	requestID := uint32(1)
	errorCode := int32(2)
	errorMessage := "hello world"

	want := &p2p.Message{
		Message: &p2p.Message_AppError{
			AppError: &p2p.AppError{
				ChainId:      chainID[:],
				RequestId:    requestID,
				ErrorCode:    errorCode,
				ErrorMessage: errorMessage,
			},
		},
	}

	outMsg, err := mb.createOutbound(want, compression.TypeNone, false)
	require.NoError(err)

	got, err := mb.parseInbound(outMsg.Bytes(), nodeID, func() {})
	require.NoError(err)

	require.Equal(nodeID, got.NodeID())
	require.Equal(AppErrorOp, got.Op())

	msg, ok := got.Message().(*p2p.AppError)
	require.True(ok)
	require.Equal(errorCode, msg.ErrorCode)
	require.Equal(errorMessage, msg.ErrorMessage)
}
