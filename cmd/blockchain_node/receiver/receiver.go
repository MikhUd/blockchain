package receiver

import "github.com/MikhUd/blockchain/internal/domain/context"

type Receiver interface {
	Receive(ctx *context.NodeContext)
}
