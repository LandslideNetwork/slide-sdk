// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package enginetest

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/landslidenetwork/slide-sdk/utils/avalanche/common"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	"github.com/landslidenetwork/slide-sdk/utils/set"
)

var (
	_ common.Sender    = (*Sender)(nil)
	_ common.AppSender = (*SenderStub)(nil)

	errSendAppRequest  = errors.New("unexpectedly called SendAppRequest")
	errSendAppResponse = errors.New("unexpectedly called SendAppResponse")
	errSendAppError    = errors.New("unexpectedly called SendAppError")
	errSendAppGossip   = errors.New("unexpectedly called SendAppGossip")
)

// Sender is a test sender
type Sender struct {
	T *testing.T

	CantSendAppRequest, CantSendAppResponse, CantSendAppError, CantSendAppGossip bool

	SendAppRequestF  func(context.Context, set.Set[ids.NodeID], uint32, []byte) error
	SendAppResponseF func(context.Context, ids.NodeID, uint32, []byte) error
	SendAppErrorF    func(context.Context, ids.NodeID, uint32, int32, string) error
	SendAppGossipF   func(context.Context, common.SendConfig, []byte) error
}

// Default set the default callable value to [cant]
func (s *Sender) Default(cant bool) {
	s.CantSendAppRequest = cant
	s.CantSendAppResponse = cant
}

// SendAppRequest calls SendAppRequestF if it was initialized. If it wasn't
// initialized and this function shouldn't be called and testing was
// initialized, then testing will fail.
func (s *Sender) SendAppRequest(ctx context.Context, nodeIDs set.Set[ids.NodeID], requestID uint32, appRequestBytes []byte) error {
	switch {
	case s.SendAppRequestF != nil:
		return s.SendAppRequestF(ctx, nodeIDs, requestID, appRequestBytes)
	case s.CantSendAppRequest && s.T != nil:
		require.FailNow(s.T, errSendAppRequest.Error())
	}
	return errSendAppRequest
}

// SendAppResponse calls SendAppResponseF if it was initialized. If it wasn't
// initialized and this function shouldn't be called and testing was
// initialized, then testing will fail.
func (s *Sender) SendAppResponse(ctx context.Context, nodeID ids.NodeID, requestID uint32, appResponseBytes []byte) error {
	switch {
	case s.SendAppResponseF != nil:
		return s.SendAppResponseF(ctx, nodeID, requestID, appResponseBytes)
	case s.CantSendAppResponse && s.T != nil:
		require.FailNow(s.T, errSendAppResponse.Error())
	}
	return errSendAppResponse
}

// SendAppError calls SendAppErrorF if it was initialized. If it wasn't
// initialized and this function shouldn't be called and testing was
// initialized, then testing will fail.
func (s *Sender) SendAppError(ctx context.Context, nodeID ids.NodeID, requestID uint32, code int32, message string) error {
	switch {
	case s.SendAppErrorF != nil:
		return s.SendAppErrorF(ctx, nodeID, requestID, code, message)
	case s.CantSendAppError && s.T != nil:
		require.FailNow(s.T, errSendAppError.Error())
	}
	return errSendAppError
}

// SendAppGossip calls SendAppGossipF if it was initialized. If it wasn't
// initialized and this function shouldn't be called and testing was
// initialized, then testing will fail.
func (s *Sender) SendAppGossip(
	ctx context.Context,
	config common.SendConfig,
	appGossipBytes []byte,
) error {
	switch {
	case s.SendAppGossipF != nil:
		return s.SendAppGossipF(ctx, config, appGossipBytes)
	case s.CantSendAppGossip && s.T != nil:
		require.FailNow(s.T, errSendAppGossip.Error())
	}
	return errSendAppGossip
}

// SenderStub is a stub sender that returns values received on method-specific channels.
type SenderStub struct {
	SentAppRequest, SentAppResponse,
	SentAppGossip chan []byte

	SentAppError chan *common.AppError
}

func (f SenderStub) SendAppRequest(_ context.Context, _ set.Set[ids.NodeID], _ uint32, bytes []byte) error {
	if f.SentAppRequest == nil {
		return nil
	}

	f.SentAppRequest <- bytes
	return nil
}

func (f SenderStub) SendAppResponse(_ context.Context, _ ids.NodeID, _ uint32, bytes []byte) error {
	if f.SentAppResponse == nil {
		return nil
	}

	f.SentAppResponse <- bytes
	return nil
}

func (f SenderStub) SendAppError(_ context.Context, _ ids.NodeID, _ uint32, errorCode int32, errorMessage string) error {
	if f.SentAppError == nil {
		return nil
	}

	f.SentAppError <- &common.AppError{
		Code:    errorCode,
		Message: errorMessage,
	}
	return nil
}

func (f SenderStub) SendAppGossip(_ context.Context, _ common.SendConfig, bytes []byte) error {
	if f.SentAppGossip == nil {
		return nil
	}

	f.SentAppGossip <- bytes
	return nil
}
