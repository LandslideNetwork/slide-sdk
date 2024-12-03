// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package warp

import (
	blst "github.com/supranational/blst/bindings/go"
)

type BitSetSignature struct {
	// Signers is a big-endian byte slice encoding which validators signed this
	// message.
	Signers   []byte                            `serialize:"true"`
	Signature [blst.BLST_P2_COMPRESS_BYTES]byte `serialize:"true"`
}
