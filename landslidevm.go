package landslidevm

import (
	"context"
	"fmt"
	"math"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	vmpb "github.com/consideritdone/landslidevm/proto/vm"
	runtimepb "github.com/consideritdone/landslidevm/proto/vm/runtime"
	"github.com/consideritdone/landslidevm/vm"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
)

const (
	// EngineAddressKey address of the runtime engine server.
	EngineAddressKey = "AVALANCHE_VM_RUNTIME_ENGINE_ADDR"
	// MinTime is the minimum amount of time a client should wait before sending
	// a keepalive ping. grpc-go default 5 minutes
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

func NewLocalAppCreator(app vm.Application) vm.AppCreator {
	return func(*vm.AppCreatorOpts) (vm.Application, error) {
		return app, nil
	}
}

func Serve[T interface {
	vm.AppCreator | *vm.LandslideVM
}](ctx context.Context, subject T, options ...grpc.ServerOption) error {
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)

	var lvm *vm.LandslideVM
	switch v := any(subject).(type) {
	case vm.AppCreator:
		lvm = vm.New(v)
	case *vm.LandslideVM:
		lvm = v
	}

	if len(options) > 0 {
		options = DefaultServerOptions
	}
	server := grpc.NewServer(options...)
	vmpb.RegisterVMServer(server, lvm)

	healthsrv := health.NewServer()
	healthsrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(server, healthsrv)

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
				if !lvm.CanShutdown() {
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
