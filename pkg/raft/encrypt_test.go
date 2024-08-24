package raft

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"testing"
)

func TestRsaEncrypt(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}

	publicKey := &privateKey.PublicKey

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})
	err = os.WriteFile("private.pem", privateKeyPEM, 0644)
	if err != nil {
		panic(err)
	}

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		panic(err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: publicKeyBytes,
	})
	err = os.WriteFile("public.pem", publicKeyPEM, 0644)
	if err != nil {
		panic(err)
	}
	publicKeyPEM2, err := os.ReadFile("public.pem")
	if err != nil {
		panic(err)
	}
	publicKeyBlock, _ := pem.Decode(publicKeyPEM2)
	publicKey2, err := x509.ParsePKIXPublicKey(publicKeyBlock.Bytes)
	if err != nil {
		panic(err)
	}

	plaintext := []byte("hello, world!")
	ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, publicKey2.(*rsa.PublicKey), plaintext)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Encrypted: %x\n", ciphertext)

	privateKeyPEM3, err := os.ReadFile("private.pem")
	if err != nil {
		panic(err)
	}
	privateKeyBlock, _ := pem.Decode(privateKeyPEM3)
	privateKey3, err := x509.ParsePKCS1PrivateKey(privateKeyBlock.Bytes)
	if err != nil {
		panic(err)
	}

	plaintext2, err := rsa.DecryptPKCS1v15(rand.Reader, privateKey3, ciphertext)
	if err != nil {
		panic(err.Error())
	}

	fmt.Printf("Decrypted: %s\n", plaintext2)
}
