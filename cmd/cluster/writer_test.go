package cluster

import (
	"github.com/MikhUd/blockchain/pkg/config"
	"github.com/MikhUd/blockchain/pkg/infrastructure/context"
	"github.com/MikhUd/blockchain/pkg/utils"
	bcproto "github.com/MikhUd/blockchain/protos/blockchain"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestWriteToPeerManager(t *testing.T) {
	cfg := &config.Config{
		NodesCount: 5,
	}
	pm := New(cfg)
	err := pm.Start()
	assert.NoError(t, err)
	assert.Equal(t, pm.cfg.NodesCount, len(pm.nodes))
	w := NewWriter(pm.addr)
	privateKey, publicKey, err := utils.GenerateKeyPair()
	tr := &bcproto.TransactionRequest{
		SenderPublicKey:            utils.PublicKeyStr(publicKey),
		SenderPrivateKey:           utils.PrivateKeyStr(privateKey),
		SenderBlockchainAddress:    "sender",
		RecipientBlockchainAddress: "recipient",
		Value:                      10.0,
	}
	ctx := context.Context{msg: tr}
	err = w.Send(&ctx)
	assert.NoError(t, err)
}
