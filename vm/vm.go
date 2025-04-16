package vm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/landslidenetwork/slide-sdk/grpcutils/p2psender"
	appsenderpb "github.com/landslidenetwork/slide-sdk/proto/appsender"
	warppb "github.com/landslidenetwork/slide-sdk/proto/warp"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/common"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/network/acp118"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/network/p2p"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/timer/mockable"
	warputils "github.com/landslidenetwork/slide-sdk/utils/avalanche/warp"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/warp/gwarp"
	"github.com/landslidenetwork/slide-sdk/utils/evm/peer"
	evmvalidators "github.com/landslidenetwork/slide-sdk/utils/evm/validators"
	"github.com/landslidenetwork/slide-sdk/utils/evm/validators/interfaces"
	"github.com/landslidenetwork/slide-sdk/utils/evm/warp/aggregator"
	"github.com/landslidenetwork/slide-sdk/utils/message"
	http2 "net/http"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/landslidenetwork/slide-sdk/grpcutils/gvalidators"

	"github.com/landslidenetwork/slide-sdk/warp"

	dbm "github.com/cometbft/cometbft-db"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/consensus"
	"github.com/cometbft/cometbft/crypto"
	"github.com/cometbft/cometbft/crypto/secp256k1"
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
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/landslidenetwork/slide-sdk/database"
	"github.com/landslidenetwork/slide-sdk/grpcutils"
	"github.com/landslidenetwork/slide-sdk/http"
	"github.com/landslidenetwork/slide-sdk/jsonrpc"
	httppb "github.com/landslidenetwork/slide-sdk/proto/http"
	messengerpb "github.com/landslidenetwork/slide-sdk/proto/messenger"
	"github.com/landslidenetwork/slide-sdk/proto/rpcdb"
	validatorstatepb "github.com/landslidenetwork/slide-sdk/proto/validatorstate"
	vmpb "github.com/landslidenetwork/slide-sdk/proto/vm"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	vmtypes "github.com/landslidenetwork/slide-sdk/vm/types"
	"github.com/landslidenetwork/slide-sdk/vm/types/block"
	"github.com/landslidenetwork/slide-sdk/vm/types/closer"
	"github.com/landslidenetwork/slide-sdk/vm/types/commit"
	vmstate "github.com/landslidenetwork/slide-sdk/vm/types/state"
)

const (
	genesisChunkSize                     = 16 * 1024 * 1024 // 16
	requirePrimaryNetworkSigners         = true
	DefaultNetworkPeerListBloomResetFreq = time.Minute
	DefaultP2PPingFrequency              = time.Second
	// The network must be "tcp", "tcp4", "tcp6", "unix" or "unixpacket".
	NetworkType = "tcp"
)

var (
	_ vmpb.VMServer = (*LandslideVM)(nil)

	dbPrefixBlockStore       = []byte("block-store")
	dbPrefixStateStore       = []byte("state-store")
	dbPrefixValidatorManager = []byte("validator-manager")
	dbPrefixTxIndexer        = []byte("tx-indexer")
	dbPrefixBlockIndexer     = []byte("block-indexer")
	dbPrefixWarp             = []byte("warp")

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
		peer.Network
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

		vmenabled         *vmtypes.Atomic[bool]
		vmstate           *vmtypes.Atomic[vmpb.State]
		vmconnected       *vmtypes.Atomic[bool]
		verifiedBlocks    sync.Map
		validatorsManager interfaces.ValidatorReader
		preferred         [32]byte
		wrappedBlocks     *vmstate.WrappedBlocksStorage

		// Avalanche Warp Messaging backend
		// Used to serve BLS signatures of warp messages over RPC
		warpBackend      warp.Backend
		warpSignerClient warputils.Signer
		warpService      *API
		warpDB           dbm.DB

		p2pClient peer.NetworkClient

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
	grpcClientMetrics := grpcPrometheus.NewClientMetrics()
	if err := registerer.Register(grpcClientMetrics); err != nil {
		return nil, err
	}

	// Register metrics for each Go plugin processes
	vm.processMetrics = registerer

	vm.logger = log.NewTMLogger(os.Stdout)

	// add to connCloser even we have defined vm.clientConn via Option
	if vm.optClientConn != nil {
		vm.connCloser.Add(vm.optClientConn)
		vm.clientConn = vm.optClientConn
	} else {
		vm.logger.Info("Server Address initial:", req.ServerAddr)
		addrData := []byte(req.ServerAddr)
		err := os.WriteFile("/tmp/vm_server_address", addrData, 0644)
		if err != nil {
			vm.logger.Error("failed to write server address to file", "err", err)
			return nil, err
		}
		clientConn, err := grpc.NewClient(
			"passthrough:///"+req.ServerAddr,
			grpc.WithChainUnaryInterceptor(grpcClientMetrics.UnaryClientInterceptor()),
			grpc.WithChainStreamInterceptor(grpcClientMetrics.StreamClientInterceptor()),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			if closerErr := vm.connCloser.Close(); closerErr != nil {
				vm.logger.Error("failed to close connCloser", "err", err)
			}
			return nil, err
		}

		vm.connCloser.Add(clientConn)
		vm.clientConn = clientConn
	}

	msgClient := messengerpb.NewMessengerClient(vm.clientConn)

	validatorStateClient := gvalidators.NewClient(validatorstatepb.NewValidatorStateClient(vm.clientConn))

	vm.warpSignerClient = gwarp.NewClient(warppb.NewSignerClient(vm.clientConn))

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
		dbClientConn, err := grpc.NewClient(
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
	vm.logger.Info("MEMPOOL INITIALIZED")

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

	vm.logger.Info("ENCODE BLOCK WITH STATUS ACCEPTED")
	blockBytes, err := vmstate.EncodeBlockWithStatus(blk, vmpb.Status_STATUS_ACCEPTED)
	if err != nil {
		vm.logger.Info(fmt.Sprintf("failed to encode block with status ACCEPTED: %s", err))
		return nil, fmt.Errorf("failed to encode block with status ACCEPTED: %w", err)
	}
	// vm.logger.Debug("initialize block", "bytes ", blockBytes)
	//vm.logger.Info("vm initialization completed")

	parentHash := block.ParentHash(blk)

	vm.warpDB = dbm.NewPrefixDB(vm.database, dbPrefixWarp)
	// TODO: implement bls secret key check
	// if vm.config.BLSSecretKey == nil {
	//	if err != nil {
	//		return nil, err
	//	}
	// }
	chainID, err := ids.ToID(req.ChainId)
	if err != nil {
		vm.logger.Info(fmt.Sprintf("failed to parse chain ID: %s", err))
		return nil, fmt.Errorf("failed to parse chain ID: %w", err)
	}
	vm.logger.Info("BLS Public KEY:", req.PublicKey)
	//secretKey, err := bls.SecretKeyFromBytes(req.PublicKey)
	//if err != nil {
	//	vm.logger.Info(fmt.Sprintf("failed to parse BLS secret key: %s", err))
	//	return nil, fmt.Errorf("failed to parse BLS secret key: %w", err)
	//}
	//vm.warpSigner = warputils.NewSigner(secretKey, req.NetworkId, chainID)

	dbValidatorManager := dbm.NewPrefixDB(vm.database, dbPrefixValidatorManager)
	vm.validatorsManager, err = evmvalidators.NewManager(dbValidatorManager, &mockable.Clock{})
	if err != nil {
		vm.logger.Info(fmt.Sprintf("failed to create validators manager: %s", err))
		return nil, fmt.Errorf("failed to create validators manager: %w", err)
	}

	vm.warpBackend = warp.NewBackend(
		req.NetworkId,
		chainID,
		vm.warpSignerClient,
		vm.logger,
		vm.warpDB,
		vm,
		vm.validatorsManager,
	)

	subnetID, err := ids.ToID(req.SubnetId)
	if err != nil {
		vm.logger.Info(fmt.Sprintf("failed to parse subnet ID: %s", err))
		return nil, fmt.Errorf("failed to parse subnet ID: %w", err)
	}
	//TODO: exclude rpcClients and AddressBook
	rpcClients := make(map[ids.NodeID]warp.Client)
	//for id, nodeURI := range vm.config.AddressBook {
	//	nodeID, err := ids.ToNodeID([]byte(id))
	//	if err != nil {
	//		vm.logger.Info(fmt.Sprintf("failed to parse nodeID from AddressBook: %s", err))
	//		return nil, err
	//	}
	//	rpcClient, err := warp.NewClient(nodeURI, string(req.ChainId))
	//	if err != nil {
	//		vm.logger.Info(fmt.Sprintf("failed to create warp client from AddressBook: %s", err))
	//		return nil, err
	//	}
	//	rpcClients[nodeID] = rpcClient
	//}

	//nodeID, err := ids.ToNodeID(req.NodeId)
	//if err != nil {
	//	return nil, err
	//}

	//p2pRouter := &router.P2PRouter{}
	//validatorsManager := validators.NewManager()
	//threshold := 5
	//minimumFailingDuration := time.Second
	//duration := 2 * time.Second
	//maxPortion := math.Pi
	//nwBenchlist, err := benchlist.NewBenchlist(p2pRouter, validatorsManager, threshold, minimumFailingDuration, duration, maxPortion, registerer)
	//if err != nil {
	//	return nil, err
	//}
	//timeoutManager, err := timeout.NewManager(nwBenchlist)
	//if err != nil {
	//	return nil, err
	//}
	//err = p2pRouter.Initialize(vm.logger, timeoutManager)
	//if err != nil {
	//	return nil, err
	//}
	//maxMessageTimeout := time.Second
	//msgCreator, err := message.NewCreator(vm.logger, registerer, compression.TypeZstd, maxMessageTimeout)
	//if err != nil {
	//	return nil, err
	//}
	//// Passes messages from the snowman engines to the network
	//
	//var p2pConfig = &network2.Config{}
	//listenAddress := net.JoinHostPort(n.Config.ListenHost, strconv.FormatUint(uint64(n.Config.ListenPort), 10))
	//listener, err := net.Listen(NetworkType, listenAddress)
	//if err != nil {
	//	return nil, err
	//}
	//dialer := dialer2.NewDialer(NetworkType, dialer2.Config{}, vm.logger)
	//
	//tlsCert, err := staking.NewTLSCert()
	//if err != nil {
	//	return nil, err
	//}
	//
	//cert, err := staking.ParseCertificate(tlsCert.Leaf.Raw)
	//if err != nil {
	//	return nil, err
	//}
	//nodeID := ids.NodeIDFromCert(cert)
	//
	//blsKey, err := bls.NewSigner()
	//if err != nil {
	//	return nil, err
	//}
	//
	//p2pConfig = &defaultConfig
	//p2pConfig.TLSConfig = peer.TLSConfig(*tlsCert, nil)
	//p2pConfig.MyNodeID = nodeID
	//p2pConfig.MyIPPort = utils.NewAtomic(ip)
	//p2pConfig.TLSKey = tlsCert.PrivateKey.(crypto.Signer)
	//p2pConfig.BLSKey = blsKey
	//
	//externalSender, err := network2.NewNetwork(p2pConfig, InitiallyP2PActiveTime, msgCreator, vm.logger, listener, dialer, p2pRouter)
	//if err != nil {
	//	return nil, err
	//}
	//allowedNodes := set.Set[ids.NodeID]{}
	//for _, nodeID := range vm.config.P2PAllowedNodes {
	//	parsedNodeID, err := ids.NodeIDFromString(nodeID)
	//	if err != nil {
	//		return nil, err
	//	}
	//	allowedNodes.Add(parsedNodeID)
	//}
	//appSender := sender.New(chainID, subnetID, nodeID, vm.logger, timeoutManager, msgCreator, externalSender, p2pRouter, allowedNodes)

	//TODO: implement
	//appSenderClient := p2psender.NewClient(appsender.NewAppSenderClient(vm.clientConn))

	//// Passes messages from the avalanche engines to the network
	//avalancheMessageSender, err := sender.New(
	//	ctx,
	//	m.MsgCreator,
	//	m.Net,
	//	m.ManagerConfig.Router,
	//	m.TimeoutManager,
	//	p2ppb.EngineType_ENGINE_TYPE_AVALANCHE,
	//	sb,
	//	avalancheMetrics,
	//)

	var appSenderClient common.AppSender
	appSenderClientIfc := ctx.Value("appSender")
	if appSenderClientIfc != nil {
		appSenderClient = appSenderClientIfc.(common.AppSender)
	} else {
		appSenderClient = p2psender.NewClient(appsenderpb.NewAppSenderClient(vm.clientConn))
		vm.logger.Debug("Setup p2p communication with avalanche engine")
	}

	p2pNetwork, err := p2p.NewNetwork(
		vm.logger,
		appSenderClient,
		registerer,
		"p2p",
	)
	if err != nil {
		vm.logger.Info(fmt.Sprintf("failed to create p2p network: %s", err))
		return nil, fmt.Errorf("failed to create p2p network: %w", err)
	}
	networkCodec := message.Codec
	vm.Network = peer.NewNetwork(p2pNetwork, appSenderClient, vm.logger, 100, networkCodec)
	vm.p2pClient = peer.NewNetworkClient(vm.Network)
	signatureGetter := aggregator.NewSignatureGetter(vm.p2pClient)

	vm.warpService = NewAPI(vm, vm.logger, req.NetworkId, validatorStateClient, subnetID, chainID, vm.warpBackend, signatureGetter, rpcClients, requirePrimaryNetworkSigners)

	//// We allow all peers to request warp messaging signatures
	//signatureRequestVerifier := signatureRequestVerifier{
	//	stateLock: stateLock,
	//	state:     state,
	//}
	// Allow signing of all warp messages. This is not typically safe, but is
	// allowed for this example.
	acp118Handler := acp118.NewHandler(
		vm.warpBackend,
		vm.warpSignerClient,
	)
	if err := p2pNetwork.AddHandler(p2p.SignatureRequestHandlerID, acp118Handler); err != nil {
		vm.logger.Info(fmt.Sprintf("failed to add p2p handler: %s", err))
		return nil, fmt.Errorf("failed to add p2p handler: %w", err)
	}

	networkHandler := newNetworkHandler(vm.warpBackend, networkCodec, vm.logger)
	vm.Network.SetRequestHandler(networkHandler)

	vm.logger.Info("vm initialization completed")
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
	jsonrpc.RegisterRPCFuncs(mux, NewRPC(vm).Routes(), vm.logger)

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
				Signature:        crypto.CRandBytes(types.MaxSignatureSize),
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
func (vm *LandslideVM) AppRequest(ctx context.Context, msg *vmpb.AppRequestMsg) (*emptypb.Empty, error) {
	nodeId, err := ids.ToNodeID(msg.NodeId)
	if err != nil {
		return nil, err
	}
	err = vm.Network.AppRequest(ctx, nodeId, msg.RequestId, msg.Deadline.AsTime(), msg.Request)
	return nil, err
}

// AppRequestFailed notify this engine that an AppRequest message it sent to [nodeID] with
// request ID [requestID] failed.
func (vm *LandslideVM) AppRequestFailed(ctx context.Context, msg *vmpb.AppRequestFailedMsg) (*emptypb.Empty, error) {
	nodeId, err := ids.ToNodeID(msg.NodeId)
	if err != nil {
		return nil, err
	}
	err = vm.Network.AppRequestFailed(ctx, nodeId, msg.RequestId, &common.AppError{
		Code:    msg.ErrorCode,
		Message: msg.ErrorMessage,
	})
	return nil, err
}

// AppResponse notify this engine of a response to the AppRequest message it sent to
// [nodeID] with request ID [requestID].
func (vm *LandslideVM) AppResponse(ctx context.Context, msg *vmpb.AppResponseMsg) (*emptypb.Empty, error) {
	nodeId, err := ids.ToNodeID(msg.NodeId)
	if err != nil {
		return nil, err
	}
	err = vm.Network.AppResponse(ctx, nodeId, msg.RequestId, msg.Response)
	return nil, err
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
