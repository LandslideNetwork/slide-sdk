package state

import (
	"github.com/cometbft/cometbft/types"

	"github.com/consideritdone/landslidevm/proto/vm"
)

// BlockWrapper is a wrapper around a block that contains additional metadata
type BlockWrapper struct {
	Block  types.Block
	Status vm.Status
}
