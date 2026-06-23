package httpapi

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

func kidFromB64Pub(b64 string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(b64)
	if err != nil {
		raw, err = base64.StdEncoding.DecodeString(b64)
	}
	if err != nil {
		return "", err
	}
	if len(raw) != ed25519.PublicKeySize {
		return "", fmt.Errorf("bad key size")
	}
	return hex.EncodeToString(sha256Sum(raw)), nil
}

func sha256Sum(b []byte) []byte {
	s := sha256.Sum256(b)
	return s[:]
}
