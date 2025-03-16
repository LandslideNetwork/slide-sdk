// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package p2p

import (
	"context"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

type ValidatorSet interface {
	Has(ctx context.Context, nodeID ids.NodeID) bool // TODO return error
}
