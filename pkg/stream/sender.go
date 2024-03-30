package stream

import (
	"github.com/MikhUd/blockchain/pkg/api/message"
	"github.com/MikhUd/blockchain/pkg/context"
	"net"
)

type Sender interface {
	Send(ctx *context.Context) error
	Addr() string
	WithConn(conn net.Conn) Sender
	Shutdown()
}

type Addressable interface {
	GetRemote() *message.PID
}
