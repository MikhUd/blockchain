package node

import (
	"context"
	"fmt"
	"github.com/MikhUd/blockchain/pkg/config"
	clusterContext "github.com/MikhUd/blockchain/pkg/context"
	"github.com/MikhUd/blockchain/pkg/domain/blockchain"
	"github.com/MikhUd/blockchain/pkg/grpcapi/message"
	"github.com/MikhUd/blockchain/pkg/grpcapi/remote"
	"github.com/MikhUd/blockchain/pkg/stream"
	"github.com/MikhUd/blockchain/pkg/utils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type Node struct {
	id          uint32
	addr        string
	stopCh      chan struct{}
	reader      *stream.Reader
	ctx         context.Context
	cancel      context.CancelFunc
	bc          *blockchain.Blockchain
	wg          sync.WaitGroup
	mu          sync.RWMutex
	dbConnStr   string
	dbPool      *pgxpool.Pool
	nodes       map[string]Node
	IsLeader    bool
	LeaderAddr  string
	clusterAddr string
	engine      *stream.Engine
	state       atomic.Uint32
}

type LeaderNode Node

const heartBeatInterval = time.Second * 5

func New(addr string, clusterAddr string, bc *blockchain.Blockchain) *Node {
	ctx, cancel := context.WithCancel(context.Background())
	n := &Node{
		id:          uuid.New().ID(),
		addr:        addr,
		ctx:         ctx,
		cancel:      cancel,
		bc:          bc,
		clusterAddr: clusterAddr,
		engine:      stream.NewEngine(nil),
	}
	n.reader = stream.NewReader(n)
	n.state.Store(config.Initialized)
	return n
}

func (n *Node) Blockchain() *blockchain.Blockchain {
	return n.bc
}

func (n *Node) Addr() string {
	return n.addr
}

func (n *Node) Receive(ctx *clusterContext.Context) error {
	var err error
	const op = "node.Receive"
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("node received new peer: %s", ctx.Sender().Addr))
	switch msg := ctx.Msg().(type) {
	case *message.TransactionRequest:
		return n.AddTransaction(msg)
	case *message.JoinResponse:
		err = n.handleJoinCluster(msg, ctx)
	case *message.HeartbeatResponse:
		err = n.handleHeartbeat(msg, ctx)
	}
	return err
}

func (n *Node) Start() error {
	const op = "node.Start"
	var (
		listener net.Listener
		err      error
	)
	if n.addr == "" {
		listener, err = net.Listen("tcp", ":0")
		n.addr = fmt.Sprintf(":%d", listener.Addr().(*net.TCPAddr).Port)
	} else {
		listener, err = net.Listen("tcp", n.addr)
	}
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("failed to listen:%s", err.Error()))
	}
	if n.state.Load() == config.Running {
		slog.With(slog.String("op", op)).Error("node already running")
		return fmt.Errorf("node already running")
	}
	n.state.Store(config.Running)
	mux := drpcmux.New()
	server := drpcserver.New(mux)
	err = remote.DRPCRegisterRemote(mux, n.reader)
	//n.dbPool, err = common.ConnectToDB(n.ctx, n.dbConnStr)
	if err != nil {
		return fmt.Errorf("failed to connect to database")
	}
	go func() {
		err = server.Serve(n.ctx, listener)
		if err != nil {
			slog.Error("server", "err", err)
		} else {
			slog.Debug("server stopped")
		}
	}()
	go func() {
		<-n.stopCh
		n.dbPool.Close()
		n.cancel()
	}()

	go func() {
		if err = n.dialCluster(); err != nil {
			//log.Fatal(err)
		}
	}()

	fmt.Printf("start node, addr: %s\n", n.addr)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
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

func (n *Node) dialCluster() error {
	var err error
	const op = "node.dialCluster"
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("try dialing with cluster: %s", n.clusterAddr))
	_, err = net.Dial("tcp", n.clusterAddr)
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("error dial cluster: %s", err.Error()))
		return err
	}
	req := &message.JoinRequest{MemberId: n.id, Address: n.addr, IsLeader: false}
	n.engine.AddWriter(stream.NewWriter(n.clusterAddr))
	ctx := clusterContext.New(req).WithReceiver(&message.PID{Addr: n.clusterAddr})
	err = n.engine.Send(ctx)
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("error send ctx to cluster: %s", err.Error()))
	}
	return err
}

func (n *Node) handleJoinCluster(msg *message.JoinResponse, ctx *clusterContext.Context) error {
	const op = "node.handleJoinCluster"
	if msg.GetAcknowledged() {
		slog.With(slog.String("op", op)).Info(fmt.Sprintf("successfully join with cluster: %s", msg.GetAddress()))
	} else {
		slog.With(slog.String("op", op)).Warn(fmt.Sprintf("cluster: %s declined join, stopping node: %s", msg.GetAddress(), n.addr))
		n.stop()
	}
	go func() {
		if err := n.periodHeartbeat(); err != nil {
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("heart beat error: %s", err.Error()))
		}
	}()
	return nil
}

func (n *Node) periodHeartbeat() error {
	const op = "node.periodHeartbeat"
	ticker := time.NewTicker(heartBeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := n.sendHeartbeat(n.clusterAddr); err != nil {
				slog.With(slog.String("op", op)).Error(fmt.Sprintf("failed to send heartbeat: %s", err.Error()))
			}
			// TODO heartbeat nodes by raft
		case <-n.ctx.Done():
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("node ctx is done, addr: %s", n.addr))
			return nil
		}
	}
}

func (n *Node) sendHeartbeat(addr string) error {
	var err error
	const op = "node.sendHeartbeat"
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("try send heartbeat: %s", addr))
	_, err = net.Dial("tcp", addr)
	if err != nil {
		slog.With(slog.String("op", op)).Error("error dial with addr:%s error: %s", addr, err.Error())
		return err
	}
	req := &message.HeartbeatRequest{MemberId: n.id, Address: n.addr}
	n.engine.AddWriter(stream.NewWriter(addr))
	ctx := clusterContext.New(req).WithReceiver(&message.PID{Addr: n.clusterAddr})
	err = n.engine.Send(ctx)
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("error send ctx to cluster: %s", err.Error()))
	}
	return err
}

func (n *Node) stop() {
	const op = "node.stop"
	if n.state.Load() != config.Running {
		slog.With(slog.String("op", op)).Warn(fmt.Sprintf("node %s already stopped", n.addr))
	}
	n.state.Store(config.Stopped)
	n.stopCh <- struct{}{}
}

func (n *Node) handleHeartbeat(msg *message.HeartbeatResponse, ctx *clusterContext.Context) error {
	var err error
	const op = "node.handleHeartbeat"
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("handle heart beat from: %s", msg.GetAddress()))
	if !msg.GetAcknowledged() {
		slog.With(slog.String("op", op)).Warn(fmt.Sprintf("decline ack from: %s", msg.GetAddress()))
		n.stop()
	}
	return err
}
