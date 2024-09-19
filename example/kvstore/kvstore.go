package main

import (
	"context"
	"fmt"

	"github.com/cometbft/cometbft/abci/example/kvstore"

	"github.com/landslidenetwork/slide-sdk/server"
	"github.com/landslidenetwork/slide-sdk/vm"
)

func main() {
	appCreator := KvStoreCreator()
	if err := server.Serve(context.Background(), appCreator); err != nil {
		panic(fmt.Sprintf("can't serve application: %s", err))
	}
}

func KvStoreCreator() vm.AppCreator {
	return func(config *vm.AppCreatorOpts) (vm.Application, error) {
		return kvstore.NewPersistentApplication(config.ChainDataDir), nil
	}
}
