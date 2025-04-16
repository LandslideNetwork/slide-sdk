// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package message

import (
	"os"
	"testing"
	"time"

	"github.com/cometbft/cometbft/libs/log"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/landslidenetwork/slide-sdk/proto/p2p"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/compression"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

var (
	dummyNodeID             = ids.EmptyNodeID
	dummyOnFinishedHandling = func() {}
)

// Benchmarks marshal-ing "Handshake" message.
//
// e.g.,
//
//	$ go install -v golang.org/x/tools/cmd/benchcmp@latest
//	$ go install -v golang.org/x/perf/cmd/benchstat@latest
//
//	$ go test -run=NONE -bench=BenchmarkMarshalHandshake > /tmp/cpu.before.txt
//	$ USE_BUILDER=true go test -run=NONE -bench=BenchmarkMarshalHandshake > /tmp/cpu.after.txt
//	$ benchcmp /tmp/cpu.before.txt /tmp/cpu.after.txt
//	$ benchstat -alpha 0.03 -geomean /tmp/cpu.before.txt /tmp/cpu.after.txt
//
//	$ go test -run=NONE -bench=BenchmarkMarshalHandshake -benchmem > /tmp/mem.before.txt
//	$ USE_BUILDER=true go test -run=NONE -bench=BenchmarkMarshalHandshake -benchmem > /tmp/mem.after.txt
//	$ benchcmp /tmp/mem.before.txt /tmp/mem.after.txt
//	$ benchstat -alpha 0.03 -geomean /tmp/mem.before.txt /tmp/mem.after.txt
func BenchmarkMarshalHandshake(b *testing.B) {
	require := require.New(b)

	id := ids.GenerateTestID()
	msg := p2p.Message{
		Message: &p2p.Message_AppRequest{
			AppRequest: &p2p.AppRequest{
				ChainId:   []byte(id.Hex()),
				RequestId: uint32(777),
				Deadline:  0,
				AppBytes:  []byte{'y', 'e', 'e', 't'},
			},
		},
	}
	msgLen := proto.Size(&msg)

	useBuilder := os.Getenv("USE_BUILDER") != ""

	codec, err := newMsgBuilder(log.NewNopLogger(), prometheus.NewRegistry(), 10*time.Second)
	require.NoError(err)

	b.Logf("proto length %d-byte (use builder %v)", msgLen, useBuilder)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if useBuilder {
			_, err = codec.createOutbound(&msg, compression.TypeNone, false)
		} else {
			_, err = proto.Marshal(&msg)
		}
		require.NoError(err)
	}
}

// Benchmarks unmarshal-ing "Version" message.
//
// e.g.,
//
//	$ go install -v golang.org/x/tools/cmd/benchcmp@latest
//	$ go install -v golang.org/x/perf/cmd/benchstat@latest
//
//	$ go test -run=NONE -bench=BenchmarkUnmarshalHandshake > /tmp/cpu.before.txt
//	$ USE_BUILDER=true go test -run=NONE -bench=BenchmarkUnmarshalHandshake > /tmp/cpu.after.txt
//	$ benchcmp /tmp/cpu.before.txt /tmp/cpu.after.txt
//	$ benchstat -alpha 0.03 -geomean /tmp/cpu.before.txt /tmp/cpu.after.txt
//
//	$ go test -run=NONE -bench=BenchmarkUnmarshalHandshake -benchmem > /tmp/mem.before.txt
//	$ USE_BUILDER=true go test -run=NONE -bench=BenchmarkUnmarshalHandshake -benchmem > /tmp/mem.after.txt
//	$ benchcmp /tmp/mem.before.txt /tmp/mem.after.txt
//	$ benchstat -alpha 0.03 -geomean /tmp/mem.before.txt /tmp/mem.after.txt
func BenchmarkUnmarshalHandshake(b *testing.B) {
	require := require.New(b)

	b.StopTimer()

	id := ids.GenerateTestID()
	msg := p2p.Message{
		Message: &p2p.Message_AppRequest{
			AppRequest: &p2p.AppRequest{
				ChainId:   []byte(id.Hex()),
				RequestId: uint32(777),
				Deadline:  0,
				AppBytes:  []byte{'y', 'e', 'e', 't'},
			},
		},
	}

	rawMsg, err := proto.Marshal(&msg)
	require.NoError(err)

	useBuilder := os.Getenv("USE_BUILDER") != ""
	codec, err := newMsgBuilder(log.NewNopLogger(), prometheus.NewRegistry(), 10*time.Second)
	require.NoError(err)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		if useBuilder {
			_, err = codec.parseInbound(rawMsg, dummyNodeID, dummyOnFinishedHandling)
			require.NoError(err)
		} else {
			var msg p2p.Message
			require.NoError(proto.Unmarshal(rawMsg, &msg))
		}
	}
}
