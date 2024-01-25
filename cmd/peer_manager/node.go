package peer_manager

import (
	"context"
	"fmt"
	"github.com/MikhUd/blockchain/internal/domain/blockchain"
	"github.com/MikhUd/blockchain/internal/utils"
	bcproto "github.com/MikhUd/blockchain/protos/blockchain"
	"log/slog"
	"net"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
	"sync"
)

type Node struct {
	addr   string
	stopCh chan struct{}
	logger slog.Logger
	bc     *blockchain.Blockchain
	wg     sync.WaitGroup
	mu     sync.RWMutex
}

func NewNode(logger slog.Logger, addr string, bc *blockchain.Blockchain) *Node {
	n := &Node{
		addr:   addr,
		logger: logger,
		bc:     bc,
	}
	return n
}

func (n *Node) Blockchain() *blockchain.Blockchain {
	return n.bc
}

func (n *Node) Addr() string {
	return n.addr
}

func (n *Node) Receive(ctx *Context) error {
	switch msg := ctx.Msg().(type) {
	case *bcproto.TransactionRequest:
		_ = n.AddTransaction(msg)
	default:
		panic(123)
	}
	return nil
}

func (n *Node) Start() error {
	n.wg.Add(1)
	n.mu.Lock()

	listener, err := net.Listen("tcp", n.addr)
	if err != nil {
		return fmt.Errorf("failed to listen")
	}
	mux := drpcmux.New()
	server := drpcserver.New(mux)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer n.wg.Done()
		err := server.Serve(ctx, listener)
		if err != nil {
			n.logger.Error("server", "err", err)
		} else {
			n.logger.Debug("server stopped")
		}
	}()
	go func() {
		<-n.stopCh
		cancel()
	}()

	fmt.Printf("start node, addr:%s", n.addr)
	n.logger.Debug("listening node", "addr", n.addr)

	return nil
}

func (n *Node) AddTransaction(tr *bcproto.TransactionRequest) error {
	publicKey := utils.PublicKeyFromString(tr.SenderPublicKey)
	signature := utils.SignatureFromString(tr.Signature)
	n.bc.AddTransaction(tr.SenderBlockchainAddress, tr.RecipientBlockchainAddress, tr.Value, publicKey, signature)
	return nil
}
