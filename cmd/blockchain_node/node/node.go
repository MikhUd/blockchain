package node

import (
	"github.com/MikhUd/blockchain/cmd/blockchain_node/stream_reader"
	"github.com/MikhUd/blockchain/internal/domain/context"
	"log/slog"
	"sync/atomic"
)

type Node struct {
	addr   string
	reader *stream_reader.Reader
	state  atomic.Uint32
	logger *slog.Logger
}

func (n *Node) Receive(ctx *context.NodeContext) {

}
