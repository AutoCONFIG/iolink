package license

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
)

type RawInput struct {
	Bytes     []byte
	SHA256    string
	Oversized bool
}

func ReadRawInput(r io.Reader) (RawInput, error) {
	hash := sha256.New()
	var buf bytes.Buffer
	limited := io.LimitReader(r, MaxEnvelopeBytes+1)
	if _, err := io.Copy(io.MultiWriter(hash, &buf), limited); err != nil {
		return RawInput{}, err
	}
	oversized := buf.Len() > MaxEnvelopeBytes
	if oversized {
		if _, err := io.Copy(hash, r); err != nil {
			return RawInput{}, err
		}
	}
	return RawInput{Bytes: buf.Bytes(), SHA256: fmt.Sprintf("%x", hash.Sum(nil)), Oversized: oversized}, nil
}
