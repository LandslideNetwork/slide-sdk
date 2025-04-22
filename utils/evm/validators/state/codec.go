// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"math"

	"github.com/landslidenetwork/slide-sdk/utils/codec"
	"github.com/landslidenetwork/slide-sdk/utils/codec/linearcodec"
)

var vdrCodec codec.Manager

func init() {
	lc := linearcodec.NewDefault()
	err := lc.RegisterType(validatorData{})
	if err != nil {
		panic(err)
	}
	vdrCodec = codec.NewManager(math.MaxInt32, lc)
}
