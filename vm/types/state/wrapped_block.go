package state

import (
	"encoding/json"

	"github.com/cometbft/cometbft/types"

	"github.com/consideritdone/landslidevm/proto/vm"
	"github.com/consideritdone/landslidevm/utils/cache"
	"github.com/consideritdone/landslidevm/utils/ids"
)

// TODO: make these configurable
const decidedCacheSize, missingCacheSize, unverifiedCacheSize, bytesToIDCacheSize int = 10000, 10000, 10000, 10000

// PointerOverhead is used to approximate the memory footprint from allocating a pointer.
const pointerOverhead = 8

func cachedBlockSize(_ ids.ID, bw *WrappedBlock) int {
	data, _ := json.Marshal(bw)
	return ids.IDLen + len(data) + 2*pointerOverhead
}

func cachedBlockBytesSize(blockBytes string, _ ids.ID) int {
	return len(blockBytes) + ids.IDLen
}

type WrappedBlock struct {
	// block is the block this wrapper is wrapping
	Block *types.Block
	// status is the status of the block
	Status vm.Status
}

type WrappedBlocksStorage struct {
	// verifiedBlocks is a map of blocks that have been verified and are
	// therefore currently in consensus.
	VerifiedBlocks map[ids.ID]*WrappedBlock
	// decidedBlocks is an LRU cache of decided blocks.
	// the block for which the Accept/Reject function was called
	DecidedBlocks cache.Cacher[ids.ID, *WrappedBlock]
	// unverifiedBlocks is an LRU cache of blocks with status processing
	// that have not yet passed verification.
	UnverifiedBlocks cache.Cacher[ids.ID, *WrappedBlock]
	// missingBlocks is an LRU cache of missing blocks
	MissingBlocks cache.Cacher[ids.ID, struct{}]
	// string([byte repr. of block]) --> the block's ID
	BytesToIDCache cache.Cacher[string, ids.ID]
}

func NewWrappedBlocksStorage() *WrappedBlocksStorage {
	return &WrappedBlocksStorage{
		VerifiedBlocks: make(map[ids.ID]*WrappedBlock),
		DecidedBlocks: cache.NewSizedLRU[ids.ID, *WrappedBlock](
			decidedCacheSize,
			cachedBlockSize,
		),
		MissingBlocks: &cache.LRU[ids.ID, struct{}]{Size: missingCacheSize},
		UnverifiedBlocks: cache.NewSizedLRU[ids.ID, *WrappedBlock](
			unverifiedCacheSize,
			cachedBlockSize,
		),
		BytesToIDCache: cache.NewSizedLRU[string, ids.ID](
			bytesToIDCacheSize,
			cachedBlockBytesSize,
		),
	}
}

// Flush each block cache
func (s *WrappedBlocksStorage) Flush() {
	s.DecidedBlocks.Flush()
	s.MissingBlocks.Flush()
	s.UnverifiedBlocks.Flush()
	s.BytesToIDCache.Flush()
}

// GetCachedBlock checks the caches for [blkID] by priority. Returning
// true if [blkID] is found in one of the caches.
func (s *WrappedBlocksStorage) GetCachedBlock(blkID ids.ID) (*WrappedBlock, bool) {
	if blk, ok := s.VerifiedBlocks[blkID]; ok {
		return blk, true
	}

	if blk, ok := s.DecidedBlocks.Get(blkID); ok {
		return blk, true
	}

	if blk, ok := s.UnverifiedBlocks.Get(blkID); ok {
		return blk, true
	}

	return nil, false
}
