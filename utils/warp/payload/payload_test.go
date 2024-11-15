// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package payload

import (
	"github.com/cometbft/cometbft/libs/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

var junkBytes = []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88}

func TestParseJunk(t *testing.T) {
	require := require.New(t)
	_, err := Parse(junkBytes)
	require.Error(err)
}

func TestParseWrongPayloadType(t *testing.T) {
	require := require.New(t)
	hash, err := ids.ToID([]byte(rand.Str(32)))
	require.NoError(err)
	hashPayload, err := NewHash(hash)
	require.NoError(err)

	shortID := []byte(rand.Str(20))
	addressedPayload, err := NewAddressedCall(
		shortID[:],
		[]byte{1, 2, 3},
	)
	require.NoError(err)

	_, err = ParseAddressedCall(hashPayload.Bytes())
	require.ErrorIs(err, errWrongType)

	_, err = ParseHash(addressedPayload.Bytes())
	require.ErrorIs(err, errWrongType)
}

func TestParse(t *testing.T) {
	require := require.New(t)
	hashPayload, err := NewHash(ids.ID{4, 5, 6})
	require.NoError(err)

	parsedHashPayload, err := Parse(hashPayload.Bytes())
	require.NoError(err)
	require.Equal(hashPayload, parsedHashPayload)

	addressedPayload, err := NewAddressedCall(
		[]byte{1, 2, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		[]byte{10, 11, 12},
	)
	require.NoError(err)

	parsedAddressedPayload, err := Parse(addressedPayload.Bytes())
	require.NoError(err)
	require.Equal(addressedPayload, parsedAddressedPayload)
}
