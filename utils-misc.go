package main

import (
	"crypto"
	"crypto/rand"
	cryptoRand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"log"
	mathRand "math/rand"
	"os"
)

var cpPrivateKey *rsa.PrivateKey

func getOrGeneratePrivateKey() {
	// Check if the private key file exists.
	_, err := os.Stat("cp.pem")
	if os.IsNotExist(err) {
		var err error
		// The private key file does not exist, so generate a new key.
		cpPrivateKey, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			log.Fatalf("[error-api] creating private key: %v", err)
		}

		// Encode the private key into PEM format.
		privateKeyBytes := x509.MarshalPKCS1PrivateKey(cpPrivateKey)
		privateKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: privateKeyBytes,
		})

		// Write the PEM to a file.
		err = os.WriteFile("cp.pem", privateKeyPEM, 0600)
		if err != nil {
			log.Fatalf("[error-api] writing private key to disk: %v", err)
		}
	} else {
		var privateKeyPEM []byte
		// The private key file exists, so read it.
		privateKeyPEM, err := os.ReadFile("cp.pem")
		if err != nil {
			log.Fatalf("[error-api] reading private key file: %v", err)
		}

		// Decode the PEM file into a private key.
		block, _ := pem.Decode(privateKeyPEM)
		if block == nil || block.Type != "RSA PRIVATE KEY" {
			log.Fatalf("[error-api] decoding PEM block containing private key")
		}

		cpPrivateKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			log.Fatalf("[error-api] parsing encoded private key: %v", err)
		}
	}
}

func signMessage(msg string) []byte {
	// Compute hash of the message.
	hash := sha256.New()
	hash.Write([]byte(msg))
	hashedMessage := hash.Sum(nil)

	// Sign the hashed message.
	signature, err := rsa.SignPKCS1v15(cryptoRand.Reader, cpPrivateKey, crypto.SHA256, hashedMessage)
	if err != nil {
		panic(err)
	}

	return signature
}

// Generates a 6 digit code for password-less login.
func generateLoginCode() int {
	return mathRand.Intn(900000) + 100000
}
