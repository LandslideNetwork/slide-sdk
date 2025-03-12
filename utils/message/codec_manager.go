package message

import (
	"math"

	"github.com/landslidenetwork/slide-sdk/utils/codec"
	"github.com/landslidenetwork/slide-sdk/utils/codec/linearcodec"
)

var Codec codec.Manager

func init() {
	lc := linearcodec.NewDefault()
	err := lc.RegisterType(MessageSignatureRequest{})
	if err != nil {
		panic(err)
	}
	err = lc.RegisterType(BlockSignatureRequest{})
	if err != nil {
		panic(err)
	}
	err = lc.RegisterType(SignatureResponse{})
	if err != nil {
		panic(err)
	}
	Codec = codec.NewManager(math.MaxInt, lc)
}
