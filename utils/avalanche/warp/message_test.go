// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package warp

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/landslidenetwork/slide-sdk/utils/crypto/bls"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

func TestMessage(t *testing.T) {
	require := require.New(t)

	unsignedMsg, err := NewUnsignedMessage(
		UnitTestID,
		ids.GenerateTestID(),
		[]byte("payload"),
	)
	require.NoError(err)

	msg, err := NewMessage(
		unsignedMsg,
		&BitSetSignature{
			Signers:   []byte{1, 2, 3},
			Signature: [bls.SignatureLen]byte{4, 5, 6},
		},
	)
	require.NoError(err)

	msgBytes := msg.Bytes()
	msg2, err := ParseMessage(msgBytes)
	require.NoError(err)
	require.Equal(msg, msg2)
}

func TestParseMessageJunk(t *testing.T) {
	bytes := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	_, err := ParseMessage(bytes)
	require.Error(t, err)
}
