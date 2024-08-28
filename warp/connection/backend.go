// (c) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package connection

import (
	"context"
	"fmt"
	dbm "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/libs/log"
	"github.com/cometbft/cometbft/store"
	"github.com/consideritdone/landslidevm/database"
	vmpb "github.com/consideritdone/landslidevm/proto/vm"
	"github.com/consideritdone/landslidevm/utils/crypto/bls"
	"github.com/consideritdone/landslidevm/utils/ids"
	vmstate "github.com/consideritdone/landslidevm/vm/types/state"
	"github.com/consideritdone/landslidevm/warp"
)

var _ Backend = &backend{}

const batchSize = 100 * 1024

type BlockClient interface {
	GetBlock(ctx context.Context, blockID ids.ID) (vmpb.Block, error)
}

// Backend tracks signature-eligible warp messages and provides an interface to fetch them.
// The backend is also used to query for warp message signatures by the signature request handler.
type Backend interface {
	//// AddMessage signs [unsignedMessage] and adds it to the warp backend database
	//AddMessage(unsignedMessage *avalancheWarp.UnsignedMessage) error
	//
	//// GetMessageSignature returns the signature of the requested message hash.
	//GetMessageSignature(messageID ids.ID) ([bls.SignatureLen]byte, error)

	// GetBlockSignature returns the signature of the requested message hash.
	GetBlockSignature(blockID ids.ID) ([bls.SignatureLen]byte, error)
	//
	//// GetMessage retrieves the [unsignedMessage] from the warp backend database if available
	//GetMessage(messageHash ids.ID) (*avalancheWarp.UnsignedMessage, error)
	//
	//// Clear clears the entire db
	//Clear() error
}

// backend implements Backend, keeps track of warp messages, and generates message signatures.
type backend struct {
	networkID     uint32
	sourceChainID ids.ID
	db            dbm.DB
	logger        log.Logger
	warpSigner    warp.Signer
	wrappedBlocks *vmstate.WrappedBlocksStorage
	blockStore    *store.BlockStore
}

// NewBackend creates a new Backend, and initializes the signature cache and message tracking database.
func NewBackend(networkID uint32, sourceChainID ids.ID,
	warpSigner warp.Signer,
	blockStore *store.BlockStore,
	wrappedBlocks *vmstate.WrappedBlocksStorage,
	db dbm.DB,
) Backend {
	return &backend{
		networkID:     networkID,
		sourceChainID: sourceChainID,
		db:            db,
		blockStore:    blockStore,
		warpSigner:    warpSigner,
		wrappedBlocks: wrappedBlocks,
	}
}

func (b *backend) Clear() error {
	return database.Clear(b.db)
}

//
//func (b *backend) AddMessage(unsignedMessage *avalancheWarp.UnsignedMessage) error {
//	messageID := unsignedMessage.ID()
//
//	// In the case when a node restarts, and possibly changes its bls key, the cache gets emptied but the database does not.
//	// So to avoid having incorrect signatures saved in the database after a bls key change, we save the full message in the database.
//	// Whereas for the cache, after the node restart, the cache would be emptied so we can directly save the signatures.
//	if err := b.db.Put(messageID[:], unsignedMessage.Bytes()); err != nil {
//		return fmt.Errorf("failed to put warp signature in db: %w", err)
//	}
//
//	var signature [bls.SignatureLen]byte
//	sig, err := b.warpSigner.Sign(unsignedMessage)
//	if err != nil {
//		return fmt.Errorf("failed to sign warp message: %w", err)
//	}
//
//	copy(signature[:], sig)
//	b.messageSignatureCache.Put(messageID, signature)
//	log.Debug("Adding warp message to backend", "messageID", messageID)
//	return nil
//}
//
//func (b *backend) GetMessageSignature(messageID ids.ID) ([bls.SignatureLen]byte, error) {
//	log.Debug("Getting warp message from backend", "messageID", messageID)
//	if sig, ok := b.messageSignatureCache.Get(messageID); ok {
//		return sig, nil
//	}
//
//	unsignedMessage, err := b.GetMessage(messageID)
//	if err != nil {
//		return [bls.SignatureLen]byte{}, fmt.Errorf("failed to get warp message %s from db: %w", messageID.String(), err)
//	}
//
//	var signature [bls.SignatureLen]byte
//	sig, err := b.warpSigner.Sign(unsignedMessage)
//	if err != nil {
//		return [bls.SignatureLen]byte{}, fmt.Errorf("failed to sign warp message: %w", err)
//	}
//
//	copy(signature[:], sig)
//	b.messageSignatureCache.Put(messageID, signature)
//	return signature, nil
//}

func (b *backend) GetBlockSignature(blockID ids.ID) ([bls.SignatureLen]byte, error) {
	b.logger.Debug("Getting block from backend", "blockID", blockID)

	wblk, ok := b.wrappedBlocks.GetCachedBlock(blockID)
	if !ok {
		if _, ok := b.wrappedBlocks.MissingBlocks.Get(blockID); ok {
			return [bls.SignatureLen]byte{}, fmt.Errorf("ERROR_NOT_FOUND")
		}

		blk := b.blockStore.LoadBlockByHash([]byte(blockID.String()))
		if blk == nil {
			b.wrappedBlocks.MissingBlocks.Put(blockID, struct{}{})
			return [bls.SignatureLen]byte{}, fmt.Errorf("ERROR_NOT_FOUND")
		}

		wblk = &vmstate.WrappedBlock{
			Block:  blk,
			Status: vmpb.Status_STATUS_ACCEPTED,
		}
	}
	if wblk.Status != vmpb.Status_STATUS_ACCEPTED {
		return [bls.SignatureLen]byte{}, fmt.Errorf("block %s was not accepted", blockID)
	}

	var signature [bls.SignatureLen]byte
	unsignedMessage, err := warp.NewUnsignedMessage(b.networkID, b.sourceChainID, wblk.Block.Hash().Bytes())
	if err != nil {
		return [bls.SignatureLen]byte{}, fmt.Errorf("failed to create new unsigned warp message: %w", err)
	}
	sig, err := b.warpSigner.Sign(unsignedMessage)
	if err != nil {
		return [bls.SignatureLen]byte{}, fmt.Errorf("failed to sign warp message: %w", err)
	}

	copy(signature[:], sig)
	return signature, nil
}

//
//func (b *backend) GetMessage(messageID ids.ID) (*avalancheWarp.UnsignedMessage, error) {
//	if message, ok := b.messageCache.Get(messageID); ok {
//		return message, nil
//	}
//
//	unsignedMessageBytes, err := b.db.Get(messageID[:])
//	if err != nil {
//		return nil, fmt.Errorf("failed to get warp message %s from db: %w", messageID.String(), err)
//	}
//
//	unsignedMessage, err := avalancheWarp.ParseUnsignedMessage(unsignedMessageBytes)
//	if err != nil {
//		return nil, fmt.Errorf("failed to parse unsigned message %s: %w", messageID.String(), err)
//	}
//	b.messageCache.Put(messageID, unsignedMessage)
//
//	return unsignedMessage, nil
//}
