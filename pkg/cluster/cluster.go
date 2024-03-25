package cluster

import (
	"context"
	"fmt"
	"github.com/MikhUd/blockchain/pkg/api/message"
	"github.com/MikhUd/blockchain/pkg/api/remote"
	"github.com/MikhUd/blockchain/pkg/config"
	clusterContext "github.com/MikhUd/blockchain/pkg/context"
	"github.com/MikhUd/blockchain/pkg/serializer"
	"github.com/MikhUd/blockchain/pkg/stream"
	"log"
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
	wg         *sync.WaitGroup
	leaderNode *nodeInfo
	nodes      map[string]*nodeInfo
	state      atomic.Uint32
	Engine     *stream.Engine
	nodesMutex sync.RWMutex
}

type nodeInfo struct {
	heartbeatMisses uint8
	addr            string
}

var ser serializer.ProtoSerializer

func New(cfg config.Config, addr string) *Cluster {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Cluster{
		addr:   addr,
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
		wg:     &sync.WaitGroup{},
		nodes:  make(map[string]*nodeInfo),
	}
	c.reader = stream.NewReader(c)
	c.state.Store(config.Initialized)
	c.Engine = stream.NewEngine(nil)
	return c
}

func (c *Cluster) Addr() string {
	return c.addr
}

func (c *Cluster) Start() error {
	const op = "cluster.Start"
	if c.state.Load() == config.Running {
		slog.With(slog.String("op", op)).Error("cluster already running")
		return fmt.Errorf("cluster already running")
	}
	c.state.Store(config.Running)
	c.wg.Add(1)
	listener, err := net.Listen("tcp", c.addr)
	if err != nil {
		fmt.Println(err)
		return fmt.Errorf("failed to listen")
	}

	mux := drpcmux.New()
	server := drpcserver.New(mux)
	ctx, cancel := context.WithCancel(context.Background())
	err = remote.DRPCRegisterRemote(mux, c.reader)

	go func() {
		defer c.wg.Done()
		err = server.Serve(ctx, listener)
		if err != nil {
			slog.Error("server", "err", err)
		} else {
			slog.Debug("server stopped")
		}
	}()
	go func() {
		<-c.stopCh
		cancel()
	}()

	slog.Info(fmt.Sprintf("start cluster, addr:%s", c.addr))
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	return nil
}

func (c *Cluster) Receive(ctx *clusterContext.Context) error {
	var err error
	const op = "cluster.Receive"
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("cluster received new peer: %s", ctx.Sender().GetAddr()))
	if c.state.Load() != config.Running {
		go func() {
			if err = c.Start(); err != nil {
				slog.With(slog.String("op", op)).Error(fmt.Sprintf("got cluster receive error: %s", err.Error()))
				log.Fatal(err)
			}
		}()
	}

	switch msg := ctx.Msg().(type) {
	case *message.JoinMessage:
		go func() {
			err = c.handleJoin(msg, ctx)
		}()
	case *message.HeartbeatMessage:
		err = c.handleHeartbeat(msg, ctx)
	}

	return err
}

func (c *Cluster) handleJoin(msg *message.JoinMessage, ctx *clusterContext.Context) error {
	var (
		err        error
		remoteId   = msg.Remote.GetId()
		remoteAddr = msg.Remote.GetAddr()
		neighbors  = make([]*message.PID, 0)
	)
	const op = "cluster.handleJoin"
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("cluster received join: %s", remoteAddr))
	rawconn, err := net.Dial("tcp", remoteAddr)
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("member join error: %s", err.Error()))
		return err
	}
	if err = rawconn.SetDeadline(time.Now().Add(time.Minute * time.Duration(c.cfg.MemberJoinTimeout))); err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("failed to set deadline: %s", err.Error()))
		return err
	}
	//TODO: send join event for all members in cluster, wait confirmation, then accept new node
	for id, n := range c.nodes {
		neighbors = append(neighbors, &message.PID{Id: id, Addr: n.addr})
	}
	c.nodes[remoteId] = &nodeInfo{heartbeatMisses: 0, addr: remoteAddr}
	joinResp := &message.ClusterJoinMessage{Acknowledged: true, Neighbors: neighbors}
	data, err := ser.Serialize(joinResp)
	if err != nil {
		slog.With(slog.String("op", op)).Error("error send join response: %s", err.Error())
		return err
	}
	resp := &message.JoinMessage{Remote: &message.PID{Addr: c.Addr()}, Data: data, TypeName: ser.TypeName(joinResp)}
	c.Engine.AddWriter(stream.NewWriter(remoteAddr))
	ctx = clusterContext.New(resp).WithParent(ctx).WithReceiver(&message.PID{Addr: remoteAddr})
	err = c.Engine.Send(ctx)
	if err != nil {
		slog.With(slog.String("op", op)).Error("error send join response: %s", err.Error())
	}
	return err
}

func (c *Cluster) handleHeartbeat(msg *message.HeartbeatMessage, ctx *clusterContext.Context) error {
	var (
		err          error
		remoteId     = msg.Remote.GetId()
		remoteAddr   = msg.Remote.GetAddr()
		acknowledged = false
	)
	const op = "cluster.handleHeartBeat"
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
	resp := &message.HeartbeatMessage{Remote: &message.PID{Addr: c.Addr()}, Acknowledged: acknowledged}
	c.Engine.AddWriter(stream.NewWriter(remoteAddr))
	ctx = clusterContext.New(resp).WithParent(ctx).WithReceiver(&message.PID{Addr: remoteAddr})
	err = c.Engine.Send(ctx)
	if err != nil {
		slog.With(slog.String("op", op)).Error("error send heart beat response: %s", err.Error())
	}
	return err
}

func (c *Cluster) manageNodes() {
	c.wg.Add(1)
	defer c.wg.Done()
	ticker := time.NewTicker(time.Second * time.Duration(c.cfg.NodeHeartBeatInterval))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.removeInactiveNodes()
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *Cluster) removeInactiveNodes() {
	const op = "cluster.removeInactiveNodes"
	c.nodesMutex.Lock()
	defer c.nodesMutex.Unlock()
	for id, node := range c.nodes {
		if node.heartbeatMisses > uint8(c.cfg.MaxNodeHeartBeatMisses) {
			if node.addr == c.leaderNode.addr {
				slog.With(slog.String("op", op)).Info(fmt.Sprintf("leader node is dead, stop cluster until election, node: %s, addr: %s", id, node.addr))
				c.state.Store(config.Stopped)
			}
			slog.With(slog.String("op", op)).Info(fmt.Sprintf("remove inactive node: %s, addr: %s", id, node.addr))
		}
	}
}
