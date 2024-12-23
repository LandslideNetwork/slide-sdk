// (c) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package messages

import (
	"fmt"

	"github.com/landslidenetwork/slide-sdk/utils/crypto/bls"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

var (
	_ Request = MessageSignatureRequest{}
	_ Request = BlockSignatureRequest{}
)

// MessageSignatureRequest is used to request a warp message's signature.
type MessageSignatureRequest struct {
	MessageID ids.ID `serialize:"true"`
}

func (s MessageSignatureRequest) String() string {
	return fmt.Sprintf("MessageSignatureRequest(MessageID=%s)", s.MessageID.String())
}

// BlockSignatureRequest is used to request a warp message's signature.
type BlockSignatureRequest struct {
	BlockID ids.ID `serialize:"true"`
}

func (s BlockSignatureRequest) String() string {
	return fmt.Sprintf("BlockSignatureRequest(BlockID=%s)", s.BlockID.String())
}

// SignatureResponse is the response to a BlockSignatureRequest or MessageSignatureRequest.
// The response contains a BLS signature of the requested message, signed by the responding node's BLS private key.
type SignatureResponse struct {
	Signature [bls.SignatureLen]byte `serialize:"true"`
}
