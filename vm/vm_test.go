package vm

import (
	"context"
	_ "embed"
	"net"
	"testing"

	dbm "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/abci/example/kvstore"
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

func newKvApp(t *testing.T, vmdb, appdb dbm.DB) vmpb.VMServer {
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
	initRes, err := vm.Initialize(context.TODO(), &vmpb.InitializeRequest{
		DbServerAddr: "inmemory",
		GenesisBytes: kvstorevmGenesis,
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

	return vm
}

func newFreshKvApp(t *testing.T) vmpb.VMServer {
	vmdb := dbm.NewMemDB()
	appdb := dbm.NewMemDB()
	return newKvApp(t, vmdb, appdb)
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
	vm := newFreshKvApp(t)

	buildRes1, err := vm.BuildBlock(context.Background(), &vmpb.BuildBlockRequest{})
	require.NoError(t, err)
	require.Equal(t, buildRes1.Height, uint64(2))

	buildRes2, err := vm.BuildBlock(context.Background(), &vmpb.BuildBlockRequest{})
	require.NoError(t, err)
	require.Equal(t, buildRes2.Height, uint64(2))
}

func TestRejectBlock(t *testing.T) {
	vm := newFreshKvApp(t)

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
	vm := newFreshKvApp(t)

	buildRes, err := vm.BuildBlock(context.Background(), &vmpb.BuildBlockRequest{})
	require.NoError(t, err)
	require.Equal(t, buildRes.Height, uint64(2))

	_, err = vm.BlockAccept(context.Background(), &vmpb.BlockAcceptRequest{
		Id: buildRes.GetId(),
	})
	require.NoError(t, err)
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
	vm := newFreshKvApp(t)
	vmLnd := vm.(*LandslideVM)

	require.False(t, vmLnd.CanShutdown())

	_, err := vm.Shutdown(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)

	require.True(t, vmLnd.CanShutdown())
}
