// (c) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package message

import (
	"fmt"

	"github.com/landslidenetwork/slide-sdk/utils/codec"

	"github.com/landslidenetwork/slide-sdk/utils/ids"
	"github.com/landslidenetwork/slide-sdk/utils/units"
)

const (
	// EthMsgSoftCapSize is the ideal size of encoded transaction bytes we send in
	// any [EthTxsGossip] or [AtomicTxGossip] message. We do not limit inbound messages to
	// this size, however. Max inbound message size is enforced by the codec
	// (512KB).
	EthMsgSoftCapSize = 64 * units.KiB
)

var (
	_ GossipMessage = TxsGossip{}
)

type GossipMessage interface {
	// types implementing GossipMessage should also implement fmt.Stringer for logging purposes.
	fmt.Stringer

	// Handle this gossip message with the gossip handler.
	Handle(handler GossipHandler, nodeID ids.NodeID) error
}

type TxsGossip struct {
	Txs []byte `serialize:"true"`
}

func (msg TxsGossip) Handle(handler GossipHandler, nodeID ids.NodeID) error {
	return handler.HandleTxs(nodeID, msg)
}

func (msg TxsGossip) String() string {
	return fmt.Sprintf("TxsGossip(Len=%d)", len(msg.Txs))
}

func ParseGossipMessage(codec codec.Manager, bytes []byte) (GossipMessage, error) {
	var msg GossipMessage
	err := codec.Unmarshal(bytes, &msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func BuildGossipMessage(codec codec.Manager, msg GossipMessage) ([]byte, error) {
	bytes, err := codec.Marshal(&msg)
	return bytes, err
}
