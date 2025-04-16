package subnets

import (
	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

var _ Subnet = (*subnet)(nil)

type Allower interface {
	// IsAllowed filters out nodes that are not allowed to connect to this subnets
	IsAllowed(nodeID ids.NodeID, isValidator bool) bool
}

// Subnet keeps track of the currently bootstrapping chains in a subnets. If no
// chains in the subnets are currently bootstrapping, the subnets is considered
// bootstrapped.
type Subnet interface {
	// common.BootstrapTracker
	//
	//// AddChain adds a chain to this Subnet
	// AddChain(chainID ids.ID) bool
	//
	//// Config returns config of this Subnet
	// Config() Config

	Allower
}

type subnet struct {
	// lock            sync.RWMutex
	// bootstrapping   set.Set[ids.ID]
	// bootstrapped    set.Set[ids.ID]
	config   Config
	myNodeID ids.NodeID
	// bootstrapSignal common.PreemptionSignal
}

func New(myNodeID ids.NodeID, config Config) Subnet {
	return &subnet{
		config:   config,
		myNodeID: myNodeID,
	}
}

// func (s *subnets) AllBootstrapped() <-chan struct{} {
//	return s.bootstrapSignal.Listen()
// }
//
// func (s *subnets) IsBootstrapped() bool {
//	s.lock.RLock()
//	defer s.lock.RUnlock()
//
//	return s.bootstrapping.Len() == 0
// }
//
// func (s *subnets) Bootstrapped(chainID ids.ID) {
//	s.lock.Lock()
//	defer s.lock.Unlock()
//
//	s.bootstrapping.Remove(chainID)
//	s.bootstrapped.Add(chainID)
//	if s.bootstrapping.Len() > 0 {
//		return
//	}
//
//	s.bootstrapSignal.Preempt()
// }
//
// func (s *subnets) AddChain(chainID ids.ID) bool {
//	s.lock.Lock()
//	defer s.lock.Unlock()
//
//	if s.bootstrapping.Contains(chainID) || s.bootstrapped.Contains(chainID) {
//		return false
//	}
//
//	s.bootstrapping.Add(chainID)
//	return true
// }

func (s *subnet) Config() Config {
	return s.config
}

func (s *subnet) IsAllowed(nodeID ids.NodeID, isValidator bool) bool {
	// Case 1: NodeID is this node
	// Case 2: This subnets is not validator-only subnets
	// Case 3: NodeID is a validator for this chain
	// Case 4: NodeID is explicitly allowed whether it's subnets validator or not
	return nodeID == s.myNodeID ||
		!s.config.ValidatorOnly ||
		isValidator ||
		s.config.AllowedNodes.Contains(nodeID)
}
