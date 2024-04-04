package peer

import (
	"github.com/cometbft/cometbft/libs/log"
	"github.com/consideritdone/landslidevm/utils/version"
)

// peerTracker tracks the bandwidth of responses coming from peers,
// preferring to contact peers with known good bandwidth, connecting
// to new peers with an exponentially decaying probability.
// Note: is not thread safe, caller must handle synchronization.
type Tracker struct {
	peers  map[string]*peerInfo // all peers we are connected to
	logger log.Logger
}

func NewPeerTracker(logger log.Logger) *Tracker {
	return &Tracker{
		peers:  make(map[string]*peerInfo),
		logger: logger,
	}
}

// information we track on a given peer
type peerInfo struct {
	version *version.Application
}

// Connected should be called when [nodeID] connects to this node
func (p *Tracker) Connected(nodeID string, nodeVersion *version.Application) {
	if peer := p.peers[nodeID]; peer != nil {
		// Peer is already connected, update the version if it has changed.
		// Log a warning message since the consensus engine should never call Connected on a peer
		// that we have already marked as Connected.
		if nodeVersion.Compare(peer.version) != 0 {
			p.peers[nodeID] = &peerInfo{
				version: nodeVersion,
			}
			p.logger.Info("updating node version of already connected peer", "nodeID", nodeID, "storedVersion", peer.version, "nodeVersion", nodeVersion)
		} else {
			p.logger.Info("ignoring peer connected event for already connected peer with identical version", "nodeID", nodeID)
		}
		return
	}

	p.peers[nodeID] = &peerInfo{
		version: nodeVersion,
	}
}

// Disconnected should be called when [nodeID] disconnects from this node
func (p *Tracker) Disconnected(nodeID string) {
	delete(p.peers, nodeID)
}
