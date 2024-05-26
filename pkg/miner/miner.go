package miner

import (
	"github.com/MikhUd/blockchain/pkg/domain/blockchain"
	"log"
	"strconv"
	"strings"
)

type Miner struct {
	bc *blockchain.Blockchain
}

func New(bc *blockchain.Blockchain) Miner {
	return Miner{
		bc: bc,
	}
}

func (m *Miner) Mining() bool {
	log.Println("action=mining")
	m.bc.Lock()
	defer m.bc.Unlock()

	if len(m.bc.TransactionPool()) == 0 {
		return false
	}
	m.bc.AddTransaction(m.bc.Config().MiningSender, m.bc.Addr(), m.bc.Config().MiningReward, nil, nil)
	nonce := m.ProofOfWork()
	log.Printf("NONCE: %d\n", nonce)
	previousHash := m.bc.LastBlock().Hash()
	m.bc.CreateBlock(nonce, previousHash)

	log.Println("action=finish_mining")
	return true
}

// ProofOfWork простой алгоритм майнинга вычисляющий хеш блока на основе случайного числа
func (m *Miner) ProofOfWork() int {
	transactions := m.bc.CopyTransactionPool()
	previousHash := m.bc.LastBlock().Hash()
	timestamp := m.bc.LastBlock().Timestamp

	zeros := strings.Repeat("0", int(m.bc.Config().MiningDifficulty))
	nonce := 0
	vpBuf := m.bc.ValidProofBytesBuffer(previousHash, transactions, timestamp)

	for !m.bc.ValidProof(nonce, zeros, &vpBuf) {
		vpBuf = vpBuf[:len(vpBuf)-len(strconv.Itoa(nonce))]
		nonce += 1
	}
	return nonce
}
