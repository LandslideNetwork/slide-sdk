package codec

import (
	"errors"

	"github.com/consideritdone/landslidevm/utils/wrappers"
)

var (
	ErrUnsupportedType           = errors.New("unsupported type")
	ErrMaxSliceLenExceeded       = errors.New("max slice length exceeded")
	ErrDoesNotImplementInterface = errors.New("does not implement interface")
	ErrUnexportedField           = errors.New("unexported field")
	ErrExtraSpace                = errors.New("trailing buffer space")
	ErrMarshalZeroLength         = errors.New("can't marshal zero length value")
	ErrUnmarshalZeroLength       = errors.New("can't unmarshal zero length value")
)

// Codec marshals and unmarshals
type Codec interface {
	MarshalInto(interface{}, *wrappers.Packer) error
	Unmarshal([]byte, interface{}) error

	// Returns the size, in bytes, of [value] when it's marshaled
	Size(value interface{}) (int, error)
}
