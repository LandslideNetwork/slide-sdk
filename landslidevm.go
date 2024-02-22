package cometvm

import (
	"github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/proxy"
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
