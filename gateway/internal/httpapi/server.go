package httpapi

import (
	"database/sql"

	"github.com/agent-auth/gateway/internal/acl"
	"github.com/agent-auth/gateway/internal/audit"
	"github.com/agent-auth/gateway/internal/config"
	"github.com/agent-auth/gateway/internal/embed"
	"github.com/agent-auth/gateway/internal/gen"
	"github.com/agent-auth/gateway/internal/retrieve"
	"github.com/agent-auth/gateway/internal/verify"
)

const unifiedRefusal = "not permitted or no data"

// Server holds wired gateway dependencies.
type Server struct {
	Cfg       config.Config
	DB        *sql.DB
	Verifier  verify.Verifier
	ACL       acl.Store
	Retriever retrieve.Retriever
	Embedder  embed.Embedder
	Gen       gen.Router
	Audit     audit.Appender
	AuditRead *audit.SQLiteAudit
}
