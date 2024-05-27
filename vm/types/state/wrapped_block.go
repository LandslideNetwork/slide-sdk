package state

import (
	"encoding/json"

	"github.com/consideritdone/landslidevm/proto/vm"
	"github.com/consideritdone/landslidevm/utils/cache"
	"github.com/consideritdone/landslidevm/utils/ids"
)

// TODO: make these configurable
const decidedCacheSize, missingCacheSize, unverifiedCacheSize, bytesToIDCacheSize int = 16, 16, 16, 16

// PointerOverhead is used to approximate the memory footprint from allocating a pointer.
const pointerOverhead = 8

func cachedBlockSize(_ ids.ID, bw *vm.Block) int {
	data, _ := json.Marshal(bw)
	return ids.IDLen + len(data) + 2*pointerOverhead
}

func cachedBlockBytesSize(blockBytes string, _ ids.ID) int {
	return len(blockBytes) + ids.IDLen
}

type WrappedBlocksStorage struct {
	// verifiedBlocks is a map of blocks that have been verified and are
	// therefore currently in consensus.
	verifiedBlocks map[ids.ID]*vm.Block
	// decidedBlocks is an LRU cache of decided blocks.
	// the block for which the Accept/Reject function was called
	decidedBlocks cache.Cacher[ids.ID, *vm.Block]
	// unverifiedBlocks is an LRU cache of blocks with status processing
	// that have not yet passed verification.
	unverifiedBlocks cache.Cacher[ids.ID, *vm.Block]
	// missingBlocks is an LRU cache of missing blocks
	missingBlocks cache.Cacher[ids.ID, struct{}]
	// string([byte repr. of block]) --> the block's ID
	bytesToIDCache cache.Cacher[string, ids.ID]
}

func NewWrappedBlocksStorage() *WrappedBlocksStorage {
	return &WrappedBlocksStorage{
		verifiedBlocks: make(map[ids.ID]*vm.Block),
		decidedBlocks: cache.NewSizedLRU[ids.ID, *vm.Block](
			decidedCacheSize,
			cachedBlockSize,
		),
		missingBlocks: &cache.LRU[ids.ID, struct{}]{Size: missingCacheSize},
		unverifiedBlocks: cache.NewSizedLRU[ids.ID, *vm.Block](
			unverifiedCacheSize,
			cachedBlockSize,
		),
		bytesToIDCache: cache.NewSizedLRU[string, ids.ID](
			bytesToIDCacheSize,
			cachedBlockBytesSize,
		),
	}
}

// Flush each block cache
func (s *WrappedBlocksStorage) Flush() {
	s.decidedBlocks.Flush()
	s.missingBlocks.Flush()
	s.unverifiedBlocks.Flush()
	s.bytesToIDCache.Flush()
}
