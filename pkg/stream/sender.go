package stream

import (
	"github.com/MikhUd/blockchain/pkg/api/message"
	"github.com/MikhUd/blockchain/pkg/context"
)

type Sender interface {
	Send(ctx *context.Context) error
	Addr() string
	Shutdown()
}

type Addressable interface {
	GetRemote() *message.PID
}
