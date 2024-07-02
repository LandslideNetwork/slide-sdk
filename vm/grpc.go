package vm

import (
	"context"
	"net"

	cmtnet "github.com/cometbft/cometbft/libs/net"
	rpctypes "github.com/cometbft/cometbft/rpc/jsonrpc/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/consideritdone/landslidevm/proto/tendermint/abci"

	cmgrpcproto "github.com/consideritdone/landslidevm/proto/tendermint/rpc/grpc"
)

// Config is an gRPC server configuration.
type Config struct {
	MaxOpenConnections int
}

// StartGRPCServer starts a new gRPC BroadcastAPIServer using the given
// net.Listener.
// NOTE: This function blocks - you may want to call it in a go-routine.
func StartGRPCServer(rpc *RPC, ln net.Listener) error {
	grpcServer := grpc.NewServer()
	cmgrpcproto.RegisterBroadcastAPIServer(grpcServer, &broadcastAPI{rpc: rpc})
	return grpcServer.Serve(ln)
}

// StartGRPCClient dials the gRPC server using address and returns a new
// BroadcastAPIClient.
func StartGRPCClient(address string) cmgrpcproto.BroadcastAPIClient {
	conn, err := grpc.Dial(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialerFunc),
	)
	if err != nil {
		panic(err)
	}
	return cmgrpcproto.NewBroadcastAPIClient(conn)
}

func dialerFunc(_ context.Context, addr string) (net.Conn, error) {
	return cmtnet.Connect(addr)
}

type broadcastAPI struct {
	rpc *RPC
}

func (bapi *broadcastAPI) Ping(context.Context, *cmgrpcproto.RequestPing) (*cmgrpcproto.ResponsePing, error) {
	return &cmgrpcproto.ResponsePing{}, nil
}

func (bapi *broadcastAPI) BroadcastTx(_ context.Context, req *cmgrpcproto.RequestBroadcastTx) (*cmgrpcproto.ResponseBroadcastTx, error) {
	// NOTE: there's no way to get client's remote address
	// see https://stackoverflow.com/questions/33684570/session-and-remote-ip-address-in-grpc-go
	res, err := bapi.rpc.BroadcastTxCommit(&rpctypes.Context{}, req.Tx)
	if err != nil {
		return nil, err
	}

	return &cmgrpcproto.ResponseBroadcastTx{
		CheckTx: &abci.ResponseCheckTx{
			Code: res.CheckTx.Code,
			Data: res.CheckTx.Data,
			Log:  res.CheckTx.Log,
		},
		TxResult: &abci.ExecTxResult{
			Code: res.TxResult.Code,
			Data: res.TxResult.Data,
			Log:  res.TxResult.Log,
		},
	}, nil
}
