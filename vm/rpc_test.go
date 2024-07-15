package vm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/cometbft/cometbft/abci/example/kvstore"
	"github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/libs/bytes"
	"github.com/cometbft/cometbft/libs/pubsub"
	"github.com/cometbft/cometbft/libs/rand"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/rpc/jsonrpc/client"
	rpcserver "github.com/cometbft/cometbft/rpc/jsonrpc/server"
	"github.com/cometbft/cometbft/types"
	"github.com/cometbft/cometbft/version"
	vmpb "github.com/consideritdone/landslidevm/proto/vm"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	bftjson "github.com/cometbft/cometbft/libs/json"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	rpcclienthttp "github.com/cometbft/cometbft/rpc/client/http"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/stretchr/testify/require"

	"github.com/consideritdone/landslidevm/jsonrpc"
)

type HandlerRPC func(vmLnd *LandslideVM) http.Handler

type BlockBuilder func(*testing.T, context.Context, *LandslideVM)

type txRuntimeEnv struct {
	key, value, hash []byte
	initHeight       int64
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

func noAction(t *testing.T, ctx context.Context, vm *LandslideVM) {

}

func setupServer(t *testing.T, handler HandlerRPC, blockBuilder BlockBuilder) (*http.Server, *LandslideVM, string, context.CancelFunc) {
	vm := newFreshKvApp(t)
	vmLnd := vm.(*LandslideVM)

	mux := handler(vmLnd)

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

	return server, vmLnd, address, cancel
}

func setupRPCServer(t *testing.T, handler HandlerRPC, blockBuilder BlockBuilder) (*http.Server, *LandslideVM, rpcclient.Client, context.CancelFunc) {
	server, vmLnd, address, cancel := setupServer(t, handler, blockBuilder)
	client, err := rpcclienthttp.New("tcp://"+address, "/websocket")
	require.NoError(t, err)
	return server, vmLnd, client, cancel
}

func setupWSRPCServer(t *testing.T, handler HandlerRPC, blockBuilder BlockBuilder) (*http.Server, *LandslideVM, *client.WSClient, context.CancelFunc) {
	server, vmLnd, address, cancel := setupServer(t, handler, blockBuilder)
	client, err := client.NewWS("tcp://"+address, "/websocket")
	require.NoError(t, err)
	return server, vmLnd, client, cancel
}

func setupRPC(vmLnd *LandslideVM) http.Handler {
	mux := http.NewServeMux()
	jsonrpc.RegisterRPCFuncs(mux, NewRPC(vmLnd).Routes(), vmLnd.logger)
	return mux
}

func setupWSRPC(vmLnd *LandslideVM) http.Handler {
	mux := http.NewServeMux()
	jsonrpc.RegisterRPCFuncs(mux, NewRPC(vmLnd).Routes(), vmLnd.logger)
	tmRPC := NewRPC(vmLnd)
	wm := rpcserver.NewWebsocketManager(tmRPC.TMRoutes(),
		rpcserver.OnDisconnect(func(remoteAddr string) {
			err := vmLnd.eventBus.UnsubscribeAll(context.Background(), remoteAddr)
			if err != nil && err != pubsub.ErrSubscriptionNotFound {
				vmLnd.logger.Error("Failed to unsubscribe addr from events", "addr", remoteAddr, "err", err)
			}
		}),
		rpcserver.ReadLimit(config.DefaultRPCConfig().MaxBodyBytes),
		rpcserver.WriteChanCapacity(config.DefaultRPCConfig().WebSocketWriteBufferSize),
	)
	wm.SetLogger(vmLnd.logger)
	mux.HandleFunc("/websocket", wm.WebsocketHandler)
	return mux
}

// MakeTxKV returns a text transaction, allong with expected key, value pair
func MakeTxKV() ([]byte, []byte, []byte) {
	k := []byte(rand.Str(2))
	v := []byte(rand.Str(2))
	return k, v, append(k, append([]byte("="), v...)...)
}

func testABCIInfo(t *testing.T, client rpcclient.Client, expected *coretypes.ResultABCIInfo) {
	result, err := client.ABCIInfo(context.Background())
	require.NoError(t, err)
	require.Equal(t, expected.Response.Version, result.Response.Version)
	require.Equal(t, expected.Response.AppVersion, result.Response.AppVersion)
	require.Equal(t, expected.Response.LastBlockHeight, result.Response.LastBlockHeight)
	require.NotNil(t, result.Response.LastBlockAppHash)
}

func testABCIQuery(t *testing.T, client rpcclient.Client, path string, data bytes.HexBytes, expected interface{}) {
	result, err := client.ABCIQuery(context.Background(), path, data)
	require.NoError(t, err)
	require.True(t, result.Response.IsOK())
	require.EqualValues(t, expected, result.Response.Value)
}

func testBroadcastTxCommit(t *testing.T, client rpcclient.Client, vm *LandslideVM, tx types.Tx) *coretypes.ResultBroadcastTxCommit {
	initMempoolSize := vm.mempool.Size()
	result, err := client.BroadcastTxCommit(context.Background(), tx)
	waitForStateUpdate(result.Height, vm)
	require.NoError(t, err)
	require.True(t, result.CheckTx.IsOK())
	require.True(t, result.TxResult.IsOK())
	require.Equal(t, initMempoolSize, vm.mempool.Size())
	return result
}

func testBroadcastTxSync(t *testing.T, client rpcclient.Client, tx types.Tx) *coretypes.ResultBroadcastTx {
	result, err := client.BroadcastTxSync(context.Background(), tx)
	require.NoError(t, err)
	require.Equal(t, result.Code, abcitypes.CodeTypeOK)
	return result
}

func testBroadcastTxAsync(t *testing.T, client rpcclient.Client, tx types.Tx) *coretypes.ResultBroadcastTx {
	result, err := client.BroadcastTxAsync(context.Background(), tx)
	require.NoError(t, err)
	require.NotNil(t, result.Hash)
	require.Equal(t, result.Code, abcitypes.CodeTypeOK)
	return result
}

func testStatus(t *testing.T, client rpcclient.Client, expected *coretypes.ResultStatus) {
	result, err := client.Status(context.Background())
	require.NoError(t, err)
	//TODO: test node info moniker
	//require.Equal(t, expected.NodeInfo.Moniker, result.NodeInfo.Moniker)
	require.Equal(t, expected.SyncInfo.LatestBlockHeight, result.SyncInfo.LatestBlockHeight)
}

func testNetInfo(t *testing.T, client rpcclient.Client, expected *coretypes.ResultNetInfo) {
	_, err := client.NetInfo(context.Background())
	require.NoError(t, err)
	//TODO: check equality
	//require.Equal(t, expected.Listening, result.Listening)
	//require.Equal(t, expected.Peers, result.Peers)
	//require.Equal(t, expected.Listeners, result.Listeners)
	//require.Equal(t, expected.NPeers, result.NPeers)
	//TODO: OR compare to desired values
	//require.NoError(t, err, "%d: %+v", i, err)
	//assert.True(t, netinfo.Listening)
	//assert.Empty(t, netinfo.Peers)

}

func testConsensusState(t *testing.T, client rpcclient.Client, expected *coretypes.ResultConsensusState) {
	_, err := client.ConsensusState(context.Background())
	require.NoError(t, err)
	//TODO: check equality
	//require.Equal(t, expected.RoundState, result.RoundState)
	//TODO: OR compare to desired values
	//assert.NotEmpty(t, cons.RoundState)
}

func testDumpConsensusState(t *testing.T, client rpcclient.Client, expected *coretypes.ResultDumpConsensusState) {
	_, err := client.DumpConsensusState(context.Background())
	require.NoError(t, err)
	//TODO: check equality
	//require.Equal(t, expected.RoundState, result.RoundState)
	//require.EqualValues(t, expected.Peers, result.Peers)
	//TODO: OR compare to desired values
	//assert.NotEmpty(t, cons.RoundState)
	//require.ElementsMatch(t, expected.Peers, result.Peers)
}

func testConsensusParams(t *testing.T, client rpcclient.Client, height *int64, expected *coretypes.ResultConsensusParams) {
	result, err := client.ConsensusParams(context.Background(), height)
	require.NoError(t, err)
	//TODO: check equality
	require.Equal(t, expected.BlockHeight, result.BlockHeight)
	//require.Equal(t, expected.ConsensusParams.Version.App, result.ConsensusParams.Version.App)
	//require.Equal(t, expected.ConsensusParams.Hash(), result.ConsensusParams.Hash())
}

func testHealth(t *testing.T, client rpcclient.Client) {
	_, err := client.Health(context.Background())
	require.NoError(t, err)
}

func testBlockchainInfo(t *testing.T, client rpcclient.Client, minHeight int64, maxHeight int64, expected *coretypes.ResultBlockchainInfo) {
	result, err := client.BlockchainInfo(context.Background(), minHeight, maxHeight)
	require.NoError(t, err)
	require.Equal(t, expected.LastHeight, result.LastHeight)
	//TODO: implement same sorting method
	//lastMeta := result.BlockMetas[len(result.BlockMetas)-1]
	//expectedLastMeta := expected.BlockMetas[len(expected.BlockMetas)-1]
	//require.Equal(t, expectedLastMeta.NumTxs, lastMeta.NumTxs)
	//require.Equal(t, expectedLastMeta.Header.AppHash, lastMeta.Header.AppHash)
	//require.Equal(t, expectedLastMeta.BlockID, lastMeta.BlockID)
}

func testBlock(t *testing.T, client rpcclient.Client, height *int64, expected *coretypes.ResultBlock) *coretypes.ResultBlock {
	result, err := client.Block(context.Background(), height)
	require.NoError(t, err)
	require.Equal(t, expected.Block.ChainID, result.Block.ChainID)
	require.Equal(t, expected.Block.Height, result.Block.Height)
	require.Equal(t, expected.Block.AppHash, result.Block.AppHash)
	return result
}

func testBlockByHash(t *testing.T, client rpcclient.Client, hash []byte, expected *coretypes.ResultBlock) *coretypes.ResultBlock {
	result, err := client.BlockByHash(context.Background(), hash)
	require.NoError(t, err)
	require.Equal(t, expected.Block.ChainID, result.Block.ChainID)
	require.Equal(t, expected.Block.Height, result.Block.Height)
	require.Equal(t, expected.Block.AppHash, result.Block.AppHash)
	return result
}

func testBlockResults(t *testing.T, client rpcclient.Client, height *int64, expected *coretypes.ResultBlockResults) {
	result, err := client.BlockResults(context.Background(), height)
	require.NoError(t, err)
	require.NotNil(t, result)
	//require.Equal(t, expected.Height, result.Height)
	//require.Equal(t, expected.AppHash, result.AppHash)
	//require.Equal(t, expected.TxsResults, result.TxsResults)
}

func testBlockSearch(t *testing.T, client rpcclient.Client, query string, page *int, perPage *int, orderBy string, expected *coretypes.ResultBlockSearch) {
	result, err := client.BlockSearch(context.Background(), query, page, perPage, orderBy)
	require.NoError(t, err)
	require.Equal(t, expected.TotalCount, result.TotalCount)
	sort.Slice(expected.Blocks, func(i, j int) bool {
		return expected.Blocks[i].Block.Height < expected.Blocks[j].Block.Height
	})
	sort.Slice(result.Blocks, func(i, j int) bool {
		return result.Blocks[i].Block.Height < result.Blocks[j].Block.Height
	})
	require.Equal(t, expected.Blocks, result.Blocks)
}

func testTx(t *testing.T, client rpcclient.Client, hash []byte, prove bool, expected *coretypes.ResultTx) {
	result, err := client.Tx(context.Background(), hash, prove)
	require.NoError(t, err)
	require.EqualValues(t, expected.Hash, result.Hash)
	require.EqualValues(t, expected.Tx, result.Tx)
	require.EqualValues(t, expected.Height, result.Height)
	require.EqualValues(t, expected.TxResult, result.TxResult)
}

func testTxSearch(t *testing.T, client rpcclient.Client, query string, prove bool, page *int, perPage *int, orderBy string, expected *coretypes.ResultTxSearch) {
	result, err := client.TxSearch(context.Background(), query, prove, page, perPage, orderBy)
	require.NoError(t, err)
	require.EqualValues(t, expected.TotalCount, result.TotalCount)
	require.EqualValues(t, expected.Txs, result.Txs)
}

func testCommit(t *testing.T, client rpcclient.Client, height *int64, expected *coretypes.ResultCommit) {
	result, err := client.Commit(context.Background(), height)
	require.NoError(t, err)
	//TODO: implement tests for all fields of result
	//require.Equal(t, expected.Version, result.Version)
	require.Equal(t, expected.ChainID, result.ChainID)
	require.Equal(t, expected.Height, result.Height)
	//require.Equal(t, expected.Time, result.Time)
	//require.Equal(t, expected.LastBlockID, result.LastBlockID)
	require.Equal(t, expected.LastCommitHash, result.LastCommitHash)
	require.Equal(t, expected.DataHash, result.DataHash)
	require.Equal(t, expected.ValidatorsHash, result.ValidatorsHash)
	require.Equal(t, expected.NextValidatorsHash, result.NextValidatorsHash)
	require.Equal(t, expected.ConsensusHash, result.ConsensusHash)
	require.Equal(t, expected.AppHash, result.AppHash)
	require.Equal(t, expected.LastResultsHash, result.LastResultsHash)
	require.Equal(t, expected.EvidenceHash, result.EvidenceHash)
	require.Equal(t, expected.ProposerAddress, result.ProposerAddress)
	//TODO: fix empty height for non-genesis blocks, or even simulate signatures
	//require.Equal(t, expected.Commit.Height, result.Commit.Height)
	//require.Equal(t, expected.Commit.Round, result.Commit.Round)
	//require.Equal(t, expected.Commit.BlockID, result.Commit.BlockID)
	//require.EqualValues(t, expected.Commit.Signatures, result.Commit.Signatures)
}

func testUnconfirmedTxs(t *testing.T, client rpcclient.Client, limit *int, expected *coretypes.ResultUnconfirmedTxs) {
	result, err := client.UnconfirmedTxs(context.Background(), limit)
	require.NoError(t, err)
	require.Equal(t, expected.Total, result.Total)
	require.Equal(t, expected.Count, result.Count)
	require.EqualValues(t, expected.Txs, result.Txs)
}

func testNumUnconfirmedTxs(t *testing.T, client rpcclient.Client, expected *coretypes.ResultUnconfirmedTxs) {
	result, err := client.NumUnconfirmedTxs(context.Background())
	require.NoError(t, err)
	require.Equal(t, expected.Total, result.Total)
	require.Equal(t, expected.Count, result.Count)
}

func testCheckTx(t *testing.T, client rpcclient.Client, tx types.Tx, expected *coretypes.ResultCheckTx) {
	result, err := client.CheckTx(context.Background(), tx)
	require.NoError(t, err)
	require.Equal(t, result.Code, expected.Code)
}

func testSubscribe(t *testing.T, client rpcclient.Client) {
	const subscriber = "test-client"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	newBlockSub, err := client.Subscribe(ctx, subscriber, types.EventQueryNewBlock.String())
	require.NoError(t, err)
	// make sure to unregister after the test is over
	defer func() {
		if deferErr := client.UnsubscribeAll(ctx, subscriber); deferErr != nil {
			panic(deferErr)
		}
	}()

	select {
	case event := <-newBlockSub:
		t.Log("EVENT:", event)
	case <-ctx.Done():
		t.Error("timed out waiting for event")
	}
}

func waitForStateUpdate(expectedHeight int64, vm *LandslideVM) {
	for {
		if vm.state.LastBlockHeight() == expectedHeight {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func checkTxResult(t *testing.T, client rpcclient.Client, vm *LandslideVM, env *txRuntimeEnv) {
	ctx, cancelCtx := context.WithTimeout(context.Background(), 10*time.Second)
	for {
		select {
		case <-ctx.Done():
			cancelCtx()
			t.Fatal("Broadcast tx timeout exceeded")
		default:
			if vm.state.LastBlockHeight() == env.initHeight+1 {
				cancelCtx()
				testABCIQuery(t, client, "/key", env.key, env.value)
				//testABCIQuery(t, client, map[string]interface{}{"path": "/hash", "data": fmt.Sprintf("%x", env.hash)}, env.value)
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func checkCommittedTxResult(t *testing.T, client rpcclient.Client, env *txRuntimeEnv) {
	testABCIQuery(t, client, "/key", env.key, env.value)
	//testABCIQuery(t, client, map[string]interface{}{"path": "/hash", "data": fmt.Sprintf("%x", env.hash)}, env.value)
}

func TestBlockProduction(t *testing.T) {
	server, vm, client, cancel := setupRPCServer(t, setupRPC, buildAccept)
	defer server.Close()
	defer vm.mempool.Flush()
	defer cancel()

	initialHeight := vm.state.LastBlockHeight()

	for i := 1; i < 10; i++ {
		testStatus(t, client, &coretypes.ResultStatus{
			NodeInfo: p2p.DefaultNodeInfo{},
			SyncInfo: coretypes.SyncInfo{
				LatestBlockHeight: initialHeight + int64(i) - 1,
			},
			ValidatorInfo: coretypes.ValidatorInfo{},
		})

		// write something
		_, _, tx := MakeTxKV()
		previousAppHash := vm.state.AppHash()
		bres := testBroadcastTxCommit(t, client, vm, tx)

		testBlock(t, client, &bres.Height, &coretypes.ResultBlock{
			Block: &types.Block{
				Header: types.Header{
					ChainID: vm.state.ChainID(),
					Height:  bres.Height,
					AppHash: previousAppHash,
				},
			},
		})
	}
}

func TestABCIService(t *testing.T) {
	server, vm, client, cancel := setupRPCServer(t, setupRPC, buildAccept)
	defer server.Close()
	defer vm.mempool.Flush()
	defer cancel()

	t.Run("ABCIInfo", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			initialHeight := vm.state.LastBlockHeight()
			testABCIInfo(t, client, &coretypes.ResultABCIInfo{
				Response: abcitypes.ResponseInfo{
					Version:         version.ABCIVersion,
					AppVersion:      kvstore.AppVersion,
					LastBlockHeight: initialHeight,
				},
			})
			_, _, tx := MakeTxKV()
			testBroadcastTxCommit(t, client, vm, tx)
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
			testBroadcastTxCommit(t, client, vm, tx)
			path := "/key"
			testABCIQuery(t, client, path, k, v)
		}
	})

	t.Run("BroadcastTxCommit", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			k, v, tx := MakeTxKV()
			result := testBroadcastTxCommit(t, client, vm, tx)
			checkCommittedTxResult(t, client, &txRuntimeEnv{key: k, value: v, hash: result.Hash})
		}
	})

	t.Run("BroadcastTxAsync", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			k, v, tx := MakeTxKV()
			initHeight := vm.state.LastBlockHeight()
			result := testBroadcastTxAsync(t, client, tx)
			checkTxResult(t, client, vm, &txRuntimeEnv{key: k, value: v, hash: result.Hash, initHeight: initHeight})
		}
	})

	t.Run("BroadcastTxSync", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			k, v, tx := MakeTxKV()
			initHeight := vm.state.LastBlockHeight()
			result := testBroadcastTxSync(t, client, tx)
			checkTxResult(t, client, vm, &txRuntimeEnv{key: k, value: v, hash: result.Hash, initHeight: initHeight})
		}
		cancel()
		_, _, tx := MakeTxKV()
		initMempoolSize := vm.mempool.Size()
		testBroadcastTxSync(t, client, tx)
		//result := testBroadcastTxSync(t, client, vm, map[string]interface{}{"tx": tx})
		require.Equal(t, initMempoolSize+1, vm.mempool.Size())
		//TODO: kvstore return empty check tx result, use another app or implement missing methods
		//require.EqualValues(t, string(tx), result.Data.String())
		require.EqualValues(t, types.Tx(tx), vm.mempool.ReapMaxTxs(-1)[0])
	})
}

func TestStatusService(t *testing.T) {
	server, vm, client, cancel := setupRPCServer(t, setupRPC, buildAccept)
	defer server.Close()
	defer vm.mempool.Flush()
	defer cancel()

	t.Run("Status", func(t *testing.T) {
		initialHeight := vm.state.LastBlockHeight()
		for i := 0; i < 3; i++ {
			_, _, tx := MakeTxKV()
			result := testBroadcastTxCommit(t, client, vm, tx)
			require.EqualValues(t, result.Height, initialHeight+int64(1)+int64(i))
			testStatus(t, client, &coretypes.ResultStatus{
				NodeInfo: p2p.DefaultNodeInfo{},
				SyncInfo: coretypes.SyncInfo{
					LatestBlockHeight: initialHeight + int64(i) + 1,
				},
				ValidatorInfo: coretypes.ValidatorInfo{},
			})
		}
	})
}

func TestNetworkService(t *testing.T) {
	server, vm, client, cancel := setupRPCServer(t, setupRPC, buildAccept)
	defer server.Close()
	defer cancel()

	t.Run("NetInfo", func(t *testing.T) {
		testNetInfo(t, client, &coretypes.ResultNetInfo{
			Listening: true,
			Listeners: nil,
			NPeers:    0,
			Peers:     nil,
		})
	})

	t.Run("DumpConsensusState", func(t *testing.T) {
		testDumpConsensusState(t, client, &coretypes.ResultDumpConsensusState{
			RoundState: json.RawMessage{},
			Peers:      []coretypes.PeerStateInfo{},
		})
	})

	//TODO: implement consensus_state rpc method, than uncomment this code block
	t.Run("ConsensusState", func(t *testing.T) {
		testConsensusState(t, client, &coretypes.ResultConsensusState{
			RoundState: json.RawMessage{},
		})
	})

	t.Run("ConsensusParams", func(t *testing.T) {
		initialHeight := vm.state.LastBlockHeight()
		for i := 0; i < 3; i++ {
			_, _, tx := MakeTxKV()
			result := testBroadcastTxCommit(t, client, vm, tx)
			require.EqualValues(t, result.Height, initialHeight+int64(1)+int64(i))
			lastBlockHeight := vm.state.LastBlockHeight()
			testConsensusParams(t, client, &lastBlockHeight, &coretypes.ResultConsensusParams{
				BlockHeight: result.Height,
				//TODO: compare consensus params
				//ConsensusParams: types.ConsensusParams{},
			})
		}
	})

	t.Run("Health", func(t *testing.T) {
		testHealth(t, client)
	})
}

func TestHistoryService(t *testing.T) {
	server, vm, client, cancel := setupRPCServer(t, setupRPC, buildAccept)
	defer server.Close()
	defer cancel()

	t.Run("Genesis", func(t *testing.T) {
		result, err := client.Genesis(context.Background())
		require.NoError(t, err)
		require.Equal(t, vm.genesis, result.Genesis)
	})

	t.Run("GenesisChunked", func(t *testing.T) {
		first, err := client.GenesisChunked(context.Background(), 0)
		require.NoError(t, err)

		decoded := make([]string, 0, first.TotalChunks)
		for i := 0; i < first.TotalChunks; i++ {
			chunk, err := client.GenesisChunked(context.Background(), uint(i))
			require.NoError(t, err)
			data, err := base64.StdEncoding.DecodeString(chunk.Data)
			require.NoError(t, err)
			decoded = append(decoded, string(data))
		}
		doc := []byte(strings.Join(decoded, ""))

		var out types.GenesisDoc
		require.NoError(t, bftjson.Unmarshal(doc, &out), "first: %+v, doc: %s", first, string(doc))
	})

	t.Run("BlockchainInfo", func(t *testing.T) {
		blkMetas := make([]*types.BlockMeta, 0)
		for i := int64(1); i <= vm.state.LastBlockHeight(); i++ {
			blk := testBlock(t, client, &i, &coretypes.ResultBlock{
				Block: &types.Block{
					Header: types.Header{
						ChainID: vm.state.ChainID(),
						Height:  i,
						AppHash: vm.state.AppHash(),
					},
				},
			})
			bps, err := blk.Block.MakePartSet(types.BlockPartSizeBytes)
			require.NoError(t, err)
			blkMetas = append(blkMetas, &types.BlockMeta{
				BlockID:   types.BlockID{Hash: blk.Block.Hash(), PartSetHeader: bps.Header()},
				BlockSize: blk.Block.Size(),
				Header:    blk.Block.Header,
				NumTxs:    len(blk.Block.Data.Txs),
			})
		}
		initialHeight := vm.state.LastBlockHeight()
		testBlockchainInfo(t, client, 0, 0, &coretypes.ResultBlockchainInfo{
			LastHeight: initialHeight,
			BlockMetas: blkMetas,
		})
		_, _, tx := MakeTxKV()
		prevStateAppHash := vm.state.AppHash()
		bres := testBroadcastTxCommit(t, client, vm, tx)
		blk := testBlock(t, client, &bres.Height, &coretypes.ResultBlock{
			Block: &types.Block{
				Header: types.Header{
					ChainID: vm.state.ChainID(),
					Height:  bres.Height,
					AppHash: prevStateAppHash,
				},
			},
		})
		bps, err := blk.Block.MakePartSet(types.BlockPartSizeBytes)
		require.NoError(t, err)
		blkMetas = append(blkMetas, &types.BlockMeta{
			BlockID:   types.BlockID{Hash: blk.Block.Hash(), PartSetHeader: bps.Header()},
			BlockSize: blk.Block.Size(),
			Header:    blk.Block.Header,
			NumTxs:    len(blk.Block.Data.Txs),
		})
		//TODO: fix test blockchain info, unexpected height, uncomment this block of code
		testBlockchainInfo(t, client, 0, 0, &coretypes.ResultBlockchainInfo{
			LastHeight: initialHeight + 1,
			BlockMetas: blkMetas,
		})
	})
}

func TestSignService(t *testing.T) {
	server, vm, client, cancel := setupRPCServer(t, setupRPC, buildAccept)
	defer server.Close()
	defer cancel()

	t.Run("Block", func(t *testing.T) {
		initialHeight := vm.state.LastBlockHeight()
		for i := 0; i < 3; i++ {
			_, _, tx := MakeTxKV()
			prevAppHash := vm.state.AppHash()
			result := testBroadcastTxCommit(t, client, vm, tx)
			require.EqualValues(t, result.Height, initialHeight+int64(1)+int64(i))
			lastBlockHeight := vm.state.LastBlockHeight()
			testBlock(t, client, &lastBlockHeight, &coretypes.ResultBlock{
				Block: &types.Block{
					Header: types.Header{
						ChainID: vm.state.ChainID(),
						Height:  result.Height,
						AppHash: prevAppHash,
					},
				},
			})
		}
	})

	t.Run("BlockByHash", func(t *testing.T) {
		prevAppHash := vm.state.AppHash()
		_, _, tx := MakeTxKV()
		result := testBroadcastTxCommit(t, client, vm, tx)
		blk := testBlock(t, client, &result.Height, &coretypes.ResultBlock{
			Block: &types.Block{
				Header: types.Header{
					ChainID: vm.state.ChainID(),
					Height:  result.Height,
					AppHash: prevAppHash,
				},
			},
		})

		hash := blk.Block.Hash()
		//TODO: fix block search by hash: calcBlockHash give hash of different length in comparison of store and get block
		reply := testBlockByHash(t, client, hash.Bytes(), &coretypes.ResultBlock{
			Block: &types.Block{
				Header: types.Header{
					ChainID: vm.state.ChainID(),
					Height:  result.Height,
					AppHash: prevAppHash,
				},
			},
		})
		require.EqualValues(t, hash[:], reply.Block.Hash().Bytes())
	})

	//TODO: implement block_results rpc method, than uncomment this block of code
	t.Run("BlockResults", func(t *testing.T) {
		prevAppHash := vm.state.AppHash()
		_, _, tx := MakeTxKV()
		result := testBroadcastTxCommit(t, client, vm, tx)
		testBlockResults(t, client, nil, &coretypes.ResultBlockResults{
			Height:     result.Height,
			AppHash:    prevAppHash,
			TxsResults: []*abcitypes.ExecTxResult{&result.TxResult},
		})

		testBlockResults(t, client, &result.Height, &coretypes.ResultBlockResults{
			Height:     result.Height,
			AppHash:    prevAppHash,
			TxsResults: []*abcitypes.ExecTxResult{&result.TxResult},
		})
	})

	t.Run("Tx", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			_, _, tx := MakeTxKV()
			result := testBroadcastTxCommit(t, client, vm, tx)
			testTx(t, client, result.Hash.Bytes(), false, &coretypes.ResultTx{
				Hash:     result.Hash,
				Height:   result.Height,
				Index:    0,
				TxResult: result.TxResult,
				Tx:       tx,
				Proof:    types.TxProof{},
			})
		}
	})

	t.Run("TxSearch", func(t *testing.T) {
		_, _, tx := MakeTxKV()
		txReply := testBroadcastTxCommit(t, client, vm, tx)
		testTxSearch(t, client, fmt.Sprintf("tx.hash='%s'", txReply.Hash), false, nil, nil, "asc", &coretypes.ResultTxSearch{
			Txs: []*coretypes.ResultTx{{
				Hash:   txReply.Hash,
				Height: txReply.Height,
				//TODO: check index
				Index:    0,
				TxResult: txReply.TxResult,
				Tx:       tx,
				//TODO: check proof
				Proof: types.TxProof{},
			}},
			TotalCount: 1,
		})
		testTxSearch(t, client, fmt.Sprintf("tx.height=%d", txReply.Height), false, nil, nil, "asc", &coretypes.ResultTxSearch{
			Txs: []*coretypes.ResultTx{{
				Hash:   txReply.Hash,
				Height: txReply.Height,
				//TODO: check index
				Index:    0,
				TxResult: txReply.TxResult,
				Tx:       tx,
				//TODO: check proof
				Proof: types.TxProof{},
			}},
			TotalCount: 1,
		})
	})

	t.Run("Commit", func(t *testing.T) {
		prevAppHash := vm.state.AppHash()
		_, _, tx := MakeTxKV()
		txReply := testBroadcastTxCommit(t, client, vm, tx)
		lastBlockHeight := vm.state.LastBlockHeight()
		blk := testBlock(t, client, &lastBlockHeight, &coretypes.ResultBlock{
			Block: &types.Block{
				Header: types.Header{
					ChainID: vm.state.ChainID(),
					Height:  txReply.Height,
					AppHash: prevAppHash,
				},
			},
		})
		//TODO: implement check for all result commit fields
		testCommit(t, client, &txReply.Height, &coretypes.ResultCommit{
			SignedHeader: types.SignedHeader{
				Header: &types.Header{
					//Version:            bftversion.Consensus{},
					ChainID: vm.state.ChainID(),
					Height:  txReply.Height,
					//Time:               time.Time{},
					LastBlockID:        blk.BlockID,
					LastCommitHash:     blk.Block.LastCommitHash,
					DataHash:           blk.Block.DataHash,
					ValidatorsHash:     blk.Block.ValidatorsHash,
					NextValidatorsHash: blk.Block.NextValidatorsHash,
					ConsensusHash:      blk.Block.ConsensusHash,
					AppHash:            prevAppHash,
					LastResultsHash:    blk.Block.LastResultsHash,
					EvidenceHash:       blk.Block.EvidenceHash,
					ProposerAddress:    blk.Block.ProposerAddress,
				},
				Commit: &types.Commit{
					Height:     txReply.Height,
					Round:      0,
					BlockID:    types.BlockID{},
					Signatures: nil,
				},
			},
			CanonicalCommit: false,
		})
	})

	t.Run("BlockSearch", func(t *testing.T) {
		initialHeight := vm.state.LastBlockHeight()
		prevAppHash := vm.state.AppHash()
		_, _, tx := MakeTxKV()
		result := testBroadcastTxCommit(t, client, vm, tx)
		blk := testBlock(t, client, &result.Height, &coretypes.ResultBlock{
			Block: &types.Block{
				Header: types.Header{
					ChainID: vm.state.ChainID(),
					Height:  result.Height,
					AppHash: prevAppHash,
				},
			},
		})
		testBlockSearch(t, client, fmt.Sprintf("block.height=%d", initialHeight+1), nil, nil, "asc", &coretypes.ResultBlockSearch{
			Blocks:     []*coretypes.ResultBlock{blk},
			TotalCount: 1,
		})
		prevAppHash = vm.state.AppHash()
		_, _, tx = MakeTxKV()
		result = testBroadcastTxCommit(t, client, vm, tx)
		blk2 := testBlock(t, client, &result.Height, &coretypes.ResultBlock{
			Block: &types.Block{
				Header: types.Header{
					ChainID: vm.state.ChainID(),
					Height:  result.Height,
					AppHash: prevAppHash,
				},
			},
		})
		testBlockSearch(t, client, fmt.Sprintf("block.height>%d", initialHeight), nil, nil, "asc", &coretypes.ResultBlockSearch{
			Blocks:     []*coretypes.ResultBlock{blk, blk2},
			TotalCount: 2,
		})
	})
}

func TestMempoolService(t *testing.T) {
	server, vm, client, cancel := setupRPCServer(t, setupRPC, noAction)
	defer server.Close()
	defer cancel()

	t.Run("UnconfirmedTxs", func(t *testing.T) {
		limit := 10
		var count int
		_, _, tx := MakeTxKV()
		txs := []types.Tx{tx}
		testBroadcastTxSync(t, client, tx)
		if vm.mempool.Size() < limit {
			count = vm.mempool.Size()
		} else {
			count = limit
		}
		testUnconfirmedTxs(t, client, &limit, &coretypes.ResultUnconfirmedTxs{
			Count: count,
			Total: vm.mempool.Size(),
			Txs:   txs,
		})
		for i := 0; i < 3; i++ {
			_, _, tx = MakeTxKV()
			txs = append(txs, tx)
			testBroadcastTxSync(t, client, tx)
		}
		if vm.mempool.Size() < limit {
			count = vm.mempool.Size()
		} else {
			count = limit
		}
		testUnconfirmedTxs(t, client, &limit, &coretypes.ResultUnconfirmedTxs{
			Count: count,
			Total: vm.mempool.Size(),
			Txs:   txs,
		})
	})

	t.Run("NumUnconfirmedTxs", func(t *testing.T) {
		_, _, tx := MakeTxKV()
		txs := []types.Tx{tx}
		testBroadcastTxSync(t, client, tx)
		testNumUnconfirmedTxs(t, client, &coretypes.ResultUnconfirmedTxs{
			Count: vm.mempool.Size(),
			Total: vm.mempool.Size(),
		})
		for i := 0; i < 3; i++ {
			_, _, tx = MakeTxKV()
			txs = append(txs, tx)
			testBroadcastTxSync(t, client, tx)
		}
		testNumUnconfirmedTxs(t, client, &coretypes.ResultUnconfirmedTxs{
			Count: vm.mempool.Size(),
			Total: vm.mempool.Size(),
		})
	})

	t.Run("CheckTx", func(t *testing.T) {
		_, _, tx := MakeTxKV()
		testCheckTx(t, client, tx, &coretypes.ResultCheckTx{
			ResponseCheckTx: abcitypes.ResponseCheckTx{Code: kvstore.CodeTypeOK},
		})
		testCheckTx(t, client, []byte("inappropriate tx"), &coretypes.ResultCheckTx{
			ResponseCheckTx: abcitypes.ResponseCheckTx{Code: kvstore.CodeTypeInvalidTxFormat},
		})
	})
}

//TODO: implement complicated combinations
//TODO: implement rpc methods below, than implement according unit tests
//{"Header", "header", map[string]interface{}{}, new(ctypes.ResultHeader)},
//{"HeaderByHash", "header_by_hash", map[string]interface{}{}, new(ctypes.ResultHeader)},
//{"Validators", "validators", map[string]interface{}{}, new(ctypes.ResultValidators)},

func TestWSRPC(t *testing.T) {
	server, vm, client, cancel := setupWSRPCServer(t, setupWSRPC, buildAccept)
	defer server.Close()
	defer cancel()

	t.Log(vm)
	t.Log(client)

	wsc := &WSClient{
		WSClient: client,
	}
	err := wsc.Start()
	require.Nil(t, err)
	testSubscribe(t, wsc)
	//go func() {
	//	_, _, tx := MakeTxKV()
	//	testBroadcastTxCommit(t, client, vm, tx)
	//}()

	//cl3, err := client.NewWS(addr, websocketEndpoint)
	//require.Nil(t, err)
	//cl3.SetLogger(log.TestingLogger())
	//err = cl3.Start()
	//require.Nil(t, err)
	//fmt.Printf("=== testing server on %s using WS client", addr)
	//testWithWSClient(t, cl3)
	err = wsc.Stop()
	require.NoError(t, err)

	//msg := <-cl.ResponsesCh
	//if msg.Error != nil {
	//	return "", err
	//}
	//result := new(ResultEcho)
	//err = json.Unmarshal(msg.Result, result)
	//if err != nil {
	//	return "", nil
	//}
	//err := client.Start()
	//defer client.Stop()
	//require.Nil(t, err)
	//fmt.Println(vm)
	//
	//// on Subscribe
	//testSubscribe(t, client, map[string]interface{}{"query": "TestHeaderEvents"})
	//result := testBroadcastTxCommit(t, client, vm, map[string]interface{}{"tx": tx})
	//
	////// on Unsubscribe
	////err = client.Unsubscribe(context.Background(), "TestHeaderEvents",
	////	types.QueryForEvent(types.EventNewBlockHeader).String())
	////require.NoError(t, err)
	////
	////// on UnsubscribeAll
	////err = client.UnsubscribeAll(context.Background(), "TestHeaderEvents")
	////require.NoError(t, err)
	//err = client.Stop()
	//require.Nil(t, err)
}
