// (c) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package aggregator

import (
	"context"
	"fmt"
	"github.com/landslidenetwork/slide-sdk/utils/warp/messages"
	"time"

	"github.com/landslidenetwork/slide-sdk/utils/crypto/bls"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	avalancheWarp "github.com/landslidenetwork/slide-sdk/utils/warp"
	"github.com/landslidenetwork/slide-sdk/utils/warp/payload"
)

const (
	initialRetryFetchSignatureDelay = 100 * time.Millisecond
	maxRetryFetchSignatureDelay     = 5 * time.Second
	retryBackoffFactor              = 2
)

var _ SignatureGetter = (*NetworkSignatureGetter)(nil)

// SignatureGetter defines the minimum network interface to perform signature aggregation
type SignatureGetter interface {
	// GetSignature attempts to fetch a BLS Signature from [nodeID] for [unsignedWarpMessage]
	GetSignature(ctx context.Context, nodeID ids.NodeID, unsignedWarpMessage *avalancheWarp.UnsignedMessage) (*bls.Signature, error)
}

type NetworkClient interface {
	SendAppRequest(ctx context.Context, nodeID ids.NodeID, message []byte) ([]byte, error)
}

// NetworkSignatureGetter fetches warp signatures on behalf of the
// aggregator using VM App-Specific Messaging
type NetworkSignatureGetter struct {
	Client NetworkClient
}

func NewSignatureGetter(client NetworkClient) *NetworkSignatureGetter {
	return &NetworkSignatureGetter{
		Client: client,
	}
}

// GetSignature attempts to fetch a BLS Signature of [unsignedWarpMessage] from [nodeID] until it succeeds or receives an invalid response
//
// Note: this function will continue attempting to fetch the signature from [nodeID] until it receives an invalid value or [ctx] is cancelled.
// The caller is responsible to cancel [ctx] if it no longer needs to fetch this signature.
func (s *NetworkSignatureGetter) GetSignature(ctx context.Context, nodeID ids.NodeID, unsignedWarpMessage *avalancheWarp.UnsignedMessage) (*bls.Signature, error) {
	var signatureReqBytes []byte
	parsedPayload, err := payload.Parse(unsignedWarpMessage.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to parse unsigned message payload: %w", err)
	}
	switch p := parsedPayload.(type) {
	case *payload.AddressedCall:
		signatureReq := messages.MessageSignatureRequest{
			MessageID: unsignedWarpMessage.ID(),
		}
		signatureReqBytes, err = messages.RequestToBytes(messages.Codec, signatureReq)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal signature request: %w", err)
		}
	case *payload.Hash:
		signatureReq := messages.BlockSignatureRequest{
			BlockID: p.Hash,
		}
		signatureReqBytes, err = messages.RequestToBytes(messages.Codec, signatureReq)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal signature request: %w", err)
		}
	}

	delay := initialRetryFetchSignatureDelay
	timer := time.NewTimer(delay)
	defer timer.Stop()
	for {
		signatureRes, err := s.Client.SendAppRequest(ctx, nodeID, signatureReqBytes)
		// If the client fails to retrieve a response perform an exponential backoff.
		// Note: it is up to the caller to ensure that [ctx] is eventually cancelled
		if err != nil {
			// Wait until the retry delay has elapsed before retrying.
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(delay)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-timer.C:
			}

			// Exponential backoff.
			delay *= retryBackoffFactor
			if delay > maxRetryFetchSignatureDelay {
				delay = maxRetryFetchSignatureDelay
			}
			continue
		}
		var response messages.SignatureResponse
		if err = messages.Codec.Unmarshal(signatureRes, &response); err != nil {
			return nil, fmt.Errorf("failed to unmarshal signature res: %w", err)
		}
		if response.Signature == [bls.SignatureLen]byte{} {
			return nil, fmt.Errorf("received empty signature response")
		}
		blsSignature, err := bls.SignatureFromBytes(response.Signature[:])
		if err != nil {
			return nil, fmt.Errorf("failed to parse signature from res: %w", err)
		}
		return blsSignature, nil
	}
}
