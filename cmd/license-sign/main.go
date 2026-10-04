package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"git.hyhy.fun/rsplab/iolink/internal/license"
)

func main() {
	keyPath := flag.String("private-key", "", "path to a protected RSA private key PEM")
	flag.Parse()
	if *keyPath == "" {
		fail("private-key is required")
	}
	keyBytes, err := os.ReadFile(*keyPath)
	if err != nil {
		fail("private key unavailable")
	}
	info, err := os.Stat(*keyPath)
	if err != nil || info.Mode().Perm()&0o077 != 0 {
		fail("private key permissions invalid")
	}
	key, err := license.ParsePrivateKey(keyBytes)
	if err != nil {
		fail("private key invalid")
	}
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, license.MaxPayloadBytes+1))
	if err != nil || len(raw) > license.MaxPayloadBytes {
		fail("payload exceeds 45 KiB")
	}
	_, err = license.ParsePayload(raw)
	if err != nil {
		fail("payload invalid")
	}
	digest := sha256.Sum256(raw)
	signature, err := rsa.SignPSS(rand.Reader, key, crypto.SHA256, digest[:], &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: crypto.SHA256})
	if err != nil {
		fail("signature failed")
	}
	if len(base64.StdEncoding.EncodeToString(raw)) > 62000 {
		fail("encoded payload exceeds import limit")
	}
	if err := json.NewEncoder(os.Stdout).Encode(license.Envelope{PayloadB64: base64.StdEncoding.EncodeToString(raw), SignatureB64: base64.StdEncoding.EncodeToString(signature)}); err != nil {
		fail("output failed")
	}
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
