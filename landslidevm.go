package landslidevm

import (
	"context"

	"github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/proxy"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

var (
	_ LandslideVMServer = (*LandslideVM)(nil)
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

// Initialize this VM.
func (vm *LandslideVM) Initialize(context.Context, *InitializeRequest) (*InitializeResponse, error) {
	panic("ToDo: implement me")
}

// SetState communicates to VM its next state it starts
func (vm *LandslideVM) SetState(context.Context, *SetStateRequest) (*SetStateResponse, error) {
	panic("ToDo: implement me")
}

// Shutdown is called when the node is shutting down.
func (vm *LandslideVM) Shutdown(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// Creates the HTTP handlers for custom chain network calls.
func (vm *LandslideVM) CreateHandlers(context.Context, *emptypb.Empty) (*CreateHandlersResponse, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) Connected(context.Context, *ConnectedRequest) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) Disconnected(context.Context, *DisconnectedRequest) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// Attempt to create a new block from data contained in the VM.
func (vm *LandslideVM) BuildBlock(context.Context, *BuildBlockRequest) (*BuildBlockResponse, error) {
	panic("ToDo: implement me")
}

// Attempt to create a block from a stream of bytes.
func (vm *LandslideVM) ParseBlock(context.Context, *ParseBlockRequest) (*ParseBlockResponse, error) {
	panic("ToDo: implement me")
}

// Attempt to load a block.
func (vm *LandslideVM) GetBlock(context.Context, *GetBlockRequest) (*GetBlockResponse, error) {
	panic("ToDo: implement me")
}

// Notify the VM of the currently preferred block.
func (vm *LandslideVM) SetPreference(context.Context, *SetPreferenceRequest) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// Attempt to verify the health of the VM.
func (vm *LandslideVM) Health(context.Context, *emptypb.Empty) (*HealthResponse, error) {
	panic("ToDo: implement me")
}

// Version returns the version of the VM.
func (vm *LandslideVM) Version(context.Context, *emptypb.Empty) (*VersionResponse, error) {
	panic("ToDo: implement me")
}

// Notify this engine of a request for data from [nodeID].
func (vm *LandslideVM) AppRequest(context.Context, *AppRequestMsg) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// Notify this engine that an AppRequest message it sent to [nodeID] with
// request ID [requestID] failed.
func (vm *LandslideVM) AppRequestFailed(context.Context, *AppRequestFailedMsg) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// Notify this engine of a response to the AppRequest message it sent to
// [nodeID] with request ID [requestID].
func (vm *LandslideVM) AppResponse(context.Context, *AppResponseMsg) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// Notify this engine of a gossip message from [nodeID].
func (vm *LandslideVM) AppGossip(context.Context, *AppGossipMsg) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// Attempts to gather metrics from a VM.
func (vm *LandslideVM) Gather(context.Context, *emptypb.Empty) (*GatherResponse, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) CrossChainAppRequest(context.Context, *CrossChainAppRequestMsg) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) CrossChainAppRequestFailed(context.Context, *CrossChainAppRequestFailedMsg) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) CrossChainAppResponse(context.Context, *CrossChainAppResponseMsg) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// BatchedChainVM
func (vm *LandslideVM) GetAncestors(context.Context, *GetAncestorsRequest) (*GetAncestorsResponse, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) BatchedParseBlock(context.Context, *BatchedParseBlockRequest) (*BatchedParseBlockResponse, error) {
	panic("ToDo: implement me")
}

// HeightIndexedChainVM
func (vm *LandslideVM) VerifyHeightIndex(context.Context, *emptypb.Empty) (*VerifyHeightIndexResponse, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) GetBlockIDAtHeight(context.Context, *GetBlockIDAtHeightRequest) (*GetBlockIDAtHeightResponse, error) {
	panic("ToDo: implement me")
}

// StateSyncableVM
//
// StateSyncEnabled indicates whether the state sync is enabled for this VM.
func (vm *LandslideVM) StateSyncEnabled(context.Context, *emptypb.Empty) (*StateSyncEnabledResponse, error) {
	panic("ToDo: implement me")
}

// GetOngoingSyncStateSummary returns an in-progress state summary if it exists.
func (vm *LandslideVM) GetOngoingSyncStateSummary(context.Context, *emptypb.Empty) (*GetOngoingSyncStateSummaryResponse, error) {
	panic("ToDo: implement me")
}

// GetLastStateSummary returns the latest state summary.
func (vm *LandslideVM) GetLastStateSummary(context.Context, *emptypb.Empty) (*GetLastStateSummaryResponse, error) {
	panic("ToDo: implement me")
}

// ParseStateSummary parses a state summary out of [summaryBytes].
func (vm *LandslideVM) ParseStateSummary(context.Context, *ParseStateSummaryRequest) (*ParseStateSummaryResponse, error) {
	panic("ToDo: implement me")
}

// GetStateSummary retrieves the state summary that was generated at height
// [summaryHeight].
func (vm *LandslideVM) GetStateSummary(context.Context, *GetStateSummaryRequest) (*GetStateSummaryResponse, error) {
	panic("ToDo: implement me")
}

// Block
func (vm *LandslideVM) BlockVerify(context.Context, *BlockVerifyRequest) (*BlockVerifyResponse, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) BlockAccept(context.Context, *BlockAcceptRequest) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

func (vm *LandslideVM) BlockReject(context.Context, *BlockRejectRequest) (*emptypb.Empty, error) {
	panic("ToDo: implement me")
}

// StateSummary
func (vm *LandslideVM) StateSummaryAccept(context.Context, *StateSummaryAcceptRequest) (*StateSummaryAcceptResponse, error) {
	panic("ToDo: implement me")
}
