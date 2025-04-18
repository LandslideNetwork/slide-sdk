// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package timeout

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/landslidenetwork/slide-sdk/utils/avalanche/networking/benchlist"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

func TestManagerFire(t *testing.T) {
	benchlist := benchlist.NewNopBenchlist()
	manager, err := NewManager(
		benchlist,
	)
	require.NoError(t, err)
	t.Logf("%t", manager.IsBenched(ids.EmptyNodeID))

	wg := sync.WaitGroup{}
	wg.Add(1)

	wg.Wait()
}
