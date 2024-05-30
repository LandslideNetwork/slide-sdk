package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cometbft/cometbft/abci/example/kvstore"
	"github.com/cometbft/cometbft/libs/rand"
	"github.com/cometbft/cometbft/p2p"
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
	t.Logf("%v %v", expected, result.Response.Value)
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
	//initMempoolSize := vm.mempool.Size()

	result := new(coretypes.ResultBroadcastTx)
	_, err := client.Call(context.Background(), "broadcast_tx_sync", params, result)
	require.NoError(t, err)
	require.Equal(t, result.Code, abcitypes.CodeTypeOK)
	//require.Equal(t, initMempoolSize+1, vm.mempool.Size())
	//tx := types.Tx(params["tx"].([]byte))
	//require.EqualValues(t, tx.String(), result.Data.String())
	//require.EqualValues(t, tx, vm.mempool.ReapMaxTxs(-1)[0])
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

func testStatus(t *testing.T, client *client.Client, expected *coretypes.ResultStatus) {
	result := new(coretypes.ResultStatus)
	_, err := client.Call(context.Background(), "status", map[string]interface{}{}, result)
	require.NoError(t, err)
	require.Equal(t, expected.NodeInfo.Moniker, result.NodeInfo.Moniker)
	require.Equal(t, expected.SyncInfo.LatestBlockHeight, result.SyncInfo.LatestBlockHeight)
}

func testNetInfo(t *testing.T, client *client.Client, expected *coretypes.ResultNetInfo) {
	result := new(coretypes.ResultNetInfo)
	_, err := client.Call(context.Background(), "net_info", map[string]interface{}{}, result)
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

func testConsensusState(t *testing.T, client *client.Client, expected *coretypes.ResultConsensusState) {
	result := new(coretypes.ResultConsensusState)
	_, err := client.Call(context.Background(), "consensus_state", map[string]interface{}{}, result)
	require.NoError(t, err)
	//TODO: check equality
	//require.Equal(t, expected.RoundState, result.RoundState)
	//TODO: OR compare to desired values
	//assert.NotEmpty(t, cons.RoundState)
}

func testDumpConsensusState(t *testing.T, client *client.Client, expected *coretypes.ResultDumpConsensusState) {
	result := new(coretypes.ResultDumpConsensusState)
	_, err := client.Call(context.Background(), "dump_consensus_state", map[string]interface{}{}, result)
	require.NoError(t, err)
	//TODO: check equality
	//require.Equal(t, expected.RoundState, result.RoundState)
	//require.EqualValues(t, expected.Peers, result.Peers)
	//TODO: OR compare to desired values
	//assert.NotEmpty(t, cons.RoundState)
	//require.ElementsMatch(t, expected.Peers, result.Peers)
}

func testConsensusParams(t *testing.T, client *client.Client, params map[string]interface{}, expected *coretypes.ResultConsensusParams) {
	result := new(coretypes.ResultConsensusParams)
	_, err := client.Call(context.Background(), "consensus_params", params, result)
	require.NoError(t, err)
	//TODO: check equality
	require.Equal(t, expected.BlockHeight, result.BlockHeight)
	//require.Equal(t, expected.ConsensusParams.Version.App, result.ConsensusParams.Version.App)
	//require.Equal(t, expected.ConsensusParams.Hash(), result.ConsensusParams.Hash())
}

func testHealth(t *testing.T, client *client.Client) {
	result := new(coretypes.ResultHealth)
	_, err := client.Call(context.Background(), "health", map[string]interface{}{}, result)
	require.NoError(t, err)
}

func testBlockchainInfo(t *testing.T, client *client.Client, expected *coretypes.ResultBlockchainInfo) {
	result := new(coretypes.ResultBlockchainInfo)
	_, err := client.Call(context.Background(), "blockchain", map[string]interface{}{}, result)
	require.NoError(t, err)
	require.Equal(t, expected.LastHeight, result.LastHeight)
	lastMeta := result.BlockMetas[len(result.BlockMetas)-1]
	expectedLastMeta := expected.BlockMetas[len(expected.BlockMetas)-1]
	require.Equal(t, expectedLastMeta.NumTxs, lastMeta.NumTxs)
	require.Equal(t, expected.LastHeight, lastMeta.Header.AppHash)
	require.Equal(t, expectedLastMeta.BlockID, lastMeta.BlockID)
}

func testBlock(t *testing.T, client *client.Client, params map[string]interface{}, expected *coretypes.ResultBlock) *coretypes.ResultBlock {
	result := new(coretypes.ResultBlock)
	_, err := client.Call(context.Background(), "block", params, result)
	require.NoError(t, err)
	require.Equal(t, expected.Block.ChainID, result.Block.ChainID)
	require.Equal(t, expected.Block.Height, result.Block.Height)
	require.Equal(t, expected.Block.AppHash, result.Block.AppHash)
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
			if vm.state.LastBlockHeight == env.initHeight+1 {
				cancelCtx()
				testABCIQuery(t, client, map[string]interface{}{"path": "/key", "data": fmt.Sprintf("%x", env.key)}, env.value)
				//testABCIQuery(t, client, map[string]interface{}{"path": "/hash", "data": fmt.Sprintf("%x", env.hash)}, env.value)
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func checkCommittedTxResult(t *testing.T, client *client.Client, env *txRuntimeEnv) {
	testABCIQuery(t, client, map[string]interface{}{"path": "/key", "data": fmt.Sprintf("%x", env.key)}, env.value)
	//testABCIQuery(t, client, map[string]interface{}{"path": "/hash", "data": fmt.Sprintf("%x", env.hash)}, env.value)
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
			initHeight := vm.state.LastBlockHeight
			result := testBroadcastTxAsync(t, client, vm, map[string]interface{}{"tx": tx})
			checkTxResult(t, client, vm, &txRuntimeEnv{key: k, value: v, hash: result.Hash, initHeight: initHeight})
		}
	})

	t.Run("BroadcastTxSync", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			k, v, tx := MakeTxKV()
			initHeight := vm.state.LastBlockHeight
			result := testBroadcastTxSync(t, client, vm, map[string]interface{}{"tx": tx})
			checkTxResult(t, client, vm, &txRuntimeEnv{key: k, value: v, hash: result.Hash, initHeight: initHeight})
		}
		cancel()
		_, _, tx := MakeTxKV()
		initMempoolSize := vm.mempool.Size()
		result := testBroadcastTxSync(t, client, vm, map[string]interface{}{"tx": tx})
		require.Equal(t, initMempoolSize+1, vm.mempool.Size())
		require.EqualValues(t, string(tx), result.Data.String())
		require.EqualValues(t, types.Tx(tx), vm.mempool.ReapMaxTxs(-1)[0])
	})
}

func TestStatusService(t *testing.T) {
	server, vm, client, cancel := setupRPC(t, buildAccept)
	defer server.Close()
	defer vm.mempool.Flush()
	defer cancel()

	t.Run("Status", func(t *testing.T) {
		initialHeight := vm.state.LastBlockHeight
		for i := 0; i < 3; i++ {
			_, _, tx := MakeTxKV()
			result := testBroadcastTxCommit(t, client, vm, map[string]interface{}{"tx": tx})
			require.EqualValues(t, result.Height, initialHeight+int64(1)+int64(i))
			testStatus(t, client, &coretypes.ResultStatus{
				NodeInfo: p2p.DefaultNodeInfo{},
				SyncInfo: coretypes.SyncInfo{
					LatestBlockHeight: initialHeight + int64(i),
				},
				ValidatorInfo: coretypes.ValidatorInfo{},
			})
		}
	})
}

func TestNetworkService(t *testing.T) {
	server, vm, client, cancel := setupRPC(t, buildAccept)
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

	t.Run("ConsensusState", func(t *testing.T) {
		testConsensusState(t, client, &coretypes.ResultConsensusState{
			RoundState: json.RawMessage{},
		})
	})

	t.Run("ConsensusParams", func(t *testing.T) {
		initialHeight := vm.state.LastBlockHeight
		for i := 0; i < 3; i++ {
			_, _, tx := MakeTxKV()
			result := testBroadcastTxCommit(t, client, vm, map[string]interface{}{"tx": tx})
			require.EqualValues(t, result.Height, initialHeight+int64(1)+int64(i))
			testConsensusParams(t, client, map[string]interface{}{"height": vm.state.LastBlockHeight}, &coretypes.ResultConsensusParams{
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
	server, vm, client, cancel := setupRPC(t, buildAccept)
	defer server.Close()
	defer cancel()

	//txReply, err := service.BroadcastTxSync(&rpctypes.Context{}, []byte{0x00})
	//assert.NoError(t, err)
	//assert.Equal(t, atypes.CodeTypeOK, txReply.Code)
	//
	//blk, err := vm.BuildBlock(context.Background())
	//assert.NoError(t, err)
	//assert.NotNil(t, blk)
	//assert.NoError(t, blk.Accept(context.Background()))

	t.Run("Genesis", func(t *testing.T) {
		//reply, err := service.Genesis(&rpctypes.Context{})
		//assert.NoError(t, err)
		//assert.Equal(t, vm.genesis, reply.Genesis)
	})

	t.Run("GenesisChunked", func(t *testing.T) {
		//first, err := service.GenesisChunked(&rpctypes.Context{}, 0)
		//require.NoError(t, err)
		//
		//decoded := make([]string, 0, first.TotalChunks)
		//for i := 0; i < first.TotalChunks; i++ {
		//	chunk, err := service.GenesisChunked(&rpctypes.Context{}, uint(i))
		//	require.NoError(t, err)
		//	data, err := base64.StdEncoding.DecodeString(chunk.Data)
		//	require.NoError(t, err)
		//	decoded = append(decoded, string(data))
		//
		//}
		//doc := []byte(strings.Join(decoded, ""))
		//
		//var out types.GenesisDoc
		//require.NoError(t, tmjson.Unmarshal(doc, &out), "first: %+v, doc: %s", first, string(doc))
	})

	t.Run("BlockchainInfo", func(t *testing.T) {
		initialHeight := vm.state.LastBlockHeight
		blkMetas := make([]*types.BlockMeta, 0)
		for i := int64(0); i < initialHeight; i++ {
			blk := testBlock(t, client, map[string]interface{}{"height": vm.state.LastBlockHeight}, &coretypes.ResultBlock{
				Block: &types.Block{
					Header: types.Header{
						ChainID: vm.state.ChainID,
						Height:  i,
						AppHash: vm.state.AppHash,
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
		testBlockchainInfo(t, client, &coretypes.ResultBlockchainInfo{
			LastHeight: initialHeight,
			BlockMetas: blkMetas,
		})
		_, _, tx := MakeTxKV()
		testBroadcastTxCommit(t, client, vm, map[string]interface{}{"tx": tx})
		blk := testBlock(t, client, map[string]interface{}{"height": vm.state.LastBlockHeight}, &coretypes.ResultBlock{
			Block: &types.Block{
				Header: types.Header{
					ChainID: vm.state.ChainID,
					Height:  vm.state.LastBlockHeight,
					AppHash: vm.state.AppHash,
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
		testBlockchainInfo(t, client, &coretypes.ResultBlockchainInfo{
			LastHeight: initialHeight + 1,
			BlockMetas: blkMetas,
		})
	})
}

func TestSignService(t *testing.T) {
	server, vm, client, cancel := setupRPC(t, buildAccept)
	defer server.Close()
	defer cancel()
	//_, _, tx := MakeTxKV()
	//tx2 := []byte{0x02}
	//tx3 := []byte{0x03}
	//vm, service, msgs := mustNewKVTestVm(t)
	//
	//blk0, err := vm.BuildBlock(context.Background())
	//assert.ErrorIs(t, err, errNoPendingTxs, "expecting error no txs")
	//assert.Nil(t, blk0)
	//
	//txReply, err := service.BroadcastTxSync(&rpctypes.Context{}, tx)
	//assert.NoError(t, err)
	//assert.Equal(t, atypes.CodeTypeOK, txReply.Code)
	//
	//// build 1st block
	//blk1, err := vm.BuildBlock(context.Background())
	//assert.NoError(t, err)
	//assert.NotNil(t, blk1)
	//assert.NoError(t, blk1.Accept(context.Background()))
	//height1 := int64(blk1.Height())

	t.Run("Block", func(t *testing.T) {
		initialHeight := vm.state.LastBlockHeight
		for i := 0; i < 3; i++ {
			_, _, tx := MakeTxKV()
			result := testBroadcastTxCommit(t, client, vm, map[string]interface{}{"tx": tx})
			require.EqualValues(t, result.Height, initialHeight+int64(1)+int64(i))
			testBlock(t, client, map[string]interface{}{"height": vm.state.LastBlockHeight}, &coretypes.ResultBlock{
				Block: &types.Block{
					Header: types.Header{
						ChainID: vm.state.ChainID,
						Height:  result.Height,
						AppHash: vm.state.AppHash,
					},
				},
			})
		}
	})

	t.Run("BlockByHash", func(t *testing.T) {
		//replyWithoutHash, err := service.BlockByHash(&rpctypes.Context{}, []byte{})
		//assert.NoError(t, err)
		//assert.Nil(t, replyWithoutHash.Block)
		//
		//hash := blk1.ID()
		//reply, err := service.BlockByHash(&rpctypes.Context{}, hash[:])
		//assert.NoError(t, err)
		//if assert.NotNil(t, reply.Block) {
		//	assert.EqualValues(t, hash[:], reply.Block.Hash().Bytes())
		//}
	})

	t.Run("BlockResults", func(t *testing.T) {
		//replyWithoutHeight, err := service.BlockResults(&rpctypes.Context{}, nil)
		//assert.NoError(t, err)
		//assert.Equal(t, height1, replyWithoutHeight.Height)
		//
		//reply, err := service.BlockResults(&rpctypes.Context{}, &height1)
		//assert.NoError(t, err)
		//if assert.NotNil(t, reply.TxsResults) {
		//	assert.Equal(t, height1, reply.Height)
		//}
	})

	t.Run("Tx", func(t *testing.T) {
		//time.Sleep(2 * time.Second)
		//
		//reply, err := service.Tx(&rpctypes.Context{}, txReply.Hash.Bytes(), false)
		//assert.NoError(t, err)
		//assert.EqualValues(t, txReply.Hash, reply.Hash)
		//assert.EqualValues(t, tx, reply.Tx)
	})

	t.Run("TxSearch", func(t *testing.T) {
		//txReply2, err := service.BroadcastTxAsync(&rpctypes.Context{}, tx2)
		//assert.NoError(t, err)
		//assert.Equal(t, atypes.CodeTypeOK, txReply2.Code)
		//
		//blk2, err := vm.BuildBlock(context.Background())
		//require.NoError(t, err)
		//assert.NotNil(t, blk2)
		//assert.NoError(t, blk2.Accept(context.Background()))
		//
		//time.Sleep(time.Second)
		//
		//reply, err := service.TxSearch(&rpctypes.Context{}, fmt.Sprintf("tx.hash='%s'", txReply2.Hash), false, nil, nil, "asc")
		//assert.NoError(t, err)
		//assert.True(t, len(reply.Txs) > 0)
		//
		//// TODO: need to fix
		//// reply2, err := service.TxSearch(&rpctypes.Context{}, fmt.Sprintf("tx.height=%d", blk2.Height()), false, nil, nil, "desc")
		//// assert.NoError(t, err)
		//// assert.True(t, len(reply2.Txs) > 0)
	})

	//TODO: Check logic of test
	t.Run("Commit", func(t *testing.T) {
		//txReply, err := service.BroadcastTxAsync(&rpctypes.Context{}, tx3)
		//require.NoError(t, err)
		//assert.Equal(t, atypes.CodeTypeOK, txReply.Code)
		//
		//assert, require := assert.New(t), require.New(t)
		//
		//// get an offset of height to avoid racing and guessing
		//s, err := service.Status(&rpctypes.Context{})
		//require.NoError(err)
		//// sh is start height or status height
		//sh := s.SyncInfo.LatestBlockHeight
		//
		//// look for the future
		//h := sh + 20
		//_, err = service.Block(&rpctypes.Context{}, &h)
		//require.Error(err) // no block yet
		//
		//// write something
		//k, v, tx := MakeTxKV()
		//bres, err := broadcastTx(t, vm, msgs, tx)
		//require.NoError(err)
		//require.True(bres.DeliverTx.IsOK())
		//time.Sleep(2 * time.Second)
		//
		//txh := bres.Height
		//apph := txh
		//
		//// wait before querying
		//err = WaitForHeight(service, apph, nil)
		//require.NoError(err)
		//
		//qres, err := service.ABCIQuery(&rpctypes.Context{}, "/key", k, 0, false)
		//require.NoError(err)
		//if assert.True(qres.Response.IsOK()) {
		//	assert.Equal(k, qres.Response.Key)
		//	assert.EqualValues(v, qres.Response.Value)
		//}
		//
		//// make sure we can lookup the tx with proof
		//ptx, err := service.Tx(&rpctypes.Context{}, bres.Hash, true)
		//require.NoError(err)
		//assert.EqualValues(txh, ptx.Height)
		//assert.EqualValues(tx, ptx.Tx)
		//
		//// and we can even check the block is added
		//block, err := service.Block(&rpctypes.Context{}, &apph)
		//require.NoError(err)
		//appHash := block.Block.Header.AppHash
		//assert.True(len(appHash) > 0)
		//assert.EqualValues(apph, block.Block.Header.Height)
		//
		//blockByHash, err := service.BlockByHash(&rpctypes.Context{}, block.BlockID.Hash)
		//require.NoError(err)
		//require.Equal(block, blockByHash)
		//
		//// now check the results
		//blockResults, err := service.BlockResults(&rpctypes.Context{}, &txh)
		//require.Nil(err, "%+v", err)
		//assert.Equal(txh, blockResults.Height)
		//if assert.Equal(2, len(blockResults.TxsResults)) {
		//	// check success code
		//	assert.EqualValues(0, blockResults.TxsResults[0].Code)
		//}
		//
		//// check blockchain info, now that we know there is info
		//info, err := service.BlockchainInfo(&rpctypes.Context{}, apph, apph)
		//require.NoError(err)
		//assert.True(info.LastHeight >= apph)
		//if assert.Equal(1, len(info.BlockMetas)) {
		//	lastMeta := info.BlockMetas[0]
		//	assert.EqualValues(apph, lastMeta.Header.Height)
		//	blockData := block.Block
		//	assert.Equal(blockData.Header.AppHash, lastMeta.Header.AppHash)
		//	assert.Equal(block.BlockID, lastMeta.BlockID)
		//}
		//
		//// and get the corresponding commit with the same apphash
		//commit, err := service.Commit(&rpctypes.Context{}, &apph)
		//require.NoError(err)
		//assert.NotNil(commit)
		//assert.Equal(appHash, commit.Header.AppHash)
		//
		//// compare the commits (note Commit(2) has commit from Block(3))
		//h = apph - 1
		//commit2, err := service.Commit(&rpctypes.Context{}, &h)
		//require.NoError(err)
		//assert.Equal(block.Block.LastCommitHash, commit2.Commit.Hash())
		//
		//// and we got a proof that works!
		//pres, err := service.ABCIQuery(&rpctypes.Context{}, "/key", k, 0, true)
		//require.NoError(err)
		//assert.True(pres.Response.IsOK())
	})

	t.Run("BlockSearch", func(t *testing.T) {
		//reply, err := service.BlockSearch(&rpctypes.Context{}, "block.height=2", nil, nil, "desc")
		//assert.NoError(t, err)
		//assert.True(t, len(reply.Blocks) > 0)
	})
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
		//+{"Health", "health", map[string]interface{}{}, new(ctypes.ResultHealth)},
		//+{"Status", "status", map[string]interface{}{}, new(ctypes.ResultStatus)},
		//?{"NetInfo", "net_info", map[string]interface{}{}, new(ctypes.ResultNetInfo)},
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
		//?{"DumpConsensusState", "dump_consensus_state", map[string]interface{}{}, new(ctypes.ResultDumpConsensusState)},
		//?{"ConsensusState", "consensus_state", map[string]interface{}{}, new(ctypes.ResultConsensusState)},
		//?{"ConsensusParams", "consensus_params", map[string]interface{}{}, new(ctypes.ResultConsensusParams)},
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
