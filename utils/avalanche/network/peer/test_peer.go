// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package peer

import (
	"context"
	"crypto"
	"log"
	"net"
	"net/netip"
	"time"

	tmlog "github.com/cometbft/cometbft/libs/log"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/landslidenetwork/slide-sdk/utils"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/constants"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/message"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/networking/router"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/uptime"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/validators"
	"github.com/landslidenetwork/slide-sdk/utils/crypto/bls"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	"github.com/landslidenetwork/slide-sdk/utils/set"
	"github.com/landslidenetwork/slide-sdk/utils/staking"
	"github.com/landslidenetwork/slide-sdk/utils/version"
)

const maxMessageToSend = 1024

var (
	InitiallyActiveTime = time.Date(2020, time.December, 5, 5, 0, 0, 0, time.UTC)
)

// StartTestPeer provides a simple interface to create a peer that has finished
// the p2p handshake.
//
// This function will generate a new TLS key to use when connecting to the peer.
//
// The returned peer will not throttle inbound or outbound messages.
//
//   - [ctx] provides a way of canceling the connection request.
//   - [ip] is the remote that will be dialed to create the connection.
//   - [networkID] will be sent to the peer during the handshake. If the peer is
//     expecting a different [networkID], the handshake will fail and an error
//     will be returned.
//   - [router] will be called with all non-handshake messages received by the
//     peer.
func StartTestPeer(
	ctx context.Context,
	ip netip.AddrPort,
	networkID uint32,
	router router.InboundHandler,
) (Peer, error) {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, constants.NetworkType, ip.String())
	if err != nil {
		return nil, err
	}

	tlsCert, err := staking.NewTLSCert()
	if err != nil {
		return nil, err
	}

	tlsConfg := TLSConfig(*tlsCert, nil)
	clientUpgrader := NewTLSClientUpgrader(
		tlsConfg,
		prometheus.NewCounter(prometheus.CounterOpts{}),
	)

	peerID, conn, cert, err := clientUpgrader.Upgrade(conn)
	if err != nil {
		return nil, err
	}

	mc, err := message.NewCreator(
		tmlog.NewNopLogger(),
		prometheus.NewRegistry(),
		constants.DefaultNetworkCompressionType,
		10*time.Second,
	)
	if err != nil {
		return nil, err
	}

	tlsKey := tlsCert.PrivateKey.(crypto.Signer)
	blsKey, err := bls.NewSigner()
	if err != nil {
		return nil, err
	}

	var onFailure SendFailedFunc = func(msg message.OutboundMessage) {
		log.Fatal("Failed to send message:", msg)
	}

	peer := Start(
		&Config{
			MessageCreator:       mc,
			Log:                  tmlog.NewNopLogger(),
			Network:              TestNetwork,
			Router:               router,
			VersionCompatibility: version.GetCompatibility(InitiallyActiveTime),
			MySubnets:            set.Set[ids.ID]{},
			// Beacons:              validators.NewManager(),
			Validators:         validators.NewManager(),
			NetworkID:          networkID,
			PingFrequency:      constants.DefaultPingFrequency,
			PongTimeout:        constants.DefaultPingPongTimeout,
			MaxClockDifference: time.Minute,
			UptimeCalculator:   uptime.NoOpCalculator,
			IPSigner: NewIPSigner(
				utils.NewAtomic(netip.AddrPortFrom(
					netip.IPv6Loopback(),
					1,
				)),
				tlsKey,
				blsKey,
			),
		},
		conn,
		cert,
		peerID,
		NewBlockingMessageQueue(
			onFailure,
			tmlog.NewNopLogger(),
			maxMessageToSend,
		),
	)
	return peer, peer.AwaitReady(ctx)
}
