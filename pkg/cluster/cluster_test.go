package cluster

import (
	"github.com/MikhUd/blockchain/pkg/domain/blockchain"
	"github.com/MikhUd/blockchain/pkg/node"
	"github.com/MikhUd/blockchain/pkg/utils"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestClusterInitialize(t *testing.T) {
	cfg := *utils.LoadConfig(utils.LocalPath)
	c := New(cfg, ":8080").WithTimeout(time.Millisecond * 200)
	err := c.Start()
	assert.NoError(t, err)
}

func TestClusterAppendNodes(t *testing.T) {
	var (
		err         error
		clusterAddr = ":8000"
		nodeAddr    = ":13000"
	)
	cfg := *utils.LoadConfig(utils.LocalPath)
	cfg.NodeHeartbeatIntervalMs = 100
	c := New(cfg, clusterAddr).WithTimeout(time.Second * 1)
	go func() {
		err = c.Start()
		assert.NoError(t, err)
		<-c.stopCh
		return
	}()
	bc := blockchain.New(cfg)
	n, _ := node.New(nodeAddr, bc, cfg).WithTimeout(time.Second * 1)
	err = n.Start(clusterAddr)
	lenNodes := len(c.nodes)
	assert.Equal(t, 1, lenNodes)
}
