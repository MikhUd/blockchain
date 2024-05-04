package cluster

import (
	"context"
	"fmt"
	"github.com/MikhUd/blockchain/pkg/api/message"
	"github.com/MikhUd/blockchain/pkg/api/remote"
	"github.com/MikhUd/blockchain/pkg/config"
	clusterContext "github.com/MikhUd/blockchain/pkg/context"
	"github.com/MikhUd/blockchain/pkg/serializer"
	"github.com/MikhUd/blockchain/pkg/status"
	"github.com/MikhUd/blockchain/pkg/stream"
	"github.com/MikhUd/blockchain/pkg/utils"
	"github.com/MikhUd/blockchain/pkg/waitgroup"
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

type Cluster struct {
	addr       string
	cfg        config.Config
	ctx        context.Context
	cancel     context.CancelFunc
	stopCh     chan struct{}
	reader     *stream.Reader
	wg         *waitgroup.WaitGroup
	leaderNode *nodeInfo
	nodes      map[string]*nodeInfo
	state      atomic.Uint32
	Engine     *stream.Engine
	nodesMutex sync.RWMutex
	mu         sync.RWMutex
	connPool   map[string]net.Conn
	ln         net.Listener
}

type nodeInfo struct {
	addr            string
	heartbeatMisses uint8
	state           atomic.Uint32
}

var ser serializer.ProtoSerializer

func New(cfg config.Config, addr string) *Cluster {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Cluster{
		addr:     addr,
		cfg:      cfg,
		ctx:      ctx,
		cancel:   cancel,
		stopCh:   make(chan struct{}, 1),
		wg:       &waitgroup.WaitGroup{},
		nodes:    make(map[string]*nodeInfo),
		connPool: make(map[string]net.Conn),
	}
	c.reader = stream.NewReader(c)
	c.state.Store(status.Initialized)
	c.Engine = stream.NewEngine(addr)
	return c
}

func (c *Cluster) GetAddr() string {
	return c.addr
}

func (c *Cluster) WithTimeout(timeout time.Duration) *Cluster {
	c.ctx, c.cancel = context.WithTimeout(c.ctx, timeout)
	return c
}

func (c *Cluster) GetMutex() *sync.RWMutex {
	return &c.mu
}

func (c *Cluster) GetConnPool() map[string]net.Conn {
	return c.connPool
}

func (c *Cluster) GetEngine() *stream.Engine {
	return c.Engine
}

func (c *Cluster) Start() error {
	var (
		op  = "cluster.Start"
		err error
	)
	fmt.Println("CLUSTER START")
	if c.state.Load() == status.Running {
		slog.With(slog.String("op", op)).Error("cluster already running")
		return fmt.Errorf("cluster already running")
	}
	c.ln, err = net.Listen("tcp", c.addr)
	if err != nil {
		return fmt.Errorf("failed to listen")
	}
	mux := drpcmux.New()
	server := drpcserver.New(mux)
	err = remote.DRPCRegisterRemote(mux, c.reader)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		err = server.Serve(c.ctx, c.ln)
		if err != nil {
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("listener serve error: %s", err))
		} else {
			slog.With(slog.String("op", op)).Debug("server stopped")
		}
		fmt.Println("wgggg123 done")
	}()
	if _, dl := c.ctx.Deadline(); dl == true {
		go func() {
			c.checkDeadline()
		}()
	}
	go func() {
		c.manageNodes()
	}()
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("start cluster, addr: %s", c.addr))
	c.state.Store(status.Running)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	for {
		select {
		case <-stop:
			fmt.Println("STOP TRIGG")
			if c.state.Load() != status.Stopped {
				if err = c.stop(); err != nil {
					slog.With(slog.String("op", op)).Error(fmt.Sprintf("error stopping cluster: %s", err.Error()))
				}
			}
			return err
		case <-c.stopCh:
			fmt.Println("STOPCH TRIGG")
			if c.state.Load() != status.Stopped {
				if err = c.stop(); err != nil {
					slog.With(slog.String("op", op)).Error(fmt.Sprintf("error stopping cluster: %s", err.Error()))
				}
			}
			return err
		}
	}
}

func (c *Cluster) checkDeadline() {
	for {
		select {
		case <-c.ctx.Done():
			fmt.Println("DEADLINE EXPIRED")
			c.stop()
			return
		}
	}
}

func (c *Cluster) stop() error {
	var op = "cluster.stop"
	fmt.Println("CLUSTER STOP")
	if c.state.Load() != status.Running {
		return fmt.Errorf(fmt.Sprintf("cluster: %s already stopped", c.addr))
	}
	c.state.Store(status.Stopped)
	close(c.stopCh)
	c.ln.Close()
	c.wg.Wait()
	fmt.Println("WG DONE")
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("cluster: %s stopped", c.addr))
	return nil
}

func (c *Cluster) Receive(ctx *clusterContext.Context) error {
	var (
		op  = "cluster.Receive"
		err error
	)
	switch msg := ctx.Msg().(type) {
	case *message.JoinMessage:
		go func() {
			if err = c.handleJoin(msg, ctx); err != nil {
				slog.With(slog.String("op", op)).Error(fmt.Sprintf("handle join error: %s", err.Error()))
			}
		}()
	case *message.HeartbeatMessage:
		if err := c.handleHeartbeat(msg, ctx); err != nil {
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("handle heartbeat error: %s", err.Error()))
		}
	case *message.SetLeaderMessage:
		go func() {
			if err = c.handleSetLeader(msg, ctx); err != nil {
				slog.With(slog.String("op", op)).Error(fmt.Sprintf("handle setLeader error: %s", err.Error()))
			}
		}()
	}
	return err
}

func (c *Cluster) handleJoin(msg *message.JoinMessage, ctx *clusterContext.Context) error {
	var (
		op         = "cluster.handleJoin"
		err        error
		remoteId   = msg.Remote.GetId()
		remoteAddr = msg.Remote.GetAddr()
		members    = make([]*message.PID, 0)
		leaderNode *message.PID
	)
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("cluster received join: %s", remoteAddr))
	if _, ok := c.connPool[remoteAddr]; ok {
		utils.CloseConn(c, remoteAddr)
	}
	conn, err := net.Dial("tcp", remoteAddr)
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("member join error: %s", err.Error()))
		return err
	}
	if err = conn.SetDeadline(time.Now().Add(time.Minute * time.Duration(c.cfg.MemberJoinTimeout))); err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("failed to set deadline: %s", err.Error()))
		return err
	}
	for id, node := range c.nodes {
		if node.state.Load() == status.Running {
			members = append(members, &message.PID{Id: id, Addr: node.addr})
		}
	}
	node := &nodeInfo{addr: remoteAddr, heartbeatMisses: 0}
	node.state.Store(status.Running)
	c.nodes[remoteId] = node
	if c.leaderNode != nil {
		leaderNode = &message.PID{Addr: c.leaderNode.addr}
	}
	resp := &message.ClusterJoinMessage{Acknowledged: true, Members: members, LeaderNode: leaderNode}
	data, err := ser.Serialize(resp)
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("error send join response: %s", err.Error()))
		return err
	}
	msg = &message.JoinMessage{Remote: &message.PID{Addr: c.addr}, Data: data, TypeName: ser.TypeName(resp)}
	ctx = clusterContext.New(msg).WithParent(ctx).WithReceiver(&message.PID{Addr: remoteAddr})
	err = c.Engine.Send(ctx)
	utils.SaveConn(c, remoteAddr, conn)
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("error send join response: %s", err.Error()))
	}
	return err
}

func (c *Cluster) handleHeartbeat(msg *message.HeartbeatMessage, ctx *clusterContext.Context) error {
	var (
		op           = "cluster.handleHeartBeat"
		err          error
		remoteId     = msg.Remote.GetId()
		remoteAddr   = msg.Remote.GetAddr()
		acknowledged = false
	)
	c.nodesMutex.Lock()
	defer c.nodesMutex.Unlock()
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("cluster received heart beat from: %s", msg.GetRemote().GetAddr()))
	rawconn, err := net.Dial("tcp", remoteAddr)
	if err = rawconn.SetDeadline(time.Now().Add(time.Second * time.Duration(c.cfg.HeartBeatTimeout))); err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("failed to set deadline: %s", err.Error()))
	}
	if n, ok := c.nodes[remoteId]; ok {
		n.heartbeatMisses = 0
		acknowledged = true
	}
	resp := &message.HeartbeatMessage{Remote: &message.PID{Addr: c.addr}, Acknowledged: acknowledged}
	ctx = clusterContext.New(resp).WithParent(ctx).WithReceiver(&message.PID{Addr: remoteAddr})
	err = c.Engine.Send(ctx)
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("error send heart beat response: %s", err.Error()))
	}
	return err
}

func (c *Cluster) manageNodes() {
	c.wg.Add(1)
	defer c.wg.Done()
	ticker := time.NewTicker(time.Millisecond * time.Duration(c.cfg.MemberHeartbeatIntervalMs))
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.removeInactiveNodes()
		case <-c.stopCh:
			return
		}
	}
}

func (c *Cluster) removeInactiveNodes() {
	var op = "cluster.removeInactiveNodes"
	c.nodesMutex.Lock()
	defer c.nodesMutex.Unlock()
	for id, node := range c.nodes {
		if node.state.Load() != status.Running {
			continue
		}
		if node.heartbeatMisses > c.cfg.MaxMemberHeartbeatMisses {
			if c.leaderNode != nil && node.addr == c.leaderNode.addr {
				c.leaderNode = nil
				slog.With(slog.String("op", op)).Info(fmt.Sprintf("inactive leader node: %s, addr: %s", id, node.addr))
			}
			slog.With(slog.String("op", op)).Info(fmt.Sprintf("stop inactive node: %s, addr: %s", id, node.addr))
			node.state.Store(status.Stopped)
			_ = utils.CloseConn(c, node.addr)
		} else {
			node.heartbeatMisses++
		}
	}
}

func (c *Cluster) handleSetLeader(msg *message.SetLeaderMessage, ctx *clusterContext.Context) error {
	var (
		op  = "cluster.handleSetLeader"
		err error
	)
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("set new leader node: %s", msg.GetRemote().GetAddr()))
	c.leaderNode = &nodeInfo{addr: msg.GetRemote().GetAddr(), heartbeatMisses: 0}
	return err
}
