// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package bls

import (
	"github.com/cometbft/cometbft/crypto"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSecretKeyFromBytesZero(t *testing.T) {
	require := require.New(t)

	var skArr [SecretKeyLen]byte
	skBytes := skArr[:]
	_, err := SecretKeyFromBytes(skBytes)
	require.ErrorIs(err, errFailedSecretKeyDeserialize)
}

func TestSecretKeyFromBytesWrongSize(t *testing.T) {
	require := require.New(t)

	skBytes := crypto.CRandBytes(SecretKeyLen + 1)
	_, err := SecretKeyFromBytes(skBytes)
	require.ErrorIs(err, errFailedSecretKeyDeserialize)
}

func TestSecretKeyBytes(t *testing.T) {
	require := require.New(t)

	msg := crypto.CRandBytes(1234)

	sk, err := NewSecretKey()
	require.NoError(err)
	sig := Sign(sk, msg)
	skBytes := SecretKeyToBytes(sk)

	sk2, err := SecretKeyFromBytes(skBytes)
	require.NoError(err)
	sig2 := Sign(sk2, msg)
	sk2Bytes := SecretKeyToBytes(sk2)

	require.Equal(sk, sk2)
	require.Equal(skBytes, sk2Bytes)
	require.Equal(sig, sig2)
}
