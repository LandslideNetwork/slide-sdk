package warp

import (
	"math"

	"github.com/landslidenetwork/slide-sdk/utils/codec"
	"github.com/landslidenetwork/slide-sdk/utils/codec/linearcodec"
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
