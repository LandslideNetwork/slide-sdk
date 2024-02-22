package landslidevm

import (
	"context"
	"net/http"
	"time"

	"github.com/ava-labs/avalanchego/api/health"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/version"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm/grpcutils"
	"github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/proxy"
)

var (
	_ common.AppRequestHandler            = (*LandslideVM)(nil)
	_ common.AppResponseHandler           = (*LandslideVM)(nil)
	_ common.AppGossipHandler             = (*LandslideVM)(nil)
	_ common.NetworkAppHandler            = (*LandslideVM)(nil)
	_ common.CrossChainAppRequestHandler  = (*LandslideVM)(nil)
	_ common.CrossChainAppResponseHandler = (*LandslideVM)(nil)
	_ common.CrossChainAppHandler         = (*LandslideVM)(nil)
	_ common.AppHandler                   = (*LandslideVM)(nil)
	_ health.Checker                      = (*LandslideVM)(nil)
	_ validators.Connector                = (*LandslideVM)(nil)
	_ common.VM                           = (*LandslideVM)(nil)
	_ block.Getter                        = (*LandslideVM)(nil)
	_ block.Parser                        = (*LandslideVM)(nil)
	_ block.ChainVM                       = (*LandslideVM)(nil)
)

type (
	Application = types.Application

	AppCreatorOpts struct {
		NetworkID uint32
	}

	AppCreator func(AppCreatorOpts) (Application, error)

	LandslideVM struct {
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

func Serve(ctx context.Context, creator AppCreator, opts ...grpcutils.ServerOption) {
	rpcchainvm.Serve(ctx, New(creator), opts...)
}

// Notify this engine of a request for an AppResponse with the same
// requestID.
//
// The meaning of request, and what should be sent in response to it, is
// application (VM) specific.
//
// It is not guaranteed that request is well-formed or valid.
//
// This function can be called by any node at any time.
func (vm *LandslideVM) AppRequest(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	deadline time.Time,
	request []byte,
) error {
	panic("ToDo: implement me")
}

// Notify this engine of the response to a previously sent AppRequest with
// the same requestID.
//
// The meaning of response is application (VM) specifc.
//
// It is not guaranteed that response is well-formed or valid.
func (vm *LandslideVM) AppResponse(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	response []byte,
) error {
	panic("ToDo: implement me")
}

// Notify this engine that an AppRequest it issued has failed.
//
// This function will be called if an AppRequest message with nodeID and
// requestID was previously sent by this engine and will not receive a
// response.
func (vm *LandslideVM) AppRequestFailed(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	appErr *common.AppError,
) error {
	panic("ToDo: implement me")
}

// Notify this engine of a gossip message from nodeID.
//
// The meaning of msg is application (VM) specific, and the VM defines how
// to react to this message.
//
// This message is not expected in response to any event, and it does not
// need to be responded to.
func (vm *LandslideVM) AppGossip(
	ctx context.Context,
	nodeID ids.NodeID,
	msg []byte,
) error {
	panic("ToDo: implement me")
}

// Notify this engine of a request for a CrossChainAppResponse with the same
// requestID.
//
// The meaning of request, and what should be sent in response to it, is
// application (VM) specific.
//
// Guarantees surrounding the request are specific to the implementation of
// the requesting VM. For example, the request may or may not be guaranteed
// to be well-formed/valid depending on the implementation of the requesting
// VM.
func (vm *LandslideVM) CrossChainAppRequest(
	ctx context.Context,
	chainID ids.ID,
	requestID uint32,
	deadline time.Time,
	request []byte,
) error {
	panic("ToDo: implement me")
}

// Notify this engine of the response to a previously sent
// CrossChainAppRequest with the same requestID.
//
// The meaning of response is application (VM) specifc.
//
// Guarantees surrounding the response are specific to the implementation of
// the responding VM. For example, the response may or may not be guaranteed
// to be well-formed/valid depending on the implementation of the requesting
// VM.
func (vm *LandslideVM) CrossChainAppResponse(
	ctx context.Context,
	chainID ids.ID,
	requestID uint32,
	response []byte,
) error {
	panic("ToDo: implement me")
}

// Notify this engine that a CrossChainAppRequest it issued has failed.
//
// This function will be called if a CrossChainAppRequest message with
// nodeID and requestID was previously sent by this engine and will not
// receive a response.
func (vm *LandslideVM) CrossChainAppRequestFailed(
	ctx context.Context,
	chainID ids.ID,
	requestID uint32,
	appErr *common.AppError,
) error {
	panic("ToDo: implement me")
}

// HealthCheck returns health check results and, if not healthy, a non-nil
// error
//
// It is expected that the results are json marshallable.
func (vm *LandslideVM) HealthCheck(context.Context) (interface{}, error) {
	panic("ToDo: implement me")
}

// Connector represents a handler that is called when a connection is marked as
// connected or disconnected
func (vm *LandslideVM) Connected(
	ctx context.Context,
	nodeID ids.NodeID,
	nodeVersion *version.Application,
) error {
	panic("ToDo: implement me")
}

// Connector represents a handler that is called when a connection is marked as
// connected or disconnected
func (vm *LandslideVM) Disconnected(ctx context.Context, nodeID ids.NodeID) error {
	panic("ToDo: implement me")
}

// Initialize this VM.
// [chainCtx]: Metadata about this VM.
//
//	[chainCtx.networkID]: The ID of the network this VM's chain is
//	                      running on.
//	[chainCtx.chainID]: The unique ID of the chain this VM is running on.
//	[chainCtx.Log]: Used to log messages
//	[chainCtx.NodeID]: The unique staker ID of this node.
//	[chainCtx.Lock]: A Read/Write lock shared by this VM and the
//	                 consensus engine that manages this VM. The write
//	                 lock is held whenever code in the consensus engine
//	                 calls the VM.
//
// [dbManager]: The manager of the database this VM will persist data to.
// [genesisBytes]: The byte-encoding of the genesis information of this
//
//	VM. The VM uses it to initialize its state. For
//	example, if this VM were an account-based payments
//	system, `genesisBytes` would probably contain a genesis
//	transaction that gives coins to some accounts, and this
//	transaction would be in the genesis block.
//
// [toEngine]: The channel used to send messages to the consensus engine.
// [fxs]: Feature extensions that attach to this VM.
func (vm *LandslideVM) Initialize(
	ctx context.Context,
	chainCtx *snow.Context,
	db database.Database,
	genesisBytes []byte,
	upgradeBytes []byte,
	configBytes []byte,
	toEngine chan<- common.Message,
	fxs []*common.Fx,
	appSender common.AppSender,
) error {

	panic("ToDo: implement me")
}

// SetState communicates to VM its next state it starts
func (vm *LandslideVM) SetState(ctx context.Context, state snow.State) error {
	panic("ToDo: implement me")
}

// Shutdown is called when the node is shutting down.
func (vm *LandslideVM) Shutdown(context.Context) error {
	panic("ToDo: implement me")
}

// Version returns the version of the VM.
func (vm *LandslideVM) Version(context.Context) (string, error) {
	panic("ToDo: implement me")
}

// Creates the HTTP handlers for custom chain network calls.
//
// This exposes handlers that the outside world can use to communicate with
// the chain. Each handler has the path:
// [Address of node]/ext/bc/[chain ID]/[extension]
//
// Returns a mapping from [extension]s to HTTP handlers.
//
// For example, if this VM implements an account-based payments system,
// it have an extension called `accounts`, where clients could get
// information about their accounts.
func (vm *LandslideVM) CreateHandlers(context.Context) (map[string]http.Handler, error) {
	panic("ToDo: implement me")
}

// Attempt to load a block.
//
// If the block does not exist, database.ErrNotFound should be returned.
//
// It is expected that blocks that have been successfully verified should be
// returned correctly. It is also expected that blocks that have been
// accepted by the consensus engine should be able to be fetched. It is not
// required for blocks that have been rejected by the consensus engine to be
// able to be fetched.
func (vm *LandslideVM) GetBlock(ctx context.Context, blkID ids.ID) (snowman.Block, error) {
	panic("ToDo: implement me")
}

// Attempt to create a block from a stream of bytes.
//
// The block should be represented by the full byte array, without extra
// bytes.
//
// It is expected for all historical blocks to be parseable.
func (vm *LandslideVM) ParseBlock(ctx context.Context, blockBytes []byte) (snowman.Block, error) {
	panic("ToDo: implement me")
}

// Attempt to create a new block from data contained in the VM.
//
// If the VM doesn't want to issue a new block, an error should be
// returned.
func (vm *LandslideVM) BuildBlock(context.Context) (snowman.Block, error) {
	panic("ToDo: implement me")
}

// Notify the VM of the currently preferred block.
//
// This should always be a block that has no children known to consensus.
func (vm *LandslideVM) SetPreference(ctx context.Context, blkID ids.ID) error {
	panic("ToDo: implement me")
}

// LastAccepted returns the ID of the last accepted block.
//
// If no blocks have been accepted by consensus yet, it is assumed there is
// a definitionally accepted block, the Genesis block, that will be
// returned.
func (vm *LandslideVM) LastAccepted(context.Context) (ids.ID, error) {
	panic("ToDo: implement me")
}

// VerifyHeightIndex should return:
//   - nil if the height index is available.
//   - ErrIndexIncomplete if the height index is not currently available.
//   - Any other non-standard error that may have occurred when verifying the
//     index.
//
// TODO: Remove after v1.11.x activates.
func (vm *LandslideVM) VerifyHeightIndex(context.Context) error {
	panic("ToDo: implement me")
}

// GetBlockIDAtHeight returns:
// - The ID of the block that was accepted with [height].
// - database.ErrNotFound if the [height] index is unknown.
//
// Note: A returned value of [database.ErrNotFound] typically means that the
//
//	underlying VM was state synced and does not have access to the
//	blockID at [height].
func (vm *LandslideVM) GetBlockIDAtHeight(ctx context.Context, height uint64) (ids.ID, error) {
	panic("ToDo: implement me")
}
