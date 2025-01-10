package bls

import (
	"errors"

	blst "github.com/supranational/blst/bindings/go"
)

var (
	ciphersuiteSignature          = []byte("BLS_SIG_BLS12381G2_XMD:SHA-256_SSWU_RO_POP_")
	ErrNoPublicKeys               = errors.New("no public keys")
	ErrFailedPublicKeyDecompress  = errors.New("couldn't decompress public key")
	errInvalidPublicKey           = errors.New("invalid public key")
	errFailedPublicKeyAggregation = errors.New("couldn't aggregate public keys")
)

type (
	PublicKey          = blst.P1Affine
	AggregatePublicKey = blst.P1Aggregate
)

// Verify the [sig] of [msg] against the [pk].
// The [sig] and [pk] may have been an aggregation of other signatures and keys.
// Invariant: [pk] and [sig] have both been validated.
func Verify(pk *PublicKey, sig *Signature, msg []byte) bool {
	return sig.Verify(false, pk, false, msg, ciphersuiteSignature)
}

// PublicKeyFromValidUncompressedBytes parses the uncompressed big-endian format
// of the public key into a public key. It is assumed that the provided bytes
// are valid.
func PublicKeyFromValidUncompressedBytes(pkBytes []byte) *PublicKey {
	return new(PublicKey).Deserialize(pkBytes)
}

// PublicKeyToUncompressedBytes returns the uncompressed big-endian format of
// the public key.
func PublicKeyToUncompressedBytes(key *PublicKey) []byte {
	return key.Serialize()
}

// PublicKeyToCompressedBytes returns the compressed big-endian format of the
// public key.
func PublicKeyToCompressedBytes(pk *PublicKey) []byte {
	return pk.Compress()
}

// PublicKeyFromCompressedBytes parses the compressed big-endian format of the
// public key into a public key.
func PublicKeyFromCompressedBytes(pkBytes []byte) (*PublicKey, error) {
	pk := new(PublicKey).Uncompress(pkBytes)
	if pk == nil {
		return nil, ErrFailedPublicKeyDecompress
	}
	if !pk.KeyValidate() {
		return nil, errInvalidPublicKey
	}
	return pk, nil
}

// AggregatePublicKeys aggregates a non-zero number of public keys into a single
// aggregated public key.
// Invariant: all [pks] have been validated.
func AggregatePublicKeys(pks []*PublicKey) (*PublicKey, error) {
	if len(pks) == 0 {
		return nil, ErrNoPublicKeys
	}

	var agg AggregatePublicKey
	if !agg.Aggregate(pks, false) {
		return nil, errFailedPublicKeyAggregation
	}
	return agg.ToAffine(), nil
}
