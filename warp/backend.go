package warp

import (
	"context"
	"fmt"

	"github.com/landslidenetwork/slide-sdk/utils/avalanche/network/acp118"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/warp"
	payload2 "github.com/landslidenetwork/slide-sdk/utils/avalanche/warp/payload"

	"github.com/landslidenetwork/slide-sdk/utils/evm/validators/interfaces"
	"github.com/landslidenetwork/slide-sdk/utils/evm/warp/messages"

	dbm "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/libs/log"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

// Backend tracks signature-eligible warp messages and provides an interface to fetch them.
// The backend is also used to query for warp message signatures by the signature request handler.
type Backend interface {
	// AddMessage signs [unsignedMessage] and adds it to the warp backend database
	AddMessage(unsignedMessage *warp.UnsignedMessage) error
	// GetMessageSignature returns the signature of the requested message.
	GetMessageSignature(message *warp.UnsignedMessage) ([]byte, error)
	// GetBlockSignature returns the signature of a hash payload containing blockID if it's the ID of an accepted block.
	GetBlockSignature(blockID ids.ID) ([]byte, error)
	// GetMessage retrieves the [unsignedMessage] from the warp backend database if available
	// TODO: After E-Upgrade, the backend no longer needs to store the mapping from messageHash
	// to unsignedMessage (and this method can be removed).
	GetMessage(messageHash ids.ID) (*warp.UnsignedMessage, error)

	acp118.Verifier
}

// backend implements Backend, keeps track of warp messages, and generates message signatures.
type backend struct {
	logger          log.Logger
	blockClient     BlockClient
	validatorReader interfaces.ValidatorReader
	networkID       uint32
	sourceChainID   ids.ID
	db              dbm.DB
	warpSigner      warp.Signer
	stats           *verifierStats
}

// NewBackend creates a new Backend, and initializes the signature cache and message tracking database.
func NewBackend(networkID uint32, sourceChainID ids.ID, warpSigner warp.Signer, logger log.Logger, db dbm.DB, blkReceiver BlockReceiver, validatorReader interfaces.ValidatorReader) Backend {
	return &backend{
		networkID:       networkID,
		sourceChainID:   sourceChainID,
		warpSigner:      warpSigner,
		logger:          logger,
		db:              db,
		stats:           newVerifierStats(),
		blockClient:     &blockStorageClient{receiver: blkReceiver},
		validatorReader: validatorReader,
	}
}

func (b *backend) AddMessage(unsignedMessage *warp.UnsignedMessage) error {
	messageID := unsignedMessage.ID()

	// In the case when a node restarts, and possibly changes its bls key, the cache gets emptied but the database does not.
	// So to avoid having incorrect signatures saved in the database after a bls key change, we save the full message in the database.
	// Whereas for the cache, after the node restart, the cache would be emptied so we can directly save the signatures.
	if err := b.db.Set(messageID[:], unsignedMessage.Bytes()); err != nil {
		return fmt.Errorf("failed to put warp signature in db: %w", err)
	}

	if _, err := b.signMessage(unsignedMessage); err != nil {
		return fmt.Errorf("failed to sign warp message: %w", err)
	}
	//TODO: save message signature to prefixdb
	b.logger.Debug("Adding warp message to backend", "messageID", messageID)
	return nil
}

func (b *backend) GetMessage(messageID ids.ID) (*warp.UnsignedMessage, error) {
	unsignedMessageBytes, err := b.db.Get(messageID[:])
	if err != nil {
		return nil, fmt.Errorf("failed to get warp message %s from db: %w", messageID.String(), err)
	}

	unsignedMessage, err := warp.ParseUnsignedMessage(unsignedMessageBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse unsigned message %s: %w", messageID.String(), err)
	}

	return unsignedMessage, nil
}

func (b *backend) GetMessageSignature(unsignedMessage *warp.UnsignedMessage) ([]byte, error) {
	messageID := unsignedMessage.ID()

	b.logger.Debug("Getting warp message from backend", "messageID", messageID)
	if err := b.Verify(context.Background(), unsignedMessage, nil); err != nil {
		return nil, fmt.Errorf("failed to validate warp message: %w", err)
	}
	if err := b.ValidateMessage(unsignedMessage); err != nil {
		return []byte{}, fmt.Errorf("failed to validate warp message: %w", err)
	}

	return b.signMessage(unsignedMessage)
}

func (b *backend) GetBlockSignature(blockID ids.ID) ([]byte, error) {
	b.logger.Debug("Getting block from backend", "blockID", blockID)

	blockHashPayload, err := payload2.NewHash(blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to create new block hash payload: %w", err)
	}

	unsignedMessage, err := warp.NewUnsignedMessage(b.networkID, b.sourceChainID, blockHashPayload.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to create new unsigned warp message: %w", err)
	}

	if err := b.verifyBlockMessage(context.Background(), blockHashPayload); err != nil {
		return nil, fmt.Errorf("failed to validate block message: %w", err)
	}

	sig, err := b.warpSigner.Sign(unsignedMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to sign warp message: %w", err)
	}
	return sig, nil
}

func (b *backend) ValidateMessage(unsignedMessage *warp.UnsignedMessage) error {
	// Known on-chain messages should be signed
	if _, err := b.GetMessage(unsignedMessage.ID()); err == nil {
		return nil
	}

	// Try to parse the payload as an AddressedCall
	addressedCall, err := payload2.ParseAddressedCall(unsignedMessage.Payload)
	if err != nil {
		return fmt.Errorf("failed to parse unknown message as AddressedCall: %w", err)
	}

	// Further, parse the payload to see if it is a known type.
	parsed, err := messages.Parse(addressedCall.Payload)
	if err != nil {
		return fmt.Errorf("failed to parse unknown message: %w", err)
	}

	// Check if the message is a known type that can be signed on demand
	signable, ok := parsed.(messages.Signable)
	if !ok {
		return fmt.Errorf("parsed message is not Signable: %T", signable)
	}

	// Check if the message should be signed according to its type
	if err := signable.VerifyMessage(addressedCall.SourceAddress); err != nil {
		return fmt.Errorf("failed to verify Signable message: %w", err)
	}
	return nil
}

func (b *backend) signMessage(unsignedMessage *warp.UnsignedMessage) ([]byte, error) {
	sig, err := b.warpSigner.Sign(unsignedMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to sign warp message: %w", err)
	}

	return sig, nil
}
