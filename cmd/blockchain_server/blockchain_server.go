package blockchain_server

import (
	"encoding/json"
	"fmt"
	"github.com/MikhUd/blockchain/internal/config"
	"github.com/MikhUd/blockchain/internal/domain/blockchain"
	"github.com/MikhUd/blockchain/internal/domain/transaction"
	"github.com/MikhUd/blockchain/internal/domain/wallet"
	"github.com/MikhUd/blockchain/internal/infrastructure/miner"
	"github.com/MikhUd/blockchain/internal/utils"
	"io"
	"log"
	"net/http"
	"strconv"
)

var cache = make(map[uint16]map[string]*blockchain.Blockchain)

type BlockchainServer struct {
	port   uint16
	config config.Config
}

func New(port uint16, cfg config.Config) *BlockchainServer {
	return &BlockchainServer{port: port, config: cfg}
}

func (bcs *BlockchainServer) Port() uint16 {
	return bcs.port
}

func (bcs *BlockchainServer) GetBlockchain(cfg config.Config) *blockchain.Blockchain {
	bc, ok := cache[bcs.port]["blockchain"]
	if cache[bcs.port] == nil {
		cache[bcs.port] = make(map[string]*blockchain.Blockchain)
	}
	if !ok {
		minersWallet := wallet.New(bcs.config.Version)
		bc = blockchain.New(minersWallet.BlockchainAddr(), bcs.Port(), cfg)
		cache[bcs.port]["blockchain"] = bc
	}
	return bc
}

func (bcs *BlockchainServer) GetChain(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Add("Content-Type", "application/json")
		bc := bcs.GetBlockchain(bcs.config)
		encoded, _ := json.Marshal(bc)
		_, err := io.WriteString(w, string(encoded))
		if err != nil {
			fmt.Printf("ERROR:%s", err.Error())
		}
	default:
		log.Println("Error: Invalid http method")
	}
}

func (bcs *BlockchainServer) Transactions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Add("Content-Type", "application/json")
		bc := bcs.GetBlockchain(bcs.config)
		transactions := bc.TransactionPool()
		encoded, _ := json.Marshal(struct {
			Transactions []*transaction.Transaction `json:"transactions"`
			Length       int                        `json:"length"`
		}{
			Transactions: transactions,
			Length:       len(transactions),
		})
		io.WriteString(w, string(encoded))
	case http.MethodPost:
		decoder := json.NewDecoder(r.Body)
		var t blockchain.TransactionRequest
		err := decoder.Decode(&t)
		if err != nil {
			log.Printf("Error: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !t.Validate() {
			log.Printf("Error: missing field(s)")
			w.WriteHeader(http.StatusBadRequest)
		}
		publicKey := utils.PublicKeyFromString(*t.SenderPublicKey)
		signature := utils.SignatureFromString(*t.Signature)
		bc := bcs.GetBlockchain(bcs.config)
		isCreated := bc.CreateTransaction(*t.SenderBlockchainAddress, *t.RecipientBlockchainAddress, *t.Value, publicKey, signature)
		w.Header().Add("Content-Type", "application/json")
		if !isCreated {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
	default:
		log.Println("ERROR: Invalid http method")
	}
}

func (bcs *BlockchainServer) Mine(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		bc := bcs.GetBlockchain(bcs.config)
		m := miner.New(bc)
		isMined := m.Mining()
		if !isMined {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Header().Add("Content-Type", "application/json")
	default:
		log.Printf("Error: Invalid http method")
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (bcs *BlockchainServer) StartMine(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		bc := bcs.GetBlockchain(bcs.config)
		m := miner.New(bc)
		m.Mining()
		w.WriteHeader(http.StatusOK)
		w.Header().Add("Content-Type", "application/json")
	default:
		log.Printf("Error: Invalid http method")
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (bcs *BlockchainServer) Amount(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		bcAddr := r.URL.Query().Get("blockchain_address")
		amount := bcs.GetBlockchain(bcs.config).CalculateTotalAmount(bcAddr)
		ar := &blockchain.AmountResponse{Amount: amount}
		encoded, _ := json.Marshal(ar)
		w.WriteHeader(http.StatusOK)
		w.Header().Add("Content-Type", "application/json")
		io.WriteString(w, string(encoded))
	default:
		log.Printf("Error: invalid http method")
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (bcs *BlockchainServer) Consensus(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPut:
		bc := bcs.GetBlockchain(bcs.config)
		replaced := bc.ResolveConflicts()
		w.Header().Add("Content-Type", "application/json")
		if replaced {
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (bcs *BlockchainServer) Run() {
	bcs.GetBlockchain(bcs.config).Run()
	mux := http.NewServeMux()
	mux.HandleFunc("/", bcs.GetChain)
	mux.HandleFunc("/transactions", bcs.Transactions)
	mux.HandleFunc("/mine", bcs.Mine)
	mux.HandleFunc("/mine/start", bcs.StartMine)
	mux.HandleFunc("/amount", bcs.Amount)
	mux.HandleFunc("/consensus", bcs.Consensus)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:", utils.GetHost())+strconv.Itoa(int(bcs.Port())), mux))
}
