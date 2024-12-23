// (c) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package peer

import (
	"context"
	"github.com/landslidenetwork/slide-sdk/utils/ids"

	"github.com/landslidenetwork/slide-sdk/utils/version"
)

// NetworkClient defines ability to send request / response through the Network
type NetworkClient interface {
	// SendAppRequestAny synchronously sends request to an arbitrary peer with a
	// node version greater than or equal to minVersion.
	// Returns response bytes, the ID of the chosen peer, and ErrRequestFailed if
	// the request should be retried.
	SendAppRequestAny(ctx context.Context, minVersion *version.Application, request []byte) ([]byte, ids.NodeID, error)

	// SendAppRequest synchronously sends request to the selected nodeID
	// Returns response bytes, and ErrRequestFailed if the request should be retried.
	SendAppRequest(ctx context.Context, nodeID ids.NodeID, request []byte) ([]byte, error)

	// TrackBandwidth should be called for each valid request with the bandwidth
	// (length of response divided by request time), and with 0 if the response is invalid.
	TrackBandwidth(nodeID ids.NodeID, bandwidth float64)
}
