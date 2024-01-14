package blockchain_node

import (
	"github.com/MikhUd/blockchain/protos/blockchain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSerializer(t *testing.T) {
	msg := &blockchain.TestMessage{Data: []byte("test")}
	b, err := ProtoSerializer{}.Serialize(msg)
	require.NoError(t, err)
	dmsg, err := ProtoSerializer{}.Deserialize(b, ProtoSerializer{}.TypeName(msg))
	require.NoError(t, err)
	assert.Equal(t, msg.Data, dmsg.(*blockchain.TestMessage).Data)
}

func TestTransactionRequest(t *testing.T) {
	tr := &blockchain.TransactionRequest{
		SenderPublicKey:            "public_key",
		SenderPrivateKey:           "private_key",
		SenderBlockchainAddress:    "sender",
		RecipientBlockchainAddress: "recipient",
		Value:                      10.0,
	}
	b, err := ProtoSerializer{}.Serialize(tr)
	require.NoError(t, err)
	msg := &blockchain.Message{Data: b}
	b, err = ProtoSerializer{}.Serialize(msg)
	require.NoError(t, err)
	dmsg, err := ProtoSerializer{}.Deserialize(b, ProtoSerializer{}.TypeName(msg))
	require.NoError(t, err)
	assert.IsType(t, msg, dmsg)
	dTr, err := ProtoSerializer{}.Deserialize(dmsg.(*blockchain.Message).Data, ProtoSerializer{}.TypeName(tr))
	require.NoError(t, err)
	assert.IsType(t, tr, dTr)
}
