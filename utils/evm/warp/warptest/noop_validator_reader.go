// (c) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// warptest exposes common functionality for testing the warp package.
package warptest

import (
	"time"

	"github.com/landslidenetwork/slide-sdk/utils/evm/validators/interfaces"
	stateinterfaces "github.com/landslidenetwork/slide-sdk/utils/evm/validators/state/interfaces"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

var _ interfaces.ValidatorReader = &NoOpValidatorReader{}

type NoOpValidatorReader struct{}

func (NoOpValidatorReader) GetValidatorAndUptime(ids.ID) (stateinterfaces.Validator, time.Duration, time.Time, error) {
	return stateinterfaces.Validator{}, 0, time.Time{}, nil
}
