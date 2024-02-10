package node

import (
	"github.com/MikhUd/blockchain/cmd/cluster"
	"github.com/MikhUd/blockchain/pkg/config"
	"github.com/MikhUd/blockchain/pkg/grpcapi/message"
	"github.com/MikhUd/blockchain/pkg/infrastructure/context"
	"github.com/MikhUd/blockchain/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAddTransactionToContextAndDecodeIt(t *testing.T) {
	privateKey, publicKey, err := utils.GenerateKeyPair()
	require.NoError(t, err)
	tr := &message.TransactionRequest{
		SenderPublicKey:            utils.PublicKeyStr(publicKey),
		SenderPrivateKey:           utils.PrivateKeyStr(privateKey),
		SenderBlockchainAddress:    "sender",
		RecipientBlockchainAddress: "recipient",
		Value:                      10.0,
	}
	b, err := cluster.ProtoSerializer{}.Serialize(tr)
	require.NoError(t, err)
	msg := &message.BlockchainMessage{Data: b}
	ctx := &context.Context{msg: msg, sender: &message.PID{Addr: utils.GetRandomListenAddr()}}
	switch m := ctx.Msg().(type) {
	case *message.BlockchainMessage:
		dmsg := ctx.Msg().(*message.BlockchainMessage)
		assert.IsType(t, msg, dmsg)
		_, err := cluster.ProtoSerializer{}.Deserialize(dmsg.Data, cluster.ProtoSerializer{}.TypeName(tr))
		require.NoError(t, err)
	default:
		t.Errorf("Unexpected message type: %T", m)
	}
}

func TestAddTransactionToBlockchain(t *testing.T) {
	cfg := config.Config{
		NodesCount: 1,
	}
	pm := cluster.New(cfg)
	err := pm.Start()
	assert.NoError(t, err)
	privateKey, publicKey, err := utils.GenerateKeyPair()
	require.NoError(t, err)
	tr := &message.TransactionRequest{
		SenderPublicKey:            utils.PublicKeyStr(publicKey),
		SenderPrivateKey:           utils.PrivateKeyStr(privateKey),
		SenderBlockchainAddress:    "sender",
		RecipientBlockchainAddress: "recipient",
		Value:                      10.0,
	}
	err = pm.Engine.Send(tr)
	assert.NoError(t, err)
	for _, n := range pm.GetNodes() {
		assert.Equal(t, 1, len(n.bc.TransactionPool()))
	}
}

func TestStreamWriter(t *testing.T) {
	cfg := config.Config{
		NodesCount: 1,
	}
	pm := cluster.New(cfg)
	err := pm.Start()
	assert.NoError(t, err)
	writer := cluster.NewWriter(pm.addr)
	for _, n := range pm.GetNodes() {
		err = writer.Send(&context.Context{receiver: n})
	}
	assert.NoError(t, err)
}
