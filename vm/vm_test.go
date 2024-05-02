package vm

import (
	"context"
	_ "embed"
	"testing"

	dbm "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/abci/example/kvstore"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	vmpb "github.com/consideritdone/landslidevm/proto/vm"
)

var (
	//go:embed testdata/genesis.json
	kvstorevmGenesis []byte
)

type mockClientConn struct {
}

func (m *mockClientConn) Invoke(ctx context.Context, method string, args any, reply any, opts ...grpc.CallOption) error {
	return nil
}

func (m *mockClientConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, nil
}

func newKvApp(t *testing.T, vmdb, appdb dbm.DB) vmpb.VMServer {
	mockConn := &mockClientConn{}
	vm := NewViaDB(vmdb, func(*AppCreatorOpts) (Application, error) {
		return kvstore.NewApplication(appdb), nil
	}, WithClientConn(mockConn))
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
