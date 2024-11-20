// (c) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package messages

import (
	"fmt"
)

// Payload provides a common interface for all payloads implemented by this
// package.
type Payload interface {
	// Bytes returns the binary representation of this payload.
	Bytes() []byte

	// initialize the payload with the provided binary representation.
	initialize(b []byte)
}

// Signable is an optional interface that payloads can implement to allow
// on-the-fly signing of incoming messages by the warp backend.
type Signable interface {
	VerifyMesssage(sourceAddress []byte) error
}

func Parse(bytes []byte) (Payload, error) {
	var payload Payload
	if err := Codec.Unmarshal(bytes, &payload); err != nil {
		return nil, err
	}
	payload.initialize(bytes)
	return payload, nil
}

func initialize(p Payload) error {
	bytes, err := Codec.Marshal(&p)
	if err != nil {
		return fmt.Errorf("couldn't marshal %T payload: %w", p, err)
	}
	p.initialize(bytes)
	return nil
}
