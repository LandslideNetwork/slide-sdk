package vm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	http2 "net/http"
	"os"
	"slices"
	"sync"
	"time"

	dbm "github.com/cometbft/cometbft-db"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/consensus"
	"github.com/cometbft/cometbft/crypto"
	"github.com/cometbft/cometbft/crypto/secp256k1"
	"github.com/cometbft/cometbft/libs/log"
	"github.com/cometbft/cometbft/libs/pubsub"
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
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	rpcserver "github.com/cometbft/cometbft/rpc/jsonrpc/server"
	"github.com/consideritdone/landslidevm/database"
	"github.com/consideritdone/landslidevm/grpcutils"
	"github.com/consideritdone/landslidevm/http"
	"github.com/consideritdone/landslidevm/jsonrpc"
	httppb "github.com/consideritdone/landslidevm/proto/http"
	messengerpb "github.com/consideritdone/landslidevm/proto/messenger"
	"github.com/consideritdone/landslidevm/proto/rpcdb"
	vmpb "github.com/consideritdone/landslidevm/proto/vm"
	"github.com/consideritdone/landslidevm/utils/ids"
	vmtypes "github.com/consideritdone/landslidevm/vm/types"
	"github.com/consideritdone/landslidevm/vm/types/block"
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

	// TODO: use internal app validators instead
	proposerAddress = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	proposerPubKey  = secp256k1.PubKey{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	Version = "0.0.0"

	ErrNotFound     = errors.New("not found")
	ErrUnknownState = errors.New("unknown state")
)

type (
	Application = abcitypes.Application

	AppCreatorOpts struct {
		NetworkID    uint32
		SubnetID     []byte
		ChainID      []byte
		NodeID       []byte
		PublicKey    []byte
		XChainID     []byte
		CChainID     []byte
		AvaxAssetID  []byte
		GenesisBytes []byte
		UpgradeBytes []byte
		ConfigBytes  []byte
		Config       *vmtypes.Config
		ChainDataDir string
	}

	AppCreator func(*AppCreatorOpts) (Application, error)

	LandslideVM struct {
		allowShutdown *vmtypes.Atomic[bool]

		processMetrics prometheus.Gatherer
		serverCloser   grpcutils.ServerCloser
		connCloser     closer.Closer

		database       dbm.DB
		databaseClient rpcdb.DatabaseClient
		appCreator     AppCreator
		app            proxy.AppConns
		appOpts        *AppCreatorOpts
		logger         log.Logger

		toEngine chan messengerpb.Message
		closed   chan struct{}

		blockStore *store.BlockStore
		stateStore state.Store
		state      state.State
		genesis    *types.GenesisDoc
		genChunks  []string

		mempool  *mempool.CListMempool
		eventBus *types.EventBus

		bootstrapped *vmtypes.Atomic[bool]

		txIndexer      txindex.TxIndexer
		blockIndexer   indexer.BlockIndexer
		indexerService *txindex.IndexerService

		vmenabled      *vmtypes.Atomic[bool]
		vmstate        *vmtypes.Atomic[vmpb.State]
		vmconnected    *vmtypes.Atomic[bool]
		verifiedBlocks sync.Map
		preferred      [32]byte
		wrappedBlocks  *vmstate.WrappedBlocksStorage

		clientConn    grpc.ClientConnInterface
		optClientConn *grpc.ClientConn
		config        vmtypes.VMConfig
	}
)

func New(creator AppCreator) *LandslideVM {
	return NewViaDB(nil, creator)
}

func NewViaDB(database dbm.DB, creator AppCreator, options ...func(*LandslideVM)) *LandslideVM {
	vm := &LandslideVM{
		appCreator:     creator,
		database:       database,
		allowShutdown:  vmtypes.NewAtomic(false),
		vmenabled:      vmtypes.NewAtomic(false),
		vmstate:        vmtypes.NewAtomic(vmpb.State_STATE_UNSPECIFIED),
		vmconnected:    vmtypes.NewAtomic(false),
		bootstrapped:   vmtypes.NewAtomic(false),
		verifiedBlocks: sync.Map{},
		wrappedBlocks:  vmstate.NewWrappedBlocksStorage(),
	}

	for _, o := range options {
		o(vm)
	}

	return vm
}

// WithClientConn sets the client connection for the VM.
func WithClientConn(clientConn grpc.ClientConnInterface) func(vm *LandslideVM) {
	return func(vm *LandslideVM) {
		vm.clientConn = clientConn
	}
}

// WithOptClientConn sets the optional client connection for the VM.
// it overrides the client connection set by WithClientConn.
func WithOptClientConn(clientConn *grpc.ClientConn) func(vm *LandslideVM) {
	return func(vm *LandslideVM) {
		vm.optClientConn = clientConn
	}
}

// Initialize initializes the VM.
// This method should only be accessible by the AvalancheGo node and not exposed publicly.
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

	// add to connCloser even we have defined vm.clientConn via Option
	if vm.optClientConn != nil {
		vm.connCloser.Add(vm.optClientConn)
		vm.clientConn = vm.optClientConn
	} else {
		clientConn, err := grpc.Dial(
			"passthrough:///"+req.ServerAddr,
			grpc.WithChainUnaryInterceptor(grpcClientMetrics.UnaryClientInterceptor()),
			grpc.WithChainStreamInterceptor(grpcClientMetrics.StreamClientInterceptor()),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			// Ignore closing errors to return the original error
			_ = vm.connCloser.Close()
			return nil, err
		}

		vm.connCloser.Add(clientConn)
		vm.clientConn = clientConn
	}

	vm.logger = log.NewTMLogger(os.Stdout)

	msgClient := messengerpb.NewMessengerClient(vm.clientConn)

	vm.toEngine = make(chan messengerpb.Message, 1)
	vm.closed = make(chan struct{})
	go func() {
		for {
			select {
			case msg, ok := <-vm.toEngine:
				if !ok {
					vm.logger.Error("channel closed")
					return
				}
				// Nothing to do with the error within the goroutine
				_, err := msgClient.Notify(context.Background(), &messengerpb.NotifyRequest{
					Message: msg,
				})
				if err != nil {
					vm.logger.Error("failed to notify", "err", err)
				}

			case <-vm.closed:
				return
			}
		}
	}()

	// Dial the database
	if vm.database == nil {
		dbClientConn, err := grpc.Dial(
			"passthrough:///"+req.DbServerAddr,
			grpc.WithChainUnaryInterceptor(grpcClientMetrics.UnaryClientInterceptor()),
			grpc.WithChainStreamInterceptor(grpcClientMetrics.StreamClientInterceptor()),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			vm.logger.Error("failed to dial database", "err", err)
			return nil, err
		}
		vm.connCloser.Add(dbClientConn)
		vm.databaseClient = rpcdb.NewDatabaseClient(dbClientConn)
		vm.database = database.New(vm.databaseClient)
	}

	dbBlockStore := dbm.NewPrefixDB(vm.database, dbPrefixBlockStore)
	vm.blockStore = store.NewBlockStore(dbBlockStore)
	dbStateStore := dbm.NewPrefixDB(vm.database, dbPrefixStateStore)
	vm.stateStore = state.NewStore(dbStateStore, state.StoreOptions{DiscardABCIResponses: false})

	vm.appOpts = &AppCreatorOpts{
		NetworkID:    req.NetworkId,
		SubnetID:     req.SubnetId,
		ChainID:      req.ChainId,
		NodeID:       req.NodeId,
		PublicKey:    req.PublicKey,
		XChainID:     req.XChainId,
		CChainID:     req.CChainId,
		AvaxAssetID:  req.AvaxAssetId,
		GenesisBytes: req.GenesisBytes,
		UpgradeBytes: req.UpgradeBytes,
		ConfigBytes:  req.ConfigBytes,
		ChainDataDir: req.ChainDataDir,
	}
	app, err := vm.appCreator(vm.appOpts)
	if err != nil {
		vm.logger.Error("failed to create app", "err", err)
		return nil, err
	}

	// Set the default configuration
	var cfg vmtypes.Config
	cfg.VMConfig.SetDefaults()
	if len(vm.appOpts.ConfigBytes) > 0 {
		if err := json.Unmarshal(vm.appOpts.ConfigBytes, &cfg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config %s: %w", string(vm.appOpts.ConfigBytes), err)
		}
	}
	if err := cfg.VMConfig.Validate(); err != nil {
		return nil, err
	}
	vm.config = cfg.VMConfig

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

	go func() {
		for {
			<-vm.mempool.TxsAvailable()
			vm.toEngine <- messengerpb.Message_MESSAGE_BUILD_BLOCK
		}
	}()

	var blk *types.Block
	if vm.state.LastBlockHeight > 0 {
		vm.logger.Debug("loading last block", "height", vm.state.LastBlockHeight)
		blk = vm.blockStore.LoadBlock(vm.state.LastBlockHeight)
	} else {
		vm.logger.Debug("creating genesis block")
		executor := vmstate.NewBlockExecutor(
			vm.stateStore,
			vm.logger,
			vm.app.Consensus(),
			vm.mempool,
			vm.blockStore,
			vm.config.ConsensusParams.Block.MaxBytes,
			vm.config.ConsensusParams.Block.MaxGas,
			vm.config.ConsensusParams.Evidence.MaxBytes,
		)
		executor.SetEventBus(vm.eventBus)

		blk, err = executor.CreateProposalBlock(context.Background(), vm.state.LastBlockHeight+1, vm.state, &types.ExtendedCommit{}, proposerAddress)
		if err != nil {
			return nil, err
		}

		bps, err := blk.MakePartSet(types.BlockPartSizeBytes)
		if err != nil {
			return nil, err
		}

		blockID := types.BlockID{
			Hash:          blk.Hash(),
			PartSetHeader: bps.Header(),
		}

		newstate, err := executor.ApplyBlock(vm.state, blockID, blk)
		if err != nil {
			return nil, err
		}

		vm.blockStore.SaveBlock(blk, bps, commit.MakeCommit(blk.Height, blk.Time, vm.state.Validators, blockID))
		err = vm.stateStore.Save(newstate)
		if err != nil {
			vm.logger.Error("failed to save state", "err", err)
			return nil, err
		}
		vm.state = newstate
	}

	blockBytes, err := vmstate.EncodeBlockWithStatus(blk, vmpb.Status_STATUS_ACCEPTED)
	if err != nil {
		return nil, err
	}
	// vm.logger.Debug("initialize block", "bytes ", blockBytes)
	vm.logger.Info("vm initialization completed")

	parentHash := block.ParentHash(blk)

	return &vmpb.InitializeResponse{
		LastAcceptedId:       blk.Hash(),
		LastAcceptedParentId: parentHash[:],
		Height:               uint64(blk.Height),
		Bytes:                blockBytes,
		Timestamp:            timestamppb.New(blk.Time),
	}, nil
}

// SetState communicates to VM its next state it starts
func (vm *LandslideVM) SetState(_ context.Context, req *vmpb.SetStateRequest) (*vmpb.SetStateResponse, error) {
	vm.logger.Info("SetState", "state", req.State)
	switch req.State {
	case vmpb.State_STATE_BOOTSTRAPPING:
		vm.bootstrapped.Set(false)
	case vmpb.State_STATE_NORMAL_OP:
		vm.bootstrapped.Set(true)
	default:
		vm.logger.Error("SetState", "state", req.State)
		return nil, ErrUnknownState
	}
	blk := vm.blockStore.LoadBlock(vm.state.LastBlockHeight)
	if blk == nil {
		return nil, ErrNotFound
	}

	vm.logger.Debug("SetState", "LastAcceptedId", vm.state.LastBlockID.Hash, "block", blk.Hash())
	parentHash := block.ParentHash(blk)
	res := vmpb.SetStateResponse{
		LastAcceptedId:       blk.Hash(),
		LastAcceptedParentId: parentHash[:],
		Height:               uint64(blk.Height),
		Bytes:                vm.state.Bytes(),
		Timestamp:            timestamppb.New(blk.Time),
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
	fmt.Println("Shutdown")
	vm.allowShutdown.Set(true)
	if vm.closed != nil {
		close(vm.closed)
	}
	var err error
	if vm.indexerService != nil {
		err = vm.indexerService.Stop()
	}
	if vm.eventBus != nil {
		err = errors.Join(err, vm.eventBus.Stop())
	}
	if vm.app != nil {
		err = errors.Join(err, vm.app.Stop())
	}
	if vm.stateStore != nil {
		err = errors.Join(err, vm.stateStore.Close())
	}
	if vm.blockStore != nil {
		err = errors.Join(err, vm.blockStore.Close())
	}
	vm.serverCloser.Stop()
	err = errors.Join(err, vm.connCloser.Close())
	return &emptypb.Empty{}, err
}

// CreateHandlers creates the HTTP handlers for custom chain network calls.
func (vm *LandslideVM) CreateHandlers(context.Context, *emptypb.Empty) (*vmpb.CreateHandlersResponse, error) {
	server := grpcutils.NewServer()
	vm.serverCloser.Add(server)

	mux := http2.NewServeMux()
	tmRPC := NewRPC(vm)
	wm := rpcserver.NewWebsocketManager(tmRPC.TMRoutes(),
		rpcserver.OnDisconnect(func(remoteAddr string) {
			err := vm.eventBus.UnsubscribeAll(context.Background(), remoteAddr)
			if err != nil && err != pubsub.ErrSubscriptionNotFound {
				vm.logger.Error("Failed to unsubscribe addr from events", "addr", remoteAddr, "err", err)
			}
		}),
		rpcserver.ReadLimit(config.DefaultRPCConfig().MaxBodyBytes),
		rpcserver.WriteChanCapacity(config.DefaultRPCConfig().WebSocketWriteBufferSize),
	)
	wm.SetLogger(vm.logger)
	mux.HandleFunc("/websocket", wm.WebsocketHandler)
	mux.HandleFunc("/v1/websocket", wm.WebsocketHandler)
	jsonrpc.RegisterRPCFuncs(mux, tmRPC.Routes(), vm.logger)

	httppb.RegisterHTTPServer(server, http.NewServer(mux))

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

func (vm *LandslideVM) Connected(context.Context, *vmpb.ConnectedRequest) (*emptypb.Empty, error) {
	vm.logger.Info("Connected")
	vm.vmconnected.Set(true)
	return &emptypb.Empty{}, nil
}

func (vm *LandslideVM) Disconnected(context.Context, *vmpb.DisconnectedRequest) (*emptypb.Empty, error) {
	vm.logger.Info("Disconnected")
	vm.vmconnected.Set(false)
	return &emptypb.Empty{}, nil
}

// BuildBlock attempts to create a new block from data contained in the VM.
// This method should be restricted to the AvalancheGo node.
func (vm *LandslideVM) BuildBlock(context.Context, *vmpb.BuildBlockRequest) (*vmpb.BuildBlockResponse, error) {
	vm.logger.Info("BuildBlock")
	executor := vmstate.NewBlockExecutor(
		vm.stateStore,
		vm.logger,
		vm.app.Consensus(),
		vm.mempool,
		vm.blockStore,
		vm.config.ConsensusParams.Block.MaxBytes,
		vm.config.ConsensusParams.Block.MaxGas,
		vm.config.ConsensusParams.Evidence.MaxBytes,
	)
	executor.SetEventBus(vm.eventBus)

	signatures := make([]types.ExtendedCommitSig, len(vm.state.Validators.Validators))
	for i := range signatures {
		signatures[i] = types.ExtendedCommitSig{
			CommitSig: types.CommitSig{
				BlockIDFlag:      types.BlockIDFlagNil,
				Timestamp:        time.Now(),
				ValidatorAddress: vm.state.Validators.Validators[i].Address,
				Signature:        crypto.CRandBytes(types.MaxSignatureSize), // todo: sign the block
			},
		}
	}

	lastComm := types.ExtendedCommit{
		Height:             vm.state.LastBlockHeight,
		Round:              0,
		BlockID:            vm.state.LastBlockID,
		ExtendedSignatures: signatures,
	}

	blk, err := executor.CreateProposalBlock(context.Background(), vm.state.LastBlockHeight+1, vm.state, &lastComm, proposerAddress)
	if err != nil {
		vm.logger.Error("failed to create proposal block", "err", err)
		return nil, err
	}

	blkStatus := vmpb.Status_STATUS_PROCESSING
	blkBytes, err := vmstate.EncodeBlockWithStatus(blk, blkStatus)
	if err != nil {
		vm.logger.Error("failed to encode block", "err", err)
		return nil, err
	}

	blkID, err := ids.ToID(blk.Hash())
	if err != nil {
		vm.logger.Error("failed to convert block hash to ID", "err", err)
		return nil, err
	}
	vm.wrappedBlocks.UnverifiedBlocks.Put(blkID, &vmstate.WrappedBlock{
		Block:  blk,
		Status: blkStatus,
	})
	vm.wrappedBlocks.MissingBlocks.Evict(blkID)

	return &vmpb.BuildBlockResponse{
		Id:                blk.Hash(),
		ParentId:          blk.LastBlockID.Hash,
		Bytes:             blkBytes,
		Height:            uint64(blk.Height),
		Timestamp:         timestamppb.New(blk.Time),
		VerifyWithContext: false,
	}, nil
}

// ParseBlock attempt to create a block from a stream of bytes.
func (vm *LandslideVM) ParseBlock(_ context.Context, req *vmpb.ParseBlockRequest) (*vmpb.ParseBlockResponse, error) {
	vm.logger.Info("ParseBlock")
	// vm.logger.Debug("ParseBlock", "bytes", req.Bytes)
	var (
		blk       *types.Block
		blkStatus vmpb.Status
		blkID     ids.ID
		err       error
	)

	// Check if the block is already cached
	blkID, blkIDCached := vm.wrappedBlocks.BytesToIDCache.Get(string(req.Bytes))
	if !blkIDCached {
		blk, blkStatus, err = vmstate.DecodeBlockWithStatus(req.Bytes)
		if err != nil {
			vm.logger.Error("failed to decode block", "err", err)
			return nil, err
		}

		blkID, err = ids.ToID(blk.Hash())
		if err != nil {
			vm.logger.Error("failed to convert block hash to ID", "err", err)
			return nil, err
		}

		vm.wrappedBlocks.BytesToIDCache.Put(string(req.Bytes), blkID)
	}

	wblk, ok := vm.wrappedBlocks.GetCachedBlock(blkID)
	if !ok {
		wblk := &vmstate.WrappedBlock{
			Block:  blk,
			Status: blkStatus,
		}
		switch blkStatus {
		case vmpb.Status_STATUS_ACCEPTED, vmpb.Status_STATUS_REJECTED:
			vm.wrappedBlocks.DecidedBlocks.Put(blkID, wblk)
		case vmpb.Status_STATUS_PROCESSING:
			vm.wrappedBlocks.UnverifiedBlocks.Put(blkID, wblk)
		default:
			vm.logger.Error("found unexpected status for blk", "id", blkID, "status", blkStatus)
			return nil, fmt.Errorf("found unexpected status for blk %s: %s", blkID, blkStatus)
		}

		vm.wrappedBlocks.MissingBlocks.Evict(blkID)
	} else {
		blk = wblk.Block
		blkStatus = wblk.Status
	}

	return &vmpb.ParseBlockResponse{
		Id:                blk.Hash(),
		ParentId:          blk.LastBlockID.Hash,
		Status:            blkStatus,
		Height:            uint64(blk.Height),
		Timestamp:         timestamppb.New(blk.Time),
		VerifyWithContext: false,
	}, nil
}

// GetBlock attempt to load a block.
func (vm *LandslideVM) GetBlock(_ context.Context, req *vmpb.GetBlockRequest) (*vmpb.GetBlockResponse, error) {
	vm.logger.Info("GetBlock", "id", req.GetId())
	var (
		blk       *types.Block
		blkStatus vmpb.Status
	)

	blkID, err := ids.ToID(req.GetId())
	if err != nil {
		vm.logger.Error("failed to convert block hash to ID", "err", err)
		return nil, err
	}

	wblk, ok := vm.wrappedBlocks.GetCachedBlock(blkID)
	if !ok {
		if _, ok := vm.wrappedBlocks.MissingBlocks.Get(blkID); ok {
			return &vmpb.GetBlockResponse{
				Err: vmpb.Error_ERROR_NOT_FOUND,
			}, nil
		}

		blk = vm.blockStore.LoadBlockByHash(req.GetId())
		if blk == nil {
			vm.wrappedBlocks.MissingBlocks.Put(blkID, struct{}{})
			return &vmpb.GetBlockResponse{
				Err: vmpb.Error_ERROR_NOT_FOUND,
			}, nil
		}

		wblk = &vmstate.WrappedBlock{
			Block:  blk,
			Status: vmpb.Status_STATUS_ACCEPTED,
		}
	}

	blk = wblk.Block
	blkStatus = wblk.Status

	switch blkStatus {
	case vmpb.Status_STATUS_ACCEPTED, vmpb.Status_STATUS_REJECTED:
		vm.wrappedBlocks.DecidedBlocks.Put(blkID, wblk)
	case vmpb.Status_STATUS_PROCESSING:
		vm.wrappedBlocks.UnverifiedBlocks.Put(blkID, wblk)
	default:
		vm.logger.Error("found unexpected status for blk", "id", blkID, "status", blkStatus)
		return nil, fmt.Errorf("found unexpected status for blk %s: %s", blkID, blkStatus)
	}

	blockBytes, err := vmstate.EncodeBlockWithStatus(blk, blkStatus)
	if err != nil {
		vm.logger.Error("failed to encode block", "err", err)
		return nil, err
	}

	return &vmpb.GetBlockResponse{
		ParentId:  blk.LastBlockID.Hash,
		Bytes:     blockBytes,
		Status:    blkStatus,
		Height:    uint64(blk.Height),
		Timestamp: timestamppb.New(blk.Time),
	}, nil
}

// SetPreference notify the VM of the currently preferred block.
func (vm *LandslideVM) SetPreference(_ context.Context, req *vmpb.SetPreferenceRequest) (*emptypb.Empty, error) {
	vm.preferred = [32]byte(req.GetId())

	vm.logger.Debug("SetPreference", "id", req.GetId())
	return &emptypb.Empty{}, nil
}

// Health attempt to verify the health of the VM.
func (vm *LandslideVM) Health(ctx context.Context, in *emptypb.Empty) (*vmpb.HealthResponse, error) {
	dbHealth, err := vm.databaseClient.HealthCheck(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("failed to check db health: %w", err)
	}
	report := map[string]interface{}{
		"database": dbHealth,
	}

	details, err := json.Marshal(report)
	return &vmpb.HealthResponse{
		Details: details,
	}, err
}

// Version returns the version of the VM.
func (vm *LandslideVM) Version(context.Context, *emptypb.Empty) (*vmpb.VersionResponse, error) {
	return &vmpb.VersionResponse{
		Version: Version,
	}, nil
}

// AppRequest notify this engine of a request for data from [nodeID].
func (vm *LandslideVM) AppRequest(context.Context, *vmpb.AppRequestMsg) (*emptypb.Empty, error) {
	return nil, errors.New("TODO: implement me 3")
}

// AppRequestFailed notify this engine that an AppRequest message it sent to [nodeID] with
// request ID [requestID] failed.
func (vm *LandslideVM) AppRequestFailed(context.Context, *vmpb.AppRequestFailedMsg) (*emptypb.Empty, error) {
	return nil, errors.New("TODO: implement me 4")
}

// AppResponse notify this engine of a response to the AppRequest message it sent to
// [nodeID] with request ID [requestID].
func (vm *LandslideVM) AppResponse(context.Context, *vmpb.AppResponseMsg) (*emptypb.Empty, error) {
	return nil, errors.New("TODO: implement me 5")
}

// AppGossip notify this engine of a gossip message from [nodeID].
func (vm *LandslideVM) AppGossip(context.Context, *vmpb.AppGossipMsg) (*emptypb.Empty, error) {
	return nil, errors.New("TODO: implement me 6")
}

// Gather attempts to gather metrics from a VM.
func (vm *LandslideVM) Gather(context.Context, *emptypb.Empty) (*vmpb.GatherResponse, error) {
	// Gather metrics registered by rpcchainvm server Gatherer. These
	// metrics are collected for each Go plugin process.
	pluginMetrics, err := vm.processMetrics.Gather()
	if err != nil {
		return nil, err
	}

	return &vmpb.GatherResponse{MetricFamilies: pluginMetrics}, err
}

func (vm *LandslideVM) CrossChainAppRequest(context.Context, *vmpb.CrossChainAppRequestMsg) (*emptypb.Empty, error) {
	return nil, errors.New("TODO: implement me 8")
}

func (vm *LandslideVM) CrossChainAppRequestFailed(context.Context, *vmpb.CrossChainAppRequestFailedMsg) (*emptypb.Empty, error) {
	return nil, errors.New("TODO: implement me 9")
}

func (vm *LandslideVM) CrossChainAppResponse(context.Context, *vmpb.CrossChainAppResponseMsg) (*emptypb.Empty, error) {
	return nil, errors.New("TODO: implement me 10")
}

func (vm *LandslideVM) GetAncestors(context.Context, *vmpb.GetAncestorsRequest) (*vmpb.GetAncestorsResponse, error) {
	return nil, errors.New("TODO: implement me 11")
}

func (vm *LandslideVM) BatchedParseBlock(ctx context.Context, req *vmpb.BatchedParseBlockRequest) (*vmpb.BatchedParseBlockResponse, error) {
	vm.logger.Info("BatchedParseBlock")
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

func (vm *LandslideVM) GetBlockIDAtHeight(_ context.Context, req *vmpb.GetBlockIDAtHeightRequest) (*vmpb.GetBlockIDAtHeightResponse, error) {
	vm.logger.Info("GetBlockIDAtHeight")
	blk := vm.blockStore.LoadBlock(int64(req.GetHeight()))
	if blk == nil {
		return &vmpb.GetBlockIDAtHeightResponse{
			Err: vmpb.Error_ERROR_NOT_FOUND,
		}, nil
	}
	return &vmpb.GetBlockIDAtHeightResponse{BlkId: blk.Hash()}, nil
}

// StateSyncEnabled indicates whether the state sync is enabled for this VM.
func (vm *LandslideVM) StateSyncEnabled(context.Context, *emptypb.Empty) (*vmpb.StateSyncEnabledResponse, error) {
	vm.logger.Info("StateSyncEnabled")
	return &vmpb.StateSyncEnabledResponse{Enabled: vm.vmenabled.Get()}, nil
}

// GetOngoingSyncStateSummary returns an in-progress state summary if it exists.
func (vm *LandslideVM) GetOngoingSyncStateSummary(context.Context, *emptypb.Empty) (*vmpb.GetOngoingSyncStateSummaryResponse, error) {
	panic("ToDo: implement me 12")
}

// GetLastStateSummary returns the latest state summary.
func (vm *LandslideVM) GetLastStateSummary(context.Context, *emptypb.Empty) (*vmpb.GetLastStateSummaryResponse, error) {
	panic("ToDo: implement me 13")
}

// ParseStateSummary parses a state summary out of [summaryBytes].
func (vm *LandslideVM) ParseStateSummary(context.Context, *vmpb.ParseStateSummaryRequest) (*vmpb.ParseStateSummaryResponse, error) {
	return nil, errors.New("TODO: implement me 14")
}

// GetStateSummary retrieves the state summary that was generated at height
// [summaryHeight].
func (vm *LandslideVM) GetStateSummary(context.Context, *vmpb.GetStateSummaryRequest) (*vmpb.GetStateSummaryResponse, error) {
	return nil, errors.New("TODO: implement me 15")
}

func (vm *LandslideVM) BlockVerify(_ context.Context, req *vmpb.BlockVerifyRequest) (*vmpb.BlockVerifyResponse, error) {
	vm.logger.Info("BlockVerify")
	// vm.logger.Debug("block verify", "bytes", req.Bytes)

	blk, blkStatus, err := vmstate.DecodeBlockWithStatus(req.Bytes)
	if err != nil {
		vm.logger.Error("failed to decode block", "err", err)
		return nil, err
	}

	vm.logger.Info("ValidateBlock")
	err = vmstate.ValidateBlock(vm.state, blk)
	if err != nil {
		vm.logger.Error("failed to validate block", "err", err)
		return nil, err
	}

	blkID, err := ids.ToID(blk.Hash())
	if err != nil {
		vm.logger.Error("failed to convert block hash to ID", "err", err)
		return nil, err
	}

	vm.wrappedBlocks.UnverifiedBlocks.Evict(blkID)
	vm.wrappedBlocks.VerifiedBlocks[blkID] = &vmstate.WrappedBlock{
		Block:  blk,
		Status: blkStatus,
	}

	return &vmpb.BlockVerifyResponse{Timestamp: timestamppb.New(blk.Time)}, nil
}

// BlockAccept notifies the VM that a block has been accepted.
// This is a critical method and should not be exposed publicly.
func (vm *LandslideVM) BlockAccept(_ context.Context, req *vmpb.BlockAcceptRequest) (*emptypb.Empty, error) {
	vm.logger.Info("BlockAccept")

	blkID, err := ids.ToID(req.GetId())
	if err != nil {
		vm.logger.Error("failed to convert block hash to ID", "err", err)
		return nil, err
	}

	wblk, exist := vm.wrappedBlocks.GetCachedBlock(blkID)
	if !exist {
		return nil, ErrNotFound
	}

	executor := vmstate.NewBlockExecutor(
		vm.stateStore,
		vm.logger,
		vm.app.Consensus(),
		vm.mempool,
		vm.blockStore,
		vm.config.ConsensusParams.Block.MaxBytes,
		vm.config.ConsensusParams.Block.MaxGas,
		vm.config.ConsensusParams.Evidence.MaxBytes,
	)
	executor.SetEventBus(vm.eventBus)

	blk := wblk.Block
	bps, err := blk.MakePartSet(types.BlockPartSizeBytes)
	if err != nil {
		vm.logger.Error("failed to make part set", "err", err)
		return nil, err
	}
	blockID := types.BlockID{
		Hash:          blk.Hash(),
		PartSetHeader: bps.Header(),
	}

	newstate, err := executor.ApplyBlock(vm.state, blockID, blk)
	if err != nil {
		vm.logger.Error("failed to apply block", "err", err)
		return nil, err
	}
	vm.blockStore.SaveBlock(blk, bps, commit.MakeCommit(blk.Height, blk.Time, vm.state.Validators, blockID))

	err = vm.stateStore.Save(newstate)
	if err != nil {
		vm.logger.Error("failed to save state", "err", err)
		return nil, err
	}

	vm.state = newstate

	delete(vm.wrappedBlocks.VerifiedBlocks, blkID)
	vm.wrappedBlocks.MissingBlocks.Evict(blkID)
	vm.wrappedBlocks.UnverifiedBlocks.Evict(blkID)
	vm.wrappedBlocks.DecidedBlocks.Put(blkID, &vmstate.WrappedBlock{
		Block:  blk,
		Status: vmpb.Status_STATUS_ACCEPTED,
	})

	return &emptypb.Empty{}, nil
}

func (vm *LandslideVM) BlockReject(_ context.Context, req *vmpb.BlockRejectRequest) (*emptypb.Empty, error) {
	vm.logger.Info("BlockReject")
	blkID, err := ids.ToID(req.GetId())
	if err != nil {
		vm.logger.Error("failed to convert block hash to ID", "err", err)
		return nil, err
	}

	blk, exist := vm.wrappedBlocks.GetCachedBlock(blkID)
	if !exist {
		return nil, ErrNotFound
	}

	blk.Status = vmpb.Status_STATUS_REJECTED
	delete(vm.wrappedBlocks.VerifiedBlocks, blkID)
	vm.wrappedBlocks.DecidedBlocks.Put(blkID, blk)

	return &emptypb.Empty{}, nil
}

func (vm *LandslideVM) StateSummaryAccept(context.Context, *vmpb.StateSummaryAcceptRequest) (*vmpb.StateSummaryAcceptResponse, error) {
	return nil, errors.New("TODO: implement me 16")
}
