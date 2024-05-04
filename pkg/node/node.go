package node

import (
	drpcserver "
	"context"
	"crypto/tls"
	"fmt"
	"github.com/MikhUd/blockchain/pkg/api/message"
	"github.com/MikhUd/blockchain/pkg/api/remote"
	"github.com/MikhUd/blockchain/pkg/config"
	"github.com/MikhUd/blockchain/pkg/consts"
	clusterContext "github.com/MikhUd/blockchain/pkg/context"
	"github.com/MikhUd/blockchain/pkg/domain/blockchain"
	"github.com/MikhUd/blockchain/pkg/raft"
	"github.com/MikhUd/blockchain/pkg/serializer"
	"github.com/MikhUd/blockchain/pkg/status"
	"github.com/MikhUd/blockchain/pkg/stream"
	"github.com/MikhUd/blockchain/pkg/utils"
	"github.com/MikhUd/blockchain/pkg/waitgroup"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"storj.io/drpc/drpcmux"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type Node struct {
	id        string
	addr      string
	dbConnStr string
	cfg       config.Config
	tlsCfg    *tls.Config
	bc        *blockchain.Blockchain
	wg        *waitgroup.WaitGroup
	mu        sync.RWMutex
	dbPool    *pgxpool.Pool
	engine    *stream.Engine
	status    atomic.Uint32
	ln        net.Listener
	stopCh    chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
	raft      *raft.Raft
}

var (
	ser         serializer.ProtoSerializer
	dialTimeout = time.Millisecond * 200
)

func New(addr string, bc *blockchain.Blockchain, cfg config.Config) *Node {
	ctx, cancel := context.WithCancel(context.Background())
	n := &Node{
		id:     uuid.New().String(),
		addr:   addr,
		stopCh: make(chan struct{}, 1),
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
		bc:     bc,
		wg:     &waitgroup.WaitGroup{},
		engine: stream.NewEngine(addr),
	}
	n.status.Store(status.Initialized)
	return n
}

func (n *Node) WithTimeout(timeout time.Duration) *Node {
	n.ctx, n.cancel = context.WithTimeout(n.ctx, timeout)
	return n
}

func (n *Node) Start() error {
	var (
		op  = "node.Start"
		err error
	)
	if n.status.Load() == status.Running {
		slog.With(slog.String("op", op)).Error("node already running")
		return fmt.Errorf("node already running")
	}
	if n.addr == "" {
		n.ln, err = net.Listen("tcp", ":0")
		n.addr = fmt.Sprintf(":%d", n.ln.Addr().(*net.TCPAddr).Port)
	} else {
		n.ln, err = net.Listen("tcp", n.addr)
	}
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("failed to listen:%s", err.Error()))
	}
	n.status.Store(status.Running)
	mux := drpcmux.New()
	server := drpcserver.New(mux)
	err = remote.DRPCRegisterRemote(mux, stream.NewReader(n))
	//n.dbPool, err = common.ConnectToDB(n.ctx, n.dbConnStr)
	if err != nil {
		return fmt.Errorf("failed to connect to database")
	}
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		err = server.Serve(n.ctx, n.ln)
		if err != nil {
			slog.Error("server", "err", err)
		} else {
			slog.Debug("server stopped")
		}
	}()
	go func() {
		<-n.stopCh
		n.stop()
		//n.dbPool.Close()
	}()
	if _, dl := n.ctx.Deadline(); dl == true {
		go n.checkDeadline()
	}
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("node: %s started", n.addr))
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	for {
		select {
		case <-stop:
			fmt.Println("STOP TRIGG")
			if n.status.Load() != status.Stopped {
				if err = n.stop(); err != nil {
					slog.With(slog.String("op", op)).Error(fmt.Sprintf("error stopping node: %s", err.Error()))
				}
			}
			return err
		case <-n.stopCh:
			fmt.Println("STOPCH TRIGG")
			if n.status.Load() != status.Stopped {
				if err = n.stop(); err != nil {
					slog.With(slog.String("op", op)).Error(fmt.Sprintf("error stopping node: %s", err.Error()))
				}
			}
			return err
		}
	}
}

func (n *Node) checkDeadline() {
	select {
	case <-n.ctx.Done():
		n.stop()
		return
	}
}

func (n *Node) stop() error {
	var op = "node.stop"
	if n.status.Load() != status.Running {
		return fmt.Errorf(fmt.Sprintf("node %s already stopped", n.addr))
	}
	n.status.Store(status.Stopped)
	close(n.stopCh)
	n.ln.Close()
	go n.wg.PeriodicCheck(time.Second)
	n.wg.Wait()
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("node: %s stopped", n.addr))
	return nil
}

/*func (n *Node) dialCluster() {
	var op = "node.dialCluster"
	go func() {
		if err := n.dial(n.cluster.addr, nil, dialTimeout); err != nil {
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("error dial cluster: %s", n.cluster.addr))
			n.cluster.info.status.Store(status.Stopped)
			n.stopCh <- struct{}{}
			return
		}
		n.cluster.info.status.CompareAndSwap(status.Stopped, status.Running)
	}()
}*/

/*func (n *Node) tryRecover(addr string, maxAttempts uint8, attemptsCounter *uint8) error {
	var (
		op  = "node.tryRecover"
		err error
	)
	ticker := time.NewTicker(time.Millisecond * time.Duration(n.cfg.NodeRecoverTimeout))
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			*attemptsCounter++
			utils.CloseConn(n, addr)
			if *attemptsCounter > maxAttempts {
				slog.With(slog.String("op", op)).Warn(fmt.Sprintf("recover max attempts are over, addr: %s", addr))
				if addr == n.cluster.addr {
					n.stop()
				}
				return err
			}
			if err = n.dial(addr, nil, dialTimeout); err == nil {
				slog.With(slog.String("op", op)).Info(fmt.Sprintf("successfully recovered conn with addr: %s", addr))
				*attemptsCounter = 0
				if n.status.Load() != status.Running {
					n.Start()
				}
				return err
			}
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("recover failed, attempt number: %d, error: %v", *attemptsCounter, err))
		case <-n.stopCh:
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("node stopped, addr: %s", n.addr))
			return nil
		}
	}
}*/

func (n *Node) Blockchain() *blockchain.Blockchain {
	return n.bc
}

func (n *Node) Receive(ctx *clusterContext.Context) error {
	var err error
	switch msg := ctx.Msg().(type) {
	case *message.TransactionRequest:
		err = n.AddTransaction(msg)
		//case *message.JoinMessage:
		//	err = n.handleJoin(msg)
		//case *message.HeartbeatMessage:
		//	err = n.handleHeartbeat(msg)
		//case *message.ElectionMessage:
		//	err = n.handleVote(msg)
		//case *message.SetLeaderMessage:
		//	err = n.handleSetLeader(msg)
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

/*func (n *Node) dial(addr string, msg any, timeout time.Duration) error {
	var (
		op         = "node.dial"
		err        error
		dialResult = make(chan struct{}, 1)
		conn       net.Conn
	)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	switch n.tlsCfg {
	case nil:
		go func() {
			if _, ok := n.connPool[addr]; ok != true {
				conn, err = net.Dial("tcp", addr)
				if conn != nil {
					utils.SaveConn(n, addr, conn)
				}
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
				sendCtx := clusterContext.New(req).WithReceiver(&message.PID{Addr: addr})
				err = n.engine.Send(sendCtx)
				if err != nil {
					slog.With(slog.String("op", op)).Error(fmt.Sprintf("error send ctx: %s, NODE ADDR: %s", err.Error(), n.addr))
				}
				if member, ok := n.memberPool.members[addr]; ok {
					member.status.Store(status.Running)
				}
			} else {
				fmt.Println("GOT ERROR", err)
			}
			dialResult <- struct{}{}
		}()
		//TODO: implement tls
	}
	select {
	case <-dialResult:
		fmt.Println("DIAL RESULT, ADDR:", addr)
		if err != nil {
			return fmt.Errorf(fmt.Sprintf("error dialing: %s", err.Error()))
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf(fmt.Sprintf("dial timeout for address: %s", addr))
	}
}*/

/*func (n *Node) handleJoin(msg *message.JoinMessage) error {
	var (
		op          = "node.handleJoin"
		err         error
		msgData     = msg.GetData()
		msgTypeName = msg.GetTypeName()
		joinData    any
	)
	joinData, err = ser.Deserialize(msgData, msgTypeName)
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("error deserialize join message: %s, msg_type: %s", err.Error(), msgTypeName))
		return err
	}
	switch joinData := joinData.(type) {
	case *message.ClusterJoinMessage:
		return n.handleClusterJoin(joinData, msg.GetRemote().GetAddr())
	case *message.NodeJoinMessage:
		return n.handleNodeJoin(joinData, msg.GetRemote().GetAddr())
	}
	return err
}*/

/*func (n *Node) handleClusterJoin(msg *message.ClusterJoinMessage, clusterAddr string) error {
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
		return n.tryRecover(n.cluster.addr, n.cfg.MaxClusterJoinAttempts, &n.cluster.info.joinDeclines)
	}
	go func() {
		if err = n.startHeartbeat(n.cluster.addr, n.cfg.ClusterHeartbeatIntervalMs, n.cfg.MaxClusterHeartbeatMisses, &n.cluster.info.heartbeatMisses, true, n.cluster.info); err != nil {
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("heartbeat error for: %s, error: %s", n.cluster.addr, err.Error()))
		}
	}()
	if leaderNode != nil {
		slog.With(slog.String("op", op)).Info(fmt.Sprintf("found leader node in cluster: %s, node addr: %s", clusterAddr, leaderNode.GetAddr()))
		n.LeaderAddr = leaderNode.GetAddr()
	}
	if len(members) > 0 {
		n.memberPool.wg.Add(len(members))
		fmt.Println("start dial nodes, wg len:", n.memberPool.wg.GetCount())
		for _, neighbor := range members {
			if neighbor.GetAddr() == n.addr {
				n.memberPool.wg.Done()
				continue
			}
			go func(addr string) {
				if err = n.dial(addr, &message.NodeJoinMessage{}, dialTimeout); err != nil {
					slog.With(slog.String("op", op)).Error(fmt.Sprintf("error dial neighbor node: %s", addr))
				}
			}(neighbor.GetAddr())
			n.awaitAcceptingFromMember(neighbor.GetAddr())
			n.memberPool.wg.Done()
		}
		go n.memberPool.wg.PeriodicCheck(time.Second)
		if n.LeaderAddr == "" {
			n.startElection()
		}
	}
	return err
}*/

func (n *Node) awaitAcceptingFromMember(addr string) {
	go func(addr string) {
		member := n.initializeMember(addr)
		member.status.CompareAndSwap(status.Initialized, status.Pending)
		for {
			select {
			case <-member.acceptCh:
				utils.Print(consts.Green, fmt.Sprintf("member successfully joined:%s", addr))
				member.status.CompareAndSwap(status.Pending, status.Running)
				return
			case <-member.ctx.Done():
				member.status.CompareAndSwap(status.Pending, status.Stopped)
				utils.Print(consts.Red, fmt.Sprintf("member context is done:%s", addr))
				return
			}
		}
	}(addr)
}

func (n *Node) handleNodeJoin(msg *message.NodeJoinMessage, nodeAddr string) error {
	var (
		op  = "node.handleNodeJoin"
		err error
	)
	fmt.Println("NODE JOIN:", nodeAddr, "ACK:", msg.Acknowledged)
	for addr, node := range n.memberPool.members {
		if nodeAddr == addr && node.state.Load() == status.Running {
			return err
		}
	}
	if msg.GetAcknowledged() {
		n.acceptMember(nodeAddr)
	}
	member := n.initializeMember(nodeAddr)
	//TODO: check asking node before approve
	err = n.dial(nodeAddr, &message.NodeJoinMessage{Acknowledged: true}, dialTimeout)
	fmt.Println("AZAZA, NODE ADDR:", nodeAddr)
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("error dialed with node: %s, err:%s", nodeAddr, err.Error()))
		err = n.tryRecover(nodeAddr, n.cfg.MaxNodeHeartbeatMisses, &member.joinDeclines)
		if err != nil {
			return err
		}
	}
	return err
}

func (n *Node) handleHeartbeat(msg *message.HeartbeatMessage) error {
	var (
		err  error
		addr = msg.GetRemote().GetAddr()
	)
	fmt.Println("RECEIVE HEARTBEAT FROM:", msg.GetRemote().GetAddr())
	if msg.GetAcknowledged() == false {
		resp := &message.HeartbeatMessage{Remote: &message.PID{Id: n.id, Addr: n.addr}, Acknowledged: true}
		fmt.Println("send ack heartbeat to:", addr)
		ctx := clusterContext.New(resp).WithReceiver(&message.PID{Addr: addr})
		return n.engine.Send(ctx)
	} else {
		if member, ok := n.memberPool.members[addr]; ok {
			utils.Print(consts.Green, fmt.Sprintf("MEMBER ALIVE:%s HEARTBEAT MISSES SET TO ZERO", addr))
			member.heartbeatMisses = 0
		}
	}
	return err
}

func (n *Node) handleLostConnWithoutRecovering(connAddr string) error {
	fmt.Println("Handle lost conn without recovering", connAddr)
	for addr, node := range n.memberPool.members {
		if addr == connAddr {
			node.state.Store(status.Stopped)
			return utils.CloseConn(n, connAddr)
		}
	}
	return nil
}

func (n *Node) GetAddr() string {
	return n.addr
}
