package block

import (
	"github.com/cometbft/cometbft/types"
)

func ParentHash(block *types.Block) [32]byte {
	var parentHash [32]byte
	if block.LastBlockID.Hash != nil {
		parentHash = [32]byte(block.LastBlockID.Hash)
	}

	return parentHash
}
