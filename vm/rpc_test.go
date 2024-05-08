package vm

import (
	"context"
	"github.com/cometbft/cometbft/libs/rand"
	vmpb "github.com/consideritdone/landslidevm/proto/vm"
	"github.com/stretchr/testify/assert"
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
		// panic(err)
		// require.NoError(t, err)
	}()

	// wait for servers to start
	time.Sleep(time.Second * 2)

	client, err := client.New("tcp://" + address)
	require.NoError(t, err)

	return server, vmLnd, client
}

// MakeTxKV returns a text transaction, allong with expected key, value pair
func MakeTxKV() ([]byte, []byte, []byte) {
	k := []byte(rand.Str(2))
	v := []byte(rand.Str(2))
	return k, v, append(k, append([]byte("="), v...)...)
}

func testABCIInfo(t *testing.T, client *client.Client, expected *ctypes.ResultABCIInfo) {
	result := new(ctypes.ResultABCIInfo)
	_, err := client.Call(context.Background(), "abci_info", map[string]interface{}{}, result)
	require.NoError(t, err)
	require.Equal(t, expected.Response.AppVersion, result.Response.AppVersion)
	require.Equal(t, expected.Response.LastBlockHeight, result.Response.LastBlockHeight)
	require.Equal(t, expected.Response.LastBlockHeight, result.Response.LastBlockAppHash)
	//TODO: deepEqual
}

func testABCIQuery(t *testing.T, client *client.Client, expected *ctypes.ResultABCIQuery) {
	result := new(ctypes.ResultABCIInfo)
	_, err := client.Call(context.Background(), "abci_query", map[string]interface{}{}, result)
	require.NoError(t, err)
	//t.Logf("%+v", reply)
	//require.Equal(t, expected.Response.AppVersion, result.Response.AppVersion)
	//require.Equal(t, expected.Response.LastBlockHeight, result.Response.LastBlockHeight)
	//require.Equal(t, expected.Response.LastBlockHeight, result.Response.LastBlockAppHash)
	//TODO: deepEqual
	//reply, err := service.ABCIQuery(&rpctypes.Context{}, "/key", k, 0, false)
	//if assert.Nil(t, err) && assert.True(t, reply.Response.IsOK()) {
	//	assert.EqualValues(t, v, reply.Response.Value)
	//}
	//spew.Dump(vm.mempool.Size())
}

func testBroadcastTxCommit(t *testing.T, client *client.Client, vm *LandslideVM, expected *ctypes.ResultBroadcastTxCommit) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func(ctx context.Context) {
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
					_, err = vm.BlockAccept(ctx, &vmpb.BlockAcceptRequest{})
					require.NoError(t, err)
				} else {
					time.Sleep(500 * time.Millisecond)
				}
			}
		}
	}(ctx)

	result := new(ctypes.ResultABCIInfo)
	_, err := client.Call(context.Background(), "broadcast_tx_commit", map[string]interface{}{}, result)
	assert.NoError(t, err)
	//TODO: deep equal
	//assert.True(t, reply.CheckTx.IsOK())
	//assert.True(t, reply.DeliverTx.IsOK())
	//assert.Equal(t, 0, vm.mempool.Size())
}

func TestABCIService(t *testing.T) {
	server, vm, client := setupRPC(t)
	defer server.Close()

	t.Run("ABCIInfo", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			k, v, tx := MakeTxKV()
			t.Logf("%+v %+v %+v", k, v, tx)
			testBroadcastTxCommit(t, client, vm, &ctypes.ResultBroadcastTxCommit{})
			testABCIInfo(t, client, &ctypes.ResultABCIInfo{})
		}
	})

	t.Run("ABCIQuery", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			k, v, tx := MakeTxKV()
			t.Logf("%+v %+v %+v", k, v, tx)
			testBroadcastTxCommit(t, client, vm, &ctypes.ResultBroadcastTxCommit{})
			testABCIQuery(t, client, &ctypes.ResultABCIQuery{})
		}
	})
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

func TestRPC(t *testing.T) {
	//TODO: complicated combinations
	server, _, client := setupRPC(t)
	defer server.Close()

	tests := []struct {
		name     string
		method   string
		params   map[string]interface{}
		response interface{}
	}{
		{"Health", "health", map[string]interface{}{}, new(ctypes.ResultHealth)},
		{"Status", "status", map[string]interface{}{}, new(ctypes.ResultStatus)},
		{"NetInfo", "net_info", map[string]interface{}{}, new(ctypes.ResultNetInfo)},
		{"Blockchain", "blockchain", map[string]interface{}{}, new(ctypes.ResultBlockchainInfo)},
		{"Genesis", "genesis", map[string]interface{}{}, new(ctypes.ResultGenesis)},
		{"GenesisChunk", "genesis_chunked", map[string]interface{}{}, new(ctypes.ResultGenesisChunk)},
		{"Block", "block", map[string]interface{}{}, new(ctypes.ResultBlock)},
		{"BlockResults", "block_results", map[string]interface{}{}, new(ctypes.ResultBlockResults)},
		{"Commit", "commit", map[string]interface{}{}, new(ctypes.ResultCommit)},
		{"Header", "header", map[string]interface{}{}, new(ctypes.ResultHeader)},
		{"HeaderByHash", "header_by_hash", map[string]interface{}{}, new(ctypes.ResultHeader)},
		{"CheckTx", "check_tx", map[string]interface{}{}, new(ctypes.ResultCheckTx)},
		{"Tx", "tx", map[string]interface{}{}, new(ctypes.ResultTx)},
		{"TxSearch", "tx_search", map[string]interface{}{}, new(ctypes.ResultTxSearch)},
		{"BlockSearch", "block_search", map[string]interface{}{}, new(ctypes.ResultBlockSearch)},
		{"Validators", "validators", map[string]interface{}{}, new(ctypes.ResultValidators)},
		{"DumpConsensusState", "dump_consensus_state", map[string]interface{}{}, new(ctypes.ResultDumpConsensusState)},
		{"ConsensusState", "consensus_state", map[string]interface{}{}, new(ctypes.ResultConsensusState)},
		{"ConsensusParams", "consensus_params", map[string]interface{}{}, new(ctypes.ResultConsensusParams)},
		{"UnconfirmedTxs", "unconfirmed_txs", map[string]interface{}{}, new(ctypes.ResultUnconfirmedTxs)},
		{"NumUnconfirmedTxs", "num_unconfirmed_txs", map[string]interface{}{}, new(ctypes.ResultUnconfirmedTxs)},
		{"BroadcastTxSync", "broadcast_tx_sync", map[string]interface{}{}, new(ctypes.ResultBroadcastTx)},
		{"BroadcastTxAsync", "broadcast_tx_async", map[string]interface{}{}, new(ctypes.ResultBroadcastTx)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.Call(context.Background(), tt.method, tt.params, tt.response)
			require.NoError(t, err)
			t.Logf("%s result %+v", tt.name, tt.response)
		})
	}
}
