package cluster

import (
	"github.com/MikhUd/blockchain/pkg/infrastructure/context"
)

type Sender interface {
	Send(ctx *context.Context) error
}
