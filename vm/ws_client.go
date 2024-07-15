package vm

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/cometbft/cometbft/libs/bytes"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cometbft/cometbft/rpc/jsonrpc/client"
	"github.com/cometbft/cometbft/types"
)

type WSClient struct {
	*client.WSClient
}

func (ws *WSClient) Status(ctx context.Context) (*ctypes.ResultStatus, error) {
	params := make(map[string]interface{})
	err := ws.Call(context.Background(), "status", params)
	if err != nil {
		return nil, err
	}

	msg := <-ws.ResponsesCh
	if msg.Error != nil {
		return nil, err
	}
	result := new(ctypes.ResultStatus)
	err = json.Unmarshal(msg.Result, result)
	if err != nil {
		return nil, nil
	}
	return result, nil
}

func (ws *WSClient) ABCIInfo(ctx context.Context) (*ctypes.ResultABCIInfo, error) {
	params := make(map[string]interface{})
	err := ws.Call(context.Background(), "abci_info", params)
	if err != nil {
		return nil, err
	}

	msg := <-ws.ResponsesCh
	if msg.Error != nil {
		return nil, err
	}
	result := new(ctypes.ResultABCIInfo)
	err = json.Unmarshal(msg.Result, result)
	if err != nil {
		return nil, nil
	}
	return result, nil
}

func (ws *WSClient) ABCIQuery(
	ctx context.Context,
	path string,
	data bytes.HexBytes,
) (*ctypes.ResultABCIQuery, error) {
	return ws.ABCIQueryWithOptions(ctx, path, data, rpcclient.DefaultABCIQueryOptions)
}

func (ws *WSClient) ABCIQueryWithOptions(
	ctx context.Context,
	path string,
	data bytes.HexBytes,
	opts rpcclient.ABCIQueryOptions,
) (*ctypes.ResultABCIQuery, error) {
	params := map[string]interface{}{"path": path, "data": data, "height": opts.Height, "prove": opts.Prove}
	err := ws.Call(context.Background(), "abci_query", params)
	if err != nil {
		return nil, err
	}

	msg := <-ws.ResponsesCh
	if msg.Error != nil {
		return nil, err
	}
	result := new(ctypes.ResultABCIQuery)
	err = json.Unmarshal(msg.Result, result)
	if err != nil {
		return nil, nil
	}
	return result, nil
}

func (ws *WSClient) BroadcastTxCommit(
	ctx context.Context,
	tx types.Tx,
) (*ctypes.ResultBroadcastTxCommit, error) {
	params := map[string]interface{}{"tx": tx}
	err := ws.Call(ctx, "broadcast_tx_commit", params)
	if err != nil {
		return nil, err
	}

	msg := <-ws.ResponsesCh
	if msg.Error != nil {
		return nil, err
	}
	result := new(ctypes.ResultBroadcastTxCommit)
	err = json.Unmarshal(msg.Result, result)
	if err != nil {
		return nil, nil
	}
	return result, nil
}

func (ws *WSClient) BroadcastTxAsync(
	ctx context.Context,
	tx types.Tx,
) (*ctypes.ResultBroadcastTx, error) {
	return ws.broadcastTX(ctx, "broadcast_tx_async", tx)
}

func (ws *WSClient) BroadcastTxSync(
	ctx context.Context,
	tx types.Tx,
) (*ctypes.ResultBroadcastTx, error) {
	return ws.broadcastTX(ctx, "broadcast_tx_sync", tx)
}

func (ws *WSClient) broadcastTX(
	ctx context.Context,
	route string,
	tx types.Tx,
) (*ctypes.ResultBroadcastTx, error) {
	params := map[string]interface{}{"tx": tx}
	err := ws.Call(ctx, route, params)
	if err != nil {
		return nil, err
	}

	result := new(ctypes.ResultBroadcastTx)

	return result, nil
}

func (ws *WSClient) UnconfirmedTxs(
	ctx context.Context,
	limit *int,
) (*ctypes.ResultUnconfirmedTxs, error) {
	params := make(map[string]interface{})
	if limit != nil {
		params["limit"] = limit
	}
	err := ws.Call(ctx, "unconfirmed_txs", params)
	if err != nil {
		return nil, err
	}

	result := new(ctypes.ResultUnconfirmedTxs)

	return result, nil
}

func (ws *WSClient) NumUnconfirmedTxs(ctx context.Context) (*ctypes.ResultUnconfirmedTxs, error) {
	params := make(map[string]interface{})
	err := ws.Call(ctx, "num_unconfirmed_txs", params)
	if err != nil {
		return nil, err
	}

	result := new(ctypes.ResultUnconfirmedTxs)

	return result, nil
}

func (ws *WSClient) CheckTx(ctx context.Context, tx types.Tx) (*ctypes.ResultCheckTx, error) {
	params := map[string]interface{}{"tx": tx}
	err := ws.Call(ctx, "check_tx", params)
	if err != nil {
		return nil, err
	}

	result := new(ctypes.ResultCheckTx)
	return result, nil
}

func (ws *WSClient) NetInfo(ctx context.Context) (*ctypes.ResultNetInfo, error) {
	params := make(map[string]interface{})
	err := ws.Call(ctx, "net_info", params)
	if err != nil {
		return nil, err
	}

	result := new(ctypes.ResultNetInfo)

	return result, nil
}

func (ws *WSClient) DumpConsensusState(ctx context.Context) (*ctypes.ResultDumpConsensusState, error) {
	params := make(map[string]interface{})
	err := ws.Call(ctx, "dump_consensus_state", params)
	if err != nil {
		return nil, err
	}

	result := new(ctypes.ResultDumpConsensusState)

	return result, nil
}

func (ws *WSClient) ConsensusState(ctx context.Context) (*ctypes.ResultConsensusState, error) {
	params := make(map[string]interface{})
	err := ws.Call(ctx, "consensus_state", params)
	if err != nil {
		return nil, err
	}

	result := new(ctypes.ResultConsensusState)

	return result, nil
}

func (ws *WSClient) ConsensusParams(
	ctx context.Context,
	height *int64,
) (*ctypes.ResultConsensusParams, error) {
	params := make(map[string]interface{})
	if height != nil {
		params["height"] = height
	}
	err := ws.Call(ctx, "consensus_params", params)
	if err != nil {
		return nil, err
	}

	result := new(ctypes.ResultConsensusParams)
	return result, nil
}

func (ws *WSClient) Health(ctx context.Context) (*ctypes.ResultHealth, error) {
	params := make(map[string]interface{})

	err := ws.Call(ctx, "health", params)
	if err != nil {
		return nil, err
	}

	result := new(ctypes.ResultHealth)

	return result, nil
}

func (ws *WSClient) BlockchainInfo(
	ctx context.Context,
	minHeight,
	maxHeight int64,
) (*ctypes.ResultBlockchainInfo, error) {
	params := map[string]interface{}{"minHeight": minHeight, "maxHeight": maxHeight}
	err := ws.Call(ctx, "blockchain", params)
	if err != nil {
		return nil, err
	}

	result := new(ctypes.ResultBlockchainInfo)

	return result, nil
}

func (ws *WSClient) Genesis(ctx context.Context) (*ctypes.ResultGenesis, error) {
	params := make(map[string]interface{})
	err := ws.Call(ctx, "genesis", params)
	if err != nil {
		return nil, err
	}

	result := new(ctypes.ResultGenesis)

	return result, nil
}

func (ws *WSClient) GenesisChunked(ctx context.Context, id uint) (*ctypes.ResultGenesisChunk, error) {
	params := map[string]interface{}{"chunk": id}
	err := ws.Call(ctx, "genesis_chunked", params)
	if err != nil {
		return nil, err
	}

	result := new(ctypes.ResultGenesisChunk)

	return result, nil
}

func (ws *WSClient) Block(ctx context.Context, height *int64) (*ctypes.ResultBlock, error) {
	params := make(map[string]interface{})
	if height != nil {
		params["height"] = height
	}
	err := ws.Call(ctx, "block", params)
	if err != nil {
		return nil, err
	}

	result := new(ctypes.ResultBlock)

	return result, nil
}

func (ws *WSClient) BlockByHash(ctx context.Context, hash []byte) (*ctypes.ResultBlock, error) {
	params := map[string]interface{}{
		"hash": hash,
	}
	err := ws.Call(ctx, "block_by_hash", params)
	if err != nil {
		return nil, err
	}
	result := new(ctypes.ResultBlock)
	return result, nil
}

func (ws *WSClient) BlockResults(
	ctx context.Context,
	height *int64,
) (*ctypes.ResultBlockResults, error) {
	params := make(map[string]interface{})
	if height != nil {
		params["height"] = height
	}
	err := ws.Call(ctx, "block_results", params)
	if err != nil {
		return nil, err
	}
	result := new(ctypes.ResultBlockResults)
	return result, nil
}

func (ws *WSClient) Header(ctx context.Context, height *int64) (*ctypes.ResultHeader, error) {
	params := make(map[string]interface{})
	if height != nil {
		params["height"] = height
	}
	err := ws.Call(ctx, "header", params)
	if err != nil {
		return nil, err
	}
	result := new(ctypes.ResultHeader)
	return result, nil
}

func (ws *WSClient) HeaderByHash(ctx context.Context, hash bytes.HexBytes) (*ctypes.ResultHeader, error) {
	params := map[string]interface{}{
		"hash": hash,
	}
	err := ws.Call(ctx, "header_by_hash", params)
	if err != nil {
		return nil, err
	}
	result := new(ctypes.ResultHeader)
	return result, nil
}

func (ws *WSClient) Commit(ctx context.Context, height *int64) (*ctypes.ResultCommit, error) {
	params := make(map[string]interface{})
	if height != nil {
		params["height"] = height
	}
	err := ws.Call(ctx, "commit", params)
	if err != nil {
		return nil, err
	}
	result := new(ctypes.ResultCommit)
	return result, nil
}

func (ws *WSClient) Tx(ctx context.Context, hash []byte, prove bool) (*ctypes.ResultTx, error) {
	params := map[string]interface{}{
		"hash":  hash,
		"prove": prove,
	}
	err := ws.Call(ctx, "tx", params)
	if err != nil {
		return nil, err
	}
	result := new(ctypes.ResultTx)
	return result, nil
}

func (ws *WSClient) TxSearch(
	ctx context.Context,
	query string,
	prove bool,
	page,
	perPage *int,
	orderBy string,
) (*ctypes.ResultTxSearch, error) {
	params := map[string]interface{}{
		"query":    query,
		"prove":    prove,
		"order_by": orderBy,
	}

	if page != nil {
		params["page"] = page
	}
	if perPage != nil {
		params["per_page"] = perPage
	}

	err := ws.Call(ctx, "tx_search", params)
	if err != nil {
		return nil, err
	}

	result := new(ctypes.ResultTxSearch)

	return result, nil
}

func (ws *WSClient) BlockSearch(
	ctx context.Context,
	query string,
	page, perPage *int,
	orderBy string,
) (*ctypes.ResultBlockSearch, error) {
	params := map[string]interface{}{
		"query":    query,
		"order_by": orderBy,
	}

	if page != nil {
		params["page"] = page
	}
	if perPage != nil {
		params["per_page"] = perPage
	}

	err := ws.Call(ctx, "block_search", params)
	if err != nil {
		return nil, err
	}

	result := new(ctypes.ResultBlockSearch)

	return result, nil
}

func (ws *WSClient) Validators(
	ctx context.Context,
	height *int64,
	page,
	perPage *int,
) (*ctypes.ResultValidators, error) {
	params := make(map[string]interface{})
	if page != nil {
		params["page"] = page
	}
	if perPage != nil {
		params["per_page"] = perPage
	}
	if height != nil {
		params["height"] = height
	}
	err := ws.Call(ctx, "validators", params)
	if err != nil {
		return nil, err
	}

	result := new(ctypes.ResultValidators)

	return result, nil
}

func (ws *WSClient) BroadcastEvidence(
	ctx context.Context,
	ev types.Evidence,
) (*ctypes.ResultBroadcastEvidence, error) {
	params := map[string]interface{}{"evidence": ev}
	err := ws.Call(ctx, "broadcast_evidence", params)
	if err != nil {
		return nil, err
	}

	result := new(ctypes.ResultBroadcastEvidence)

	return result, nil
}

//-----------------------------------------------------------------------------
// WSEvents

var errNotRunning = errors.New("client is not running. Use .Start() method to start")

// Subscribe to a query. Note the server must have a "subscribe" route
// defined.
func (ws *WSClient) Subscribe(ctx context.Context, subscriber string, query string, outCapacity ...int) (out <-chan ctypes.ResultEvent, err error) {
	params := map[string]interface{}{"query": query}
	err = ws.Call(ctx, "subscribe", params)
	if err != nil {
		return nil, err
	}
	return ws., nil
}

// Unsubscribe from a query. Note the server must have a "unsubscribe" route
// defined.
func (ws *WSClient) Unsubscribe(ctx context.Context, subscriber string, query string) error {
	params := map[string]interface{}{"query": query}
	return ws.Call(ctx, "unsubscribe", params)
}

// UnsubscribeAll from all. Note the server must have a "unsubscribe_all" route
// defined.
func (ws *WSClient) UnsubscribeAll(ctx context.Context, _ string) error {
	params := map[string]interface{}{}
	return ws.Call(ctx, "unsubscribe_all", params)
}

//// WSEvents is a wrapper around WSClient, which implements EventsClient.
//type WSEvents struct {
//	service.BaseService
//	remote   string
//	endpoint string
//	ws       *jsonrpcclient.WSClient
//
//	mtx           cmtsync.RWMutex
//	subscriptions map[string]chan ctypes.ResultEvent // query -> chan
//}
//
//func newWSEvents(remote, endpoint string) (*WSEvents, error) {
//	w := &WSEvents{
//		endpoint:      endpoint,
//		remote:        remote,
//		subscriptions: make(map[string]chan ctypes.ResultEvent),
//	}
//	w.BaseService = *service.NewBaseService(nil, "WSEvents", w)
//
//	var err error
//	w.ws, err = jsonrpcclient.NewWS(w.remote, w.endpoint, jsonrpcclient.OnReconnect(func() {
//		// resubscribe immediately
//		w.redoSubscriptionsAfter(0 * time.Second)
//	}))
//	if err != nil {
//		return nil, err
//	}
//	w.ws.SetLogger(w.Logger)
//
//	return w, nil
//}
//
//// OnStart implements service.Service by starting WSClient and event loop.
//func (w *WSEvents) OnStart() error {
//	if err := w.ws.Start(); err != nil {
//		return err
//	}
//
//	go w.eventListener()
//
//	return nil
//}
//
//// OnStop implements service.Service by stopping WSClient.
//func (w *WSEvents) OnStop() {
//	if err := w.ws.Stop(); err != nil {
//		w.Logger.Error("Can't stop ws client", "err", err)
//	}
//}

//// Subscribe implements EventsClient by using WSClient to subscribe given
//// subscriber to query. By default, returns a channel with cap=1. Error is
//// returned if it fails to subscribe.
////
//// Channel is never closed to prevent clients from seeing an erroneous event.
////
//// It returns an error if WSEvents is not running.
//func (ws *WSClient) Subscribe(ctx context.Context, _, query string,
//	outCapacity ...int,
//) (out <-chan ctypes.ResultEvent, err error) {
//	if !w.IsRunning() {
//		return nil, errNotRunning
//	}
//
//	if err := w.ws.Subscribe(ctx, query); err != nil {
//		return nil, err
//	}
//
//	outCap := 1
//	if len(outCapacity) > 0 {
//		outCap = outCapacity[0]
//	}
//
//	outc := make(chan ctypes.ResultEvent, outCap)
//	w.mtx.Lock()
//	// subscriber param is ignored because CometBFT will override it with
//	// remote IP anyway.
//	w.subscriptions[query] = outc
//	w.mtx.Unlock()
//
//	return outc, nil
//}
//
//// Unsubscribe implements EventsClient by using WSClient to unsubscribe given
//// subscriber from query.
////
//// It returns an error if WSEvents is not running.
//func (ws *WSClient) Unsubscribe(ctx context.Context, _, query string) error {
//	if !w.IsRunning() {
//		return errNotRunning
//	}
//
//	if err := w.ws.Unsubscribe(ctx, query); err != nil {
//		return err
//	}
//
//	w.mtx.Lock()
//	_, ok := w.subscriptions[query]
//	if ok {
//		delete(w.subscriptions, query)
//	}
//	w.mtx.Unlock()
//
//	return nil
//}
//
//// UnsubscribeAll implements EventsClient by using WSClient to unsubscribe
//// given subscriber from all the queries.
////
//// It returns an error if WSEvents is not running.
//func (ws *WSClient) UnsubscribeAll(ctx context.Context, _ string) error {
//	if !ws.IsRunning() {
//		return errNotRunning
//	}
//
//	if err := ws.UnsubscribeAll(ctx); err != nil {
//		return err
//	}
//
//	ws.mtx.Lock()
//	w.subscriptions = make(map[string]chan ctypes.ResultEvent)
//	w.mtx.Unlock()
//
//	return nil
//}
