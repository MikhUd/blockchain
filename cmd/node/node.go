package node

import (
	"context"
	"fmt"
	"github.com/MikhUd/blockchain/pkg/config"
	"github.com/MikhUd/blockchain/pkg/domain/blockchain"
	"github.com/MikhUd/blockchain/pkg/grpcapi/message"
	clusterContext "github.com/MikhUd/blockchain/pkg/infrastructure/context"
	"github.com/MikhUd/blockchain/pkg/utils"
	"log/slog"
	"net"
	"reflect"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
	"sync"
)

type Node struct {
	addr   string
	stopCh chan struct{}
	bc     *blockchain.Blockchain
	wg     sync.WaitGroup
	mu     sync.RWMutex
	cfg    config.Config
}

func NewNode(addr string, bc *blockchain.Blockchain, cfg config.Config) *Node {
	n := &Node{
		addr: addr,
		bc:   bc,
	}
	return n
}

func (n *Node) Blockchain() *blockchain.Blockchain {
	return n.bc
}

func (n *Node) Addr() string {
	return n.addr
}

func (n *Node) Receive(ctx *clusterContext.Context) error {
	switch msg := ctx.Msg().(type) {
	case *message.TransactionRequest:
		_ = n.AddTransaction(msg)
	default:
		panic(reflect.TypeOf(msg))
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
			slog.Error("server", "err", err)
		} else {
			slog.Debug("server stopped")
		}
	}()
	go func() {
		<-n.stopCh
		cancel()
	}()

	fmt.Printf("start node, addr:%s\n", n.addr)
	slog.Debug("listening node", "addr", n.addr)

	return nil
}

func (n *Node) AddTransaction(tr *message.TransactionRequest) error {
	slog.Info(fmt.Sprintf("count_before:%v\n", len(n.bc.TransactionPool())))
	publicKey := utils.PublicKeyFromString(tr.SenderPublicKey)
	privateKey := utils.PrivateKeyFromString(tr.SenderPrivateKey, publicKey)
	signature, err := utils.GenerateTransactionSignature(tr.SenderBlockchainAddress, privateKey, tr.RecipientBlockchainAddress, tr.Value)
	if err != nil {
		return err
	}
	success := n.bc.CreateTransaction(tr.SenderBlockchainAddress, tr.RecipientBlockchainAddress, tr.Value, publicKey, signature)
	slog.Info(fmt.Sprintf("is_transacted:%v\n", success))
	slog.Info(fmt.Sprintf("count:%v\n", len(n.bc.TransactionPool())))
	return nil
}
