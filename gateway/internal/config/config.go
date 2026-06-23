// Package config loads gateway bootstrap settings from environment variables.
package config

import (
	"crypto/ed25519"
	"encoding/pem"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime settings for the gateway (see IMPLEMENTATION.md bootstrap table).
type Config struct {
	DBPath          string
	GatewayAddr     string
	IssuerPubKey    ed25519.PublicKey
	OBOAud          string
	ClockSkew       time.Duration
	DefaultK        int
	OllamaURL       string
	LocalModel      string
	FrontierModel   string
	AuditVerifyRows int
}

// Load reads configuration from the environment with documented defaults.
func Load() (Config, error) {
	cfg := Config{
		DBPath:          env("APP_DB", "./app.db"),
		GatewayAddr:     env("GATEWAY_ADDR", ":8080"),
		OBOAud:          env("OBO_AUD", "agent-auth"),
		ClockSkew:       envDuration("CLOCK_SKEW", 30*time.Second),
		DefaultK:        envInt("DEFAULT_K", 5),
		OllamaURL:       strings.TrimRight(env("OLLAMA_URL", "http://127.0.0.1:11434"), "/"),
		LocalModel:      env("LOCAL_MODEL", "phi4-mini"),
		FrontierModel:   env("FRONTIER_MODEL", "llama3.2"),
		AuditVerifyRows: envInt("AUDIT_VERIFY_ROWS", 0),
	}
	path := os.Getenv("ISSUER_PUBKEY_PATH")
	if path == "" {
		return cfg, fmt.Errorf("ISSUER_PUBKEY_PATH is required")
	}
	pub, err := loadEd25519PublicKey(path)
	if err != nil {
		return cfg, fmt.Errorf("issuer public key: %w", err)
	}
	cfg.IssuerPubKey = pub
	return cfg, nil
}

func loadEd25519PublicKey(path string) (ed25519.PublicKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(raw) == ed25519.PublicKeySize {
		return ed25519.PublicKey(raw), nil
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM or raw key file")
	}
	if block.Type == "PUBLIC KEY" || block.Type == "ED25519 PUBLIC KEY" {
		if len(block.Bytes) == ed25519.PublicKeySize {
			return ed25519.PublicKey(block.Bytes), nil
		}
	}
	return nil, fmt.Errorf("unsupported key format")
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envDuration(k string, def time.Duration) time.Duration {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
