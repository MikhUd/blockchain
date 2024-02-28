package stream

import "github.com/MikhUd/blockchain/pkg/context"

type Sender interface {
	Send(ctx *context.Context) error
	Addr() string
}
