package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/MikhUd/blockchain/pkg/domain/signature"
	"github.com/MikhUd/blockchain/pkg/domain/transaction"
	"math/big"
)

func String2BigIntTuple(s string) (big.Int, big.Int) {
	bx, _ := hex.DecodeString(s[:64])
	by, _ := hex.DecodeString(s[64:])
	var (
		bix big.Int
		biy big.Int
	)
	_ = bix.SetBytes(bx)
	_ = biy.SetBytes(by)
	return bix, biy
}

func SignatureFromString(s string) *signature.Signature {
	x, y := String2BigIntTuple(s)
	return &signature.Signature{R: &x, S: &y}
}

func PublicECDSAKeyFromString(s string) *ecdsa.PublicKey {
	x, y := String2BigIntTuple(s)
	return &ecdsa.PublicKey{Curve: elliptic.P256(), X: &x, Y: &y}
}

func PrivateKeyFromString(s string, publicKey *ecdsa.PublicKey) *ecdsa.PrivateKey {
	b, _ := hex.DecodeString(s)
	var bi big.Int
	_ = bi.SetBytes(b)
	return &ecdsa.PrivateKey{PublicKey: *publicKey, D: &bi}
}

func GenerateTransactionSignature(
	sender string,
	senPrivateKey *ecdsa.PrivateKey,
	recipient string,
	value float32,
) (*signature.Signature, error) {
	tr := &transaction.Transaction{SenderBlockchainAddress: sender, RecipientBlockchainAddress: recipient, Value: value}
	encodedWalletTransaction, _ := json.Marshal(tr)
	hash := sha256.Sum256(encodedWalletTransaction)
	r, s, err := ecdsa.Sign(rand.Reader, senPrivateKey, hash[:])
	if err != nil {
		return nil, err
	}
	return &signature.Signature{
		R: r,
		S: s,
	}, nil
}

func GenerateKeyPair() (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return privateKey, &privateKey.PublicKey, nil
}

func PrivateKeyStr(privateKey *ecdsa.PrivateKey) string {
	return fmt.Sprintf("%x", privateKey.D.Bytes())
}

func PublicKeyStr(publicKey *ecdsa.PublicKey) string {
	return fmt.Sprintf("%064x%064x", publicKey.X.Bytes(), publicKey.Y.Bytes())
}
