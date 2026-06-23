// Command gateway is the production identity gateway entrypoint: wire-up, listen, run
// migrations, and run audit.Verify(n) at boot (refusing to start on a tamper mismatch).
// See ../../DECISION.md (Component layout).
package main

import (
	"log"
	"net/http"

	"github.com/agent-auth/gateway/internal/httpapi"
)

func main() {
	// TODO: open stores, run migrations, audit.Verify(n) — fail-closed on error.
	addr := ":8080"
	log.Printf("gateway listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, httpapi.NewRouter()))
}
