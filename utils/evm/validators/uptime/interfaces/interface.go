// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package interfaces

import (
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/uptime"
	validatorsstateinterfaces "github.com/landslidenetwork/slide-sdk/utils/evm/validators/state/interfaces"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

type PausableManager interface {
	uptime.Manager
	validatorsstateinterfaces.StateCallbackListener
	IsPaused(nodeID ids.NodeID) bool
}
