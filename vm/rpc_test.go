package vm

import (
	"context"
	"fmt"
	"github.com/cometbft/cometbft/abci/example/kvstore"
	"github.com/cometbft/cometbft/libs/rand"
	"github.com/cometbft/cometbft/types"
	"github.com/cometbft/cometbft/version"
	vmpb "github.com/consideritdone/landslidevm/proto/vm"
	"net/http"
	"testing"
	"time"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cometbft/cometbft/rpc/jsonrpc/client"
	"github.com/stretchr/testify/require"

	"github.com/consideritdone/landslidevm/jsonrpc"
)

type txRuntimeEnv struct {
	key, value, hash []byte
	initMemPoolSize  int
}

func buildAccept(t *testing.T, ctx context.Context, vm *LandslideVM) {
	end := false
	for !end {
		select {
		case <-ctx.Done():
			end = true
		default:
			if vm.mempool.Size() > 0 {
				block, err := vm.BuildBlock(ctx, &vmpb.BuildBlockRequest{})
				t.Logf("new block: %#v", block)
				require.NoError(t, err)
				_, err = vm.BlockAccept(ctx, &vmpb.BlockAcceptRequest{
					Id: block.Id,
				})
				require.NoError(t, err)
			} else {
				time.Sleep(500 * time.Millisecond)
			}
		}
	}
}

func setupRPC(t *testing.T, blockBuilder func(*testing.T, context.Context, *LandslideVM)) (*http.Server, *LandslideVM, *client.Client, context.CancelFunc) {
	vm := newFreshKvApp(t)
	vmLnd := vm.(*LandslideVM)
	mux := http.NewServeMux()
	jsonrpc.RegisterRPCFuncs(mux, NewRPC(vmLnd).Routes(), vmLnd.logger)

	address := "127.0.0.1:44444"
	server := &http.Server{Addr: address, Handler: mux}
	ctx, cancel := context.WithCancel(context.Background())
	go blockBuilder(t, ctx, vmLnd)
	go func() {
		err := server.ListenAndServe()
		t.Log(err)
	}()

	// wait for servers to start
	time.Sleep(time.Second * 2)

	client, err := client.New("tcp://" + address)
	require.NoError(t, err)

	return server, vmLnd, client, cancel
}

// MakeTxKV returns a text transaction, allong with expected key, value pair
func MakeTxKV() ([]byte, []byte, []byte) {
	k := []byte(rand.Str(2))
	v := []byte(rand.Str(2))
	return k, v, append(k, append([]byte("="), v...)...)
}

func testABCIInfo(t *testing.T, client *client.Client, expected *coretypes.ResultABCIInfo) {
	result := new(coretypes.ResultABCIInfo)
	_, err := client.Call(context.Background(), "abci_info", map[string]interface{}{}, result)
	require.NoError(t, err)
	require.Equal(t, expected.Response.Version, result.Response.Version)
	require.Equal(t, expected.Response.AppVersion, result.Response.AppVersion)
	require.Equal(t, expected.Response.LastBlockHeight, result.Response.LastBlockHeight)
	require.NotNil(t, result.Response.LastBlockAppHash)
}

func testABCIQuery(t *testing.T, client *client.Client, params map[string]interface{}, expected interface{}) {
	result := new(coretypes.ResultABCIQuery)
	_, err := client.Call(context.Background(), "abci_query", params, result)
	require.NoError(t, err)
	require.True(t, result.Response.IsOK())
	require.EqualValues(t, expected, result.Response.Value)
}

func testBroadcastTxCommit(t *testing.T, client *client.Client, vm *LandslideVM, params map[string]interface{}) *coretypes.ResultBroadcastTxCommit {
	initMempoolSize := vm.mempool.Size()
	result := new(coretypes.ResultBroadcastTxCommit)
	_, err := client.Call(context.Background(), "broadcast_tx_commit", params, result)
	require.NoError(t, err)
	require.True(t, result.CheckTx.IsOK())
	require.True(t, result.TxResult.IsOK())
	require.Equal(t, initMempoolSize, vm.mempool.Size())
	return result
}

func testBroadcastTxSync(t *testing.T, client *client.Client, vm *LandslideVM, params map[string]interface{}) *coretypes.ResultBroadcastTx {
	initMempoolSize := vm.mempool.Size()

	result := new(coretypes.ResultBroadcastTx)
	_, err := client.Call(context.Background(), "broadcast_tx_sync", params, result)
	require.NoError(t, err)
	require.Equal(t, result, abcitypes.CodeTypeOK)
	require.Equal(t, initMempoolSize+1, vm.mempool.Size())
	tx := params["tx"].(types.Tx)
	require.EqualValues(t, tx, result.Data.String())
	require.EqualValues(t, tx, vm.mempool.ReapMaxTxs(-1)[0])
	return result
}

func testBroadcastTxAsync(t *testing.T, client *client.Client, vm *LandslideVM, params map[string]interface{}) *coretypes.ResultBroadcastTx {
	result := new(coretypes.ResultBroadcastTx)
	_, err := client.Call(context.Background(), "broadcast_tx_async", params, result)
	require.NoError(t, err)
	require.NotNil(t, result.Hash)
	require.Equal(t, result.Code, abcitypes.CodeTypeOK)
	return result
}

func checkTxResult(t *testing.T, client *client.Client, vm *LandslideVM, env *txRuntimeEnv) {
	ctx, cancelCtx := context.WithTimeout(context.Background(), 10*time.Second)
	for {
		select {
		case <-ctx.Done():
			cancelCtx()
			t.Fatal("Broadcast tx timeout exceeded")
		default:
			if vm.mempool.Size() == env.initMemPoolSize+1 {
				cancelCtx()
				testABCIQuery(t, client, map[string]interface{}{"path": "/key", "data": fmt.Sprintf("%x", env.key)}, env.value)
				testABCIQuery(t, client, map[string]interface{}{"path": "/hash", "data": fmt.Sprintf("%x", env.hash)}, env.value)
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func checkCommittedTxResult(t *testing.T, client *client.Client, env *txRuntimeEnv) {
	testABCIQuery(t, client, map[string]interface{}{"path": "/key", "data": fmt.Sprintf("%x", env.key)}, env.value)
	testABCIQuery(t, client, map[string]interface{}{"path": "/hash", "data": fmt.Sprintf("%x", env.hash)}, env.value)
}

func TestABCIService(t *testing.T) {
	server, vm, client, cancel := setupRPC(t, buildAccept)
	defer server.Close()
	defer vm.mempool.Flush()
	defer cancel()

	t.Run("ABCIInfo", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			initialHeight := vm.state.LastBlockHeight
			testABCIInfo(t, client, &coretypes.ResultABCIInfo{
				Response: abcitypes.ResponseInfo{
					Version:         version.ABCIVersion,
					AppVersion:      kvstore.AppVersion,
					LastBlockHeight: initialHeight,
				},
			})
			_, _, tx := MakeTxKV()
			testBroadcastTxCommit(t, client, vm, map[string]interface{}{"tx": tx})
			testABCIInfo(t, client, &coretypes.ResultABCIInfo{
				Response: abcitypes.ResponseInfo{
					Version:         version.ABCIVersion,
					AppVersion:      kvstore.AppVersion,
					LastBlockHeight: initialHeight + 1,
				},
			})
		}
	})

	t.Run("ABCIQuery", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			k, v, tx := MakeTxKV()
			testBroadcastTxCommit(t, client, vm, map[string]interface{}{"tx": tx})
			path := "/key"
			params := map[string]interface{}{"path": path, "data": fmt.Sprintf("%x", k)}
			testABCIQuery(t, client, params, v)
		}
	})

	t.Run("BroadcastTxCommit", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			k, v, tx := MakeTxKV()
			result := testBroadcastTxCommit(t, client, vm, map[string]interface{}{"tx": tx})
			checkCommittedTxResult(t, client, &txRuntimeEnv{key: k, value: v, hash: result.Hash})
		}
	})

	t.Run("BroadcastTxAsync", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			k, v, tx := MakeTxKV()
			initMempoolSize := vm.mempool.Size()
			result := testBroadcastTxAsync(t, client, vm, map[string]interface{}{"tx": tx})
			checkTxResult(t, client, vm, &txRuntimeEnv{key: k, value: v, hash: result.Hash, initMemPoolSize: initMempoolSize})
		}
	})

	t.Run("BroadcastTxSync", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			k, v, tx := MakeTxKV()
			initMempoolSize := vm.mempool.Size()
			result := testBroadcastTxSync(t, client, vm, map[string]interface{}{"tx": tx})
			checkTxResult(t, client, vm, &txRuntimeEnv{key: k, value: v, hash: result.Hash, initMemPoolSize: initMempoolSize})
		}
	})
}

func TestStatusService(t *testing.T) {
	server, vm, client, cancel := setupRPC(t, buildAccept)
	defer server.Close()
	defer vm.mempool.Flush()
	defer cancel()

	t.Run("Status", func(t *testing.T) {
		testStatus(t, client, &coretypes.ResultStatus{})
	})
}

func TestHealth(t *testing.T) {
	server, _, client, cancel := setupRPC(t, buildAccept)
	defer server.Close()
	defer cancel()

	result := new(coretypes.ResultHealth)
	_, err := client.Call(context.Background(), "health", map[string]interface{}{}, result)
	require.NoError(t, err)

	t.Logf("Health result %+v", result)
}

func testStatus(t *testing.T, client *client.Client, expected *coretypes.ResultStatus) {
	result := new(coretypes.ResultStatus)
	_, err := client.Call(context.Background(), "status", map[string]interface{}{}, result)
	require.NoError(t, err)
	require.Equal(t, expected.NodeInfo.Moniker, result.NodeInfo.Moniker)
}

func TestRPC(t *testing.T) {
	//TODO: complicated combinations
	server, _, client, cancel := setupRPC(t, buildAccept)
	defer server.Close()
	defer cancel()

	tests := []struct {
		name     string
		method   string
		params   map[string]interface{}
		response interface{}
	}{
		//{"Health", "health", map[string]interface{}{}, new(ctypes.ResultHealth)},
		//{"Status", "status", map[string]interface{}{}, new(ctypes.ResultStatus)},
		//{"NetInfo", "net_info", map[string]interface{}{}, new(ctypes.ResultNetInfo)},
		//{"Blockchain", "blockchain", map[string]interface{}{}, new(ctypes.ResultBlockchainInfo)},
		//{"Genesis", "genesis", map[string]interface{}{}, new(ctypes.ResultGenesis)},
		//{"GenesisChunk", "genesis_chunked", map[string]interface{}{}, new(ctypes.ResultGenesisChunk)},
		//{"Block", "block", map[string]interface{}{}, new(ctypes.ResultBlock)},
		//{"BlockResults", "block_results", map[string]interface{}{}, new(ctypes.ResultBlockResults)},
		//{"Commit", "commit", map[string]interface{}{}, new(ctypes.ResultCommit)},
		//{"Header", "header", map[string]interface{}{}, new(ctypes.ResultHeader)},
		//{"HeaderByHash", "header_by_hash", map[string]interface{}{}, new(ctypes.ResultHeader)},
		//{"CheckTx", "check_tx", map[string]interface{}{}, new(ctypes.ResultCheckTx)},
		//{"Tx", "tx", map[string]interface{}{}, new(ctypes.ResultTx)},
		//{"TxSearch", "tx_search", map[string]interface{}{}, new(ctypes.ResultTxSearch)},
		//{"BlockSearch", "block_search", map[string]interface{}{}, new(ctypes.ResultBlockSearch)},
		//{"Validators", "validators", map[string]interface{}{}, new(ctypes.ResultValidators)},
		//{"DumpConsensusState", "dump_consensus_state", map[string]interface{}{}, new(ctypes.ResultDumpConsensusState)},
		//{"ConsensusState", "consensus_state", map[string]interface{}{}, new(ctypes.ResultConsensusState)},
		//{"ConsensusParams", "consensus_params", map[string]interface{}{}, new(ctypes.ResultConsensusParams)},
		//{"UnconfirmedTxs", "unconfirmed_txs", map[string]interface{}{}, new(ctypes.ResultUnconfirmedTxs)},
		//{"NumUnconfirmedTxs", "num_unconfirmed_txs", map[string]interface{}{}, new(ctypes.ResultUnconfirmedTxs)},
		//+{"BroadcastTxSync", "broadcast_tx_sync", map[string]interface{}{}, new(ctypes.ResultBroadcastTx)},
		//+{"BroadcastTxAsync", "broadcast_tx_async", map[string]interface{}{}, new(ctypes.ResultBroadcastTx)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.Call(context.Background(), tt.method, tt.params, tt.response)
			require.NoError(t, err)
			t.Logf("%s result %+v", tt.name, tt.response)
		})
	}
}
