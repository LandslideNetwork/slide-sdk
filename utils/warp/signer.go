// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package warp

import (
	"errors"
	"github.com/landslidenetwork/slide-sdk/utils/crypto/bls"
	blst "github.com/supranational/blst/bindings/go"

	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

var (
	_ Signer = (*signer)(nil)

	ErrWrongSourceChainID = errors.New("wrong SourceChainID")
	ErrWrongNetworkID     = errors.New("wrong networkID")
	ciphersuiteSignature  = []byte("BLS_SIG_BLS12381G2_XMD:SHA-256_SSWU_RO_POP_")
)

type Signer interface {
	// Assumes the unsigned message is correctly initialized.
	Sign(msg *UnsignedMessage) ([]byte, error)
}

func NewSigner(sk *blst.SecretKey, networkID uint32, chainID ids.ID) Signer {
	return &signer{
		sk:        sk,
		networkID: networkID,
		chainID:   chainID,
	}
}

type signer struct {
	sk        *blst.SecretKey
	networkID uint32
	chainID   ids.ID
}

func (s *signer) Sign(msg *UnsignedMessage) ([]byte, error) {
	if msg.SourceChainID != s.chainID {
		return nil, ErrWrongSourceChainID
	}
	if msg.NetworkID != s.networkID {
		return nil, ErrWrongNetworkID
	}

	msgBytes := msg.Bytes()
	signature := bls.Sign(s.sk, msgBytes)
	return signature.Compress(), nil
}
