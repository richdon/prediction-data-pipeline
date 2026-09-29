package main

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func signRequest(key ed25519.PrivateKey, method, path string) (string, string) {
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	path = strings.Split(path, "?")[0]
	msg := ts + method + path
	sig := ed25519.Sign(key, []byte(msg))
	return base64.StdEncoding.EncodeToString(sig), ts
}

func buildAuthHeaders(keyPath, apiKeyID, method,  urlPath, endpoint string) http.Header {
	pk, err := loadPrivateKey(keyPath)
	if err != nil {
		log.Panicln(err)
	}
	sig, ts := signRequest(pk, method, urlPath + endpoint)
	headers := http.Header{}
	headers.Set("KALSHI-ACCESS-KEY", apiKeyID)
	headers.Set("KALSHI-ACCESS-SIGNATURE", sig)
	headers.Set("KALSHI-ACCESS-TIMESTAMP", ts)
	return headers
}

func loadPrivateKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Decode the private key remove the -----BEGIN PRIVATE KEY----- parts
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	// Get the key
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	// Use reflection to validate the return key is the required type because its an interface
	privKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an ed25519 private key")
	}
	return privKey, nil
}