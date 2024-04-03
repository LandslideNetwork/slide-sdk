package main

import (
	"context"
	"fmt"

	"github.com/cometbft/cometbft/abci/example/kvstore"
	"github.com/consideritdone/landslidevm"
)

func main() {
	appCreator := landslidevm.NewLocalAppCreator(kvstore.NewInMemoryApplication())
	if err := landslidevm.Serve(context.Background(), appCreator); err != nil {
		panic(fmt.Sprintf("can't serve application: %s", err))
	}
}
