package node

import (
	"context"
	"errors"
	"fmt"
	"github.com/MikhUd/blockchain/pkg/api/message"
	"github.com/MikhUd/blockchain/pkg/api/remote"
	"github.com/MikhUd/blockchain/pkg/config"
	clusterContext "github.com/MikhUd/blockchain/pkg/context"
	"github.com/MikhUd/blockchain/pkg/domain/blockchain"
	"github.com/MikhUd/blockchain/pkg/serializer"
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
	id                      string
	addr                    string
	cfg                     config.Config
	stopCh                  chan struct{}
	reader                  *stream.Reader
	ctx                     context.Context
	cancel                  context.CancelFunc
	bc                      *blockchain.Blockchain
	wg                      sync.WaitGroup
	mu                      sync.RWMutex
	dbConnStr               string
	dbPool                  *pgxpool.Pool
	LeaderAddr              string
	clusterAddr             string
	reserveClusterAddresses []string
	engine                  *stream.Engine
	state                   atomic.Uint32
	neighbors               []*NeighborInfo
	votes                   int
	electionTime            time.Duration
	leaderCh                chan struct{}
}

type NeighborInfo struct {
	addr  string
	state atomic.Uint32
}

func New(addr string, clusterAddr string, bc *blockchain.Blockchain, cfg config.Config) *Node {
	ctx, cancel := context.WithCancel(context.Background())
	n := &Node{
		id:           uuid.New().String(),
		addr:         addr,
		stopCh:       make(chan struct{}),
		cfg:          cfg,
		ctx:          ctx,
		cancel:       cancel,
		bc:           bc,
		clusterAddr:  clusterAddr,
		engine:       stream.NewEngine(nil),
		electionTime: time.Duration(utils.RandElection(cfg)) * time.Millisecond,
		leaderCh:     make(chan struct{}),
	}
	n.reader = stream.NewReader(n)
	n.state.Store(config.Initialized)
	return n
}

var ser serializer.ProtoSerializer

func (n *Node) Blockchain() *blockchain.Blockchain {
	return n.bc
}

func (n *Node) Addr() string {
	return n.addr
}

func (n *Node) Receive(ctx *clusterContext.Context) error {
	var err error
	switch msg := ctx.Msg().(type) {
	case *message.TransactionRequest:
		err = n.AddTransaction(msg)
	case *message.JoinMessage:
		err = n.handleJoin(msg)
	case *message.HeartbeatMessage:
		err = n.handleHeartbeat(msg)
	case *message.ElectionMessage:
		err = n.handleVote(msg)
	case *message.SetLeaderMessage:
		err = n.handleSetLeader(msg)
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
		slog.With(slog.String("op", op)).Info(fmt.Sprintf("node: %s stopped", n.addr))
		n.dbPool.Close()
		n.cancel()
	}()

	go func() {
		if err = n.dial(n.clusterAddr, nil); err != nil {
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("error dial cluster: %s node: %s stopped", n.clusterAddr, n.addr))
			n.stopCh <- struct{}{}
		}
	}()

	slog.With(slog.String("op", op)).Info(fmt.Sprintf("node: %s started, election time: %s", n.addr, n.electionTime.String()))
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	return nil
}

func (n *Node) AddTransaction(tr *message.TransactionRequest) error {
	publicKey := utils.PublicKeyFromString(tr.SenderPublicKey)
	privateKey := utils.PrivateKeyFromString(tr.SenderPrivateKey, publicKey)
	signature, err := utils.GenerateTransactionSignature(tr.SenderBlockchainAddress, privateKey, tr.RecipientBlockchainAddress, tr.Value)
	if err != nil {
		return err
	}
	_ = n.bc.CreateTransaction(tr.SenderBlockchainAddress, tr.RecipientBlockchainAddress, tr.Value, publicKey, signature)
	return nil
}

func (n *Node) dial(addr string, msg any) error {
	var err error
	const op = "node.dial"
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("try dialing with addr: %s", addr))
	_, err = net.Dial("tcp", addr)
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("error dialing: %s", err.Error()))
		return err
	}
	req := &message.JoinMessage{Remote: &message.PID{Id: n.id, Addr: n.addr}}
	if msg != nil {
		data, err := ser.Serialize(msg)
		if err != nil {
			slog.With("op", op).Error(fmt.Sprintf("error serialize message: %s", err.Error()))
			return err
		}
		req.Data = data
		req.TypeName = ser.TypeName(msg)
	}
	n.engine.AddWriter(stream.NewWriter(addr))
	ctx := clusterContext.New(req).WithReceiver(&message.PID{Addr: addr})
	err = n.engine.Send(ctx)
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("error send ctx: %s", err.Error()))
	}
	return err
}

func (n *Node) handleJoin(msg *message.JoinMessage) error {
	var (
		err         error
		msgData     = msg.GetData()
		msgTypeName = msg.GetTypeName()
		joinData    any
	)
	const op = "node.handleJoin"
	joinData, err = ser.Deserialize(msgData, msgTypeName)
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("error deserialize join message: %s", err.Error()))
		return err
	}
	switch joinData := joinData.(type) {
	case *message.ClusterJoinMessage:
		return n.handleClusterJoin(joinData, msg.GetRemote().GetAddr())
	case *message.NodeJoinMessage:
		return n.handleNodeJoin(joinData, msg.GetRemote().GetAddr())
	}
	return err
}

func (n *Node) handleClusterJoin(msg *message.ClusterJoinMessage, clusterAddr string) error {
	var err error
	const op = "node.handleClusterJoin"
	var (
		neighbors  = msg.GetNeighbors()
		leaderNode = msg.GetLeaderNode()
	)
	if msg.GetAcknowledged() {
		slog.With(slog.String("op", op)).Info(fmt.Sprintf("successfully join with: %s", clusterAddr))
	} else {
		slog.With(slog.String("op", op)).Warn(fmt.Sprintf("cluster %s declined join, stopping node: %s", clusterAddr, n.addr))
		n.stop()
	}
	go func() {
		if err := n.periodHeartbeat(n.clusterAddr, n.cfg.NodeHeartBeatInterval); err != nil {
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("heart beat error: %s", err.Error()))
			return
		}
	}()
	if leaderNode != nil {
		slog.With(slog.String("op", op)).Info(fmt.Sprintf("found leader node in cluster: %s, node addr: %s", clusterAddr, leaderNode.GetAddr()))
		go func(addr string) {
			if err = n.dial(addr, &message.NodeJoinMessage{}); err != nil {
				slog.With(slog.String("op", op)).Error(fmt.Sprintf("error dial leader node: %s", addr))
				return
			}
			n.LeaderAddr = addr
		}(leaderNode.GetAddr())
	}
	if len(neighbors) > 0 {
		for _, neighbor := range neighbors {
			go func(addr string) {
				if err = n.dial(addr, &message.NodeJoinMessage{}); err != nil {
					slog.With(slog.String("op", op)).Error(fmt.Sprintf("error dial neighbor node: %s", addr))
					return
				}
			}(neighbor.GetAddr())
		}
	}
	if n.LeaderAddr == "" {
		go func() {
			if err = n.election(); err != nil {
				slog.With(slog.String("op", op)).Error(fmt.Sprintf("election error: %s", err.Error()))
				return
			}
		}()
	}
	return err
}

func (n *Node) handleNodeJoin(msg *message.NodeJoinMessage, nodeAddr string) error {
	var err error
	const op = "node.handleNodeJoin"
	for _, neighbor := range n.neighbors {
		if nodeAddr == neighbor.addr {
			return err
		}
	}
	if msg.GetAcknowledged() {
		n.mu.Lock()
		neighbor := &NeighborInfo{addr: nodeAddr}
		neighbor.state.Store(config.Active)
		n.neighbors = append(n.neighbors, neighbor)
		n.mu.Unlock()
		slog.With(slog.String("op", op)).Info(fmt.Sprintf("successfully dialed with node, append new neighbor node: %s", nodeAddr))
	}
	//TODO: check asking node before approve
	return n.dial(nodeAddr, &message.NodeJoinMessage{Acknowledged: true})
}

func (n *Node) periodHeartbeat(addr string, duration int) error {
	const op = "node.periodHeartbeat"
	ticker := time.NewTicker(time.Second * time.Duration(duration))
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := n.sendHeartbeat(addr); err != nil {
				slog.With(slog.String("op", op)).Error(fmt.Sprintf("failed to send heartbeat: %s", err.Error()))
			}
		case <-n.ctx.Done():
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("node ctx is done, addr: %s", n.addr))
			return nil
		}
	}
}

func (n *Node) sendHeartbeat(addr string) error {
	var err error
	const op = "node.sendHeartbeat"
	_, err = net.Dial("tcp", addr)
	if err != nil {
		slog.With(slog.String("op", op)).Error("error dial with addr:%s error: %s", addr, err.Error())
		return err
	}
	msg := &message.HeartbeatMessage{Remote: &message.PID{Id: n.id, Addr: n.addr}}
	n.engine.AddWriter(stream.NewWriter(addr))
	ctx := clusterContext.New(msg).WithReceiver(&message.PID{Addr: addr})
	err = n.engine.Send(ctx)
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("error send ctx: %s", err.Error()))
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

func (n *Node) handleHeartbeat(msg *message.HeartbeatMessage) error {
	var (
		err        error
		remoteAddr = msg.GetRemote().GetAddr()
	)
	const op = "node.handleHeartbeat"
	if !msg.GetAcknowledged() {
		slog.With(slog.String("op", op)).Warn(fmt.Sprintf("decline ack from: %s", remoteAddr))
		n.stop()
	}
	return err
}

func (n *Node) election() error {
	var err error
	const op = "node.election"
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("no leader node in cluster, starting election on node: %s, election time: %s", n.addr, n.electionTime.String()))
	n.votes = 0
	ticker := time.NewTicker(n.electionTime)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			err = n.voteBroadcast()
			if err != nil {
				slog.With(slog.String("op", op)).Error(fmt.Sprintf("vote broadcast err: %s", err.Error()))
			}
		case <-n.leaderCh:
			slog.With(slog.String("op", op)).Info(fmt.Sprintf("leader selected, stopping election on node: %s", n.addr))
			return err
		case <-n.ctx.Done():
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("node ctx is done, addr: %s", n.addr))
			return err
		}
	}
}

func (n *Node) voteBroadcast() error {
	var err error
	const op = "node.voteBroadcast"
	n.wg.Add(len(n.neighbors))
	for _, neighbor := range n.neighbors {
		go func(neighbor *NeighborInfo) {
			if neighbor.state.Load() != config.Active {
				n.wg.Done()
				return
			}
			err = n.requestVote(neighbor.addr)
			if err != nil {
				slog.With(slog.String("op", op)).Error(fmt.Sprintf("send vote error: %s", err.Error()))
			}
			if errors.Is(err, clusterContext.Refused) {
				ok := neighbor.state.CompareAndSwap(config.Active, config.Inactive)
				if ok {
					slog.With(slog.String("op", op)).Error(fmt.Sprintf("neighbor %s refuse connection, set inactive status", neighbor.addr))
				}
			}
			n.wg.Done()
		}(neighbor)
	}
	n.wg.Wait()
	return err
}

func (n *Node) requestVote(addr string) error {
	var err error
	const op = "node.requestVote"
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("send vote to: %s", addr))
	msg := &message.ElectionMessage{Remote: &message.PID{Id: n.id, Addr: n.addr}}
	n.engine.AddWriter(stream.NewWriter(addr))
	ctx := clusterContext.New(msg).WithReceiver(&message.PID{Addr: addr})
	err = n.engine.Send(ctx)
	return err
}

func (n *Node) handleVote(msg *message.ElectionMessage) error {
	var (
		err error
		ack bool
	)
	const op = "node.handleVote"
	ack = true
	if msg.GetAcknowledged() == true {
		n.votes++
		if n.votes > len(n.neighbors)/2 {
			slog.With(slog.String("op", op)).Info(fmt.Sprintf("this node become leader: %s", n.addr))
			n.wg.Add(len(n.neighbors))
			for _, neighbor := range n.neighbors {
				go func(neighbor *NeighborInfo) {
					err = n.sendSetLeader(neighbor.addr)
					if err != nil {
						slog.With(slog.String("op", op)).Error(fmt.Sprintf("send set leader error: %s", err.Error()))
					}
					n.wg.Done()
				}(neighbor)
			}
			n.wg.Wait()
		}
		return err
	}
	if n.LeaderAddr != "" {
		ack = false
	}
	msg = &message.ElectionMessage{Acknowledged: ack, Remote: &message.PID{Id: n.id, Addr: n.addr}}
	n.engine.AddWriter(stream.NewWriter(msg.GetRemote().GetAddr()))
	ctx := clusterContext.New(msg).WithReceiver(&message.PID{Addr: msg.GetRemote().GetAddr()})
	err = n.engine.Send(ctx)
	return err
}

func (n *Node) sendSetLeader(addr string) error {
	msg := &message.SetLeaderMessage{Remote: &message.PID{Id: n.id, Addr: n.addr}}
	n.engine.AddWriter(stream.NewWriter(addr))
	ctx := clusterContext.New(msg).WithReceiver(&message.PID{Addr: addr})
	n.leaderCh <- struct{}{}
	return n.engine.Send(ctx)
}

func (n *Node) handleSetLeader(msg *message.SetLeaderMessage) error {
	var err error
	const op = "node.handleSetLeader"
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("setting new leader node: %s", msg.GetRemote().GetAddr()))
	n.LeaderAddr = msg.GetRemote().GetAddr()
	n.leaderCh <- struct{}{}
	return err
}
