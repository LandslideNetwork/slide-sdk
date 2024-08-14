package payload

import (
	"github.com/consideritdone/landslidevm/utils"
	"github.com/consideritdone/landslidevm/utils/units"

	"github.com/consideritdone/landslidevm/codec"
	"github.com/consideritdone/landslidevm/codec/linearcodec"
)

const (
	CodecVersion = 0

	MaxMessageSize = 24 * units.KiB
)

var Codec codec.Manager

func init() {
	Codec = codec.NewManager(MaxMessageSize)
	lc := linearcodec.NewDefault()

	err := utils.Err(
		lc.RegisterType(&Hash{}),
		lc.RegisterType(&AddressedCall{}),
		Codec.RegisterCodec(CodecVersion, lc),
	)
	if err != nil {
		panic(err)
	}
}
