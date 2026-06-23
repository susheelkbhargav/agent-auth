package verify

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/agent-auth/gateway/internal/store"
	"github.com/golang-jwt/jwt/v5"
)

// StandardVerifier implements Method A: DPoP jwk in header, OBO from issuer key.
type StandardVerifier struct {
	IssuerPubKey ed25519.PublicKey
	NonceStore   store.NonceStore
	Audience     string
	ClockSkew    time.Duration
}

// Verify runs PoP → nonce → OBO; any failure returns ErrDenied only.
func (v *StandardVerifier) Verify(ctx context.Context, r *Request) (*Principal, error) {
	if r == nil {
		return nil, ErrDenied
	}
	agentPub, dpopClaims, err := v.verifyDPoP(r)
	if err != nil {
		return nil, ErrDenied
	}
	if err := v.checkDPoPClaims(r, dpopClaims); err != nil {
		return nil, ErrDenied
	}
	if v.NonceStore != nil {
		seen, err := v.NonceStore.SeenBefore(ctx, dpopClaims.JTI)
		if err != nil || seen {
			return nil, ErrDenied
		}
	}
	oboClaims, err := v.verifyOBO(r.OBO)
	if err != nil {
		return nil, ErrDenied
	}
	if err := bindDPoPToOBO(r.OBO, agentPub, dpopClaims, oboClaims); err != nil {
		return nil, ErrDenied
	}
	return &Principal{
		UserID:    oboClaims.Subject,
		UserRoles: oboClaims.UserRoles,
		AgentKID:  kidFromPub(agentPub),
		TaskScope: oboClaims.TaskScope,
	}, nil
}

type dpopClaims struct {
	HTM      string `json:"htm"`
	HTU      string `json:"htu"`
	ATH      string `json:"ath"`
	JTI      string `json:"jti"`
	BodyHash string `json:"body_hash"`
	IAT      int64  `json:"iat"`
	jwt.RegisteredClaims
}

type oboClaims struct {
	UserRoles []string `json:"user_roles"`
	TaskScope []string `json:"task_scope"`
	Act       string   `json:"act"`
	jwt.RegisteredClaims
}

func (v *StandardVerifier) verifyDPoP(r *Request) (ed25519.PublicKey, *dpopClaims, error) {
	var agentPub ed25519.PublicKey
	claims := &dpopClaims{}
	_, err := jwt.ParseWithClaims(r.Sig, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodEdDSA {
			return nil, fmt.Errorf("bad alg")
		}
		raw, ok := t.Header["jwk"]
		if !ok {
			return nil, fmt.Errorf("missing jwk")
		}
		b, err := json.Marshal(raw)
		if err != nil {
			return nil, err
		}
		pub, err := ed25519FromJWK(b)
		if err != nil {
			return nil, err
		}
		agentPub = pub
		return pub, nil
	}, jwt.WithValidMethods([]string{"EdDSA"}))
	if err != nil {
		return nil, nil, err
	}
	return agentPub, claims, nil
}

func (v *StandardVerifier) checkDPoPClaims(r *Request, c *dpopClaims) error {
	if c.HTM != r.Method || c.HTU != r.URI || c.JTI == "" {
		return fmt.Errorf("bind")
	}
	if r.Nonce != "" && c.JTI != r.Nonce {
		return fmt.Errorf("nonce")
	}
	wantBody := base64.RawURLEncoding.EncodeToString(sha256Sum(r.Body))
	if c.BodyHash != wantBody {
		return fmt.Errorf("body")
	}
	if err := checkClock(r.Timestamp, c.IAT, v.ClockSkew); err != nil {
		return err
	}
	return nil
}

func (v *StandardVerifier) verifyOBO(raw string) (*oboClaims, error) {
	claims := &oboClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		return v.IssuerPubKey, nil
	}, jwt.WithValidMethods([]string{"EdDSA"}))
	if err != nil {
		return nil, err
	}
	if v.Audience != "" {
		ok := false
		for _, aud := range claims.Audience {
			if aud == v.Audience {
				ok = true
				break
			}
		}
		if !ok {
			return nil, fmt.Errorf("aud")
		}
	}
	if claims.ExpiresAt != nil && time.Now().After(claims.ExpiresAt.Time) {
		return nil, fmt.Errorf("exp")
	}
	if len(claims.TaskScope) == 0 {
		return nil, fmt.Errorf("task_scope")
	}
	return claims, nil
}

func bindDPoPToOBO(oboRaw string, agentPub ed25519.PublicKey, d *dpopClaims, o *oboClaims) error {
	act := kidFromPub(agentPub)
	if o.Act != act {
		return fmt.Errorf("act")
	}
	wantATH := base64.RawURLEncoding.EncodeToString(sha256Sum([]byte(oboRaw)))
	if d.ATH != wantATH {
		return fmt.Errorf("ath")
	}
	return nil
}

func kidFromPub(pub ed25519.PublicKey) string {
	return hex.EncodeToString(sha256Sum(pub))
}

func sha256Sum(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}

func checkClock(tsHeader string, iat int64, skew time.Duration) error {
	if tsHeader == "" {
		return nil
	}
	ts, err := strconv.ParseInt(tsHeader, 10, 64)
	if err != nil {
		return err
	}
	diff := math.Abs(float64(ts - iat))
	if time.Duration(diff)*time.Second > skew {
		return fmt.Errorf("skew")
	}
	return nil
}

// MintOBO creates a signed OBO JWT for tests and pylib parity (issuer private key).
func MintOBO(priv ed25519.PrivateKey, aud, sub string, roles, taskScope []string, agentPub ed25519.PublicKey, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := oboClaims{
		UserRoles: roles,
		TaskScope: taskScope,
		Act:       kidFromPub(agentPub),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   sub,
			Audience:  jwt.ClaimStrings{aud},
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	return tok.SignedString(priv)
}

// MintDPoP creates a DPoP JWT signed by the agent key (tests / tooling).
func MintDPoP(agentPriv ed25519.PrivateKey, method, uri, oboRaw string, body []byte, jti string, iat int64) (string, error) {
	x := base64.RawURLEncoding.EncodeToString(agentPriv.Public().(ed25519.PublicKey))
	claims := dpopClaims{
		HTM:      method,
		HTU:      uri,
		ATH:      base64.RawURLEncoding.EncodeToString(sha256Sum([]byte(oboRaw))),
		JTI:      jti,
		BodyHash: base64.RawURLEncoding.EncodeToString(sha256Sum(body)),
		IAT:      iat,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tok.Header["jwk"] = map[string]string{
		"kty": "OKP",
		"crv": "Ed25519",
		"x":   x,
	}
	return tok.SignedString(agentPriv)
}

// ParseLabelStrings converts string slices to label sets at HTTP boundary.
func ParseLabelStrings(ss []string) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
