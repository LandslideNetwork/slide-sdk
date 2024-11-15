// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package warp

import (
	"fmt"

	"github.com/landslidenetwork/slide-sdk/utils/hashing"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

// UnsignedMessage defines the standard format for an unsigned Warp message.
type UnsignedMessage struct {
	NetworkID     uint32 `serialize:"true"`
	SourceChainID ids.ID `serialize:"true"`
	Payload       []byte `serialize:"true"`

	bytes []byte
	id    ids.ID
}

// NewUnsignedMessage creates a new *UnsignedMessage and initializes it.
func NewUnsignedMessage(
	networkID uint32,
	sourceChainID ids.ID,
	payload []byte,
) (*UnsignedMessage, error) {
	msg := &UnsignedMessage{
		NetworkID:     networkID,
		SourceChainID: sourceChainID,
		Payload:       payload,
	}
	err := msg.Initialize()
	return msg, err
}

// ParseUnsignedMessage converts a slice of bytes into an initialized
// *UnsignedMessage.
func ParseUnsignedMessage(b []byte) (*UnsignedMessage, error) {
	msg := &UnsignedMessage{
		bytes: b,
		id:    hashing.ComputeHash256Array(b),
	}
	err := Codec.Unmarshal(b, msg)
	return msg, err
}

// Initialize recalculates the result of Bytes().
func (m *UnsignedMessage) Initialize() error {
	bytes, err := Codec.Marshal(m)
	if err != nil {
		return fmt.Errorf("couldn't marshal warp unsigned message: %w", err)
	}
	m.bytes = bytes
	m.id = hashing.ComputeHash256Array(m.bytes)
	return nil
}

// Bytes returns the binary representation of this message. It assumes that the
// message is initialized from either New, Parse, or an explicit call to
// Initialize.
func (m *UnsignedMessage) Bytes() []byte {
	return m.bytes
}

// ID returns an identifier for this message. It assumes that the
// message is initialized from either New, Parse, or an explicit call to
// Initialize.
func (m *UnsignedMessage) ID() ids.ID {
	return m.id
}