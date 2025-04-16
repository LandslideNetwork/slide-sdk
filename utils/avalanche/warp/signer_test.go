// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package warp_test

import (
	"testing"

	"github.com/landslidenetwork/slide-sdk/utils/avalanche/constants"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/warp"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/warp/signertest"

	"github.com/landslidenetwork/slide-sdk/utils/crypto/bls"

	"github.com/stretchr/testify/require"

	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

func TestSigner(t *testing.T) {
	for name, test := range signertest.SignerTests {
		t.Run(name, func(t *testing.T) {
			sk, err := bls.NewSigner()
			require.NoError(t, err)

			chainID := ids.GenerateTestID()
			s := warp.NewSigner(sk, constants.UnitTestID, chainID)

			test(t, s, sk, constants.UnitTestID, chainID)
		})
	}
}
