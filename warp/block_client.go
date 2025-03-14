package warp

import (
	"context"
	"github.com/landslidenetwork/slide-sdk/database"
	vmpb "github.com/landslidenetwork/slide-sdk/proto/vm"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	"golang.org/x/exp/slices"
)

type blockStorageClient struct {
	receiver BlockReceiver
}

type BlockReceiver interface {
	// Attempt to load a block.
	GetBlock(context.Context, *vmpb.GetBlockRequest) (*vmpb.GetBlockResponse, error)
	// HeightIndexedChainVM
	GetBlockIDAtHeight(context.Context, *vmpb.GetBlockIDAtHeightRequest) (*vmpb.GetBlockIDAtHeightResponse, error)
}

type BlockClient interface {
	GetAcceptedBlock(ctx context.Context, blockID ids.ID) (*vmpb.GetBlockResponse, error)
}

// GetAcceptedBlock attempts to retrieve block [blkID] from the VM. This method
// only returns accepted blocks.
func (client *blockStorageClient) GetAcceptedBlock(ctx context.Context, blkID ids.ID) (*vmpb.GetBlockResponse, error) {
	blkResp, err := client.receiver.GetBlock(ctx, &vmpb.GetBlockRequest{Id: blkID[:]})
	if err != nil {
		return nil, err
	}

	height := blkResp.Height
	acceptedBlkIDResp, err := client.receiver.GetBlockIDAtHeight(ctx, &vmpb.GetBlockIDAtHeightRequest{Height: height})
	if err != nil {
		return nil, err
	}

	if slices.Equal(acceptedBlkIDResp.BlkId, blkID[:]) {
		// The provided block is not accepted.
		return nil, database.ErrNotFound
	}
	return blkResp, nil
}
