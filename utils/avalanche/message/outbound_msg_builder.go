// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package message

import (
	"github.com/landslidenetwork/slide-sdk/utils/ips"
	"net/netip"
	"time"

	"github.com/landslidenetwork/slide-sdk/proto/p2p"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/compression"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

var _ OutboundMsgBuilder = (*outMsgBuilder)(nil)

// OutboundMsgBuilder builds outbound messages. Outbound messages are returned
// with a reference count of 1. Once the reference count hits 0, the message
// bytes should no longer be accessed.
type OutboundMsgBuilder interface {
	Handshake(
		networkID uint32,
		myTime uint64,
		ip netip.AddrPort,
		client string,
		major uint32,
		minor uint32,
		patch uint32,
		ipSigningTime uint64,
		ipNodeIDSig []byte,
		ipBLSSig []byte,
		trackedSubnets []ids.ID,
		supportedACPs []uint32,
		objectedACPs []uint32,
		knownPeersFilter []byte,
		knownPeersSalt []byte,
		requestAllSubnetIPs bool,
	) (OutboundMessage, error)

	GetPeerList(
		knownPeersFilter []byte,
		knownPeersSalt []byte,
		requestAllSubnetIPs bool,
	) (OutboundMessage, error)

	PeerList(
		peers []*ips.ClaimedIPPort,
		bypassThrottling bool,
	) (OutboundMessage, error)

	Ping(
		primaryUptime uint32,
	) (OutboundMessage, error)

	Pong() (OutboundMessage, error)

	AppRequest(
		chainID ids.ID,
		requestID uint32,
		deadline time.Duration,
		msg []byte,
	) (OutboundMessage, error)

	AppResponse(
		chainID ids.ID,
		requestID uint32,
		msg []byte,
	) (OutboundMessage, error)

	AppError(
		chainID ids.ID,
		requestID uint32,
		errorCode int32,
		errorMessage string,
	) (OutboundMessage, error)

	AppGossip(
		chainID ids.ID,
		msg []byte,
	) (OutboundMessage, error)
}

type outMsgBuilder struct {
	compressionType compression.Type

	builder *msgBuilder
}

// Use "message.NewCreator" to import this function
// since we do not expose "msgBuilder" yet
func newOutboundBuilder(compressionType compression.Type, builder *msgBuilder) OutboundMsgBuilder {
	return &outMsgBuilder{
		compressionType: compressionType,
		builder:         builder,
	}
}

func (b *outMsgBuilder) Ping(
	primaryUptime uint32,
) (OutboundMessage, error) {
	return b.builder.createOutbound(
		&p2p.Message{
			Message: &p2p.Message_Ping{
				Ping: &p2p.Ping{
					Uptime: primaryUptime,
				},
			},
		},
		compression.TypeNone,
		false,
	)
}

func (b *outMsgBuilder) Pong() (OutboundMessage, error) {
	return b.builder.createOutbound(
		&p2p.Message{
			Message: &p2p.Message_Pong{
				Pong: &p2p.Pong{},
			},
		},
		compression.TypeNone,
		false,
	)
}

func (b *outMsgBuilder) Handshake(
	networkID uint32,
	myTime uint64,
	ip netip.AddrPort,
	client string,
	major uint32,
	minor uint32,
	patch uint32,
	ipSigningTime uint64,
	ipNodeIDSig []byte,
	ipBLSSig []byte,
	trackedSubnets []ids.ID,
	supportedACPs []uint32,
	objectedACPs []uint32,
	knownPeersFilter []byte,
	knownPeersSalt []byte,
	requestAllSubnetIPs bool,
) (OutboundMessage, error) {
	subnetIDBytes := make([][]byte, len(trackedSubnets))
	encodeIDs(trackedSubnets, subnetIDBytes)
	// TODO: Use .AsSlice() after v1.12.x activates.
	addr := ip.Addr().As16()
	return b.builder.createOutbound(
		&p2p.Message{
			Message: &p2p.Message_Handshake{
				Handshake: &p2p.Handshake{
					NetworkId:      networkID,
					MyTime:         myTime,
					IpAddr:         addr[:],
					IpPort:         uint32(ip.Port()),
					IpSigningTime:  ipSigningTime,
					IpNodeIdSig:    ipNodeIDSig,
					TrackedSubnets: subnetIDBytes,
					Client: &p2p.Client{
						Name:  client,
						Major: major,
						Minor: minor,
						Patch: patch,
					},
					SupportedAcps: supportedACPs,
					ObjectedAcps:  objectedACPs,
					KnownPeers: &p2p.BloomFilter{
						Filter: knownPeersFilter,
						Salt:   knownPeersSalt,
					},
					IpBlsSig:   ipBLSSig,
					AllSubnets: requestAllSubnetIPs,
				},
			},
		},
		compression.TypeNone,
		true,
	)
}

func (b *outMsgBuilder) GetPeerList(
	knownPeersFilter []byte,
	knownPeersSalt []byte,
	requestAllSubnetIPs bool,
) (OutboundMessage, error) {
	return b.builder.createOutbound(
		&p2p.Message{
			Message: &p2p.Message_GetPeerList{
				GetPeerList: &p2p.GetPeerList{
					KnownPeers: &p2p.BloomFilter{
						Filter: knownPeersFilter,
						Salt:   knownPeersSalt,
					},
					AllSubnets: requestAllSubnetIPs,
				},
			},
		},
		b.compressionType,
		false,
	)
}

func (b *outMsgBuilder) PeerList(peers []*ips.ClaimedIPPort, bypassThrottling bool) (OutboundMessage, error) {
	claimIPPorts := make([]*p2p.ClaimedIpPort, len(peers))
	for i, p := range peers {
		// TODO: Use .AsSlice() after v1.12.x activates.
		ip := p.AddrPort.Addr().As16()
		claimIPPorts[i] = &p2p.ClaimedIpPort{
			X509Certificate: p.Cert.Raw,
			IpAddr:          ip[:],
			IpPort:          uint32(p.AddrPort.Port()),
			Timestamp:       p.Timestamp,
			Signature:       p.Signature,
			TxId:            ids.Empty[:],
		}
	}
	return b.builder.createOutbound(
		&p2p.Message{
			Message: &p2p.Message_PeerList_{
				PeerList_: &p2p.PeerList{
					ClaimedIpPorts: claimIPPorts,
				},
			},
		},
		b.compressionType,
		bypassThrottling,
	)
}

func (b *outMsgBuilder) AppRequest(
	chainID ids.ID,
	requestID uint32,
	deadline time.Duration,
	msg []byte,
) (OutboundMessage, error) {
	return b.builder.createOutbound(
		&p2p.Message{
			Message: &p2p.Message_AppRequest{
				AppRequest: &p2p.AppRequest{
					ChainId:   chainID[:],
					RequestId: requestID,
					Deadline:  uint64(deadline),
					AppBytes:  msg,
				},
			},
		},
		b.compressionType,
		false,
	)
}

func (b *outMsgBuilder) AppResponse(chainID ids.ID, requestID uint32, msg []byte) (OutboundMessage, error) {
	return b.builder.createOutbound(
		&p2p.Message{
			Message: &p2p.Message_AppResponse{
				AppResponse: &p2p.AppResponse{
					ChainId:   chainID[:],
					RequestId: requestID,
					AppBytes:  msg,
				},
			},
		},
		b.compressionType,
		false,
	)
}

func (b *outMsgBuilder) AppError(chainID ids.ID, requestID uint32, errorCode int32, errorMessage string) (OutboundMessage, error) {
	return b.builder.createOutbound(
		&p2p.Message{
			Message: &p2p.Message_AppError{
				AppError: &p2p.AppError{
					ChainId:      chainID[:],
					RequestId:    requestID,
					ErrorCode:    errorCode,
					ErrorMessage: errorMessage,
				},
			},
		},
		b.compressionType,
		false,
	)
}

func (b *outMsgBuilder) AppGossip(chainID ids.ID, msg []byte) (OutboundMessage, error) {
	return b.builder.createOutbound(
		&p2p.Message{
			Message: &p2p.Message_AppGossip{
				AppGossip: &p2p.AppGossip{
					ChainId:  chainID[:],
					AppBytes: msg,
				},
			},
		},
		b.compressionType,
		false,
	)
}
