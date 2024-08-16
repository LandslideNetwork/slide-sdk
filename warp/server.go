package warp

import (
	"github.com/consideritdone/landslidevm/database"
	"github.com/consideritdone/landslidevm/utils/ids"
)

var _ Server = &server{}

//
//const batchSize = ethdb.IdealBatchSize
//
//type BlockClient interface {
//	GetBlock(ctx context.Context, blockID ids.ID) (snowman.Block, error)
//}

// Server tracks signature-eligible warp messages and provides an interface to fetch them.
// The server is also used to query for warp message signatures by the signature request handler.
type Server interface {
	//// AddMessage signs [unsignedMessage] and adds it to the warp server database
	//AddMessage(unsignedMessage *avalancheWarp.UnsignedMessage) error
	//
	//// GetMessageSignature returns the signature of the requested message hash.
	//GetMessageSignature(messageID ids.ID) ([bls.SignatureLen]byte, error)
	//
	//// GetBlockSignature returns the signature of the requested message hash.
	//GetBlockSignature(blockID ids.ID) ([bls.SignatureLen]byte, error)
	//
	//// GetMessage retrieves the [unsignedMessage] from the warp server database if available
	//GetMessage(messageHash ids.ID) (*avalancheWarp.UnsignedMessage, error)

	// Clear clears the entire db
	Clear() error
}

// server implements Server, keeps track of warp messages, and generates message signatures.
type server struct {
	networkID     uint32
	sourceChainID ids.ID
	db            database.Database
	//warpSigner            avalancheWarp.Signer
	//blockClient           BlockClient
	//messageSignatureCache *cache.LRU[ids.ID, [bls.SignatureLen]byte]
	//blockSignatureCache   *cache.LRU[ids.ID, [bls.SignatureLen]byte]
	//messageCache          *cache.LRU[ids.ID, *avalancheWarp.UnsignedMessage]
}

// NewServer creates a new Server, and initializes the signature cache and message tracking database.
func NewServer(networkID uint32, sourceChainID ids.ID,
//warpSigner avalancheWarp.Signer, blockClient BlockClient,
	db database.Database, cacheSize int) Server {
	return &server{
		networkID:     networkID,
		sourceChainID: sourceChainID,
		db:            db,
		//warpSigner:            warpSigner,
		//blockClient:           blockClient,
		//messageSignatureCache: &cache.LRU[ids.ID, [bls.SignatureLen]byte]{Size: cacheSize},
		//blockSignatureCache:   &cache.LRU[ids.ID, [bls.SignatureLen]byte]{Size: cacheSize},
		//messageCache:          &cache.LRU[ids.ID, *avalancheWarp.UnsignedMessage]{Size: cacheSize},
	}
}

func (s *server) Clear() error {
	//b.messageSignatureCache.Flush()
	//b.blockSignatureCache.Flush()
	//b.messageCache.Flush()
	//return database.Clear(b.db, batchSize)
	return nil
}
