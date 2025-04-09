// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package p2p

import (
	"context"
	"encoding/binary"
	"errors"
	"github.com/cometbft/cometbft/libs/log"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/common"
	"github.com/landslidenetwork/slide-sdk/utils/ids"
	"github.com/landslidenetwork/slide-sdk/utils/set"
	"github.com/prometheus/client_golang/prometheus"
	"sync"
	"time"
)

var (
	//	_ validators.Connector = (*Network)(nil)
	_ common.AppHandler = (*Network)(nil)
	//	_ NodeSampler          = (*peerSampler)(nil)
	//
	opLabel      = "op"
	handlerLabel = "handlerID"
	labelNames   = []string{opLabel, handlerLabel}
)

// NewNetwork returns an instance of Network
func NewNetwork(
	log log.Logger,
	sender common.AppSender,
	registerer prometheus.Registerer,
	namespace string,
) (*Network, error) {
	metrics := metrics{
		msgTime: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "msg_time",
				Help:      "message handling time (ns)",
			},
			labelNames,
		),
		msgCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "msg_count",
				Help:      "message count (n)",
			},
			labelNames,
		),
	}

	err := errors.Join(
		registerer.Register(metrics.msgTime),
		registerer.Register(metrics.msgCount),
	)
	if err != nil {
		return nil, err
	}

	return &Network{
		Peers:  &Peers{},
		log:    log,
		sender: sender,
		router: newRouter(log, sender, metrics),
	}, nil
}

// Network exposes networking state and supports building p2p application
// protocols
type Network struct {
	Peers *Peers

	log    log.Logger
	sender common.AppSender

	router *router
}

func (n *Network) AppRequest(ctx context.Context, nodeID ids.NodeID, requestID uint32, deadline time.Time, request []byte) error {
	return n.router.AppRequest(ctx, nodeID, requestID, deadline, request)
}

func (n *Network) AppResponse(ctx context.Context, nodeID ids.NodeID, requestID uint32, response []byte) error {
	return n.router.AppResponse(ctx, nodeID, requestID, response)
}

func (n *Network) AppRequestFailed(ctx context.Context, nodeID ids.NodeID, requestID uint32, appErr *common.AppError) error {
	return n.router.AppRequestFailed(ctx, nodeID, requestID, appErr)
}

func (n *Network) AppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error {
	return n.router.AppGossip(ctx, nodeID, msg)
}

//func (n *Network) Connected(_ context.Context, nodeID ids.NodeID, _ *version.Application) error {
//	n.Peers.add(nodeID)
//	return nil
//}
//
//func (n *Network) Disconnected(_ context.Context, nodeID ids.NodeID) error {
//	n.Peers.remove(nodeID)
//	return nil
//}

// AddHandler reserves an identifier for an application protocol
func (n *Network) AddHandler(handlerID uint64, handler Handler) error {
	return n.router.addHandler(handlerID, handler)
}

// Peers contains metadata about the current set of connected peers
type Peers struct {
	lock sync.RWMutex
	set  set.SampleableSet[ids.NodeID]
}

func ProtocolPrefix(handlerID uint64) []byte {
	return binary.AppendUvarint(nil, handlerID)
}
