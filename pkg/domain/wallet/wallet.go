package wallet

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/json"
	"github.com/MikhUd/blockchain/pkg/utils"
	"github.com/btcsuite/btcutil/base58"
	"golang.org/x/crypto/ripemd160"
)

type Wallet struct {
	privateKey     *ecdsa.PrivateKey
	publicKey      *ecdsa.PublicKey
	blockchainAddr string
}

// New создает новый кошелек для блокчейна.
func New(version uint8) *Wallet {
	wallet := &Wallet{}

	privateKey, publicKey, err := utils.GenerateKeyPair()
	if err != nil {
		panic(err)
	}

	wallet.privateKey = privateKey
	wallet.publicKey = publicKey

	publicKeyHash := hashPublicKey(wallet.publicKey)
	wallet.blockchainAddr = generateAddr(version, publicKeyHash)

	return wallet
}

// BlockchainAddr возвращает сгенерированный блокчейн адрес кошелька
func (w *Wallet) BlockchainAddr() string {
	return w.blockchainAddr
}

func (w *Wallet) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		PrivateKey        string `json:"private_key"`
		PublicKey         string `json:"public_key"`
		BlockchainAddress string `json:"blockchain_address"`
	}{
		PrivateKey:        utils.PrivateKeyStr(w.privateKey),
		PublicKey:         utils.PublicKeyStr(w.publicKey),
		BlockchainAddress: w.BlockchainAddr(),
	})
}

// PrivateKey возвращает приватный ключ в формате *ecdsa.PrivateKey
func (w *Wallet) PrivateKey() *ecdsa.PrivateKey {
	return w.privateKey
}

// PublicKey возвращает приватный ключ в формате *ecdsa.PublicKey
func (w *Wallet) PublicKey() *ecdsa.PublicKey {
	return w.publicKey
}

// hashPublicKey хеширует публичный ключ ECDSA.
func hashPublicKey(publicKey *ecdsa.PublicKey) []byte {
	hash := sha256.New()
	hash.Write(publicKey.X.Bytes())
	hash.Write(publicKey.Y.Bytes())
	digest := hash.Sum(nil)

	hash = ripemd160.New()
	hash.Write(digest)
	return hash.Sum(nil)
}

// generateAddr создает адрес блокчейна на основе публичного ключа и версии.
func generateAddr(version byte, publicKeyHash []byte) string {
	versionDigest := make([]byte, 21)
	versionDigest[0] = version
	copy(versionDigest[1:], publicKeyHash)

	hash := sha256.New()
	hash.Write(versionDigest)
	digest := hash.Sum(nil)

	hash = sha256.New()
	hash.Write(digest)
	digest = hash.Sum(nil)

	checksum := digest[:4]

	digestChecksum := make([]byte, 25)
	copy(digestChecksum[:21], versionDigest[:])
	copy(digestChecksum[21:], checksum[:])

	return base58.Encode(digestChecksum)
}

type TransactionRequest struct {
	SenderPublicKey            *string `json:"sender_public_key"`
	SenderPrivateKey           *string `json:"sender_private_key"`
	SenderBlockchainAddress    *string `json:"sender_blockchain_address"`
	RecipientBlockchainAddress *string `json:"recipient_blockchain_address"`
	Value                      *string `json:"value"`
}

func (tr *TransactionRequest) Validate() bool {
	if tr.SenderPublicKey == nil || tr.SenderPrivateKey == nil || tr.SenderBlockchainAddress == nil || tr.RecipientBlockchainAddress == nil || tr.Value == nil {
		return false
	}
	return true
}
