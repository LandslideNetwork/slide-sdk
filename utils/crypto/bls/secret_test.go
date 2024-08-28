package bls

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/consideritdone/landslidevm/utils"
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

	skBytes := utils.RandomBytes(SecretKeyLen + 1)
	_, err := SecretKeyFromBytes(skBytes)
	require.ErrorIs(err, errFailedSecretKeyDeserialize)
}

func TestSecretKeyBytes(t *testing.T) {
	require := require.New(t)

	msg := utils.RandomBytes(1234)

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
