package node

import (
	"context"
	"crypto/tls"
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
	id              string
	addr            string
	cfg             config.Config
	tlsCfg          *tls.Config
	stopCh          chan struct{}
	reader          *stream.Reader
	ctx             context.Context
	cancel          context.CancelFunc
	bc              *blockchain.Blockchain
	wg              sync.WaitGroup
	mu              sync.RWMutex
	dbConnStr       string
	dbPool          *pgxpool.Pool
	LeaderAddr      string
	cluster         *ClusterInfo
	reserveClusters []*MemberInfo
	engine          *stream.Engine
	state           atomic.Uint32
	members         map[string]*MemberInfo
	activeMembers   map[string]struct{}
	votes           map[string]struct{}
	electionTime    time.Duration
	leaderCh        chan struct{}
	connPool        map[string]net.Conn
}

type MemberInfo struct {
	heartbeatMisses uint8
	state           atomic.Uint32
}

type ClusterInfo struct {
	addr string
	info *MemberInfo
}

var (
	ser         serializer.ProtoSerializer
	dialTimeout = time.Millisecond * 200
)

func New(addr string, clusterAddr string, bc *blockchain.Blockchain, cfg config.Config) *Node {
	ctx, cancel := context.WithCancel(context.Background())
	n := &Node{
		id:            uuid.New().String(),
		addr:          addr,
		stopCh:        make(chan struct{}),
		cfg:           cfg,
		ctx:           ctx,
		cancel:        cancel,
		bc:            bc,
		cluster:       &ClusterInfo{addr: clusterAddr, info: &MemberInfo{heartbeatMisses: 0}},
		engine:        stream.NewEngine(nil),
		electionTime:  time.Duration(utils.RandElection(cfg)) * time.Millisecond,
		leaderCh:      make(chan struct{}),
		members:       make(map[string]*MemberInfo),
		activeMembers: make(map[string]struct{}),
		votes:         make(map[string]struct{}),
		connPool:      make(map[string]net.Conn),
	}
	n.reader = stream.NewReader(n)
	n.state.Store(config.Initialized)
	return n
}

func (n *Node) GetMutex() *sync.RWMutex {
	return &n.mu
}

func (n *Node) GetConnPool() map[string]net.Conn {
	return n.connPool
}

func (n *Node) GetEngine() *stream.Engine {
	return n.engine
}

func (n *Node) Start() error {
	var (
		op       = "node.Start"
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
		n.stop()
		//n.dbPool.Close()
	}()
	n.dialCluster()
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("node: %s started, election time: %s", n.addr, n.electionTime.String()))
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	return nil
}

func (n *Node) stop() {
	var op = "node.stop"
	if n.state.Load() != config.Running {
		slog.With(slog.String("op", op)).Warn(fmt.Sprintf("node %s already stopped", n.addr))
	}
	n.state.Store(config.Stopped)
	go func() {
		_ = n.tryRecover(n.cluster.addr, n.cfg.MaxClusterHeartbeatMisses, &n.cluster.info.heartbeatMisses)
	}()
}

func (n *Node) dialCluster() {
	var op = "node.dialCluster"
	go func() {
		if err := n.dial(n.cluster.addr, nil, dialTimeout); err != nil {
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("error dial cluster: %s", n.cluster.addr))
			n.stopCh <- struct{}{}
		}
	}()
}

func (n *Node) tryRecover(addr string, maxAttempts uint8, attemptsCounter *uint8) error {
	var (
		op  = "node.tryRecover"
		err error
	)
	if attemptsCounter == nil {
		*attemptsCounter = 0
	}
	ticker := time.NewTicker(time.Millisecond * time.Duration(n.cfg.NodeRecoverTimeout))
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			*attemptsCounter++
			_ = utils.CloseConn(n, addr)
			if err = n.dial(addr, nil, dialTimeout); err == nil {
				slog.With(slog.String("op", op)).Info(fmt.Sprintf("successfully recovered node: %s", n.addr))
				*attemptsCounter = 0
				n.state.Store(config.Running)
				return err
			}
			if *attemptsCounter > maxAttempts {
				slog.With(slog.String("op", op)).Warn(fmt.Sprintf("recover max attempts are over, addr: %s", addr))
				return err
			}
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("recover failed, attempt number: %d", *attemptsCounter))
		case <-n.ctx.Done():
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("node ctx is done, addr: %s", n.addr))
			return nil
		}
	}
}

func (n *Node) Blockchain() *blockchain.Blockchain {
	return n.bc
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

func (n *Node) dial(addr string, msg any, timeout time.Duration) error {
	var (
		op         = "node.dial"
		err        error
		dialResult = make(chan struct{})
		conn       net.Conn
	)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	switch n.tlsCfg {
	case nil:
		go func() {
			if connFromPool, ok := n.connPool[addr]; ok != true {
				conn, err = net.Dial("tcp", addr)
				if conn != nil {
					utils.SaveConn(n, addr, conn)
				}
			} else {
				conn = connFromPool
			}
			if err == nil {
				req := &message.JoinMessage{Remote: &message.PID{Id: n.id, Addr: n.addr}}
				if msg != nil {
					data, err := ser.Serialize(msg)
					if err != nil {
						slog.With("op", op).Error(fmt.Sprintf("error serialize message: %s", err.Error()))
						dialResult <- struct{}{}
						return
					}
					req.Data = data
					req.TypeName = ser.TypeName(msg)
				}
				n.engine.AddWriter(stream.NewWriter(addr).WithConn(conn))
				sendCtx := clusterContext.New(req).WithReceiver(&message.PID{Addr: addr})
				err = n.engine.Send(sendCtx)
				if err != nil {
					slog.With(slog.String("op", op)).Error(fmt.Sprintf("error send ctx: %s", err.Error()))
				}
			}
			dialResult <- struct{}{}
		}()
		//TODO: implement tls
	}
	select {
	case <-dialResult:
		if err != nil {
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("error dialing: %s", err.Error()))
			return err
		}
	case <-ctx.Done():
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("dial timeout for address: %s", addr))
		return ctx.Err()
	}
	return err
}

func (n *Node) handleJoin(msg *message.JoinMessage) error {
	var (
		op          = "node.handleJoin"
		err         error
		msgData     = msg.GetData()
		msgTypeName = msg.GetTypeName()
		joinData    any
	)
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
	var (
		op  = "node.handleClusterJoin"
		err error
	)
	var (
		members    = msg.GetMembers()
		leaderNode = msg.GetLeaderNode()
	)
	if msg.GetAcknowledged() {
		slog.With(slog.String("op", op)).Info(fmt.Sprintf("successfully join with: %s", clusterAddr))
	} else {
		slog.With(slog.String("op", op)).Warn(fmt.Sprintf("cluster %s declined join, stopping node: %s", clusterAddr, n.addr))
		n.stop()
	}
	go func() {
		if err = n.startHeartbeat(n.cluster.addr, n.cfg.ClusterHeartbeatIntervalMs, n.cfg.MaxClusterHeartbeatMisses, &n.cluster.info.heartbeatMisses, true); err != nil {
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("heartbeat error for: %s, error: %s", n.cluster.addr, err.Error()))
		}
	}()
	if leaderNode != nil {
		slog.With(slog.String("op", op)).Info(fmt.Sprintf("found leader node in cluster: %s, node addr: %s", clusterAddr, leaderNode.GetAddr()))
		n.LeaderAddr = leaderNode.GetAddr()
	}
	if len(members) > 0 {
		n.wg.Add(len(members))
		for _, neighbor := range members {
			go func(addr string) {
				defer n.wg.Done()
				if err = n.dial(addr, &message.NodeJoinMessage{}, dialTimeout); err != nil {
					slog.With(slog.String("op", op)).Error(fmt.Sprintf("error dial neighbor node: %s", addr))
					return
				}
			}(neighbor.GetAddr())
		}
		n.wg.Wait()
		if n.LeaderAddr == "" {
			n.startElection()
		}
	}
	return err
}

func (n *Node) heartbeat(addr string, interval int, errCh chan<- struct{}) {
	var err error
	if err = n.periodHeartbeat(addr, interval); err != nil {
		errCh <- struct{}{}
	}
	close(errCh)
}

func (n *Node) startElection() {
	var (
		op  = "node.startElection"
		err error
	)
	go func() {
		if err = n.election(); err != nil {
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("election error: %s", err.Error()))
			return
		}
	}()
}

func (n *Node) startHeartbeat(addr string, interval int, maxAttempts uint8, attemptsCounter *uint8, recover bool) error {
	var (
		op    = "node.startHeartbeat"
		errCh = make(chan struct{})
		err   error
	)
	go n.heartbeat(addr, interval, errCh)
	<-errCh
	if addr == n.cluster.addr {
		n.LeaderAddr = ""
		n.members = make(map[string]*MemberInfo)
	}
	member, ok := n.members[addr]
	if ok == true {
		member.state.Store(config.Inactive)
		if _, ok := n.activeMembers[addr]; ok == true {
			delete(n.activeMembers, addr)
		}
	}
	if addr == n.LeaderAddr {
		n.LeaderAddr = ""
		n.startElection()
	}
	if recover == true {
		err = n.tryRecover(addr, maxAttempts, attemptsCounter)
		if err != nil {
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("stopping heartbeat for: %s", addr))
			return err
		}
	}
	return n.handleLostConnWithoutRecovering(addr)
}

func (n *Node) handleNodeJoin(msg *message.NodeJoinMessage, nodeAddr string) error {
	var (
		op       = "node.handleNodeJoin"
		err      error
		neighbor *MemberInfo
	)
	for addr, node := range n.members {
		if nodeAddr == addr && node.state.Load() == config.Active {
			return err
		}
	}
	if msg.GetAcknowledged() {
		n.mu.Lock()
		neighbor = &MemberInfo{heartbeatMisses: 0}
		neighbor.state.Store(config.Active)
		n.members[nodeAddr] = neighbor
		n.activeMembers[nodeAddr] = struct{}{}
		n.mu.Unlock()
		slog.With(slog.String("op", op)).Info(fmt.Sprintf("successfully dialed with node, append new neighbor node: %s", nodeAddr))
		go func() {
			if err = n.startHeartbeat(nodeAddr, n.cfg.NodeHeartbeatIntervalMs, uint8(n.cfg.MaxNodeHeartbeatMisses), &neighbor.heartbeatMisses, false); err != nil {
				slog.With(slog.String("op", op)).Error(fmt.Sprintf("heartbeat error for: %s, error: %s", nodeAddr, err.Error()))
			}
		}()
	}
	//TODO: check asking node before approve
	err = n.dial(nodeAddr, &message.NodeJoinMessage{Acknowledged: true}, dialTimeout)
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("error dialed with node: %s", nodeAddr))
		return err
	}
	return err
}

func (n *Node) periodHeartbeat(addr string, duration int) error {
	var op = "node.periodHeartbeat"
	ticker := time.NewTicker(time.Millisecond * time.Duration(duration))
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := n.sendHeartbeat(addr); err != nil {
				slog.With(slog.String("op", op)).Error(fmt.Sprintf("failed to send heartbeat: %s", err.Error()))
				return fmt.Errorf(fmt.Sprintf("heartbeat to addr: %s failed", addr))
			}
		case <-n.ctx.Done():
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("node ctx is done, addr: %s", n.addr))
			return nil
		}
	}
}

func (n *Node) sendHeartbeat(addr string) error {
	var (
		op  = "node.sendHeartbeat"
		err error
	)
	_, err = net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	//TODO: check before acknowledge
	msg := &message.HeartbeatMessage{Remote: &message.PID{Id: n.id, Addr: n.addr}, Acknowledged: true}
	n.engine.AddWriter(stream.NewWriter(addr))
	ctx := clusterContext.New(msg).WithReceiver(&message.PID{Addr: addr})
	err = n.engine.Send(ctx)
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("error send ctx: %s", err.Error()))
	}
	return err
}

func (n *Node) handleHeartbeat(msg *message.HeartbeatMessage) error {
	var (
		op         = "node.handleHeartbeat"
		err        error
		remoteAddr = msg.GetRemote().GetAddr()
	)
	if !msg.GetAcknowledged() {
		slog.With(slog.String("op", op)).Warn(fmt.Sprintf("decline ack from: %s", remoteAddr))
	}
	return err
}

func (n *Node) election() error {
	var (
		op  = "node.election"
		err error
	)
	n.electionTime = time.Duration(utils.RandElection(n.cfg)) * time.Millisecond
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("no leader node in cluster, starting election, election time: %s, members count: %d", n.electionTime.String(), len(n.members)))
	n.votes = make(map[string]struct{})
	ticker := time.NewTicker(n.electionTime)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			err = n.broadcastVote()
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

func (n *Node) broadcastVote() error {
	var (
		op  = "node.voteBroadcast"
		err error
	)
	n.wg.Add(len(n.members))
	for addr, node := range n.members {
		go func(addr string, node *MemberInfo) {
			if node.state.Load() != config.Active {
				n.wg.Done()
				return
			}
			err = n.requestVote(addr)
			if err != nil {
				slog.With(slog.String("op", op)).Error(fmt.Sprintf("send vote error: %s", err.Error()))
			}
			if errors.Is(err, clusterContext.Refused) {
				ok := node.state.CompareAndSwap(config.Active, config.Inactive)
				if ok {
					slog.With(slog.String("op", op)).Error(fmt.Sprintf("neighbor %s refuse connection, set inactive status", addr))
				}
			}
			n.wg.Done()
		}(addr, node)
	}
	n.wg.Wait()
	return err
}

func (n *Node) requestVote(addr string) error {
	var (
		err error
	)
	msg := &message.ElectionMessage{Remote: &message.PID{Id: n.id, Addr: n.addr}}
	n.engine.AddWriter(stream.NewWriter(addr))
	ctx := clusterContext.New(msg).WithReceiver(&message.PID{Addr: addr})
	err = n.engine.Send(ctx)
	return err
}

func (n *Node) handleVote(msg *message.ElectionMessage) error {
	var (
		op   = "node.handleVote"
		addr = msg.GetRemote().GetAddr()
		err  error
	)
	ack := true
	//n.mu.Lock()
	//defer n.mu.Unlock()
	fmt.Println("VOTE FROM", msg.GetRemote().GetAddr())
	fmt.Println("VOTES COUNT", len(n.votes))
	fmt.Println("ACK", msg.GetAcknowledged())
	if msg.GetAcknowledged() == true {
		n.votes[addr] = struct{}{}
		if len(n.votes) > len(n.activeMembers)/2 {
			slog.With(slog.String("op", op)).Info(fmt.Sprintf("this node becomes new leader: %s", n.addr))
			n.broadcastSetLeader()
		}
		return nil
	}
	if n.LeaderAddr != "" {
		fmt.Println("LEADER ADDR NOT NIL")
		ack = false
	}
	msg = &message.ElectionMessage{Acknowledged: ack, Remote: &message.PID{Id: n.id, Addr: n.addr}}
	n.engine.AddWriter(stream.NewWriter(msg.GetRemote().GetAddr()))
	ctx := clusterContext.New(msg).WithReceiver(&message.PID{Addr: msg.GetRemote().GetAddr()})
	err = n.engine.Send(ctx)
	return err
}

func (n *Node) broadcastSetLeader() {
	var op = "node.broadcastSetLeader"
	n.wg.Add(len(n.members) + 1)
	for addr := range n.activeMembers {
		go func(addr string) {
			defer n.wg.Done()
			if err := n.sendSetLeader(addr); err != nil {
				slog.With(slog.String("op", op)).Error(fmt.Sprintf("send set leader error: %s", err.Error()))
			}
		}(addr)
	}
	go func() {
		defer n.wg.Done()
		if err := n.sendSetLeader(n.cluster.addr); err != nil {
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("send set leader error: %s", err.Error()))
		}
	}()
	n.wg.Wait()
	n.leaderCh <- struct{}{}
}

func (n *Node) sendSetLeader(addr string) error {
	msg := &message.SetLeaderMessage{Remote: &message.PID{Id: n.id, Addr: n.addr}}
	n.engine.AddWriter(stream.NewWriter(addr))
	ctx := clusterContext.New(msg).WithReceiver(&message.PID{Addr: addr})
	return n.engine.Send(ctx)
}

func (n *Node) handleSetLeader(msg *message.SetLeaderMessage) error {
	var (
		op  = "node.handleSetLeader"
		err error
	)
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("setting new leader node: %s", msg.GetRemote().GetAddr()))
	n.LeaderAddr = msg.GetRemote().GetAddr()
	n.leaderCh <- struct{}{}
	return err
}

func (n *Node) handleLostConnWithoutRecovering(connAddr string) error {
	for addr, node := range n.members {
		if addr == connAddr {
			node.state.Store(config.Inactive)
			return utils.CloseConn(n, connAddr)
		}
	}
	return nil
}
