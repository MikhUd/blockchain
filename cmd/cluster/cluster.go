package cluster

import (
	"context"
	"fmt"
	"github.com/MikhUd/blockchain/cmd/node"
	"github.com/MikhUd/blockchain/pkg/config"
	"github.com/MikhUd/blockchain/pkg/domain/blockchain"
	"github.com/MikhUd/blockchain/pkg/domain/wallet"
	"github.com/MikhUd/blockchain/pkg/grpcapi/cluster"
	clusterContext "github.com/MikhUd/blockchain/pkg/infrastructure/context"
	"github.com/MikhUd/blockchain/pkg/utils"
	"log/slog"
	"net"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
	"sync"
	"sync/atomic"
)

type Cluster struct {
	addr   string
	cfg    config.Config
	stopCh chan struct{}
	reader *reader
	wg     *sync.WaitGroup
	nodes  map[string]*node.Node
	state  atomic.Uint32
	Engine *Engine
}

type Receiver interface {
	Receive(ctx *context.Context) error
}

func New(cfg config.Config) *Cluster {
	c := &Cluster{
		addr: utils.GetRandomListenAddr(),
		wg:   &sync.WaitGroup{},
		cfg:  cfg,
	}
	c.reader = newReader(c)
	c.state.Store(config.Initialized)
	c.Engine = &Engine{writer: NewWriter(c.Addr())}
	return c
}

func (c *Cluster) Addr() string {
	return c.addr
}

func (c *Cluster) Receive(ctx clusterContext.Context) error {
	const op = "cluster.Invoke"
	slog.With(slog.String("op", op)).Info("cluster received new peer")
	if c.state.Load() != config.Running {
		err := c.Start()
		if err != nil {
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("got cluster invoke error: %s", err.Error()))
			return err
		}
	}

	if ctx.Receiver() == nil {
		for _, n := range c.nodes {
			err := n.Receive(&ctx)
			if err != nil {
				slog.With(slog.String("op", op)).Error(fmt.Sprintf("got node receive error: %s", err.Error()))
				return err
			}
		}
		slog.With(slog.String("op", op)).Info(fmt.Sprintf("ctx processed on %d nodes", len(c.nodes)))
	} else {
		err := ctx.Receiver().Receive(&ctx)
		if err != nil {
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("got node receive error: %s", err.Error()))
			return err
		}
	}

	return nil
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
		return fmt.Errorf("failed to listen")
	}

	mux := drpcmux.New()
	server := drpcserver.New(mux)
	ctx, cancel := context.WithCancel(context.Background())
	err = cluster.DRPCRegisterPeerManager(mux, c.reader)

	go func() {
		defer c.wg.Done()
		err := server.Serve(ctx, listener)
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

	c.SpawnNodes(c.cfg.NodesCount)

	slog.Info(fmt.Sprintf("start cluster, addr:%s", c.addr))
	slog.Debug("listening", "addr", c.addr)
	return nil
}

func (c *Cluster) GetNode(addr string) *node.Node {
	return c.nodes[addr]
}

func (c *Cluster) GetNodes() map[string]*node.Node {
	return c.nodes
}

func (c *Cluster) AddNode(n *node.Node) error {
	const op = "cluster.AddNode"
	_, err := net.Dial("tcp", n.Addr())
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("node nconn init error: %s", err.Error()))
		return err
	}

	c.nodes[n.Addr()] = n

	return nil
}

func (c *Cluster) SpawnNodes(count int) bool {
	c.nodes = make(map[string]*node.Node)
	for i := 0; i < count; i++ {
		randAddr := utils.GetRandomListenAddr()
		minersWallet := wallet.New(c.cfg.Version)
		bc := blockchain.New(minersWallet.BlockchainAddr(), uint16(utils.GetPortFromAddr(randAddr)), c.cfg)
		n := node.NewNode(randAddr, bc, c.cfg)
		err := n.Start()
		if err != nil {
			slog.Error(err.Error())
		}
		c.nodes[n.Addr()] = n
	}

	return true
}
