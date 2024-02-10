package wallet_transaction

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"github.com/MikhUd/blockchain/pkg/domain/signature"
)

type Transaction struct {
	senderPrivateKey           *ecdsa.PrivateKey
	senderPublicKey            *ecdsa.PublicKey
	senderBlockchainAddress    string
	recipientBlockchainAddress string
	value                      float32
}

func New(
	privateKey *ecdsa.PrivateKey,
	publicKey *ecdsa.PublicKey,
	sender string,
	recipient string,
	value float32,
) *Transaction {
	return &Transaction{
		senderPrivateKey:           privateKey,
		senderPublicKey:            publicKey,
		senderBlockchainAddress:    sender,
		recipientBlockchainAddress: recipient,
		value:                      value,
	}
}

func (t *Transaction) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Sender    string  `json:"sender_blockchain_address"`
		Recipient string  `json:"recipient_blockchain_address"`
		Value     float32 `json:"value"`
	}{
		Sender:    t.senderBlockchainAddress,
		Recipient: t.recipientBlockchainAddress,
		Value:     t.value,
	})
}

func (t *Transaction) GenerateSignature() *signature.Signature {
	encodedWalletTransaction, _ := json.Marshal(t)
	hash := sha256.Sum256(encodedWalletTransaction)
	r, s, _ := ecdsa.Sign(rand.Reader, t.senderPrivateKey, hash[:])
	return &signature.Signature{
		R: r,
		S: s,
	}
}
