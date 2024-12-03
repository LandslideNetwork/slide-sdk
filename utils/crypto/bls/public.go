package bls

import blst "github.com/supranational/blst/bindings/go"

var ciphersuiteSignature = []byte("BLS_SIG_BLS12381G2_XMD:SHA-256_SSWU_RO_POP_")

type PublicKey = blst.P1Affine

// Verify the [sig] of [msg] against the [pk].
// The [sig] and [pk] may have been an aggregation of other signatures and keys.
// Invariant: [pk] and [sig] have both been validated.
func Verify(pk *PublicKey, sig *Signature, msg []byte) bool {
	return sig.Verify(false, pk, false, msg, ciphersuiteSignature)
}
