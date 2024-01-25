package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"github.com/MikhUd/blockchain/internal/domain/public_key"
	"github.com/MikhUd/blockchain/internal/domain/signature"
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

func PublicKeyFromString(s string) *ecdsa.PublicKey {
	x, y := String2BigIntTuple(s)
	return &ecdsa.PublicKey{Curve: elliptic.P256(), X: &x, Y: &y}
}

func PrivateKeyFromString(s string, publicKey *ecdsa.PublicKey) *ecdsa.PrivateKey {
	b, _ := hex.DecodeString(s)
	var bi big.Int
	_ = bi.SetBytes(b)
	return &ecdsa.PrivateKey{PublicKey: *publicKey, D: &bi}
}

func GenerateRandomSignature() (*signature.Signature, error) {
	// Генерация случайных байт для R и S
	randomBytes := make([]byte, 64)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, err
	}

	// Преобразование байт в строки
	randomHex := hex.EncodeToString(randomBytes)

	// Создание big.Int из строк
	r, s := String2BigIntTuple(randomHex)

	// Возвращение новой сигнатуры
	return &signature.Signature{R: &r, S: &s}, nil
}

func GenerateRandomPublicKey() (*public_key.PublicKey, error) {
	// Генерация случайных байт для X и Y
	randomBytes := make([]byte, 64)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, err
	}

	// Преобразование байт в строку
	randomHex := hex.EncodeToString(randomBytes)

	// Создание *ecdsa.PublicKey из строки
	pk := PublicKeyFromString(randomHex)

	return &public_key.PublicKey{Curve: pk.Curve, X: pk.X, Y: pk.Y}, nil
}
