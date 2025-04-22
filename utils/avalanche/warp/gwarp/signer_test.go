// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gwarp

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/landslidenetwork/slide-sdk/grpcutils"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/constants"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/warp/signertest"
	"github.com/landslidenetwork/slide-sdk/utils/crypto/bls"
	"github.com/landslidenetwork/slide-sdk/utils/ids"

	"github.com/landslidenetwork/slide-sdk/utils/avalanche/warp"

	pb "github.com/landslidenetwork/slide-sdk/proto/warp"
)

type testSigner struct {
	client    *Client
	server    warp.Signer
	sk        bls.Signer
	networkID uint32
	chainID   ids.ID
}

func setupSigner(t testing.TB) *testSigner {
	require := require.New(t)

	localSigner, err := bls.NewSigner()
	require.NoError(err)

	chainID := ids.GenerateTestID()

	s := &testSigner{
		server:    warp.NewSigner(localSigner, constants.UnitTestID, chainID),
		sk:        localSigner,
		networkID: constants.UnitTestID,
		chainID:   chainID,
	}

	listener, err := grpcutils.NewListener()
	require.NoError(err)
	serverCloser := grpcutils.ServerCloser{}

	server := grpcutils.NewServer()
	pb.RegisterSignerServer(server, NewServer(s.server))
	serverCloser.Add(server)

	go grpcutils.Serve(listener, server)

	conn, err := grpcutils.Dial(listener.Addr().String())
	require.NoError(err)

	s.client = NewClient(pb.NewSignerClient(conn))

	t.Cleanup(func() {
		serverCloser.Stop()
		_ = conn.Close()
		_ = listener.Close()
	})

	return s
}

func TestInterface(t *testing.T) {
	for name, test := range signertest.SignerTests {
		t.Run(name, func(t *testing.T) {
			s := setupSigner(t)
			test(t, s.client, s.sk, s.networkID, s.chainID)
		})
	}
}
