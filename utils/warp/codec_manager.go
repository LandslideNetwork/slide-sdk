package warp

import (
	"github.com/landslidenetwork/slide-sdk/utils/codec"
	"github.com/landslidenetwork/slide-sdk/utils/codec/linearcodec"
	"math"
)

var Codec codec.Manager

func init() {
	lc := linearcodec.NewDefault()
	err := lc.RegisterType(&BitSetSignature{})
	if err != nil {
		panic(err)
	}
	Codec = codec.NewManager(math.MaxInt, lc)
}
