// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package peer

import (
	"time"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/message"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/networking/router"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/timer/mockable"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/uptime"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/validators"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	"github.com/landslidenetwork/slide-sdk/utils/set"
	"github.com/landslidenetwork/slide-sdk/utils/version"
)

type Config struct {
	// Size, in bytes, of the buffer this peer reads messages into
	ReadBufferSize int
	// Size, in bytes, of the buffer this peer writes messages into
	WriteBufferSize int
	Clock           mockable.Clock
	MessageCreator  message.Creator

	Log                  log.Logger
	Network              Network
	Router               router.InboundHandler
	VersionCompatibility version.Compatibility
	MyNodeID             ids.NodeID
	// MySubnets does not include the primary network ID
	MySubnets          set.Set[ids.ID]
	Validators         validators.Manager
	NetworkID          uint32
	PingFrequency      time.Duration
	PongTimeout        time.Duration
	MaxClockDifference time.Duration

	SupportedACPs []uint32
	ObjectedACPs  []uint32

	// Unix time of the last message sent and received respectively
	// Must only be accessed atomically
	LastSent, LastReceived int64

	// Calculates uptime of peers
	UptimeCalculator uptime.Calculator

	// Signs my IP so I can send my signed IP address in the Handshake message
	IPSigner *IPSigner
}
