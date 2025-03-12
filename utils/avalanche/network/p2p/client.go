package p2p

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
