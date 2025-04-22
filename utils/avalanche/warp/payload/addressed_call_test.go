// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package payload

import (
	"encoding/base64"
	"testing"

	"github.com/cometbft/cometbft/libs/rand"

	"github.com/stretchr/testify/require"
)

func TestAddressedCall(t *testing.T) {
	require := require.New(t)
	shortID := []byte(rand.Str(2))

	addressedPayload, err := NewAddressedCall(
		shortID,
		[]byte{1, 2, 3},
	)
	require.NoError(err)

	addressedPayloadBytes := addressedPayload.Bytes()
	parsedAddressedPayload, err := ParseAddressedCall(addressedPayloadBytes)
	require.NoError(err)
	require.Equal(addressedPayload, parsedAddressedPayload)
}

func TestParseAddressedCallJunk(t *testing.T) {
	_, err := ParseAddressedCall(junkBytes)
	require.Error(t, err)
}

func TestAddressedCallBytes(t *testing.T) {
	require := require.New(t)
	base64Payload := "AAAAAAABAAAAEAECAwAAAAAAAAAAAAAAAAAAAAADCgsM"
	addressedPayload, err := NewAddressedCall(
		[]byte{1, 2, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		[]byte{10, 11, 12},
	)
	require.NoError(err)
	require.Equal(base64Payload, base64.StdEncoding.EncodeToString(addressedPayload.Bytes()))
}
