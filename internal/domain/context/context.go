package context

import (
	"github.com/MikhUd/blockchain/cmd/blockchain_node/receiver"
	"github.com/MikhUd/blockchain/protos/node"
)

type NodeContext struct {
	pid      *node.PID
	sender   *node.PID
	receiver *receiver.Receiver
	msg      any
}

func (nc *NodeContext) PID() *node.PID {
	return nc.pid
}

func (nc *NodeContext) Sender() *node.PID {
	return nc.sender
}

func (nc *NodeContext) Receiver() *receiver.Receiver {
	return nc.receiver
}

func (nc *NodeContext) Msg() any {
	return nc.msg
}
