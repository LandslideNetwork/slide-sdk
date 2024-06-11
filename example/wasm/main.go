package main

import (
	"context"
	"fmt"
	"os"

	"cosmossdk.io/log"
	"github.com/CosmWasm/wasmd/app"
	"github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/consideritdone/landslidevm"
)

func main() {
	db, err := dbm.NewDB("dbName", dbm.MemDBBackend, "")
	if err != nil {
		panic(err)
	}
	logger := log.NewNopLogger()

	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount(app.Bech32PrefixAccAddr, app.Bech32PrefixAccPub)
	cfg.SetBech32PrefixForValidator(app.Bech32PrefixValAddr, app.Bech32PrefixValPub)
	cfg.SetBech32PrefixForConsensusNode(app.Bech32PrefixConsAddr, app.Bech32PrefixConsPub)
	cfg.SetAddressVerifier(wasmtypes.VerifyAddressLen())
	cfg.Seal()
	wasmApp := app.NewWasmApp(logger, db, nil, true, sims.NewAppOptionsWithFlagHome(os.TempDir()), []keeper.Option{}, baseapp.SetChainID("landslide-test"))

	appCreator := landslidevm.NewLocalAppCreator(server.NewCometABCIWrapper(wasmApp))
	if err := landslidevm.Serve(context.Background(), appCreator); err != nil {
		panic(fmt.Sprintf("can't serve application: %s", err))
	}
}
