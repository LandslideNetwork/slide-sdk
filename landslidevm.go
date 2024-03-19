package landslidevm

import (
	"context"
	"encoding/base64"
	"fmt"
	"math"
	"net"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	dbm "github.com/cometbft/cometbft-db"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/consensus"
	"github.com/cometbft/cometbft/crypto/secp256k1"
	"github.com/cometbft/cometbft/crypto/tmhash"
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
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/consideritdone/landslidevm/proto/rpcdb"
	vmpb "github.com/consideritdone/landslidevm/proto/vm"
	runtimepb "github.com/consideritdone/landslidevm/proto/vm/runtime"
	"github.com/consideritdone/landslidevm/utils"
	"github.com/consideritdone/landslidevm/utils/database"
	"github.com/consideritdone/landslidevm/utils/wrappers"
)

const (
	// Address of the runtime engine server.
	EngineAddressKey = "AVALANCHE_VM_RUNTIME_ENGINE_ADDR"
	// MinTime is the minimum amount of time a client should wait before sending
	// a keepalive ping. grpc-go default 5 mins
	defaultServerKeepAliveMinTime = 5 * time.Second
	// After a duration of this time if the server doesn't see any activity it
	// pings the client to see if the transport is still alive.
	// If set below 1s, a minimum value of 1s will be used instead.
	// grpc-go default 2h
	defaultServerKeepAliveInterval = 2 * time.Hour
	// After having pinged for keepalive check, the server waits for a duration
	// of Timeout and if no activity is seen even after that the connection is
	// closed. grpc-go default 20s
	defaultServerKeepAliveTimeout = 20 * time.Second
	// If true, client sends keepalive pings even with no active RPCs. If false,
	// when there are no active RPCs, Time and Timeout will be ignored and no
	// keepalive pings will be sent. grpc-go default false
	defaultPermitWithoutStream = true
	//
	defaultRuntimeDialTimeout = 5 * time.Second
	// rpcChainVMProtocol should be bumped anytime changes are made which
	// require the plugin vm to upgrade to latest avalanchego release to be
	// compatible.
	rpcChainVMProtocol uint = 34

	genesisChunkSize = 16 * 1024 * 1024 // 16
)

var (
	_ vmpb.VMServer = (*LandslideVM)(nil)

	DefaultServerOptions = []grpc.ServerOption{
		grpc.MaxRecvMsgSize(math.MaxInt),
		grpc.MaxSendMsgSize(math.MaxInt),
		grpc.MaxConcurrentStreams(math.MaxUint32),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             defaultServerKeepAliveMinTime,
			PermitWithoutStream: defaultPermitWithoutStream,
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    defaultServerKeepAliveInterval,
			Timeout: defaultServerKeepAliveTimeout,
		}),
	}

	dbPrefixBlockStore   = []byte("block-store")
	dbPrefixStateStore   = []byte("state-store")
	dbPrefixTxIndexer    = []byte("tx-indexer")
	dbPrefixBlockIndexer = []byte("block-indexer")

	proposerAddress = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	proposerPubKey  = secp256k1.PubKey{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
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
		allowShutdown atomic.Bool

		processMetrics prometheus.Gatherer
		serverCloser   wrappers.ServerCloser
		connCloser     wrappers.Closer

		database   dbm.DB
		appCreator AppCreator
		app        proxy.AppConns
		logger     log.Logger
		rpcConfig  *config.RPCConfig

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
	}
)

func NewLocalAppCreator(app Application) AppCreator {
	return func(*AppCreatorOpts) (Application, error) {
		return app, nil
	}
}

func New(creator AppCreator) *LandslideVM {
	return &LandslideVM{appCreator: creator}
}

func NewLocalGenesisDocProvider(data []byte) node.GenesisDocProvider {
	return func() (*types.GenesisDoc, error) {
		return types.GenesisDocFromJSON(data)
	}
}

func Serve[T interface{ AppCreator | *LandslideVM }](ctx context.Context, subject T, options ...grpc.ServerOption) error {
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)

	var vm *LandslideVM
	switch v := any(subject).(type) {
	case AppCreator:
		vm = New(v)
	case *LandslideVM:
		vm = v
	}

	if len(options) > 0 {
		options = DefaultServerOptions
	}
	server := grpc.NewServer(options...)
	vmpb.RegisterVMServer(server, vm)

	health := health.NewServer()
	health.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(server, health)

	go func(ctx context.Context) {
		defer func() {
			server.GracefulStop()
			fmt.Println("vm server: graceful termination success")
		}()

		for {
			select {
			case s := <-signals:
				// We drop all signals until our parent process has notified us
				// that we are shutting down. Once we are in the shutdown
				// workflow, we will gracefully exit upon receiving a SIGTERM.
				if !vm.CanShutdown() {
					fmt.Printf("runtime engine: ignoring signal: %s\n", s)
					continue
				}

				switch s {
				case syscall.SIGINT:
					fmt.Printf("runtime engine: ignoring signal: %s\n", s)
				case syscall.SIGTERM:
					fmt.Printf("runtime engine: received shutdown signal: %s\n", s)
					return
				}
			case <-ctx.Done():
				fmt.Println("runtime engine: context has been cancelled")
				return
			}
		}
	}(ctx)

	runtimeAddr, runtimeAddrExist := os.LookupEnv(EngineAddressKey)
	if !runtimeAddrExist {
		return fmt.Errorf("required env var missing: %q", EngineAddressKey)
	}

	clientConn, err := grpc.Dial("passthrough:///" + runtimeAddr)
	if err != nil {
		return fmt.Errorf("failed to create client conn: %w", err)
	}

	client := runtimepb.NewRuntimeClient(clientConn)
	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		return fmt.Errorf("failed to create new listener: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, defaultRuntimeDialTimeout)
	defer cancel()

	_, err = client.Initialize(ctx, &runtimepb.InitializeRequest{
		ProtocolVersion: uint32(rpcChainVMProtocol),
		Addr:            listener.Addr().String(),
	})
	if err != nil {
		_ = listener.Close()
		return fmt.Errorf("failed to initialize vm runtime: %w", err)
	}

	_ = server.Serve(listener)
	_ = listener.Close()
	return nil
}

// Initialize this VM.
func (vm *LandslideVM) Initialize(ctx context.Context, req *vmpb.InitializeRequest) (*vmpb.InitializeResponse, error) {
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
	grpcClientMetrics := grpc_prometheus.NewClientMetrics()
	if err := registerer.Register(grpcClientMetrics); err != nil {
		return nil, err
	}

	// Register metrics for each Go plugin processes
	vm.processMetrics = registerer

	// Dial the database
	dbClientConn, err := grpc.Dial(
		"passthrough:///"+req.DbServerAddr,
		grpc.WithChainUnaryInterceptor(grpcClientMetrics.UnaryClientInterceptor()),
		grpc.WithChainStreamInterceptor(grpcClientMetrics.StreamClientInterceptor()),
	)
	if err != nil {
		return nil, err
	}
	vm.connCloser.Add(dbClientConn)
	vm.database = database.NewDB(rpcdb.NewDatabaseClient(dbClientConn))
	vm.logger = log.NewTMLogger(os.Stdout)

	dbBlockStore := dbm.NewPrefixDB(vm.database, dbPrefixBlockStore)
	vm.blockStore = store.NewBlockStore(dbBlockStore)
	dbStateStore := dbm.NewPrefixDB(vm.database, dbPrefixStateStore)
	vm.stateStore = state.NewStore(dbStateStore, state.StoreOptions{DiscardABCIResponses: false})

	app, err := vm.appCreator(&AppCreatorOpts{
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
	})
	if err != nil {
		return nil, err
	}

	vm.state, vm.genesis, err = node.LoadStateFromDBOrGenesisDocProvider(
		dbStateStore,
		NewLocalGenesisDocProvider(req.GenesisBytes),
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

	if vm.state.LastBlockHeight == 0 {
		block := vm.state.MakeBlock(1, make([]types.Tx, 0), utils.MakeCommit(1, time.Now(), proposerAddress), nil, proposerAddress)
		block.LastBlockID = types.BlockID{
			Hash: tmhash.Sum([]byte{}),
			PartSetHeader: types.PartSetHeader{
				Total: 0,
				Hash:  tmhash.Sum([]byte{}),
			},
		}
		panic("ToDo: accept first block")
	}

	vm.logger.Info("vm initialization completed")
	panic("ToDo: implement return response")
}

// SetState communicates to VM its next state it starts
func (vm *LandslideVM) SetState(context.Context, *vmpb.SetStateRequest) (*vmpb.SetStateResponse, error) {
	panic("ToDo: implement me")
}

// CanShutdown lets known when vm ready to shutting down
func (vm *LandslideVM) CanShutdown() bool {
	return vm.allowShutdown.Load()
}

// Shutdown is called when the node is shutting down.
func (vm *LandslideVM) Shutdown(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// Creates the HTTP handlers for custom chain network calls.
func (vm *LandslideVM) CreateHandlers(context.Context, *emptypb.Empty) (*vmpb.CreateHandlersResponse, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) Connected(context.Context, *vmpb.ConnectedRequest) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) Disconnected(context.Context, *vmpb.DisconnectedRequest) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// Attempt to create a new block from data contained in the VM.
func (vm *LandslideVM) BuildBlock(context.Context, *vmpb.BuildBlockRequest) (*vmpb.BuildBlockResponse, error) {
	panic("ToDo: implement me")
}

// Attempt to create a block from a stream of bytes.
func (vm *LandslideVM) ParseBlock(context.Context, *vmpb.ParseBlockRequest) (*vmpb.ParseBlockResponse, error) {
	panic("ToDo: implement me")
}

// Attempt to load a block.
func (vm *LandslideVM) GetBlock(context.Context, *vmpb.GetBlockRequest) (*vmpb.GetBlockResponse, error) {
	panic("ToDo: implement me")
}

// Notify the VM of the currently preferred block.
func (vm *LandslideVM) SetPreference(context.Context, *vmpb.SetPreferenceRequest) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// Attempt to verify the health of the VM.
func (vm *LandslideVM) Health(context.Context, *emptypb.Empty) (*vmpb.HealthResponse, error) {
	panic("ToDo: implement me")
}

// Version returns the version of the VM.
func (vm *LandslideVM) Version(context.Context, *emptypb.Empty) (*vmpb.VersionResponse, error) {
	panic("ToDo: implement me")
}

// Notify this engine of a request for data from [nodeID].
func (vm *LandslideVM) AppRequest(context.Context, *vmpb.AppRequestMsg) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// Notify this engine that an AppRequest message it sent to [nodeID] with
// request ID [requestID] failed.
func (vm *LandslideVM) AppRequestFailed(context.Context, *vmpb.AppRequestFailedMsg) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// Notify this engine of a response to the AppRequest message it sent to
// [nodeID] with request ID [requestID].
func (vm *LandslideVM) AppResponse(context.Context, *vmpb.AppResponseMsg) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// Notify this engine of a gossip message from [nodeID].
func (vm *LandslideVM) AppGossip(context.Context, *vmpb.AppGossipMsg) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// Attempts to gather metrics from a VM.
func (vm *LandslideVM) Gather(context.Context, *emptypb.Empty) (*vmpb.GatherResponse, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) CrossChainAppRequest(context.Context, *vmpb.CrossChainAppRequestMsg) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) CrossChainAppRequestFailed(context.Context, *vmpb.CrossChainAppRequestFailedMsg) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) CrossChainAppResponse(context.Context, *vmpb.CrossChainAppResponseMsg) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// BatchedChainVM
func (vm *LandslideVM) GetAncestors(context.Context, *vmpb.GetAncestorsRequest) (*vmpb.GetAncestorsResponse, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) BatchedParseBlock(context.Context, *vmpb.BatchedParseBlockRequest) (*vmpb.BatchedParseBlockResponse, error) {
	panic("ToDo: implement me")
}

// HeightIndexedChainVM
func (vm *LandslideVM) VerifyHeightIndex(context.Context, *emptypb.Empty) (*vmpb.VerifyHeightIndexResponse, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) GetBlockIDAtHeight(context.Context, *vmpb.GetBlockIDAtHeightRequest) (*vmpb.GetBlockIDAtHeightResponse, error) {
	panic("ToDo: implement me")
}

// StateSyncableVM
//
// StateSyncEnabled indicates whether the state sync is enabled for this VM.
func (vm *LandslideVM) StateSyncEnabled(context.Context, *emptypb.Empty) (*vmpb.StateSyncEnabledResponse, error) {
	panic("ToDo: implement me")
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
	panic("ToDo: implement me")
}

// GetStateSummary retrieves the state summary that was generated at height
// [summaryHeight].
func (vm *LandslideVM) GetStateSummary(context.Context, *vmpb.GetStateSummaryRequest) (*vmpb.GetStateSummaryResponse, error) {
	panic("ToDo: implement me")
}

// Block
func (vm *LandslideVM) BlockVerify(context.Context, *vmpb.BlockVerifyRequest) (*vmpb.BlockVerifyResponse, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) BlockAccept(context.Context, *vmpb.BlockAcceptRequest) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) BlockReject(context.Context, *vmpb.BlockRejectRequest) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// StateSummary
func (vm *LandslideVM) StateSummaryAccept(context.Context, *vmpb.StateSummaryAcceptRequest) (*vmpb.StateSummaryAcceptResponse, error) {
	panic("ToDo: implement me")
}
