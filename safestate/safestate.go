package safestate

import (
	"github.com/cometbft/cometbft/state"
	"github.com/cometbft/cometbft/types"
	"sync"
)

type SafeState struct {
	state.State
	mtx *sync.RWMutex
}

func New(state state.State) SafeState {
	return SafeState{
		State: state,
		mtx:   &sync.RWMutex{},
	}
}

func (ss *SafeState) StateCopy() state.State {
	ss.mtx.RLock()
	defer ss.mtx.RUnlock()
	return ss.State
}

func (ss *SafeState) StateBytes() []byte {
	ss.mtx.RLock()
	defer ss.mtx.RUnlock()
	return ss.State.Bytes()
}

func (ss *SafeState) LastBlockHeight() int64 {
	ss.mtx.RLock()
	defer ss.mtx.RUnlock()
	return ss.State.LastBlockHeight
}

func (ss *SafeState) LastBlockID() types.BlockID {
	ss.mtx.RLock()
	defer ss.mtx.RUnlock()
	return ss.State.LastBlockID
}

func (ss *SafeState) Validators() *types.ValidatorSet {
	ss.mtx.RLock()
	defer ss.mtx.RUnlock()
	return ss.State.Validators
}

func (ss *SafeState) AppHash() []byte {
	ss.mtx.RLock()
	defer ss.mtx.RUnlock()
	return ss.State.AppHash
}

func (ss *SafeState) ChainID() string {
	ss.mtx.RLock()
	defer ss.mtx.RUnlock()
	return ss.State.ChainID
}
