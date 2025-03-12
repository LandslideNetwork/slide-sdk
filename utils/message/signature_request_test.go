// (c) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package message

import (
	"encoding/base64"
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/landslidenetwork/slide-sdk/utils/crypto/bls"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	"github.com/stretchr/testify/require"
)

// TestMarshalMessageSignatureRequest asserts that the structure or serialization logic hasn't changed, primarily to
// ensure compatibility with the network.
func TestMarshalMessageSignatureRequest(t *testing.T) {
	signatureRequest := MessageSignatureRequest{
		MessageID: ids.ID{68, 79, 70, 65, 72, 73, 64, 107},
	}

	base64MessageSignatureRequest := "AABET0ZBSElAawAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=="
	signatureRequestBytes, err := Codec.Marshal(signatureRequest)
	require.NoError(t, err)
	require.Equal(t, base64MessageSignatureRequest, base64.StdEncoding.EncodeToString(signatureRequestBytes))

	var s MessageSignatureRequest
	err = Codec.Unmarshal(signatureRequestBytes, &s)
	require.NoError(t, err)
	require.Equal(t, signatureRequest.MessageID, s.MessageID)
}

// TestMarshalBlockSignatureRequest asserts that the structure or serialization logic hasn't changed, primarily to
// ensure compatibility with the network.
func TestMarshalBlockSignatureRequest(t *testing.T) {
	signatureRequest := BlockSignatureRequest{
		BlockID: ids.ID{68, 79, 70, 65, 72, 73, 64, 107},
	}

	base64BlockSignatureRequest := "AABET0ZBSElAawAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=="
	signatureRequestBytes, err := Codec.Marshal(signatureRequest)
	require.NoError(t, err)
	require.Equal(t, base64BlockSignatureRequest, base64.StdEncoding.EncodeToString(signatureRequestBytes))

	var s BlockSignatureRequest
	err = Codec.Unmarshal(signatureRequestBytes, &s)
	require.NoError(t, err)
	require.Equal(t, signatureRequest.BlockID, s.BlockID)
}

// TestMarshalSignatureResponse asserts that the structure or serialization logic hasn't changed, primarily to
// ensure compatibility with the network.
func TestMarshalSignatureResponse(t *testing.T) {
	var signature [bls.SignatureLen]byte
	sig, err := hex.DecodeString("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	require.NoError(t, err, "failed to decode string to hex")

	copy(signature[:], sig)
	signatureResponse := SignatureResponse{
		Signature: signature,
	}

	base64SignatureResponse := "AAABI0VniavN7wEjRWeJq83vASNFZ4mrze8BI0VniavN7wEjRWeJq83vASNFZ4mrze8BI0VniavN7wEjRWeJq83vASNFZ4mrze8BI0VniavN7wEjRWeJq83vASNFZ4mrze8="
	signatureResponseBytes, err := Codec.Marshal(signatureResponse)
	require.NoError(t, err)
	require.Equal(t, base64SignatureResponse, base64.StdEncoding.EncodeToString(signatureResponseBytes))

	var s SignatureResponse
	err = Codec.Unmarshal(signatureResponseBytes, &s)
	require.NoError(t, err)
	require.Equal(t, signatureResponse.Signature, s.Signature)
}

// TestMarshalMessageSignatureRequest asserts that the structure or serialization logic hasn't changed, primarily to
// ensure compatibility with the network.
func TestMarshalMessageSignatureRequestInterface(t *testing.T) {
	var signatureRequest Request = MessageSignatureRequest{
		MessageID: ids.ID{68, 79, 70, 65, 72, 73, 64, 107},
	}

	base64MessageSignatureRequest := "AABET0ZBSElAawAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=="
	valType := reflect.TypeOf(signatureRequest)
	t.Log(valType.String())
	signatureRequestBytes, err := Codec.Marshal(&signatureRequest)
	require.NoError(t, err)
	require.Equal(t, base64MessageSignatureRequest, base64.StdEncoding.EncodeToString(signatureRequestBytes))

	var s Request
	err = Codec.Unmarshal(signatureRequestBytes, &s)
	require.NoError(t, err)
}
