// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package network

import (
	"net/netip"

	"github.com/landslidenetwork/slide-sdk/utils/ips"
	"github.com/landslidenetwork/slide-sdk/utils/staking"
)

var (
	ip      *ips.ClaimedIPPort
	otherIP *ips.ClaimedIPPort

	defaultLoopbackAddrPort = netip.AddrPortFrom(
		netip.AddrFrom4([4]byte{127, 0, 0, 1}),
		9651,
	)
)

func init() {
	{
		cert, err := staking.NewTLSCert()
		if err != nil {
			panic(err)
		}
		stakingCert, err := staking.ParseCertificate(cert.Leaf.Raw)
		if err != nil {
			panic(err)
		}
		ip = ips.NewClaimedIPPort(
			stakingCert,
			defaultLoopbackAddrPort,
			1,   // timestamp
			nil, // signature
		)
	}

	{
		cert, err := staking.NewTLSCert()
		if err != nil {
			panic(err)
		}
		stakingCert, err := staking.ParseCertificate(cert.Leaf.Raw)
		if err != nil {
			panic(err)
		}
		otherIP = ips.NewClaimedIPPort(
			stakingCert,
			defaultLoopbackAddrPort,
			1,   // timestamp
			nil, // signature
		)
	}
}
