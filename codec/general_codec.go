package codec

// GeneralCodec marshals and unmarshals structs including interfaces
type GeneralCodec interface {
	Codec
	Registry
}
