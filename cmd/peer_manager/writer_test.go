package peer_manager

import (
	"github.com/MikhUd/blockchain/internal/config"
	"github.com/MikhUd/blockchain/internal/utils"
	bcproto "github.com/MikhUd/blockchain/protos/blockchain"
	"github.com/stretchr/testify/assert"
	"log/slog"
	"testing"
)

func TestWriteToPeerManager(t *testing.T) {
	cfg := &config.Config{
		NodesCount: 5,
	}
	pm := New(cfg).WithLogger(slog.Default())
	err := pm.Start()
	assert.NoError(t, err)
	assert.Equal(t, pm.cfg.NodesCount, len(pm.nodes))
	w := NewWriter(pm.addr)
	sign, err := utils.GenerateRandomSignature()
	assert.NoError(t, err)
	pubKey, err := utils.GenerateRandomPublicKey()
	assert.NoError(t, err)
	tr := &bcproto.TransactionRequest{
		SenderPublicKey:            pubKey.String(),
		SenderBlockchainAddress:    "sender",
		RecipientBlockchainAddress: "recipient",
		Signature:                  sign.String(),
		Value:                      10.0,
	}
	ctx := Context{msg: tr}
	err = w.Send(&ctx)
	assert.NoError(t, err)
}
