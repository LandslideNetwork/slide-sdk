package validators

import (
	"context"

	"github.com/consideritdone/landslidevm/utils/version"

	"github.com/consideritdone/landslidevm/utils/ids"
)

// Connector represents a handler that is called when a connection is marked as
// connected or disconnected
type Connector interface {
	Connected(
		ctx context.Context,
		nodeID ids.NodeID,
		nodeVersion *version.Application,
	) error
	Disconnected(ctx context.Context, nodeID ids.NodeID) error
}
