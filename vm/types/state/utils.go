package state

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/cometbft/cometbft/crypto"
	"github.com/cometbft/cometbft/libs/json"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cometbft/cometbft/state"
	"github.com/cometbft/cometbft/types"
	cmttime "github.com/cometbft/cometbft/types/time"

	"github.com/landslidenetwork/slide-sdk/proto/vm"
)

func EncodeBlockWithStatus(blk *types.Block, status vm.Status) ([]byte, error) {
	blockBytes, err := EncodeBlock(blk)
	if err != nil {
		return nil, err
	}
	wrappedBlk := &vm.Block{
		Block:  blockBytes,
		Status: status,
	}

	data, err := json.Marshal(wrappedBlk)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func DecodeBlockWithStatus(data []byte) (*types.Block, vm.Status, error) {
	wrappedBlk := new(vm.Block)

	if err := json.Unmarshal(data, wrappedBlk); err != nil {
		return nil, vm.Status_STATUS_UNSPECIFIED, err
	}

	blk, err := DecodeBlock(wrappedBlk.Block)
	if err != nil {
		return nil, vm.Status_STATUS_UNSPECIFIED, err
	}

	return blk, wrappedBlk.Status, nil
}

func EncodeBlock(block *types.Block) ([]byte, error) {
	proto, err := block.ToProto()
	if err != nil {
		return nil, err
	}
	return proto.Marshal()
}

func DecodeBlock(data []byte) (*types.Block, error) {
	protoBlock := new(tmproto.Block)
	if err := protoBlock.Unmarshal(data); err != nil {
		return nil, err
	}
	return types.BlockFromProto(protoBlock)
}

func ValidateBlock(state state.State, block *types.Block) error {
	// Validate internal consistency.
	if err := block.ValidateBasic(); err != nil {
		return err
	}

	// Validate basic info.
	if block.Version.App != state.Version.Consensus.App ||
		block.Version.Block != state.Version.Consensus.Block {
		return fmt.Errorf("wrong Block.Header.Version. Expected %v, got %v",
			state.Version.Consensus,
			block.Version,
		)
	}
	if block.ChainID != state.ChainID {
		return fmt.Errorf("wrong Block.Header.ChainID. Expected %v, got %v",
			state.ChainID,
			block.ChainID,
		)
	}
	if state.LastBlockHeight == 0 && block.Height != state.InitialHeight {
		return fmt.Errorf("wrong Block.Header.Height. Expected %v for initial block, got %v",
			block.Height, state.InitialHeight)
	}
	if state.LastBlockHeight > 0 && block.Height != state.LastBlockHeight+1 {
		return fmt.Errorf("wrong Block.Header.Height. Expected %v, got %v",
			state.LastBlockHeight+1,
			block.Height,
		)
	}
	// Validate prev block info.
	if !block.LastBlockID.Equals(state.LastBlockID) {
		return fmt.Errorf("wrong Block.Header.LastBlockID.  Expected %v, got %v",
			state.LastBlockID,
			block.LastBlockID,
		)
	}
	// Validate app info
	if !bytes.Equal(block.AppHash, state.AppHash) {
		return fmt.Errorf("wrong Block.Header.AppHash.  Expected %X, got %v",
			state.AppHash,
			block.AppHash,
		)
	}
	if !bytes.Equal(block.ConsensusHash, state.ConsensusParams.Hash()) {
		return fmt.Errorf("wrong Block.Header.ConsensusHash.  Expected %X, got %v",
			state.ConsensusParams.Hash(),
			block.ConsensusHash,
		)
	}
	if !bytes.Equal(block.LastResultsHash, state.LastResultsHash) {
		return fmt.Errorf("wrong Block.Header.LastResultsHash.  Expected %X, got %v",
			state.LastResultsHash,
			block.LastResultsHash,
		)
	}
	if !bytes.Equal(block.ValidatorsHash, state.Validators.Hash()) {
		return fmt.Errorf("wrong Block.Header.ValidatorsHash.  Expected %X, got %v",
			state.Validators.Hash(),
			block.ValidatorsHash,
		)
	}
	if !bytes.Equal(block.NextValidatorsHash, state.NextValidators.Hash()) {
		return fmt.Errorf("wrong Block.Header.NextValidatorsHash.  Expected %X, got %v",
			state.NextValidators.Hash(),
			block.NextValidatorsHash,
		)
	}

	// Validate block LastCommit.
	if block.Height == state.InitialHeight {
		if len(block.LastCommit.Signatures) != 0 {
			return errors.New("initial block can't have LastCommit signatures")
		}
	}

	// NOTE: We can't actually verify it's the right proposer because we don't
	// know what round the block was first proposed. So just check that it's
	// a legit address and a known validator.
	if len(block.ProposerAddress) != crypto.AddressSize {
		return fmt.Errorf("expected ProposerAddress size %d, got %d",
			crypto.AddressSize,
			len(block.ProposerAddress),
		)
	}

	// Validate block Time
	switch {
	case block.Height > state.InitialHeight:
		if !(block.Time.After(state.LastBlockTime) || block.Time.Equal(state.LastBlockTime)) {
			return fmt.Errorf("block time %v not greater than or equal to last block time %v",
				block.Time,
				state.LastBlockTime,
			)
		}
		medianTime := MedianTime(block.LastCommit, state.LastValidators)
		if !block.Time.Equal(medianTime) {
			return fmt.Errorf("invalid block time. Expected %v, got %v",
				medianTime,
				block.Time,
			)
		}

	case block.Height == state.InitialHeight:
		genesisTime := state.LastBlockTime
		if !block.Time.Equal(genesisTime) {
			return fmt.Errorf("block time %v is not equal to genesis time %v",
				block.Time,
				genesisTime,
			)
		}

	default:
		return fmt.Errorf("block height %v lower than initial height %v",
			block.Height, state.InitialHeight)
	}

	// Check evidence doesn't exceed the limit amount of bytes.
	if max, got := state.ConsensusParams.Evidence.MaxBytes, block.Evidence.ByteSize(); got > max {
		return types.NewErrEvidenceOverflow(max, got)
	}

	return nil
}

// MedianTime computes a median time for a given Commit (based on Timestamp field of votes messages) and the
// corresponding validator set. The computed time is always between timestamps of
// the votes sent by honest processes, i.e., a faulty processes can not arbitrarily increase or decrease the
// computed value.
func MedianTime(commit *types.Commit, validators *types.ValidatorSet) time.Time {
	weightedTimes := make([]*cmttime.WeightedTime, len(commit.Signatures))
	totalVotingPower := int64(0)

	for i, commitSig := range commit.Signatures {
		if commitSig.BlockIDFlag == types.BlockIDFlagAbsent {
			continue
		}
		_, validator := validators.GetByAddress(commitSig.ValidatorAddress)
		// If there's no condition, TestValidateBlockCommit panics; not needed normally.
		if validator != nil {
			totalVotingPower += validator.VotingPower
			weightedTimes[i] = cmttime.NewWeightedTime(commitSig.Timestamp, validator.VotingPower)
		}
	}

	return cmttime.WeightedMedian(weightedTimes, totalVotingPower)
}
