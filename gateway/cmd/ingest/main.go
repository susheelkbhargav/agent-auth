// Command ingest is the OFFLINE pipeline: chunk → label → embed → load Chroma + ACL store.
// It never runs on the request path; labels are frozen here and read deterministically at
// request time. See ../../DECISION.md (Ingest & ACL store).
package main

import "log"

func main() {
	// TODO: load corpus, sensitivity-aware chunk, tiered label (rule + Presidio),
	// embed, write Chroma payloads + ACL store rows.
	log.Println("ingest: not yet implemented")
}
