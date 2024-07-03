package vm

import (
	"context"
	_ "embed"
	"testing"

	dbm "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/abci/example/kvstore"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

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

func TestChainId(t *testing.T) {

	/*
		I[2024-07-03|15:28:06.054] chainIDRaw                                   chain=55E1FCFDDE01F9F6D4C16FA2ED89CE65A8669120A86F321EEF121891CAB61241
		I[2024-07-03|15:28:06.054] CChainId                                     chain=55E1FCFDDE01F9F6D4C16FA2ED89CE65A8669120A86F321EEF121891CAB61241
		I[2024-07-03|15:28:06.054] XChainId                                     chain=B573C177E55B1368EB620A3287408EB6BEA1E9B1197B7D06A5C4B57CF7D8726F
		I[2024-07-03|15:28:06.054] SubnetId                                     chain=D32CFAD24C07B14ADBE1DCEC4336CA0FFBF150B8099C98B38B69DDEF0FFBE344
		I[2024-07-03|15:28:06.054] NodeId                                       chain=F29BCE5F34A74301EB0DE716D5194E4A4AEA5D7A
		I[2024-07-03|15:28:06.054] ChainDataDir                                 chain=/tmp/e2e-test-landslide/nodes/node5/chainData/2DinS2RhV8APTEmyehwpToKhCQUDGbn3UyEdTvrZ6mXfNBZAPE
		I[2024-07-03|15:28:06.054] AvaxAssetId                                  chain=94B45AA6E4464A9AD3FC4E73C7947BA1632D7C820BAF0EBBFC7548C4823F26BD
	*/

	/*
		I[2024-07-03|15:30:09.921] chainIDRaw                                   chain=55E1FCFDDE01F9F6D4C16FA2ED89CE65A8669120A86F321EEF121891CAB61241
		I[2024-07-03|15:30:09.921] CChainId                                     chain=55E1FCFDDE01F9F6D4C16FA2ED89CE65A8669120A86F321EEF121891CAB61241
		I[2024-07-03|15:30:09.921] XChainId                                     chain=B573C177E55B1368EB620A3287408EB6BEA1E9B1197B7D06A5C4B57CF7D8726F
		I[2024-07-03|15:30:09.921] SubnetId                                     chain=D32CFAD24C07B14ADBE1DCEC4336CA0FFBF150B8099C98B38B69DDEF0FFBE344
		I[2024-07-03|15:30:09.921] NodeId                                       chain=F29BCE5F34A74301EB0DE716D5194E4A4AEA5D7A
		I[2024-07-03|15:30:09.921] ChainDataDir                                 chain=/tmp/e2e-test-landslide/nodes/node5/chainData/oXDez3R5zPPqZTykiTezYFiziFbSzLNzq7uZirs5HQssbQn2q
		I[2024-07-03|15:30:09.921] AvaxAssetId                                  chain=94B45AA6E4464A9AD3FC4E73C7947BA1632D7C820BAF0EBBFC7548C4823F26BD
	*/

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

// TestShutdownWithoutInit tests VM Shutdown function. This function called without Initialize in Avalanchego Factory
// https://github.com/ava-labs/avalanchego/blob/0c4efd743e1d737f4e8970d0e0ebf229ea44406c/vms/manager.go#L129
func TestShutdownWithoutInit(t *testing.T) {
	vmdb := dbm.NewMemDB()
	appdb := dbm.NewMemDB()
	mockConn := &mockClientConn{}
	vm := NewViaDB(vmdb, func(*AppCreatorOpts) (Application, error) {
		return kvstore.NewApplication(appdb), nil
	}, WithClientConn(mockConn))
	require.NotNil(t, vm)
	_, err := vm.Shutdown(context.Background(), &emptypb.Empty{})
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
