package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"os"
)

var privateKey *rsa.PrivateKey
var pkErr error

// func getOrGeneratePrivateKey() {
// 	// Generate a new private key.
// 	privateKey, pkErr = rsa.GenerateKey(rand.Reader, 2048)
// 	if pkErr != nil {
// 		log.Fatalf("failed to create private key: %v", pkErr)
// 	}

// 	// Encode the private key into PEM format.
// 	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
// 	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
// 		Type:  "RSA PRIVATE KEY",
// 		Bytes: privateKeyBytes,
// 	})

// 	// Write the PEM to a file.
// 	pkErr = os.WriteFile("cp.pem", privateKeyPEM, 0600)
// 	if pkErr != nil {
// 		log.Fatalf("failed to write private key to disk: %v", pkErr)
// 	}
// }

func getOrGeneratePrivateKey() {
	// Check if the private key file exists.
	_, pkErr = os.Stat("cp.pem")
	if os.IsNotExist(pkErr) {
		// The private key file does not exist, so generate a new key.
		privateKey, pkErr = rsa.GenerateKey(rand.Reader, 2048)
		if pkErr != nil {
			log.Fatalf("failed to create private key: %v", pkErr)
		}

		// Encode the private key into PEM format.
		privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
		privateKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: privateKeyBytes,
		})

		// Write the PEM to a file.
		pkErr = os.WriteFile("cp.pem", privateKeyPEM, 0600)
		if pkErr != nil {
			log.Fatalf("failed to write private key to disk: %v", pkErr)
		}
	} else {
		var privateKeyPEM []byte
		// The private key file exists, so read it.
		privateKeyPEM, pkErr = os.ReadFile("cp.pem")
		if pkErr != nil {
			log.Fatalf("failed to read private key file: %v", pkErr)
		}

		// Decode the PEM file into a private key.
		block, _ := pem.Decode(privateKeyPEM)
		if block == nil || block.Type != "RSA PRIVATE KEY" {
			panic("failed to decode PEM block containing private key")
		}

		privateKey, pkErr = x509.ParsePKCS1PrivateKey(block.Bytes)
		if pkErr != nil {
			log.Fatalf("failed to parse encoded private key: %v", pkErr)
		}
	}
}

func signMessage() {
	// Message to sign.
	message := "Hello, world!"

	// Compute hash of the message.
	hash := sha256.New()
	hash.Write([]byte(message))
	hashedMessage := hash.Sum(nil)

	// Sign the hashed message.
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashedMessage)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Signature: %x\n", signature)
}
