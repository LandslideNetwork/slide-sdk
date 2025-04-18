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
	Allower
}

type subnet struct {
	config   Config
	myNodeID ids.NodeID
}

func New(myNodeID ids.NodeID, config Config) Subnet {
	return &subnet{
		config:   config,
		myNodeID: myNodeID,
	}
}

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
