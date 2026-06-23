package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/agent-auth/gateway/internal/acl"
	"github.com/agent-auth/gateway/internal/labelvocab"
	"github.com/agent-auth/gateway/internal/meter"
	"github.com/agent-auth/gateway/internal/resolve"
	"github.com/agent-auth/gateway/internal/retrieve"
	"github.com/agent-auth/gateway/internal/route"
	"github.com/agent-auth/gateway/internal/stats"
	"github.com/agent-auth/gateway/internal/verify"
)

type retrieveRequest struct {
	Query     string   `json:"query"`
	TaskScope []string `json:"task_scope"`
}

type retrieveResponse struct {
	Result string            `json:"result"`
	Chunks []chunkResponse   `json:"chunks"`
}

type chunkResponse struct {
	ID          string   `json:"id"`
	Text        string   `json:"text"`
	ParentDocID string   `json:"parent_doc_id"`
	TokenCount  int      `json:"token_count"`
	Labels      []string `json:"labels"`
	Score       float64  `json:"score"`
}

func (s *Server) handleRetrieve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "{}", http.StatusForbidden)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		write403(w)
		return
	}
	var req retrieveRequest
	if json.Unmarshal(body, &req) != nil || req.Query == "" {
		write403(w)
		return
	}

	vreq := &verify.Request{
		Body:      body,
		OBO:       bearerToken(r.Header.Get("Authorization")),
		Sig:       r.Header.Get("X-Sig"),
		Nonce:     r.Header.Get("X-Nonce"),
		Timestamp: r.Header.Get("X-Timestamp"),
		Method:    r.Method,
		URI:       r.URL.Path,
	}
	principal, err := s.Verifier.Verify(r.Context(), vreq)
	if err != nil {
		write403(w)
		return
	}
	if s.ACL.IsRevoked(principal.AgentKID) {
		writeUnifiedRefusal(w)
		return
	}

	reqTask := acl.LabelsFromStrings(req.TaskScope)
	oboTask := acl.LabelsFromStrings(principal.TaskScope)
	grants := acl.GrantsUnion(s.ACL, principal.UserRoles)
	agentScope := s.ACL.AgentScope(principal.AgentKID)
	eff := resolve.Effective(grants, agentScope, reqTask, oboTask)
	if len(eff) == 0 {
		writeUnifiedRefusal(w)
		return
	}

	qVec, err := s.Embedder.Embed(r.Context(), req.Query)
	if err != nil {
		writeUnifiedRefusal(w)
		return
	}
	k := s.Cfg.DefaultK
	shadow, err := s.Retriever.ShadowTopK(r.Context(), qVec, k)
	if err != nil {
		writeUnifiedRefusal(w)
		return
	}
	auth, err := s.Retriever.PrefilterTopK(r.Context(), qVec, eff, k)
	if err != nil {
		writeUnifiedRefusal(w)
		return
	}

	if len(auth) == 0 {
		m := meter.Compute(shadow, auth, eff, route.Refuse, meter.TierForShadow(shadow))
		if err := s.record(r, m, principal, eff, []string{}, route.Refuse); err != nil {
			write403(w)
			return
		}
		_ = stats.Add(s.DB, m)
		writeUnifiedRefusal(w)
		return
	}

	chunkLabels := make([]labelvocab.LabelSet, len(auth))
	for i, c := range auth {
		chunkLabels[i] = c.RequiredLabels
	}
	tier := route.Decide(chunkLabels)
	tierNaive := meter.TierForShadow(shadow)
	m := meter.Compute(shadow, auth, eff, tier, tierNaive)

	var answer string
	if tier == route.Refuse {
		writeUnifiedRefusal(w)
		return
	}
	gen := s.Gen.ForTier(tier == route.Local)
	answer, err = gen.Generate(r.Context(), req.Query, auth)
	if err != nil {
		writeUnifiedRefusal(w)
		return
	}

	ids := make([]string, len(auth))
	for i, c := range auth {
		ids[i] = c.ID
	}
	if err := s.record(r, m, principal, eff, ids, tier); err != nil {
		write403(w)
		return
	}
	_ = stats.Add(s.DB, m)

	writeJSON(w, http.StatusOK, retrieveResponse{
		Result: answer,
		Chunks: toChunkResponses(auth),
	})
}

func (s *Server) record(r *http.Request, m meter.Result, p *verify.Principal, eff labelvocab.LabelSet, chunkIDs []string, tier route.Tier) error {
	payload, _ := json.Marshal(map[string]any{
		"ts":            time.Now().UTC().Format(time.RFC3339),
		"user_id":       p.UserID,
		"agent_kid":     p.AgentKID,
		"effective":     eff.Strings(),
		"chunk_ids":     chunkIDs,
		"would_be":      m.WouldBeTokens,
		"auth_tokens":   m.AuthTokens,
		"leaks_blocked": m.LeaksBlocked,
		"tier":          tierString(tier),
		"path":          r.URL.Path,
	})
	_, err := s.Audit.Append(payload)
	return err
}

func tierString(t route.Tier) string {
	switch t {
	case route.Local:
		return "local"
	case route.Frontier:
		return "frontier"
	default:
		return "refuse"
	}
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		write403(w)
		return
	}
	snap, err := stats.Read(s.DB)
	if err != nil {
		write403(w)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		write403(w)
		return
	}
	if s.AuditRead == nil {
		write403(w)
		return
	}
	rows, err := s.AuditRead.List(100)
	if err != nil {
		write403(w)
		return
	}
	if err := s.Audit.Verify(0); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "chain invalid", "rows": rows})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "rows": rows})
}

type adminAgentRequest struct {
	PublicKey string   `json:"public_key"` // base64 raw 32-byte ed25519 pub
	Scope     []string `json:"scope"`
}

func (s *Server) handleAdminAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		write403(w)
		return
	}
	var req adminAgentRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.PublicKey == "" {
		write403(w)
		return
	}
	kid, err := kidFromB64Pub(req.PublicKey)
	if err != nil {
		write403(w)
		return
	}
	st, ok := s.ACL.(*acl.SQLiteStore)
	if !ok {
		write403(w)
		return
	}
	if err := st.RegisterAgent(kid, req.Scope); err != nil {
		write403(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"kid": kid, "scope": req.Scope})
}

func bearerToken(h string) string {
	const p = "Bearer "
	if len(h) > len(p) && h[:len(p)] == p {
		return h[len(p):]
	}
	return ""
}

func write403(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte("{}"))
}

func writeUnifiedRefusal(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, retrieveResponse{Result: unifiedRefusal, Chunks: []chunkResponse{}})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func toChunkResponses(chunks []retrieve.Chunk) []chunkResponse {
	out := make([]chunkResponse, len(chunks))
	for i, c := range chunks {
		labels := make([]string, 0, len(c.RequiredLabels))
		for l := range c.RequiredLabels {
			labels = append(labels, string(l))
		}
		out[i] = chunkResponse{
			ID: c.ID, Text: c.Text, ParentDocID: c.ParentDocID,
			TokenCount: c.TokenCount, Labels: labels, Score: c.Score,
		}
	}
	return out
}
