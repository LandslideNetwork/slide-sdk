package vm

import (
	"context"
	"net/http"
	"testing"
	"time"

	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cometbft/cometbft/rpc/jsonrpc/client"
	"github.com/stretchr/testify/require"

	"github.com/consideritdone/landslidevm/jsonrpc"
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
		//panic(err)
		//require.NoError(t, err)
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
