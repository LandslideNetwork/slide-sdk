// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package network

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/common"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/constants"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/message"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/network/dialer"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/network/peer"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/networking/router"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/networking/sender"
	"github.com/landslidenetwork/slide-sdk/utils/bloom"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	"github.com/landslidenetwork/slide-sdk/utils/ips"
	safemath "github.com/landslidenetwork/slide-sdk/utils/math"
	"github.com/landslidenetwork/slide-sdk/utils/set"
	"github.com/landslidenetwork/slide-sdk/utils/subnets"
	"github.com/landslidenetwork/slide-sdk/utils/version"
	"github.com/landslidenetwork/slide-sdk/utils/wrappers"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

var (
	_ Network = (*network)(nil)

	errNotValidator = errors.New("node is not a validator")
)

// Network defines the functionality of the networking library.
type Network interface {
	// All consensus messages can be sent through this interface. Thread safety
	// must be managed internally in the network.
	sender.ExternalSender
	peer.Network

	// StartClose this network and all existing connections it has. Calling
	// StartClose multiple times is handled gracefully.
	StartClose()

	// Should only be called once, will run until either a fatal error occurs,
	// or the network is closed.
	Dispatch() error

	// Attempt to connect to this IP. The network will never stop attempting to
	// connect to this ID.
	ManuallyTrack(nodeID ids.NodeID, ip netip.AddrPort)

	// PeerInfo returns information about peers. If [nodeIDs] is empty, returns
	// info about all peers that have finished the handshake. Otherwise, returns
	// info about the peers in [nodeIDs] that have finished the handshake.
	PeerInfo(nodeIDs []ids.NodeID) []peer.Info

	// NodeUptime returns given node's primary network UptimeResults in the view of
	// this node's peer validators.
	NodeUptime() (UptimeResult, error)
}

type UptimeResult struct {
	// RewardingStakePercentage shows what percent of network stake thinks we're
	// above the uptime requirement.
	RewardingStakePercentage float64

	// WeightedAveragePercentage is the average perceived uptime of this node,
	// weighted by stake.
	// Note that this is different from RewardingStakePercentage, which shows
	// the percent of the network stake that thinks this node is above the
	// uptime requirement. WeightedAveragePercentage is weighted by uptime.
	// i.e If uptime requirement is 85 and a peer reports 40 percent it will be
	// counted (40*weight) in WeightedAveragePercentage but not in
	// RewardingStakePercentage since 40 < 85
	WeightedAveragePercentage float64
}

// To avoid potential deadlocks, we maintain that locks must be grabbed in the
// following order:
//
// 1. peersLock
// 2. manuallyTrackedIDsLock
//
// If a higher lock (e.g. manuallyTrackedIDsLock) is held when trying to grab a
// lower lock (e.g. peersLock) a deadlock could occur.
type network struct {
	config          *Config
	peerConfig      *peer.Config
	tlsConnRejected prometheus.Counter
	// Listens for and accepts new inbound connections
	listener net.Listener
	// Makes new outbound connections
	dialer dialer.Dialer
	// Does TLS handshakes for inbound connections
	serverUpgrader peer.Upgrader
	// Does TLS handshakes for outbound connections
	clientUpgrader peer.Upgrader
	//
	// ensures the close of the network only happens once.
	closeOnce sync.Once
	// Cancelled on close
	onCloseCtx context.Context
	// Call [onCloseCtxCancel] to cancel [onCloseCtx] during close()
	onCloseCtxCancel context.CancelFunc

	sendFailRateCalculator safemath.Averager
	//
	//	// Tracks which peers know about which peers
	ipTracker *ipTracker
	peersLock sync.RWMutex
	// trackedIPs contains the set of IPs that we are currently attempting to
	// connect to. An entry is added to this set when we first start attempting
	// to connect to the peer. An entry is deleted from this set once we have
	// finished the handshake.
	trackedIPs      map[ids.NodeID]*trackedIP
	connectingPeers peer.Set
	connectedPeers  peer.Set
	closing         bool

	// router is notified about all peer [Connected] and [Disconnected] events
	// as well as all non-handshake peer messages.
	//
	// It is ensured that [Connected] and [Disconnected] are called in
	// consistent ways. Specifically, the a peer starts in the disconnected
	// state and the network can change the peer's state from disconnected to
	// connected and back.
	//
	// It is ensured that [HandleInbound] is only called with a message from a
	// peer that is in the connected state.
	//
	// It is expected that the implementation of this interface can handle
	// concurrent calls to [Connected], [Disconnected], and [HandleInbound].
	router router.ExternalHandler
}

// NewNetwork returns a new Network implementation with the provided parameters.
func NewNetwork(
	config *Config,
	minCompatibleTime time.Time,
	msgCreator message.Creator,
	metricsRegisterer prometheus.Registerer,
	log log.Logger,
	listener net.Listener,
	dialer dialer.Dialer,
	router router.ExternalHandler,
) (Network, error) {
	ipTracker, err := newIPTracker(config.TrackedSubnets, log, metricsRegisterer)
	if err != nil {
		return nil, fmt.Errorf("initializing ip tracker failed with: %w", err)
	}
	config.Validators.RegisterCallbackListener(ipTracker)
	peerConfig := &peer.Config{
		MessageCreator: msgCreator,

		Log:                  log,
		Network:              nil, // This is set below.
		Router:               router,
		VersionCompatibility: version.GetCompatibility(minCompatibleTime),
		MyNodeID:             config.MyNodeID,
		Validators:           config.Validators,
		PingFrequency:        config.PingFrequency,
		PongTimeout:          config.PingPongTimeout,
		MaxClockDifference:   config.MaxClockDifference,
		UptimeCalculator:     config.UptimeCalculator,
		IPSigner:             peer.NewIPSigner(config.MyIPPort, config.TLSKey, config.BLSKey),
	}

	onCloseCtx, cancel := context.WithCancel(context.Background())
	tlsConnRejected := prometheus.NewCounter(prometheus.CounterOpts{Namespace: "P2P Network", Name: "tls_conn_rejected"})
	n := &network{
		config:           config,
		peerConfig:       peerConfig,
		tlsConnRejected:  tlsConnRejected,
		listener:         listener,
		dialer:           dialer,
		serverUpgrader:   peer.NewTLSServerUpgrader(config.TLSConfig, tlsConnRejected),
		clientUpgrader:   peer.NewTLSClientUpgrader(config.TLSConfig, tlsConnRejected),
		onCloseCtx:       onCloseCtx,
		onCloseCtxCancel: cancel,

		sendFailRateCalculator: safemath.NewSyncAverager(safemath.NewAverager(
			0,
			config.SendFailRateHalflife,
			time.Now(),
		)),

		trackedIPs:      make(map[ids.NodeID]*trackedIP),
		ipTracker:       ipTracker,
		connectingPeers: peer.NewSet(),
		connectedPeers:  peer.NewSet(),
		router:          router,
	}
	n.peerConfig.Network = n
	return n, nil
}

func (n *network) Send(
	msg message.OutboundMessage,
	config common.SendConfig,
	subnetID ids.ID,
	allower subnets.Allower,
) set.Set[ids.NodeID] {
	namedPeers := n.getPeers(config.NodeIDs, allower)

	var (
		sampledPeers = n.samplePeers(config, subnetID, allower)
		sentTo       = set.NewSet[ids.NodeID](len(namedPeers) + len(sampledPeers))
		now          = n.peerConfig.Clock.Time()
	)

	// send to peers and update metrics
	//
	// Note: It is guaranteed that namedPeers and sampledPeers are disjoint.
	for _, peers := range [][]peer.Peer{namedPeers, sampledPeers} {
		for _, peer := range peers {
			if peer.Send(n.onCloseCtx, msg) {
				sentTo.Add(peer.ID())

				// TODO: move send fail rate calculations into the peer metrics
				// record metrics for success
				n.sendFailRateCalculator.Observe(0, now)
			} else {
				// record metrics for failure
				n.sendFailRateCalculator.Observe(1, now)
			}
		}
	}
	return sentTo
}

// Connected is called after the peer finishes the handshake.
// Will not be called after [Disconnected] is called with this peer.
func (n *network) Connected(nodeID ids.NodeID) {
	n.peersLock.Lock()
	peer, ok := n.connectingPeers.GetByID(nodeID)
	if !ok {
		n.peerConfig.Log.Error(
			"unexpectedly connected to peer when not marked as attempting to connect",
			zap.Stringer("nodeID", nodeID),
		)
		n.peersLock.Unlock()
		return
	}

	if tracked, ok := n.trackedIPs[nodeID]; ok {
		tracked.stopTracking()
		delete(n.trackedIPs, nodeID)
	}
	n.connectingPeers.Remove(nodeID)
	n.connectedPeers.Add(peer)
	n.peersLock.Unlock()

	peerIP := peer.IP()
	newIP := ips.NewClaimedIPPort(
		peer.Cert(),
		peerIP.AddrPort,
		peerIP.Timestamp,
		peerIP.TLSSignature,
	)
	trackedSubnets := peer.TrackedSubnets()
	n.ipTracker.Connected(newIP, trackedSubnets)

	peerVersion := peer.Version()
	n.router.Connected(nodeID, peerVersion, constants.PrimaryNetworkID)
	for subnetID := range n.peerConfig.MySubnets {
		if trackedSubnets.Contains(subnetID) {
			n.router.Connected(nodeID, peerVersion, subnetID)
		}
	}
}

// AllowConnection returns true if this node should have a connection to the
// provided nodeID. If the node is attempting to connect to the minimum number
// of peers, then it should only connect if this node is a validator, or the
// peer is a validator/beacon.
func (n *network) AllowConnection(nodeID ids.NodeID) bool {
	if !n.config.RequireValidatorToConnect {
		return true
	}
	_, areWeAPrimaryNetworkAValidator := n.config.Validators.GetValidator(ids.Empty, n.config.MyNodeID)
	return areWeAPrimaryNetworkAValidator || n.ipTracker.WantsConnection(nodeID)
}

func (n *network) Track(claimedIPPorts []*ips.ClaimedIPPort) error {
	_, areWeAPrimaryNetworkAValidator := n.config.Validators.GetValidator(constants.PrimaryNetworkID, n.config.MyNodeID)
	for _, ip := range claimedIPPorts {
		if err := n.track(ip, areWeAPrimaryNetworkAValidator); err != nil {
			return err
		}
	}
	return nil
}

// Disconnected is called after the peer's handling has been shutdown.
// It is not guaranteed that [Connected] was previously called with [nodeID].
// It is guaranteed that [Connected] will not be called with [nodeID] after this
// call. Note that this is from the perspective of a single peer object, because
// a peer with the same ID can reconnect to this network instance.
func (n *network) Disconnected(nodeID ids.NodeID) {
	n.peersLock.RLock()
	_, connecting := n.connectingPeers.GetByID(nodeID)
	peer, connected := n.connectedPeers.GetByID(nodeID)
	n.peersLock.RUnlock()

	if connecting {
		n.disconnectedFromConnecting(nodeID)
	}
	if connected {
		n.disconnectedFromConnected(peer, nodeID)
	}
}

func (n *network) KnownPeers() ([]byte, []byte) {
	return n.ipTracker.Bloom()
}

// There are 3 types of responses:
//
// - Respond with subnets IPs tracked by both ourselves and the peer
//   - We do not consider ourself to be a primary network validator
//
// - Respond with all subnets IPs
//   - The peer requests all peers
//   - We believe ourself to be a primary network validator
//
// - Respond with subnets IPs tracked by the peer
//   - The peer does not request all peers
//   - We believe ourself to be a primary network validator
//
// The reason we allow the peer to request all peers is so that we can avoid
// sending unnecessary data in the case that we consider them a primary network
// validator but they do not consider themselves one.
func (n *network) Peers(
	peerID ids.NodeID,
	trackedSubnets set.Set[ids.ID],
	requestAllPeers bool,
	knownPeers *bloom.ReadFilter,
	salt []byte,
) []*ips.ClaimedIPPort {
	_, areWeAPrimaryNetworkValidator := n.config.Validators.GetValidator(constants.PrimaryNetworkID, n.config.MyNodeID)

	// Only return IPs for subnets that we are tracking.
	var allowedSubnets func(ids.ID) bool
	if areWeAPrimaryNetworkValidator {
		allowedSubnets = func(ids.ID) bool { return true }
	} else {
		allowedSubnets = func(subnetID ids.ID) bool {
			return subnetID == constants.PrimaryNetworkID || n.ipTracker.trackedSubnets.Contains(subnetID)
		}
	}

	if areWeAPrimaryNetworkValidator && requestAllPeers {
		// Return IPs for all subnets.
		return getGossipableIPs(
			n.ipTracker,
			n.ipTracker.subnets,
			allowedSubnets,
			peerID,
			knownPeers,
			salt,
			int(n.config.PeerListNumValidatorIPs),
		)
	}
	return getGossipableIPs(
		n.ipTracker,
		trackedSubnets,
		allowedSubnets,
		peerID,
		knownPeers,
		salt,
		int(n.config.PeerListNumValidatorIPs),
	)
}

// Dispatch starts accepting connections from other nodes attempting to connect
// to this node.
func (n *network) Dispatch() error {
	go n.runTimers() // Periodically perform operations
	for {            // Continuously accept new connections
		if n.onCloseCtx.Err() != nil {
			break
		}

		conn, err := n.listener.Accept() // Returns error when n.Close() is called
		if err != nil {
			n.peerConfig.Log.Debug("error during server accept", zap.Error(err))
			// Sleep for a small amount of time to try to wait for the
			// error to go away.
			time.Sleep(time.Millisecond)
			continue
		}

		// Note: listener.Accept is rate limited outside of this package, so a
		// peer can not just arbitrarily spin up goroutines here.
		go func() {
			// Note: Calling [RemoteAddr] with the Proxy protocol enabled may
			// block for up to ProxyReadHeaderTimeout. Therefore, we ensure to
			// call this function inside the go-routine, rather than the main
			// accept loop.
			remoteAddr := conn.RemoteAddr().String()
			ip, err := ips.ParseAddrPort(remoteAddr)
			if err != nil {
				n.peerConfig.Log.Error("failed to parse remote address",
					zap.String("peerIP", remoteAddr),
					zap.Error(err),
				)
				_ = conn.Close()
				return
			}

			n.peerConfig.Log.Debug("starting to upgrade connection",
				zap.String("direction", "inbound"),
				zap.Stringer("peerIP", ip),
			)

			if err := n.upgrade(conn, n.serverUpgrader); err != nil {
				n.peerConfig.Log.Info("failed to upgrade connection",
					zap.String("direction", "inbound"),
					zap.Error(err),
				)
			}
		}()
	}
	n.StartClose()

	n.peersLock.RLock()
	connecting := n.connectingPeers.Sample(n.connectingPeers.Len(), peer.NoPrecondition)
	connected := n.connectedPeers.Sample(n.connectedPeers.Len(), peer.NoPrecondition)
	n.peersLock.RUnlock()

	errs := wrappers.Errs{}
	for _, peer := range append(connecting, connected...) {
		errs.Add(peer.AwaitClosed(context.TODO()))
	}
	return errs.Err
}

func (n *network) ManuallyTrack(nodeID ids.NodeID, ip netip.AddrPort) {
	n.ipTracker.ManuallyTrack(nodeID)

	n.peersLock.Lock()
	defer n.peersLock.Unlock()

	_, connected := n.connectedPeers.GetByID(nodeID)
	if connected {
		// If I'm currently connected to [nodeID] then they will have told me
		// how to connect to them in the future, and I don't need to attempt to
		// connect to them now.
		return
	}

	_, isTracked := n.trackedIPs[nodeID]
	if !isTracked {
		tracked := newTrackedIP(ip)
		n.trackedIPs[nodeID] = tracked
		n.dial(nodeID, tracked)
	}
}

func (n *network) track(ip *ips.ClaimedIPPort, trackAllSubnets bool) error {
	//// To avoid signature verification when the IP isn't needed, we
	//// optimistically filter out IPs. This can result in us not tracking an IP
	//// that we otherwise would have. This case can only happen if the node
	//// became a validator between the time we verified the signature and when we
	//// processed the IP; which should be very rare.
	////
	//// Note: Avoiding signature verification when the IP isn't needed is a
	//// **significant** performance optimization.
	//if !n.ipTracker.ShouldVerifyIP(ip, trackAllSubnets) {
	//	n.metrics.numUselessPeerListBytes.Add(float64(ip.Size()))
	//	return nil
	//}

	// Perform all signature verification and hashing before grabbing the peer
	// lock.
	signedIP := peer.SignedIP{
		UnsignedIP: peer.UnsignedIP{
			AddrPort:  ip.AddrPort,
			Timestamp: ip.Timestamp,
		},
		TLSSignature: ip.Signature,
	}
	maxTimestamp := n.peerConfig.Clock.Time().Add(n.peerConfig.MaxClockDifference)
	if err := signedIP.Verify(ip.Cert, maxTimestamp); err != nil {
		return err
	}

	n.peersLock.Lock()
	defer n.peersLock.Unlock()

	if !n.ipTracker.AddIP(ip) {
		return nil
	}

	if _, connected := n.connectedPeers.GetByID(ip.NodeID); connected {
		// If I'm currently connected to [nodeID] then I'll attempt to dial them
		// when we disconnect.
		return nil
	}

	tracked, isTracked := n.trackedIPs[ip.NodeID]
	if isTracked {
		// Stop tracking the old IP and start tracking the new one.
		tracked = tracked.trackNewIP(ip.AddrPort)
	} else {
		tracked = newTrackedIP(ip.AddrPort)
	}
	n.trackedIPs[ip.NodeID] = tracked
	n.dial(ip.NodeID, tracked)
	return nil
}

// getPeers returns a slice of connected peers from a set of [nodeIDs].
//
//   - [nodeIDs] the IDs of the peers that should be returned if they are
//     connected.
//   - [subnetID] the subnetID whose membership should be considered to
//     determine if the node is a validator.
//   - [allower] interface that determines if a node is allowed to connect to
//     the subnets based on its validator status.
func (n *network) getPeers(
	nodeIDs set.Set[ids.NodeID],
	allower subnets.Allower,
) []peer.Peer {
	peers := make([]peer.Peer, 0, nodeIDs.Len())

	n.peersLock.RLock()
	defer n.peersLock.RUnlock()

	for nodeID := range nodeIDs {
		peer, ok := n.connectedPeers.GetByID(nodeID)
		if !ok {
			continue
		}

		_, areTheyAValidator := n.config.Validators.GetValidator(ids.Empty, nodeID)
		// check if the peer is allowed to connect to the subnets
		if !allower.IsAllowed(nodeID, areTheyAValidator) {
			continue
		}

		peers = append(peers, peer)
	}

	return peers
}

// samplePeers samples connected peers attempting to align with the number of
// requested validators, non-validators, and peers. This function will
// explicitly ignore nodeIDs already included in the send config.
func (n *network) samplePeers(
	config common.SendConfig,
	subnetID ids.ID,
	allower subnets.Allower,
) []peer.Peer {
	// As an optimization, if there are fewer validators than
	// [numValidatorsToSample], only attempt to sample [numValidatorsToSample]
	// validators to potentially avoid iterating over the entire peer set.
	numValidatorsToSample := min(config.Validators, n.config.Validators.NumValidators(subnetID))

	n.peersLock.RLock()
	defer n.peersLock.RUnlock()

	return n.connectedPeers.Sample(
		numValidatorsToSample+config.NonValidators+config.Peers,
		func(p peer.Peer) bool {
			// Only return peers that are tracking [subnetID]
			if trackedSubnets := p.TrackedSubnets(); !trackedSubnets.Contains(subnetID) {
				return false
			}

			peerID := p.ID()
			// if the peer was already explicitly included, don't include in the
			// sample
			if config.NodeIDs.Contains(peerID) {
				return false
			}

			_, areTheyAValidator := n.config.Validators.GetValidator(subnetID, peerID)
			// check if the peer is allowed to connect to the subnets
			if !allower.IsAllowed(peerID, areTheyAValidator) {
				return false
			}

			if config.Peers > 0 {
				config.Peers--
				return true
			}

			if areTheyAValidator {
				numValidatorsToSample--
				return numValidatorsToSample >= 0
			}

			config.NonValidators--
			return config.NonValidators >= 0
		},
	)
}

func (n *network) disconnectedFromConnecting(nodeID ids.NodeID) {
	n.peersLock.Lock()
	defer n.peersLock.Unlock()

	n.connectingPeers.Remove(nodeID)

	// The peer that is disconnecting from us didn't finish the handshake
	tracked, ok := n.trackedIPs[nodeID]
	if ok {
		if n.ipTracker.WantsConnection(nodeID) {
			tracked := tracked.trackNewIP(tracked.ip)
			n.trackedIPs[nodeID] = tracked
			n.dial(nodeID, tracked)
		} else {
			tracked.stopTracking()
			delete(n.trackedIPs, nodeID)
		}
	}
}

func (n *network) disconnectedFromConnected(peer peer.Peer, nodeID ids.NodeID) {
	n.ipTracker.Disconnected(nodeID)
	n.router.Disconnected(nodeID)

	n.peersLock.Lock()
	defer n.peersLock.Unlock()

	n.connectedPeers.Remove(nodeID)

	// The peer that is disconnecting from us finished the handshake
	if ip, wantsConnection := n.ipTracker.GetIP(nodeID); wantsConnection {
		tracked := newTrackedIP(ip.AddrPort)
		n.trackedIPs[nodeID] = tracked
		n.dial(nodeID, tracked)
	}
}

// dial will spin up a new goroutine and attempt to establish a connection with
// [nodeID] at [ip].
//
// If the connection established at [ip] doesn't match [nodeID]:
// - attempts to reach [nodeID] at [ip] will be halted.
// - the connection will be checked to see if the connection is desired or not.
//
// If [ip] has been flagged with [ip.stopTracking] then this goroutine will
// exit.
//
// If [nodeID] is marked as connecting or connected then this goroutine will
// exit.
//
// If [nodeID] is no longer marked as desired then this goroutine will exit and
// the entry in the [trackedIP]s set will be removed.
//
// If initiating a connection to [ip] fails, then dial will reattempt. However,
// there is a randomized exponential backoff to avoid spamming connection
// attempts.
func (n *network) dial(nodeID ids.NodeID, ip *trackedIP) {
	n.peerConfig.Log.Info("attempting to dial node",
		zap.Stringer("nodeID", nodeID),
		zap.Stringer("ip", ip.ip),
	)
	go func() {

		for {
			timer := time.NewTimer(ip.getDelay())

			select {
			case <-n.onCloseCtx.Done():
				timer.Stop()
				return
			case <-ip.onStopTracking:
				timer.Stop()
				return
			case <-timer.C:
			}

			n.peersLock.Lock()
			// If we no longer desire a connect to nodeID, we should cleanup
			// trackedIPs and this goroutine. This prevents a memory leak when
			// the tracked nodeID leaves the validator set and is never able to
			// be connected to.
			if !n.ipTracker.WantsConnection(nodeID) {
				// Typically [n.trackedIPs[nodeID]] will already equal [ip], but
				// the reference to [ip] is refreshed to avoid any potential
				// race conditions before removing the entry.
				if ip, exists := n.trackedIPs[nodeID]; exists {
					ip.stopTracking()
					delete(n.trackedIPs, nodeID)
				}
				n.peersLock.Unlock()
				return
			}
			_, connecting := n.connectingPeers.GetByID(nodeID)
			_, connected := n.connectedPeers.GetByID(nodeID)
			n.peersLock.Unlock()

			// While it may not be strictly needed to stop attempting to connect
			// to an already connected peer here. It does prevent unnecessary
			// outbound connections. Additionally, because the peer would
			// immediately drop a duplicated connection, this prevents any
			// "connection reset by peer" errors from interfering with the
			// later duplicated connection check.
			if connecting || connected {
				n.peerConfig.Log.Info(
					"exiting attempt to dial peer",
					zap.String("reason", "already connected"),
					zap.Stringer("nodeID", nodeID),
				)
				return
			}

			// Increase the delay that we will use for a future connection
			// attempt.
			ip.increaseDelay(
				n.config.InitialReconnectDelay,
				n.config.MaxReconnectDelay,
			)

			// If the network is configured to disallow private IPs and the
			// provided IP is private, we skip all attempts to initiate a
			// connection.
			//
			// Invariant: We perform this check inside of the looping goroutine
			// because this goroutine must clean up the trackedIPs entry if
			// nodeID leaves the validator set. This is why we continue the loop
			// rather than returning even though we will never initiate an
			// outbound connection with this IP.
			if !n.config.AllowPrivateIPs && !ips.IsPublic(ip.ip.Addr()) {
				n.peerConfig.Log.Info("skipping connection dial",
					zap.String("reason", "outbound connections to private IPs are prohibited"),
					zap.Stringer("nodeID", nodeID),
					zap.Stringer("peerIP", ip.ip),
					zap.Duration("delay", ip.delay),
				)
				continue
			}

			conn, err := n.dialer.Dial(n.onCloseCtx, ip.ip)
			if err != nil {
				n.peerConfig.Log.Info(
					"failed to reach peer, attempting again",
					zap.Stringer("nodeID", nodeID),
					zap.Stringer("peerIP", ip.ip),
					zap.Duration("delay", ip.delay),
				)
				continue
			}

			n.peerConfig.Log.Info("starting to upgrade connection",
				zap.String("direction", "outbound"),
				zap.Stringer("nodeID", nodeID),
				zap.Stringer("peerIP", ip.ip),
			)

			err = n.upgrade(conn, n.clientUpgrader)
			if err != nil {
				n.peerConfig.Log.Info(
					"failed to upgrade, attempting again",
					zap.Stringer("nodeID", nodeID),
					zap.Stringer("peerIP", ip.ip),
					zap.Duration("delay", ip.delay),
				)
				continue
			}
			return
		}
	}()
}

// upgrade the provided connection, which may be an inbound connection or an
// outbound connection, with the provided [upgrader].
//
// If the connection is successfully upgraded, [nil] will be returned.
//
// If the connection is desired by the node, then the resulting upgraded
// connection will be used to create a new peer. Otherwise the connection will
// be immediately closed.
func (n *network) upgrade(conn net.Conn, upgrader peer.Upgrader) error {
	upgradeTimeout := n.peerConfig.Clock.Time().Add(n.config.ReadHandshakeTimeout)
	if err := conn.SetReadDeadline(upgradeTimeout); err != nil {
		_ = conn.Close()
		n.peerConfig.Log.Info("failed to set the read deadline",
			zap.Error(err),
		)
		return err
	}

	nodeID, tlsConn, cert, err := upgrader.Upgrade(conn)
	if err != nil {
		_ = conn.Close()
		n.peerConfig.Log.Info("failed to upgrade connection",
			zap.Error(err),
		)
		return err
	}

	if err := tlsConn.SetReadDeadline(time.Time{}); err != nil {
		_ = tlsConn.Close()
		n.peerConfig.Log.Info("failed to clear the read deadline",
			zap.Error(err),
		)
		return err
	}

	// At this point we have successfully upgraded the connection and will
	// return a nil error.

	if nodeID == n.config.MyNodeID {
		_ = tlsConn.Close()
		n.peerConfig.Log.Info("dropping connection to myself")
		return nil
	}

	if !n.AllowConnection(nodeID) {
		_ = tlsConn.Close()
		n.peerConfig.Log.Info(
			"dropping undesired connection",
			zap.Stringer("nodeID", nodeID),
		)
		return nil
	}

	n.peersLock.Lock()
	if n.closing {
		n.peersLock.Unlock()

		_ = tlsConn.Close()
		n.peerConfig.Log.Info(
			"dropping connection",
			zap.String("reason", "shutting down the p2p network"),
			zap.Stringer("nodeID", nodeID),
		)
		return nil
	}

	if _, connecting := n.connectingPeers.GetByID(nodeID); connecting {
		n.peersLock.Unlock()

		_ = tlsConn.Close()
		n.peerConfig.Log.Info(
			"dropping connection",
			zap.String("reason", "already connecting to peer"),
			zap.Stringer("nodeID", nodeID),
		)
		return nil
	}

	if _, connected := n.connectedPeers.GetByID(nodeID); connected {
		n.peersLock.Unlock()

		_ = tlsConn.Close()
		n.peerConfig.Log.Info(
			"dropping connection",
			zap.String("reason", "already connecting to peer"),
			zap.Stringer("nodeID", nodeID),
		)
		return nil
	}

	n.peerConfig.Log.Info("starting handshake",
		zap.Stringer("nodeID", nodeID),
	)

	msgQueueBufferSize := 1024
	var onFailure peer.SendFailedFunc = func(msg message.OutboundMessage) {
		n.peerConfig.Log.Error("Failed to send message:", msg)
	}

	// peer.Start requires there is only ever one peer instance running with the
	// same [peerConfig.InboundMsgThrottler]. This is guaranteed by the above
	// de-duplications for [connectingPeers] and [connectedPeers].
	peer := peer.Start(
		n.peerConfig,
		tlsConn,
		cert,
		nodeID,
		peer.NewBlockingMessageQueue(
			onFailure,
			n.peerConfig.Log,
			msgQueueBufferSize,
		),
	)
	n.connectingPeers.Add(peer)
	n.peersLock.Unlock()
	return nil
}

func (n *network) PeerInfo(nodeIDs []ids.NodeID) []peer.Info {
	n.peersLock.RLock()
	defer n.peersLock.RUnlock()

	if len(nodeIDs) == 0 {
		return n.connectedPeers.AllInfo()
	}
	return n.connectedPeers.Info(nodeIDs)
}

func (n *network) StartClose() {
	n.closeOnce.Do(func() {
		n.peerConfig.Log.Info("shutting down the p2p networking")

		if err := n.listener.Close(); err != nil {
			n.peerConfig.Log.Debug("closing the network listener",
				zap.Error(err),
			)
		}

		n.peersLock.Lock()
		defer n.peersLock.Unlock()

		n.closing = true
		n.onCloseCtxCancel()

		for nodeID, tracked := range n.trackedIPs {
			tracked.stopTracking()
			delete(n.trackedIPs, nodeID)
		}

		for i := 0; i < n.connectingPeers.Len(); i++ {
			peer, _ := n.connectingPeers.GetByIndex(i)
			peer.StartClose()
		}

		for i := 0; i < n.connectedPeers.Len(); i++ {
			peer, _ := n.connectedPeers.GetByIndex(i)
			peer.StartClose()
		}
	})
}

func (n *network) NodeUptime() (UptimeResult, error) {
	myStake := n.config.Validators.GetWeight(constants.PrimaryNetworkID, n.config.MyNodeID)
	if myStake == 0 {
		return UptimeResult{}, errNotValidator
	}

	totalWeightInt, err := n.config.Validators.TotalWeight(constants.PrimaryNetworkID)
	if err != nil {
		return UptimeResult{}, fmt.Errorf("error while fetching weight for primary network %w", err)
	}

	var (
		totalWeight          = float64(totalWeightInt)
		totalWeightedPercent = 100 * float64(myStake)
		rewardingStake       = float64(myStake)
	)

	n.peersLock.RLock()
	defer n.peersLock.RUnlock()

	for i := 0; i < n.connectedPeers.Len(); i++ {
		peer, _ := n.connectedPeers.GetByIndex(i)

		nodeID := peer.ID()
		weight := n.config.Validators.GetWeight(constants.PrimaryNetworkID, nodeID)
		if weight == 0 {
			// this is not a validator skip it.
			continue
		}

		observedUptime := peer.ObservedUptime()
		percent := float64(observedUptime)
		weightFloat := float64(weight)
		totalWeightedPercent += percent * weightFloat

		// if this peer thinks we're above requirement add the weight
		if percent/100 >= n.config.UptimeRequirement {
			rewardingStake += weightFloat
		}
	}

	return UptimeResult{
		WeightedAveragePercentage: math.Abs(totalWeightedPercent / totalWeight),
		RewardingStakePercentage:  math.Abs(100 * rewardingStake / totalWeight),
	}, nil
}

func (n *network) runTimers() {
	pullGossipPeerlists := time.NewTicker(n.config.PeerListPullGossipFreq)
	resetPeerListBloom := time.NewTicker(n.config.PeerListBloomResetFreq)
	updateUptimes := time.NewTicker(n.config.UptimeMetricFreq)
	defer func() {
		resetPeerListBloom.Stop()
		updateUptimes.Stop()
	}()

	for {
		select {
		case <-n.onCloseCtx.Done():
			return
		case <-pullGossipPeerlists.C:
			n.pullGossipPeerLists()
		case <-resetPeerListBloom.C:
			if err := n.ipTracker.ResetBloom(); err != nil {
				n.peerConfig.Log.Error("failed to reset ip tracker bloom filter",
					zap.Error(err),
				)
			} else {
				n.peerConfig.Log.Debug("reset ip tracker bloom filter")
			}
		case <-updateUptimes.C:
			primaryUptime, err := n.NodeUptime()
			if err != nil {
				n.peerConfig.Log.Debug("failed to get primary network uptime",
					zap.Error(err),
				)
			}
			n.peerConfig.Log.Debug("primary uptime", primaryUptime)
		}
	}
}

// pullGossipPeerLists requests validators from peers in the network
func (n *network) pullGossipPeerLists() {
	peers := n.samplePeers(
		common.SendConfig{
			Validators: 1,
		},
		constants.PrimaryNetworkID,
		subnets.NoOpAllower,
	)

	for _, p := range peers {
		p.StartSendGetPeerList()
	}
}

//lint:ignore U1000 will be used as getter to implement network infrastructure
func (n *network) getLastReceived() (time.Time, bool) {
	lastReceived := atomic.LoadInt64(&n.peerConfig.LastReceived)
	if lastReceived == 0 {
		return time.Time{}, false
	}
	return time.Unix(lastReceived, 0), true
}

//lint:ignore U1000 will be used as getter to implement network infrastructure
func (n *network) getLastSent() (time.Time, bool) {
	lastSent := atomic.LoadInt64(&n.peerConfig.LastSent)
	if lastSent == 0 {
		return time.Time{}, false
	}
	return time.Unix(lastSent, 0), true
}
