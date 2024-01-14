package wallet

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
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

	privateKey, publicKey, err := generateKeyPair()
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
		PrivateKey:        w.PrivateKeyStr(),
		PublicKey:         w.PublicKeyStr(),
		BlockchainAddress: w.BlockchainAddr(),
	})
}

// PrivateKey возвращает приватный ключ в формате *ecdsa.PrivateKey
func (w *Wallet) PrivateKey() *ecdsa.PrivateKey {
	return w.privateKey
}

// PrivateKeyStr возвращает приватный ключ в формате string
func (w *Wallet) PrivateKeyStr() string {
	return fmt.Sprintf("%x", w.privateKey.D.Bytes())
}

// PublicKey возвращает приватный ключ в формате *ecdsa.PublicKey
func (w *Wallet) PublicKey() *ecdsa.PublicKey {
	return w.publicKey
}

// PublicKeyStr возвращает приватный ключ в формате string
func (w *Wallet) PublicKeyStr() string {
	return fmt.Sprintf("%064x%064x", w.publicKey.X.Bytes(), w.publicKey.Y.Bytes())
}

// generateKeyPair генерирует приватный и публичный ключи ECDSA.
func generateKeyPair() (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return privateKey, &privateKey.PublicKey, nil
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
