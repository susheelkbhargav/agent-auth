package verify_test

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"testing"
	"time"

	"github.com/agent-auth/gateway/internal/store"
	"github.com/agent-auth/gateway/internal/verify"
)

func TestStandardVerifier_RoundTrip(t *testing.T) {
	t.Parallel()
	issuerPub, issuerPriv, _ := ed25519.GenerateKey(nil)
	agentPub, agentPriv, _ := ed25519.GenerateKey(nil)

	body := []byte(`{"query":"lab","task_scope":["lab"]}`)
	obo, err := verify.MintOBO(issuerPriv, "agent-auth", "alice", []string{"provider"}, []string{"lab"}, agentPub, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	jti := "test-jti-001"
	dpop, err := verify.MintDPoP(agentPriv, "POST", "/v1/retrieve", obo, body, jti, time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}

	v := &verify.StandardVerifier{
		IssuerPubKey: issuerPub,
		NonceStore:   store.NewMemNonceStore(time.Minute),
		Audience:     "agent-auth",
		ClockSkew:    time.Minute,
	}
	p, err := v.Verify(context.Background(), &verify.Request{
		Body: body, OBO: obo, Sig: dpop, Nonce: jti,
		Method: "POST", URI: "/v1/retrieve",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.UserID != "alice" || len(p.UserRoles) != 1 {
		t.Fatalf("principal: %+v", p)
	}
	b, _ := json.Marshal(p)
	if len(b) == 0 {
		t.Fatal("empty")
	}
}
