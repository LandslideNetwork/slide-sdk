package vm

import (
	"context"

	abci "github.com/cometbft/cometbft/abci/types"
	mempl "github.com/cometbft/cometbft/mempool"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cometbft/cometbft/types"
	"github.com/consideritdone/landslidevm/jsonrpc"
)

type RPC struct {
	vm *LandslideVM
}

func NewRPC(vm *LandslideVM) *RPC {
	return &RPC{vm}
}

func (rpc *RPC) Routes() map[string]jsonrpc.Handler {
	return map[string]jsonrpc.Handler{
		"broadcast_tx_commit": jsonrpc.TypedHandlerFunc(rpc.BroadcastTxSync),
	}
}

func (rpc *RPC) BroadcastTxAsync(ctx context.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	err := rpc.vm.mempool.CheckTx(tx, nil, mempl.TxInfo{})
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultBroadcastTx{Hash: tx.Hash()}, nil
}

func (rpc *RPC) BroadcastTxSync(ctx context.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	resCh := make(chan *abci.ResponseCheckTx, 1)
	err := rpc.vm.mempool.CheckTx(tx, func(res *abci.ResponseCheckTx) {
		resCh <- res
	}, mempl.TxInfo{})
	if err != nil {
		return nil, err
	}
	res := <-resCh
	return &ctypes.ResultBroadcastTx{
		Code:      res.GetCode(),
		Data:      res.GetData(),
		Log:       res.GetLog(),
		Codespace: res.GetCodespace(),
		Hash:      tx.Hash(),
	}, nil
}
