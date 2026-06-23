// Command ingest runs the Python ingestlib offline pipeline (WikiDoc + Synthea + Presidio).
package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

func main() {
	appDB := flag.String("app-db", "./app.db", "sqlite database path")
	ollama := flag.String("ollama", "http://127.0.0.1:11434", "ollama base url")
	keysDir := flag.String("keys", "./demo/keys", "demo agent pub keys for scope seeding")
	wikidocLimit := flag.Int("wikidoc-limit", 50, "HuggingFace WikiDoc row limit")
	flag.Parse()

	root, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	ingestPy := filepath.Join(root, "ingestlib", "ingest.py")
	cmd := exec.Command("python3", ingestPy,
		"--app-db", *appDB,
		"--ollama", *ollama,
		"--keys", *keysDir,
		"--wikidoc-limit", strconv.Itoa(*wikidocLimit),
		"--gateway-root", root,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}
