package state

import (
	"errors"
	"fmt"

	"github.com/cometbft/cometbft/crypto"
	"github.com/cometbft/cometbft/libs/json"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cometbft/cometbft/state"
	"github.com/cometbft/cometbft/types"

	"github.com/consideritdone/landslidevm/proto/vm"
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

func DecodeBlockWithStatus(data []byte) (*types.Block, *vm.Status, error) {
	wrappedBlk := new(vm.Block)

	if err := json.Unmarshal(data, wrappedBlk); err != nil {
		return nil, nil, err
	}

	blk, err := DecodeBlock(wrappedBlk.Block)
	if err != nil {
		return nil, nil, err
	}

	return blk, &wrappedBlk.Status, nil
}

func EncodeBlock(block *types.Block) ([]byte, error) {
	proto, err := block.ToProto()
	if err != nil {
		return nil, err
	}
	return proto.Marshal()
}

func DecodeBlock(data []byte) (*types.Block, error) {
	protoBlock := new(cmtproto.Block)
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

	return nil
}
