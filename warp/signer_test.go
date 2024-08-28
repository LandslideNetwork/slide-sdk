package warp

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/consideritdone/landslidevm/utils/crypto/bls"
	"github.com/consideritdone/landslidevm/utils/ids"

	"github.com/consideritdone/landslidevm/utils/constants"
)

func TestSigner(t *testing.T) {
	for name, test := range SignerTests {
		t.Run(name, func(t *testing.T) {
			sk, err := bls.NewSecretKey()
			require.NoError(t, err)

			chainID := ids.GenerateTestID()
			s := NewSigner(sk, constants.UnitTestID, chainID)

			test(t, s, sk, constants.UnitTestID, chainID)
		})
	}
}
