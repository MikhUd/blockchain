package peer_manager

import (
	"context"
	"fmt"
	"github.com/MikhUd/blockchain/cmd/blockchain_node/node"
	"github.com/MikhUd/blockchain/cmd/blockchain_node/stream_reader"
	"github.com/MikhUd/blockchain/internal/config"
	nodectx "github.com/MikhUd/blockchain/internal/domain/context"
	"github.com/MikhUd/blockchain/internal/utils"
	"github.com/MikhUd/blockchain/protos/blockchain"
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
	reader *stream_reader.Reader
	wg     *sync.WaitGroup
	state  atomic.Uint32
	logger *slog.Logger
	nodes  map[string]*node.Node
}

const (
	stateInitialized = 1
	stateRunning     = 2
	stateStopped     = 3
)

func New(cfg *config.Config) *PeerManager {
	pm := &PeerManager{
		addr: utils.GetRandomListenAddr(),
		cfg:  cfg,
	}
	pm.state.Store(stateInitialized)
	pm.reader = stream_reader.New(pm)
	return pm
}

func (pm *PeerManager) WithLogger(logger *slog.Logger) *PeerManager {
	pm.logger = logger
	return pm
}

func (pm *PeerManager) Start() error {
	pm.wg.Add(1)
	if pm.state.Load() != stateInitialized {
		return fmt.Errorf("node already started")
	}
	pm.state.Store(stateRunning)

	listener, err := net.Listen("tcp", pm.addr)
	if err != nil {
		return fmt.Errorf("failed to listen")
	}

	mux := drpcmux.New()
	server := drpcserver.New(mux)
	ctx, cancel := context.WithCancel(context.Background())
	err = blockchain.DRPCRegisterBlockchainNode(mux, pm.reader)

	go func() {
		defer pm.wg.Done()
		err := server.Serve(ctx, listener)
		if err != nil {
			pm.logger.Error("server", "err", err)
		} else {
			pm.logger.Debug("server stopped")
		}
	}()
	go func() {
		<-pm.stopCh
		cancel()
	}()

	fmt.Printf("start peer manager, addr:%s", pm.addr)
	pm.logger.Debug("listening", "addr", pm.addr)
	return nil
}

func (pm *PeerManager) Stop() error {
	if pm.state.Load() != stateRunning {
		return fmt.Errorf("peer manager already stopped")
	}
	pm.state.Store(stateStopped)
	return nil
}

func (pm *PeerManager) Receive(ctx *nodectx.NodeContext) {
	switch ctx.Msg().(type) {

	}
	//for _, node := range pm.Nodes() {
	//node.
	//}
}

func (pm *PeerManager) GetNode(addr string) *node.Node {
	return pm.nodes[addr]
}

func (pm *PeerManager) Nodes() map[string]*node.Node {
	return pm.nodes
}
