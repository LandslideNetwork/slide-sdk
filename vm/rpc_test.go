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

var (
	expectedResultHealth = ctypes.ResultHealth{}
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

func TestRPC(t *testing.T) {
	server, _, client := setupRPC(t)
	defer server.Close()

	tests := []struct {
		name     string
		method   string
		params   map[string]interface{}
		response interface{}
		expected interface{}
	}{
		{"Health", "health", map[string]interface{}{}, new(ctypes.ResultHealth), ctypes.ResultHealth{}},
		{"Status", "status", map[string]interface{}{}, new(ctypes.ResultStatus), ctypes.ResultStatus{}},
		{"NetInfo", "net_info", map[string]interface{}{}, new(ctypes.ResultNetInfo), ctypes.ResultNetInfo{}},
		{"Blockchain", "blockchain", map[string]interface{}{}, new(ctypes.ResultBlockchainInfo), ctypes.ResultBlockchainInfo{}},
		{"Genesis", "genesis", map[string]interface{}{}, new(ctypes.ResultGenesis), ctypes.ResultGenesis{}},
		{"GenesisChunk", "genesis_chunked", map[string]interface{}{}, new(ctypes.ResultGenesisChunk), ctypes.ResultGenesisChunk{}},
		{"Block", "block", map[string]interface{}{}, new(ctypes.ResultBlock), ctypes.ResultBlock{}},
		{"BlockResults", "block_results", map[string]interface{}{}, new(ctypes.ResultBlockResults), ctypes.ResultBlockResults{}},
		{"Commit", "commit", map[string]interface{}{}, new(ctypes.ResultCommit), ctypes.ResultCommit{}},
		{"Header", "header", map[string]interface{}{}, new(ctypes.ResultHeader), ctypes.ResultHeader{}},
		{"HeaderByHash", "header_by_hash", map[string]interface{}{}, new(ctypes.ResultHeader), ctypes.ResultHeader{}},
		{"CheckTx", "check_tx", map[string]interface{}{}, new(ctypes.ResultCheckTx), ctypes.ResultCheckTx{}},
		{"Tx", "tx", map[string]interface{}{}, new(ctypes.ResultTx), ctypes.ResultTx{}},
		{"TxSearch", "tx_search", map[string]interface{}{}, new(ctypes.ResultTxSearch), ctypes.ResultTxSearch{}},
		{"BlockSearch", "block_search", map[string]interface{}{}, new(ctypes.ResultBlockSearch), ctypes.ResultBlockSearch{}},
		{"Validators", "validators", map[string]interface{}{}, new(ctypes.ResultValidators), ctypes.ResultValidators{}},
		{"DumpConsensusState", "dump_consensus_state", map[string]interface{}{}, new(ctypes.ResultDumpConsensusState), ctypes.ResultDumpConsensusState{}},
		{"ConsensusState", "consensus_state", map[string]interface{}{}, new(ctypes.ResultConsensusState), ctypes.ResultConsensusState{}},
		{"ConsensusParams", "consensus_params", map[string]interface{}{}, new(ctypes.ResultConsensusParams), ctypes.ResultConsensusParams{}},
		{"UnconfirmedTxs", "unconfirmed_txs", map[string]interface{}{}, new(ctypes.ResultUnconfirmedTxs), ctypes.ResultUnconfirmedTxs{}},
		{"NumUnconfirmedTxs", "num_unconfirmed_txs", map[string]interface{}{}, new(ctypes.ResultUnconfirmedTxs), ctypes.ResultUnconfirmedTxs{}},
		{"BroadcastTxSync", "broadcast_tx_sync", map[string]interface{}{}, new(ctypes.ResultBroadcastTx), ctypes.ResultBroadcastTx{}},
		{"BroadcastTxAsync", "broadcast_tx_async", map[string]interface{}{}, new(ctypes.ResultBroadcastTx), ctypes.ResultBroadcastTx{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.Call(context.Background(), tt.method, tt.params, tt.response)
			require.NoError(t, err)
			require.Equal(t, tt.expected, tt.response)
		})
	}
}
