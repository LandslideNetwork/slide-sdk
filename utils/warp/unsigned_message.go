// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package warp

import (
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
