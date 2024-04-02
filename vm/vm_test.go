package vm

import (
	"context"
	_ "embed"
	"testing"

	"github.com/cometbft/cometbft/abci/example/kvstore"
	vmpb "github.com/consideritdone/landslidevm/proto/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	//go:embed testdata/genesis.json
	countervmGenesis []byte
)

func newKvApp(t *testing.T) vmpb.VMServer {
	vm := New(func(*AppCreatorOpts) (Application, error) {
		return kvstore.NewInMemoryApplication(), nil
	})
	require.NotNil(t, vm)

	res, err := vm.Initialize(context.Background(), &vmpb.InitializeRequest{
		DbServerAddr: "inmemory",
		GenesisBytes: countervmGenesis,
	})
	assert.NoError(t, err)
	assert.NotNil(t, res)

	return vm
}

func TestCreation(t *testing.T) {
	newKvApp(t)
}

func TestBlockMethods(t *testing.T) {
	vm := newKvApp(t)
	_ = vm
}
