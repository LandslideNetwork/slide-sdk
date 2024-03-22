package landslidevm

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
	//go:embed example/countervm/genesis.json
	countervmGenesis []byte
)

func TestVmCreation(t *testing.T) {
	require.NotNil(t, New(NewLocalAppCreator(kvstore.NewInMemoryApplication())))
}

func TestVmInitialize(t *testing.T) {
	vm := New(NewLocalAppCreator(kvstore.NewInMemoryApplication()))
	require.NotNil(t, vm)

	res, err := vm.Initialize(context.Background(), &vmpb.InitializeRequest{
		DbServerAddr: "inmemory",
		GenesisBytes: countervmGenesis,
	})
	// ToDo: assert.NoError(t, err)
	assert.ErrorContains(t, err, "ToDo: add first block acceptance")
	// ToDo: assert.NotNil(t, res)
	assert.Nil(t, res)
}
