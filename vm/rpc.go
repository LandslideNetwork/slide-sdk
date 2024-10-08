package vm

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	tmbytes "github.com/cometbft/cometbft/libs/bytes"
	tmmath "github.com/cometbft/cometbft/libs/math"
	tmquery "github.com/cometbft/cometbft/libs/pubsub/query"
	mempl "github.com/cometbft/cometbft/mempool"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/proxy"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	rpctypes "github.com/cometbft/cometbft/rpc/jsonrpc/types"
	"github.com/cometbft/cometbft/store"
	"github.com/cometbft/cometbft/types"
	"github.com/cometbft/cometbft/version"

	"github.com/landslidenetwork/slide-sdk/jsonrpc"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

type RPC struct {
	vm *LandslideVM
}

func NewRPC(vm *LandslideVM) *RPC {
	return &RPC{vm}
}

func (rpc *RPC) Routes() map[string]*jsonrpc.RPCFunc {
	return map[string]*jsonrpc.RPCFunc{

		// info AP
		"health":              jsonrpc.NewRPCFunc(rpc.Health, ""),
		"status":              jsonrpc.NewRPCFunc(rpc.Status, ""),
		"blockchain":          jsonrpc.NewRPCFunc(rpc.BlockchainInfo, "minHeight,maxHeight", jsonrpc.Cacheable()),
		"genesis":             jsonrpc.NewRPCFunc(rpc.Genesis, "", jsonrpc.Cacheable()),
		"genesis_chunked":     jsonrpc.NewRPCFunc(rpc.GenesisChunked, "chunk", jsonrpc.Cacheable()),
		"block":               jsonrpc.NewRPCFunc(rpc.Block, "height", jsonrpc.Cacheable("height")),
		"block_by_hash":       jsonrpc.NewRPCFunc(rpc.BlockByHash, "hash", jsonrpc.Cacheable()),
		"block_results":       jsonrpc.NewRPCFunc(rpc.BlockResults, "height", jsonrpc.Cacheable("height")),
		"commit":              jsonrpc.NewRPCFunc(rpc.Commit, "height", jsonrpc.Cacheable("height")),
		"check_tx":            jsonrpc.NewRPCFunc(rpc.CheckTx, "tx"),
		"tx":                  jsonrpc.NewRPCFunc(rpc.Tx, "hash,prove", jsonrpc.Cacheable()),
		"unconfirmed_txs":     jsonrpc.NewRPCFunc(rpc.UnconfirmedTxs, "limit"),
		"num_unconfirmed_txs": jsonrpc.NewRPCFunc(rpc.NumUnconfirmedTxs, ""),
		"tx_search":           jsonrpc.NewRPCFunc(rpc.TxSearch, "query,prove,page,per_page,order_by"),
		"block_search":        jsonrpc.NewRPCFunc(rpc.BlockSearch, "query,page,per_page,order_by"),
		"validators":          jsonrpc.NewRPCFunc(rpc.Validators, "height,page,per_page", jsonrpc.Cacheable("height")),
		"consensus_params":    jsonrpc.NewRPCFunc(rpc.ConsensusParams, "height", jsonrpc.Cacheable("height")),

		// tx broadcast API
		"broadcast_tx_commit": jsonrpc.NewRPCFunc(rpc.BroadcastTxCommit, "tx"),
		"broadcast_tx_sync":   jsonrpc.NewRPCFunc(rpc.BroadcastTxSync, "tx"),
		"broadcast_tx_async":  jsonrpc.NewRPCFunc(rpc.BroadcastTxAsync, "tx"),

		// abci API
		"abci_query": jsonrpc.NewRPCFunc(rpc.ABCIQuery, "path,data,height,prove"),
		"abci_info":  jsonrpc.NewRPCFunc(rpc.ABCIInfo, "", jsonrpc.Cacheable()),
	}
}

// UnconfirmedTxs gets unconfirmed transactions (maximum ?limit entries)
// including their number.
func (rpc *RPC) UnconfirmedTxs(_ *rpctypes.Context, limitPtr *int) (*ctypes.ResultUnconfirmedTxs, error) {
	// reuse per_page validator
	limit := validatePerPage(limitPtr)
	txs := rpc.vm.mempool.ReapMaxTxs(limit)
	return &ctypes.ResultUnconfirmedTxs{
		Count:      len(txs),
		Total:      rpc.vm.mempool.Size(),
		TotalBytes: rpc.vm.mempool.SizeBytes(),
		Txs:        txs,
	}, nil
}

// NumUnconfirmedTxs gets number of unconfirmed transactions.
func (rpc *RPC) NumUnconfirmedTxs(*rpctypes.Context) (*ctypes.ResultUnconfirmedTxs, error) {
	return &ctypes.ResultUnconfirmedTxs{
		Count:      rpc.vm.mempool.Size(),
		Total:      rpc.vm.mempool.Size(),
		TotalBytes: rpc.vm.mempool.SizeBytes(),
	}, nil
}

// CheckTx checks the transaction without executing it. The transaction won't
// be added to the mempool either.
func (rpc *RPC) CheckTx(_ *rpctypes.Context, tx types.Tx) (*ctypes.ResultCheckTx, error) {
	res, err := rpc.vm.app.Mempool().CheckTx(context.Background(), &abci.RequestCheckTx{Tx: tx})
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultCheckTx{ResponseCheckTx: *res}, nil
}

// ABCIInfo returns the latest information about the application.
func (rpc *RPC) ABCIInfo(_ *rpctypes.Context) (*ctypes.ResultABCIInfo, error) {
	resInfo, err := rpc.vm.app.Query().Info(context.Background(), proxy.RequestInfo)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultABCIInfo{Response: *resInfo}, nil
}

// ABCIQuery queries the application for some information.
func (rpc *RPC) ABCIQuery(
	_ *rpctypes.Context,
	path string,
	data tmbytes.HexBytes,
	height int64,
	prove bool,
) (*ctypes.ResultABCIQuery, error) {
	resQuery, err := rpc.vm.app.Query().Query(context.Background(), &abci.RequestQuery{
		Path:   path,
		Data:   data,
		Height: height,
		Prove:  prove,
	})
	if err != nil {
		return nil, err
	}

	return &ctypes.ResultABCIQuery{Response: *resQuery}, nil
}

func (rpc *RPC) BroadcastTxCommit(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	rpc.vm.logger.Info("BroadcastTxCommit called")
	subscriber := ctx.RemoteAddr()

	if rpc.vm.eventBus.NumClients() >= rpc.vm.config.MaxSubscriptionClients {
		return nil, fmt.Errorf("max_subscription_clients %d reached", rpc.vm.config.MaxSubscriptionClients)
	} else if rpc.vm.eventBus.NumClientSubscriptions(subscriber) >= rpc.vm.config.MaxSubscriptionsPerClient {
		return nil, fmt.Errorf("max_subscriptions_per_client %d reached", rpc.vm.config.MaxSubscriptionsPerClient)
	}

	// Subscribe to tx being committed in block.
	subCtx, cancel := context.WithTimeout(context.Background(), time.Duration(rpc.vm.config.TimeoutBroadcastTxCommit)*time.Second)
	defer cancel()

	q := types.EventQueryTxFor(tx)
	deliverTxSub, err := rpc.vm.eventBus.Subscribe(subCtx, subscriber, q)
	if err != nil {
		err = fmt.Errorf("failed to subscribe to tx: %w", err)
		rpc.vm.logger.Error("Error on broadcast_tx_commit", "err", err)
		return nil, err
	}
	defer func() {
		if err := rpc.vm.eventBus.Unsubscribe(context.Background(), subscriber, q); err != nil {
			rpc.vm.logger.Error("Error unsubscribing from eventBus", "err", err)
		}
	}()

	// Broadcast tx and wait for CheckTx result
	checkTxResCh := make(chan *abci.ResponseCheckTx, 1)
	err = rpc.vm.mempool.CheckTx(tx, func(res *abci.ResponseCheckTx) {
		select {
		case <-ctx.Context().Done():
		case checkTxResCh <- res:
		}
	}, mempl.TxInfo{})
	if err != nil {
		rpc.vm.logger.Error("Error on broadcastTxCommit", "err", err)
		return nil, fmt.Errorf("error on broadcastTxCommit: %v", err)
	}

	select {
	case <-ctx.Context().Done():
		return nil, fmt.Errorf("broadcast confirmation not received: %w", ctx.Context().Err())
	case checkTxRes := <-checkTxResCh:
		if checkTxRes.Code != abci.CodeTypeOK {
			return &ctypes.ResultBroadcastTxCommit{
				CheckTx:  *checkTxRes,
				TxResult: abci.ExecTxResult{},
				Hash:     tx.Hash(),
			}, nil
		}

		// Wait for the tx to be included in a block or timeout.
		select {
		case msg := <-deliverTxSub.Out(): // The tx was included in a block.
			eventDataTx, ok := msg.Data().(types.EventDataTx)
			if !ok {
				err = fmt.Errorf("expected types.EventDataTx, got %T", msg.Data())
				rpc.vm.logger.Error("Error on broadcastTxCommit", "err", err)
				return nil, err
			}
			return &ctypes.ResultBroadcastTxCommit{
				CheckTx:  *checkTxRes,
				TxResult: eventDataTx.Result,
				Hash:     tx.Hash(),
				Height:   eventDataTx.Height,
			}, nil
		case <-deliverTxSub.Canceled():
			var reason string
			if deliverTxSub.Err() == nil {
				reason = "CometBFT exited"
			} else {
				reason = deliverTxSub.Err().Error()
			}
			err = fmt.Errorf("deliverTxSub was cancelled (reason: %s)", reason)
			rpc.vm.logger.Error("Error on broadcastTxCommit", "err", err)
			return &ctypes.ResultBroadcastTxCommit{
				CheckTx:  *checkTxRes,
				TxResult: abci.ExecTxResult{},
				Hash:     tx.Hash(),
			}, err
		case <-time.After(time.Duration(rpc.vm.config.TimeoutBroadcastTxCommit) * time.Second):
			err = errors.New("timed out waiting for tx to be included in a block")
			rpc.vm.logger.Error("Error on broadcastTxCommit", "err", err)
			return &ctypes.ResultBroadcastTxCommit{
				CheckTx:  *checkTxRes,
				TxResult: abci.ExecTxResult{},
				Hash:     tx.Hash(),
			}, err
		}
	}
}

func (rpc *RPC) BroadcastTxAsync(_ *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	rpc.vm.logger.Info("BroadcastTxAsync called")
	err := rpc.vm.mempool.CheckTx(tx, nil, mempl.TxInfo{})
	if err != nil {
		rpc.vm.logger.Error("Error on broadcastTxAsync", "err", err)
		return nil, err
	}
	return &ctypes.ResultBroadcastTx{Hash: tx.Hash()}, nil
}

// BroadcastTxSync returns with the response from CheckTx. Does not wait for
// the transaction result.
// More: https://docs.cometbft.com/v0.38.x/rpc/#/Tx/broadcast_tx_sync
func (rpc *RPC) BroadcastTxSync(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	rpc.vm.logger.Info("BroadcastTxSync called")
	resCh := make(chan *abci.ResponseCheckTx, 1)
	err := rpc.vm.mempool.CheckTx(tx, func(res *abci.ResponseCheckTx) {
		select {
		case <-ctx.Context().Done():
		case resCh <- res:
		}
	}, mempl.TxInfo{})
	if err != nil {
		rpc.vm.logger.Error("Error on BroadcastTxSync", "err", err)
		return nil, err
	}
	select {
	case <-ctx.Context().Done():
		return nil, fmt.Errorf("broadcast confirmation not received: %w", ctx.Context().Err())
	case res := <-resCh:
		return &ctypes.ResultBroadcastTx{
			Code:      res.Code,
			Data:      res.Data,
			Log:       res.Log,
			Codespace: res.Codespace,
			Hash:      tx.Hash(),
		}, nil
	}
}

// filterMinMax returns error if either min or max are negative or min > max
// if 0, use blockstore base for min, latest block height for max
// enforce limit.
func filterMinMax(base, height, min, max, limit int64) (int64, int64, error) {
	// filter negatives
	if min < 0 || max < 0 {
		return min, max, fmt.Errorf("heights must be non-negative")
	}

	// adjust for default values
	if min == 0 {
		min = 1
	}
	if max == 0 {
		max = height
	}

	// limit max to the height
	max = tmmath.MinInt64(height, max)

	// limit min to the base
	min = tmmath.MaxInt64(base, min)

	// limit min to within `limit` of max
	// so the total number of blocks returned will be `limit`
	min = tmmath.MaxInt64(min, max-limit+1)

	if min > max {
		return min, max, fmt.Errorf("min height %d can't be greater than max height %d", min, max)
	}
	return min, max, nil
}

func (rpc *RPC) BlockchainInfo(
	_ *rpctypes.Context,
	minHeight, maxHeight int64,
) (*ctypes.ResultBlockchainInfo, error) {
	// maximum 20 block metas
	const limit int64 = 20
	var err error
	minHeight, maxHeight, err = filterMinMax(
		rpc.vm.blockStore.Base(),
		rpc.vm.blockStore.Height(),
		minHeight,
		maxHeight,
		limit)
	if err != nil {
		return nil, err
	}
	rpc.vm.logger.Debug("BlockchainInfoHandler", "maxHeight", maxHeight, "minHeight", minHeight)

	var blockMetas []*types.BlockMeta
	for height := maxHeight; height >= minHeight; height-- {
		blockMeta := rpc.vm.blockStore.LoadBlockMeta(height)
		if blockMeta != nil {
			blockMetas = append(blockMetas, blockMeta)
		}
	}

	return &ctypes.ResultBlockchainInfo{
		LastHeight: rpc.vm.blockStore.Height(),
		BlockMetas: blockMetas,
	}, nil
}

func (rpc *RPC) Genesis(_ *rpctypes.Context) (*ctypes.ResultGenesis, error) {
	if len(rpc.vm.genChunks) > 1 {
		return nil, errors.New("genesis response is large, please use the genesis_chunked API instead")
	}

	return &ctypes.ResultGenesis{Genesis: rpc.vm.genesis}, nil
}

func (rpc *RPC) GenesisChunked(_ *rpctypes.Context, chunk uint) (*ctypes.ResultGenesisChunk, error) {
	if rpc.vm.genChunks == nil {
		return nil, fmt.Errorf("service configuration error, genesis chunks are not initialized")
	}

	if len(rpc.vm.genChunks) == 0 {
		return nil, fmt.Errorf("service configuration error, there are no chunks")
	}

	id := int(chunk)

	if id < 0 || id > len(rpc.vm.genChunks)-1 {
		return nil, fmt.Errorf("there are %d chunks, %d is invalid", len(rpc.vm.genChunks)-1, id)
	}

	return &ctypes.ResultGenesisChunk{
		TotalChunks: len(rpc.vm.genChunks),
		ChunkNumber: id,
		Data:        rpc.vm.genChunks[id],
	}, nil
}

func (rpc *RPC) ConsensusParams(
	_ *rpctypes.Context,
	heightPtr *int64,
) (*ctypes.ResultConsensusParams, error) {
	return &ctypes.ResultConsensusParams{
		BlockHeight:     rpc.vm.blockStore.Height(),
		ConsensusParams: *rpc.vm.genesis.ConsensusParams,
	}, nil
}

func (rpc *RPC) Health(*rpctypes.Context) (*ctypes.ResultHealth, error) {
	return &ctypes.ResultHealth{}, nil
}

// bsHeight can be either latest committed or uncommitted (+1) height.
func getHeight(bs *store.BlockStore, heightPtr *int64) (int64, error) {
	if heightPtr == nil {
		return bs.Height(), nil
	}

	height := *heightPtr
	if height <= 0 {
		return 0, fmt.Errorf("height must be greater than 0, but got %d", height)
	}
	if height > bs.Height() {
		return 0, fmt.Errorf(
			"height %d must be less than or equal to the current blockchain height %d",
			height,
			bs.Height(),
		)
	}
	bsBase := bs.Base()
	if height < bsBase {
		return 0, fmt.Errorf("height %d is not available, lowest height is %d", height, bsBase)
	}

	return height, nil
}

func (rpc *RPC) Block(_ *rpctypes.Context, heightPtr *int64) (*ctypes.ResultBlock, error) {
	height, err := getHeight(rpc.vm.blockStore, heightPtr)
	if err != nil {
		return nil, err
	}
	block := rpc.vm.blockStore.LoadBlock(height)
	blockMeta := rpc.vm.blockStore.LoadBlockMeta(height)

	if blockMeta == nil {
		rpc.vm.logger.Info("Block not found", "height", height)
		return &ctypes.ResultBlock{BlockID: types.BlockID{}, Block: block}, nil
	}

	rpc.vm.logger.Info("Block response", "height", height, "block", block, "blockMeta", blockMeta)
	return &ctypes.ResultBlock{BlockID: blockMeta.BlockID, Block: block}, nil
}

func (rpc *RPC) BlockByHash(_ *rpctypes.Context, hash []byte) (*ctypes.ResultBlock, error) {
	block := rpc.vm.blockStore.LoadBlockByHash(hash)
	if block == nil {
		return &ctypes.ResultBlock{BlockID: types.BlockID{}, Block: nil}, nil
	}
	blockMeta := rpc.vm.blockStore.LoadBlockMeta(block.Height)
	return &ctypes.ResultBlock{BlockID: blockMeta.BlockID, Block: block}, nil
}

// BlockResults retrieves the results of a block at a given height.
func (rpc *RPC) BlockResults(_ *rpctypes.Context, heightPtr *int64) (*ctypes.ResultBlockResults, error) {
	height, err := getHeight(rpc.vm.blockStore, heightPtr)
	if err != nil {
		return nil, err
	}

	results, err := rpc.vm.stateStore.LoadFinalizeBlockResponse(height)
	if err != nil {
		return nil, err
	}

	return &ctypes.ResultBlockResults{
		Height:                height,
		TxsResults:            results.TxResults,
		FinalizeBlockEvents:   results.Events,
		ValidatorUpdates:      results.ValidatorUpdates,
		ConsensusParamUpdates: results.ConsensusParamUpdates,
		AppHash:               results.AppHash,
	}, nil
}

func (rpc *RPC) Commit(_ *rpctypes.Context, heightPtr *int64) (*ctypes.ResultCommit, error) {
	height, err := getHeight(rpc.vm.blockStore, heightPtr)
	if err != nil {
		return nil, err
	}

	blockMeta := rpc.vm.blockStore.LoadBlockMeta(height)
	if blockMeta == nil {
		return nil, nil
	}

	header := blockMeta.Header

	var commit *types.Commit
	if height == rpc.vm.blockStore.Height() {
		commit = rpc.vm.blockStore.LoadSeenCommit(height)
	} else {
		commit = rpc.vm.blockStore.LoadBlockCommit(height)
	}

	return ctypes.NewResultCommit(&header, commit, !(height == rpc.vm.blockStore.Height())), nil
}

var (
	// see README
	defaultPerPage = 30
	maxPerPage     = 100
)

func validatePerPage(perPagePtr *int) int {
	if perPagePtr == nil { // no per_page parameter
		return defaultPerPage
	}

	perPage := *perPagePtr
	if perPage < 1 {
		return defaultPerPage
	} else if perPage > maxPerPage {
		return maxPerPage
	}
	return perPage
}

func validatePage(pagePtr *int, perPage, totalCount int) (int, error) {
	if perPage < 1 {
		return 1, fmt.Errorf("zero or negative perPage: %d", perPage)
	}

	if pagePtr == nil { // no page parameter
		return 1, nil
	}

	pages := ((totalCount - 1) / perPage) + 1
	if pages == 0 {
		pages = 1 // one page (even if it's empty)
	}
	page := *pagePtr
	if page <= 0 || page > pages {
		return 1, fmt.Errorf("page should be within [1, %d] range, given %d", pages, page)
	}

	return page, nil
}

func validateSkipCount(page, perPage int) int {
	skipCount := (page - 1) * perPage
	if skipCount < 0 {
		return 0
	}
	return skipCount
}

// Validators fetches and verifies validators.
func (rpc *RPC) Validators(
	_ *rpctypes.Context,
	heightPtr *int64,
	pagePtr, perPagePtr *int,
) (*ctypes.ResultValidators, error) {
	height, err := getHeight(rpc.vm.blockStore, heightPtr)
	if err != nil {
		return nil, err
	}

	validators, err := rpc.vm.stateStore.LoadValidators(height)
	if err != nil {
		return nil, err
	}

	totalCount := len(validators.Validators)
	perPage := validatePerPage(perPagePtr)
	page, err := validatePage(pagePtr, perPage, totalCount)
	if err != nil {
		return nil, err
	}

	skipCount := validateSkipCount(page, perPage)

	v := validators.Validators[skipCount : skipCount+tmmath.MinInt(perPage, totalCount-skipCount)]

	return &ctypes.ResultValidators{
		BlockHeight: height,
		Validators:  v,
		Count:       len(v),
		Total:       totalCount,
	}, nil
}

func (rpc *RPC) Tx(_ *rpctypes.Context, hash []byte, prove bool) (*ctypes.ResultTx, error) {
	rpc.vm.logger.Info("Tx called", "hash", hash)
	r, err := rpc.vm.txIndexer.Get(hash)
	if err != nil {
		rpc.vm.logger.Error("Error on Tx", "err", err)
		return nil, err
	}

	if r == nil {
		rpc.vm.logger.Error("Error on Tx", "tx not found", hash)
		return nil, fmt.Errorf("tx (%X) not found", hash)
	}

	height := r.Height
	index := r.Index

	var proof types.TxProof
	if prove {
		block := rpc.vm.blockStore.LoadBlock(height)

		if r.Index > math.MaxInt32 {
			return nil, errors.New("index value overflows int on 32-bit systems")
		}
		proof = block.Data.Txs.Proof(int(index))
	}

	return &ctypes.ResultTx{
		Hash:     hash,
		Height:   r.Height,
		Index:    r.Index,
		TxResult: r.Result,
		Tx:       r.Tx,
		Proof:    proof,
	}, nil
}

// TxSearch defines a method to search for a paginated set of transactions by
// transaction event search criteria.
func (rpc *RPC) TxSearch(
	ctx *rpctypes.Context,
	query string,
	prove bool,
	pagePtr, perPagePtr *int,
	orderBy string,
) (*ctypes.ResultTxSearch, error) {
	q, err := tmquery.New(query)
	if err != nil {
		return nil, err
	}

	results, err := rpc.vm.txIndexer.Search(ctx.Context(), q)
	if err != nil {
		return nil, err
	}

	// sort results (must be done before pagination)
	switch orderBy {
	case "desc":
		sort.Slice(results, func(i, j int) bool {
			if results[i].Height == results[j].Height {
				return results[i].Index > results[j].Index
			}
			return results[i].Height > results[j].Height
		})
	case "asc", "":
		sort.Slice(results, func(i, j int) bool {
			if results[i].Height == results[j].Height {
				return results[i].Index < results[j].Index
			}
			return results[i].Height < results[j].Height
		})
	default:
		return nil, errors.New("expected order_by to be either `asc` or `desc` or empty")
	}

	// paginate results
	totalCount := len(results)
	perPage := validatePerPage(perPagePtr)

	page, err := validatePage(pagePtr, perPage, totalCount)
	if err != nil {
		return nil, err
	}

	skipCount := validateSkipCount(page, perPage)
	pageSize := tmmath.MinInt(perPage, totalCount-skipCount)

	apiResults := make([]*ctypes.ResultTx, 0, pageSize)
	for i := skipCount; i < skipCount+pageSize; i++ {
		r := results[i]

		var proof types.TxProof
		if prove {
			block := rpc.vm.blockStore.LoadBlock(r.Height)

			if r.Index > math.MaxInt32 {
				return nil, errors.New("index value overflows int on 32-bit systems")
			}
			proof = block.Data.Txs.Proof(int(r.Index))
		}

		apiResults = append(apiResults, &ctypes.ResultTx{
			Hash:     types.Tx(r.Tx).Hash(),
			Height:   r.Height,
			Index:    r.Index,
			TxResult: r.Result,
			Tx:       r.Tx,
			Proof:    proof,
		})
	}

	return &ctypes.ResultTxSearch{Txs: apiResults, TotalCount: totalCount}, nil
}

func (rpc *RPC) BlockSearch(
	ctx *rpctypes.Context,
	query string,
	pagePtr, perPagePtr *int,
	orderBy string,
) (*ctypes.ResultBlockSearch, error) {
	q, err := tmquery.New(query)
	if err != nil {
		return nil, err
	}

	results, err := rpc.vm.blockIndexer.Search(ctx.Context(), q)
	if err != nil {
		return nil, err
	}

	// sort results (must be done before pagination)
	switch orderBy {
	case "desc", "":
		sort.Slice(results, func(i, j int) bool { return results[i] > results[j] })

	case "asc":
		sort.Slice(results, func(i, j int) bool { return results[i] < results[j] })

	default:
		return nil, errors.New("expected order_by to be either `asc` or `desc` or empty")
	}

	// paginate results
	totalCount := len(results)
	perPage := validatePerPage(perPagePtr)

	page, err := validatePage(pagePtr, perPage, totalCount)
	if err != nil {
		return nil, err
	}

	skipCount := validateSkipCount(page, perPage)
	pageSize := tmmath.MinInt(perPage, totalCount-skipCount)

	apiResults := make([]*ctypes.ResultBlock, 0, pageSize)
	for i := skipCount; i < skipCount+pageSize; i++ {
		block := rpc.vm.blockStore.LoadBlock(results[i])
		if block != nil {
			blockMeta := rpc.vm.blockStore.LoadBlockMeta(block.Height)
			if blockMeta != nil {
				apiResults = append(apiResults, &ctypes.ResultBlock{
					Block:   block,
					BlockID: blockMeta.BlockID,
				})
			}
		}
	}

	return &ctypes.ResultBlockSearch{Blocks: apiResults, TotalCount: totalCount}, nil
}

func (rpc *RPC) Status(_ *rpctypes.Context) (*ctypes.ResultStatus, error) {
	var (
		earliestBlockHeight   int64
		earliestBlockHash     tmbytes.HexBytes
		earliestAppHash       tmbytes.HexBytes
		earliestBlockTimeNano int64
	)

	if earliestBlockMeta := rpc.vm.blockStore.LoadBaseMeta(); earliestBlockMeta != nil {
		earliestBlockHeight = earliestBlockMeta.Header.Height
		earliestAppHash = earliestBlockMeta.Header.AppHash
		earliestBlockHash = earliestBlockMeta.BlockID.Hash
		earliestBlockTimeNano = earliestBlockMeta.Header.Time.UnixNano()
	}

	var (
		err                 error
		latestBlockHash     tmbytes.HexBytes
		latestAppHash       tmbytes.HexBytes
		latestBlockTimeNano int64

		latestHeight = rpc.vm.blockStore.Height()
	)

	if latestHeight != 0 {
		if latestBlockMeta := rpc.vm.blockStore.LoadBlockMeta(latestHeight); latestBlockMeta != nil {
			latestBlockHash = latestBlockMeta.BlockID.Hash
			latestAppHash = latestBlockMeta.Header.AppHash
			latestBlockTimeNano = latestBlockMeta.Header.Time.UnixNano()
		}
	}

	chainID := ids.Empty
	if rpc.vm.appOpts.ChainID != nil {
		chainID, err = ids.ToID(rpc.vm.appOpts.ChainID)
		if err != nil {
			return nil, err
		}
	}

	result := &ctypes.ResultStatus{
		NodeInfo: p2p.DefaultNodeInfo{
			ProtocolVersion: p2p.NewProtocolVersion(
				version.P2PProtocol,
				version.BlockProtocol,
				0,
			),
			DefaultNodeID: p2p.ID(fmt.Sprintf("%x", rpc.vm.appOpts.NodeID)),
			ListenAddr:    fmt.Sprintf("/ext/bc/%s/rpc", chainID),
			Network:       rpc.vm.config.NetworkName,
			Version:       version.TMCoreSemVer,
			Channels:      nil,
			Moniker:       fmt.Sprintf("%x", rpc.vm.appOpts.NodeID),
			Other:         p2p.DefaultNodeInfoOther{},
		},
		SyncInfo: ctypes.SyncInfo{
			LatestBlockHash:     latestBlockHash,
			LatestAppHash:       latestAppHash,
			LatestBlockHeight:   latestHeight,
			LatestBlockTime:     time.Unix(0, latestBlockTimeNano),
			EarliestBlockHash:   earliestBlockHash,
			EarliestAppHash:     earliestAppHash,
			EarliestBlockHeight: earliestBlockHeight,
			EarliestBlockTime:   time.Unix(0, earliestBlockTimeNano),
			CatchingUp:          false,
		},
		// TODO: use internal app validators instead
		ValidatorInfo: ctypes.ValidatorInfo{
			Address:     proposerPubKey.Address(),
			PubKey:      proposerPubKey,
			VotingPower: 0,
		},
	}

	return result, nil
}
