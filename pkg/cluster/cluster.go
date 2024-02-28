package cluster

import (
	"context"
	"fmt"
	"github.com/MikhUd/blockchain/pkg/config"
	clusterContext "github.com/MikhUd/blockchain/pkg/context"
	"github.com/MikhUd/blockchain/pkg/grpcapi/message"
	"github.com/MikhUd/blockchain/pkg/grpcapi/remote"
	"github.com/MikhUd/blockchain/pkg/node"
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

const (
	memberJoinTimeout = time.Minute * 10
	heartBeatTimeout  = time.Second * 10
)

type Cluster struct {
	addr       string
	cfg        config.Config
	stopCh     chan struct{}
	reader     *stream.Reader
	wg         *sync.WaitGroup
	nodes      []nodeInfo
	leaderNode *node.LeaderNode
	state      atomic.Uint32
	Engine     *stream.Engine
	nodesMutex sync.RWMutex
}

type nodeInfo struct {
	id              uint32
	heartbeatMisses uint8
	address         string
}

func New(cfg config.Config, addr string) *Cluster {
	c := &Cluster{
		addr:  addr,
		wg:    &sync.WaitGroup{},
		cfg:   cfg,
		nodes: make([]nodeInfo, 0),
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
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("cluster received new peer: %s", ctx.Sender().Addr))
	if c.state.Load() != config.Running {
		go func() {
			if err = c.Start(); err != nil {
				slog.With(slog.String("op", op)).Error(fmt.Sprintf("got cluster receive error: %s", err.Error()))
				log.Fatal(err)
			}
		}()
	}

	switch msg := ctx.Msg().(type) {
	case *message.JoinRequest:
		go func() {
			err = c.handleJoin(msg, ctx)
		}()
	case *message.HeartbeatRequest:
		err = c.handleHeartbeat(msg, ctx)
	}

	return err
}

func (c *Cluster) handleJoin(msg *message.JoinRequest, ctx *clusterContext.Context) error {
	var err error
	const op = "cluster.handleJoin"
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("cluster received join: %s", msg.Address))
	rawconn, err := net.Dial("tcp", msg.Address)
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("member join error: %s", err.Error()))
	}
	if err = rawconn.SetDeadline(time.Now().Add(memberJoinTimeout)); err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("failed to set deadline: %s", err.Error()))
	}
	resp := &message.JoinResponse{Acknowledged: true, Address: c.Addr()}
	c.Engine.AddWriter(stream.NewWriter(msg.Address))
	ctx = clusterContext.New(resp).WithParent(ctx).WithReceiver(&message.PID{Addr: msg.Address})
	err = c.Engine.Send(ctx)
	if err != nil {
		slog.With(slog.String("op", op)).Error("error send join response: %s", err.Error())
	}
	return err
}

func (c *Cluster) handleHeartbeat(msg *message.HeartbeatRequest, ctx *clusterContext.Context) error {
	var err error
	const op = "cluster.handleHeartBeat"
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("cluster received heart beat from: %s", msg.Address))
	rawconn, err := net.Dial("tcp", msg.Address)
	if err = rawconn.SetDeadline(time.Now().Add(heartBeatTimeout)); err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("failed to set deadline: %s", err.Error()))
	}
	c.nodesMutex.Lock()
	defer c.nodesMutex.Unlock()
	c.nodes = append(c.nodes, nodeInfo{id: msg.MemberId, heartbeatMisses: 0, address: msg.Address})
	resp := &message.HeartbeatResponse{Acknowledged: true, Address: c.Addr()}
	c.Engine.AddWriter(stream.NewWriter(msg.Address))
	ctx = clusterContext.New(resp).WithParent(ctx).WithReceiver(&message.PID{Addr: msg.Address})
	err = c.Engine.Send(ctx)
	if err != nil {
		slog.With(slog.String("op", op)).Error("error send heart beat response: %s", err.Error())
	}
	return err
}
