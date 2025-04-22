// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package interfaces

import (
	"context"
	"time"

	avalancheuptime "github.com/landslidenetwork/slide-sdk/utils/avalanche/uptime"
	stateinterfaces "github.com/landslidenetwork/slide-sdk/utils/evm/validators/state/interfaces"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

type ValidatorReader interface {
	// GetValidatorAndUptime returns the uptime of the validator specified by validationID
	GetValidatorAndUptime(validationID ids.ID) (stateinterfaces.Validator, time.Duration, time.Time, error)
}

type Manager interface {
	stateinterfaces.State
	avalancheuptime.Manager
	ValidatorReader

	// Sync updates the validator set managed
	// by the manager
	Sync(ctx context.Context) error
	// DispatchSync starts the sync process
	DispatchSync(ctx context.Context)
}
