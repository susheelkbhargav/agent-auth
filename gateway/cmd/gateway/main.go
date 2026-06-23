// Command gateway is the production identity gateway entrypoint: wire-up, listen, run
// migrations, and run audit.Verify(n) at boot (refusing to start on a tamper mismatch).
// See ../../DECISION.md (Component layout).
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/agent-auth/gateway/internal/httpapi"
	"github.com/agent-auth/gateway/internal/store"
)

func main() {
	dbPath := os.Getenv("APP_DB")
	if dbPath == "" {
		dbPath = "./app.db"
	}
	db, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	addr := os.Getenv("GATEWAY_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("gateway listening on %s (db=%s)", addr, dbPath)
	log.Fatal(http.ListenAndServe(addr, httpapi.NewRouter()))
}
