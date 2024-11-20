// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"errors"
	"fmt"

	"github.com/landslidenetwork/slide-sdk/utils/wrappers"
)

const (
	// initial capacity of byte slice that values are marshaled into.
	// Larger value --> need less memory allocations but possibly have allocated but unused memory
	// Smaller value --> need more memory allocations but more efficient use of allocated memory
	initialSliceCap = 128
	//default version of codec
	defaultVersion = 0
)

var (
	ErrMarshalNil      = errors.New("can't marshal nil pointer or interface")
	ErrUnmarshalNil    = errors.New("can't unmarshal nil")
	ErrUnmarshalTooBig = errors.New("byte array exceeds maximum length")
	ErrExtraSpace      = errors.New("trailing buffer space")
)

var _ Manager = (*manager)(nil)

// Manager describes the functionality for managing codec versions.
type Manager interface {
	// Size returns the size, in bytes, of [value] when it's marshaled
	// using the codec with the given version.
	// RegisterCodec must have been called with that version.
	// If [value] is nil, returns [ErrMarshalNil]
	Size(value interface{}) (int, error)

	// Marshal the given value using the codec with the given version.
	// RegisterCodec must have been called with that version.
	Marshal(source interface{}) (destination []byte, err error)

	// Unmarshal the given bytes into the given destination. [destination] must
	// be a pointer or an interface. Returns the version of the codec that
	// produces the given bytes.
	Unmarshal(source []byte, destination interface{}) error
}

// NewManager returns a new codec manager.
func NewManager(maxSize int, codec Codec) Manager {
	return &manager{
		maxSize: maxSize,
		codec:   codec,
	}
}

type manager struct {
	maxSize int
	codec   Codec
}

func (m *manager) Size(value interface{}) (int, error) {
	if value == nil {
		return 0, ErrMarshalNil // can't marshal nil
	}

	res, err := m.codec.Size(value)

	// Add [wrappers.ShortLen] for the codec version
	return wrappers.ShortLen + res, err
}

// To marshal an interface, [value] must be a pointer to the interface.
func (m *manager) Marshal(value interface{}) ([]byte, error) {
	if value == nil {
		return nil, ErrMarshalNil // can't marshal nil
	}

	p := wrappers.Packer{
		MaxSize: m.maxSize,
		Bytes:   make([]byte, 0, initialSliceCap),
	}
	p.PackShort(defaultVersion)
	if p.Errored() {
		return nil, ErrCantPackVersion // Should never happen
	}
	return p.Bytes, m.codec.MarshalInto(value, &p)
}

// Unmarshal unmarshals [bytes] into [dest], where [dest] must be a pointer or
// interface.
func (m *manager) Unmarshal(bytes []byte, dest interface{}) error {
	if dest == nil {
		return ErrUnmarshalNil
	}

	if byteLen := len(bytes); byteLen > m.maxSize {
		return fmt.Errorf("%w: %d > %d", ErrUnmarshalTooBig, byteLen, m.maxSize)
	}

	p := wrappers.Packer{
		Bytes: bytes,
	}
	p.UnpackShort()
	if p.Errored() { // Make sure the codec version is correct
		return ErrCantUnpackVersion
	}
	if err := m.codec.UnmarshalFrom(&p, dest); err != nil {
		return err
	}
	if p.Offset != len(bytes) {
		return fmt.Errorf("%w: read %d provided %d",
			ErrExtraSpace,
			p.Offset,
			len(bytes),
		)
	}

	return nil
}
