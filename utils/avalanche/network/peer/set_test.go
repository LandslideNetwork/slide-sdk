// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package peer

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

func TestSetSample(t *testing.T) {
	require := require.New(t)

	set := NewSet()

	peer1 := &peer{
		id: ids.BuildTestNodeID([]byte{0x01}),
	}
	peer2 := &peer{
		id: ids.BuildTestNodeID([]byte{0x02}),
	}

	// Case: Empty
	peers := set.Sample(0, NoPrecondition)
	require.Empty(peers)

	peers = set.Sample(-1, NoPrecondition)
	require.Empty(peers)

	peers = set.Sample(1, NoPrecondition)
	require.Empty(peers)

	// Case: 1 peer
	set.Add(peer1)

	peers = set.Sample(0, NoPrecondition)
	require.Empty(peers)

	peers = set.Sample(1, NoPrecondition)
	require.Equal([]Peer{peer1}, peers)

	peers = set.Sample(2, NoPrecondition)
	require.Equal([]Peer{peer1}, peers)

	// Case: 2 peers
	set.Add(peer2)

	peers = set.Sample(1, NoPrecondition)
	require.Len(peers, 1)
}
