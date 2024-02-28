package stream

import (
	"github.com/MikhUd/blockchain/pkg/cluster"
	"github.com/MikhUd/blockchain/pkg/config"
	"github.com/MikhUd/blockchain/pkg/context"
	"github.com/MikhUd/blockchain/pkg/grpcapi/message"
	"github.com/MikhUd/blockchain/pkg/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestWriteToCluster(t *testing.T) {
	cfg := config.Config{
		NodesCount: 5,
	}
	c := cluster.New(cfg, utils.GetRandomListenAddr())
	err := c.Start()
	assert.NoError(t, err)
	assert.Equal(t, c.cfg.NodesCount, len(c.nodes))
	w := NewWriter(c.addr)
	privateKey, publicKey, err := utils.GenerateKeyPair()
	tr := &message.TransactionRequest{
		SenderPublicKey:            utils.PublicKeyStr(publicKey),
		SenderPrivateKey:           utils.PrivateKeyStr(privateKey),
		SenderBlockchainAddress:    "sender",
		RecipientBlockchainAddress: "recipient",
		Value:                      10.0,
	}
	err = w.Send(context.New(tr))
	assert.NoError(t, err)
}
