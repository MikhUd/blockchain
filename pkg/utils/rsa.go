package utils

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

func PublicRSAKeyFromString(keyStr string) (*rsa.PublicKey, error) {
	publicKeyBlock, _ := pem.Decode([]byte(keyStr))
	publicKey, err := x509.ParsePKIXPublicKey(publicKeyBlock.Bytes)
	if err != nil {
		return nil, err
	}
	publicKeyRSA, ok := publicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not RSA")
	}
	return publicKeyRSA, nil
}
