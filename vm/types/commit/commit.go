package commit

import (
	"time"

	"github.com/cometbft/cometbft/types"
)

func NewCommit(height int64, round int32, blockID types.BlockID, commitSigs []types.CommitSig) *types.Commit {
	return &types.Commit{
		Height:     height,
		Round:      round,
		BlockID:    blockID,
		Signatures: commitSigs,
	}
}

func MakeCommit(height int64, timestamp time.Time, validator []byte, bps types.PartSetHeader) *types.Commit {
	commitSig := []types.CommitSig{}
	if height > 1 {
		commitSig = []types.CommitSig{{
			BlockIDFlag:      types.BlockIDFlagNil,
			Timestamp:        time.Now(),
			ValidatorAddress: validator,
			Signature:        []byte{0x0},
		}}
	}
	blockID := types.BlockID{
		Hash:          []byte(""),
		PartSetHeader: bps,
	}
	return NewCommit(height, 0, blockID, commitSig)
}
