package p2p

import (
	"context"

	"github.com/landslidenetwork/slide-sdk/utils/ids"
)

// AppResponseCallback is called upon receiving an AppResponse for an AppRequest
// issued by Client.
// Callers should check [err] to see whether the AppRequest failed or not.
type AppResponseCallback func(
	ctx context.Context,
	nodeID ids.NodeID,
	responseBytes []byte,
	err error,
)

// PrefixMessage prefixes the original message with the protocol identifier.
//
// Only gossip and request messages need to be prefixed.
// Response messages don't need to be prefixed because request ids are tracked
// which map to the expected response handler.
func PrefixMessage(prefix, msg []byte) []byte {
	messageBytes := make([]byte, len(prefix)+len(msg))
	copy(messageBytes, prefix)
	copy(messageBytes[len(prefix):], msg)
	return messageBytes
}
