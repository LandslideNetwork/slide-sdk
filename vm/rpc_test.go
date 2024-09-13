package vm

import (
	"context"
	"github.com/cometbft/cometbft/libs/bytes"
	"github.com/consideritdone/landslidevm/utils/ids"
	warpConnection "github.com/consideritdone/landslidevm/warp/connection"
	"maps"
	"net/http"
	"testing"
	"time"

	coretypes "github.com/cometbft/cometbft/rpc/core/types"

	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cometbft/cometbft/rpc/jsonrpc/client"
	rpctypes "github.com/cometbft/cometbft/rpc/jsonrpc/types"
	"github.com/stretchr/testify/require"

	"github.com/consideritdone/landslidevm/jsonrpc"
)

func setupRPC(t *testing.T) (*http.Server, *LandslideVM, *client.Client) {
	vm := newFreshKvApp(t)
	vmLnd := vm.(*LandslideVM)
	mux := http.NewServeMux()
	rpcRoutes := NewRPC(vmLnd).Routes()
	warpRoutes := warpConnection.NewAPI(vmLnd.appOpts.NetworkID, ids.ID(vmLnd.appOpts.SubnetID),
		ids.ID(vmLnd.appOpts.ChainID), vmLnd.warpBackend).Routes()
	maps.Copy(rpcRoutes, warpRoutes)
	jsonrpc.RegisterRPCFuncs(mux, rpcRoutes, vmLnd.logger)

	address := "127.0.0.1:44444"
	server := &http.Server{Addr: address, Handler: mux}
	go func() {
		server.ListenAndServe()
		// panic(err)
		// require.NoError(t, err)
	}()

	// wait for servers to start
	time.Sleep(time.Second * 2)

	client, err := client.New("tcp://" + address)
	require.NoError(t, err)

	return server, vmLnd, client
}

func TestHealth(t *testing.T) {
	server, _, client := setupRPC(t)
	defer server.Close()

	result := new(ctypes.ResultHealth)
	_, err := client.Call(context.Background(), "health", map[string]interface{}{}, result)
	require.NoError(t, err)

	t.Logf("Health result %+v", result)
}

func TestStatus(t *testing.T) {
	server, _, client := setupRPC(t)
	defer server.Close()

	result := new(ctypes.ResultStatus)
	_, err := client.Call(context.Background(), "status", map[string]interface{}{}, result)
	require.NoError(t, err)

	t.Logf("Status result %+v", result)
}

// TestRPC is a test RPC server for the LandslideVM.
type TestRPC struct {
	vm *LandslideVM
}

// NewTestRPC creates a new TestRPC.
func NewTestRPC(vm *LandslideVM) *TestRPC {
	return &TestRPC{vm}
}

// Routes returns the available RPC routes.
func (rpc *TestRPC) Routes() map[string]*jsonrpc.RPCFunc {
	return map[string]*jsonrpc.RPCFunc{
		"test_panic": jsonrpc.NewRPCFunc(rpc.TestPanic, ""),
	}
}

// NumUnconfirmedTxs gets number of unconfirmed transactions.
func (rpc *TestRPC) TestPanic(_ *rpctypes.Context) (*ctypes.ResultUnconfirmedTxs, error) {
	panic("test panic")
}

// setupTestRPC sets up a test server and client for the LandslideVM.
func setupTestRPC(t *testing.T) (*http.Server, *LandslideVM, *client.Client) {
	vm := newFreshKvApp(t)
	vmLnd := vm.(*LandslideVM)
	mux := http.NewServeMux()
	rpcRoutes := NewRPC(vmLnd).Routes()
	warpRoutes := warpConnection.NewAPI(vmLnd.appOpts.NetworkID, ids.ID(vmLnd.appOpts.SubnetID),
		ids.ID(vmLnd.appOpts.ChainID), vmLnd.warpBackend).Routes()
	maps.Copy(rpcRoutes, warpRoutes)
	jsonrpc.RegisterRPCFuncs(mux, rpcRoutes, vmLnd.logger)

	address := "127.0.0.1:44444"
	server := &http.Server{Addr: address, Handler: mux, ReadHeaderTimeout: time.Second * 5, WriteTimeout: time.Second * 5}
	go func() {
		_ = server.ListenAndServe()
	}()

	// wait for servers to start
	time.Sleep(time.Second * 2)

	rpcClient, err := client.New("tcp://" + address)
	require.NoError(t, err)

	return server, vmLnd, rpcClient
}

//
//// TestPanic tests that the server recovers from a panic.
//func TestPanic(t *testing.T) {
//	server, _, rpcClient := setupTestRPC(t)
//	defer server.Close()
//
//	result := new(ctypes.ResultStatus)
//	_, err := rpcClient.Call(context.Background(), "test_panic", map[string]interface{}{}, result)
//	require.Error(t, err)
//
//	t.Logf("Panic result %+v", err)
//}

// TestPanic tests that the server recovers from a panic.
func TestGetBlockSignature(t *testing.T) {
	server, vm, rpcClient := setupTestRPC(t)
	defer server.Close()

	result := new(coretypes.ResultBlock)
	_, err := rpcClient.Call(context.Background(), "block", map[string]interface{}{"height": vm.state.LastBlockHeight}, result)
	require.NoError(t, err)

	resultSig := new(bytes.HexBytes)
	blkID, err := ids.ToID(result.Block.Hash().Bytes())
	require.NoError(t, err)
	_, err = rpcClient.Call(context.Background(), "get_block_signature", map[string]interface{}{"blockID": blkID}, resultSig)
	require.NoError(t, err)

	t.Logf("GetBlockSignature result %+v", result)
}
