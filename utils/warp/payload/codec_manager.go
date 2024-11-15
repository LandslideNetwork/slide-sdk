package payload

import (
	"github.com/landslidenetwork/slide-sdk/utils/codec"
	"github.com/landslidenetwork/slide-sdk/utils/codec/linearcodec"
)

const MaxMessageSize = 24 * 1024

var Codec codec.Manager

func init() {
	lc := linearcodec.NewDefault()
	lc.RegisterType(&Hash{})
	err := lc.RegisterType(&AddressedCall{})
	if err != nil {
		panic(err)
	}
	Codec = codec.NewManager(MaxMessageSize, lc)
}
