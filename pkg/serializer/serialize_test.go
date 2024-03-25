package serializer

import (
	"github.com/MikhUd/blockchain/pkg/api/message"
	"github.com/MikhUd/blockchain/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSerializer(t *testing.T) {
	msg := &message.TestMessage{Data: []byte("test")}
	b, err := ProtoSerializer{}.Serialize(msg)
	require.NoError(t, err)
	dmsg, err := ProtoSerializer{}.Deserialize(b, ProtoSerializer{}.TypeName(msg))
	require.NoError(t, err)
	assert.Equal(t, msg.Data, dmsg.(*message.TestMessage).Data)
}

func TestTransactionRequest(t *testing.T) {
	privateKey, publicKey, err := utils.GenerateKeyPair()
	require.NoError(t, err)
	tr := &message.TransactionRequest{
		SenderPublicKey:            utils.PublicKeyStr(publicKey),
		SenderPrivateKey:           utils.PrivateKeyStr(privateKey),
		SenderBlockchainAddress:    "sender",
		RecipientBlockchainAddress: "recipient",
		Value:                      10.0,
	}
	b, err := ProtoSerializer{}.Serialize(tr)
	require.NoError(t, err)
	msg := &message.BlockchainMessage{Data: b}
	b, err = ProtoSerializer{}.Serialize(msg)
	require.NoError(t, err)
	dmsg, err := ProtoSerializer{}.Deserialize(b, ProtoSerializer{}.TypeName(msg))
	require.NoError(t, err)
	assert.IsType(t, msg, dmsg)
	dTr, err := ProtoSerializer{}.Deserialize(dmsg.(*message.BlockchainMessage).Data, ProtoSerializer{}.TypeName(tr))
	require.NoError(t, err)
	assert.IsType(t, tr, dTr)
}

func TestNodeJoinMessage(t *testing.T) {
	joinMsg := &message.NodeJoinMessage{Acknowledged: true}
	serialized, err := ProtoSerializer{}.Serialize(joinMsg)
	require.NoError(t, err)
	dmsg, err := ProtoSerializer{}.Deserialize(serialized, ProtoSerializer{}.TypeName(joinMsg))
	require.NoError(t, err)
	assert.IsType(t, joinMsg, dmsg)
}
