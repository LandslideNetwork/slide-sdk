package message

import (
	"github.com/landslidenetwork/slide-sdk/utils/codec"
	"github.com/landslidenetwork/slide-sdk/utils/codec/linearcodec"
	"github.com/landslidenetwork/slide-sdk/utils/units"
)

const maxMessageSize = 2*units.MiB - 64*units.KiB

var Codec codec.Manager

func init() {
	lc := linearcodec.NewDefault()
	// Gossip types
	err := lc.RegisterType(TxsGossip{})
	if err != nil {
		panic(err)
	}
	err = lc.RegisterType(MessageSignatureRequest{})
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
	Codec = codec.NewManager(maxMessageSize, lc)
}
