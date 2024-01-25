package peer_manager

import (
	"github.com/MikhUd/blockchain/internal/config"
	"github.com/stretchr/testify/assert"
	"log/slog"
	"testing"
)

func TestNodesInitialization(t *testing.T) {
	cfg := &config.Config{
		NodesCount: 5,
	}
	pm := New(cfg).WithLogger(slog.Default())
	err := pm.Start()
	assert.NoError(t, err)
	assert.Equal(t, pm.cfg.NodesCount, len(pm.nodes))
}
