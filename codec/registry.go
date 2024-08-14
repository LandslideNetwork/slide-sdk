package codec

import "errors"

var ErrDuplicateType = errors.New("duplicate type registration")

// Registry registers new types that can be marshaled into
type Registry interface {
	RegisterType(interface{}) error
}
