package main

import (
	"context"
	"fmt"

	"cosmossdk.io/log"
	"github.com/CosmWasm/wasmd/app"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/server"

	"github.com/consideritdone/landslidevm"
)

func main() {
	db, err := dbm.NewDB("dbName", dbm.MemDBBackend, "")
	if err != nil {
		panic(err)
	}
	logger := log.NewNopLogger()
	wasmApp := app.NewWasmApp(logger, db, nil, true, nil, nil)

	appCreator := landslidevm.NewLocalAppCreator(server.NewCometABCIWrapper(wasmApp))
	if err := landslidevm.Serve(context.Background(), appCreator); err != nil {
		panic(fmt.Sprintf("can't serve application: %s", err))
	}
}
