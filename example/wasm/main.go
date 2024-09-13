package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"cosmossdk.io/log"
	"github.com/CosmWasm/wasmd/app"
	"github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/server/api"
	srvconfig "github.com/cosmos/cosmos-sdk/server/config"
	servergrpc "github.com/cosmos/cosmos-sdk/server/grpc"
	"github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/consideritdone/landslidevm"
	"github.com/consideritdone/landslidevm/utils/ids"
	"github.com/consideritdone/landslidevm/vm"
	vmtypes "github.com/consideritdone/landslidevm/vm/types"
)

// AppConfig is a Wasm App Config
type AppConfig struct {
	RPCPort  uint16 `json:"rpc_port"`
	GRPCPort uint16 `json:"grpc_port"`
	APIPort  uint16 `json:"api_port"`
	APIHost  string `json:"api_host"`
}

func main() {
	appCreator := WasmCreator()
	if err := landslidevm.Serve(context.Background(), appCreator); err != nil {
		panic(fmt.Sprintf("can't serve application: %s", err))
	}
}

func WasmCreator() vm.AppCreator {
	return func(config *vm.AppCreatorOpts) (vm.Application, error) {
		db, err := dbm.NewDB("wasm", dbm.GoLevelDBBackend, config.ChainDataDir)
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

		srvCfg := *srvconfig.DefaultConfig()
		grpcCfg := srvCfg.GRPC
		var (
			vmCfg  vmtypes.Config
			appCfg AppConfig
		)
		vmCfg.VMConfig.SetDefaults()

		if len(config.ConfigBytes) > 0 {
			if err := json.Unmarshal(config.ConfigBytes, &vmCfg); err != nil {
				return nil, fmt.Errorf("failed to unmarshal config %s: %w", string(config.ConfigBytes), err)
			}

			if err := vmCfg.VMConfig.Validate(); err != nil {
				return nil, err
			}

			// Unmarshal wasm app config
			if err := json.Unmarshal(vmCfg.AppConfig, &appCfg); err != nil {
				// set the grpc port, if it is set to 0, disable gRPC
				if appCfg.GRPCPort > 0 {
					grpcCfg.Address = fmt.Sprintf("127.0.0.1:%d", appCfg.GRPCPort)
				} else {
					grpcCfg.Enable = false
				}
			}
		}

		chainID := vmCfg.VMConfig.NetworkName
		var wasmApp = app.NewWasmApp(
			logger,
			db,
			nil,
			true,
			sims.NewAppOptionsWithFlagHome(os.TempDir()),
			[]keeper.Option{},
			baseapp.SetChainID(chainID),
		)

		// early return if gRPC is disabled
		if !grpcCfg.Enable {
			return server.NewCometABCIWrapper(wasmApp), nil
		}

		interfaceRegistry := wasmApp.InterfaceRegistry()
		marshaller := codec.NewProtoCodec(interfaceRegistry)
		clientCtx := client.Context{}.
			WithCodec(marshaller).
			WithLegacyAmino(makeCodec()).
			WithTxConfig(tx.NewTxConfig(marshaller, tx.DefaultSignModes)).
			WithInterfaceRegistry(interfaceRegistry).
			WithChainID(chainID)

		avaChainID, err := ids.ToID(config.ChainID)
		if err != nil {
			return nil, err
		}

		rpcURI := fmt.Sprintf(
			"http://127.0.0.1:%d/ext/bc/%s/rpc",
			appCfg.RPCPort,
			avaChainID,
		)

		clientCtx = clientCtx.WithNodeURI(rpcURI)
		rpcclient, err := rpchttp.New(rpcURI, "/websocket")
		if err != nil {
			return nil, err
		}
		clientCtx = clientCtx.WithClient(rpcclient)

		// use the provided clientCtx to register the services
		wasmApp.RegisterTxService(clientCtx)
		wasmApp.RegisterTendermintService(clientCtx)
		wasmApp.RegisterNodeService(clientCtx, srvconfig.Config{})

		maxSendMsgSize := grpcCfg.MaxSendMsgSize
		if maxSendMsgSize == 0 {
			maxSendMsgSize = srvconfig.DefaultGRPCMaxSendMsgSize
		}

		maxRecvMsgSize := grpcCfg.MaxRecvMsgSize
		if maxRecvMsgSize == 0 {
			maxRecvMsgSize = srvconfig.DefaultGRPCMaxRecvMsgSize
		}

		// if gRPC is enabled, configure gRPC client for gRPC gateway
		grpcClient, err := grpc.NewClient(
			grpcCfg.Address,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(
				grpc.ForceCodec(codec.NewProtoCodec(clientCtx.InterfaceRegistry).GRPCCodec()),
				grpc.MaxCallRecvMsgSize(maxRecvMsgSize),
				grpc.MaxCallSendMsgSize(maxSendMsgSize),
			),
		)
		if err != nil {
			return nil, err
		}

		clientCtx = clientCtx.WithGRPCClient(grpcClient)
		logger.Debug("gRPC client assigned to client context", "target", grpcCfg.Address)

		g, ctx := getCtx(logger, false)

		grpcSrv, err := servergrpc.NewGRPCServer(clientCtx, wasmApp, grpcCfg)
		if err != nil {
			return nil, err
		}

		// Start the gRPC server in a goroutine. Note, the provided ctx will ensure
		// that the server is gracefully shut down.
		g.Go(func() error {
			return servergrpc.StartGRPCServer(ctx, logger.With("module", "grpc-server"), grpcCfg, grpcSrv)
		})

		if appCfg.APIPort == 0 {
			appCfg.APIPort = 1317
		}
		if appCfg.APIHost == "" {
			appCfg.APIHost = "localhost"
		}
		apiURI := fmt.Sprintf("tcp://%s", net.JoinHostPort(appCfg.APIHost, strconv.Itoa(int(appCfg.APIPort))))

		srvCfg.API.Enable = true
		srvCfg.API.Swagger = false
		srvCfg.API.EnableUnsafeCORS = true
		srvCfg.API.Address = apiURI

		apiSrv := api.New(clientCtx, logger.With(log.ModuleKey, "api-server"), grpcSrv)
		wasmApp.RegisterAPIRoutes(apiSrv, srvCfg.API)
		g.Go(func() error {
			return apiSrv.Start(ctx, srvCfg)
		})

		return server.NewCometABCIWrapper(wasmApp), nil
	}
}

// custom tx codec
func makeCodec() *codec.LegacyAmino {
	cdc := codec.NewLegacyAmino()
	sdk.RegisterLegacyAminoCodec(cdc)
	cryptocodec.RegisterCrypto(cdc)
	return cdc
}

func getCtx(logger log.Logger, block bool) (*errgroup.Group, context.Context) {
	ctx, cancelFn := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)
	// listen for quit signals so the calling parent process can gracefully exit
	listenForQuitSignals(g, block, cancelFn, logger)
	return g, ctx
}

// listenForQuitSignals listens for SIGINT and SIGTERM. When a signal is received,
// the cleanup function is called, indicating the caller can gracefully exit or
// return.
//
// Note, the blocking behavior of this depends on the block argument.
// The caller must ensure the corresponding context derived from the cancelFn is used correctly.
func listenForQuitSignals(g *errgroup.Group, block bool, cancelFn context.CancelFunc, logger log.Logger) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	f := func() {
		sig := <-sigCh
		cancelFn()

		logger.Info("caught signal", "signal", sig.String())
	}

	if block {
		g.Go(func() error {
			f()
			return nil
		})
	} else {
		go f()
	}
}
