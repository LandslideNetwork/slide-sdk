package main

import (
	"context"
	"fmt"
	"os"

	"cosmossdk.io/log"
	"github.com/CosmWasm/wasmd/app"
	"github.com/CosmWasm/wasmd/x/wasm/keeper"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/testutil/sims"

	"github.com/consideritdone/landslidevm"
)

func main() {
	db, err := dbm.NewDB("dbName", dbm.MemDBBackend, "")
	if err != nil {
		panic(err)
	}
	logger := log.NewNopLogger()
	wasmApp := app.NewWasmApp(logger, db, nil, true, sims.NewAppOptionsWithFlagHome(os.TempDir()), []keeper.Option{})

	appCreator := landslidevm.NewLocalAppCreator(server.NewCometABCIWrapper(wasmApp))
	if err := landslidevm.Serve(context.Background(), appCreator); err != nil {
		panic(fmt.Sprintf("can't serve application: %s", err))
	}
}
