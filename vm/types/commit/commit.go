package commit

import (
	"time"

	"github.com/cometbft/cometbft/types"
)

func MakeCommit(height int64, timestamp time.Time, validators *types.ValidatorSet, blockID types.BlockID) *types.Commit {
	var commitSigs []types.CommitSig
	round := int32(0)
	if height > 1 {
		commitSigs = make([]types.CommitSig, len(validators.Validators))
		for i := range commitSigs {
			commitSigs[i] = types.CommitSig{
				BlockIDFlag:      types.BlockIDFlagCommit,
				Timestamp:        timestamp,
				ValidatorAddress: validators.Validators[i].Address,
				Signature:        []byte{0x0},
			}
		}
	}

	return &types.Commit{Height: height, Round: round, BlockID: blockID, Signatures: commitSigs}
}
