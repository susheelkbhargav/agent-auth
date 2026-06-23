// Command keygen writes Ed25519 issuer and demo agent keys (raw 32/64 bytes).
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {
	out := flag.String("out", "./demo/keys", "output directory")
	flag.Parse()
	if err := os.MkdirAll(*out, 0o700); err != nil {
		log.Fatal(err)
	}
	if err := writeKeyPair(filepath.Join(*out, "issuer")); err != nil {
		log.Fatal(err)
	}
	for _, name := range []string{"doctor", "billing", "patient"} {
		if err := writeKeyPair(filepath.Join(*out, name)); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println("keys written to", *out)
}

func writeKeyPair(prefix string) error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	if err := os.WriteFile(prefix+"_priv.raw", priv, 0o600); err != nil {
		return err
	}
	return os.WriteFile(prefix+"_pub.raw", pub, 0o600)
}
