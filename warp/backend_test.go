// (c) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package warp

import (
	"os"
	"testing"

	dbm "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/libs/log"
	"github.com/cometbft/cometbft/libs/rand"

	"github.com/landslidenetwork/slide-sdk/utils/crypto/bls"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	warputils "github.com/landslidenetwork/slide-sdk/utils/warp"
	"github.com/stretchr/testify/require"
)

var (
	err                 error
	networkID           uint32 = 54321
	sourceChainID              = ids.GenerateTestID()
	testUnsignedMessage *warputils.UnsignedMessage
)

func init() {
	testUnsignedMessage, err = warputils.NewUnsignedMessage(networkID, sourceChainID, []byte(rand.Str(30)))
	if err != nil {
		panic(err)
	}
}

func TestAddAndGetValidMessage(t *testing.T) {
	logger := log.NewTMLogger(os.Stdout)
	db := dbm.NewMemDB()

	sk, err := bls.NewSecretKey()
	require.NoError(t, err)
	warpSigner := warputils.NewSigner(sk, networkID, sourceChainID)
	backend := NewBackend(networkID, sourceChainID, warpSigner, logger, db)
	require.NoError(t, err)

	// Add testUnsignedMessage to the warp backend
	err = backend.AddMessage(testUnsignedMessage)
	require.NoError(t, err)

	// Verify that a signature is returned successfully, and compare to expected signature.
	msg, err := backend.GetMessage(testUnsignedMessage.ID())
	require.NoError(t, err)
	require.NotNil(t, msg)

	expectedSig, err := warpSigner.Sign(testUnsignedMessage)
	require.NoError(t, err)
	require.NotNil(t, expectedSig)

	// Verify that a signature is returned successfully, and compare to expected signature.
	signature, err := backend.GetMessageSignature(testUnsignedMessage)
	require.NoError(t, err)
	require.NoError(t, err)
	require.Equal(t, expectedSig, signature)
}

func TestAddAndGetUnknownMessage(t *testing.T) {
	logger := log.NewTMLogger(os.Stdout)
	db := dbm.NewMemDB()

	sk, err := bls.NewSecretKey()
	require.NoError(t, err)
	warpSigner := warputils.NewSigner(sk, networkID, sourceChainID)
	backend := NewBackend(networkID, sourceChainID, warpSigner, logger, db)
	require.NoError(t, err)

	// Try getting a signature for a message that was not added.
	msg, err := backend.GetMessage(testUnsignedMessage.ID())
	require.Error(t, err)
	require.Nil(t, msg)
	// Try getting a signature for a message that was not added.
	_, err = backend.GetMessageSignature(testUnsignedMessage)
	require.Error(t, err)
}
