// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package message

import (
	"bytes"
	"fmt"
	"github.com/cometbft/cometbft/libs/log"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/landslidenetwork/slide-sdk/proto/p2p"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/compression"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

func TestMessage(t *testing.T) {
	t.Parallel()

	mb, err := newMsgBuilder(
		log.NewNopLogger(),
		prometheus.NewRegistry(),
		5*time.Second,
	)
	require.NoError(t, err)

	testID := ids.GenerateTestID()
	compressibleContainers := [][]byte{
		bytes.Repeat([]byte{0}, 100),
		bytes.Repeat([]byte{0}, 32),
		bytes.Repeat([]byte{0}, 32),
	}

	tests := []struct {
		desc             string
		op               Op
		msg              *p2p.Message
		compressionType  compression.Type
		bypassThrottling bool
		bytesSaved       bool // if true, outbound message saved bytes must be non-zero
	}{
		{
			desc: "ping message with no compression no uptime",
			op:   PingOp,
			msg: &p2p.Message{
				Message: &p2p.Message_Ping{
					Ping: &p2p.Ping{},
				},
			},
			compressionType:  compression.TypeNone,
			bypassThrottling: true,
			bytesSaved:       false,
		},
		{
			desc: "pong message with no compression",
			op:   PongOp,
			msg: &p2p.Message{
				Message: &p2p.Message_Pong{
					Pong: &p2p.Pong{},
				},
			},
			compressionType:  compression.TypeNone,
			bypassThrottling: true,
			bytesSaved:       false,
		},
		{
			desc: "ping message with no compression and uptime",
			op:   PingOp,
			msg: &p2p.Message{
				Message: &p2p.Message_Ping{
					Ping: &p2p.Ping{
						Uptime: 100,
					},
				},
			},
			compressionType:  compression.TypeNone,
			bypassThrottling: true,
			bytesSaved:       false,
		},
		{
			desc: "app_request message with no compression",
			op:   AppRequestOp,
			msg: &p2p.Message{
				Message: &p2p.Message_AppRequest{
					AppRequest: &p2p.AppRequest{
						ChainId:   testID[:],
						RequestId: 1,
						Deadline:  1,
						AppBytes:  compressibleContainers[0],
					},
				},
			},
			compressionType:  compression.TypeNone,
			bypassThrottling: true,
			bytesSaved:       false,
		},
		{
			desc: "app_request message with zstd compression",
			op:   AppRequestOp,
			msg: &p2p.Message{
				Message: &p2p.Message_AppRequest{
					AppRequest: &p2p.AppRequest{
						ChainId:   testID[:],
						RequestId: 1,
						Deadline:  1,
						AppBytes:  compressibleContainers[0],
					},
				},
			},
			compressionType:  compression.TypeZstd,
			bypassThrottling: true,
			bytesSaved:       true,
		},
		{
			desc: "app_response message with no compression",
			op:   AppResponseOp,
			msg: &p2p.Message{
				Message: &p2p.Message_AppResponse{
					AppResponse: &p2p.AppResponse{
						ChainId:   testID[:],
						RequestId: 1,
						AppBytes:  compressibleContainers[0],
					},
				},
			},
			compressionType:  compression.TypeNone,
			bypassThrottling: true,
			bytesSaved:       false,
		},
		{
			desc: "app_response message with zstd compression",
			op:   AppResponseOp,
			msg: &p2p.Message{
				Message: &p2p.Message_AppResponse{
					AppResponse: &p2p.AppResponse{
						ChainId:   testID[:],
						RequestId: 1,
						AppBytes:  compressibleContainers[0],
					},
				},
			},
			compressionType:  compression.TypeZstd,
			bypassThrottling: true,
			bytesSaved:       true,
		},
	}

	for _, tv := range tests {
		t.Run(tv.desc, func(t *testing.T) {
			require := require.New(t)

			encodedMsg, err := mb.createOutbound(tv.msg, tv.compressionType, tv.bypassThrottling)
			require.NoError(err)

			require.Equal(tv.bypassThrottling, encodedMsg.BypassThrottling())
			require.Equal(tv.op, encodedMsg.Op())

			if bytesSaved := encodedMsg.BytesSavedCompression(); tv.bytesSaved {
				require.Positive(bytesSaved)
			}

			parsedMsg, err := mb.parseInbound(encodedMsg.Bytes(), ids.EmptyNodeID, func() {})
			require.NoError(err)
			require.Equal(tv.op, parsedMsg.Op())
		})
	}
}

// Tests the Stringer interface on inbound messages
func TestInboundMessageToString(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	mb, err := newMsgBuilder(
		log.NewNopLogger(),
		prometheus.NewRegistry(),
		5*time.Second,
	)
	require.NoError(err)

	// msg that will become the tested InboundMessage
	msg := &p2p.Message{
		Message: &p2p.Message_Pong{
			Pong: &p2p.Pong{},
		},
	}
	msgBytes, err := proto.Marshal(msg)
	require.NoError(err)

	inboundMsg, err := mb.parseInbound(msgBytes, ids.EmptyNodeID, func() {})
	require.NoError(err)

	require.Equal("NodeID-111111111111111111116DBWJs Op: pong Message: ", inboundMsg.String())

	require.Equal("NodeID-111111111111111111116DBWJs Op: get_state_summary_frontier_failed Message: ChainID: 11111111111111111111111111111111LpoYY RequestID: 1", fmt.Sprintf("%s Op: %s Message: %s",
		ids.EmptyNodeID, GetStateSummaryFrontierFailedOp, fmt.Sprintf(
			"ChainID: %s RequestID: %d",
			"11111111111111111111111111111111LpoYY", 1,
		)))
}

func TestEmptyInboundMessage(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	mb, err := newMsgBuilder(
		log.NewNopLogger(),
		prometheus.NewRegistry(),
		5*time.Second,
	)
	require.NoError(err)

	msg := &p2p.Message{}
	msgBytes, err := proto.Marshal(msg)
	require.NoError(err)

	_, err = mb.parseInbound(msgBytes, ids.EmptyNodeID, func() {})
	require.ErrorIs(err, errUnknownMessageType)
}

func TestNilInboundMessage(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	mb, err := newMsgBuilder(
		log.NewNopLogger(),
		prometheus.NewRegistry(),
		5*time.Second,
	)
	require.NoError(err)

	msg := &p2p.Message{
		Message: &p2p.Message_Ping{
			Ping: nil,
		},
	}
	msgBytes, err := proto.Marshal(msg)
	require.NoError(err)

	parsedMsg, err := mb.parseInbound(msgBytes, ids.EmptyNodeID, func() {})
	require.NoError(err)

	require.IsType(&p2p.Ping{}, parsedMsg.message)
	pingMsg := parsedMsg.message.(*p2p.Ping)
	require.NotNil(pingMsg)
}
