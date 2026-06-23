// Command gateway is the production identity gateway entrypoint.
package main

import (
	"log"
	"net/http"

	"github.com/agent-auth/gateway/internal/acl"
	"github.com/agent-auth/gateway/internal/audit"
	"github.com/agent-auth/gateway/internal/config"
	"github.com/agent-auth/gateway/internal/embed"
	"github.com/agent-auth/gateway/internal/gen"
	"github.com/agent-auth/gateway/internal/httpapi"
	"github.com/agent-auth/gateway/internal/retrieve"
	"github.com/agent-auth/gateway/internal/store"
	"github.com/agent-auth/gateway/internal/verify"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	aud := audit.NewSQLiteAudit(db.SQL)
	if err := aud.Verify(cfg.AuditVerifyRows); err != nil {
		log.Fatalf("audit verify: %v", err)
	}

	srv := &httpapi.Server{
		Cfg: cfg,
		DB:  db.SQL,
		Verifier: &verify.StandardVerifier{
			IssuerPubKey: cfg.IssuerPubKey,
			NonceStore:   store.NewMemNonceStore(cfg.ClockSkew),
			Audience:     cfg.OBOAud,
			ClockSkew:    cfg.ClockSkew,
		},
		ACL:       acl.NewSQLiteStore(db.SQL),
		Retriever: retrieve.NewSQLiteRetriever(db.SQL),
		Embedder:  embed.NewOllamaEmbedder(cfg.OllamaURL, "nomic-embed-text"),
		Gen: gen.Router{
			Local:    gen.NewOllamaGenerator(cfg.OllamaURL, cfg.LocalModel),
			Frontier: gen.NewOllamaGenerator(cfg.OllamaURL, cfg.FrontierModel),
		},
		Audit:     aud,
		AuditRead: aud,
	}

	log.Printf("gateway listening on %s (db=%s)", cfg.GatewayAddr, cfg.DBPath)
	log.Fatal(http.ListenAndServe(cfg.GatewayAddr, httpapi.NewRouter(srv)))
}
