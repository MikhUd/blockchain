package blockchain

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/MikhUd/blockchain/pkg/config"
	"github.com/MikhUd/blockchain/pkg/domain/block"
	"github.com/MikhUd/blockchain/pkg/domain/signature"
	"github.com/MikhUd/blockchain/pkg/domain/transaction"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type Blockchain struct {
	transactionPool []*transaction.Transaction
	chain           []*block.Block
	addr            string
	config          config.Config
	mu              sync.RWMutex
	neighbors       []string
	muxNeighbors    sync.RWMutex
}

func (bc *Blockchain) Lock() {
	bc.mu.Lock()
}

func (bc *Blockchain) Unlock() {
	bc.mu.Unlock()
}

func (bc *Blockchain) Config() *config.Config {
	return &bc.config
}

func (bc *Blockchain) Addr() string {
	return bc.addr
}

func (bc *Blockchain) Neighbors() []string {
	return bc.neighbors
}

func (bc *Blockchain) CreateBlock(nonce int, previousHash [sha256.Size]byte) *block.Block {
	b := block.New(nonce, previousHash, bc.transactionPool)
	bc.chain = append(bc.chain, b)
	bc.transactionPool = []*transaction.Transaction{}
	return b
}

func (bc *Blockchain) UnmarshalJSON(data []byte) error {
	v := &struct {
		Blocks *[]*block.Block `json:"chain"`
	}{
		Blocks: &bc.chain,
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	return nil
}

func (bc *Blockchain) ResolveConflicts() bool {
	var longestChain []*block.Block = nil
	maxLength := len(bc.chain)
	for _, n := range bc.neighbors {
		endpoint := fmt.Sprintf("http://%s/chain", n)
		resp, _ := http.Get(endpoint)
		if resp.StatusCode == http.StatusOK {
			var bcResp Blockchain
			decoder := json.NewDecoder(resp.Body)
			_ = decoder.Decode(&bcResp)
			chain := bcResp.chain
			if len(chain) > maxLength && bc.ValidChain(chain) {
				maxLength = len(chain)
				longestChain = chain
			}
		}
	}
	if longestChain != nil {
		bc.chain = longestChain
		return true
	}
	return false
}

func (bc *Blockchain) LastBlock() *block.Block {
	return bc.chain[len(bc.chain)-1]
}

func New(cfg config.Config) *Blockchain {
	b := &block.Block{}
	blockChain := &Blockchain{
		config: cfg,
	}
	blockChain.CreateBlock(0, b.Hash())
	return blockChain
}

/*
	func (bc *Blockchain) Run() {
		bc.StartSyncNeighbors()
		bc.ResolveConflicts()
	}

	/*func (bc *Blockchain) SetNeighbors() {
		bc.neighbors = utils.FindNeighbors(utils.GetHost(), bc.port, 0, 1, 5000, 5003)
		log.Printf("Neighbors: %v", bc.neighbors)
	}

	(bc *Blockchain) SyncNeighbors() {
		bc.muxNeighbors.Lock()
		defer bc.muxNeighbors.Unlock()
		bc.SetNeighbors()
	}

	func (bc *Blockchain) StartSyncNeighbors() {
		bc.SyncNeighbors()
		_ = time.AfterFunc(time.Second*20, bc.StartSyncNeighbors)
	}
*/
func (bc *Blockchain) TransactionPool() []*transaction.Transaction {
	return bc.transactionPool
}

func (bc *Blockchain) ClearTransactionPool() {
	bc.transactionPool = bc.transactionPool[:0]
}

func (bc *Blockchain) WithBlockchainAddr(addr string) *Blockchain {
	bc.addr = addr
	return bc
}

func (bc *Blockchain) Print() {
	for i, b := range bc.chain {
		fmt.Printf("%s chain %d %s\n", strings.Repeat("=", 25), i, strings.Repeat("=", 25))
		b.Print()
	}
}

func (bc *Blockchain) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Blocks []*block.Block `json:"chain"`
	}{
		Blocks: bc.chain,
	})
}

func (bc *Blockchain) CreateTransaction(
	sender string,
	recipient string,
	value float32,
	senderPublicKey *ecdsa.PublicKey,
	signature *signature.Signature,
) bool {
	isTransacted := bc.AddTransaction(sender, recipient, value, senderPublicKey, signature)
	//TODO
	//sync
	return isTransacted
}

func (bc *Blockchain) AddTransaction(
	sender string,
	recipient string,
	value float32,
	senderPublicKey *ecdsa.PublicKey,
	signature *signature.Signature,
) bool {
	t := transaction.New(sender, recipient, value)
	/*
		if sender != bc.MiningSender && bc.CalculateTotalAmount(sender) < value {
			log.Println("ERROR: Not enough money")
			return false
		}
	*/
	if sender == bc.config.MiningSender || bc.verifyTransactionSignature(senderPublicKey, signature, t) {
		bc.transactionPool = append(bc.transactionPool, t)
		return true
	}
	return false
}

func (bc *Blockchain) verifyTransactionSignature(
	senderPublicKey *ecdsa.PublicKey,
	signature *signature.Signature,
	transaction *transaction.Transaction,
) bool {
	encodedTransaction, _ := json.Marshal(transaction)
	hash := sha256.Sum256(encodedTransaction)
	return ecdsa.Verify(senderPublicKey, hash[:], signature.R, signature.S)
}

func (bc *Blockchain) CopyTransactionPool() []*transaction.Transaction {
	transactions := make([]*transaction.Transaction, 0)
	for _, tr := range bc.transactionPool {
		transactions = append(transactions, transaction.New(
			tr.SenderBlockchainAddress,
			tr.RecipientBlockchainAddress,
			tr.Value,
		))
	}
	return transactions
}

func (bc *Blockchain) ValidProofBytesBuffer(previousHash [sha256.Size]byte, transactions []*transaction.Transaction, timestamp int64) []byte {
	var vpBuf []byte
	vpBuf = append(vpBuf, previousHash[:]...)
	vpBuf = append(vpBuf, []byte(strconv.Itoa(int(timestamp)))...)
	for _, t := range transactions {
		t.AppendBytesToBuffer(&vpBuf)
	}
	return vpBuf
}

func (bc *Blockchain) ValidProof(nonce int, zeros string, vpBuf *[]byte) bool {
	*vpBuf = strconv.AppendInt(*vpBuf, int64(nonce), 10)
	hash := sha256.Sum256(*vpBuf)
	guessHashStr := hex.EncodeToString(hash[:])
	return guessHashStr[:bc.config.MiningDifficulty] == zeros
}

func (bc *Blockchain) ValidChain(chain []*block.Block) bool {
	preBlock := chain[0]
	currentIndex := 1
	for currentIndex < len(chain) {
		b := chain[currentIndex]
		if b.PreviousHash != preBlock.Hash() {
			return false
		}
		vpBuf := bc.ValidProofBytesBuffer(b.PreviousHash, b.Transactions, preBlock.Timestamp)
		if !bc.ValidProof(b.Nonce, strings.Repeat("0", int(bc.config.MiningDifficulty)), &vpBuf) {
			return false
		}
		preBlock = b
		currentIndex += 1
	}
	return true
}

func (bc *Blockchain) CalculateTotalAmount(address string) float32 {
	var totalAmount float32 = 0.0
	for _, b := range bc.chain {
		for _, t := range b.Transactions {
			value := t.Value
			if address == t.RecipientBlockchainAddress {
				totalAmount += value
			}

			if address == t.SenderBlockchainAddress {
				totalAmount -= value
			}
		}
	}
	return totalAmount
}

type TransactionRequest struct {
	SenderBlockchainAddress    *string  `json:"sender_blockchain_address"`
	RecipientBlockchainAddress *string  `json:"recipient_blockchain_address"`
	SenderPublicKey            *string  `json:"sender_public_key"`
	Value                      *float32 `json:"value"`
	Signature                  *string  `json:"signature"`
}

type AmountResponse struct {
	Amount float32 `json:"amount"`
}

func (ar *AmountResponse) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Amount float32 `json:"amount"`
	}{
		Amount: ar.Amount,
	})
}

func (tr *TransactionRequest) Validate() bool {
	if tr.SenderBlockchainAddress == nil || tr.RecipientBlockchainAddress == nil || tr.SenderPublicKey == nil || tr.Value == nil || tr.Signature == nil {
		return false
	}
	return true
}
