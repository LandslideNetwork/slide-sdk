// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package bls

import (
	"crypto/rand"
	"errors"
	"runtime"

	blst "github.com/supranational/blst/bindings/go"
)

const SecretKeyLen = blst.BLST_SCALAR_BYTES

var (
	errFailedSecretKeyDeserialize = errors.New("couldn't deserialize secret key")
)

type SecretKey = blst.SecretKey

// NewSecretKey generates a new secret key from the local source of
// cryptographically secure randomness.
func NewSecretKey() (*SecretKey, error) {
	var ikm [32]byte
	_, err := rand.Read(ikm[:])
	if err != nil {
		return nil, err
	}
	sk := blst.KeyGen(ikm[:])
	ikm = [32]byte{} // zero out the ikm
	return sk, nil
}

// SecretKeyToBytes returns the big-endian format of the secret key.
func SecretKeyToBytes(sk *SecretKey) []byte {
	return sk.Serialize()
}

// SecretKeyFromBytes parses the big-endian format of the secret key into a
// secret key.
func SecretKeyFromBytes(skBytes []byte) (*SecretKey, error) {
	sk := new(SecretKey).Deserialize(skBytes)
	if sk == nil {
		return nil, errFailedSecretKeyDeserialize
	}
	runtime.SetFinalizer(sk, func(sk *SecretKey) {
		sk.Zeroize()
	})
	return sk, nil
}

// PublicFromSecretKey returns the public key that corresponds to this secret
// key.
func PublicFromSecretKey(sk *SecretKey) *PublicKey {
	return new(PublicKey).From(sk)
}

// Sign [msg] to authorize this message from this [sk].
func Sign(sk *SecretKey, msg []byte) *Signature {
	return new(Signature).Sign(sk, msg, ciphersuiteSignature)
}
