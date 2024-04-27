package vm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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

	"github.com/consideritdone/landslidevm/jsonrpc"
)

type RPC struct {
	vm *LandslideVM
}

func NewRPC(vm *LandslideVM) *RPC {
	return &RPC{vm}
}

func (rpc *RPC) Routes() map[string]*jsonrpc.RPCFunc {
	return map[string]*jsonrpc.RPCFunc{
		"status": jsonrpc.NewRPCFunc(rpc.Status, ""),

		// abci API
		"abci_query": jsonrpc.NewRPCFunc(rpc.ABCIQuery, "path,data,height,prove"),
		"abci_info":  jsonrpc.NewRPCFunc(rpc.ABCIInfo, "", jsonrpc.Cacheable()),
	}
}

func (rpc *RPC) ABCIInfo(ctx context.Context) (*ctypes.ResultABCIInfo, error) {
	resInfo, err := rpc.vm.app.Query().Info(context.Background(), proxy.RequestInfo)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultABCIInfo{Response: *resInfo}, nil
}

type ABCIQueryArgs struct {
	Path string           `json:"path"`
	Data tmbytes.HexBytes `json:"data"`
}

type ABCIQueryWithOptionsArgs struct {
	Path string           `json:"path"`
	Data tmbytes.HexBytes `json:"data"`
	Opts ABCIQueryOptions `json:"opts"`
}

type ABCIQueryOptions struct {
	Height int64 `json:"height"`
	Prove  bool  `json:"prove"`
}

var (
	DefaultABCIQueryOptions = ABCIQueryOptions{Height: 0, Prove: false}
)

func (rpc *RPC) ABCIQuery(req *http.Request, args *ABCIQueryArgs, reply *ctypes.ResultABCIQuery) error {
	return rpc.ABCIQueryWithOptions(req, &ABCIQueryWithOptionsArgs{args.Path, args.Data, DefaultABCIQueryOptions}, reply)
}

func (rpc *RPC) ABCIQueryWithOptions(
	_ *http.Request,
	args *ABCIQueryWithOptionsArgs,
	reply *ctypes.ResultABCIQuery,
) error {
	resQuery, err := rpc.vm.app.Query().Query(context.Background(), &abci.RequestQuery{
		Path:   args.Path,
		Data:   args.Data,
		Height: args.Opts.Height,
		Prove:  args.Opts.Prove,
	})
	if err != nil {
		return err
	}
	reply.Response = *resQuery
	return nil
}

func (rpc *RPC) BroadcastTxAsync(ctx context.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	err := rpc.vm.mempool.CheckTx(tx, nil, mempl.TxInfo{})
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultBroadcastTx{Hash: tx.Hash()}, nil
}

func (rpc *RPC) BroadcastTxSync(ctx context.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	resCh := make(chan *abci.ResponseCheckTx, 1)
	err := rpc.vm.mempool.CheckTx(tx, func(res *abci.ResponseCheckTx) {
		resCh <- res
	}, mempl.TxInfo{})
	if err != nil {
		return nil, err
	}
	res := <-resCh
	return &ctypes.ResultBroadcastTx{
		Code:      res.GetCode(),
		Data:      res.GetData(),
		Log:       res.GetLog(),
		Codespace: res.GetCodespace(),
		Hash:      tx.Hash(),
	}, nil
}

type BlockchainInfoArgs struct {
	MinHeight int64 `json:"minHeight"`
	MaxHeight int64 `json:"maxHeight"`
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
	_ *http.Request,
	args *BlockchainInfoArgs,
	reply *ctypes.ResultBlockchainInfo,
) error {
	// maximum 20 block metas
	const limit int64 = 20
	var err error
	args.MinHeight, args.MaxHeight, err = filterMinMax(
		rpc.vm.blockStore.Base(),
		rpc.vm.blockStore.Height(),
		args.MinHeight,
		args.MaxHeight,
		limit)
	if err != nil {
		return err
	}
	rpc.vm.logger.Debug("BlockchainInfoHandler", "maxHeight", args.MaxHeight, "minHeight", args.MinHeight)

	var blockMetas []*types.BlockMeta
	for height := args.MaxHeight; height >= args.MinHeight; height-- {
		blockMeta := rpc.vm.blockStore.LoadBlockMeta(height)
		blockMetas = append(blockMetas, blockMeta)
	}

	reply.LastHeight = rpc.vm.blockStore.Height()
	reply.BlockMetas = blockMetas
	return nil
}

func (rpc *RPC) Genesis(_ *http.Request, _ *struct{}, reply *ctypes.ResultGenesis) error {
	if len(rpc.vm.genChunks) > 1 {
		return errors.New("genesis response is large, please use the genesis_chunked API instead")
	}

	reply.Genesis = rpc.vm.genesis
	return nil
}

type GenesisChunkedArgs struct {
	Chunk uint `json:"chunk"`
}

func (rpc *RPC) GenesisChunked(_ *http.Request, args *GenesisChunkedArgs, reply *ctypes.ResultGenesisChunk) error {
	if rpc.vm.genChunks == nil {
		return fmt.Errorf("service configuration error, genesis chunks are not initialized")
	}

	if len(rpc.vm.genChunks) == 0 {
		return fmt.Errorf("service configuration error, there are no chunks")
	}

	id := int(args.Chunk)

	if id > len(rpc.vm.genChunks)-1 {
		return fmt.Errorf("there are %d chunks, %d is invalid", len(rpc.vm.genChunks)-1, id)
	}

	reply.TotalChunks = len(rpc.vm.genChunks)
	reply.ChunkNumber = id
	reply.Data = rpc.vm.genChunks[id]
	return nil
}

// ToDo: no peers, because it's vm
func (rpc *RPC) NetInfo(_ *http.Request, _ *struct{}, reply *ctypes.ResultNetInfo) error {
	return nil
}

// ToDo: we doesn't have consensusState
func (rpc *RPC) DumpConsensusState(_ *http.Request, _ *struct{}, reply *ctypes.ResultDumpConsensusState) error {
	return nil
}

// ToDo: we doesn't have consensusState
func (rpc *RPC) ConsensusState(_ *http.Request, _ *struct{}, reply *ctypes.ResultConsensusState) error {
	return nil
}

type ConsensusParamsArgs struct {
	Height *int64 `json:"height"`
}

func (rpc *RPC) ConsensusParams(_ *http.Request, args *ConsensusParamsArgs, reply *ctypes.ResultConsensusParams) error {
	reply.BlockHeight = rpc.vm.blockStore.Height()
	reply.ConsensusParams = *rpc.vm.genesis.ConsensusParams
	return nil
}

func (rpc *RPC) Health(_ *http.Request, _ *struct{}, reply *ctypes.ResultHealth) error {
	*reply = ctypes.ResultHealth{}
	return nil
}

type BlockHeightArgs struct {
	Height *int64 `json:"height"`
}

// bsHeight can be either latest committed or uncommitted (+1) height.
func getHeight(bs *store.BlockStore, heightPtr *int64) (int64, error) {
	bsHeight := bs.Height()
	bsBase := bs.Base()
	if heightPtr != nil {
		height := *heightPtr
		if height <= 0 {
			return 0, fmt.Errorf("height must be greater than 0, but got %d", height)
		}
		if height > bsHeight {
			return 0, fmt.Errorf("height %d must be less than or equal to the current blockchain height %d", height, bsHeight)
		}
		if height < bsBase {
			return 0, fmt.Errorf("height %d is not available, lowest height is %d", height, bsBase)
		}
		return height, nil
	}
	return bsHeight, nil
}

func (rpc *RPC) Block(_ *http.Request, args *BlockHeightArgs, reply *ctypes.ResultBlock) error {
	height, err := getHeight(rpc.vm.blockStore, args.Height)
	if err != nil {
		return err
	}
	block := rpc.vm.blockStore.LoadBlock(height)
	blockMeta := rpc.vm.blockStore.LoadBlockMeta(height)

	if blockMeta != nil {
		reply.BlockID = blockMeta.BlockID
	}
	reply.Block = block
	return nil
}

type BlockHashArgs struct {
	Hash []byte `json:"hash"`
}

func (rpc *RPC) BlockByHash(_ *http.Request, args *BlockHashArgs, reply *ctypes.ResultBlock) error {
	block := rpc.vm.blockStore.LoadBlockByHash(args.Hash)
	if block == nil {
		reply.BlockID = types.BlockID{}
		reply.Block = nil
		return nil
	}
	blockMeta := rpc.vm.blockStore.LoadBlockMeta(block.Height)
	reply.BlockID = blockMeta.BlockID
	reply.Block = block
	return nil
}

func (rpc *RPC) BlockResults(_ *http.Request, args *BlockHeightArgs, reply *ctypes.ResultBlockResults) error {
	// height, err := getHeight(rpc.vm.blockStore, args.Height)
	// if err != nil {
	// 	return err
	// }

	// TODO make IBC reply it realised in landslidevm, but not realised in comet bft
	// results, err := rpc.vm.stateStore.
	// if err != nil {
	// 	return err
	// }

	// reply.Height = height
	// reply.TxsResults = results.DeliverTxs
	// reply.BeginBlockEvents = results.BeginBlock.Events
	// reply.EndBlockEvents = results.EndBlock.Events
	// reply.ValidatorUpdates = results.EndBlock.ValidatorUpdates
	// reply.ConsensusParamUpdates = results.EndBlock.ConsensusParamUpdates
	return nil
}

type (
	CommitArgs struct {
		Height *int64 `json:"height"`
	}

	ValidatorsArgs struct {
		Height  *int64 `json:"height"`
		Page    *int   `json:"page"`
		PerPage *int   `json:"perPage"`
	}

	TxArgs struct {
		Hash  []byte `json:"hash"`
		Prove bool   `json:"prove"`
	}

	TxSearchArgs struct {
		Query   string `json:"query"`
		Prove   bool   `json:"prove"`
		Page    *int   `json:"page"`
		PerPage *int   `json:"perPage"`
		OrderBy string `json:"orderBy"`
	}

	BlockSearchArgs struct {
		Query   string `json:"query"`
		Page    *int   `json:"page"`
		PerPage *int   `json:"perPage"`
		OrderBy string `json:"orderBy"`
	}
)

func (rpc *RPC) Commit(_ *http.Request, args *CommitArgs, reply *ctypes.ResultCommit) error {
	height, err := getHeight(rpc.vm.blockStore, args.Height)
	if err != nil {
		return err
	}

	blockMeta := rpc.vm.blockStore.LoadBlockMeta(height)
	if blockMeta == nil {
		return nil
	}

	header := blockMeta.Header
	commit := rpc.vm.blockStore.LoadBlockCommit(height)
	res := ctypes.NewResultCommit(&header, commit, !(height == rpc.vm.blockStore.Height()))

	reply.SignedHeader = res.SignedHeader
	reply.CanonicalCommit = res.CanonicalCommit
	return nil
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
		panic(fmt.Sprintf("zero or negative perPage: %d", perPage))
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

func (rpc *RPC) Validators(_ *http.Request, args *ValidatorsArgs, reply *ctypes.ResultValidators) error {
	height, err := getHeight(rpc.vm.blockStore, args.Height)
	if err != nil {
		return err
	}

	validators, err := rpc.vm.stateStore.LoadValidators(height)
	if err != nil {
		return err
	}

	totalCount := len(validators.Validators)
	perPage := validatePerPage(args.PerPage)
	page, err := validatePage(args.Page, perPage, totalCount)
	if err != nil {
		return err
	}

	skipCount := validateSkipCount(page, perPage)

	reply.BlockHeight = height
	reply.Validators = validators.Validators[skipCount : skipCount+tmmath.MinInt(perPage, totalCount-skipCount)]
	reply.Count = len(reply.Validators)
	reply.Total = totalCount
	return nil
}

func (rpc *RPC) Tx(_ *http.Request, args *TxArgs, reply *ctypes.ResultTx) error {
	r, err := rpc.vm.txIndexer.Get(args.Hash)
	if err != nil {
		return err
	}

	if r == nil {
		return fmt.Errorf("tx (%X) not found", args.Hash)
	}

	height := r.Height
	index := r.Index

	var proof types.TxProof
	if args.Prove {
		block := rpc.vm.blockStore.LoadBlock(height)
		proof = block.Data.Txs.Proof(int(index)) // XXX: overflow on 32-bit machines
	}

	reply.Hash = args.Hash
	reply.Height = height
	reply.Index = index
	reply.TxResult = r.Result
	reply.Tx = r.Tx
	reply.Proof = proof
	return nil
}

func (rpc *RPC) TxSearch(req *http.Request, args *TxSearchArgs, reply *ctypes.ResultTxSearch) error {
	q, err := tmquery.New(args.Query)
	if err != nil {
		return err
	}

	var ctx context.Context
	if req != nil {
		ctx = req.Context()
	} else {
		ctx = context.Background()
	}

	results, err := rpc.vm.txIndexer.Search(ctx, q)
	if err != nil {
		return err
	}

	// sort results (must be done before pagination)
	switch args.OrderBy {
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
		return errors.New("expected order_by to be either `asc` or `desc` or empty")
	}

	// paginate results
	totalCount := len(results)
	perPage := validatePerPage(args.PerPage)

	page, err := validatePage(args.Page, perPage, totalCount)
	if err != nil {
		return err
	}

	skipCount := validateSkipCount(page, perPage)
	pageSize := tmmath.MinInt(perPage, totalCount-skipCount)

	apiResults := make([]*ctypes.ResultTx, 0, pageSize)
	for i := skipCount; i < skipCount+pageSize; i++ {
		r := results[i]

		var proof types.TxProof
		if args.Prove {
			block := rpc.vm.blockStore.LoadBlock(r.Height)
			proof = block.Data.Txs.Proof(int(r.Index)) // XXX: overflow on 32-bit machines
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

	reply.Txs = apiResults
	reply.TotalCount = totalCount
	return nil
}

func (rpc *RPC) BlockSearch(req *http.Request, args *BlockSearchArgs, reply *ctypes.ResultBlockSearch) error {
	q, err := tmquery.New(args.Query)
	if err != nil {
		return err
	}

	var ctx context.Context
	if req != nil {
		ctx = req.Context()
	} else {
		ctx = context.Background()
	}

	results, err := rpc.vm.blockIndexer.Search(ctx, q)
	if err != nil {
		return err
	}

	// sort results (must be done before pagination)
	switch args.OrderBy {
	case "desc", "":
		sort.Slice(results, func(i, j int) bool { return results[i] > results[j] })

	case "asc":
		sort.Slice(results, func(i, j int) bool { return results[i] < results[j] })

	default:
		return errors.New("expected order_by to be either `asc` or `desc` or empty")
	}

	// paginate results
	totalCount := len(results)
	perPage := validatePerPage(args.PerPage)

	page, err := validatePage(args.Page, perPage, totalCount)
	if err != nil {
		return err
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

	reply.Blocks = apiResults
	reply.TotalCount = totalCount
	return nil
}

func (rpc *RPC) Status(ctx *rpctypes.Context) (*ctypes.ResultStatus, error) {
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

	result := &ctypes.ResultStatus{
		NodeInfo: p2p.DefaultNodeInfo{
			ProtocolVersion: p2p.NewProtocolVersion(
				version.P2PProtocol,
				version.BlockProtocol,
				0,
			),
			DefaultNodeID: p2p.ID(rpc.vm.appOpts.NodeId),
			ListenAddr:    "",
			Network:       fmt.Sprintf("%d", rpc.vm.appOpts.NetworkId),
			Version:       version.TMCoreSemVer,
			Channels:      nil,
			Moniker:       "",
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
		//TODO: use internal app validators instead
		ValidatorInfo: ctypes.ValidatorInfo{
			Address:     proposerPubKey.Address(),
			PubKey:      proposerPubKey,
			VotingPower: 0,
		},
	}

	return result, nil
}
