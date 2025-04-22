// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package common

import (
	"context"

	"github.com/landslidenetwork/slide-sdk/utils/ids"
	"github.com/landslidenetwork/slide-sdk/utils/set"
)

// SendConfig is used to specify who to send messages to over the p2p network.
type SendConfig struct {
	NodeIDs       set.Set[ids.NodeID]
	Validators    int
	NonValidators int
	Peers         int
}

// Sender defines how a consensus engine sends messages and requests to other
// validators.
//
// Messages can be categorized as either: requests, responses, or gossip. Gossip
// messages do not include requestIDs, because no response is expected from the
// peer. However, both requests and responses include requestIDs.
//
// It is expected that each [nodeID + requestID + expected response type] that
// is outstanding at any given time is unique.
//
// As an example, it is valid to send `Get(nodeA, request0)` and
// `PullQuery(nodeA, request0)` because they have different expected response
// types, `Put` and `Chits`.
//
// Additionally, after having sent `Get(nodeA, request0)` and receiving either
// `Put(nodeA, request0)` or `GetFailed(nodeA, request0)`, it is valid to resend
// `Get(nodeA, request0)`. Because the initial `Get` request is no longer
// outstanding.
//
// This means that requestIDs can be reused. In practice, requests always have a
// reasonable maximum timeout, so it is generally safe to assume that by the
// time the requestID space has been exhausted, the beginning of the requestID
// space is free of conflicts.
type Sender interface {
	AppSender
}

// AppSender sends VM-level messages to nodes in the network.
type AppSender interface {
	// Send an application-level request.
	//
	// The VM corresponding to this AppSender may receive either:
	// * An AppResponse from nodeID with ID [requestID]
	// * An AppRequestFailed from nodeID with ID [requestID]
	//
	// A nil return value guarantees that the VM corresponding to this AppSender
	// will receive exactly one of the above messages.
	//
	// A non-nil return value guarantees that the VM corresponding to this
	// AppSender will receive at most one of the above messages.
	SendAppRequest(ctx context.Context, nodeIDs set.Set[ids.NodeID], requestID uint32, appRequestBytes []byte) error
	// Send an application-level response to a request.
	// This response must be in response to an AppRequest that the VM corresponding
	// to this AppSender received from [nodeID] with ID [requestID].
	SendAppResponse(ctx context.Context, nodeID ids.NodeID, requestID uint32, appResponseBytes []byte) error
	// SendAppError sends an application-level error to an AppRequest
	SendAppError(ctx context.Context, nodeID ids.NodeID, requestID uint32, errorCode int32, errorMessage string) error
	// Gossip an application-level message.
	SendAppGossip(
		ctx context.Context,
		config SendConfig,
		appGossipBytes []byte,
	) error
}
