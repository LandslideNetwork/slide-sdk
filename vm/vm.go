package vm

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/cometbft/cometbft/crypto/secp256k1"
	"os"
	"slices"
	"sync"
	"time"

	dbm "github.com/cometbft/cometbft-db"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/consensus"
	"github.com/cometbft/cometbft/libs/log"
	"github.com/cometbft/cometbft/mempool"
	"github.com/cometbft/cometbft/node"
	"github.com/cometbft/cometbft/proxy"
	"github.com/cometbft/cometbft/state"
	"github.com/cometbft/cometbft/state/indexer"
	blockidxkv "github.com/cometbft/cometbft/state/indexer/block/kv"
	"github.com/cometbft/cometbft/state/txindex"
	txidxkv "github.com/cometbft/cometbft/state/txindex/kv"
	"github.com/cometbft/cometbft/store"
	"github.com/cometbft/cometbft/types"
	grpcPrometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/consideritdone/landslidevm/database"
	"github.com/consideritdone/landslidevm/grpcutils"
	"github.com/consideritdone/landslidevm/http"
	"github.com/consideritdone/landslidevm/jsonrpc"
	httppb "github.com/consideritdone/landslidevm/proto/http"
	"github.com/consideritdone/landslidevm/proto/rpcdb"
	vmpb "github.com/consideritdone/landslidevm/proto/vm"
	vmtypes "github.com/consideritdone/landslidevm/vm/types"
	"github.com/consideritdone/landslidevm/vm/types/closer"
	"github.com/consideritdone/landslidevm/vm/types/commit"
	vmstate "github.com/consideritdone/landslidevm/vm/types/state"
)

const (
	genesisChunkSize = 16 * 1024 * 1024 // 16
)

var (
	_ vmpb.VMServer = (*LandslideVM)(nil)

	dbPrefixBlockStore   = []byte("block-store")
	dbPrefixStateStore   = []byte("state-store")
	dbPrefixTxIndexer    = []byte("tx-indexer")
	dbPrefixBlockIndexer = []byte("block-indexer")

	//TODO: use internal app validators instead
	proposerAddress = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	proposerPubKey  = secp256k1.PubKey{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	ErrNotFound = errors.New("not found")
)

type (
	Application = abcitypes.Application

	AppCreatorOpts struct {
		NetworkId    uint32
		SubnetId     []byte
		ChainId      []byte
		NodeId       []byte
		PublicKey    []byte
		XChainId     []byte
		CChainId     []byte
		AvaxAssetId  []byte
		GenesisBytes []byte
		UpgradeBytes []byte
		ConfigBytes  []byte
	}

	AppCreator func(*AppCreatorOpts) (Application, error)

	LandslideVM struct {
		allowShutdown *vmtypes.Atomic[bool]

		processMetrics prometheus.Gatherer
		serverCloser   grpcutils.ServerCloser
		connCloser     closer.Closer

		database   dbm.DB
		appCreator AppCreator
		app        proxy.AppConns
		appOpts    *AppCreatorOpts
		logger     log.Logger

		blockStore *store.BlockStore
		stateStore state.Store
		state      state.State
		genesis    *types.GenesisDoc
		genChunks  []string

		mempool  *mempool.CListMempool
		eventBus *types.EventBus

		txIndexer      txindex.TxIndexer
		blockIndexer   indexer.BlockIndexer
		indexerService *txindex.IndexerService

		vmenabled      *vmtypes.Atomic[bool]
		vmstate        *vmtypes.Atomic[vmpb.State]
		vmconnected    *vmtypes.Atomic[bool]
		verifiedBlocks sync.Map
		preferred      [32]byte
	}
)

func New(creator AppCreator) *LandslideVM {
	return NewViaDB(nil, creator)
}

func NewViaDB(database dbm.DB, creator AppCreator) *LandslideVM {
	return &LandslideVM{
		appCreator:     creator,
		database:       database,
		allowShutdown:  vmtypes.NewAtomic(true),
		vmenabled:      vmtypes.NewAtomic(false),
		vmstate:        vmtypes.NewAtomic(vmpb.State_STATE_UNSPECIFIED),
		vmconnected:    vmtypes.NewAtomic(false),
		verifiedBlocks: sync.Map{},
	}
}

// Initialize this VM.
func (vm *LandslideVM) Initialize(_ context.Context, req *vmpb.InitializeRequest) (*vmpb.InitializeResponse, error) {
	registerer := prometheus.NewRegistry()

	// Current state of process metrics
	processCollector := collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})
	if err := registerer.Register(processCollector); err != nil {
		return nil, err
	}

	// Go process metrics using debug.GCStats
	goCollector := collectors.NewGoCollector()
	if err := registerer.Register(goCollector); err != nil {
		return nil, err
	}

	// gRPC client metrics
	grpcClientMetrics := grpcPrometheus.NewClientMetrics()
	if err := registerer.Register(grpcClientMetrics); err != nil {
		return nil, err
	}

	// Register metrics for each Go plugin processes
	vm.processMetrics = registerer

	// Dial the database
	if vm.database == nil {
		dbClientConn, err := grpc.Dial(
			"passthrough:///"+req.DbServerAddr,
			grpc.WithChainUnaryInterceptor(grpcClientMetrics.UnaryClientInterceptor()),
			grpc.WithChainStreamInterceptor(grpcClientMetrics.StreamClientInterceptor()),
		)
		if err != nil {
			return nil, err
		}
		vm.connCloser.Add(dbClientConn)
		vm.database = database.New(rpcdb.NewDatabaseClient(dbClientConn))
	}
	vm.logger = log.NewTMLogger(os.Stdout)

	dbBlockStore := dbm.NewPrefixDB(vm.database, dbPrefixBlockStore)
	vm.blockStore = store.NewBlockStore(dbBlockStore)
	dbStateStore := dbm.NewPrefixDB(vm.database, dbPrefixStateStore)
	vm.stateStore = state.NewStore(dbStateStore, state.StoreOptions{DiscardABCIResponses: false})

	vm.appOpts = &AppCreatorOpts{
		NetworkId:    req.NetworkId,
		SubnetId:     req.SubnetId,
		ChainId:      req.CChainId,
		NodeId:       req.NodeId,
		PublicKey:    req.PublicKey,
		XChainId:     req.XChainId,
		CChainId:     req.CChainId,
		AvaxAssetId:  req.AvaxAssetId,
		GenesisBytes: req.GenesisBytes,
		UpgradeBytes: req.UpgradeBytes,
		ConfigBytes:  req.ConfigBytes,
	}
	app, err := vm.appCreator(vm.appOpts)
	if err != nil {
		return nil, err
	}

	vm.state, vm.genesis, err = node.LoadStateFromDBOrGenesisDocProvider(
		dbStateStore,
		func() (*types.GenesisDoc, error) {
			return types.GenesisDocFromJSON(req.GenesisBytes)
		},
	)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(req.GenesisBytes); i += genesisChunkSize {
		end := i + genesisChunkSize
		if end > len(req.GenesisBytes) {
			end = len(req.GenesisBytes)
		}
		vm.genChunks = append(vm.genChunks, base64.StdEncoding.EncodeToString(req.GenesisBytes[i:end]))
	}

	vm.app = proxy.NewAppConns(proxy.NewLocalClientCreator(app), proxy.NopMetrics())
	vm.app.SetLogger(vm.logger.With("module", "proxy"))
	if err := vm.app.Start(); err != nil {
		return nil, fmt.Errorf("error starting proxy app connections: %v", err)
	}

	vm.eventBus = types.NewEventBus()
	vm.eventBus.SetLogger(vm.logger.With("module", "events"))
	if err := vm.eventBus.Start(); err != nil {
		return nil, err
	}

	dbTxIndexer := dbm.NewPrefixDB(vm.database, dbPrefixTxIndexer)
	vm.txIndexer = txidxkv.NewTxIndex(dbTxIndexer)

	dbBlockIndexer := dbm.NewPrefixDB(vm.database, dbPrefixBlockIndexer)
	vm.blockIndexer = blockidxkv.New(dbBlockIndexer)

	vm.indexerService = txindex.NewIndexerService(vm.txIndexer, vm.blockIndexer, vm.eventBus, true)
	vm.indexerService.SetLogger(vm.logger.With("module", "indexer"))
	if err := vm.indexerService.Start(); err != nil {
		return nil, err
	}

	handshaker := consensus.NewHandshaker(
		vm.stateStore,
		vm.state,
		vm.blockStore,
		vm.genesis,
	)
	handshaker.SetLogger(vm.logger.With("module", "consensus"))
	handshaker.SetEventBus(vm.eventBus)
	if err := handshaker.Handshake(vm.app); err != nil {
		return nil, fmt.Errorf("error during handshake: %v", err)
	}

	vm.state, err = vm.stateStore.Load()
	if err != nil {
		return nil, err
	}

	vm.mempool = mempool.NewCListMempool(
		config.DefaultMempoolConfig(),
		vm.app.Mempool(),
		vm.state.LastBlockHeight,
		mempool.WithMetrics(mempool.NopMetrics()),
		mempool.WithPreCheck(state.TxPreCheck(vm.state)),
		mempool.WithPostCheck(state.TxPostCheck(vm.state)),
	)
	vm.mempool.SetLogger(vm.logger.With("module", "mempool"))
	vm.mempool.EnableTxsAvailable()

	var block *types.Block
	if vm.state.LastBlockHeight > 0 {
		block = vm.blockStore.LoadBlock(vm.state.LastBlockHeight)
	} else {
		executor := vmstate.NewBlockExecutor(vm.stateStore, vm.logger, vm.app.Consensus(), vm.mempool, vm.blockStore)
		executor.SetEventBus(vm.eventBus)

		block, err = executor.CreateProposalBlock(context.Background(), vm.state.LastBlockHeight+1, vm.state, &types.ExtendedCommit{}, proposerAddress)
		if err != nil {
			return nil, err
		}

		bps, err := block.MakePartSet(types.BlockPartSizeBytes)
		if err != nil {
			return nil, err
		}

		newstate, err := executor.ApplyBlock(vm.state, types.BlockID{
			Hash:          block.Hash(),
			PartSetHeader: bps.Header(),
		}, block)
		if err != nil {
			return nil, err
		}

		vm.blockStore.SaveBlock(block, bps, commit.MakeCommit(block.Height, block.Time, block.ProposerAddress, bps.Header()))
		vm.stateStore.Save(newstate)
		vm.state = newstate
	}
	vm.logger.Info("vm initialization completed")

	blockBytes, err := vmstate.EncodeBlock(block)
	if err != nil {
		return nil, err
	}
	return &vmpb.InitializeResponse{
		LastAcceptedId:       block.Hash(),
		LastAcceptedParentId: block.LastBlockID.Hash,
		Height:               uint64(block.Height),
		Bytes:                blockBytes,
		Timestamp:            timestamppb.New(block.Time),
	}, nil
}

// SetState communicates to VM its next state it starts
func (vm *LandslideVM) SetState(_ context.Context, req *vmpb.SetStateRequest) (*vmpb.SetStateResponse, error) {
	block := vm.blockStore.LoadBlock(vm.state.LastBlockHeight)
	if block == nil {
		return nil, ErrNotFound
	}
	res := vmpb.SetStateResponse{
		LastAcceptedId:       block.Hash(),
		LastAcceptedParentId: block.LastBlockID.Hash,
		Height:               uint64(block.Height),
		Bytes:                vm.state.Bytes(),
		Timestamp:            timestamppb.New(block.Time),
	}
	vm.vmstate.Set(req.State)
	return &res, nil
}

// CanShutdown lets known when vm ready to shutting down
func (vm *LandslideVM) CanShutdown() bool {
	return vm.allowShutdown.Get()
}

// Shutdown is called when the node is shutting down.
func (vm *LandslideVM) Shutdown(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	vm.allowShutdown.Set(true)
	vm.serverCloser.Stop()
	if err := vm.connCloser.Close(); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// CreateHandlers creates the HTTP handlers for custom chain network calls.
func (vm *LandslideVM) CreateHandlers(context.Context, *emptypb.Empty) (*vmpb.CreateHandlersResponse, error) {
	server := grpcutils.NewServer()
	vm.serverCloser.Add(server)
	httppb.RegisterHTTPServer(server, http.NewServer(
		jsonrpc.NewServer(NewRPC(vm).Routes()),
	))

	listener, err := grpcutils.NewListener()
	if err != nil {
		return nil, err
	}

	go grpcutils.Serve(listener, server)

	return &vmpb.CreateHandlersResponse{
		Handlers: []*vmpb.Handler{
			{
				Prefix:     "/rpc",
				ServerAddr: listener.Addr().String(),
			},
		},
	}, nil
}

/*
func (vm *VMServer) CreateHandlers(ctx context.Context, _ *emptypb.Empty) (*vmpb.CreateHandlersResponse, error) {
	handlers, err := vm.vm.CreateHandlers(ctx)
	if err != nil {
		return nil, err
	}
	resp := &vmpb.CreateHandlersResponse{}
	for prefix, handler := range handlers {
		serverListener, err := grpcutils.NewListener()
		if err != nil {
			return nil, err
		}
		server := grpcutils.NewServer()
		vm.serverCloser.Add(server)
		httppb.RegisterHTTPServer(server, ghttp.NewServer(handler))

		// Start HTTP service
		go grpcutils.Serve(serverListener, server)

		resp.Handlers = append(resp.Handlers, &vmpb.Handler{
			Prefix:     prefix,
			ServerAddr: serverListener.Addr().String(),
		})
	}
	return resp, nil
}

*/

func (vm *LandslideVM) Connected(context.Context, *vmpb.ConnectedRequest) (*emptypb.Empty, error) {
	vm.vmconnected.Set(true)
	return &emptypb.Empty{}, nil
}

func (vm *LandslideVM) Disconnected(context.Context, *vmpb.DisconnectedRequest) (*emptypb.Empty, error) {
	vm.vmconnected.Set(true)
	return &emptypb.Empty{}, nil
}

// BuildBlock attempt to create a new block from data contained in the VM.
func (vm *LandslideVM) BuildBlock(context.Context, *vmpb.BuildBlockRequest) (*vmpb.BuildBlockResponse, error) {
	executor := vmstate.NewBlockExecutor(vm.stateStore, vm.logger, vm.app.Consensus(), vm.mempool, vm.blockStore)
	executor.SetEventBus(vm.eventBus)

	signatures := make([]types.ExtendedCommitSig, len(vm.state.Validators.Validators))
	commit := types.ExtendedCommit{
		ExtendedSignatures: signatures,
	}
	block, err := executor.CreateProposalBlock(context.Background(), vm.state.LastBlockHeight+1, vm.state, &commit, proposerAddress)
	if err != nil {
		return nil, err
	}
	block.Time = time.Now()

	blockBytes, err := vmstate.EncodeBlock(block)
	if err != nil {
		return nil, err
	}

	vm.verifiedBlocks.Store([32]byte(block.Hash()), block)
	return &vmpb.BuildBlockResponse{
		Id:                block.Hash(),
		ParentId:          block.LastBlockID.Hash,
		Bytes:             blockBytes,
		Height:            uint64(block.Height),
		Timestamp:         timestamppb.New(block.Time),
		VerifyWithContext: false,
	}, nil
}

// ParseBlock attempt to create a block from a stream of bytes.
func (vm *LandslideVM) ParseBlock(_ context.Context, req *vmpb.ParseBlockRequest) (*vmpb.ParseBlockResponse, error) {
	block, err := vmstate.DecodeBlock(req.GetBytes())
	if err != nil {
		return nil, err
	}

	if _, ok := vm.verifiedBlocks.Load(block.Hash()); !ok {
		vm.verifiedBlocks.Store(block.Hash(), block)
	}

	return &vmpb.ParseBlockResponse{
		Id:                block.Hash(),
		ParentId:          block.LastBlockID.Hash,
		Status:            vmpb.Status_STATUS_UNSPECIFIED,
		Height:            uint64(block.Height),
		Timestamp:         timestamppb.New(block.Time),
		VerifyWithContext: false,
	}, nil
}

// GetBlock attempt to load a block.
func (vm *LandslideVM) GetBlock(_ context.Context, req *vmpb.GetBlockRequest) (*vmpb.GetBlockResponse, error) {
	block := vm.blockStore.LoadBlockByHash(req.GetId())
	if block == nil {
		return &vmpb.GetBlockResponse{
			Err: vmpb.Error_ERROR_NOT_FOUND,
		}, nil
	}

	blockBytes, err := vmstate.EncodeBlock(block)
	if err != nil {
		return nil, err
	}

	return &vmpb.GetBlockResponse{
		ParentId:  block.LastBlockID.Hash,
		Bytes:     blockBytes,
		Status:    vmpb.Status_STATUS_UNSPECIFIED,
		Height:    uint64(block.Height),
		Timestamp: timestamppb.New(block.Time),
	}, nil
}

// SetPreference notify the VM of the currently preferred block.
func (vm *LandslideVM) SetPreference(_ context.Context, req *vmpb.SetPreferenceRequest) (*emptypb.Empty, error) {
	vm.preferred = [32]byte(req.GetId())
	return &emptypb.Empty{}, nil
}

// Health attempt to verify the health of the VM.
func (vm *LandslideVM) Health(context.Context, *emptypb.Empty) (*vmpb.HealthResponse, error) {
	return nil, errors.New("TODO: implement me")
}

// Version returns the version of the VM.
func (vm *LandslideVM) Version(context.Context, *emptypb.Empty) (*vmpb.VersionResponse, error) {
	return nil, errors.New("TODO: implement me")
}

// AppRequest notify this engine of a request for data from [nodeID].
func (vm *LandslideVM) AppRequest(context.Context, *vmpb.AppRequestMsg) (*emptypb.Empty, error) {
	return nil, errors.New("TODO: implement me")
}

// AppRequestFailed notify this engine that an AppRequest message it sent to [nodeID] with
// request ID [requestID] failed.
func (vm *LandslideVM) AppRequestFailed(context.Context, *vmpb.AppRequestFailedMsg) (*emptypb.Empty, error) {
	return nil, errors.New("TODO: implement me")
}

// AppResponse notify this engine of a response to the AppRequest message it sent to
// [nodeID] with request ID [requestID].
func (vm *LandslideVM) AppResponse(context.Context, *vmpb.AppResponseMsg) (*emptypb.Empty, error) {
	return nil, errors.New("TODO: implement me")
}

// AppGossip notify this engine of a gossip message from [nodeID].
func (vm *LandslideVM) AppGossip(context.Context, *vmpb.AppGossipMsg) (*emptypb.Empty, error) {
	return nil, errors.New("TODO: implement me")
}

// Gather attempts to gather metrics from a VM.
func (vm *LandslideVM) Gather(context.Context, *emptypb.Empty) (*vmpb.GatherResponse, error) {
	return nil, errors.New("TODO: implement me")
}

func (vm *LandslideVM) CrossChainAppRequest(context.Context, *vmpb.CrossChainAppRequestMsg) (*emptypb.Empty, error) {
	return nil, errors.New("TODO: implement me")
}

func (vm *LandslideVM) CrossChainAppRequestFailed(context.Context, *vmpb.CrossChainAppRequestFailedMsg) (*emptypb.Empty, error) {
	return nil, errors.New("TODO: implement me")
}

func (vm *LandslideVM) CrossChainAppResponse(context.Context, *vmpb.CrossChainAppResponseMsg) (*emptypb.Empty, error) {
	return nil, errors.New("TODO: implement me")
}

func (vm *LandslideVM) GetAncestors(context.Context, *vmpb.GetAncestorsRequest) (*vmpb.GetAncestorsResponse, error) {
	return nil, errors.New("TODO: implement me")
}

func (vm *LandslideVM) BatchedParseBlock(ctx context.Context, req *vmpb.BatchedParseBlockRequest) (*vmpb.BatchedParseBlockResponse, error) {
	responses := make([]*vmpb.ParseBlockResponse, len(req.Request))
	var err error
	for i := range req.Request {
		responses[i], err = vm.ParseBlock(ctx, &vmpb.ParseBlockRequest{Bytes: slices.Clone(req.Request[i])})
		if err != nil {
			return nil, err
		}
	}
	return &vmpb.BatchedParseBlockResponse{Response: responses}, nil
}

func (vm *LandslideVM) VerifyHeightIndex(context.Context, *emptypb.Empty) (*vmpb.VerifyHeightIndexResponse, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) GetBlockIDAtHeight(_ context.Context, req *vmpb.GetBlockIDAtHeightRequest) (*vmpb.GetBlockIDAtHeightResponse, error) {
	block := vm.blockStore.LoadBlock(int64(req.GetHeight()))
	if block == nil {
		return &vmpb.GetBlockIDAtHeightResponse{
			Err: vmpb.Error_ERROR_NOT_FOUND,
		}, nil
	}
	return &vmpb.GetBlockIDAtHeightResponse{BlkId: block.Hash()}, nil
}

// StateSyncEnabled indicates whether the state sync is enabled for this VM.
func (vm *LandslideVM) StateSyncEnabled(context.Context, *emptypb.Empty) (*vmpb.StateSyncEnabledResponse, error) {
	return &vmpb.StateSyncEnabledResponse{Enabled: vm.vmenabled.Get()}, nil
}

// GetOngoingSyncStateSummary returns an in-progress state summary if it exists.
func (vm *LandslideVM) GetOngoingSyncStateSummary(context.Context, *emptypb.Empty) (*vmpb.GetOngoingSyncStateSummaryResponse, error) {
	panic("ToDo: implement me")
}

// GetLastStateSummary returns the latest state summary.
func (vm *LandslideVM) GetLastStateSummary(context.Context, *emptypb.Empty) (*vmpb.GetLastStateSummaryResponse, error) {
	panic("ToDo: implement me")
}

// ParseStateSummary parses a state summary out of [summaryBytes].
func (vm *LandslideVM) ParseStateSummary(context.Context, *vmpb.ParseStateSummaryRequest) (*vmpb.ParseStateSummaryResponse, error) {
	return nil, errors.New("TODO: implement me")
}

// GetStateSummary retrieves the state summary that was generated at height
// [summaryHeight].
func (vm *LandslideVM) GetStateSummary(context.Context, *vmpb.GetStateSummaryRequest) (*vmpb.GetStateSummaryResponse, error) {
	return nil, errors.New("TODO: implement me")
}

func (vm *LandslideVM) BlockVerify(_ context.Context, req *vmpb.BlockVerifyRequest) (*vmpb.BlockVerifyResponse, error) {
	block, err := vmstate.DecodeBlock(req.Bytes)
	if err != nil {
		return nil, err
	}

	err = vmstate.ValidateBlock(vm.state, block)
	if err != nil {
		return nil, err
	}

	return &vmpb.BlockVerifyResponse{Timestamp: timestamppb.New(block.Time)}, nil
}

func (vm *LandslideVM) BlockAccept(_ context.Context, req *vmpb.BlockAcceptRequest) (*emptypb.Empty, error) {
	rawBlock, exist := vm.verifiedBlocks.Load([32]byte(req.GetId()))
	if !exist {
		return nil, ErrNotFound
	}
	block, ok := rawBlock.(*types.Block)
	if !ok {
		return nil, ErrNotFound
	}

	executor := vmstate.NewBlockExecutor(vm.stateStore, vm.logger, vm.app.Consensus(), vm.mempool, vm.blockStore)
	executor.SetEventBus(vm.eventBus)

	bps, err := block.MakePartSet(types.BlockPartSizeBytes)
	if err != nil {
		return nil, err
	}

	newstate, err := executor.ApplyBlock(vm.state, types.BlockID{
		Hash:          block.Hash(),
		PartSetHeader: bps.Header(),
	}, block)
	if err != nil {
		return nil, err
	}

	vm.blockStore.SaveBlock(block, bps, commit.MakeCommit(block.Height, block.Time, block.ProposerAddress, bps.Header()))
	vm.stateStore.Save(newstate)
	vm.state = newstate
	vm.verifiedBlocks.Delete([32]byte(req.GetId()))
	return &emptypb.Empty{}, nil
}

func (vm *LandslideVM) BlockReject(_ context.Context, req *vmpb.BlockRejectRequest) (*emptypb.Empty, error) {
	_, exist := vm.verifiedBlocks.LoadAndDelete([32]byte(req.GetId()))
	if !exist {
		return nil, ErrNotFound
	}
	return &emptypb.Empty{}, nil
}

func (vm *LandslideVM) StateSummaryAccept(context.Context, *vmpb.StateSummaryAcceptRequest) (*vmpb.StateSummaryAcceptResponse, error) {
	return nil, errors.New("TODO: implement me")
}
