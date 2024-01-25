package peer_manager

import (
	"context"
	"fmt"
	"github.com/MikhUd/blockchain/internal/config"
	"github.com/MikhUd/blockchain/internal/domain/blockchain"
	"github.com/MikhUd/blockchain/internal/utils"
	bcproto "github.com/MikhUd/blockchain/protos/blockchain"
	"log/slog"
	"net"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
	"sync"
	"sync/atomic"
)

type PeerManager struct {
	addr   string
	cfg    *config.Config
	stopCh chan struct{}
	reader *reader
	wg     *sync.WaitGroup
	Logger *slog.Logger
	nodes  map[string]*Node
	state  atomic.Uint32
	engine *Engine
}

func New(cfg *config.Config) *PeerManager {
	pm := &PeerManager{
		addr: utils.GetRandomListenAddr(),
		wg:   &sync.WaitGroup{},
		cfg:  cfg,
	}
	pm.reader = newReader(pm)
	pm.state.Store(config.Initialized)
	return pm
}

func (pm *PeerManager) Addr() string {
	return pm.addr
}

func (pm *PeerManager) WithLogger(logger *slog.Logger) *PeerManager {
	pm.Logger = logger
	return pm
}

func (pm *PeerManager) WithEngine(engine *Engine) *PeerManager {
	pm.engine = engine
	return pm
}

func (pm *PeerManager) Receive(ctx Context) error {
	const op = "peer_manager.Invoke"
	pm.Logger.With(slog.String("op", op)).Info("peer manager received new peer")
	if pm.state.Load() != config.Running {
		err := pm.Start()
		if err != nil {
			pm.Logger.With(slog.String("op", op)).Error(fmt.Sprintf("got invoke error: %s", err.Error()))
			return err
		}
	}

	if ctx.Receiver() == nil {
		for _, n := range pm.nodes {
			err := n.Receive(&ctx)
			if err != nil {
				pm.Logger.With(slog.String("op", op)).Error(fmt.Sprintf("got node receive error: %s", err.Error()))
				return err
			}
		}
		pm.Logger.With(slog.String("op", op)).Info(fmt.Sprintf("ctx processed on %d nodes", len(pm.nodes)))
	}

	return nil
}

func (pm *PeerManager) Start() error {
	if pm.state.Load() == config.Running {
		return fmt.Errorf("peer manager already running")
	}
	pm.state.Store(config.Running)
	pm.wg.Add(1)
	listener, err := net.Listen("tcp", pm.addr)
	if err != nil {
		return fmt.Errorf("failed to listen")
	}

	mux := drpcmux.New()
	server := drpcserver.New(mux)
	ctx, cancel := context.WithCancel(context.Background())
	err = bcproto.DRPCRegisterPeerManager(mux, pm.reader)

	go func() {
		defer pm.wg.Done()
		err := server.Serve(ctx, listener)
		if err != nil {
			pm.Logger.Error("server", "err", err)
		} else {
			pm.Logger.Debug("server stopped")
		}
	}()
	go func() {
		<-pm.stopCh
		cancel()
	}()

	bc := pm.GetBlockchain()
	pm.SpawnNodes(pm.cfg.NodesCount, bc)

	fmt.Printf("start peer manager, addr:%s", pm.addr)
	pm.Logger.Debug("listening", "addr", pm.addr)
	return nil
}

func (pm *PeerManager) GetNode(addr string) *Node {
	return pm.nodes[addr]
}

func (pm *PeerManager) GetNodes() map[string]*Node {
	return pm.nodes
}

func (pm *PeerManager) GetBlockchain() *blockchain.Blockchain {
	return &blockchain.Blockchain{}
}

func (pm *PeerManager) SpawnNodes(count int, bc *blockchain.Blockchain) bool {
	pm.nodes = make(map[string]*Node)
	for i := 0; i < count; i++ {
		n := NewNode(*pm.Logger, utils.GetRandomListenAddr(), bc)
		err := n.Start()
		if err != nil {
			pm.Logger.Error(err.Error())
		}
		pm.nodes[n.Addr()] = n
	}

	return true
}
