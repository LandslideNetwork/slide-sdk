package vm

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/cometbft/cometbft/libs/log"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/common"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/constants"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/engine/enginetest"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/message"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/network/p2p"
	"github.com/landslidenetwork/slide-sdk/utils/codec"
	"github.com/landslidenetwork/slide-sdk/utils/codec/linearcodec"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	evmmessage "github.com/landslidenetwork/slide-sdk/utils/message"
	"github.com/landslidenetwork/slide-sdk/utils/version"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
	"math"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	dbm "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/abci/example/kvstore"
	"github.com/cometbft/cometbft/libs/rand"
	vmtypes "github.com/landslidenetwork/slide-sdk/vm/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"

	vmpb "github.com/landslidenetwork/slide-sdk/proto/vm"
)

var (
	//go:embed testdata/genesis.json
	kvstorevmGenesis []byte
)

const bufSize = 1024 * 1024

var lis *bufconn.Listener

type HelloRequest struct {
	Message string `serialize:"true"`
}

func (h HelloRequest) Handle(ctx context.Context, nodeID ids.NodeID, requestID uint32, handler evmmessage.RequestHandler) ([]byte, error) {
	// casting is only necessary for test since RequestHandler does not implement anything at the moment
	return handler.(TestRequestHandler).HandleHelloRequest(ctx, nodeID, requestID, &h)
}

func (h HelloRequest) String() string {
	return fmt.Sprintf("HelloRequest(%s)", h.Message)
}

type GreetingRequest struct {
	Greeting string `serialize:"true"`
}

func (g GreetingRequest) Handle(ctx context.Context, nodeID ids.NodeID, requestID uint32, handler evmmessage.RequestHandler) ([]byte, error) {
	// casting is only necessary for test since RequestHandler does not implement anything at the moment
	return handler.(TestRequestHandler).HandleGreetingRequest(ctx, nodeID, requestID, &g)
}

func (g GreetingRequest) String() string {
	return fmt.Sprintf("GreetingRequest(%s)", g.Greeting)
}

type HelloResponse struct {
	Response string `serialize:"true"`
}

type GreetingResponse struct {
	Greet string `serialize:"true"`
}

type TestMessage struct {
	Message string `serialize:"true"`
}

func (t TestMessage) Handle(ctx context.Context, nodeID ids.NodeID, requestID uint32, handler evmmessage.RequestHandler) ([]byte, error) {
	return handler.(*testRequestHandler).handleTestRequest(ctx, nodeID, requestID, &t)
}

func (t TestMessage) String() string {
	return fmt.Sprintf("TestMessage(%s)", t.Message)
}

type TestRequestHandler interface {
	HandleHelloRequest(ctx context.Context, nodeID ids.NodeID, requestID uint32, request *HelloRequest) ([]byte, error)
	HandleGreetingRequest(ctx context.Context, nodeID ids.NodeID, requestID uint32, request *GreetingRequest) ([]byte, error)
}

type testRequestHandler struct {
	evmmessage.RequestHandler
	calls              uint32
	processingDuration time.Duration
	response           []byte
	err                error
}

func (r *testRequestHandler) handleTestRequest(ctx context.Context, _ ids.NodeID, _ uint32, _ *TestMessage) ([]byte, error) {
	r.calls++
	select {
	case <-time.After(r.processingDuration):
		break
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return r.response, r.err
}

func init() {
	lis = bufconn.Listen(bufSize)
	s := grpc.NewServer()
	// Register your server implementations here, e.g., pb.RegisterGreeterServer(s, &server{})
	go func() {
		if err := s.Serve(lis); err != nil {
			panic("Server exited with error: " + err.Error())
		}
	}()
}

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func newKvApp(t *testing.T, vmdb, appdb dbm.DB) (vmpb.VMServer, *enginetest.Sender) {
	mockConn, err := grpc.NewClient(
		"bufnet",
		grpc.WithContextDialer(bufDialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}

	vm := NewViaDB(vmdb, func(*AppCreatorOpts) (Application, error) {
		return kvstore.NewApplication(appdb), nil
	}, WithOptClientConn(mockConn))
	require.NotNil(t, vm)
	//sk, err := bls.NewSecretKey()
	//if err != nil {
	//	t.Fatalf("Failed to generate secret key: %v", err)
	//}
	//skBytes := bls.SecretKeyToBytes(sk)
	vmCfg := vmtypes.Config{}
	vmCfg.VMConfig.SetDefaults()

	//vmCfg.VMConfig.BLSSecretKey = skBytes

	appSender := &enginetest.Sender{T: t}
	appSender.CantSendAppGossip = true
	appSender.SendAppGossipF = func(context.Context, common.SendConfig, []byte) error { return nil }

	cfg, err := json.Marshal(vmCfg)
	if err != nil {
		t.Fatalf("Failed to marshal vm config to json: %v", err)
	}
	ctx := context.WithValue(context.Background(), "appSender", appSender)
	initRes, err := vm.Initialize(ctx, &vmpb.InitializeRequest{
		DbServerAddr: "inmemory",
		GenesisBytes: kvstorevmGenesis,
		ChainId:      rand.Bytes(32),
		SubnetId:     rand.Bytes(32),
		NodeId:       ids.GenerateTestNodeID().Bytes(),
		ConfigBytes:  cfg,
	})
	require.NoError(t, err)
	require.NotNil(t, initRes)
	require.Equal(t, initRes.Height, uint64(1))

	blockRes, err := vm.GetBlock(context.TODO(), &vmpb.GetBlockRequest{
		Id: initRes.LastAcceptedId,
	})
	require.NoError(t, err)
	require.NotNil(t, blockRes)
	require.NotEqual(t, blockRes.Err, vmpb.Error_ERROR_NOT_FOUND)

	return vm, appSender
}

func NewFreshKvApp(t *testing.T) (vmpb.VMServer, *enginetest.Sender) {
	vmdb := dbm.NewMemDB()
	appdb := dbm.NewMemDB()
	return newKvApp(t, vmdb, appdb)
}

func newMessageCreator(t *testing.T) message.Creator {
	t.Helper()

	mc, err := message.NewCreator(
		log.NewNopLogger(),
		prometheus.NewRegistry(),
		constants.DefaultNetworkCompressionType,
		10*time.Second,
	)
	require.NoError(t, err)

	return mc
}

func buildCodec(t *testing.T, types ...interface{}) codec.Manager {
	lc := linearcodec.NewDefault()
	for _, typ := range types {
		assert.NoError(t, lc.RegisterType(typ))
	}

	//assert.NoError(t, codecManager.Register(message.Version, c))
	codecManager := codec.NewManager(math.MaxInt, lc)
	return codecManager
}

// marshalStruct is a helper method used to marshal an object as `interface{}`
// so that the codec is able to include the TypeID in the resulting bytes
func marshalStruct(codec codec.Manager, obj interface{}) ([]byte, error) {
	return codec.Marshal(&obj)
}

//// GenesisVM creates a VM instance with the genesis test bytes and returns
//// the channel use to send messages to the engine, the VM, database manager,
//// and sender.
//// If [genesisJSON] is empty, defaults to using [genesisJSONLatest]
//func GenesisVM(t *testing.T,
//	finishBootstrapping bool,
//	genesisJSON string,
//	configJSON string,
//	upgradeJSON string,
//) (
//	*LandslideVM,
//	database.Database,
//	*enginetest.Sender,
//) {
//	vm := &LandslideVM{}
//	ctx, dbManager, genesisBytes, issuer, _ := setupGenesis(t, genesisJSON)
//	appSender := &enginetest.Sender{T: t}
//	appSender.CantSendAppGossip = true
//	appSender.SendAppGossipF = func(context.Context, common.SendConfig, []byte) error { return nil }
//	_, err := vm.Initialize(
//		context.Background(),
//		&vmpb.InitializeRequest{
//			NetworkId:    0,
//			SubnetId:     nil,
//			ChainId:      nil,
//			NodeId:       nil,
//			PublicKey:    nil,
//			XChainId:     nil,
//			CChainId:     nil,
//			AvaxAssetId:  nil,
//			ChainDataDir: "",
//			GenesisBytes: nil,
//			UpgradeBytes: nil,
//			ConfigBytes:  nil,
//			DbServerAddr: "",
//			ServerAddr:   "",
//		},
//		//ctx,
//		//dbManager,
//		//genesisBytes,
//		//[]byte(upgradeJSON),
//		//[]byte(configJSON),
//		//issuer,
//		//[]*commonEng.Fx{},
//		//appSender,
//	)
//	require.NoError(t, err, "error initializing GenesisVM")
//
//	if finishBootstrapping {
//		_, err = vm.SetState(context.Background(), &vmpb.SetStateRequest{State: vmpb.State_STATE_BOOTSTRAPPING})
//		require.NoError(t, err)
//		_, err = vm.SetState(context.Background(), &vmpb.SetStateRequest{State: vmpb.State_STATE_NORMAL_OP})
//		require.NoError(t, err)
//	}
//
//	return vm, dbManager, appSender
//}

func TestCreation(t *testing.T) {
	vm := New(func(*AppCreatorOpts) (Application, error) {
		return kvstore.NewInMemoryApplication(), nil
	})
	require.NotNil(t, vm)
}

func TestReCreation(t *testing.T) {
	vmdb := dbm.NewMemDB()
	appdb := dbm.NewMemDB()

	newKvApp(t, vmdb, appdb)
	newKvApp(t, vmdb, appdb)
}

func TestBuildBlock(t *testing.T) {
	vm, _ := NewFreshKvApp(t)

	buildRes1, err := vm.BuildBlock(context.Background(), &vmpb.BuildBlockRequest{})
	require.NoError(t, err)
	require.Equal(t, buildRes1.Height, uint64(2))

	buildRes2, err := vm.BuildBlock(context.Background(), &vmpb.BuildBlockRequest{})
	require.NoError(t, err)
	require.Equal(t, buildRes2.Height, uint64(2))
}

func TestRejectBlock(t *testing.T) {
	vm, _ := NewFreshKvApp(t)

	buildRes1, err := vm.BuildBlock(context.Background(), &vmpb.BuildBlockRequest{})
	require.NoError(t, err)
	require.Equal(t, buildRes1.Height, uint64(2))

	buildRes2, err := vm.BuildBlock(context.Background(), &vmpb.BuildBlockRequest{})
	require.NoError(t, err)
	require.Equal(t, buildRes2.Height, uint64(2))

	_, err = vm.BlockReject(context.Background(), &vmpb.BlockRejectRequest{
		Id: buildRes1.Id,
	})
	require.NoError(t, err)

	_, err = vm.BlockReject(context.Background(), &vmpb.BlockRejectRequest{
		Id: buildRes2.Id,
	})
	require.NoError(t, err)
}

func TestAcceptBlock(t *testing.T) {
	vm, _ := NewFreshKvApp(t)

	buildRes, err := vm.BuildBlock(context.Background(), &vmpb.BuildBlockRequest{})
	require.NoError(t, err)
	require.Equal(t, buildRes.Height, uint64(2))

	_, err = vm.BlockAccept(context.Background(), &vmpb.BlockAcceptRequest{
		Id: buildRes.GetId(),
	})
	require.NoError(t, err)
}

func TestRequestRequestsRoutingAndResponse(t *testing.T) {
	vm, _ := NewFreshKvApp(t)
	vmLnd := vm.(*LandslideVM)
	callNum := uint32(0)
	//senderWg := &sync.WaitGroup{}
	//var net Network
	var lock sync.Mutex
	contactedNodes := make(map[ids.NodeID]struct{})
	//sender := testAppSender{
	//	sendAppRequestFn: func(_ context.Context, nodes set.Set[ids.NodeID], requestID uint32, requestBytes []byte) error {
	//		nodeID, _ := nodes.Pop()
	//		lock.Lock()
	//		contactedNodes[nodeID] = struct{}{}
	//		lock.Unlock()
	//		senderWg.Add(1)
	//		go func() {
	//			defer senderWg.Done()
	//			//TODO: implement
	//			//if err := net.AppRequest(context.Background(), nodeID, requestID, time.Now().Add(5*time.Second), requestBytes); err != nil {
	//			//	panic(err)
	//			//}
	//		}()
	//		return nil
	//	},
	//	sendAppResponseFn: func(nodeID ids.NodeID, requestID uint32, responseBytes []byte) error {
	//		senderWg.Add(1)
	//		go func() {
	//			defer senderWg.Done()
	//			//TODO: implement
	//			//if err := net.AppResponse(context.Background(), nodeID, requestID, responseBytes); err != nil {
	//			//	panic(err)
	//			//}
	//			atomic.AddUint32(&callNum, 1)
	//		}()
	//		return nil
	//	},
	//}

	//codecManager := buildCodec(t, sdk.SignatureRequest{})
	//p2pNetwork, err := p2p.NewNetwork(log.NewNopLogger(), nil, prometheus.NewRegistry(), "")
	//require.NoError(t, err)
	//net = NewNetwork(p2pNetwork, sender, log.NewNopLogger(), 16)
	////TODO: implement if necessary
	////net.SetRequestHandler(&HelloGreetingRequestHandler{codec: codecManager})
	//client := peer.NewNetworkClient(vmLnd.Network)

	//nodes := []ids.NodeID{
	//	ids.GenerateTestNodeID(),
	//	ids.GenerateTestNodeID(),
	//	ids.GenerateTestNodeID(),
	//	ids.GenerateTestNodeID(),
	//	ids.GenerateTestNodeID(),
	//}
	//for _, nodeID := range nodes {
	//	assert.NoError(t, net.Connected(context.Background(), nodeID, defaultPeerVersion))
	//}

	requestMessage := evmmessage.BlockSignatureRequest{BlockID: ids.GenerateTestID()}
	//defer net.Shutdown()

	totalRequests := 5000
	numCallsPerRequest := 1 // on sending response
	totalCalls := totalRequests * numCallsPerRequest

	requestWg := &sync.WaitGroup{}
	requestWg.Add(totalCalls)
	//nodeIdx := 0
	////for i := 0; i < totalCalls; i++ {
	////for i := 0; i == 0; i++ {
	//	nodeIdx = (nodeIdx + 1) % (len(nodes))
	//	nodeID := nodes[nodeIdx]
	//go func(wg *sync.WaitGroup, nodeID ids.NodeID) {
	//	defer wg.Done()
	//requestBytes, err := evmmessage.RequestToBytes(evmmessage.Codec, requestMessage)
	requestBytes, err := evmmessage.Codec.Marshal(requestMessage)
	assert.NoError(t, err)
	//nodeID, err := ids.ToNodeID(vmLnd.appOpts.NodeID)
	nodeID := ids.GenerateTestNodeID()
	responseBytes, err := vmLnd.p2pClient.SendAppRequest(context.Background(), nodeID, requestBytes)
	assert.NoError(t, err)
	assert.NotNil(t, responseBytes)

	var response evmmessage.SignatureResponse
	if err = evmmessage.Codec.Unmarshal(responseBytes, &response); err != nil {
		panic(fmt.Errorf("unexpected error during unmarshal: %w", err))
	}
	assert.Equal(t, "signature", response.Signature)
	lock.Lock()
	contactedNodes[nodeID] = struct{}{}
	lock.Unlock()
	//}(requestWg, nodeID)
	//}
	//
	//requestWg.Wait()
	//senderWg.Wait()
	assert.Equal(t, totalCalls, int(atomic.LoadUint32(&callNum)))
	//for _, nodeID := range nodes {
	//	if _, exists := contactedNodes[nodeID]; !exists {
	//		t.Fatalf("expected nodeID %s to be contacted but was not", nodeID)
	//	}
	//}

	// ensure empty nodeID is not allowed
	_, err = vmLnd.p2pClient.SendAppRequest(context.Background(), ids.EmptyNodeID, []byte("hello there"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot send request to empty nodeID")
}

func TestP2PAppRequest(t *testing.T) {
	vm, _ := NewFreshKvApp(t)

	//mc := newMessageCreator(t)
	//nodeId, err := ids.ToID([]byte(rand.Str(20)))
	//require.NoError(t, err)
	//outboundAppRequestMsg, err := mc.AppRequest(nodeId, 1, 5*time.Minute, []byte("content"))

	codecManager := buildCodec(t, evmmessage.BlockSignatureRequest{})

	nodeID := ids.GenerateTestNodeID()
	_, err := vm.Connected(context.Background(), &vmpb.ConnectedRequest{
		NodeId: nodeID.Bytes(),
		Major:  uint32(version.CurrentApp.Major),
		Minor:  uint32(version.CurrentApp.Minor),
		Patch:  uint32(version.CurrentApp.Patch),
	})
	require.NoError(t, err)

	blkSignatureRequest := evmmessage.BlockSignatureRequest{BlockID: ids.GenerateTestID()}

	protocolAppRequestBytes, err := evmmessage.RequestToBytes(codecManager, blkSignatureRequest)
	require.NoError(t, err)

	appRequestBytes := p2p.PrefixMessage(
		p2p.ProtocolPrefix(p2p.SignatureRequestHandlerID),
		protocolAppRequestBytes,
	)

	appRequestRes, err := vm.AppRequest(context.Background(), &vmpb.AppRequestMsg{
		NodeId:    nodeID.Bytes(),
		RequestId: 1,
		Deadline:  timestamppb.New(time.Now().Add(5 * time.Minute)),
		Request:   appRequestBytes,
	})
	require.NoError(t, err)
	t.Log(appRequestRes.String())
	//require.Equal(t, appRequestRes.Height, uint64(2))
	//
	//_, err = vm.BlockAccept(context.Background(), &vmpb.BlockAcceptRequest{
	//	Id: buildRes.GetId(),
	//})
	//require.NoError(t, err)
}

//func TestBlockSignatureRequestsToVM(t *testing.T) {
//	vm, _ := NewFreshKvApp(t)
//	vmLnd := vm.(*LandslideVM)
//
//	defer func() {
//		_, err := vm.Shutdown(context.Background(), &emptypb.Empty{})
//		require.NoError(t, err)
//	}()
//
//	lastAcceptedID, err := vmLnd.GetBlockIDAtHeight(context.Background(), &vmpb.GetBlockIDAtHeightRequest{Height: uint64(vmLnd.state.LastBlockHeight)})
//	require.NoError(t, err)
//
//	blkId, err := ids.ToID(lastAcceptedID.BlkId)
//	require.NoError(t, err)
//	signature, err := vmLnd.warpBackend.GetBlockSignature(blkId)
//	require.NoError(t, err)
//	var knownSignature [bls.SignatureLen]byte
//	copy(knownSignature[:], signature)
//
//	tests := map[string]struct {
//		blockID          ids.ID
//		expectedResponse [bls.SignatureLen]byte
//	}{
//		"known": {
//			blockID:          blkId,
//			expectedResponse: knownSignature,
//		},
//		"unknown": {
//			blockID:          ids.GenerateTestID(),
//			expectedResponse: [bls.SignatureLen]byte{},
//		},
//	}
//
//	for name, test := range tests {
//		calledSendAppResponseFn := false
//		//appSender.SendAppResponseF = func(ctx context.Context, nodeID ids.NodeID, requestID uint32, responseBytes []byte) error {
//		//	calledSendAppResponseFn = true
//		//	var response evmmessage.SignatureResponse
//		//	err := message.Codec.Unmarshal(responseBytes, &response)
//		//	require.NoError(t, err)
//		//	require.Equal(t, test.expectedResponse, response.Signature)
//		//
//		//	return nil
//		//}
//		t.Run(name, func(t *testing.T) {
//			var signatureRequest evmmessage.Request = evmmessage.BlockSignatureRequest{
//				BlockID: test.blockID,
//			}
//
//			requestBytes, err := evmmessage.Codec.Marshal(&signatureRequest)
//			require.NoError(t, err)
//
//			// Send the app request and make sure we called SendAppResponseFn
//			deadline := time.Now().Add(60 * time.Second)
//			err = vmLnd.Network.AppRequest(context.Background(), ids.GenerateTestNodeID(), 1, deadline, requestBytes)
//			require.NoError(t, err)
//			require.True(t, calledSendAppResponseFn)
//		})
//	}
//}

// TestShutdownWithoutInit tests VM Shutdown function. This function called without Initialize in Avalanchego Factory
// https://github.com/ava-labs/avalanchego/blob/0c4efd743e1d737f4e8970d0e0ebf229ea44406c/vms/manager.go#L129
func TestShutdownWithoutInit(t *testing.T) {
	vmdb := dbm.NewMemDB()
	appdb := dbm.NewMemDB()

	mockConn, err := grpc.NewClient(
		"bufnet",
		grpc.WithContextDialer(bufDialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}

	vm := NewViaDB(vmdb, func(*AppCreatorOpts) (Application, error) {
		return kvstore.NewApplication(appdb), nil
	}, WithOptClientConn(mockConn))
	require.NotNil(t, vm)
	_, err = vm.Shutdown(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
}

// allowShutdown should be false by default https://github.com/ava-labs/avalanchego/blob/c8a5d0b11bcfe8b8a74983a9b0ef04fc68e78cf3/vms/rpcchainvm/vm.go#L40
func TestAllowShutdown(t *testing.T) {
	vm, _ := NewFreshKvApp(t)
	vmLnd := vm.(*LandslideVM)

	require.False(t, vmLnd.CanShutdown())

	_, err := vm.Shutdown(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)

	require.True(t, vmLnd.CanShutdown())
}
