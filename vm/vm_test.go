package vm

import (
	"context"
	_ "embed"
	"testing"

	dbm "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/abci/example/kvstore"
	vmpb "github.com/consideritdone/landslidevm/proto/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	//go:embed testdata/genesis.json
	countervmGenesis []byte
)

func newKvApp(t *testing.T, vmdb, appdb dbm.DB) vmpb.VMServer {
	vm := NewViaDB(vmdb, func(*AppCreatorOpts) (Application, error) {
		return kvstore.NewApplication(appdb), nil
	})
	require.NotNil(t, vm)
	initRes, err := vm.Initialize(context.TODO(), &vmpb.InitializeRequest{
		DbServerAddr: "inmemory",
		GenesisBytes: countervmGenesis,
	})
	assert.NoError(t, err)
	assert.NotNil(t, initRes)
	assert.Equal(t, initRes.Height, uint64(1))

	blockRes, err := vm.GetBlock(context.TODO(), &vmpb.GetBlockRequest{
		Id: initRes.LastAcceptedId,
	})
	assert.NoError(t, err)
	assert.NotNil(t, blockRes)
	assert.NotEqual(t, blockRes.Err, vmpb.Error_ERROR_NOT_FOUND)

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
	assert.Equal(t, buildRes1.Height, uint64(2))

	buildRes2, err := vm.BuildBlock(context.Background(), &vmpb.BuildBlockRequest{})
	require.NoError(t, err)
	assert.Equal(t, buildRes2.Height, uint64(2))
}

func TestRejectBlock(t *testing.T) {
	vm := newFreshKvApp(t)

	buildRes1, err := vm.BuildBlock(context.Background(), &vmpb.BuildBlockRequest{})
	require.NoError(t, err)
	assert.Equal(t, buildRes1.Height, uint64(2))

	buildRes2, err := vm.BuildBlock(context.Background(), &vmpb.BuildBlockRequest{})
	require.NoError(t, err)
	assert.Equal(t, buildRes2.Height, uint64(2))

	_, err = vm.BlockReject(context.Background(), &vmpb.BlockRejectRequest{
		Id: buildRes1.Id,
	})
	assert.NoError(t, err)

	_, err = vm.BlockReject(context.Background(), &vmpb.BlockRejectRequest{
		Id: buildRes2.Id,
	})
	assert.NoError(t, err)
}

func TestAcceptBlock(t *testing.T) {
	vm := newFreshKvApp(t)

	buildRes, err := vm.BuildBlock(context.Background(), &vmpb.BuildBlockRequest{})
	require.NoError(t, err)
	assert.Equal(t, buildRes.Height, uint64(2))

	_, err = vm.BlockAccept(context.Background(), &vmpb.BlockAcceptRequest{
		Id: buildRes.GetId(),
	})
	require.NoError(t, err)
}
