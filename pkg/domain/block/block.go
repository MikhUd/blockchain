package block

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/MikhUd/blockchain/pkg/domain/transaction"
	"time"
)

type Block struct {
	Timestamp    int64
	Nonce        int
	PreviousHash [sha256.Size]byte
	Transactions []*transaction.Transaction
}

func New(nonce int, previousHash [sha256.Size]byte, transactions []*transaction.Transaction) *Block {
	b := &Block{
		Nonce:        nonce,
		PreviousHash: previousHash,
		Timestamp:    time.Now().UnixNano(),
		Transactions: transactions,
	}
	return b
}

func (b *Block) Print() {
	fmt.Printf("timestamp     %-20d\n", b.Timestamp)
	fmt.Printf("nonce         %-20d\n", b.Nonce)
	fmt.Printf("previous_hash %-20x\n", b.PreviousHash)
	for _, t := range b.Transactions {
		t.Print()
	}
}

func (b *Block) Hash() [sha256.Size]byte {
	encodedBlock, _ := json.Marshal(b)
	return sha256.Sum256(encodedBlock)
}

func (b *Block) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Timestamp    int64                      `json:"timestamp"`
		Nonce        int                        `json:"nonce"`
		PreviousHash string                     `json:"previous_hash"`
		Transactions []*transaction.Transaction `json:"transactions"`
	}{
		Timestamp:    b.Timestamp,
		Nonce:        b.Nonce,
		PreviousHash: fmt.Sprintf("%x", b.PreviousHash),
		Transactions: b.Transactions,
	})
}

func (b *Block) UnmarshalJSON(data []byte) error {
	var previousHash string
	v := &struct {
		Timestamp    *int64                      `json:"timestamp"`
		Nonce        *int                        `json:"nonce"`
		PreviousHash *string                     `json:"previous_hash"`
		Transactions *[]*transaction.Transaction `json:"transactions"`
	}{
		Timestamp:    &b.Timestamp,
		Nonce:        &b.Nonce,
		PreviousHash: &previousHash,
		Transactions: &b.Transactions,
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	ph, _ := hex.DecodeString(*v.PreviousHash)
	copy(b.PreviousHash[:], ph[:sha256.Size])
	return nil
}
