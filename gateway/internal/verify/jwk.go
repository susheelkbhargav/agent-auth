package verify

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

type jwkKey struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
}

func ed25519FromJWK(raw json.RawMessage) (ed25519.PublicKey, error) {
	var k jwkKey
	if err := json.Unmarshal(raw, &k); err != nil {
		return nil, err
	}
	if k.Kty != "OKP" || k.Crv != "Ed25519" || k.X == "" {
		return nil, fmt.Errorf("unsupported jwk")
	}
	x, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, err
	}
	if len(x) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("bad ed25519 key length")
	}
	return ed25519.PublicKey(x), nil
}
