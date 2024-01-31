package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"regexp"
)

var privateKey *rsa.PrivateKey
var pkErr error

func getOrGeneratePrivateKey() {
	// Check if the private key file exists.
	_, pkErr = os.Stat("cp.pem")
	if os.IsNotExist(pkErr) {
		// The private key file does not exist, so generate a new key.
		privateKey, pkErr = rsa.GenerateKey(rand.Reader, 2048)
		if pkErr != nil {
			log.Fatalf("error creating private key: %v", pkErr)
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
			log.Fatalf("error writing private key to disk: %v", pkErr)
		}
	} else {
		var privateKeyPEM []byte
		// The private key file exists, so read it.
		privateKeyPEM, pkErr = os.ReadFile("cp.pem")
		if pkErr != nil {
			log.Fatalf("error reading private key file: %v", pkErr)
		}

		// Decode the PEM file into a private key.
		block, _ := pem.Decode(privateKeyPEM)
		if block == nil || block.Type != "RSA PRIVATE KEY" {
			panic("error decoding PEM block containing private key")
		}

		privateKey, pkErr = x509.ParsePKCS1PrivateKey(block.Bytes)
		if pkErr != nil {
			log.Fatalf("error parsing encoded private key: %v", pkErr)
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

func validateEmail(emailAddress string) error {
	var emailRX = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
	if !emailRX.MatchString(emailAddress) {
		return fmt.Errorf("error validating email address")
	}
	return nil
}

func sendEmail(recipient string, subject string, body string) error {
	emailSmtpServer := os.Getenv("EMAIL_SMTP_SERVER")
	emailPrimaryUser := os.Getenv("EMAIL_PRIMARY_USER")
	emailPassword := os.Getenv("EMAIL_PASSWORD")
	emailSenderAddress := os.Getenv("EMAIL_SENDER_ADDRESS")
	emailSenderName := os.Getenv("EMAIL_SENDER_NAME")

	// Email address is verified by the client, but check here again.
	if err := validateEmail(recipient); err != nil {
		return err
	}

	auth := smtp.PlainAuth(
		"",
		emailPrimaryUser,
		emailPassword,
		emailSmtpServer,
	)

	msg := []byte("To: " + recipient + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"From:  " + emailSenderName + " <" + emailSenderAddress + ">\r\n" + body + "\r\n")

	// Establish plain text connection to SMTP server.
	c, err := smtp.Dial(fmt.Sprintf("%s:587", emailSmtpServer))
	if err != nil {
		return fmt.Errorf("error connecting to SMTP server: %w", err)
	}
	// Upgrade to a secure connection using TLS.
	config := &tls.Config{ServerName: emailSmtpServer}
	if err = c.StartTLS(config); err != nil {
		return fmt.Errorf("error upgrading to secure connection with SMTP server: %w", err)
	}
	// Authenticate.
	if err = c.Auth(auth); err != nil {
		return fmt.Errorf("error authenticating with SMTP server: %w", err)
	}
	// Specify the sender.
	if err = c.Mail(emailSenderAddress); err != nil {
		return fmt.Errorf("error specifying sender to SMTP server: %w", err)
	}
	// Specify the recipient. Often a loop is used here, but we only have one.
	if err = c.Rcpt(recipient); err != nil {
		return fmt.Errorf("error specifying recipient to SMTP server: %w", err)
	}
	// Get a writer from the server. Write message, and close the writer.
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("error getting writer from SMTP server: %w", err)
	}
	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("error writing message to SMTP server: %w", err)
	}
	err = w.Close()
	if err != nil {
		return fmt.Errorf("error closing writer from SMTP server: %w", err)
	}
	// Send the QUIT command and close the connection.
	err = c.Quit()
	if err != nil {
		return fmt.Errorf("error closing connection to SMTP server: %w", err)
	}
	return nil
}
