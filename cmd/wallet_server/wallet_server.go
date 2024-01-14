package wallet_server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/MikhUd/blockchain/internal/domain/blockchain"
	"github.com/MikhUd/blockchain/internal/domain/wallet"
	"github.com/MikhUd/blockchain/internal/domain/wallet_transaction"
	"github.com/MikhUd/blockchain/internal/utils"
	"html/template"
	"io"
	"log"
	"net/http"
	"path"
	"strconv"
)

type WalletServer struct {
	port    uint16
	gateway string
}

const tempDir = "./cmd/wallet_server/templates"

func New(port uint16, gateway string) *WalletServer {
	return &WalletServer{port: port, gateway: gateway}
}

func (ws *WalletServer) Port() uint16 {
	return ws.port
}

func (ws *WalletServer) Gateway() string {
	return ws.gateway
}

func (ws *WalletServer) Index(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		t, _ := template.ParseFiles(path.Join(tempDir, "index.html"))
		t.Execute(w, "")
	default:
		log.Printf("Error: Invalid http method")
	}
}

func (ws *WalletServer) Wallet(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		w.Header().Add("Content-Type", "application/json")
		myWallet := wallet.New(0)
		m, _ := myWallet.MarshalJSON()
		io.WriteString(w, string(m))
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		log.Printf("ERROR: Invalid http method")
	}
}

func (ws *WalletServer) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		d := json.NewDecoder(r.Body)
		var t wallet.TransactionRequest
		err := d.Decode(&t)
		if err != nil {
			log.Printf("ERROR: Invalid create transaction request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if !t.Validate() {
			log.Println("ERROR: missing field(s)")
			w.WriteHeader(http.StatusBadRequest)
		}
		publicKey := utils.PublicKeyFromString(*t.SenderPublicKey)
		privateKey := utils.PrivateKeyFromString(*t.SenderPrivateKey, publicKey)
		value, err := strconv.ParseFloat(*t.Value, 32)
		if err != nil {
			log.Println("ERROR: parse error")
			w.WriteHeader(http.StatusInternalServerError)
		}
		value32 := float32(value)
		w.Header().Add("Content-Type", "application/json")
		transaction := wallet_transaction.New(privateKey, publicKey, *t.SenderBlockchainAddress, *t.RecipientBlockchainAddress, value32)
		sign := transaction.GenerateSignature()
		signStr := sign.String()
		bt := &blockchain.TransactionRequest{
			SenderBlockchainAddress:    t.SenderBlockchainAddress,
			RecipientBlockchainAddress: t.RecipientBlockchainAddress,
			SenderPublicKey:            t.SenderPublicKey,
			Value:                      &value32,
			Signature:                  &signStr,
		}
		encoded, _ := json.Marshal(bt)
		buf := bytes.NewBuffer(encoded)

		resp, _ := http.Post(fmt.Sprintf("http://%s", ws.Gateway()+"/transactions"), "application/json", buf)
		w.WriteHeader(resp.StatusCode)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		log.Printf("ERROR: Invalid http method")
	}
}

func (ws *WalletServer) WalletAmount(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		bcAddr := r.URL.Query().Get("blockchain_address")
		endpoind := fmt.Sprintf("http://%s/amount", ws.Gateway())

		client := &http.Client{}
		bcsReq, err := http.NewRequest("GET", endpoind, nil)
		if err != nil {
			panic(err)
		}
		q := bcsReq.URL.Query()
		q.Add("blockchain_address", bcAddr)
		bcsReq.URL.RawQuery = q.Encode()

		bcsResp, err := client.Do(bcsReq)
		if err != nil {
			log.Printf("Error: %v", err)
			w.WriteHeader(http.StatusBadRequest)
		}
		w.Header().Add("Content-Type", "application/json")
		if bcsResp.StatusCode == http.StatusOK {
			decoder := json.NewDecoder(bcsResp.Body)
			var bar blockchain.AmountResponse
			err := decoder.Decode(&bar)
			if err != nil {
				log.Printf("Error: %v", err)
				w.WriteHeader(http.StatusBadRequest)
			}
			encoded, _ := json.Marshal(struct {
				Message string  `json:"message"`
				Amount  float32 `json:"amount"`
			}{
				Message: "success",
				Amount:  bar.Amount,
			})
			io.WriteString(w, string(encoded))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	default:
		log.Printf("Error: invalid http method")
		w.WriteHeader(http.StatusBadRequest)
	}
}

func (ws *WalletServer) Run() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", ws.Index)
	mux.HandleFunc("/wallet", ws.Wallet)
	mux.HandleFunc("/wallet/amount", ws.WalletAmount)
	mux.HandleFunc("/transaction", ws.CreateTransaction)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+strconv.Itoa(int(ws.Port())), mux))
}
