package license

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"
	"unicode/utf8"
)

func ParsePublicKey(raw string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, fmt.Errorf("%w: public key is not PEM", ErrInvalidPayload)
	}
	if key, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PublicKey); ok {
			if rsaKey.N.BitLen() < MinRSABytes || rsaKey.N.BitLen() > MaxRSABytes {
				return nil, fmt.Errorf("%w: RSA modulus size", ErrInvalidPayload)
			}
			return rsaKey, nil
		}
	}
	if key, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		if key.N.BitLen() < MinRSABytes || key.N.BitLen() > MaxRSABytes {
			return nil, fmt.Errorf("%w: RSA modulus size", ErrInvalidPayload)
		}
		return key, nil
	}
	return nil, fmt.Errorf("%w: public key is not RSA", ErrInvalidPayload)
}

func ParsePrivateKey(raw []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("%w: private key is not PEM", ErrInvalidPayload)
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		if key.N.BitLen() >= MinRSABytes && key.N.BitLen() <= MaxRSABytes {
			return key, nil
		}
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: private key is not RSA", ErrInvalidPayload)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok || key.N.BitLen() < MinRSABytes || key.N.BitLen() > MaxRSABytes {
		return nil, fmt.Errorf("%w: RSA modulus size", ErrInvalidPayload)
	}
	return key, nil
}

func Verify(envelope Envelope, publicKey *rsa.PublicKey) (Verified, error) {
	if publicKey == nil || strings.TrimSpace(envelope.PayloadB64) == "" || strings.TrimSpace(envelope.SignatureB64) == "" || len(envelope.PayloadB64) > MaxPayloadB64Chars || len(envelope.SignatureB64) > MaxSignatureB64Chars {
		return Verified{}, ErrInvalidEnvelope
	}
	payloadRaw, err := base64.StdEncoding.DecodeString(envelope.PayloadB64)
	if err != nil || len(payloadRaw) == 0 || len(payloadRaw) > MaxPayloadBytes || !utf8.Valid(payloadRaw) || !json.Valid(payloadRaw) {
		return Verified{}, fmt.Errorf("%w: payload encoding", ErrInvalidEnvelope)
	}
	signature, err := base64.StdEncoding.DecodeString(envelope.SignatureB64)
	if err != nil || len(signature) == 0 {
		return Verified{}, fmt.Errorf("%w: signature encoding", ErrInvalidEnvelope)
	}
	digest := sha256.Sum256(payloadRaw)
	if err := rsa.VerifyPSS(publicKey, crypto.SHA256, digest[:], signature, &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: crypto.SHA256}); err != nil {
		return Verified{}, ErrInvalidSignature
	}
	payload, err := ParsePayload(payloadRaw)
	if err != nil {
		return Verified{}, err
	}
	return Verified{Payload: payload, PayloadRaw: append([]byte(nil), payloadRaw...), Signature: append([]byte(nil), signature...), SHA256: fmt.Sprintf("%x", digest)}, nil
}
