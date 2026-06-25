package httpapi

import "net/http"

// NewRouter registers gateway routes on a wired Server.
func NewRouter(s *Server) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/retrieve", s.handleRetrieve)
	mux.HandleFunc("/v1/stats", s.handleStats)
	mux.HandleFunc("/v1/stats/reset", s.handleStatsReset)
	mux.HandleFunc("/v1/audit", s.handleAudit)
	mux.HandleFunc("/v1/admin/agents", s.handleAdminAgents)
	return mux
}
