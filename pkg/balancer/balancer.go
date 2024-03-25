package balancer

import (
	"context"
	"github.com/MikhUd/blockchain/pkg/config"
	"sync/atomic"
)

type Balancer struct {
	addr     string
	cfg      config.Config
	ctx      context.Context
	cancel   context.CancelFunc
	clusters []clusterInfo
}

type clusterInfo struct {
	id              uint32
	heartbeatMisses uint8
	addr            string
	state           atomic.Uint32
}

func New(cfg config.Config, addr string) *Balancer {
	ctx, cancel := context.WithCancel(context.Background())
	b := &Balancer{
		addr:     addr,
		cfg:      cfg,
		ctx:      ctx,
		cancel:   cancel,
		clusters: make([]clusterInfo, 0),
	}
	return b
}
