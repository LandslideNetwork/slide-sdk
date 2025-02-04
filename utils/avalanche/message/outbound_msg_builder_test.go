// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package message

import (
	"github.com/cometbft/cometbft/libs/log"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/landslidenetwork/slide-sdk/utils/avalanche/compression"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

func Test_newOutboundBuilder(t *testing.T) {
	t.Parallel()

	mb, err := newMsgBuilder(
		log.NewNopLogger(),
		prometheus.NewRegistry(),
		10*time.Second,
	)
	require.NoError(t, err)

	for _, compressionType := range []compression.Type{
		compression.TypeNone,
		compression.TypeZstd,
	} {
		t.Run(compressionType.String(), func(t *testing.T) {
			builder := newOutboundBuilder(compressionType, mb)

			outMsg, err := builder.AppRequest(
				ids.GenerateTestID(),
				12345,
				time.Hour,
				[]byte{1, 2, 3, 4, 5},
			)
			require.NoError(t, err)
			t.Logf("outbound message with compression type %s built message with size %d", compressionType, len(outMsg.Bytes()))
		})
	}
}
