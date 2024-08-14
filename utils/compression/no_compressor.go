package compression

var _ Compressor = (*noCompressor)(nil)

type noCompressor struct{}

func (*noCompressor) Compress(msg []byte) ([]byte, error) {
	return msg, nil
}

func (*noCompressor) Decompress(msg []byte) ([]byte, error) {
	return msg, nil
}

func NewNoCompressor() Compressor {
	return &noCompressor{}
}
