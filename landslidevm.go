package landslidevm

import (
	"context"
	"fmt"
	"math"
	"net"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/types/known/emptypb"

	vmpb "github.com/consideritdone/landslidevm/proto/vm"
	runtimepb "github.com/consideritdone/landslidevm/proto/vm/runtime"
)

const (
	// Address of the runtime engine server.
	EngineAddressKey = "AVALANCHE_VM_RUNTIME_ENGINE_ADDR"
	// MinTime is the minimum amount of time a client should wait before sending
	// a keepalive ping. grpc-go default 5 mins
	defaultServerKeepAliveMinTime = 5 * time.Second
	// After a duration of this time if the server doesn't see any activity it
	// pings the client to see if the transport is still alive.
	// If set below 1s, a minimum value of 1s will be used instead.
	// grpc-go default 2h
	defaultServerKeepAliveInterval = 2 * time.Hour
	// After having pinged for keepalive check, the server waits for a duration
	// of Timeout and if no activity is seen even after that the connection is
	// closed. grpc-go default 20s
	defaultServerKeepAliveTimeout = 20 * time.Second
	// If true, client sends keepalive pings even with no active RPCs. If false,
	// when there are no active RPCs, Time and Timeout will be ignored and no
	// keepalive pings will be sent. grpc-go default false
	defaultPermitWithoutStream = true
	//
	defaultRuntimeDialTimeout = 5 * time.Second
	// rpcChainVMProtocol should be bumped anytime changes are made which
	// require the plugin vm to upgrade to latest avalanchego release to be
	// compatible.
	rpcChainVMProtocol uint = 34
)

var (
	_ vmpb.VMServer = (*LandslideVM)(nil)

	DefaultServerOptions = []grpc.ServerOption{
		grpc.MaxRecvMsgSize(math.MaxInt),
		grpc.MaxSendMsgSize(math.MaxInt),
		grpc.MaxConcurrentStreams(math.MaxUint32),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             defaultServerKeepAliveMinTime,
			PermitWithoutStream: defaultPermitWithoutStream,
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    defaultServerKeepAliveInterval,
			Timeout: defaultServerKeepAliveTimeout,
		}),
	}
)

type (
	Application = types.Application

	AppCreatorOpts struct {
		NetworkID uint32
	}

	AppCreator func(AppCreatorOpts) (Application, error)

	LandslideVM struct {
		allowShutdown atomic.Bool

		appCreator AppCreator
		app        proxy.AppConns
	}
)

func NewLocalAppCreator(app Application) AppCreator {
	return func(AppCreatorOpts) (Application, error) {
		return app, nil
	}
}

func New(creator AppCreator) *LandslideVM {
	return &LandslideVM{appCreator: creator}
}

func Serve[T interface{ AppCreator | *LandslideVM }](ctx context.Context, subject T, options ...grpc.ServerOption) error {
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)

	var vm *LandslideVM
	switch v := any(subject).(type) {
	case AppCreator:
		vm = New(v)
	case *LandslideVM:
		vm = v
	}

	if len(options) > 0 {
		options = DefaultServerOptions
	}
	server := grpc.NewServer(options...)
	vmpb.RegisterVMServer(server, vm)

	health := health.NewServer()
	health.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(server, health)

	go func(ctx context.Context) {
		defer func() {
			server.GracefulStop()
			fmt.Println("vm server: graceful termination success")
		}()

		for {
			select {
			case s := <-signals:
				// We drop all signals until our parent process has notified us
				// that we are shutting down. Once we are in the shutdown
				// workflow, we will gracefully exit upon receiving a SIGTERM.
				if !vm.CanShutdown() {
					fmt.Printf("runtime engine: ignoring signal: %s\n", s)
					continue
				}

				switch s {
				case syscall.SIGINT:
					fmt.Printf("runtime engine: ignoring signal: %s\n", s)
				case syscall.SIGTERM:
					fmt.Printf("runtime engine: received shutdown signal: %s\n", s)
					return
				}
			case <-ctx.Done():
				fmt.Println("runtime engine: context has been cancelled")
				return
			}
		}
	}(ctx)

	runtimeAddr, runtimeAddrExist := os.LookupEnv(EngineAddressKey)
	if !runtimeAddrExist {
		return fmt.Errorf("required env var missing: %q", EngineAddressKey)
	}

	clientConn, err := grpc.Dial("passthrough:///" + runtimeAddr)
	if err != nil {
		return fmt.Errorf("failed to create client conn: %w", err)
	}

	client := runtimepb.NewRuntimeClient(clientConn)
	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		return fmt.Errorf("failed to create new listener: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, defaultRuntimeDialTimeout)
	defer cancel()

	_, err = client.Initialize(ctx, &runtimepb.InitializeRequest{
		ProtocolVersion: uint32(rpcChainVMProtocol),
		Addr:            listener.Addr().String(),
	})
	if err != nil {
		_ = listener.Close()
		return fmt.Errorf("failed to initialize vm runtime: %w", err)
	}

	_ = server.Serve(listener)
	_ = listener.Close()
	return nil
}

// Initialize this VM.
func (vm *LandslideVM) Initialize(context.Context, *vmpb.InitializeRequest) (*vmpb.InitializeResponse, error) {
	panic("ToDo: implement me")
}

// SetState communicates to VM its next state it starts
func (vm *LandslideVM) SetState(context.Context, *vmpb.SetStateRequest) (*vmpb.SetStateResponse, error) {
	panic("ToDo: implement me")
}

// CanShutdown lets known when vm ready to shutting down
func (vm *LandslideVM) CanShutdown() bool {
	return vm.allowShutdown.Load()
}

// Shutdown is called when the node is shutting down.
func (vm *LandslideVM) Shutdown(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// Creates the HTTP handlers for custom chain network calls.
func (vm *LandslideVM) CreateHandlers(context.Context, *emptypb.Empty) (*vmpb.CreateHandlersResponse, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) Connected(context.Context, *vmpb.ConnectedRequest) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) Disconnected(context.Context, *vmpb.DisconnectedRequest) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// Attempt to create a new block from data contained in the VM.
func (vm *LandslideVM) BuildBlock(context.Context, *vmpb.BuildBlockRequest) (*vmpb.BuildBlockResponse, error) {
	panic("ToDo: implement me")
}

// Attempt to create a block from a stream of bytes.
func (vm *LandslideVM) ParseBlock(context.Context, *vmpb.ParseBlockRequest) (*vmpb.ParseBlockResponse, error) {
	panic("ToDo: implement me")
}

// Attempt to load a block.
func (vm *LandslideVM) GetBlock(context.Context, *vmpb.GetBlockRequest) (*vmpb.GetBlockResponse, error) {
	panic("ToDo: implement me")
}

// Notify the VM of the currently preferred block.
func (vm *LandslideVM) SetPreference(context.Context, *vmpb.SetPreferenceRequest) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// Attempt to verify the health of the VM.
func (vm *LandslideVM) Health(context.Context, *emptypb.Empty) (*vmpb.HealthResponse, error) {
	panic("ToDo: implement me")
}

// Version returns the version of the VM.
func (vm *LandslideVM) Version(context.Context, *emptypb.Empty) (*vmpb.VersionResponse, error) {
	panic("ToDo: implement me")
}

// Notify this engine of a request for data from [nodeID].
func (vm *LandslideVM) AppRequest(context.Context, *vmpb.AppRequestMsg) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// Notify this engine that an AppRequest message it sent to [nodeID] with
// request ID [requestID] failed.
func (vm *LandslideVM) AppRequestFailed(context.Context, *vmpb.AppRequestFailedMsg) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// Notify this engine of a response to the AppRequest message it sent to
// [nodeID] with request ID [requestID].
func (vm *LandslideVM) AppResponse(context.Context, *vmpb.AppResponseMsg) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// Notify this engine of a gossip message from [nodeID].
func (vm *LandslideVM) AppGossip(context.Context, *vmpb.AppGossipMsg) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// Attempts to gather metrics from a VM.
func (vm *LandslideVM) Gather(context.Context, *emptypb.Empty) (*vmpb.GatherResponse, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) CrossChainAppRequest(context.Context, *vmpb.CrossChainAppRequestMsg) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) CrossChainAppRequestFailed(context.Context, *vmpb.CrossChainAppRequestFailedMsg) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) CrossChainAppResponse(context.Context, *vmpb.CrossChainAppResponseMsg) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// BatchedChainVM
func (vm *LandslideVM) GetAncestors(context.Context, *vmpb.GetAncestorsRequest) (*vmpb.GetAncestorsResponse, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) BatchedParseBlock(context.Context, *vmpb.BatchedParseBlockRequest) (*vmpb.BatchedParseBlockResponse, error) {
	panic("ToDo: implement me")
}

// HeightIndexedChainVM
func (vm *LandslideVM) VerifyHeightIndex(context.Context, *emptypb.Empty) (*vmpb.VerifyHeightIndexResponse, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) GetBlockIDAtHeight(context.Context, *vmpb.GetBlockIDAtHeightRequest) (*vmpb.GetBlockIDAtHeightResponse, error) {
	panic("ToDo: implement me")
}

// StateSyncableVM
//
// StateSyncEnabled indicates whether the state sync is enabled for this VM.
func (vm *LandslideVM) StateSyncEnabled(context.Context, *emptypb.Empty) (*vmpb.StateSyncEnabledResponse, error) {
	panic("ToDo: implement me")
}

// GetOngoingSyncStateSummary returns an in-progress state summary if it exists.
func (vm *LandslideVM) GetOngoingSyncStateSummary(context.Context, *emptypb.Empty) (*vmpb.GetOngoingSyncStateSummaryResponse, error) {
	panic("ToDo: implement me")
}

// GetLastStateSummary returns the latest state summary.
func (vm *LandslideVM) GetLastStateSummary(context.Context, *emptypb.Empty) (*vmpb.GetLastStateSummaryResponse, error) {
	panic("ToDo: implement me")
}

// ParseStateSummary parses a state summary out of [summaryBytes].
func (vm *LandslideVM) ParseStateSummary(context.Context, *vmpb.ParseStateSummaryRequest) (*vmpb.ParseStateSummaryResponse, error) {
	panic("ToDo: implement me")
}

// GetStateSummary retrieves the state summary that was generated at height
// [summaryHeight].
func (vm *LandslideVM) GetStateSummary(context.Context, *vmpb.GetStateSummaryRequest) (*vmpb.GetStateSummaryResponse, error) {
	panic("ToDo: implement me")
}

// Block
func (vm *LandslideVM) BlockVerify(context.Context, *vmpb.BlockVerifyRequest) (*vmpb.BlockVerifyResponse, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) BlockAccept(context.Context, *vmpb.BlockAcceptRequest) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) BlockReject(context.Context, *vmpb.BlockRejectRequest) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// StateSummary
func (vm *LandslideVM) StateSummaryAccept(context.Context, *vmpb.StateSummaryAcceptRequest) (*vmpb.StateSummaryAcceptResponse, error) {
	panic("ToDo: implement me")
}
