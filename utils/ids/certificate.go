package ids

import "crypto"

type Certificate struct {
	Raw       []byte
	PublicKey crypto.PublicKey
}
