// (c) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package peer

import (
	"context"
	"errors"

	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

var (
	_ NetworkClient = &client{}

	ErrRequestFailed = errors.New("request failed")
)

// NetworkClient defines ability to send request / response through the Network
type NetworkClient interface {

	// SendAppRequest synchronously sends request to the selected nodeID
	// Returns response bytes, and ErrRequestFailed if the request should be retried.
	SendAppRequest(ctx context.Context, nodeID ids.NodeID, request []byte) ([]byte, error)
}

// client implements NetworkClient interface
// provides ability to send request / responses through the Network and wait for a response
// so that the caller gets the result synchronously.
type client struct {
	network Network
}

// NewNetworkClient returns Client for a given network
func NewNetworkClient(network Network) NetworkClient {
	return &client{
		network: network,
	}
}

// SendAppRequest synchronously sends request to the specified nodeID
// Returns response bytes and ErrRequestFailed if the request should be retried.
func (c *client) SendAppRequest(ctx context.Context, nodeID ids.NodeID, request []byte) ([]byte, error) {
	waitingHandler := newWaitingResponseHandler()
	if err := c.network.SendAppRequest(ctx, nodeID, request, waitingHandler); err != nil {
		return nil, err
	}
	return waitingHandler.WaitForResult(ctx)
}
