package cluster

import (
	"github.com/MikhUd/blockchain/pkg/config"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNodesInitialization(t *testing.T) {
	cfg := config.Config{
		NodesCount: 5,
	}
	pm := New(cfg)
	err := pm.Start()
	assert.NoError(t, err)
	assert.Equal(t, pm.cfg.NodesCount, len(pm.nodes))
}
