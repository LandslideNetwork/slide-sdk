// (c) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package aggregator

import (
	"context"

	"github.com/landslidenetwork/slide-sdk/utils/crypto/bls"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	avalancheWarp "github.com/landslidenetwork/slide-sdk/utils/warp"
)

// SignatureGetter defines the minimum network interface to perform signature aggregation
type SignatureGetter interface {
	// GetSignature attempts to fetch a BLS Signature from [nodeID] for [unsignedWarpMessage]
	GetSignature(ctx context.Context, nodeID ids.NodeID, unsignedWarpMessage *avalancheWarp.UnsignedMessage) (*bls.Signature, error)
}
