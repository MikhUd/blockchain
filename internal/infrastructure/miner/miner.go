package miner

import (
	"fmt"
	"github.com/MikhUd/blockchain/internal/domain/blockchain"
	"log"
	"net/http"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"
)

type Miner struct {
	blockchain *blockchain.Blockchain
}

func New(bc *blockchain.Blockchain) Miner {
	return Miner{
		blockchain: bc,
	}
}

func (m *Miner) Mining() bool {
	log.Println("action=mining")
	m.blockchain.Lock()
	defer m.blockchain.Unlock()

	memProfile, err := os.Create("mem_profile.prof")
	if err != nil {
		log.Fatal("Could not create memory profile: ", err)
	}
	defer func() {
		pprof.WriteHeapProfile(memProfile)
		memProfile.Close()
	}()
	// Начинаем профилирование
	f, err := os.Create("cpu_profile.prof")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatal(err)
	}
	defer pprof.StopCPUProfile()

	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	start := time.Now()

	if len(m.blockchain.TransactionPool()) == 0 {
		return false
	}
	m.blockchain.AddTransaction(m.blockchain.Config().MiningSender, m.blockchain.Addr(), m.blockchain.Config().MiningReward, nil, nil)
	nonce := m.ProofOfWork()
	log.Printf("NONCE: %d\n", nonce)
	previousHash := m.blockchain.LastBlock().Hash()
	m.blockchain.CreateBlock(nonce, previousHash)

	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	elapsed := time.Since(start)

	// Рассчитываем разницу в потреблении памяти
	memoryUsage := m2.TotalAlloc - m1.TotalAlloc

	// Выводим информацию о затраченном времени и памяти
	log.Println("action=finish_mining")
	log.Println("Elapsed Time:", elapsed)
	log.Println("Memory Usage:", memoryUsage, "bytes")

	log.Println("action=finish_mining")

	for _, n := range m.blockchain.Neighbors() {
		endpoint := fmt.Sprintf("http://%s/consensus", n)
		client := &http.Client{}
		req, _ := http.NewRequest("PUT", endpoint, nil)
		_, _ = client.Do(req)
	}

	return true
}

// ProofOfWork простой алгоритм майнинга вычисляющий хеш блока на основе случайного числа
func (m *Miner) ProofOfWork() int {
	transactions := m.blockchain.CopyTransactionPool()
	previousHash := m.blockchain.LastBlock().Hash()
	timestamp := m.blockchain.LastBlock().Timestamp

	zeros := strings.Repeat("0", int(m.blockchain.Config().MiningDifficulty))
	nonce := 0
	vpBuf := m.blockchain.ValidProofBytesBuffer(previousHash, transactions, timestamp)

	for !m.blockchain.ValidProof(nonce, zeros, &vpBuf) {
		vpBuf = vpBuf[:len(vpBuf)-len(strconv.Itoa(nonce))]
		nonce += 1
	}
	return nonce
}
