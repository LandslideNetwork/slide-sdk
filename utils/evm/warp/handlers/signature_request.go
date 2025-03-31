// (c) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package handlers

import (
	"context"
	"github.com/cometbft/cometbft/libs/log"
	"github.com/landslidenetwork/slide-sdk/utils/message"
	"time"

	"github.com/landslidenetwork/slide-sdk/utils/codec"
	"github.com/landslidenetwork/slide-sdk/utils/crypto/bls"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	"github.com/landslidenetwork/slide-sdk/warp"
)

// SignatureRequestHandler serves warp signature requests. It is a peer.RequestHandler for message.MessageSignatureRequest.
// TODO: After Etna, this handler can be removed and SignatureRequestHandlerP2P is sufficient.
type SignatureRequestHandler struct {
	backend warp.Backend
	codec   codec.Manager
	stats   *handlerStats
	log     log.Logger
}

func NewSignatureRequestHandler(backend warp.Backend, codec codec.Manager, logger log.Logger) *SignatureRequestHandler {
	return &SignatureRequestHandler{
		backend: backend,
		codec:   codec,
		stats:   newStats(),
		log:     logger,
	}
}

// OnMessageSignatureRequest handles message.MessageSignatureRequest, and retrieves a warp signature for the requested message ID.
// Never returns an error
// Expects returned errors to be treated as FATAL
// Returns empty response if signature is not found
// Assumes ctx is active
func (s *SignatureRequestHandler) OnMessageSignatureRequest(ctx context.Context, nodeID ids.NodeID, requestID uint32, signatureRequest message.MessageSignatureRequest) ([]byte, error) {
	s.log.Debug("OnMessageSignatureRequest START", "messageID", signatureRequest.MessageID)
	startTime := time.Now()
	s.stats.IncMessageSignatureRequest()

	// Always report signature request time
	defer func() {
		s.stats.UpdateMessageSignatureRequestTime(time.Since(startTime))
	}()

	var signature [bls.SignatureLen]byte
	unsignedMessage, err := s.backend.GetMessage(signatureRequest.MessageID)
	if err != nil {
		s.log.Debug("Unknown warp message requested", "messageID", signatureRequest.MessageID)
		s.stats.IncMessageSignatureMiss()
	} else {
		sig, err := s.backend.GetMessageSignature(unsignedMessage)
		if err != nil {
			s.log.Debug("Unknown warp signature requested", "messageID", signatureRequest.MessageID)
			s.stats.IncMessageSignatureMiss()
		} else {
			s.log.Debug("GetMessageSignature Status: success", "signature", sig)
			s.stats.IncMessageSignatureHit()
			copy(signature[:], sig)
		}
		//TODO: remove signature logging
		s.log.Debug("WARP SIGNATURE INITIAL VALUE", "signature", signature)
		s.log.Debug("WARP SIGNATURE INITIAL VALUE NR2", "signature", sig)
		s.log.Debug("WARP UNSIGNED MESSAGE INITIAL VALUE", "unsignedMessage", unsignedMessage)
	}
	s.log.Debug("GetMessageSignature Copy Signature", "signatureAfterCopy", signature)
	response := message.SignatureResponse{Signature: signature}
	responseBytes, err := s.codec.Marshal(&response)
	if err != nil {
		s.log.Error("could not marshal SignatureResponse, dropping request", "nodeID", nodeID, "requestID", requestID, "err", err)
		return nil, nil
	}
	s.log.Debug("GetMessageSignature Response Ready", "responseBytes", responseBytes)
	return responseBytes, nil
}

func (s *SignatureRequestHandler) OnBlockSignatureRequest(ctx context.Context, nodeID ids.NodeID, requestID uint32, request message.BlockSignatureRequest) ([]byte, error) {
	startTime := time.Now()
	s.stats.IncBlockSignatureRequest()

	// Always report signature request time
	defer func() {
		s.stats.UpdateBlockSignatureRequestTime(time.Since(startTime))
	}()

	var signature [bls.SignatureLen]byte
	sig, err := s.backend.GetBlockSignature(request.BlockID)
	if err != nil {
		s.log.Debug("Unknown warp signature requested", "blockID", request.BlockID)
		s.stats.IncBlockSignatureMiss()
	} else {
		s.stats.IncBlockSignatureHit()
		copy(signature[:], sig)
	}

	response := message.SignatureResponse{Signature: signature}
	responseBytes, err := s.codec.Marshal(&response)
	if err != nil {
		s.log.Error("could not marshal SignatureResponse, dropping request", "nodeID", nodeID, "requestID", requestID, "err", err)
		return nil, nil
	}

	return responseBytes, nil
}

type NoopSignatureRequestHandler struct{}

func (s *NoopSignatureRequestHandler) OnMessageSignatureRequest(ctx context.Context, nodeID ids.NodeID, requestID uint32, signatureRequest message.MessageSignatureRequest) ([]byte, error) {
	return nil, nil
}

func (s *NoopSignatureRequestHandler) OnBlockSignatureRequest(ctx context.Context, nodeID ids.NodeID, requestID uint32, signatureRequest message.BlockSignatureRequest) ([]byte, error) {
	return nil, nil
}
