package cometvm

import (
	"testing"

	"github.com/cometbft/cometbft/abci/example/kvstore"
)

func TestCreation(t *testing.T) {
	New(NewLocalAppCreator(kvstore.NewInMemoryApplication()))
}
