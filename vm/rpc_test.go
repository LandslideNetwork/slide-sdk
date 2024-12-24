package vm

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/cometbft/cometbft/libs/rand"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	warputils "github.com/landslidenetwork/slide-sdk/utils/warp"

	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cometbft/cometbft/rpc/jsonrpc/client"
	rpctypes "github.com/cometbft/cometbft/rpc/jsonrpc/types"
	"github.com/stretchr/testify/require"

	"github.com/landslidenetwork/slide-sdk/jsonrpc"
)

func setupRPC(t *testing.T) (*http.Server, *LandslideVM, *client.Client) {
	vm := newFreshKvApp(t)
	vmLnd := vm.(*LandslideVM)
	mux := http.NewServeMux()
	jsonrpc.RegisterRPCFuncs(mux, NewRPC(vmLnd).Routes(), vmLnd.logger)

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

func TestWarpGetMessage(t *testing.T) {
	server, vm, rpcClient := setupRPC(t)
	defer server.Close()

	chainID, err := ids.ToID(vm.appOpts.ChainID)
	require.NoError(t, err)
	testUnsignedMessage, err := warputils.NewUnsignedMessage(vm.appOpts.NetworkID, chainID, []byte(rand.Str(30)))
	require.NoError(t, err)
	vm.warpBackend.AddMessage(testUnsignedMessage)
	result := new(ResultGetMessage)
	_, err = rpcClient.Call(context.Background(), "warp_get_message", map[string]interface{}{"messageID": testUnsignedMessage.ID().String()}, result)
	require.NoError(t, err)
	t.Log(result.Message)
	t.Log(testUnsignedMessage.Bytes())
	require.Equal(t, result.Message, testUnsignedMessage.Bytes())
}

func TestWarpGetMessageSignature(t *testing.T) {
	server, vm, rpcClient := setupRPC(t)
	defer server.Close()

	chainID, err := ids.ToID(vm.appOpts.ChainID)
	require.NoError(t, err)
	testUnsignedMessage, err := warputils.NewUnsignedMessage(vm.appOpts.NetworkID, chainID, []byte(rand.Str(30)))
	require.NoError(t, err)
	vm.warpBackend.AddMessage(testUnsignedMessage)
	result := new(ResultGetMessageSignature)
	_, err = rpcClient.Call(context.Background(), "warp_get_message_signature", map[string]interface{}{"messageID": testUnsignedMessage.ID().String()}, result)
	require.NoError(t, err)
	expectedSig, err := vm.warpSigner.Sign(testUnsignedMessage)
	require.NoError(t, err)
	require.NotNil(t, expectedSig)

	t.Log(result.Signature)
	t.Log(expectedSig)

	require.NoError(t, err)
	require.Equal(t, expectedSig, result.Signature)
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
	jsonrpc.RegisterRPCFuncs(mux, NewTestRPC(vmLnd).Routes(), vmLnd.logger)

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

// TestPanic tests that the server recovers from a panic.
func TestPanic(t *testing.T) {
	server, _, rpcClient := setupTestRPC(t)
	defer server.Close()

	result := new(ctypes.ResultStatus)
	_, err := rpcClient.Call(context.Background(), "test_panic", map[string]interface{}{}, result)
	require.Error(t, err)

	t.Logf("Panic result %+v", err)
}
