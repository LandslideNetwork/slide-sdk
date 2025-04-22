package vm

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/network/p2p"
	"github.com/landslidenetwork/slide-sdk/utils/codec"
	"github.com/landslidenetwork/slide-sdk/utils/codec/linearcodec"
	"github.com/landslidenetwork/slide-sdk/utils/version"
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
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/common"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/engine/enginetest"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	evmmessage "github.com/landslidenetwork/slide-sdk/utils/message"
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
	vmCfg := vmtypes.Config{}
	vmCfg.VMConfig.SetDefaults()

	appSender := &enginetest.Sender{T: t}
	appSender.CantSendAppGossip = true
	appSender.SendAppGossipF = func(context.Context, common.SendConfig, []byte) error { return nil }

	cfg, err := json.Marshal(vmCfg)
	if err != nil {
		t.Fatalf("Failed to marshal vm config to json: %v", err)
	}
	type appSenderKey string
	key := appSenderKey("appSender")
	ctx := context.WithValue(context.Background(), key, appSender)
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

func buildCodec(t *testing.T, types ...interface{}) codec.Manager {
	lc := linearcodec.NewDefault()
	for _, typ := range types {
		assert.NoError(t, lc.RegisterType(typ))
	}

	codecManager := codec.NewManager(math.MaxInt, lc)
	return codecManager
}

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
	var lock sync.Mutex
	contactedNodes := make(map[ids.NodeID]struct{})

	requestMessage := evmmessage.BlockSignatureRequest{BlockID: ids.GenerateTestID()}

	totalRequests := 5000
	numCallsPerRequest := 1 // on sending response
	totalCalls := totalRequests * numCallsPerRequest

	requestWg := &sync.WaitGroup{}
	requestWg.Add(totalCalls)
	requestBytes, err := evmmessage.Codec.Marshal(requestMessage)
	assert.NoError(t, err)
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
	assert.Equal(t, totalCalls, int(atomic.LoadUint32(&callNum)))

	// ensure empty nodeID is not allowed
	_, err = vmLnd.p2pClient.SendAppRequest(context.Background(), ids.EmptyNodeID, []byte("hello there"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot send request to empty nodeID")
}

func TestP2PAppRequest(t *testing.T) {
	vm, _ := NewFreshKvApp(t)

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
}

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
