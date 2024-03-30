package cluster

import (
	"github.com/MikhUd/blockchain/pkg/utils"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestClusterInitialize(t *testing.T) {
	cfg := *utils.LoadConfig(utils.LocalPath)
	c := New(cfg, ":8080").WithTimeout(time.Second * 3)
	err := c.Start()
	assert.NoError(t, err)
}
