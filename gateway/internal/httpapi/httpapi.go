// Package httpapi wires the routes. Error path → 403 {} or unified refusal. The agent response
// is chunks[] only — never denied_count or leaks_blocked (those go to /v1/stats + audit).
// See ../../DECISION.md (HTTP contracts).
package httpapi

import "net/http"

// NewRouter returns the gateway HTTP handler. Routes are registered as packages are implemented:
//   POST /v1/retrieve · GET /v1/stats · GET /v1/audit · POST /v1/admin/agents
func NewRouter() http.Handler {
	mux := http.NewServeMux()
	// TODO: register handlers (see ../../DECISION.md).
	return mux
}
