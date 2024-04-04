package main

import (
	"crypto"
	cryptoRand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	mathRand "math/rand"
	"os"
	"time"
)

func setPrivateKey() {
	// Check for existence of private key file. Note, private key exists in a
	// non-root directory when cp-api is run by cp-admin in an e2e test.
	var pkPath string
	if env != nil && *env == "e2e" {
		pkPath = "../../cp.pem"
	} else {
		pkPath = "cp.pem"
	}
	_, err := os.Stat(pkPath)

	// If the private key file does not exist, exit the program.
	if os.IsNotExist(err) {
		fmt.Printf("[err][api] private key file is not present; use cp-admin to generate and copy: %v [%s]\n", err, cts())
		os.Exit(1)
	} else {
		// The private key file exists; read it and set global variable.
		var privateKeyPEM []byte
		privateKeyPEM, err := os.ReadFile(pkPath)
		if err != nil {
			fmt.Printf("[err][api] reading private key file: %v [%s]\n", err, cts())
			os.Exit(1)
		}

		// Decode the PEM file into a private key.
		block, _ := pem.Decode(privateKeyPEM)
		if block == nil || block.Type != "RSA PRIVATE KEY" {
			fmt.Printf("[err][api] decoding PEM block containing private key [%s]\n", cts())
			os.Exit(1)
		}

		cpPrivateKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			fmt.Printf("[err][api] parsing encoded private key: %v [%s]\n", err, cts())
			os.Exit(1)
		}
	}
}

// Returns a base64Url encoded signature of the message.
func signMessage(msg string) string {
	// Compute hash of the message.
	hash := sha256.New()
	hash.Write([]byte(msg))
	hashedMessage := hash.Sum(nil)

	// Sign the hashed message.
	signature, err := rsa.SignPKCS1v15(cryptoRand.Reader, cpPrivateKey, crypto.SHA256, hashedMessage)
	if err != nil {
		panic(err)
	}

	return base64.URLEncoding.EncodeToString(signature)
}

// Verifies the signature of a message.
func verifySignature(msg, signature string) bool {
	// Compute hash of the message.
	hash := sha256.New()
	hash.Write([]byte(msg))
	hashedMessage := hash.Sum(nil)

	// Decode the signature.
	decodedSignature, err := base64.URLEncoding.DecodeString(signature)
	if err != nil {
		fmt.Printf("[err][api] decoding signature: %v [%s]\n", err, cts())
		return false
	}

	// Verify the signature.
	err = rsa.VerifyPKCS1v15(&cpPrivateKey.PublicKey, crypto.SHA256, hashedMessage, decodedSignature)
	if err != nil {
		fmt.Printf("[err][api] verifying signature: %v [%s]\n", err, cts())
		return false
	}

	return true
}

// Generates a 6 digit code for password-less login.
func generateLoginCode() int {
	return mathRand.Intn(900000) + 100000
}

// Returns a custom timestamp (cts) for time.Now() as Day/HH:MM:SS
func cts() string {
	t := time.Now()
	return fmt.Sprintf("%02d/%02d%02d%02d", t.Day(), t.Hour(), t.Minute(), t.Second())
}
