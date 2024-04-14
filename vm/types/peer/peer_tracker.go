package peer

import (
	"github.com/cometbft/cometbft/libs/log"
	"strings"
	"sync"
)

// peerTracker tracks the bandwidth of responses coming from peers,
// preferring to contact peers with known good bandwidth, connecting
// to new peers with an exponentially decaying probability.
// Note: is not thread safe, caller must handle synchronization.
type Tracker struct {
	mtx *sync.RWMutex

	peers  map[string]*peerInfo // all peers we are connected to
	logger log.Logger
}

func NewPeerTracker(logger log.Logger) *Tracker {
	return &Tracker{
		mtx:    &sync.RWMutex{},
		peers:  make(map[string]*peerInfo),
		logger: logger,
	}
}

// information we track on a given peer
type peerInfo struct {
	version string
}

// Connected should be called when [nodeID] connects to this node
func (p *Tracker) Connected(nodeID string, nodeVersion string) {
	p.mtx.RLock()
	peer := p.peers[nodeID]
	p.mtx.RUnlock()
	if peer != nil {
		// Peer is already connected, update the version if it has changed.
		// Log a warning message since the consensus engine should never call Connected on a peer
		// that we have already marked as Connected.
		if strings.Compare(nodeVersion, peer.version) != 0 {
			p.logger.Info("updating node version of already connected peer", "nodeID", nodeID, "storedVersion", peer.version, "nodeVersion", nodeVersion)
		} else {
			p.logger.Info("ignoring peer connected event for already connected peer with identical version", "nodeID", nodeID)
			return
		}
	} else {
		p.logger.Info("adding node version of new peer", "nodeID", nodeID, "storedVersion", peer.version, "nodeVersion", nodeVersion)
	}

	p.mtx.Lock()
	p.peers[nodeID] = &peerInfo{
		version: nodeVersion,
	}
	p.mtx.Unlock()
}

// Disconnected should be called when [nodeID] disconnects from this node
func (p *Tracker) Disconnected(nodeID string) {
	p.mtx.Lock()
	delete(p.peers, nodeID)
	p.mtx.Unlock()
}
