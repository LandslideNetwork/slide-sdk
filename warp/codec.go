package warp

import (
	"math"

	"github.com/consideritdone/landslidevm/codec"
	"github.com/consideritdone/landslidevm/codec/linearcodec"
	"github.com/consideritdone/landslidevm/utils"
)

const CodecVersion = 0

var Codec codec.Manager

func init() {
	Codec = codec.NewManager(math.MaxInt)
	lc := linearcodec.NewDefault()

	err := utils.Err(
		lc.RegisterType(&BitSetSignature{}),
		Codec.RegisterCodec(CodecVersion, lc),
	)
	if err != nil {
		panic(err)
	}
}
