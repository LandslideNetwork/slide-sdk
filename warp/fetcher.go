// (c) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package warp

import (
	"context"
	"fmt"

	"github.com/landslidenetwork/slide-sdk/utils/crypto/bls"
	warputils "github.com/landslidenetwork/slide-sdk/utils/evm/warp"
	"github.com/landslidenetwork/slide-sdk/utils/evm/warp/aggregator"
	"github.com/landslidenetwork/slide-sdk/utils/evm/warp/payload"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

var _ aggregator.SignatureGetter = (*apiFetcher)(nil)

type apiFetcher struct {
	clients map[ids.NodeID]Client
}

func NewAPIFetcher(clients map[ids.NodeID]Client) *apiFetcher {
	return &apiFetcher{
		clients: clients,
	}
}

func (f *apiFetcher) GetSignature(ctx context.Context, nodeID ids.NodeID, unsignedWarpMessage *warputils.UnsignedMessage) (*bls.Signature, error) {
	client, ok := f.clients[nodeID]
	if !ok {
		return nil, fmt.Errorf("no warp client for nodeID: %s", nodeID)
	}
	var signatureBytes []byte
	parsedPayload, err := payload.Parse(unsignedWarpMessage.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to parse unsigned message payload: %w", err)
	}
	switch p := parsedPayload.(type) {
	case *payload.AddressedCall:
		signatureBytes, err = client.GetMessageSignature(ctx, unsignedWarpMessage.ID())
	case *payload.Hash:
		signatureBytes, err = client.GetBlockSignature(ctx, p.Hash)
	}
	if err != nil {
		return nil, err
	}

	signature, err := bls.SignatureFromBytes(signatureBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse signature from client %s: %w", nodeID, err)
	}
	return signature, nil
}
