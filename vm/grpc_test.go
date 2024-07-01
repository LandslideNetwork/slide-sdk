package vm

import (
	"context"
	"net"
	"testing"
	"time"

	cmgrpcproto "github.com/cometbft/cometbft/proto/tendermint/rpc/grpc"
	"github.com/stretchr/testify/assert"
)

func TestPingGRPC(t *testing.T) {
	vm := newFreshKvApp(t)
	vmLnd := vm.(*LandslideVM)
	grpcAddr := "127.0.0.1:9090"
	listenerGRPC, err := net.Listen("tcp", grpcAddr)
	assert.NoError(t, err)

	go func() {
		if err := StartGRPCServer(NewRPC(vmLnd), listenerGRPC); err != nil {
			assert.NoError(t, err)
		}
		_ = listenerGRPC.Close()
	}()

	// wait for server to start
	time.Sleep(time.Second * 2)

	client := StartGRPCClient(grpcAddr)

	result, err := client.Ping(context.Background(), &cmgrpcproto.RequestPing{})
	assert.NoError(t, err)

	t.Logf("Ping result %+v", result)
}
