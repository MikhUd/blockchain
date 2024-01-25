package peer_manager

import (
	"github.com/MikhUd/blockchain/internal/utils"
	bcproto "github.com/MikhUd/blockchain/protos/blockchain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSerializer(t *testing.T) {
	msg := &bcproto.TestMessage{Data: []byte("test")}
	b, err := ProtoSerializer{}.Serialize(msg)
	require.NoError(t, err)
	dmsg, err := ProtoSerializer{}.Deserialize(b, ProtoSerializer{}.TypeName(msg))
	require.NoError(t, err)
	assert.Equal(t, msg.Data, dmsg.(*bcproto.TestMessage).Data)
}

func TestTransactionRequest(t *testing.T) {
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
	b, err = ProtoSerializer{}.Serialize(msg)
	require.NoError(t, err)
	dmsg, err := ProtoSerializer{}.Deserialize(b, ProtoSerializer{}.TypeName(msg))
	require.NoError(t, err)
	assert.IsType(t, msg, dmsg)
	dTr, err := ProtoSerializer{}.Deserialize(dmsg.(*bcproto.BlockchainMessage).Data, ProtoSerializer{}.TypeName(tr))
	require.NoError(t, err)
	assert.IsType(t, tr, dTr)
}
