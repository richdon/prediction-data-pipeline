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

// signRequest signs a Kalshi request with key and returns the base64 signature
// and the millisecond timestamp it signed. The signed message is
// timestamp + method + path, with any query string stripped from path.
func signRequest(key ed25519.PrivateKey, method, path string) (string, string) {
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	path = strings.Split(path, "?")[0]
	msg := ts + method + path
	sig := ed25519.Sign(key, []byte(msg))
	return base64.StdEncoding.EncodeToString(sig), ts
}

// buildAuthHeaders returns the KALSHI-ACCESS-* headers for a request to
// urlPath+endpoint. It loads the private key from keyPath on every call and
// panics if the key cannot be loaded.
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

// loadPrivateKey reads a PEM-encoded PKCS#8 file at path and returns the
// Ed25519 private key it contains.
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