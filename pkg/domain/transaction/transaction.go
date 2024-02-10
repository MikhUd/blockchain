package transaction

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

type Transaction struct {
	SenderBlockchainAddress    string
	RecipientBlockchainAddress string
	Value                      float32
}

func New(sender string, recipient string, value float32) *Transaction {
	return &Transaction{
		SenderBlockchainAddress:    sender,
		RecipientBlockchainAddress: recipient,
		Value:                      value,
	}
}

func (t *Transaction) Print() {
	fmt.Printf("%s\n", strings.Repeat("-", 40))
	fmt.Printf("%s\n", t.SenderBlockchainAddress)
	fmt.Printf("%s\n", t.RecipientBlockchainAddress)
	fmt.Printf("%.1f\n", t.Value)
}

func (t *Transaction) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		SenderBlockchainAddress    string  `json:"sender_blockchain_address"`
		RecipientBlockchainAddress string  `json:"recipient_blockchain_address"`
		Value                      float32 `json:"value"`
	}{
		SenderBlockchainAddress:    t.SenderBlockchainAddress,
		RecipientBlockchainAddress: t.RecipientBlockchainAddress,
		Value:                      t.Value,
	})
}

func (t *Transaction) UnmarshalJSON(data []byte) error {
	v := &struct {
		SenderBlockchainAddress    *string  `json:"sender_blockchain_address"`
		RecipientBlockchainAddress *string  `json:"recipient_blockchain_address"`
		Value                      *float32 `json:"value"`
	}{
		SenderBlockchainAddress:    &t.SenderBlockchainAddress,
		RecipientBlockchainAddress: &t.RecipientBlockchainAddress,
		Value:                      &t.Value,
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	return nil
}

func (t *Transaction) AppendBytesToBuffer(buf *[]byte) {
	*buf = append(*buf, []byte(t.SenderBlockchainAddress)...)
	*buf = append(*buf, []byte(t.RecipientBlockchainAddress)...)
	valueBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(valueBytes, math.Float32bits(t.Value))
	*buf = append(*buf, valueBytes...)
}
