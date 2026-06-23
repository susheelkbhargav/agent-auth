// Command dbinit opens app.db and applies embedded migrations (including sqlite-vec).
package main

import (
	"flag"
	"log"

	"github.com/agent-auth/gateway/internal/store"
)

func main() {
	dbPath := flag.String("app-db", "./app.db", "sqlite database path")
	flag.Parse()

	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	log.Printf("migrated %s", *dbPath)
}
