package peer_manager

import (
	"github.com/MikhUd/blockchain/internal/config"
	"github.com/MikhUd/blockchain/internal/utils"
	bcproto "github.com/MikhUd/blockchain/protos/blockchain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
	"testing"
)

func TestAddTransactionToContextAndDecodeIt(t *testing.T) {
	sign, err := utils.GenerateRandomSignature()
	require.NoError(t, err)
	pk, err := utils.GenerateRandomPublicKey()
	require.NoError(t, err)
	tr := &bcproto.TransactionRequest{
		SenderPublicKey:            pk.String(),
		SenderBlockchainAddress:    "sender",
		RecipientBlockchainAddress: "recipient",
		Signature:                  sign.String(),
		Value:                      10.0,
	}
	b, err := ProtoSerializer{}.Serialize(tr)
	require.NoError(t, err)
	msg := &bcproto.BlockchainMessage{Data: b}
	ctx := &Context{msg: msg, sender: &bcproto.PID{Addr: utils.GetRandomListenAddr()}}
	switch m := ctx.Msg().(type) {
	case *bcproto.BlockchainMessage:
		dmsg := ctx.Msg().(*bcproto.BlockchainMessage)
		assert.IsType(t, msg, dmsg)
		_, err := ProtoSerializer{}.Deserialize(dmsg.Data, ProtoSerializer{}.TypeName(tr))
		require.NoError(t, err)
	default:
		t.Errorf("Unexpected message type: %T", m)
	}
}

func TestAddTransactionToBlockchain(t *testing.T) {
	cfg := &config.Config{
		NodesCount: 1,
	}
	pm := New(cfg).WithLogger(slog.Default())
	pm = pm.WithEngine(NewEngine(nil, NewWriter(pm.addr)))
	err := pm.Start()
	assert.NoError(t, err)
	tr := &bcproto.TransactionRequest{
		SenderPublicKey:            "public_key",
		SenderBlockchainAddress:    "sender",
		RecipientBlockchainAddress: "recipient",
		Signature:                  "signature",
		Value:                      10.0,
	}
	b, err := ProtoSerializer{}.Serialize(tr)
	msg := &bcproto.BlockchainMessage{Data: b}
	ctx := &Context{msg: msg, sender: &bcproto.PID{Addr: utils.GetRandomListenAddr()}}
	err = pm.engine.Send(ctx)
	assert.NoError(t, err)
	for _, n := range pm.GetNodes() {
		assert.Equal(t, 1, len(n.bc.TransactionPool()))
	}
}

func TestStreamWriter(t *testing.T) {
	cfg := &config.Config{
		NodesCount: 1,
	}
	pm := New(cfg).WithLogger(slog.Default())
	pm = pm.WithEngine(NewEngine(nil, NewWriter(pm.addr)))
	err := pm.Start()
	assert.NoError(t, err)
	writer := NewWriter(pm.addr)
	for _, n := range pm.GetNodes() {
		err = writer.Send(&Context{receiver: n})
	}
	assert.NoError(t, err)
}
