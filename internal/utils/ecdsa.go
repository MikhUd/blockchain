package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
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
