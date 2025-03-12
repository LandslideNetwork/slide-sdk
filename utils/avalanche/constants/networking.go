package constants

import (
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/compression"
	"time"
)

const (
	// The network must be "tcp", "tcp4", "tcp6", "unix" or "unixpacket".
	NetworkType                   = "tcp"
	DefaultNetworkCompressionType = compression.TypeZstd
	DefaultPingPongTimeout        = 30 * time.Second
	DefaultPingFrequency          = 3 * DefaultPingPongTimeout / 4
)
